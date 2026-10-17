// Package signin_handler mounts sign-in with Google and sign-out on chi. Sign-in answers with
// 303 redirects; failures land back on the page with ?signin_error=<reason>.
package signin_handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ricrac/config"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	signin_service "ricrac/internal/services/signin"
)

const signinCookie = "rr_signin"

type handlers struct {
	cfg    config.Config
	keys   auth.Keys
	signin *signin_service.SigninService
}

func Mount(r chi.Router, srv *server.Server) {
	h := &handlers{cfg: srv.Cfg, keys: srv.Keys, signin: srv.SigninService}
	r.Get("/api/auth/sign-in", h.signIn)
	r.Get("/api/auth/callback/google", h.callback)
	r.Post("/api/auth/sign-out", h.signOut)
}

func (h *handlers) signIn(w http.ResponseWriter, r *http.Request) {
	returnTo := signin_service.SafeReturnTo(r.URL.Query().Get("return_to"))
	if !h.cfg.GoogleConfigured() {
		http.Redirect(w, r, signin_service.WithSigninError(returnTo, "not_configured"), http.StatusSeeOther)
		return
	}
	location, pending := h.signin.Start(returnTo)
	secure := auth.IsSecureRequest(h.cfg, r)
	w.Header().Add("Set-Cookie", auth.BuildCookie(h.cfg.CookieName(signinCookie), pending, signin_service.SigninTTL, secure))
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func (h *handlers) signOut(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Set-Cookie", auth.BuildCookie(h.keys.SessionCookie, "", 0, auth.IsSecureRequest(h.cfg, r)))
	errdefs.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) callback(w http.ResponseWriter, r *http.Request) {
	secure := auth.IsSecureRequest(h.cfg, r)
	cookieName := h.cfg.CookieName(signinCookie)
	w.Header().Add("Set-Cookie", auth.BuildCookie(cookieName, "", 0, secure))
	pending := ""
	if cookie, err := r.Cookie(cookieName); err == nil {
		pending = cookie.Value
	}
	returnTo, user, reason := h.signin.Finish(r.Context(), pending, r.URL.Query())
	if reason != "" {
		http.Redirect(w, r, signin_service.WithSigninError(returnTo, reason), http.StatusSeeOther)
		return
	}
	w.Header().Add("Set-Cookie", auth.BuildCookie(h.keys.SessionCookie, h.keys.SessionToken(user), auth.SessionTTL, secure))
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}
