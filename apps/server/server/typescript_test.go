package server

import (
	"os"
	"testing"

	"ricrac/ops"
)

// TestWebTypes writes the dashboard API's types for the web client when they change.
func TestWebTypes(t *testing.T) {
	types, err := ops.TypeScriptFor(
		[]any{AppConfig{}, Me{}, APIKey{}, CreatedAPIKey{}, OAuthApp{}, OAuthAppSecret{}, ConnectedApp{}, OAuthClientInfo{}, OAuthRedirect{}},
		[]any{UpdateMe{}, CreateWorkspace{}, DeleteWorkspace{}, CreateAPIKey{}, SaveOAuthApp{}, OAuthDecision{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	const path = "../../web/src/api/account.gen.ts"
	if current, err := os.ReadFile(path); err == nil && string(current) == types {
		return
	}
	if err := os.WriteFile(path, []byte(types), 0o644); err != nil {
		t.Fatal(err)
	}
}
