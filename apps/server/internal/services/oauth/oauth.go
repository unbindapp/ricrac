// Package oauth_service is the OAuth 2.1 server MCP clients and dashboard apps connect through:
// its metadata, dynamic registration, client ID metadata documents, consent, codes with PKCE,
// rotating refresh tokens and revocation.
package oauth_service

import (
	"net/http"
	"sync"
	"time"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/repositories/repositories"
	account_service "ricrac/internal/services/account"
)

const (
	codeTTL       = 10 * 60
	accessTTL     = 24 * 3600
	refreshTTL    = 90 * 24 * 3600
	cimdMaxBytes  = 64 * 1024
	cimdCacheTTL  = time.Hour
	defaultClient = "MCP client"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i OAuthServiceInterface -p oauth_service -s OAuthService -o oauth_service_iface.go
type OAuthService struct {
	cfg      config.Config
	repo     repositories.RepositoriesInterface
	account  account_service.AccountServiceInterface
	limiter  *rateLimiter
	metadata *http.Client
}

func NewOAuthService(cfg config.Config, repo repositories.RepositoriesInterface, account account_service.AccountServiceInterface) *OAuthService {
	return &OAuthService{
		cfg: cfg, repo: repo, account: account,
		limiter: newRateLimiter(20, 15*time.Minute),
		metadata: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Error is an OAuth error answer: {"error": Code, "error_description": Description}.
type Error struct {
	Status      int
	Code        string
	Description string
}

func (e *Error) Error() string { return e.Code + ": " + e.Description }

func oauthError(status int, code, description string) *Error {
	return &Error{Status: status, Code: code, Description: description}
}

func (self *OAuthService) ProtectedResource() map[string]any {
	return map[string]any{
		"resource":                 self.cfg.MCPURL(),
		"authorization_servers":    []string{self.cfg.PublicURL},
		"scopes_supported":         auth.AllScopes,
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Ricrac",
	}
}

func (self *OAuthService) AuthorizationServer() map[string]any {
	base := self.cfg.PublicURL
	return map[string]any{
		"issuer":                                         base,
		"authorization_endpoint":                         base + "/oauth/authorize",
		"token_endpoint":                                 base + "/oauth/token",
		"registration_endpoint":                          base + "/oauth/register",
		"revocation_endpoint":                            base + "/oauth/revoke",
		"scopes_supported":                               auth.AllScopes,
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none", "client_secret_post", "client_secret_basic"},
		"client_id_metadata_document_supported":          true,
		"authorization_response_iss_parameter_supported": true,
	}
}

// rateLimiter counts attempts per key inside a window.
type rateLimiter struct {
	maxAttempts int
	window      time.Duration
	mu          sync.Mutex
	entries     map[string]attempts
}

type attempts struct {
	count int
	since time.Time
}

func newRateLimiter(maxAttempts int, window time.Duration) *rateLimiter {
	return &rateLimiter{maxAttempts: maxAttempts, window: window, entries: map[string]attempts{}}
}

// hit records an attempt and says whether it went over the limit.
func (l *rateLimiter) hit(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok || time.Since(entry.since) >= l.window {
		entry = attempts{since: time.Now()}
	}
	entry.count++
	l.entries[key] = entry
	return entry.count > l.maxAttempts
}

func (l *rateLimiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.entries {
		if time.Since(entry.since) >= l.window {
			delete(l.entries, key)
		}
	}
}
