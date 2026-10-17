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
	customer_service "ricrac/internal/services/customer"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	release_service "ricrac/internal/services/release"
	resolve_service "ricrac/internal/services/resolve"
	seed_service "ricrac/internal/services/seed"
	webhook_service "ricrac/internal/services/webhook"
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
	CustomerService   *customer_service.CustomerService
	ReleaseService    *release_service.ReleaseService
	WebhookService    *webhook_service.WebhookService
}
