package oauth_service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	oauth_repo "ricrac/internal/repositories/oauth"
	"ricrac/internal/services"
	mocks_services_account "ricrac/mocks/services/account"
)

type OAuthServiceSuite struct {
	services.ServiceTestSuite
	account *mocks_services_account.AccountServiceMock
	service *OAuthService
}

func (suite *OAuthServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.account = mocks_services_account.NewAccountServiceMock(suite.T())
	suite.service = NewOAuthService(config.Config{PublicURL: "https://ricrac.example"}, suite.MockRepo, suite.account)
}

func TestOAuthService(t *testing.T) {
	suite.Run(t, new(OAuthServiceSuite))
}

func (suite *OAuthServiceSuite) TestPKCE() {
	suite.True(pkceMatches("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"), "the RFC 7636 example matches")
	suite.False(pkceMatches("short", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"), "short verifiers are refused")
}

func (suite *OAuthServiceSuite) TestRedirects() {
	for uri, allowed := range map[string]bool{
		"https://claude.ai/api/mcp/auth_callback": true,
		"http://localhost:33418/callback":         true,
		"http://[::1]:8080/cb":                    true,
		"motile://ricrac":                         true,
		"http://evil.com/cb":                      false,
		"http://localhost.evil.com/cb":            false,
		"javascript://alert(1)":                   false,
		"https://x.com/cb#frag":                   false,
	} {
		suite.Equal(allowed, auth.IsAllowedRedirect(uri), uri)
	}
	suite.True(auth.RedirectMatches("http://localhost/callback", "http://localhost:3118/callback"), "loopback redirects take any port")
	suite.False(auth.RedirectMatches("http://localhost/callback", "http://127.0.0.1:3118/callback"))
	suite.False(auth.RedirectMatches("https://a.com/cb", "https://a.com:444/cb"))
}

func (suite *OAuthServiceSuite) TestClientIDURLs() {
	for id, valid := range map[string]bool{
		"https://claude.ai/oauth/claude-code-client-metadata": true,
		"https://claude.ai/":              false,
		"http://claude.ai/oauth/metadata": false,
		"https://10.0.0.1/metadata":       false,
	} {
		suite.Equal(valid, isClientIDURL(id), id)
	}
}

func (suite *OAuthServiceSuite) TestScopes() {
	all, _ := requestedScopes(nil)
	suite.Equal([]string{"read", "write", "admin"}, all)
	written, _ := requestedScopes(new("write"))
	suite.Equal([]string{"read", "write"}, written)
	_, err := requestedScopes(new("issues:create"))
	suite.Equal(`Unknown scope "issues:create". Scopes are read, write and admin.`, errdefs.As(err).Message)
}

func (suite *OAuthServiceSuite) TestBasicCredentialsAreUnescaped() {
	credentials := CredentialsOf("Basic YSUyMGI6cyUzQXQ=", url.Values{"client_id": {"ignored"}})
	suite.Equal("a b", credentials.ClientID)
	suite.Equal("s:t", *credentials.Secret)
	suite.True(credentials.Present)
}

// metadataServer serves a client ID metadata document made by doc from the server's own URL.
func (suite *OAuthServiceSuite) metadataServer(doc func(address string) map[string]any) string {
	var address string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(doc(address))
	}))
	suite.T().Cleanup(server.Close)
	address = server.URL + "/client"
	return address
}

func (suite *OAuthServiceSuite) TestMetadataDocumentsAreChecked() {
	for message, doc := range map[string]func(string) map[string]any{
		"missing field client_name": func(address string) map[string]any {
			return map[string]any{"client_id": address, "redirect_uris": []string{"https://app.example/cb"}}
		},
		"client_id does not match the document URL": func(string) map[string]any {
			return map[string]any{"client_id": "https://other.example/client", "client_name": "App", "redirect_uris": []string{"https://app.example/cb"}}
		},
		"only public clients are supported": func(address string) map[string]any {
			return map[string]any{"client_id": address, "client_name": "App", "redirect_uris": []string{"https://app.example/cb"},
				"token_endpoint_auth_method": "private_key_jwt"}
		},
		"redirect URI not allowed: http://app.example/cb": func(address string) map[string]any {
			return map[string]any{"client_id": address, "client_name": "App", "redirect_uris": []string{"http://app.example/cb"}}
		},
	} {
		address := suite.metadataServer(doc)
		_, err := suite.service.fetchClientMetadata(suite.Ctx, address)
		suite.EqualError(err, message)
	}
	address := suite.metadataServer(func(address string) map[string]any {
		return map[string]any{"client_id": address, "client_name": "App", "redirect_uris": []string{"https://app.example/cb"},
			"token_endpoint_auth_method": "private_key_jwt", "token_endpoint_auth_methods_supported": []string{"private_key_jwt", "none"}}
	})
	doc, err := suite.service.fetchClientMetadata(suite.Ctx, address)
	suite.NoError(err, "clients that also offer none are public")
	suite.Equal("App", *doc.ClientName)
}

func (suite *OAuthServiceSuite) TestFreshMetadataDocumentsAreNotFetchedAgain() {
	fetched := time.Now().Add(-time.Minute)
	cached := &ent.OAuthClient{ID: "https://app.example/client", Name: "Cached", FetchedAt: &fetched}
	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, cached.ID).Return(cached, nil)
	found, err := suite.service.cimdClient(suite.Ctx, cached.ID)
	suite.NoError(err)
	suite.Same(cached, found)
}

func (suite *OAuthServiceSuite) TestStaleMetadataDocumentsAreRefetched() {
	address := suite.metadataServer(func(address string) map[string]any {
		return map[string]any{"client_id": address, "client_name": "  " + strings.Repeat("é", 120), "redirect_uris": []string{"https://app.example/cb"}}
	})
	fetched := time.Now().Add(-2 * time.Hour)
	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, address).Return(&ent.OAuthClient{ID: address, Name: "Old", FetchedAt: &fetched}, nil)
	suite.MockOAuthRepo.EXPECT().SaveFetchedClient(mock.Anything, address, strings.Repeat("é", 100), []string{"https://app.example/cb"}).Return(nil)
	found, err := suite.service.cimdClient(suite.Ctx, address)
	suite.NoError(err)
	suite.Equal(strings.Repeat("é", 100), found.Name)
}

func (suite *OAuthServiceSuite) TestUnreadableMetadataDocumentsFallBackToTheLastCopy() {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	address := server.URL + "/client"
	fetched := time.Now().Add(-2 * time.Hour)
	cached := &ent.OAuthClient{ID: address, Name: "Cached", FetchedAt: &fetched}
	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, address).Return(cached, nil).Once()
	found, err := suite.service.cimdClient(suite.Ctx, address)
	suite.NoError(err)
	suite.Same(cached, found)

	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, address).Return(nil, nil).Once()
	_, err = suite.service.cimdClient(suite.Ctx, address)
	suite.Equal("Could not read the application's identity document.", errdefs.As(err).Message)
}

func (suite *OAuthServiceSuite) request(client *ent.OAuthClient) *OAuthRequest {
	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, client.ID).Return(client, nil)
	return &OAuthRequest{
		ClientID: client.ID, RedirectURI: "http://localhost:4567/callback", ResponseType: "code",
		CodeChallenge: new(strings.Repeat("c", 43)), CodeChallengeMethod: new("S256"), State: new("a b"),
	}
}

func (suite *OAuthServiceSuite) TestRequestsNeedPKCEOrADashboardSecret() {
	public := &ent.OAuthClient{ID: "dcr_public", Name: "Public", RedirectUris: []string{"http://localhost/callback"}}
	req := suite.request(public)
	req.CodeChallengeMethod = new("plain")
	_, err := suite.service.CheckRequest(suite.Ctx, req)
	suite.Equal("The application must use PKCE (S256).", errdefs.As(err).Message)

	owner := uuid.New()
	confidential := &ent.OAuthClient{ID: "app", Name: "App", RedirectUris: []string{"http://localhost/callback"}, SecretHash: new("h"), OwnerID: &owner}
	req = suite.request(confidential)
	req.CodeChallenge, req.CodeChallengeMethod = nil, nil
	info, err := suite.service.CheckRequest(suite.Ctx, req)
	suite.NoError(err)
	suite.True(info.Verified)
	suite.Equal("localhost:4567", info.RedirectHost)
}

func (suite *OAuthServiceSuite) TestDecliningRedirectsWithAccessDenied() {
	req := suite.request(&ent.OAuthClient{ID: "dcr_public", Name: "Public", RedirectUris: []string{"http://localhost/callback"}})
	redirect, err := suite.service.Decide(suite.Ctx, uuid.New(), nil, req, false)
	suite.NoError(err)
	suite.Equal("http://localhost:4567/callback?iss=https%3A%2F%2Fricrac.example&state=a%20b&error=access_denied", redirect)
}

func (suite *OAuthServiceSuite) TestCodesAreSingleUseAndBoundToTheirVerifier() {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, "dcr_public").Return(&ent.OAuthClient{ID: "dcr_public"}, nil)
	suite.MockOAuthRepo.EXPECT().TakeCode(mock.Anything, auth.SecretHash("code")).Return(&oauth_repo.Code{
		ClientID: "dcr_public", RedirectURI: "http://localhost/cb", CodeChallenge: new("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"),
		ExpiresAt: time.Now().Add(time.Minute),
	}, nil).Once()
	credentials := Credentials{ClientID: "dcr_public", Present: true}
	_, err := suite.service.Token(suite.Ctx, credentials, url.Values{"grant_type": {"authorization_code"}, "code": {"code"}, "code_verifier": {verifier + "x"}})
	suite.Equal(&Error{Status: 400, Code: "invalid_grant", Description: "PKCE verification failed"}, err)

	suite.MockOAuthRepo.EXPECT().TakeCode(mock.Anything, auth.SecretHash("code")).Return(nil, nil).Once()
	_, err = suite.service.Token(suite.Ctx, credentials, url.Values{"grant_type": {"authorization_code"}, "code": {"code"}, "code_verifier": {verifier}})
	suite.Equal(&Error{Status: 400, Code: "invalid_grant", Description: "Unknown or used code"}, err)
}

func (suite *OAuthServiceSuite) TestClientsWithASecretMustProveIt() {
	suite.MockOAuthRepo.EXPECT().Client(mock.Anything, "app").Return(&ent.OAuthClient{ID: "app", SecretHash: new(auth.SecretHash("right"))}, nil)
	_, err := suite.service.Token(suite.Ctx, Credentials{ClientID: "app", Secret: new("wrong"), Present: true}, url.Values{})
	suite.Equal(invalidClient, err)
	_, err = suite.service.Token(suite.Ctx, Credentials{ClientID: "app", Secret: new("right"), Present: true}, url.Values{"grant_type": {"password"}})
	suite.Equal(&Error{Status: 400, Code: "unsupported_grant_type", Description: "Use authorization_code or refresh_token"}, err)
}
