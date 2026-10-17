// Package state_service is the workflow states (issue statuses) of a team: listing, creating,
// changing and archiving them.
package state_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	state_repo "ricrac/internal/repositories/state"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	resolve_service "ricrac/internal/services/resolve"
	team_service "ricrac/internal/services/team"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i StateServiceInterface -p state_service -s StateService -o state_service_iface.go
type StateService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
	issues  issue_service.IssueServiceInterface
	teams   team_service.TeamServiceInterface
}

func NewStateService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, issues issue_service.IssueServiceInterface, teams team_service.TeamServiceInterface) *StateService {
	return &StateService{repo: repo, resolve: resolve, events: events, issues: issues, teams: teams}
}

var stateTypes = []string{"triage", "backlog", "unstarted", "started", "completed", "canceled"}

func defaultColor(kind string) string {
	switch kind {
	case "triage":
		return "#fc7840"
	case "backlog":
		return "#bec2c8"
	case "unstarted":
		return "#e2e2e2"
	case "started":
		return "#f2c94c"
	case "completed":
		return "#5e6ad2"
	}
	return "#95a2b3"
}

func (self *StateService) values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.IssueStatus, error) {
	states, err := self.repo.State().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.State().IssueCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.IssueStatus, len(states))
	for i, s := range states {
		values[i] = models.IssueStatusOf(s, counts[s.ID])
	}
	return values, nil
}

func (self *StateService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.IssueStatus, error) {
	values, err := self.values(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.IssueStatus{}, err
	}
	if len(values) == 0 {
		return models.IssueStatus{}, errdefs.NotFound("No such status")
	}
	return values[0], nil
}

// find is a status the caller can see, by id, or by name or type with its team, and its team.
// Archived statuses are found by id.
func (self *StateService) find(ctx context.Context, actor *auth.Actor, value string, team *string) (uuid.UUID, uuid.UUID, error) {
	if id, ok := utils.AsUUID(value); ok {
		teamOf, found, err := self.repo.State().TeamOf(ctx, actor.Workspace, id)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if !found {
			return uuid.Nil, uuid.Nil, errdefs.NotFound("No status %s", id)
		}
		visible, err := actor.CanSeeTeam(ctx, teamOf)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if !visible {
			return uuid.Nil, uuid.Nil, errdefs.NotFound("No status %s", id)
		}
		return id, teamOf, nil
	}
	if team == nil {
		return uuid.Nil, uuid.Nil, errdefs.BadRequest("Name the team too, or pass the status's id")
	}
	teamID, err := self.resolve.Team(ctx, actor, *team)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	state, err := self.teams.FindState(ctx, nil, teamID, value)
	return state, teamID, err
}

func checkType(kind string) (string, error) {
	kind = strings.TrimSpace(kind)
	if !slices.Contains(stateTypes, kind) {
		return "", errdefs.BadRequest("%q isn't a status type; use triage, backlog, unstarted, started, completed or canceled", kind)
	}
	return kind, nil
}

// checkWorkflow checks that a team keeps one open, one completed and one canceled status, and
// that its default and auto-close statuses fit their purpose.
func (self *StateService) checkWorkflow(ctx context.Context, tx repository.TxInterface, team uuid.UUID) error {
	states, err := self.repo.State().Active(ctx, tx, team)
	if err != nil {
		return err
	}
	var open, completed, canceled int
	for _, s := range states {
		switch s.Type {
		case "completed":
			completed++
		case "canceled":
			canceled++
		default:
			open++
		}
	}
	t, err := self.repo.Team().Get(ctx, tx, team)
	if err != nil {
		return err
	}
	defaultType, err := self.typeOf(ctx, tx, t.DefaultIssueStateID)
	if err != nil {
		return err
	}
	autoCloseType, err := self.typeOf(ctx, tx, t.AutoCloseStateID)
	if err != nil {
		return err
	}
	switch {
	case open == 0:
		return errdefs.BadRequest("A team needs at least one status that isn't completed or canceled")
	case completed == 0:
		return errdefs.BadRequest("A team needs at least one completed status")
	case canceled == 0:
		return errdefs.BadRequest("A team needs at least one canceled status")
	case defaultType != nil && *defaultType != "backlog" && *defaultType != "unstarted":
		return errdefs.BadRequest("The team's default status must stay a backlog or unstarted status")
	case autoCloseType != nil && *autoCloseType != "canceled":
		return errdefs.BadRequest("The team's auto-close status must stay a canceled status")
	}
	return nil
}

func (self *StateService) typeOf(ctx context.Context, tx repository.TxInterface, state *uuid.UUID) (*string, error) {
	if state == nil {
		return nil, nil
	}
	kind, err := self.issues.StateType(ctx, tx, *state)
	if err != nil {
		return nil, err
	}
	return &kind, nil
}

type ListInput struct {
	Team            string `json:"team" doc:"Team key, name or id."`
	IncludeArchived bool   `json:"includeArchived,omitempty" doc:"Also list archived statuses."`
	args.Page
}

func (self *StateService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.IssueStatus], error) {
	team, err := self.resolve.Team(ctx, actor, input.Team)
	if err != nil {
		return args.PageOf[models.IssueStatus]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.IssueStatus]{}, err
	}
	ids, err := self.repo.State().List(ctx, state_repo.ListFilter{
		Team: team, IncludeArchived: input.IncludeArchived, Limit: input.Size() + 1, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.IssueStatus]{}, err
	}
	values, err := self.values(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.IssueStatus]{}, err
	}
	return args.WrapOf(input.Page, values)
}

type GetInput struct {
	ID   string  `json:"id" doc:"The status's id, or its name or type together with team."`
	Team *string `json:"team" doc:"Team key, name or id, when id is a name or type."`
}

func (self *StateService) Get(ctx context.Context, actor *auth.Actor, input *GetInput) (models.IssueStatus, error) {
	state, _, err := self.find(ctx, actor, input.ID, input.Team)
	if err != nil {
		return models.IssueStatus{}, err
	}
	return self.value(ctx, nil, state)
}
