// Package server holds what the handlers call: the configuration and the services.
package server

import (
	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/opcall"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	attachment_service "ricrac/internal/services/attachment"
	automation_service "ricrac/internal/services/automation"
	comment_service "ricrac/internal/services/comment"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	notification_service "ricrac/internal/services/notification"
	resolve_service "ricrac/internal/services/resolve"
	search_service "ricrac/internal/services/search"
	seed_service "ricrac/internal/services/seed"
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
}
