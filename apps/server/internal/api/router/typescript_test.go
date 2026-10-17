package router

import (
	"os"
	"testing"

	"ricrac/config"
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/models"
	account_service "ricrac/internal/services/account"
	oauth_service "ricrac/internal/services/oauth"
)

func registry(t *testing.T) *server.Server {
	t.Helper()
	return server.New(config.Config{PublicURL: "http://localhost:3000"}, nil, nil)
}

// TestWebTypes writes the web client's types when the operations change, as the build expects
// them to be current.
func TestWebTypes(t *testing.T) {
	_, reg := New(registry(t), nil)
	types, err := reg.TypeScript()
	if err != nil {
		t.Fatal(err)
	}
	const path = "../../../../web/src/api/ops.gen.ts"
	if current, err := os.ReadFile(path); err == nil && string(current) == types {
		return
	}
	if err := os.WriteFile(path, []byte(types), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAccountWebTypes writes the dashboard API's types for the web client when they change.
func TestAccountWebTypes(t *testing.T) {
	types, err := oapi.TypeScriptFor(
		[]any{models.AppConfig{}, models.Me{}, models.APIKey{}, models.CreatedAPIKey{}, models.OAuthApp{}, models.OAuthAppSecret{},
			models.ConnectedApp{}, models.OAuthClientInfo{}, models.OAuthRedirect{}},
		[]any{account_service.UpdateMe{}, account_service.CreateWorkspace{}, account_service.DeleteWorkspace{}, account_service.CreateAPIKey{},
			account_service.SaveOAuthApp{}, oauth_service.OAuthDecision{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	const path = "../../../../web/src/api/account.gen.ts"
	if current, err := os.ReadFile(path); err == nil && string(current) == types {
		return
	}
	if err := os.WriteFile(path, []byte(types), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEveryOperationDeclaresItsResult(t *testing.T) {
	_, reg := New(registry(t), nil)
	for _, o := range reg.All() {
		if o.Result == nil {
			t.Errorf("%s doesn't declare its result", o.Name)
		}
	}
}
