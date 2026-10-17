package e2e

import (
	"context"
	"crypto/rsa"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
	"ricrac/github"
	"ricrac/mail"
	"ricrac/ops"
	"ricrac/server"
)

// backend is the server under test, seen from the tests: its router, its jobs and the few
// things tests set up that have no endpoint.
type backend struct {
	t      *testing.T
	state  *app.State
	tape   *tape
	router http.Handler
}

type mailMessage = mail.Message

func migrateDatabase(ctx context.Context, url string) error {
	pool, err := app.Connect(ctx, url)
	if err != nil {
		return err
	}
	pool.Close()
	return nil
}

func newBackend(t *testing.T, url string, tp *tape) *backend {
	pool, err := app.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	config := app.Config{PublicURL: publicURL, SecretKey: "test-secret-key-that-is-long-enough-123", MaxUploadSize: 1024 * 1024}
	return &backend{t: t, state: app.NewState(pool, config), tape: tp}
}

func (b *backend) Router() http.Handler {
	if b.router == nil {
		b.router = server.Router(b.state)
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
	found := ops.Find(op)
	if found == nil {
		return nil
	}
	return found.Result
}

func (b *backend) WorkspaceDefaults(ctx context.Context, workspace uuid.UUID) error {
	return pgx.BeginFunc(ctx, b.state.DB, func(tx pgx.Tx) error {
		return ops.WorkspaceDefaults(ctx, tx, workspace)
	})
}

func (b *backend) CreateTeam(ctx context.Context, workspace, user uuid.UUID, name, key string) (uuid.UUID, error) {
	var team uuid.UUID
	err := pgx.BeginFunc(ctx, b.state.DB, func(tx pgx.Tx) error {
		var err error
		team, err = ops.CreateTeam(ctx, tx, workspace, user, name, key)
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
	return server.Decide(ctx, b.state, user, workspaces, &server.OAuthRequest{
		ClientID: request.ClientID, RedirectURI: request.RedirectURI, ResponseType: request.ResponseType,
		CodeChallenge: request.CodeChallenge, CodeChallengeMethod: request.CodeChallengeMethod,
		State: request.State, Resource: request.Resource,
	}, true)
}

func (b *backend) Automations(ctx context.Context) error {
	return server.Automations(ctx, b.state)
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
