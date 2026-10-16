package server

import (
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
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/app"
)

const (
	codeTTL       = 10 * 60
	accessTTL     = 24 * 3600
	refreshTTL    = 90 * 24 * 3600
	cimdMaxBytes  = 64 * 1024
	cimdCacheTTL  = time.Hour
	APIKeyPrefix  = "rr_key_"
	defaultClient = "MCP client"
)

func resourceMetadataURL(base string) string {
	return base + "/.well-known/oauth-protected-resource/mcp"
}

func protectedResource(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app.WriteJSON(w, http.StatusOK, M{
			"resource":                 state.Config.MCPURL(),
			"authorization_servers":    []string{state.Config.PublicURL},
			"scopes_supported":         app.AllScopes,
			"bearer_methods_supported": []string{"header"},
			"resource_name":            "Ricrac",
		})
	}
}

func authorizationServer(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := state.Config.PublicURL
		app.WriteJSON(w, http.StatusOK, M{
			"issuer":                                         base,
			"authorization_endpoint":                         base + "/oauth/authorize",
			"token_endpoint":                                 base + "/oauth/token",
			"registration_endpoint":                          base + "/oauth/register",
			"revocation_endpoint":                            base + "/oauth/revoke",
			"scopes_supported":                               app.AllScopes,
			"response_types_supported":                       []string{"code"},
			"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":               []string{"S256"},
			"token_endpoint_auth_methods_supported":          []string{"none", "client_secret_post", "client_secret_basic"},
			"client_id_metadata_document_supported":          true,
			"authorization_response_iss_parameter_supported": true,
		})
	}
}

func oauthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	app.WriteJSON(w, status, M{"error": code, "error_description": description})
}

func stripPort(authority string) string {
	host, port, found := cutLast(authority, ":")
	if found && port != "" && strings.Trim(port, "0123456789") == "" {
		return host
	}
	return authority
}

func cutLast(s, sep string) (string, string, bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// loopbackParts is the host and path of an http URI on a loopback host.
func loopbackParts(uri string) (string, string, bool) {
	rest, ok := strings.CutPrefix(uri, "http://")
	if !ok {
		return "", "", false
	}
	authority, path, _ := strings.Cut(rest, "/")
	host := stripPort(authority)
	if host != "localhost" && host != "127.0.0.1" && host != "[::1]" {
		return "", "", false
	}
	return host, path, true
}

// IsAllowedRedirect allows https anywhere, http only on loopback (native clients such as Claude
// Code), or an app's own URI scheme. PKCE or the client secret protects the code in every case.
func IsAllowedRedirect(uri string) bool {
	if len(uri) > 2000 || strings.Contains(uri, "#") {
		return false
	}
	scheme, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return false
	}
	switch scheme {
	case "https":
		return rest != ""
	case "http":
		_, _, loopback := loopbackParts(uri)
		return loopback
	case "javascript", "data", "file", "vbscript", "":
		return false
	}
	return strings.Trim(scheme, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+.-") == ""
}

// redirectMatches is an exact match, except that loopback clients may use any port (RFC 8252
// section 7.3).
func redirectMatches(registered, requested string) bool {
	if registered == requested {
		return true
	}
	host1, path1, ok1 := loopbackParts(registered)
	host2, path2, ok2 := loopbackParts(requested)
	return ok1 && ok2 && host1 == host2 && path1 == path2
}

func register(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if state.Limiter.Hit("register:" + app.ClientIP(r)) {
			oauthError(w, http.StatusTooManyRequests, "invalid_request", "Too many registrations, try again later")
			return
		}
		var req struct {
			RedirectURIs            []string `json:"redirect_uris"`
			ClientName              *string  `json:"client_name"`
			TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "The body must be a JSON object")
			return
		}
		if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 10 {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "Provide 1 to 10 redirect_uris")
			return
		}
		for _, uri := range req.RedirectURIs {
			if !IsAllowedRedirect(uri) {
				oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "Redirect URI not allowed: "+uri)
				return
			}
		}
		method := "none"
		if req.TokenEndpointAuthMethod != nil {
			method = *req.TokenEndpointAuthMethod
		}
		var secret *string
		switch method {
		case "none":
		case "client_secret_post", "client_secret_basic":
			s := app.RandomSecret("rr_cs_")
			secret = &s
		default:
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "Unsupported auth method "+method)
			return
		}
		name := defaultClient
		if req.ClientName != nil {
			name = *req.ClientName
		}
		name = truncate(strings.TrimSpace(name), 100)
		id := "dcr_" + app.RandomSecret("")[:24]
		var secretHash *string
		if secret != nil {
			h := app.SecretHash(*secret)
			secretHash = &h
		}
		err := app.Exec(r.Context(), state.DB,
			"INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris) VALUES ($1, $2, $3, $4)",
			id, secretHash, name, req.RedirectURIs)
		if err != nil {
			slog.Error("client registration failed", "err", err)
			oauthError(w, http.StatusInternalServerError, "server_error", "Registration failed")
			return
		}
		body := M{
			"client_id":                  id,
			"client_id_issued_at":        time.Now().Unix(),
			"client_name":                name,
			"redirect_uris":              req.RedirectURIs,
			"grant_types":                []string{"authorization_code", "refresh_token"},
			"response_types":             []string{"code"},
			"token_endpoint_auth_method": method,
		}
		if secret != nil {
			body["client_secret"] = *secret
			body["client_secret_expires_at"] = 0
		}
		app.WriteJSON(w, http.StatusCreated, body)
	}
}

func truncate(s string, chars int) string {
	runes := []rune(s)
	if len(runes) <= chars {
		return s
	}
	return string(runes[:chars])
}

// isClientIDURL is an https URL with a path on a named public host, e.g. Claude's published
// client identity.
func isClientIDURL(id string) bool {
	rest, ok := strings.CutPrefix(id, "https://")
	if !ok {
		return false
	}
	authority, path, ok := strings.Cut(rest, "/")
	if !ok {
		return false
	}
	host := stripPort(authority)
	return path != "" && !strings.ContainsAny(id, "#?") && strings.Contains(host, ".") &&
		!strings.HasPrefix(host, "[") && strings.Trim(host, "0123456789.") != ""
}

type clientMetadata struct {
	ClientID                          string   `json:"client_id"`
	ClientName                        *string  `json:"client_name"`
	RedirectURIs                      []string `json:"redirect_uris"`
	TokenEndpointAuthMethod           *string  `json:"token_endpoint_auth_method"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// supportsPublicAuth: ChatGPT prefers private_key_jwt but lists none as a fallback, which is
// what we offer.
func (m clientMetadata) supportsPublicAuth() bool {
	return m.TokenEndpointAuthMethod == nil || *m.TokenEndpointAuthMethod == "none" ||
		slices.Contains(m.TokenEndpointAuthMethodsSupported, "none")
}

var metadataClient = &http.Client{
	Timeout:       5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func fetchClientMetadata(ctx context.Context, address string) (clientMetadata, error) {
	var doc clientMetadata
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return doc, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ricrac")
	res, err := metadataClient.Do(req)
	if err != nil {
		return doc, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return doc, fmt.Errorf("HTTP %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, cimdMaxBytes+1))
	if err != nil {
		return doc, err
	}
	if len(body) > cimdMaxBytes {
		return doc, errors.New("document too large")
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return doc, err
	}
	if doc.ClientName == nil {
		return doc, errors.New("missing field client_name")
	}
	if doc.ClientID != address {
		return doc, errors.New("client_id does not match the document URL")
	}
	if !doc.supportsPublicAuth() {
		return doc, errors.New("only public clients are supported")
	}
	if len(doc.RedirectURIs) == 0 || len(doc.RedirectURIs) > 10 {
		return doc, errors.New("provide 1 to 10 redirect_uris")
	}
	for _, uri := range doc.RedirectURIs {
		if !IsAllowedRedirect(uri) {
			return doc, fmt.Errorf("redirect URI not allowed: %s", uri)
		}
	}
	return doc, nil
}

// cimdClient refreshes the document at most once an hour and keeps the last copy when the fetch
// fails.
func cimdClient(ctx context.Context, state *app.State, address string) (*client, error) {
	cached, err := loadClient(ctx, state, address)
	if err != nil {
		return nil, err
	}
	if cached != nil && cached.FetchedAt != nil && time.Since(*cached.FetchedAt) < cimdCacheTTL {
		return cached, nil
	}
	doc, err := fetchClientMetadata(ctx, address)
	if err != nil {
		slog.Warn("fetching client metadata failed", "url", address, "err", err)
		if cached == nil {
			return nil, app.BadRequest("Could not read the application's identity document.")
		}
		return cached, nil
	}
	name := truncate(strings.TrimSpace(*doc.ClientName), 100)
	if name == "" {
		name = defaultClient
	}
	err = app.Exec(ctx, state.DB,
		`INSERT INTO oauth_clients (id, name, redirect_uris, fetched_at) VALUES ($1, $2, $3, now())
		 ON CONFLICT (id) DO UPDATE SET name = excluded.name, redirect_uris = excluded.redirect_uris, fetched_at = now()`,
		address, name, doc.RedirectURIs)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &client{Name: name, RedirectURIs: doc.RedirectURIs, FetchedAt: &now}, nil
}

type client struct {
	Name         string
	RedirectURIs []string
	SecretHash   *string
	OwnerID      *uuid.UUID
	FetchedAt    *time.Time
}

func loadClient(ctx context.Context, state *app.State, id string) (*client, error) {
	found, ok, err := app.Row[client](ctx, state.DB,
		"SELECT name, redirect_uris, secret_hash, owner_id, fetched_at FROM oauth_clients WHERE id = $1", id)
	if err != nil || !ok {
		return nil, err
	}
	return &found, nil
}

func resourceMatches(resource *string, base string) bool {
	if resource == nil {
		return true
	}
	r := strings.TrimRight(*resource, "/")
	return r == base || r == base+"/mcp" || r == base+"/v1"
}

// OAuthRequest is the query of /oauth/authorize, passed on by the consent page.
type OAuthRequest struct {
	ClientID            string  `json:"client_id"`
	RedirectURI         string  `json:"redirect_uri"`
	ResponseType        string  `json:"response_type"`
	CodeChallenge       *string `json:"code_challenge"`
	CodeChallengeMethod *string `json:"code_challenge_method"`
	State               *string `json:"state"`
	Scope               *string `json:"scope"`
	Resource            *string `json:"resource"`
}

type OAuthClientInfo struct {
	ClientName   string   `json:"client_name"`
	RedirectHost string   `json:"redirect_host"`
	Scopes       []string `json:"scopes"`
	// Registered in a dashboard by someone, rather than by the client itself.
	Verified bool `json:"verified"`
}

func requestedScopes(scope *string) ([]string, error) {
	if scope == nil || strings.TrimSpace(*scope) == "" {
		return slices.Clone(app.AllScopes), nil
	}
	var scopes []string
	for _, part := range strings.FieldsFunc(*scope, func(r rune) bool { return r == ' ' || r == ',' }) {
		if !slices.Contains(app.AllScopes, part) {
			return nil, app.BadRequest("Unknown scope %q. Scopes are read, write and admin.", part)
		}
		if !slices.Contains(scopes, part) {
			scopes = append(scopes, part)
		}
	}
	if !slices.Contains(scopes, "read") {
		scopes = append([]string{"read"}, scopes...)
	}
	return scopes, nil
}

// checkRequest checks an authorization request before the consent screen. Its errors are shown
// on the page, never sent to an unverified redirect URI.
func checkRequest(ctx context.Context, state *app.State, req *OAuthRequest) (*OAuthClientInfo, error) {
	var found *client
	var err error
	if isClientIDURL(req.ClientID) {
		found, err = cimdClient(ctx, state, req.ClientID)
	} else {
		found, err = loadClient(ctx, state, req.ClientID)
		if err == nil && found == nil {
			err = app.BadRequest("Unknown application. Try connecting again.")
		}
	}
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(found.RedirectURIs, func(u string) bool { return redirectMatches(u, req.RedirectURI) }) {
		return nil, app.BadRequest("The application's redirect address doesn't match its registration.")
	}
	if req.ResponseType != "code" {
		return nil, app.BadRequest("Unsupported response_type.")
	}
	pkce := req.CodeChallenge != nil && req.CodeChallengeMethod != nil && *req.CodeChallengeMethod == "S256" && len(*req.CodeChallenge) >= 43
	confidential := req.CodeChallenge == nil && req.CodeChallengeMethod == nil && found.SecretHash != nil && found.OwnerID != nil
	if !pkce && !confidential {
		return nil, app.BadRequest("The application must use PKCE (S256).")
	}
	if !resourceMatches(req.Resource, state.Config.PublicURL) {
		return nil, app.BadRequest("The application asked for access to a different server.")
	}
	scopes, err := requestedScopes(req.Scope)
	if err != nil {
		return nil, err
	}
	redirectHost := ""
	if _, rest, ok := strings.Cut(req.RedirectURI, "://"); ok {
		redirectHost, _, _ = strings.Cut(rest, "/")
		redirectHost, _, _ = strings.Cut(redirectHost, "?")
	}
	return &OAuthClientInfo{ClientName: found.Name, RedirectHost: redirectHost, Scopes: scopes, Verified: found.OwnerID != nil}, nil
}

// queryEscape keeps the RFC 3986 unreserved characters as they are.
func queryEscape(value string) string {
	var b strings.Builder
	for _, c := range []byte(value) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func withParams(uri string, params [][2]string) string {
	query := make([]string, len(params))
	for i, p := range params {
		query[i] = p[0] + "=" + queryEscape(p[1])
	}
	separator := "?"
	if strings.Contains(uri, "?") {
		separator = "&"
	}
	return uri + separator + strings.Join(query, "&")
}

// Decide records the user's decision and returns where to send the browser. Nil workspaces grants
// every workspace of the user, including ones they join later.
func Decide(ctx context.Context, state *app.State, user uuid.UUID, workspaces []uuid.UUID, req *OAuthRequest, allow bool) (string, error) {
	info, err := checkRequest(ctx, state, req)
	if err != nil {
		return "", err
	}
	params := [][2]string{{"iss", state.Config.PublicURL}}
	if req.State != nil && *req.State != "" {
		params = append(params, [2]string{"state", *req.State})
	}
	if !allow {
		params = append(params, [2]string{"error", "access_denied"})
		return withParams(req.RedirectURI, params), nil
	}
	if err := checkWorkspaces(ctx, state, user, workspaces); err != nil {
		return "", err
	}
	code := app.RandomSecret("rr_ac_")
	err = app.Exec(ctx, state.DB,
		`INSERT INTO oauth_codes (code_hash, client_id, user_id, workspace_ids, scopes, redirect_uri, code_challenge, resource, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))`,
		app.SecretHash(code), req.ClientID, user, workspaces, info.Scopes, req.RedirectURI, req.CodeChallenge, req.Resource, float64(codeTTL))
	if err != nil {
		return "", err
	}
	return withParams(req.RedirectURI, append([][2]string{{"code", code}}, params...)), nil
}

// checkWorkspaces checks that a list of workspaces to grant is non-empty and only holds the
// user's. Nil is all of them.
func checkWorkspaces(ctx context.Context, state *app.State, user uuid.UUID, workspaces []uuid.UUID) error {
	if workspaces == nil {
		return nil
	}
	if len(workspaces) == 0 {
		return app.BadRequest("Pick at least one workspace.")
	}
	members, err := app.One[bool](ctx, state.DB,
		`SELECT NOT EXISTS (SELECT unnest($2::uuid[])
		 EXCEPT SELECT workspace_id FROM workspace_members WHERE user_id = $1 AND active)`, user, workspaces)
	if err != nil {
		return err
	}
	if !members {
		return app.BadRequest("You aren't a member of one of those workspaces.")
	}
	return nil
}

func clientCredentials(r *http.Request) (string, *string, bool) {
	if value, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Basic "); ok {
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value)); err == nil {
			if id, secret, ok := strings.Cut(string(decoded), ":"); ok {
				secret = unescape(secret)
				return unescape(id), &secret, true
			}
		}
	}
	id := r.PostForm.Get("client_id")
	var secret *string
	if r.PostForm.Has("client_secret") {
		s := r.PostForm.Get("client_secret")
		secret = &s
	}
	return id, secret, r.PostForm.Has("client_id")
}

func unescape(s string) string {
	if decoded, err := url.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}

// authenticateClient is the client and whether it proved its secret. Clients with a secret may
// leave it out only when the grant is protected by PKCE. It writes the error when there's none.
func authenticateClient(state *app.State, w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	id, secret, ok := clientCredentials(r)
	invalid := func() (string, bool, bool) {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "Unknown client or bad credentials")
		return "", false, false
	}
	if !ok {
		return invalid()
	}
	found, err := loadClient(r.Context(), state, id)
	if err != nil || found == nil {
		return invalid()
	}
	if found.SecretHash != nil && secret != nil {
		if app.SecretHash(*secret) != *found.SecretHash {
			return invalid()
		}
		return id, true, true
	}
	return id, false, true
}

func issueTokens(ctx context.Context, state *app.State, w http.ResponseWriter, grant uuid.UUID, scopes []string) error {
	access := app.RandomSecret("rr_at_")
	refresh := app.RandomSecret("rr_rt_")
	for _, t := range []struct {
		token, kind string
		ttl         int
	}{{access, "access", accessTTL}, {refresh, "refresh", refreshTTL}} {
		err := app.Exec(ctx, state.DB,
			"INSERT INTO oauth_tokens (token_hash, kind, grant_id, expires_at) VALUES ($1, $2, $3, now() + make_interval(secs => $4))",
			app.SecretHash(t.token), t.kind, grant, float64(t.ttl))
		if err != nil {
			return err
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	app.WriteJSON(w, http.StatusOK, M{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    accessTTL,
		"refresh_token": refresh,
		"scope":         strings.Join(scopes, " "),
	})
	return nil
}

func pkceMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
}

func token(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		clientID, authenticated, ok := authenticateClient(state, w, r)
		if !ok {
			return
		}
		var err error
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			err = exchangeCode(state, w, r, clientID, authenticated)
		case "refresh_token":
			err = refreshTokens(state, w, r, clientID)
		default:
			oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "Use authorization_code or refresh_token")
			return
		}
		if err != nil {
			slog.Error("token endpoint failed", "err", err)
			oauthError(w, http.StatusInternalServerError, "server_error", "Token request failed")
		}
	}
}

type codeRow struct {
	ClientID      string
	UserID        uuid.UUID
	Workspaces    []uuid.UUID
	Scopes        []string
	RedirectURI   string
	CodeChallenge *string
	Resource      *string
	ExpiresAt     time.Time
}

func exchangeCode(state *app.State, w http.ResponseWriter, r *http.Request, clientID string, authenticated bool) error {
	ctx := r.Context()
	invalid := func(message string) error {
		oauthError(w, http.StatusBadRequest, "invalid_grant", message)
		return nil
	}
	form := r.PostForm
	if !form.Has("code") {
		return invalid("Missing code")
	}
	// Single use: the code is deleted whether or not the exchange succeeds.
	row, found, err := app.Row[codeRow](ctx, state.DB,
		`DELETE FROM oauth_codes WHERE code_hash = $1
		 RETURNING client_id, user_id, workspace_ids, scopes, redirect_uri, code_challenge, resource, expires_at`,
		app.SecretHash(form.Get("code")))
	if err != nil {
		return err
	}
	if !found {
		return invalid("Unknown or used code")
	}
	if row.ExpiresAt.Before(time.Now()) || row.ClientID != clientID {
		return invalid("Expired code or wrong client")
	}
	if form.Has("redirect_uri") && form.Get("redirect_uri") != row.RedirectURI {
		return invalid("redirect_uri does not match")
	}
	if row.CodeChallenge != nil {
		if !form.Has("code_verifier") || !pkceMatches(form.Get("code_verifier"), *row.CodeChallenge) {
			return invalid("PKCE verification failed")
		}
	} else if !authenticated {
		return invalid("This client must authenticate with its secret or use PKCE")
	}
	requested := row.Resource
	if form.Has("resource") {
		resource := form.Get("resource")
		requested = &resource
	}
	if !resourceMatches(requested, state.Config.PublicURL) {
		oauthError(w, http.StatusBadRequest, "invalid_target", "Tokens are only issued for this server")
		return nil
	}
	grant, err := app.One[uuid.UUID](ctx, state.DB,
		"INSERT INTO oauth_grants (client_id, user_id, workspace_ids, scopes) VALUES ($1, $2, $3, $4) RETURNING id",
		clientID, row.UserID, row.Workspaces, row.Scopes)
	if err != nil {
		return err
	}
	return issueTokens(ctx, state, w, grant, row.Scopes)
}

func refreshTokens(state *app.State, w http.ResponseWriter, r *http.Request, clientID string) error {
	ctx := r.Context()
	invalid := func() error {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Invalid or expired refresh token")
		return nil
	}
	if !r.PostForm.Has("refresh_token") {
		return invalid()
	}
	// Rotation: the old refresh token stops working as soon as it is used.
	token, found, err := app.Row[struct {
		Grant     uuid.UUID
		ExpiresAt time.Time
	}](ctx, state.DB, "DELETE FROM oauth_tokens WHERE token_hash = $1 AND kind = 'refresh' RETURNING grant_id, expires_at",
		app.SecretHash(r.PostForm.Get("refresh_token")))
	if err != nil {
		return err
	}
	if !found {
		return invalid()
	}
	grant, found, err := app.Row[struct {
		ClientID string
		Scopes   []string
	}](ctx, state.DB, "SELECT client_id, scopes FROM oauth_grants WHERE id = $1", token.Grant)
	if err != nil {
		return err
	}
	if !found || token.ExpiresAt.Before(time.Now()) || grant.ClientID != clientID {
		return invalid()
	}
	return issueTokens(ctx, state, w, token.Grant, grant.Scopes)
}

// revoke follows RFC 7009: revoking either token of a grant ends the whole grant. Unknown tokens
// are fine.
func revoke(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		clientID, _, ok := authenticateClient(state, w, r)
		if !ok {
			return
		}
		if !r.PostForm.Has("token") {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Missing token")
			return
		}
		err := app.Exec(r.Context(), state.DB,
			"DELETE FROM oauth_grants g USING oauth_tokens t WHERE t.token_hash = $1 AND t.grant_id = g.id AND g.client_id = $2",
			app.SecretHash(r.PostForm.Get("token")), clientID)
		if err != nil {
			slog.Error("revoking failed", "err", err)
			oauthError(w, http.StatusInternalServerError, "server_error", "Revocation failed")
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// bearer is the token from Authorization: Bearer <token>, or a bare API key.
func bearer(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if token, ok := strings.CutPrefix(value, "Bearer "); ok {
		value = token
	} else if token, ok := strings.CutPrefix(value, "bearer "); ok {
		value = token
	}
	return strings.TrimSpace(value)
}

// bearerCaller is the caller of a request with an OAuth access token or an API key, nil without
// a valid one.
func bearerCaller(state *app.State, r *http.Request) *app.Caller {
	token := bearer(r)
	if token == "" {
		return nil
	}
	ctx := r.Context()
	if strings.HasPrefix(token, APIKeyPrefix) {
		key, found, err := app.Row[struct {
			ID         uuid.UUID
			User       uuid.UUID
			Workspaces []uuid.UUID
			Scopes     []string
		}](ctx, state.DB, "SELECT id, user_id, workspace_ids, scopes FROM api_keys WHERE key_hash = $1", app.SecretHash(token))
		if err != nil || !found {
			return nil
		}
		_ = app.Exec(ctx, state.DB,
			"UPDATE api_keys SET last_used_at = now() WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')", key.ID)
		return &app.Caller{Context: ctx, State: state, User: key.User, Scopes: app.ParseScopes(key.Scopes), Workspaces: key.Workspaces}
	}
	grant, found, err := app.Row[struct {
		ID         uuid.UUID
		User       uuid.UUID
		Workspaces []uuid.UUID
		Scopes     []string
		ExpiresAt  time.Time
	}](ctx, state.DB,
		`SELECT g.id, g.user_id, g.workspace_ids, g.scopes, t.expires_at FROM oauth_tokens t
		 JOIN oauth_grants g ON g.id = t.grant_id WHERE t.token_hash = $1 AND t.kind = 'access'`, app.SecretHash(token))
	if err != nil || !found || grant.ExpiresAt.Before(time.Now()) {
		return nil
	}
	_ = app.Exec(ctx, state.DB,
		"UPDATE oauth_grants SET last_used_at = now() WHERE id = $1 AND last_used_at < now() - interval '1 minute'", grant.ID)
	return &app.Caller{Context: ctx, State: state, User: grant.User, Scopes: app.ParseScopes(grant.Scopes), Workspaces: grant.Workspaces}
}

// requireBearer rejects /mcp requests without a valid token, pointing clients at the OAuth
// metadata.
func requireBearer(state *app.State, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := bearerCaller(state, r)
		if caller == nil {
			unauthorized(state, w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(withCaller(r.Context(), caller)))
	})
}

func unauthorized(state *app.State, w http.ResponseWriter, r *http.Request) {
	sentToken := r.Header.Get("Authorization") != ""
	challenge := fmt.Sprintf("Bearer resource_metadata=%q", resourceMetadataURL(state.Config.PublicURL))
	message := "Authenticate with an OAuth access token or an API key: Authorization: Bearer <token>"
	if sentToken {
		challenge += `, error="invalid_token"`
		message = "The token is invalid or expired"
	}
	w.Header().Set("WWW-Authenticate", challenge)
	app.WriteJSON(w, http.StatusUnauthorized, M{"error": message})
}

// cleanupOAuth drops expired codes and tokens, grants without live tokens, and registrations
// never used.
func cleanupOAuth(ctx context.Context, state *app.State) error {
	for _, statement := range []string{
		"DELETE FROM oauth_codes WHERE expires_at < now()",
		"DELETE FROM oauth_tokens WHERE expires_at < now()",
		"DELETE FROM oauth_grants g WHERE NOT EXISTS (SELECT 1 FROM oauth_tokens t WHERE t.grant_id = g.id)",
		`DELETE FROM oauth_clients c WHERE c.owner_id IS NULL AND c.created_at < now() - interval '1 day'
		 AND NOT EXISTS (SELECT 1 FROM oauth_grants g WHERE g.client_id = c.id)`,
	} {
		if err := app.Exec(ctx, state.DB, statement); err != nil {
			return err
		}
	}
	return nil
}
