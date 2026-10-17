// Package server holds what the handlers call: the configuration and the services.
package server

import (
	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/opcall"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	account_service "ricrac/internal/services/account"
	attachment_service "ricrac/internal/services/attachment"
	automation_service "ricrac/internal/services/automation"
	comment_service "ricrac/internal/services/comment"
	customer_service "ricrac/internal/services/customer"
	cycle_service "ricrac/internal/services/cycle"
	document_service "ricrac/internal/services/document"
	email_service "ricrac/internal/services/email"
	events_service "ricrac/internal/services/events"
	favorite_service "ricrac/internal/services/favorite"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	notification_service "ricrac/internal/services/notification"
	oauth_service "ricrac/internal/services/oauth"
	release_service "ricrac/internal/services/release"
	resolve_service "ricrac/internal/services/resolve"
	search_service "ricrac/internal/services/search"
	seed_service "ricrac/internal/services/seed"
	signin_service "ricrac/internal/services/signin"
	state_service "ricrac/internal/services/state"
	team_service "ricrac/internal/services/team"
	template_service "ricrac/internal/services/template"
	webhook_service "ricrac/internal/services/webhook"
	workspace_service "ricrac/internal/services/workspace"
)

type Server struct {
	Cfg        config.Config
	Keys       auth.Keys
	Repository repositories.RepositoriesInterface
	// Runs operations by name, once the router registered them.
	Ops *opcall.Late

	AccessService       *access_service.AccessService
	ResolveService      *resolve_service.ResolveService
	EventsService       *events_service.EventsService
	SeedService         *seed_service.SeedService
	LabelService        *label_service.LabelService
	IssueService        *issue_service.IssueService
	CommentService      *comment_service.CommentService
	AttachmentService   *attachment_service.AttachmentService
	AutomationService   *automation_service.AutomationService
	NotificationService *notification_service.NotificationService
	SearchService       *search_service.SearchService
	AccountService      *account_service.AccountService
	OAuthService        *oauth_service.OAuthService
	SigninService       *signin_service.SigninService
	EmailService        *email_service.EmailService
	WorkspaceService    *workspace_service.WorkspaceService
	TeamService         *team_service.TeamService
	StateService        *state_service.StateService
	CycleService        *cycle_service.CycleService
	DocumentService     *document_service.DocumentService
	FavoriteService     *favorite_service.FavoriteService
	TemplateService     *template_service.TemplateService
	CustomerService     *customer_service.CustomerService
	ReleaseService      *release_service.ReleaseService
	WebhookService      *webhook_service.WebhookService
}
