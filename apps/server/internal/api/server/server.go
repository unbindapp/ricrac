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
	events_service "ricrac/internal/services/events"
	initiative_service "ricrac/internal/services/initiative"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	milestone_service "ricrac/internal/services/milestone"
	project_service "ricrac/internal/services/project"
	resolve_service "ricrac/internal/services/resolve"
	seed_service "ricrac/internal/services/seed"
	statusupdate_service "ricrac/internal/services/statusupdate"
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
	ProjectService      *project_service.ProjectService
	MilestoneService    *milestone_service.MilestoneService
	StatusUpdateService *statusupdate_service.StatusUpdateService
	InitiativeService   *initiative_service.InitiativeService
}
