package config

import "testing"

func TestBadOptionalSettingsTurnTheirFeatureOff(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://ricrac.example.com")
	t.Setenv("SECRET_KEY", "a-secret-key-that-is-long-enough-1234")
	t.Setenv("DATABASE_URL", "postgres://localhost/ricrac")
	t.Setenv("GITHUB_APP_ID", "5352660")
	t.Setenv("GITHUB_APP_SLUG", "ricrac-app")
	t.Setenv("GITHUB_CLIENT_ID", "Iv23example")
	t.Setenv("GITHUB_CLIENT_SECRET", "placeholder")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "placeholder")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "placeholder")
	t.Setenv("SMTP_URL", "smtps://resend:key@smtp.resend.com")
	config, err := FromEnv()
	if err != nil {
		t.Fatalf("Ricrac should still start: %v", err)
	}
	if config.GitHub != nil || config.Mail != nil {
		t.Fatal("GitHub and email stay off until they're set right")
	}
}

func TestAssistantAdminsIgnoreCaseAndSpaces(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "token")
	t.Setenv("ASSISTANT_CLAUDE_PATH", "/bin/sh")
	t.Setenv("ASSISTANT_ADMINS", " Ann@Acme.test ,bob@acme.test,")
	assistant, err := ParseAssistant()
	if err != nil {
		t.Fatal(err)
	}
	if !assistant.Manages("ann@acme.test") || !assistant.Manages("BOB@acme.test") || assistant.Manages("cy@acme.test") {
		t.Fatalf("admins %v", assistant.Admins)
	}
	if assistant.Credential != "CLAUDE_CODE_OAUTH_TOKEN=token" {
		t.Fatalf("credential %q", assistant.Credential)
	}
}
