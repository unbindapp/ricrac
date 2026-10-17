package server

import (
	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	events_service "ricrac/internal/services/events"
	label_service "ricrac/internal/services/label"
	resolve_service "ricrac/internal/services/resolve"
)

// New wires the services on the database. waker wakes the background senders.
func New(cfg config.Config, db *ent.Client, waker events_service.Waker) *Server {
	repo := repositories.NewRepositories(db)
	srv := &Server{
		Cfg:            cfg,
		Keys:           auth.NewKeys(cfg),
		Repository:     repo,
		AccessService:  access_service.NewAccessService(cfg, repo),
		ResolveService: resolve_service.NewResolveService(repo),
		EventsService:  events_service.NewEventsService(repo, waker),
	}
	srv.LabelService = label_service.NewLabelService(repo, srv.ResolveService, srv.EventsService)
	return srv
}
