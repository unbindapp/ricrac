// Package oauthserver_handler mounts the OAuth 2.1 server on chi: the well-known metadata,
// dynamic registration, the token endpoint and revocation. They answer OAuth's own error shape.
package oauthserver_handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ricrac/internal/api/server"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	oauth_service "ricrac/internal/services/oauth"
)

type handlers struct {
	oauth *oauth_service.OAuthService
}

func Mount(r chi.Router, srv *server.Server) {
	h := &handlers{oauth: srv.OAuthService}
	r.Get("/.well-known/oauth-protected-resource", h.protectedResource)
	r.Get("/.well-known/oauth-protected-resource/mcp", h.protectedResource)
	r.Get("/.well-known/oauth-authorization-server", h.authorizationServer)
	r.Get("/.well-known/oauth-authorization-server/mcp", h.authorizationServer)
	r.Post("/oauth/register", h.register)
	r.Post("/oauth/token", h.token)
	r.Post("/oauth/revoke", h.revoke)
}

func (h *handlers) protectedResource(w http.ResponseWriter, r *http.Request) {
	errdefs.WriteJSON(w, http.StatusOK, h.oauth.ProtectedResource())
}

func (h *handlers) authorizationServer(w http.ResponseWriter, r *http.Request) {
	errdefs.WriteJSON(w, http.StatusOK, h.oauth.AuthorizationServer())
}

func writeError(w http.ResponseWriter, e *oauth_service.Error) {
	w.Header().Set("Cache-Control", "no-store")
	errdefs.WriteJSON(w, e.Status, map[string]any{"error": e.Code, "error_description": e.Description})
}

// writeFailure writes OAuth errors as they are and anything else as a logged server_error.
func writeFailure(w http.ResponseWriter, err error, log, description string) {
	var oauthErr *oauth_service.Error
	if errors.As(err, &oauthErr) {
		writeError(w, oauthErr)
		return
	}
	slog.Error(log, "err", err)
	writeError(w, &oauth_service.Error{Status: http.StatusInternalServerError, Code: "server_error", Description: description})
}

func (h *handlers) register(w http.ResponseWriter, r *http.Request) {
	if h.oauth.Throttled(auth.ClientIP(r)) {
		writeError(w, &oauth_service.Error{Status: http.StatusTooManyRequests, Code: "invalid_request", Description: "Too many registrations, try again later"})
		return
	}
	var input oauth_service.Registration
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, &oauth_service.Error{Status: http.StatusBadRequest, Code: "invalid_client_metadata", Description: "The body must be a JSON object"})
		return
	}
	registered, err := h.oauth.Register(r.Context(), &input)
	if err != nil {
		writeFailure(w, err, "client registration failed", "Registration failed")
		return
	}
	errdefs.WriteJSON(w, http.StatusCreated, registered)
}

func (h *handlers) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	tokens, err := h.oauth.Token(r.Context(), oauth_service.CredentialsOf(r.Header.Get("Authorization"), r.PostForm), r.PostForm)
	if err != nil {
		writeFailure(w, err, "token endpoint failed", "Token request failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	errdefs.WriteJSON(w, http.StatusOK, tokens)
}

func (h *handlers) revoke(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	err := h.oauth.Revoke(r.Context(), oauth_service.CredentialsOf(r.Header.Get("Authorization"), r.PostForm), r.PostForm)
	if err != nil {
		writeFailure(w, err, "revoking failed", "Revocation failed")
		return
	}
	w.WriteHeader(http.StatusOK)
}
