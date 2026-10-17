// Package account_handler is the JSON API behind the dashboard (/api). Writes need the x-ricrac
// header, which cross-site forms can't send. Bodies are decoded leniently and errors answer
// {"error": message}, as the web app expects.
package account_handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ricrac/internal/api/server"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	account_service "ricrac/internal/services/account"
	oauth_service "ricrac/internal/services/oauth"
)

type handlers struct {
	keys    auth.Keys
	account *account_service.AccountService
	oauth   *oauth_service.OAuthService
}

type signedInHandler func(w http.ResponseWriter, r *http.Request, user uuid.UUID) error

type memberHandler func(w http.ResponseWriter, r *http.Request, actor *auth.Actor) error

func RegisterHandlers(srv *server.Server, api huma.API) {
	h := &handlers{keys: srv.Keys, account: srv.AccountService, oauth: srv.OAuthService}
	plain := func(method, path string, handler http.HandlerFunc) {
		operation := &huma.Operation{
			OperationID: "account_" + strings.ToLower(method) + strings.NewReplacer("/", "_", "{", "", "}", "").Replace(path),
			Method:      method,
			Path:        path,
			Hidden:      true,
		}
		api.OpenAPI().AddOperation(operation)
		api.Adapter().Handle(operation, func(ctx huma.Context) {
			r, w := humachi.Unwrap(ctx)
			requireAppHeader(handler)(w, r)
		})
	}
	signedIn := func(method, path string, handler signedInHandler) {
		plain(method, path, func(w http.ResponseWriter, r *http.Request) {
			user, err := h.signedInUser(r)
			if err == nil {
				err = handler(w, r, user)
			}
			if err != nil {
				errdefs.Write(w, err)
			}
		})
	}
	member := func(method, path string, handler memberHandler) {
		signedIn(method, path, func(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
			actor, err := h.account.Member(r.Context(), user, r.Header.Get("X-Ricrac-Workspace"))
			if err != nil {
				return err
			}
			return handler(w, r, actor)
		})
	}
	plain(http.MethodGet, "/api/config", h.config)
	plain(http.MethodGet, "/api/me", h.me)
	signedIn(http.MethodPatch, "/api/me", h.updateMe)
	signedIn(http.MethodPost, "/api/workspaces", h.createWorkspace)
	signedIn(http.MethodDelete, "/api/workspaces/{id}", h.deleteWorkspace)
	signedIn(http.MethodPost, "/api/workspaces/{id}/leave", h.leaveWorkspace)
	signedIn(http.MethodPost, "/api/invitations/{id}/accept", h.acceptInvitation)
	signedIn(http.MethodDelete, "/api/invitations/{id}", h.declineInvitation)
	signedIn(http.MethodGet, "/api/api-keys", h.listAPIKeys)
	signedIn(http.MethodPost, "/api/api-keys", h.createAPIKey)
	signedIn(http.MethodDelete, "/api/api-keys/{id}", h.deleteAPIKey)
	member(http.MethodGet, "/api/oauth-apps", h.listOAuthApps)
	member(http.MethodPost, "/api/oauth-apps", h.createOAuthApp)
	member(http.MethodPatch, "/api/oauth-apps/{id}", h.updateOAuthApp)
	member(http.MethodDelete, "/api/oauth-apps/{id}", h.deleteOAuthApp)
	member(http.MethodPost, "/api/oauth-apps/{id}/secret", h.rotateOAuthSecret)
	signedIn(http.MethodGet, "/api/connected-apps", h.connectedApps)
	signedIn(http.MethodDelete, "/api/connected-apps/{id}", h.revokeConnectedApp)
	plain(http.MethodGet, "/api/oauth/client", h.oauthClient)
	signedIn(http.MethodPost, "/api/oauth/decide", h.oauthDecide)
}

// NotFound answers the /api paths nothing serves.
func NotFound(w http.ResponseWriter, r *http.Request) {
	requireAppHeader(func(w http.ResponseWriter, r *http.Request) {
		errdefs.Write(w, errdefs.NotFound("Not found"))
	})(w, r)
}

func requireAppHeader(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exempt := strings.HasPrefix(r.URL.Path, "/api/auth/")
		if r.Method != http.MethodGet && !exempt && r.Header.Get("X-Ricrac") == "" {
			errdefs.Write(w, errdefs.Forbidden("Missing x-ricrac header"))
			return
		}
		next(w, r)
	}
}

func (h *handlers) signedInUser(r *http.Request) (uuid.UUID, error) {
	user, ok := h.keys.SessionUser(r)
	if !ok {
		return uuid.Nil, errdefs.Unauthorized()
	}
	if err := h.account.SignedIn(r.Context(), user); err != nil {
		return uuid.Nil, err
	}
	return user, nil
}

func decodeJSON(r *http.Request, into any) error {
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		return errdefs.BadRequest("The body isn't valid JSON: %s", err)
	}
	return nil
}

// pathParam is the path parameter, unescaped.
func pathParam(r *http.Request, name string) string {
	value := chi.URLParam(r, name)
	if unescaped, err := url.PathUnescape(value); err == nil {
		return unescaped
	}
	return value
}

func pathID(r *http.Request) (uuid.UUID, error) {
	value := pathParam(r, "id")
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errdefs.BadRequest("Invalid id %q", value)
	}
	return id, nil
}

func answer(w http.ResponseWriter, value any) error {
	errdefs.WriteJSON(w, http.StatusOK, value)
	return nil
}

func ok(w http.ResponseWriter) error {
	return answer(w, map[string]any{"ok": true})
}
