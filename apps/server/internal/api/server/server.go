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
	comment_service "ricrac/internal/services/comment"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	label_service "ricrac/internal/services/label"
	oauth_service "ricrac/internal/services/oauth"
	resolve_service "ricrac/internal/services/resolve"
	seed_service "ricrac/internal/services/seed"
	signin_service "ricrac/internal/services/signin"
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
	AccountService    *account_service.AccountService
	OAuthService      *oauth_service.OAuthService
	SigninService     *signin_service.SigninService
}
