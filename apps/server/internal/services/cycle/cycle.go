// Package cycle_service is cycles: a team's time-boxed iterations. They are created ahead by the
// automations, which also complete ended cycles (rolling unfinished issues over) and record daily
// progress.
package cycle_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i CycleServiceInterface -p cycle_service -s CycleService -o cycle_service_iface.go
type CycleService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewCycleService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *CycleService {
	return &CycleService{cfg: cfg, repo: repo, resolve: resolve, events: events}
}

type ListInput struct {
	Team            *string `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id."`
	Type            *string `json:"type" doc:"current, previous, next, upcoming, completed or all (default)."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	args.Page
}

type GetInput struct {
	ID            string  `json:"id" doc:"The cycle's id, number, name or \"current\" / \"next\" / \"previous\"."`
	Team          *string `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id."`
	IncludeIssues *bool   `json:"includeIssues" doc:"List the cycle's issues (default true)."`
}

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to update a cycle: its id, number or name. Omit to create one."`
	Team        *string               `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id."`
	Name        args.Nullable[string] `json:"name" doc:"Null clears it (the cycle is then shown as \"Cycle <number>\")."`
	Description args.Nullable[string] `json:"description"`
	StartsAt    *string               `json:"startsAt" doc:"ISO 8601 date or time (a date starts at 00:00 UTC). Required to create."`
	EndsAt      *string               `json:"endsAt" doc:"ISO 8601 date or time, after startsAt. Required to create."`
}

type ArchiveInput struct {
	ID   string  `json:"id" doc:"The cycle's id, number or name."`
	Team *string `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id."`
}

func (self *CycleService) values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.Cycle, error) {
	cycles, err := self.repo.Cycle().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	scopes, err := self.repo.Cycle().Scopes(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return nil, err
	}
	values := make([]models.Cycle, len(cycles))
	for i, c := range cycles {
		values[i] = models.CycleOf(c, self.cfg.PublicURL, models.CycleScope(scopes[c.ID]), now)
	}
	return values, nil
}

// value is the cycle as the API answers it, read in tx.
func (self *CycleService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.Cycle, error) {
	values, err := self.values(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.Cycle{}, err
	}
	if len(values) == 0 {
		return models.Cycle{}, errdefs.NotFound("No such cycle")
	}
	return values[0], nil
}

func (self *CycleService) find(ctx context.Context, actor *auth.Actor, team *string, value string) (uuid.UUID, error) {
	teamID, err := self.resolve.OptionalTeam(ctx, actor, team)
	if err != nil {
		return uuid.Nil, err
	}
	return self.resolve.Cycle(ctx, actor, teamID, value)
}
