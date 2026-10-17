// Package signin_service is signing in with Google: the round trip through Google's OAuth, the
// ID token's claims and the user it signs in.
package signin_service

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

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/repositories/repositories"
)

const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	// SigninTTL is how long the round trip through Google may take, in seconds.
	SigninTTL = 10 * 60
)

var googleIssuers = []string{"https://accounts.google.com", "accounts.google.com"}

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i SigninServiceInterface -p signin_service -s SigninService -o signin_service_iface.go
type SigninService struct {
	cfg  config.Config
	keys auth.Keys
	repo repositories.RepositoriesInterface
	http *http.Client
}

func NewSigninService(cfg config.Config, keys auth.Keys, repo repositories.RepositoriesInterface) *SigninService {
	return &SigninService{cfg: cfg, keys: keys, repo: repo, http: &http.Client{Timeout: 15 * time.Second}}
}

func (self *SigninService) redirectURI() string {
	return self.cfg.PublicURL + "/api/auth/callback/google"
}

// SafeReturnTo only allows paths on this site, so sign-in can't be used as an open redirect.
func SafeReturnTo(returnTo string) string {
	if strings.HasPrefix(returnTo, "/") && !strings.HasPrefix(returnTo, "//") && !strings.HasPrefix(returnTo, "/\\") {
		return returnTo
	}
	return "/"
}

func WithSigninError(returnTo, reason string) string {
	separator := "?"
	if strings.Contains(returnTo, "?") {
		separator = "&"
	}
	return returnTo + separator + "signin_error=" + reason
}

// pending is the sign-in round trip, kept in a signed cookie.
type pending struct {
	expires  int64
	state    string
	nonce    string
	verifier string
	returnTo string
}

func (p pending) encode() string {
	return fmt.Sprintf("signin:%d:%s:%s:%s:%s", p.expires, p.state, p.nonce, p.verifier, p.returnTo)
}

func decodePending(payload string) (pending, bool) {
	rest, ok := strings.CutPrefix(payload, "signin:")
	if !ok {
		return pending{}, false
	}
	parts := strings.SplitN(rest, ":", 5)
	if len(parts) != 5 {
		return pending{}, false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return pending{}, false
	}
	return pending{expires, parts[1], parts[2], parts[3], parts[4]}, true
}

// Start is where to send the browser to sign in with Google, and the signed round trip to keep
// in a cookie until it comes back.
func (self *SigninService) Start(returnTo string) (string, string) {
	round := pending{
		expires:  time.Now().Unix() + SigninTTL,
		state:    auth.RandomSecret(""),
		nonce:    auth.RandomSecret(""),
		verifier: auth.RandomSecret(""),
		returnTo: returnTo,
	}
	challenge := sha256.Sum256([]byte(round.verifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {self.cfg.GoogleClientID},
		"redirect_uri":          {self.redirectURI()},
		"scope":                 {"openid email profile"},
		"state":                 {round.state},
		"nonce":                 {round.nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	return googleAuthorizeURL + "?" + query.Encode(), self.keys.Sign(round.encode())
}

// Finish completes the round trip (the signed cookie, empty without one) with what Google
// answered. It returns where to go next and who signed in, or why it failed (expired, denied,
// failed or unverified).
func (self *SigninService) Finish(ctx context.Context, cookie string, params url.Values) (string, uuid.UUID, string) {
	var round pending
	found := false
	if payload, ok := self.keys.Verify(cookie); ok {
		round, found = decodePending(payload)
	}
	if !found || round.expires <= time.Now().Unix() {
		return "/", uuid.Nil, "expired"
	}
	if !params.Has("state") || params.Get("state") != round.state {
		return round.returnTo, uuid.Nil, "expired"
	}
	if params.Has("error") {
		if params.Get("error") == "access_denied" {
			return round.returnTo, uuid.Nil, "denied"
		}
		return round.returnTo, uuid.Nil, "failed"
	}
	if !params.Has("code") {
		return round.returnTo, uuid.Nil, "failed"
	}
	identity, err := self.exchangeCode(ctx, params.Get("code"), round)
	if err != nil {
		slog.Warn("Google sign-in failed", "err", err)
		return round.returnTo, uuid.Nil, "failed"
	}
	if !identity.EmailVerified {
		return round.returnTo, uuid.Nil, "unverified"
	}
	user, err := self.UpsertUser(ctx, identity)
	if err != nil {
		slog.Error("saving the user failed", "err", err)
		return round.returnTo, uuid.Nil, "failed"
	}
	return round.returnTo, user, ""
}

// Identity is who signed in, from Google's ID token.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          *string
	Picture       *string
}

func (self *SigninService) exchangeCode(ctx context.Context, code string, round pending) (*Identity, error) {
	if !self.cfg.GoogleConfigured() {
		return nil, errors.New("Google sign-in isn't configured")
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {self.redirectURI()},
		"client_id":     {self.cfg.GoogleClientID},
		"client_secret": {self.cfg.GoogleClientSecret},
		"code_verifier": {round.verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ricrac")
	res, err := self.http.Do(req)
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
	return IdentityFromIDToken(tokens.IDToken, self.cfg.GoogleClientID, round.nonce)
}

// IdentityFromIDToken reads the ID token, which comes straight from Google's token endpoint over
// TLS, so per OIDC Core 3.1.3.7 its signature doesn't need checking; the claims still do.
func IdentityFromIDToken(idToken, clientID, nonce string) (*Identity, error) {
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
	var claims map[string]any
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
func (self *SigninService) UpsertUser(ctx context.Context, identity *Identity) (uuid.UUID, error) {
	name, _, _ := strings.Cut(identity.Email, "@")
	if identity.Name != nil {
		name = *identity.Name
	}
	return self.repo.Account().UpsertGoogleUser(ctx, identity.Subject, identity.Email, name, auth.DisplayName(name, identity.Email), identity.Picture)
}
