package e2e

import (
	"context"
	"crypto/rsa"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"ricrac/app"
	"ricrac/config"
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/router"
	apiserver "ricrac/internal/api/server"
	"ricrac/internal/infrastructure/database"
	"ricrac/internal/infrastructure/mail"
	"ricrac/internal/integrations/github"
	"ricrac/internal/legacy"
	repository "ricrac/internal/repositories"
	oauth_service "ricrac/internal/services/oauth"
	"ricrac/ops"
	"ricrac/server"
)

// backend is the server under test, seen from the tests: its router, its jobs and the few
// things tests set up that have no endpoint.
type backend struct {
	t        *testing.T
	state    *app.State
	tape     *tape
	router   http.Handler
	registry *oapi.Registry
	server   *apiserver.Server
	bridge   *legacy.Bridge
}

type mailMessage = mail.Message

func migrateDatabase(ctx context.Context, url string) error {
	client, db, err := database.Open(ctx, url)
	if err != nil {
		return err
	}
	defer client.Close()
	return database.Migrate(ctx, db)
}

func newBackend(t *testing.T, url string, tp *tape) *backend {
	ctx := context.Background()
	cfg := config.Config{DatabaseURL: url, PublicURL: publicURL, SecretKey: "test-secret-key-that-is-long-enough-123", MaxUploadSize: 1024 * 1024}
	client, _, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	bridge, err := legacy.New(ctx, cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(bridge.Close)
	return &backend{t: t, state: bridge.State, tape: tp, server: apiserver.New(cfg, client, bridge), bridge: bridge}
}

func (b *backend) Router() http.Handler {
	if b.router == nil {
		b.server.Cfg = config.Config(b.state.Config)
		b.router, b.registry = router.New(b.server, b.bridge)
	}
	return b.router
}

func (b *backend) SessionCookie(user uuid.UUID) string {
	return b.state.Keys.SessionCookie + "=" + b.state.Keys.SessionToken(user)
}

// Sign signs a payload with the server's secret, as its signed state parameters are.
func (b *backend) Sign(payload string) string {
	return b.state.Keys.Sign(payload)
}

func (b *backend) ResultSchema(op string) M {
	b.Router()
	found := b.registry.Find(op)
	if found == nil {
		return nil
	}
	return found.Result
}

func (b *backend) WorkspaceDefaults(ctx context.Context, workspace uuid.UUID) error {
	return b.server.Repository.WithTx(ctx, func(tx repository.TxInterface) error {
		return b.server.SeedService.WorkspaceDefaults(ctx, tx, workspace)
	})
}

func (b *backend) CreateTeam(ctx context.Context, workspace, user uuid.UUID, name, key string) (uuid.UUID, error) {
	var team uuid.UUID
	err := b.server.Repository.WithTx(ctx, func(tx repository.TxInterface) error {
		var err error
		team, err = b.server.SeedService.CreateTeam(ctx, tx, workspace, user, name, key)
		return err
	})
	return team, err
}

func (b *backend) SetMailer(sender mail.Sender) {
	b.state.Mailer = sender
}

func (b *backend) ConfigureGitHub(apiURL string, client *http.Client, key *rsa.PrivateKey, webhookSecret string) {
	config := github.Config{
		AppID: 1, Slug: "ricrac-test", PrivateKey: key, WebhookSecret: webhookSecret,
		ClientID: "Iv1.test", ClientSecret: "client-secret", APIURL: apiURL, WebURL: apiURL,
	}
	b.state.Config.GitHub = &config
	b.state.GitHub = github.NewApp(config, client)
	b.router = nil
}

// OAuthDecide is the consent screen's answer: the redirect to the client, or why it failed.
func (b *backend) OAuthDecide(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID, request oauthRequest) (string, error) {
	return b.server.OAuthService.Decide(ctx, user, workspaces, &oauth_service.OAuthRequest{
		ClientID: request.ClientID, RedirectURI: request.RedirectURI, ResponseType: request.ResponseType,
		CodeChallenge: request.CodeChallenge, CodeChallengeMethod: request.CodeChallengeMethod,
		State: request.State, Resource: request.Resource,
	}, true)
}

func (b *backend) Automations(ctx context.Context) error {
	return b.server.AutomationService.Run(ctx)
}

func (b *backend) RunCycleAutomations(ctx context.Context) error {
	return ops.RunCycleAutomations(ctx, b.state)
}

func (b *backend) RunRecurringTemplates(ctx context.Context) error {
	return ops.RunRecurringTemplates(ctx, b.state)
}

func (b *backend) SendEmails(ctx context.Context) error {
	return ops.SendEmails(ctx, b.state)
}

func (b *backend) DeliverDueWebhooks(ctx context.Context) (int, error) {
	return ops.DeliverDueWebhooks(ctx, b.state)
}

func (b *backend) GitHubJobs(ctx context.Context) (bool, error) {
	return server.GitHubJobs(ctx, b.state)
}
