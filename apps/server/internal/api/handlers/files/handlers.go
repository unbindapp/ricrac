// Package files_handler serves uploaded files and takes the bytes PUT to one-time upload URLs.
package files_handler

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ricrac/internal/api/middleware"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
)

type Handler struct {
	srv   *server.Server
	authn *middleware.Auth
}

func Mount(r chi.Router, srv *server.Server, authn *middleware.Auth) {
	h := &Handler{srv: srv, authn: authn}
	r.Get("/files/{id}", h.Download)
	r.Put("/uploads/{token}", h.Upload)
}

// Upload stores the file agents PUT to the one-time upload URL.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	limit := h.srv.Cfg.MaxUploadSize
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		errdefs.Write(w, errdefs.BadRequest("Couldn't read the body"))
		return
	}
	if int64(len(body)) > limit {
		errdefs.Write(w, errdefs.Errorf(http.StatusRequestEntityTooLarge, "Files can have at most %d bytes", limit))
		return
	}
	if len(body) == 0 {
		errdefs.Write(w, errdefs.BadRequest("The body is empty; send the file's bytes"))
		return
	}
	received, err := h.srv.AttachmentService.ReceiveUpload(r.Context(), chi.URLParam(r, "token"), body)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	errdefs.WriteJSON(w, http.StatusOK, received)
}

// caller is a token or a dashboard session.
func (h *Handler) caller(r *http.Request) (*auth.Caller, error) {
	if r.Header.Get("Authorization") != "" {
		caller := h.authn.BearerCaller(r)
		if caller == nil {
			return nil, errdefs.Unauthorized()
		}
		return caller, caller.Require(auth.Read)
	}
	user, ok := h.srv.Keys.SessionUser(r)
	if !ok {
		return nil, errdefs.Unauthorized()
	}
	return auth.SessionCaller(user), nil
}

// Download serves a file to members of its workspace.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	caller, err := h.caller(r)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	notFound := errdefs.NotFound("No such file")
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errdefs.Write(w, notFound)
		return
	}
	file, err := h.srv.AttachmentService.File(r.Context(), id)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	if file == nil {
		errdefs.Write(w, notFound)
		return
	}
	reachable, err := h.srv.AccessService.Reachable(r.Context(), caller)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	if !slices.ContainsFunc(reachable, func(w auth.Reached) bool { return w.ID == file.WorkspaceID }) {
		errdefs.Write(w, notFound)
		return
	}
	header := w.Header()
	header.Set("Content-Type", file.ContentType)
	header.Set("Content-Disposition", disposition(file.Name, file.ContentType))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Security-Policy", "sandbox")
	header.Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(*file.Data)
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
