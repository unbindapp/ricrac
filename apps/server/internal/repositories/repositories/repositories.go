package repositories

import (
	"context"
	"time"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
	access_repo "ricrac/internal/repositories/access"
	account_repo "ricrac/internal/repositories/account"
	attachment_repo "ricrac/internal/repositories/attachment"
	automation_repo "ricrac/internal/repositories/automation"
	comment_repo "ricrac/internal/repositories/comment"
	customer_repo "ricrac/internal/repositories/customer"
	cycle_repo "ricrac/internal/repositories/cycle"
	document_repo "ricrac/internal/repositories/document"
	email_repo "ricrac/internal/repositories/email"
	events_repo "ricrac/internal/repositories/events"
	favorite_repo "ricrac/internal/repositories/favorite"
	issue_repo "ricrac/internal/repositories/issue"
	label_repo "ricrac/internal/repositories/label"
	notification_repo "ricrac/internal/repositories/notification"
	oauth_repo "ricrac/internal/repositories/oauth"
	release_repo "ricrac/internal/repositories/release"
	resolve_repo "ricrac/internal/repositories/resolve"
	search_repo "ricrac/internal/repositories/search"
	seed_repo "ricrac/internal/repositories/seed"
	state_repo "ricrac/internal/repositories/state"
	team_repo "ricrac/internal/repositories/team"
	template_repo "ricrac/internal/repositories/template"
	webhook_repo "ricrac/internal/repositories/webhook"
	workspace_repo "ricrac/internal/repositories/workspace"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i RepositoriesInterface -p repositories -s Repositories -o repositories_iface.go
type Repositories struct {
	db           *ent.Client
	base         *repository.BaseRepository
	access       access_repo.AccessRepositoryInterface
	resolve      resolve_repo.ResolveRepositoryInterface
	events       events_repo.EventsRepositoryInterface
	label        label_repo.LabelRepositoryInterface
	issue        issue_repo.IssueRepositoryInterface
	seed         seed_repo.SeedRepositoryInterface
	comment      comment_repo.CommentRepositoryInterface
	attachment   attachment_repo.AttachmentRepositoryInterface
	automation   automation_repo.AutomationRepositoryInterface
	notification notification_repo.NotificationRepositoryInterface
	search       search_repo.SearchRepositoryInterface
	account      account_repo.AccountRepositoryInterface
	oauth        oauth_repo.OAuthRepositoryInterface
	workspace    workspace_repo.WorkspaceRepositoryInterface
	team         team_repo.TeamRepositoryInterface
	state        state_repo.StateRepositoryInterface
	email        email_repo.EmailRepositoryInterface
	cycle        cycle_repo.CycleRepositoryInterface
	document     document_repo.DocumentRepositoryInterface
	favorite     favorite_repo.FavoriteRepositoryInterface
	template     template_repo.TemplateRepositoryInterface
	customer     customer_repo.CustomerRepositoryInterface
	release      release_repo.ReleaseRepositoryInterface
	webhook      webhook_repo.WebhookRepositoryInterface
}

func NewRepositories(db *ent.Client) *Repositories {
	return &Repositories{
		db:           db,
		base:         repository.NewBaseRepository(db),
		access:       access_repo.NewAccessRepository(db),
		resolve:      resolve_repo.NewResolveRepository(db),
		events:       events_repo.NewEventsRepository(db),
		label:        label_repo.NewLabelRepository(db),
		issue:        issue_repo.NewIssueRepository(db),
		seed:         seed_repo.NewSeedRepository(db),
		comment:      comment_repo.NewCommentRepository(db),
		attachment:   attachment_repo.NewAttachmentRepository(db),
		automation:   automation_repo.NewAutomationRepository(db),
		notification: notification_repo.NewNotificationRepository(db),
		search:       search_repo.NewSearchRepository(db),
		account:      account_repo.NewAccountRepository(db),
		oauth:        oauth_repo.NewOAuthRepository(db),
		workspace:    workspace_repo.NewWorkspaceRepository(db),
		team:         team_repo.NewTeamRepository(db),
		state:        state_repo.NewStateRepository(db),
		email:        email_repo.NewEmailRepository(db),
		cycle:        cycle_repo.NewCycleRepository(db),
		document:     document_repo.NewDocumentRepository(db),
		favorite:     favorite_repo.NewFavoriteRepository(db),
		template:     template_repo.NewTemplateRepository(db),
		customer:     customer_repo.NewCustomerRepository(db),
		release:      release_repo.NewReleaseRepository(db),
		webhook:      webhook_repo.NewWebhookRepository(db),
	}
}

func (r *Repositories) Ent() *ent.Client {
	return r.db
}

func (r *Repositories) WithTx(ctx context.Context, fn func(tx repository.TxInterface) error) error {
	return r.base.WithTx(ctx, fn)
}

// Now is Postgres' now() in tx: the transaction's start, so what it writes shares one time.
func (r *Repositories) Now(ctx context.Context, tx repository.TxInterface) (time.Time, error) {
	return r.base.Now(ctx, tx)
}

func (r *Repositories) Access() access_repo.AccessRepositoryInterface {
	return r.access
}

func (r *Repositories) Resolve() resolve_repo.ResolveRepositoryInterface {
	return r.resolve
}

func (r *Repositories) Events() events_repo.EventsRepositoryInterface {
	return r.events
}

func (r *Repositories) Label() label_repo.LabelRepositoryInterface {
	return r.label
}

func (r *Repositories) Issue() issue_repo.IssueRepositoryInterface {
	return r.issue
}

func (r *Repositories) Seed() seed_repo.SeedRepositoryInterface {
	return r.seed
}

func (r *Repositories) Comment() comment_repo.CommentRepositoryInterface {
	return r.comment
}

func (r *Repositories) Attachment() attachment_repo.AttachmentRepositoryInterface {
	return r.attachment
}

func (r *Repositories) Automation() automation_repo.AutomationRepositoryInterface {
	return r.automation
}

func (r *Repositories) Notification() notification_repo.NotificationRepositoryInterface {
	return r.notification
}

func (r *Repositories) Search() search_repo.SearchRepositoryInterface {
	return r.search
}

func (r *Repositories) Account() account_repo.AccountRepositoryInterface {
	return r.account
}

func (r *Repositories) OAuth() oauth_repo.OAuthRepositoryInterface {
	return r.oauth
}

func (r *Repositories) Workspace() workspace_repo.WorkspaceRepositoryInterface {
	return r.workspace
}

func (r *Repositories) Team() team_repo.TeamRepositoryInterface {
	return r.team
}

func (r *Repositories) State() state_repo.StateRepositoryInterface {
	return r.state
}

func (r *Repositories) Email() email_repo.EmailRepositoryInterface {
	return r.email
}

func (r *Repositories) Cycle() cycle_repo.CycleRepositoryInterface {
	return r.cycle
}

func (r *Repositories) Document() document_repo.DocumentRepositoryInterface {
	return r.document
}

func (r *Repositories) Favorite() favorite_repo.FavoriteRepositoryInterface {
	return r.favorite
}

func (r *Repositories) Template() template_repo.TemplateRepositoryInterface {
	return r.template
}

func (r *Repositories) Customer() customer_repo.CustomerRepositoryInterface {
	return r.customer
}

func (r *Repositories) Release() release_repo.ReleaseRepositoryInterface {
	return r.release
}

func (r *Repositories) Webhook() webhook_repo.WebhookRepositoryInterface {
	return r.webhook
}
