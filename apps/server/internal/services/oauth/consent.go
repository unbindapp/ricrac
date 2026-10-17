package oauth_service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	oauth_repo "ricrac/internal/repositories/oauth"
)

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

type OAuthDecision struct {
	Request OAuthRequest `json:"request"`
	// The workspaces to grant; all of the user's, including ones they join later, when left out.
	WorkspaceIDs []uuid.UUID `json:"workspaceIds"`
	Allow        bool        `json:"allow"`
}

func requestedScopes(scope *string) ([]string, error) {
	if scope == nil || strings.TrimSpace(*scope) == "" {
		return slices.Clone(auth.AllScopes), nil
	}
	var scopes []string
	for _, part := range strings.FieldsFunc(*scope, func(r rune) bool { return r == ' ' || r == ',' }) {
		if !slices.Contains(auth.AllScopes, part) {
			return nil, errdefs.BadRequest("Unknown scope %q. Scopes are read, write and admin.", part)
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

func resourceMatches(resource *string, base string) bool {
	if resource == nil {
		return true
	}
	r := strings.TrimRight(*resource, "/")
	return r == base || r == base+"/mcp" || r == base+"/v1"
}

func (self *OAuthService) client(ctx context.Context, id string) (*ent.OAuthClient, error) {
	if isClientIDURL(id) {
		return self.cimdClient(ctx, id)
	}
	found, err := self.repo.OAuth().Client(ctx, id)
	if err == nil && found == nil {
		err = errdefs.BadRequest("Unknown application. Try connecting again.")
	}
	return found, err
}

// CheckRequest checks an authorization request before the consent screen. Its errors are shown
// on the page, never sent to an unverified redirect URI.
func (self *OAuthService) CheckRequest(ctx context.Context, req *OAuthRequest) (*models.OAuthClientInfo, error) {
	found, err := self.client(ctx, req.ClientID)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(found.RedirectUris, func(u string) bool { return auth.RedirectMatches(u, req.RedirectURI) }) {
		return nil, errdefs.BadRequest("The application's redirect address doesn't match its registration.")
	}
	if req.ResponseType != "code" {
		return nil, errdefs.BadRequest("Unsupported response_type.")
	}
	pkce := req.CodeChallenge != nil && req.CodeChallengeMethod != nil && *req.CodeChallengeMethod == "S256" && len(*req.CodeChallenge) >= 43
	confidential := req.CodeChallenge == nil && req.CodeChallengeMethod == nil && found.SecretHash != nil && found.OwnerID != nil
	if !pkce && !confidential {
		return nil, errdefs.BadRequest("The application must use PKCE (S256).")
	}
	if !resourceMatches(req.Resource, self.cfg.PublicURL) {
		return nil, errdefs.BadRequest("The application asked for access to a different server.")
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
	return &models.OAuthClientInfo{ClientName: found.Name, RedirectHost: redirectHost, Scopes: scopes, Verified: found.OwnerID != nil}, nil
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

// Decide records the user's decision and returns where to send the browser. Nil workspaces
// grants every workspace of the user, including ones they join later.
func (self *OAuthService) Decide(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID, req *OAuthRequest, allow bool) (string, error) {
	info, err := self.CheckRequest(ctx, req)
	if err != nil {
		return "", err
	}
	params := [][2]string{{"iss", self.cfg.PublicURL}}
	if req.State != nil && *req.State != "" {
		params = append(params, [2]string{"state", *req.State})
	}
	if !allow {
		params = append(params, [2]string{"error", "access_denied"})
		return withParams(req.RedirectURI, params), nil
	}
	if err := self.account.CheckWorkspaces(ctx, user, workspaces); err != nil {
		return "", err
	}
	code := auth.RandomSecret("rr_ac_")
	err = self.repo.OAuth().CreateCode(ctx, auth.SecretHash(code), oauth_repo.Code{
		ClientID: req.ClientID, UserID: user, Workspaces: workspaces, Scopes: info.Scopes, RedirectURI: req.RedirectURI,
		CodeChallenge: req.CodeChallenge, Resource: req.Resource,
	}, codeTTL)
	if err != nil {
		return "", err
	}
	return withParams(req.RedirectURI, append([][2]string{{"code", code}}, params...)), nil
}
