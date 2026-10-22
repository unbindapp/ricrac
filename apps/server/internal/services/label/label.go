// Package label_service is issue labels: workspace labels and team labels, label groups,
// retiring and deleting them.
package label_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i LabelServiceInterface -p label_service -s LabelService -o label_service_iface.go
type LabelService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewLabelService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *LabelService {
	return &LabelService{repo: repo, resolve: resolve, events: events}
}

type ListInput struct {
	Team           *string `json:"team" doc:"Optional team key, name or id: its labels and the workspace labels. All labels by default."`
	IncludeRetired bool    `json:"includeRetired,omitempty" doc:"Also list retired labels."`
	Query          *string `json:"query" doc:"Part of a label's name."`
	args.Page
}

type LabelInput struct {
	ID   string  `json:"id" doc:"The label's id or name (Group/Label for a label in a group)."`
	Team *string `json:"team" doc:"Optional team key, name or id, to pick between labels of the same name."`
}

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to change a label: its id or name (Group/Label for a label in a group). Omit to create one."`
	Team        *string               `json:"team" doc:"Optional team key, name or id. On create, makes it a team label (omit for a workspace label); on update, picks between labels of the same name."`
	Name        *string               `json:"name" doc:"Required to create."`
	Color       *string               `json:"color" doc:"Hex color like #eb5757; picked for you on create when omitted."`
	Description args.Nullable[string] `json:"description" doc:"Null clears it."`
	IsGroup     *bool                 `json:"isGroup" doc:"A group holds labels and can't be put on issues itself."`
	GroupType   *string               `json:"groupType" doc:"For groups: singleSelect (an issue takes one of its labels, the default) or multiSelect."`
	Parent      args.Nullable[string] `json:"parent" doc:"The group (name or id) to put the label in; null takes it out of its group. A team label's group is a workspace group or a group of the same team."`
}

// value is the label as the API answers it, read in tx.
func (self *LabelService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.IssueLabel, error) {
	labels, err := self.values(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.IssueLabel{}, err
	}
	if len(labels) == 0 {
		return models.IssueLabel{}, errdefs.NotFound("No such label")
	}
	return labels[0], nil
}

func (self *LabelService) values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.IssueLabel, error) {
	labels, err := self.repo.Label().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.Label().IssueCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.IssueLabel, len(labels))
	for i, l := range labels {
		values[i] = models.IssueLabelOf(l, counts[l.ID])
	}
	return values, nil
}

// find is a label the caller can see (retired ones too) by id, name or Group/Name. With a team,
// its labels and the workspace labels are searched; a team's label wins a tie.
func (self *LabelService) find(ctx context.Context, actor *auth.Actor, value string, team *uuid.UUID) (uuid.UUID, error) {
	var teams []uuid.UUID
	if team != nil {
		teams = []uuid.UUID{*team}
	} else {
		var err error
		if teams, err = actor.TeamIDs(ctx); err != nil {
			return uuid.Nil, err
		}
	}
	rows, err := self.repo.Resolve().AllIssueLabels(ctx, actor.Workspace, teams)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, ok := asUUID(value); ok {
		for _, r := range rows {
			if r.ID == id || slices.Contains(r.MergedIDs, id) {
				return r.ID, nil
			}
		}
	}
	group, name := splitGroup(value)
	var matches, teamMatches []uuid.UUID
	for _, r := range rows {
		if !strings.EqualFold(r.Name, name) || group != nil && (r.Parent == nil || !strings.EqualFold(*r.Parent, *group)) {
			continue
		}
		matches = append(matches, r.ID)
		if r.TeamID != nil && (team == nil || *r.TeamID == *team) {
			teamMatches = append(teamMatches, r.ID)
		}
	}
	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(teamMatches) == 1:
		return teamMatches[0], nil
	case len(matches) == 0:
		var names []string
		for _, r := range rows[:min(len(rows), 30)] {
			if r.Parent != nil {
				names = append(names, *r.Parent+"/"+r.Name)
			} else {
				names = append(names, r.Name)
			}
		}
		return uuid.Nil, errdefs.BadRequest("No label %q. Options: %s.", value, strings.Join(names, ", "))
	}
	return uuid.Nil, errdefs.BadRequest("%q matches several labels; pass team, name it as Group/%s, or use its id", value, name)
}

// labelOf is the label the arguments name.
func (self *LabelService) labelOf(ctx context.Context, actor *auth.Actor, input *LabelInput) (uuid.UUID, error) {
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return uuid.Nil, err
	}
	return self.find(ctx, actor, input.ID, team)
}
