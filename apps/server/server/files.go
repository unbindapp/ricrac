package server

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/app"
	"ricrac/ops"
)

// upload stores the file agents PUT to the one-time upload URL.
func upload(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := state.Config.MaxUploadSize
		body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
		if err != nil {
			app.WriteError(w, app.BadRequest("Couldn't read the body"))
			return
		}
		if int64(len(body)) > limit {
			app.WriteError(w, app.Errorf(http.StatusRequestEntityTooLarge, "Files can have at most %d bytes", limit))
			return
		}
		if len(body) == 0 {
			app.WriteError(w, app.BadRequest("The body is empty; send the file's bytes"))
			return
		}
		id, found, err := app.Maybe[uuid.UUID](r.Context(), state.DB,
			`UPDATE files SET data = $2, size = $3, upload_hash = NULL, upload_expires_at = NULL
			 WHERE upload_hash = $1 AND upload_expires_at > now() AND data IS NULL RETURNING id`,
			app.SecretHash(r.PathValue("token")), body, int64(len(body)))
		if err != nil {
			app.WriteError(w, err)
			return
		}
		if !found {
			app.WriteError(w, app.NotFound("This upload URL is unknown, expired or already used; get a new one with prepare_file_upload"))
			return
		}
		app.WriteJSON(w, http.StatusOK, M{"fileId": id, "assetUrl": ops.AssetURL(state.Config.PublicURL, id), "size": len(body)})
	}
}

// fileCaller is a token or a dashboard session.
func fileCaller(state *app.State, r *http.Request) (*app.Caller, error) {
	if r.Header.Get("Authorization") != "" {
		caller := bearerCaller(state, r)
		if caller == nil {
			return nil, app.Unauthorized()
		}
		return caller, caller.Require(app.Read)
	}
	user, ok := state.Keys.SessionUser(r)
	if !ok {
		return nil, app.Unauthorized()
	}
	return app.SessionCaller(r.Context(), state, user), nil
}

// download serves a file to members of its workspace.
func download(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := fileCaller(state, r)
		if err != nil {
			app.WriteError(w, err)
			return
		}
		notFound := app.NotFound("No such file")
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			app.WriteError(w, notFound)
			return
		}
		file, found, err := app.Row[struct {
			Workspace   uuid.UUID
			Name        string
			ContentType string
			Data        []byte
		}](r.Context(), state.DB, "SELECT workspace_id, name, content_type, data FROM files WHERE id = $1 AND data IS NOT NULL", id)
		if err != nil {
			app.WriteError(w, err)
			return
		}
		if !found {
			app.WriteError(w, notFound)
			return
		}
		reachable, err := caller.Reachable()
		if err != nil {
			app.WriteError(w, err)
			return
		}
		if !slices.ContainsFunc(reachable, func(w app.Reached) bool { return w.ID == file.Workspace }) {
			app.WriteError(w, notFound)
			return
		}
		h := w.Header()
		h.Set("Content-Type", file.ContentType)
		h.Set("Content-Disposition", disposition(file.Name, file.ContentType))
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "sandbox")
		h.Set("Cache-Control", "private, max-age=3600")
		_, _ = w.Write(file.Data)
	}
}

// disposition downloads types a browser would run instead of showing them.
func disposition(name, contentType string) string {
	kind := "inline"
	for _, active := range []string{"html", "svg", "xml", "javascript", "ecmascript"} {
		if strings.Contains(contentType, active) {
			kind = "attachment"
		}
	}
	ascii := strings.Map(func(c rune) rune {
		if c == ' ' || c > ' ' && c < 0x7f && c != '"' && c != '\\' {
			return c
		}
		return '_'
	}, name)
	var encoded strings.Builder
	for _, b := range []byte(name) {
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' {
			encoded.WriteByte(b)
			continue
		}
		fmt.Fprintf(&encoded, "%%%02X", b)
	}
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", kind, ascii, encoded.String())
}
