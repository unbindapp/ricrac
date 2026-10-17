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
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	notification_service "ricrac/internal/services/notification"
	oauth_service "ricrac/internal/services/oauth"
	resolve_service "ricrac/internal/services/resolve"
	search_service "ricrac/internal/services/search"
	seed_service "ricrac/internal/services/seed"
	signin_service "ricrac/internal/services/signin"
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
	srv.SigninService = signin_service.NewSigninService(cfg, srv.Keys, repo)
	return srv
}
