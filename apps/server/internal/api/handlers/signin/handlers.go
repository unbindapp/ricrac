// Package signin_handler mounts sign-in with Google, by emailed link, and sign-out on chi. Sign-in
// with Google answers with 303 redirects; failures land back on the page with
// ?signin_error=<reason>. Emailed links answer JSON, like the dashboard API.
package signin_handler

import (
	"encoding/json"
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
	r.Post("/api/auth/email", h.sendLink)
	r.Post("/api/auth/email/callback", h.finishLink)
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

type sendLinkInput struct {
	Email    string `json:"email"`
	ReturnTo string `json:"returnTo"`
}

type finishLinkInput struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func (h *handlers) sendLink(w http.ResponseWriter, r *http.Request) {
	var input sendLinkInput
	if err := readJSON(r, &input); err != nil {
		errdefs.Write(w, err)
		return
	}
	if err := h.signin.SendLink(r.Context(), input.Email, input.ReturnTo); err != nil {
		errdefs.Write(w, err)
		return
	}
	errdefs.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) finishLink(w http.ResponseWriter, r *http.Request) {
	var input finishLinkInput
	if err := readJSON(r, &input); err != nil {
		errdefs.Write(w, err)
		return
	}
	returnTo, user, err := h.signin.FinishLink(r.Context(), input.Email, input.Token)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	w.Header().Add("Set-Cookie", auth.BuildCookie(h.keys.SessionCookie, h.keys.SessionToken(user), auth.SessionTTL, auth.IsSecureRequest(h.cfg, r)))
	errdefs.WriteJSON(w, http.StatusOK, map[string]any{"returnTo": returnTo})
}

func readJSON(r *http.Request, into any) error {
	if r.Header.Get("X-Ricrac") == "" {
		return errdefs.Forbidden("Missing x-ricrac header")
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return errdefs.BadRequest("The body must be JSON like {\"email\": \"name@example.com\"}")
	}
	return nil
}
