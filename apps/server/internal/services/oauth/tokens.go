package oauth_service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
)

// Credentials is how a client identified itself at the token or revocation endpoint.
type Credentials struct {
	ClientID string
	Secret   *string
	// Whether it named itself at all.
	Present bool
}

// CredentialsOf reads HTTP Basic credentials, or else client_id and client_secret in the form.
func CredentialsOf(authorization string, form url.Values) Credentials {
	if value, ok := strings.CutPrefix(authorization, "Basic "); ok {
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value)); err == nil {
			if id, secret, ok := strings.Cut(string(decoded), ":"); ok {
				secret = unescape(secret)
				return Credentials{ClientID: unescape(id), Secret: &secret, Present: true}
			}
		}
	}
	credentials := Credentials{ClientID: form.Get("client_id"), Present: form.Has("client_id")}
	if form.Has("client_secret") {
		secret := form.Get("client_secret")
		credentials.Secret = &secret
	}
	return credentials
}

func unescape(s string) string {
	if decoded, err := url.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}

var invalidClient = oauthError(http.StatusUnauthorized, "invalid_client", "Unknown client or bad credentials")

// authenticate is the client and whether it proved its secret. Clients with a secret may leave
// it out only when the grant is protected by PKCE.
func (self *OAuthService) authenticate(ctx context.Context, credentials Credentials) (string, bool, error) {
	if !credentials.Present {
		return "", false, invalidClient
	}
	found, err := self.repo.OAuth().Client(ctx, credentials.ClientID)
	if err != nil || found == nil {
		return "", false, invalidClient
	}
	if found.SecretHash == nil || credentials.Secret == nil {
		return credentials.ClientID, false, nil
	}
	if auth.SecretHash(*credentials.Secret) != *found.SecretHash {
		return "", false, invalidClient
	}
	return credentials.ClientID, true, nil
}

func pkceMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
}

// Token answers the token endpoint: an authorization code or a refresh token for new tokens.
// *Error answers are OAuth errors; other errors are failures of the server.
func (self *OAuthService) Token(ctx context.Context, credentials Credentials, form url.Values) (map[string]any, error) {
	clientID, authenticated, err := self.authenticate(ctx, credentials)
	if err != nil {
		return nil, err
	}
	switch form.Get("grant_type") {
	case "authorization_code":
		return self.exchangeCode(ctx, form, clientID, authenticated)
	case "refresh_token":
		return self.refresh(ctx, form, clientID)
	}
	return nil, oauthError(http.StatusBadRequest, "unsupported_grant_type", "Use authorization_code or refresh_token")
}

func invalidGrant(description string) *Error {
	return oauthError(http.StatusBadRequest, "invalid_grant", description)
}

func (self *OAuthService) exchangeCode(ctx context.Context, form url.Values, clientID string, authenticated bool) (map[string]any, error) {
	if !form.Has("code") {
		return nil, invalidGrant("Missing code")
	}
	// Single use: the code is deleted whether or not the exchange succeeds.
	code, err := self.repo.OAuth().TakeCode(ctx, auth.SecretHash(form.Get("code")))
	if err != nil {
		return nil, err
	}
	if code == nil {
		return nil, invalidGrant("Unknown or used code")
	}
	if code.ExpiresAt.Before(time.Now()) || code.ClientID != clientID {
		return nil, invalidGrant("Expired code or wrong client")
	}
	if form.Has("redirect_uri") && form.Get("redirect_uri") != code.RedirectURI {
		return nil, invalidGrant("redirect_uri does not match")
	}
	if code.CodeChallenge != nil {
		if !form.Has("code_verifier") || !pkceMatches(form.Get("code_verifier"), *code.CodeChallenge) {
			return nil, invalidGrant("PKCE verification failed")
		}
	} else if !authenticated {
		return nil, invalidGrant("This client must authenticate with its secret or use PKCE")
	}
	requested := code.Resource
	if form.Has("resource") {
		resource := form.Get("resource")
		requested = &resource
	}
	if !resourceMatches(requested, self.cfg.PublicURL) {
		return nil, oauthError(http.StatusBadRequest, "invalid_target", "Tokens are only issued for this server")
	}
	grant, err := self.repo.OAuth().CreateGrant(ctx, clientID, code.UserID, code.Workspaces, code.Scopes)
	if err != nil {
		return nil, err
	}
	return self.issueTokens(ctx, grant, code.Scopes)
}

func (self *OAuthService) refresh(ctx context.Context, form url.Values, clientID string) (map[string]any, error) {
	invalid := invalidGrant("Invalid or expired refresh token")
	if !form.Has("refresh_token") {
		return nil, invalid
	}
	// Rotation: the old refresh token stops working as soon as it is used.
	token, err := self.repo.OAuth().TakeRefreshToken(ctx, auth.SecretHash(form.Get("refresh_token")))
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, invalid
	}
	grant, err := self.repo.OAuth().Grant(ctx, token.Grant)
	if err != nil {
		return nil, err
	}
	if grant == nil || token.ExpiresAt.Before(time.Now()) || grant.ClientID != clientID {
		return nil, invalid
	}
	return self.issueTokens(ctx, token.Grant, grant.Scopes)
}

func (self *OAuthService) issueTokens(ctx context.Context, grant uuid.UUID, scopes []string) (map[string]any, error) {
	access := auth.RandomSecret("rr_at_")
	refresh := auth.RandomSecret("rr_rt_")
	for _, t := range []struct {
		token, kind string
		ttl         int
	}{{access, "access", accessTTL}, {refresh, "refresh", refreshTTL}} {
		if err := self.repo.OAuth().CreateToken(ctx, auth.SecretHash(t.token), t.kind, grant, t.ttl); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    accessTTL,
		"refresh_token": refresh,
		"scope":         strings.Join(scopes, " "),
	}, nil
}

// Revoke follows RFC 7009: revoking either token of a grant ends the whole grant. Unknown
// tokens are fine.
func (self *OAuthService) Revoke(ctx context.Context, credentials Credentials, form url.Values) error {
	clientID, _, err := self.authenticate(ctx, credentials)
	if err != nil {
		return err
	}
	if !form.Has("token") {
		return oauthError(http.StatusBadRequest, "invalid_request", "Missing token")
	}
	return self.repo.OAuth().RevokeGrantOf(ctx, auth.SecretHash(form.Get("token")), clientID)
}

// Cleanup forgets old registration attempts and drops expired codes and tokens, grants
// without live tokens, and registrations never used.
func (self *OAuthService) Cleanup(ctx context.Context) error {
	self.limiter.prune()
	return self.repo.OAuth().Cleanup(ctx)
}
