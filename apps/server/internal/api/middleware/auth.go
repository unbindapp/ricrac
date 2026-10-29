package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
)

// Authenticator is the bearer tokens it accepts.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*auth.Caller, error)
}

type Auth struct {
	cfg    config.Config
	keys   auth.Keys
	tokens Authenticator
}

func NewAuth(cfg config.Config, keys auth.Keys, tokens Authenticator) *Auth {
	return &Auth{cfg: cfg, keys: keys, tokens: tokens}
}

type callerKey struct{}

func WithCaller(ctx context.Context, caller *auth.Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, caller)
}

// CallerFrom is the caller RequireBearer let in.
func CallerFrom(ctx context.Context) *auth.Caller {
	caller, _ := ctx.Value(callerKey{}).(*auth.Caller)
	return caller
}

// Bearer is the token of the Authorization header.
func Bearer(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if token, ok := strings.CutPrefix(value, "Bearer "); ok {
		value = token
	} else if token, ok := strings.CutPrefix(value, "bearer "); ok {
		value = token
	}
	return strings.TrimSpace(value)
}

// BearerCaller is the caller of a request with an OAuth access token or an API key, nil without
// a valid one.
func (a *Auth) BearerCaller(r *http.Request) *auth.Caller {
	caller, err := a.tokens.Authenticate(r.Context(), Bearer(r))
	if err != nil {
		return nil
	}
	return caller
}

// RESTCaller is the caller of a /v1 request (a bearer token, or the dashboard's session cookie)
// and the workspace named in x-ricrac-workspace, used when the arguments don't name one. It
// writes the error answer when there's none.
func (a *Auth) RESTCaller(w http.ResponseWriter, r *http.Request) (*auth.Caller, string, bool) {
	workspace := r.Header.Get("X-Ricrac-Workspace")
	if caller := a.BearerCaller(r); caller != nil {
		return caller, workspace, true
	}
	user, ok := a.keys.SessionUser(r)
	if r.Header.Get("Authorization") != "" || !ok {
		a.Unauthorized(w, r)
		return nil, "", false
	}
	if r.Method != http.MethodGet && r.Header.Get("X-Ricrac") == "" {
		errdefs.Write(w, errdefs.Forbidden("Missing x-ricrac header"))
		return nil, "", false
	}
	return auth.SessionCaller(user), workspace, true
}

// SignedOut says the request has no credentials: no Authorization header and no valid session.
func (a *Auth) SignedOut(r *http.Request) bool {
	if r.Header.Get("Authorization") != "" {
		return false
	}
	_, ok := a.keys.SessionUser(r)
	return !ok
}

// RequireBearer rejects requests without a valid token, pointing clients at the OAuth metadata.
func (a *Auth) RequireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := a.BearerCaller(r)
		if caller == nil {
			a.Unauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithCaller(r.Context(), caller)))
	})
}

// ResourceMetadataURL is where clients learn how to get a token for MCP.
func ResourceMetadataURL(base string) string {
	return base + "/.well-known/oauth-protected-resource/mcp"
}

func (a *Auth) Unauthorized(w http.ResponseWriter, r *http.Request) {
	sentToken := r.Header.Get("Authorization") != ""
	challenge := fmt.Sprintf("Bearer resource_metadata=%q", ResourceMetadataURL(a.cfg.PublicURL))
	message := "Authenticate with an OAuth access token or an API key: Authorization: Bearer <token>"
	if sentToken {
		challenge += `, error="invalid_token"`
		message = "The token is invalid or expired"
	}
	w.Header().Set("WWW-Authenticate", challenge)
	errdefs.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": message})
}
