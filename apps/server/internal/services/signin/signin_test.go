package signin_service

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/services"
)

type SigninServiceSuite struct {
	services.ServiceTestSuite
	keys    auth.Keys
	service *SigninService
}

func (suite *SigninServiceSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	cfg := config.Config{PublicURL: "https://ricrac.example", SecretKey: "test-secret-key-that-is-long-enough-123"}
	suite.keys = auth.NewKeys(cfg)
	suite.service = NewSigninService(cfg, suite.keys, suite.MockRepo, nil)
}

func TestSigninService(t *testing.T) {
	suite.Run(t, new(SigninServiceSuite))
}

func idToken(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func googleClaims() map[string]any {
	return map[string]any{
		"iss": "https://accounts.google.com", "aud": "client", "exp": time.Now().Unix() + 60, "nonce": "n",
		"sub": "123", "email": "Ann@Example.com", "email_verified": true, "name": "Ann Lee",
	}
}

func (suite *SigninServiceSuite) TestReadsIdentityAndRejectsWrongClaims() {
	identity, err := IdentityFromIDToken(idToken(googleClaims()), "client", "n")
	suite.NoError(err)
	suite.Equal("ann@example.com", identity.Email)
	suite.True(identity.EmailVerified)
	suite.Equal("Ann Lee", *identity.Name)
	for key, value := range map[string]any{"iss": "https://evil.com", "aud": "other", "exp": 1, "nonce": "x"} {
		bad := googleClaims()
		bad[key] = value
		_, err := IdentityFromIDToken(idToken(bad), "client", "n")
		suite.Error(err, "a wrong %s is refused", key)
	}
}

func (suite *SigninServiceSuite) TestReturnToStaysOnTheSite() {
	suite.Equal("/acme?x=1", SafeReturnTo("/acme?x=1"))
	for _, unsafe := range []string{"//evil.com", "/\\evil.com", "https://evil.com", ""} {
		suite.Equal("/", SafeReturnTo(unsafe), unsafe)
	}
	suite.Equal("/acme?x=1&signin_error=denied", WithSigninError("/acme?x=1", "denied"))
}

func (suite *SigninServiceSuite) TestRoundTripsNeedTheirCookieAndState() {
	_, cookie := suite.service.Start("/acme")
	payload, _ := suite.keys.Verify(cookie)
	round, _ := decodePending(payload)

	returnTo, user, reason := suite.service.Finish(suite.Ctx, "forged", url.Values{"state": {round.state}})
	suite.Equal([]any{"/", uuid.Nil, "expired"}, []any{returnTo, user, reason})
	_, _, reason = suite.service.Finish(suite.Ctx, cookie, url.Values{"state": {"other"}, "code": {"c"}})
	suite.Equal("expired", reason)
	returnTo, _, reason = suite.service.Finish(suite.Ctx, cookie, url.Values{"state": {round.state}, "error": {"access_denied"}})
	suite.Equal([]any{"/acme", "denied"}, []any{returnTo, reason})
	_, _, reason = suite.service.Finish(suite.Ctx, cookie, url.Values{"state": {round.state}})
	suite.Equal("failed", reason)
}
