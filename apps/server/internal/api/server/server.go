// Package server holds what the handlers call: the configuration and the services.
package server

import (
	"ricrac/config"
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

type Server struct {
	Cfg        config.Config
	Keys       auth.Keys
	Repository repositories.RepositoriesInterface
	// Runs operations by name, once the router registered them.
	Ops *opcall.Late

	AccessService     *access_service.AccessService
	ResolveService    *resolve_service.ResolveService
	EventsService     *events_service.EventsService
	SeedService       *seed_service.SeedService
	LabelService      *label_service.LabelService
	IssueService      *issue_service.IssueService
	CommentService    *comment_service.CommentService
	AttachmentService *attachment_service.AttachmentService
	CycleService      *cycle_service.CycleService
	DocumentService   *document_service.DocumentService
	FavoriteService   *favorite_service.FavoriteService
	TemplateService   *template_service.TemplateService
}
