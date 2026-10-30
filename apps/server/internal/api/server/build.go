package server

import (
	"ricrac/config"
	"ricrac/ent"
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
	discussion_service "ricrac/internal/services/discussion"
	document_service "ricrac/internal/services/document"
	email_service "ricrac/internal/services/email"
	events_service "ricrac/internal/services/events"
	favorite_service "ricrac/internal/services/favorite"
	github_service "ricrac/internal/services/github"
	initiative_service "ricrac/internal/services/initiative"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	milestone_service "ricrac/internal/services/milestone"
	notification_service "ricrac/internal/services/notification"
	oauth_service "ricrac/internal/services/oauth"
	project_service "ricrac/internal/services/project"
	release_service "ricrac/internal/services/release"
	resolve_service "ricrac/internal/services/resolve"
	search_service "ricrac/internal/services/search"
	seed_service "ricrac/internal/services/seed"
	signin_service "ricrac/internal/services/signin"
	state_service "ricrac/internal/services/state"
	statusupdate_service "ricrac/internal/services/statusupdate"
	team_service "ricrac/internal/services/team"
	template_service "ricrac/internal/services/template"
	view_service "ricrac/internal/services/view"
	webhook_service "ricrac/internal/services/webhook"
	workspace_service "ricrac/internal/services/workspace"
)

// New wires the services on the database. waker wakes the background senders.
func New(cfg config.Config, db *ent.Client, waker events_service.Waker) *Server {
	repo := repositories.NewRepositories(db)
	srv := &Server{
		Cfg:            cfg,
		Keys:           auth.NewKeys(cfg),
		Repository:     repo,
		Ops:            &opcall.Late{},
		AccessService:  access_service.NewAccessService(cfg, repo),
		ResolveService: resolve_service.NewResolveService(repo),
		EventsService:  events_service.NewEventsService(repo, waker),
	}
	srv.SeedService = seed_service.NewSeedService(repo)
	srv.LabelService = label_service.NewLabelService(repo, srv.ResolveService, srv.EventsService)
	srv.IssueService = issue_service.NewIssueService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.CommentService = comment_service.NewCommentService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.AttachmentService = attachment_service.NewAttachmentService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.AutomationService = automation_service.NewAutomationService(repo)
	srv.NotificationService = notification_service.NewNotificationService(repo, srv.ResolveService)
	srv.SearchService = search_service.NewSearchService(cfg, repo, srv.ResolveService)
	srv.AccountService = account_service.NewAccountService(cfg, repo, srv.AccessService, srv.SeedService)
	srv.OAuthService = oauth_service.NewOAuthService(cfg, repo, srv.AccountService)
	srv.EmailService = email_service.NewEmailService(cfg, repo)
	srv.SigninService = signin_service.NewSigninService(cfg, srv.Keys, repo, srv.EmailService)
	srv.WorkspaceService = workspace_service.NewWorkspaceService(cfg, repo, srv.ResolveService, srv.EventsService, srv.EmailService)
	srv.TeamService = team_service.NewTeamService(cfg, repo, srv.ResolveService, srv.EventsService, srv.IssueService, srv.SeedService, srv.WorkspaceService)
	srv.StateService = state_service.NewStateService(repo, srv.ResolveService, srv.EventsService, srv.IssueService, srv.TeamService)
	srv.CycleService = cycle_service.NewCycleService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.DocumentService = document_service.NewDocumentService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.DiscussionService = discussion_service.NewDiscussionService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.FavoriteService = favorite_service.NewFavoriteService(repo, srv.ResolveService)
	srv.TemplateService = template_service.NewTemplateService(repo, srv.ResolveService, srv.EventsService, srv.AccessService, srv.Ops)
	srv.CustomerService = customer_service.NewCustomerService(repo, srv.ResolveService, srv.EventsService)
	srv.ReleaseService = release_service.NewReleaseService(repo, srv.ResolveService, srv.EventsService)
	srv.WebhookService = webhook_service.NewWebhookService(repo, srv.ResolveService, srv.EventsService)
	srv.MilestoneService = milestone_service.NewMilestoneService(repo, srv.ResolveService, srv.EventsService)
	srv.StatusUpdateService = statusupdate_service.NewStatusUpdateService(repo, srv.ResolveService, srv.EventsService)
	srv.ProjectService = project_service.NewProjectService(cfg, repo, srv.ResolveService, srv.EventsService, srv.MilestoneService, srv.StatusUpdateService, srv.Ops)
	srv.InitiativeService = initiative_service.NewInitiativeService(cfg, repo, srv.ResolveService, srv.EventsService, srv.ProjectService, srv.StatusUpdateService)
	srv.IssueService.Uses(srv.AttachmentService, srv.CommentService)
	srv.ViewService = view_service.NewViewService(cfg, repo, srv.ResolveService, srv.EventsService, srv.IssueService)
	srv.GitHubService = github_service.NewGitHubService(cfg, srv.Keys, repo, srv.AccessService, srv.ResolveService, srv.EventsService,
		srv.IssueService, srv.CommentService, srv.AttachmentService, srv.MilestoneService, waker)
	return srv
}
