package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config is the GitHub App the server acts as.
type Config struct {
	AppID         int64
	Slug          string
	PrivateKey    *rsa.PrivateKey
	WebhookSecret string
	// The app's OAuth credentials, to learn who installed it and to link people's accounts.
	ClientID     string
	ClientSecret string
	// https://api.github.com and https://github.com, or a fake GitHub in tests.
	APIURL string
	WebURL string
}

// ParseConfig reads the app's settings; nil when none are set, an error when only some are.
func ParseConfig(appID, slug, privateKey, webhookSecret, clientID, clientSecret, apiURL, webURL string) (*Config, error) {
	values := map[string]string{
		"GITHUB_APP_ID": appID, "GITHUB_APP_SLUG": slug, "GITHUB_APP_PRIVATE_KEY": privateKey,
		"GITHUB_WEBHOOK_SECRET": webhookSecret, "GITHUB_CLIENT_ID": clientID, "GITHUB_CLIENT_SECRET": clientSecret,
	}
	var missing []string
	for _, name := range []string{"GITHUB_APP_ID", "GITHUB_APP_SLUG", "GITHUB_APP_PRIVATE_KEY", "GITHUB_WEBHOOK_SECRET", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET"} {
		if values[name] == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == len(values) {
		return nil, nil
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("the GitHub App needs %s too", strings.Join(missing, ", "))
	}
	id, err := strconv.ParseInt(appID, 10, 64)
	if err != nil {
		return nil, errors.New("GITHUB_APP_ID must be a number")
	}
	key, err := ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return &Config{
		AppID: id, Slug: slug, PrivateKey: key, WebhookSecret: webhookSecret, ClientID: clientID, ClientSecret: clientSecret,
		APIURL: strings.TrimRight(or(apiURL, "https://api.github.com"), "/"),
		WebURL: strings.TrimRight(or(webURL, "https://github.com"), "/"),
	}, nil
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// ParsePrivateKey reads the app's PEM private key, with real or literal \n line breaks.
func ParsePrivateKey(text string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.ReplaceAll(strings.TrimSpace(text), `\n`, "\n")))
	if block == nil {
		return nil, errors.New("GITHUB_APP_PRIVATE_KEY isn't a PEM private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(*rsa.PrivateKey)
	if err != nil || !ok {
		return nil, errors.New("GITHUB_APP_PRIVATE_KEY isn't an RSA private key")
	}
	return key, nil
}

// App calls GitHub as the app, as one of its installations, or as a user.
type App struct {
	Config
	HTTP *http.Client

	mu     sync.Mutex
	tokens map[int64]installationToken
}

type installationToken struct {
	token   string
	expires time.Time
}

func NewApp(config Config, client *http.Client) *App {
	return &App{Config: config, HTTP: client, tokens: map[int64]installationToken{}}
}

// BotLogin is the login GitHub shows for what the app does, like ricrac[bot].
func (a *App) BotLogin() string {
	return strings.ToLower(a.Slug) + "[bot]"
}

// InstallURL is where people install the app; GitHub hands state back to the setup URL.
func (a *App) InstallURL(state string) string {
	link := a.WebURL + "/apps/" + url.PathEscape(a.Slug) + "/installations/new"
	if state == "" {
		return link
	}
	return link + "?" + url.Values{"state": {state}}.Encode()
}

// AuthorizeURL asks someone to sign in to GitHub for the app, to learn their login.
func (a *App) AuthorizeURL(redirectURI, state string) string {
	return a.WebURL + "/login/oauth/authorize?" + url.Values{
		"client_id": {a.ClientID}, "redirect_uri": {redirectURI}, "state": {state},
	}.Encode()
}

// JWT authenticates as the app itself, for ten minutes at most.
func (a *App) JWT(now time.Time) (string, error) {
	encode := func(value any) string {
		data, _ := json.Marshal(value)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	unsigned := encode(map[string]string{"alg": "RS256", "typ": "JWT"}) + "." +
		encode(map[string]any{"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(9 * time.Minute).Unix(), "iss": strconv.FormatInt(a.AppID, 10)})
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, a.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// InstallationToken is a token acting as the installation, cached until shortly before it
// expires.
func (a *App) InstallationToken(ctx context.Context, installation int64) (string, error) {
	a.mu.Lock()
	cached, ok := a.tokens[installation]
	a.mu.Unlock()
	if ok && time.Until(cached.expires) > 5*time.Minute {
		return cached.token, nil
	}
	jwt, err := a.JWT(time.Now())
	if err != nil {
		return "", err
	}
	var created struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	path := "/app/installations/" + strconv.FormatInt(installation, 10) + "/access_tokens"
	if _, err := a.Do(ctx, jwt, http.MethodPost, path, nil, &created); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.tokens[installation] = installationToken{created.Token, created.ExpiresAt}
	a.mu.Unlock()
	return created.Token, nil
}

// ExchangeCode turns the code GitHub sent to a callback into a user access token.
func (a *App) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{"client_id": {a.ClientID}, "client_secret": {a.ClientSecret}, "code": {code}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.WebURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ricrac")
	var answer struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if _, err := a.send(req, &answer); err != nil {
		return "", err
	}
	if answer.AccessToken == "" {
		return "", fmt.Errorf("GitHub refused the code: %s %s", answer.Error, answer.ErrorDescription)
	}
	return answer.AccessToken, nil
}

// APIError is an answer of GitHub other than 2xx.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("GitHub answered %d: %s", e.Status, e.Message)
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Status == http.StatusNotFound || apiErr.Status == http.StatusGone)
}

// Do calls the REST API with a token, sending body as JSON and decoding the answer into out. It
// answers the next page, or 0 when there's none.
func (a *App) Do(ctx context.Context, token, method, path string, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.APIURL+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "ricrac")
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return a.send(req, out)
}

func (a *App) send(req *http.Request, out any) (int, error) {
	res, err := a.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return 0, &APIError{Status: res.StatusCode, Message: errorMessage(raw)}
	}
	next := nextPage(res.Header.Get("Link"))
	if out == nil || res.StatusCode == http.StatusNoContent {
		return next, nil
	}
	return next, json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(out)
}

func errorMessage(raw []byte) string {
	var parsed struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.Message != "" {
		return parsed.Message
	}
	return strings.TrimSpace(string(raw))
}

// nextPage reads the page number of rel="next" in a Link header.
func nextPage(link string) int {
	for part := range strings.SplitSeq(link, ",") {
		target, rel, ok := strings.Cut(part, ";")
		if !ok || !strings.Contains(rel, `rel="next"`) {
			continue
		}
		parsed, err := url.Parse(strings.Trim(strings.TrimSpace(target), "<>"))
		if err != nil {
			return 0
		}
		page, _ := strconv.Atoi(parsed.Query().Get("page"))
		return page
	}
	return 0
}

// RepoPath is /repos/owner/name.
func RepoPath(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func IssuePath(fullName string, number int) string {
	return RepoPath(fullName) + "/issues/" + strconv.Itoa(number)
}

func CommentPath(fullName string, comment int64) string {
	return RepoPath(fullName) + "/issues/comments/" + strconv.FormatInt(comment, 10)
}

// UserLogin is the login of a user token's owner.
func (a *App) UserLogin(ctx context.Context, token string) (string, error) {
	var user Account
	if _, err := a.Do(ctx, token, http.MethodGet, "/user", nil, &user); err != nil {
		return "", err
	}
	return user.Login, nil
}

// UserInstallations is the app's installations a user token's owner can reach.
func (a *App) UserInstallations(ctx context.Context, token string) ([]Installation, error) {
	var all []Installation
	for page := 1; page > 0 && page <= 10; {
		var answer struct {
			Installations []Installation `json:"installations"`
		}
		next, err := a.Do(ctx, token, http.MethodGet, "/user/installations?per_page=100&page="+strconv.Itoa(page), nil, &answer)
		if err != nil {
			return nil, err
		}
		all = append(all, answer.Installations...)
		page = next
	}
	return all, nil
}

// InstallationRepositories is the repositories an installation was given, at most 1000.
func (a *App) InstallationRepositories(ctx context.Context, installation int64) ([]Repository, error) {
	token, err := a.InstallationToken(ctx, installation)
	if err != nil {
		return nil, err
	}
	var all []Repository
	for page := 1; page > 0 && page <= 10; {
		var answer struct {
			Repositories []Repository `json:"repositories"`
		}
		next, err := a.Do(ctx, token, http.MethodGet, "/installation/repositories?per_page=100&page="+strconv.Itoa(page), nil, &answer)
		if err != nil {
			return nil, err
		}
		all = append(all, answer.Repositories...)
		page = next
	}
	return all, nil
}
