package server

import (
	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
)

// New wires the services on the database.
func New(cfg config.Config, db *ent.Client) *Server {
	repo := repositories.NewRepositories(db)
	return &Server{
		Cfg:           cfg,
		Keys:          auth.NewKeys(cfg),
		Repository:    repo,
		AccessService: access_service.NewAccessService(cfg, repo),
	}
}
