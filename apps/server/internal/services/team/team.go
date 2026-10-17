// Package team_service is teams: their settings (estimates, cycles, triage, automations),
// members, sub-teams, archiving and the triage rotation.
package team_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	resolve_repo "ricrac/internal/repositories/resolve"
	team_repo "ricrac/internal/repositories/team"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	resolve_service "ricrac/internal/services/resolve"
	seed_service "ricrac/internal/services/seed"
	workspace_service "ricrac/internal/services/workspace"
)

type M = map[string]any

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i TeamServiceInterface -p team_service -s TeamService -o team_service_iface.go
type TeamService struct {
	cfg        config.Config
	repo       repositories.RepositoriesInterface
	resolve    resolve_service.ResolveServiceInterface
	events     events_service.EventsServiceInterface
	issues     issue_service.IssueServiceInterface
	seed       seed_service.SeedServiceInterface
	workspaces workspace_service.WorkspaceServiceInterface
}

func NewTeamService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, issues issue_service.IssueServiceInterface, seed seed_service.SeedServiceInterface, workspaces workspace_service.WorkspaceServiceInterface) *TeamService {
	return &TeamService{cfg: cfg, repo: repo, resolve: resolve, events: events, issues: issues, seed: seed, workspaces: workspaces}
}

type IDInput struct {
	ID string `json:"id"`
}

func (self *TeamService) stats(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, ids []uuid.UUID) (map[uuid.UUID]models.TeamStats, error) {
	members, err := self.repo.Team().MemberCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	mine, err := self.repo.Team().MemberOf(ctx, tx, ids, actor.User)
	if err != nil {
		return nil, err
	}
	issues, err := self.repo.Team().OpenIssueCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	stats := make(map[uuid.UUID]models.TeamStats, len(ids))
	for _, id := range ids {
		stats[id] = models.TeamStats{IsMember: mine[id], MemberCount: members[id], IssueCount: issues[id]}
	}
	return stats, nil
}

// Value is everything about one team, as get_team answers, read in tx.
func (self *TeamService) Value(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.TeamDetail, error) {
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.TeamDetail{}, err
	}
	t, err := self.repo.Team().LoadDetail(ctx, tx, id, visible)
	if ent.IsNotFound(err) {
		return models.TeamDetail{}, errdefs.NotFound("No such team")
	}
	if err != nil {
		return models.TeamDetail{}, err
	}
	stats, err := self.stats(ctx, tx, actor, []uuid.UUID{id})
	if err != nil {
		return models.TeamDetail{}, err
	}
	labels, err := self.repo.Team().LabelCount(ctx, tx, id)
	if err != nil {
		return models.TeamDetail{}, err
	}
	everyone, err := self.repo.Team().Users(ctx, tx, t.TriageResponsibleIds)
	if err != nil {
		return models.TeamDetail{}, err
	}
	action, responsible, err := self.issues.TriageResponsible(ctx, tx, id)
	if err != nil {
		return models.TeamDetail{}, err
	}
	onDuty, err := self.repo.Team().Users(ctx, tx, responsible)
	if err != nil {
		return models.TeamDetail{}, err
	}
	allowed := issue_service.AllowedEstimates(t.EstimateType, t.EstimateAllowZero, t.EstimateExtended)
	return models.TeamDetailOf(t, self.cfg.PublicURL, stats[id], labels, allowed, everyone, action, onDuty), nil
}

type ListInput struct {
	Query           *string `json:"query" doc:"Part of a team's name or key."`
	IncludeArchived bool    `json:"includeArchived,omitempty" doc:"Also list archived teams."`
	Mine            bool    `json:"mine,omitempty" doc:"Only teams you are a member of."`
	args.Page
}

func (self *TeamService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Team], error) {
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Team]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Team]{}, err
	}
	filter := team_repo.ListFilter{
		Teams: visible, IncludeArchived: input.IncludeArchived, Pattern: utils.LikePattern(input.Query),
		Limit: input.Size() + 1, Offset: offset,
	}
	if input.Mine {
		filter.Member = &actor.User
	}
	ids, err := self.repo.Team().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.Team]{}, err
	}
	teams, err := self.repo.Team().Load(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.Team]{}, err
	}
	stats, err := self.stats(ctx, nil, actor, ids)
	if err != nil {
		return args.PageOf[models.Team]{}, err
	}
	values := make([]models.Team, len(teams))
	for i, t := range teams {
		values[i] = models.TeamOf(t, self.cfg.PublicURL, stats[t.ID])
	}
	return args.WrapOf(input.Page, values)
}

func (self *TeamService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.TeamDetail, error) {
	team, err := self.resolve.Team(ctx, actor, input.ID)
	if err != nil {
		return models.TeamDetail{}, err
	}
	return self.Value(ctx, nil, actor, team)
}

// RequireCanEdit lets workspace admins, the team's owners and members, and for a public team any
// member of the workspace change a team, its statuses and its settings. Guests may not.
func (self *TeamService) RequireCanEdit(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID) error {
	if err := actor.RequireMember(); err != nil {
		return err
	}
	if actor.Role >= auth.Admin {
		return nil
	}
	m, err := self.repo.Team().Membership(ctx, tx, team, actor.User)
	if err != nil {
		return err
	}
	if m.Owner || m.Member || !m.Private {
		return nil
	}
	return errdefs.Forbidden("Only members of this private team can change it")
}

// FindState is an active status of the team by id, name or type (the first of that type), read
// in tx so that statuses created in the same transaction are found.
func (self *TeamService) FindState(ctx context.Context, tx repository.TxInterface, team uuid.UUID, value string) (uuid.UUID, error) {
	states, err := self.repo.State().Active(ctx, tx, team)
	if err != nil {
		return uuid.Nil, err
	}
	rows := make([]resolve_repo.Typed, len(states))
	for i, s := range states {
		rows[i] = resolve_repo.Typed{ID: s.ID, Name: s.Name, Type: s.Type}
	}
	value = strings.TrimSpace(value)
	if id, found := resolve_service.FindTyped(rows, value); found {
		return id, nil
	}
	return uuid.Nil, errdefs.BadRequest("The team has no status %q. Options: %s.", value, strings.Join(resolve_service.TypedNames(rows), ", "))
}

// findTeam is a team by id, key or name; admins also find private teams they aren't in.
func (self *TeamService) findTeam(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	visible, err := self.resolve.Team(ctx, actor, value)
	if err == nil || actor.Role < auth.Admin {
		return visible, err
	}
	found, ok, findErr := self.repo.Team().FindUnarchived(ctx, actor.Workspace, strings.TrimSpace(value))
	if findErr != nil {
		return uuid.Nil, findErr
	}
	if !ok {
		return visible, err
	}
	return found, nil
}

func inRange(value, low, high int, what string) (int, error) {
	if value < low || value > high {
		return 0, errdefs.BadRequest("%s is %d to %d", what, low, high)
	}
	return value, nil
}
