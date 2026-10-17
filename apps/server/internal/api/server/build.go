package server

import (
	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/opcall"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	attachment_service "ricrac/internal/services/attachment"
	comment_service "ricrac/internal/services/comment"
	cycle_service "ricrac/internal/services/cycle"
	document_service "ricrac/internal/services/document"
	events_service "ricrac/internal/services/events"
	favorite_service "ricrac/internal/services/favorite"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	resolve_service "ricrac/internal/services/resolve"
	seed_service "ricrac/internal/services/seed"
	template_service "ricrac/internal/services/template"
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
	srv.CycleService = cycle_service.NewCycleService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.DocumentService = document_service.NewDocumentService(cfg, repo, srv.ResolveService, srv.EventsService)
	srv.FavoriteService = favorite_service.NewFavoriteService(repo, srv.ResolveService)
	srv.TemplateService = template_service.NewTemplateService(repo, srv.ResolveService, srv.EventsService, srv.AccessService, srv.Ops)
	return srv
}
