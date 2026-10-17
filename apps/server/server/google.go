package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/app"
)

const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	signinCookie       = "rr_signin"
	signinTTL          = 10 * 60
)

var googleIssuers = []string{"https://accounts.google.com", "accounts.google.com"}

func googleRedirectURI(state *app.State) string {
	return state.Config.PublicURL + "/api/auth/callback/google"
}

// safeReturnTo only allows paths on this site, so sign-in can't be used as an open redirect.
func safeReturnTo(returnTo string) string {
	if strings.HasPrefix(returnTo, "/") && !strings.HasPrefix(returnTo, "//") && !strings.HasPrefix(returnTo, "/\\") {
		return returnTo
	}
	return "/"
}

func withSigninError(returnTo, reason string) string {
	separator := "?"
	if strings.Contains(returnTo, "?") {
		separator = "&"
	}
	return returnTo + separator + "signin_error=" + reason
}

// pendingSignin is the sign-in round trip, kept in a signed cookie.
type pendingSignin struct {
	expires  int64
	state    string
	nonce    string
	verifier string
	returnTo string
}

func (p pendingSignin) encode() string {
	return fmt.Sprintf("signin:%d:%s:%s:%s:%s", p.expires, p.state, p.nonce, p.verifier, p.returnTo)
}

func decodePending(payload string) (pendingSignin, bool) {
	rest, ok := strings.CutPrefix(payload, "signin:")
	if !ok {
		return pendingSignin{}, false
	}
	parts := strings.SplitN(rest, ":", 5)
	if len(parts) != 5 {
		return pendingSignin{}, false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return pendingSignin{}, false
	}
	return pendingSignin{expires, parts[1], parts[2], parts[3], parts[4]}, true
}

func signIn(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
		if !state.Config.GoogleConfigured() {
			http.Redirect(w, r, withSigninError(returnTo, "not_configured"), http.StatusSeeOther)
			return
		}
		pending := pendingSignin{
			expires:  time.Now().Unix() + signinTTL,
			state:    app.RandomSecret(""),
			nonce:    app.RandomSecret(""),
			verifier: app.RandomSecret(""),
			returnTo: returnTo,
		}
		challenge := sha256.Sum256([]byte(pending.verifier))
		query := url.Values{
			"response_type":         {"code"},
			"client_id":             {state.Config.GoogleClientID},
			"redirect_uri":          {googleRedirectURI(state)},
			"scope":                 {"openid email profile"},
			"state":                 {pending.state},
			"nonce":                 {pending.nonce},
			"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
			"code_challenge_method": {"S256"},
			"prompt":                {"select_account"},
		}
		secure := app.IsSecureRequest(state.Config, r)
		cookie := app.BuildCookie(state.Config.CookieName(signinCookie), state.Keys.Sign(pending.encode()), signinTTL, secure)
		w.Header().Add("Set-Cookie", cookie)
		http.Redirect(w, r, googleAuthorizeURL+"?"+query.Encode(), http.StatusSeeOther)
	}
}

func signOut(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", app.BuildCookie(state.Keys.SessionCookie, "", 0, app.IsSecureRequest(state.Config, r)))
		app.WriteJSON(w, http.StatusOK, M{"ok": true})
	}
}

func googleCallback(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secure := app.IsSecureRequest(state.Config, r)
		cookieName := state.Config.CookieName(signinCookie)
		w.Header().Add("Set-Cookie", app.BuildCookie(cookieName, "", 0, secure))
		fail := func(returnTo, reason string) {
			http.Redirect(w, r, withSigninError(returnTo, reason), http.StatusSeeOther)
		}
		var pending pendingSignin
		found := false
		if cookie, err := r.Cookie(cookieName); err == nil {
			if payload, ok := state.Keys.Verify(cookie.Value); ok {
				pending, found = decodePending(payload)
			}
		}
		if !found || pending.expires <= time.Now().Unix() {
			fail("/", "expired")
			return
		}
		params := r.URL.Query()
		if !params.Has("state") || params.Get("state") != pending.state {
			fail(pending.returnTo, "expired")
			return
		}
		if params.Has("error") {
			reason := "failed"
			if params.Get("error") == "access_denied" {
				reason = "denied"
			}
			fail(pending.returnTo, reason)
			return
		}
		if !params.Has("code") {
			fail(pending.returnTo, "failed")
			return
		}
		identity, err := exchangeGoogleCode(r.Context(), state, params.Get("code"), pending)
		if err != nil {
			slog.Warn("Google sign-in failed", "err", err)
			fail(pending.returnTo, "failed")
			return
		}
		if !identity.EmailVerified {
			fail(pending.returnTo, "unverified")
			return
		}
		user, err := UpsertUser(r.Context(), state, identity)
		if err != nil {
			slog.Error("saving the user failed", "err", err)
			fail(pending.returnTo, "failed")
			return
		}
		w.Header().Add("Set-Cookie", app.BuildCookie(state.Keys.SessionCookie, state.Keys.SessionToken(user), app.SessionTTL, secure))
		http.Redirect(w, r, pending.returnTo, http.StatusSeeOther)
	}
}

// Identity is who signed in, from Google's ID token.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          *string
	Picture       *string
}

func exchangeGoogleCode(ctx context.Context, state *app.State, code string, pending pendingSignin) (*Identity, error) {
	if !state.Config.GoogleConfigured() {
		return nil, errors.New("Google sign-in isn't configured")
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {googleRedirectURI(state)},
		"client_id":     {state.Config.GoogleClientID},
		"client_secret": {state.Config.GoogleClientSecret},
		"code_verifier": {pending.verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ricrac")
	res, err := state.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("token endpoint answered %s: %s", res.Status, body)
	}
	var tokens struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil || tokens.IDToken == "" {
		return nil, fmt.Errorf("bad token response: %s", body)
	}
	return identityFromIDToken(tokens.IDToken, state.Config.GoogleClientID, pending.nonce)
}

// identityFromIDToken reads the ID token, which comes straight from Google's token endpoint over
// TLS, so per OIDC Core 3.1.3.7 its signature doesn't need checking; the claims still do.
func identityFromIDToken(idToken, clientID, nonce string) (*Identity, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil, errors.New("malformed ID token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, errors.New("malformed ID token")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var claims M
	if err := decoder.Decode(&claims); err != nil {
		return nil, errors.New("malformed ID token")
	}
	text := func(key string) string {
		s, _ := claims[key].(string)
		return s
	}
	if !slices.Contains(googleIssuers, text("iss")) {
		return nil, fmt.Errorf("unexpected issuer %s", text("iss"))
	}
	audienceOK := false
	switch aud := claims["aud"].(type) {
	case string:
		audienceOK = aud == clientID
	case []any:
		audienceOK = slices.Contains(aud, any(clientID))
	}
	if !audienceOK {
		return nil, errors.New("ID token is for another client")
	}
	expires, _ := claims["exp"].(json.Number)
	if exp, err := expires.Int64(); err != nil || exp < time.Now().Unix() {
		return nil, errors.New("ID token expired")
	}
	if n, ok := claims["nonce"].(string); !ok || n != nonce {
		return nil, errors.New("nonce mismatch")
	}
	if text("sub") == "" {
		return nil, errors.New("ID token has no subject")
	}
	if text("email") == "" {
		return nil, errors.New("ID token has no email")
	}
	optional := func(key string) *string {
		if s := text(key); s != "" {
			return &s
		}
		return nil
	}
	return &Identity{
		Subject:       text("sub"),
		Email:         strings.ToLower(text("email")),
		EmailVerified: claims["email_verified"] == true || claims["email_verified"] == "true",
		Name:          optional("name"),
		Picture:       optional("picture"),
	}, nil
}

// UpsertUser creates or refreshes the user who signed in.
func UpsertUser(ctx context.Context, state *app.State, identity *Identity) (uuid.UUID, error) {
	existing, found, err := app.Maybe[uuid.UUID](ctx, state.DB,
		"SELECT id FROM users WHERE google_subject = $1 OR email = $2 ORDER BY google_subject = $1 DESC LIMIT 1",
		identity.Subject, identity.Email)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		err := app.Exec(ctx, state.DB,
			"UPDATE users SET google_subject = $2, email = $3, avatar_url = coalesce($4, avatar_url), last_seen_at = now() WHERE id = $1",
			existing, identity.Subject, identity.Email, identity.Picture)
		return existing, err
	}
	name, _, _ := strings.Cut(identity.Email, "@")
	if identity.Name != nil {
		name = *identity.Name
	}
	return app.One[uuid.UUID](ctx, state.DB,
		"INSERT INTO users (google_subject, email, name, display_name, avatar_url) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		identity.Subject, identity.Email, name, app.DisplayName(name, identity.Email), identity.Picture)
}
