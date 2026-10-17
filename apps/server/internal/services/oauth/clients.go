package oauth_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
)

type Registration struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              *string  `json:"client_name"`
	TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
}

// Throttled records a registration from ip and says whether it went over the limit.
func (self *OAuthService) Throttled(ip string) bool {
	return self.limiter.hit("register:" + ip)
}

// Register registers a client (RFC 7591) and answers its registration.
func (self *OAuthService) Register(ctx context.Context, input *Registration) (map[string]any, error) {
	if len(input.RedirectURIs) == 0 || len(input.RedirectURIs) > 10 {
		return nil, oauthError(http.StatusBadRequest, "invalid_redirect_uri", "Provide 1 to 10 redirect_uris")
	}
	for _, uri := range input.RedirectURIs {
		if !auth.IsAllowedRedirect(uri) {
			return nil, oauthError(http.StatusBadRequest, "invalid_redirect_uri", "Redirect URI not allowed: "+uri)
		}
	}
	method := "none"
	if input.TokenEndpointAuthMethod != nil {
		method = *input.TokenEndpointAuthMethod
	}
	var secret, secretHash *string
	switch method {
	case "none":
	case "client_secret_post", "client_secret_basic":
		s := auth.RandomSecret("rr_cs_")
		h := auth.SecretHash(s)
		secret, secretHash = &s, &h
	default:
		return nil, oauthError(http.StatusBadRequest, "invalid_client_metadata", "Unsupported auth method "+method)
	}
	name := defaultClient
	if input.ClientName != nil {
		name = *input.ClientName
	}
	name = truncate(strings.TrimSpace(name), 100)
	id := "dcr_" + auth.RandomSecret("")[:24]
	if err := self.repo.OAuth().RegisterClient(ctx, id, secretHash, name, input.RedirectURIs); err != nil {
		slog.Error("client registration failed", "err", err)
		return nil, oauthError(http.StatusInternalServerError, "server_error", "Registration failed")
	}
	body := map[string]any{
		"client_id":                  id,
		"client_id_issued_at":        time.Now().Unix(),
		"client_name":                name,
		"redirect_uris":              input.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": method,
	}
	if secret != nil {
		body["client_secret"] = *secret
		body["client_secret_expires_at"] = 0
	}
	return body, nil
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
	host := auth.StripPort(authority)
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

func (self *OAuthService) fetchClientMetadata(ctx context.Context, address string) (clientMetadata, error) {
	var doc clientMetadata
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return doc, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ricrac")
	res, err := self.metadata.Do(req)
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
		if !auth.IsAllowedRedirect(uri) {
			return doc, fmt.Errorf("redirect URI not allowed: %s", uri)
		}
	}
	return doc, nil
}

// cimdClient refreshes the document at most once an hour and keeps the last copy when the fetch
// fails.
func (self *OAuthService) cimdClient(ctx context.Context, address string) (*ent.OAuthClient, error) {
	cached, err := self.repo.OAuth().Client(ctx, address)
	if err != nil {
		return nil, err
	}
	if cached != nil && cached.FetchedAt != nil && time.Since(*cached.FetchedAt) < cimdCacheTTL {
		return cached, nil
	}
	doc, err := self.fetchClientMetadata(ctx, address)
	if err != nil {
		slog.Warn("fetching client metadata failed", "url", address, "err", err)
		if cached == nil {
			return nil, errdefs.BadRequest("Could not read the application's identity document.")
		}
		return cached, nil
	}
	name := truncate(strings.TrimSpace(*doc.ClientName), 100)
	if name == "" {
		name = defaultClient
	}
	if err := self.repo.OAuth().SaveFetchedClient(ctx, address, name, doc.RedirectURIs); err != nil {
		return nil, err
	}
	now := time.Now()
	return &ent.OAuthClient{ID: address, Name: name, RedirectUris: doc.RedirectURIs, FetchedAt: &now}, nil
}
