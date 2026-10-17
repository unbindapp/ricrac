// Package server holds what the handlers call: the configuration and the services.
package server

import (
	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	events_service "ricrac/internal/services/events"
	label_service "ricrac/internal/services/label"
	resolve_service "ricrac/internal/services/resolve"
)

type Server struct {
	Cfg        config.Config
	Keys       auth.Keys
	Repository repositories.RepositoriesInterface

	AccessService  *access_service.AccessService
	ResolveService *resolve_service.ResolveService
	EventsService  *events_service.EventsService
	LabelService   *label_service.LabelService
}
