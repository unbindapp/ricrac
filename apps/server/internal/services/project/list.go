package project_service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	project_repo "ricrac/internal/repositories/project"
)

type ListInput struct {
	Team               *string `json:"team" doc:"Team key, name or id."`
	Status             *string `json:"status" doc:"Project status name, type (backlog, planned, started, paused, completed, canceled) or id."`
	Lead               *string `json:"lead" doc:"Lead (id, name, email or \"me\"), or \"null\" for projects without one."`
	Member             *string `json:"member" doc:"A project member (id, name, email or \"me\")."`
	Initiative         *string `json:"initiative" doc:"Initiative name, slug or id."`
	Label              *string `json:"label" doc:"Project label name or id."`
	Health             *string `json:"health" doc:"onTrack, atRisk or offTrack."`
	Priority           *int    `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Query              *string `json:"query" doc:"Words to find in the name, summary or description."`
	CustomView         *string `json:"customView" doc:"A saved project view (name, slug or id) whose filter applies too."`
	Filter             any     `json:"filter" doc:"Further conditions, as in project views. Fields: status {id,name,type}, lead {id,isMe}, team {id,key}, priority, health, name, startDate, targetDate, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, null; dates take ISO 8601 or a duration from now like P2W. Example: {\"status\": {\"type\": {\"in\": [\"planned\", \"started\"]}}, \"targetDate\": {\"lt\": \"P1M\"}}"`
	IncludeArchived    bool    `json:"includeArchived,omitempty"`
	IncludeTrashed     bool    `json:"includeTrashed,omitempty"`
	IncludeDescription bool    `json:"includeDescription,omitempty" doc:"Include each project's description."`
	OrderBy            *string `json:"orderBy" doc:"updatedAt (default), createdAt, targetDate, priority or manual."`
	Limit              *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor             *string `json:"cursor" doc:"The endCursor of the previous page."`
}

var projectStatusTypes = []string{"backlog", "planned", "started", "paused", "completed", "canceled"}

func projectRefs(projects []*ent.Project) []models.ProjectRef {
	refs := make([]models.ProjectRef, len(projects))
	for i, p := range projects {
		refs[i] = *models.ProjectRefOf(p)
	}
	return refs
}

func (self *ProjectService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Project], error) {
	none := args.PageOf[models.Project]{}
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	orderBy := "updatedAt"
	if input.OrderBy != nil {
		orderBy = *input.OrderBy
	}
	if !project_repo.IsListOrder(orderBy) {
		return none, errdefs.BadRequest("orderBy \"%s\" isn't one of updatedAt, createdAt, targetDate, priority, manual", orderBy)
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return none, err
	}
	filter := project_repo.ListFilter{
		Workspace: actor.Workspace, Teams: teams, IncludeArchived: input.IncludeArchived, IncludeTrashed: input.IncludeTrashed,
		OrderBy: orderBy,
	}
	if input.Team != nil {
		team, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return none, err
		}
		filter.Team = &team
	}
	if input.Status != nil {
		lower := strings.ToLower(strings.TrimSpace(*input.Status))
		if slices.Contains(projectStatusTypes, lower) {
			filter.StatusType = &lower
		} else {
			status, err := self.resolve.ProjectStatus(ctx, actor, *input.Status)
			if err != nil {
				return none, err
			}
			filter.Status = &status
		}
	}
	if input.Lead != nil {
		if utils.IsNoneWord(*input.Lead) {
			filter.WithoutLead = true
		} else {
			lead, err := self.resolve.User(ctx, actor, *input.Lead)
			if err != nil {
				return none, err
			}
			filter.Lead = &lead
		}
	}
	if input.Member != nil {
		member, err := self.resolve.User(ctx, actor, *input.Member)
		if err != nil {
			return none, err
		}
		filter.Member = &member
	}
	if input.Initiative != nil {
		initiative, err := self.resolve.Initiative(ctx, actor, *input.Initiative)
		if err != nil {
			return none, err
		}
		filter.Initiative = &initiative
	}
	if input.Label != nil {
		if filter.Labels, err = self.resolve.ProjectLabels(ctx, actor, []string{*input.Label}); err != nil {
			return none, err
		}
	}
	if input.Health != nil {
		health, err := utils.CheckHealth(*input.Health)
		if err != nil {
			return none, err
		}
		filter.Health = &health
	}
	if input.Priority != nil {
		priority, err := utils.CheckPriority(*input.Priority)
		if err != nil {
			return none, err
		}
		filter.Priority = &priority
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Query = &query
	}
	if input.CustomView != nil {
		conditions, err := self.viewConditions(ctx, actor, *input.CustomView)
		if err != nil {
			return none, err
		}
		filter.Conditions = append(filter.Conditions, conditions...)
	}
	if input.Filter != nil {
		condition, err := project_repo.Filter(input.Filter, actor.User)
		if err != nil {
			return none, err
		}
		filter.Conditions = append(filter.Conditions, condition)
	}
	if filter.Offset, err = page.Offset(); err != nil {
		return none, err
	}
	filter.Limit = page.Size() + 1
	ids, err := self.repo.Project().List(ctx, filter)
	if err != nil {
		return none, err
	}
	return self.list(ctx, actor, page, ids, input.IncludeDescription)
}

// viewConditions is what a project view keeps: its filter and its team.
func (self *ProjectService) viewConditions(ctx context.Context, actor *auth.Actor, reference string) ([]project_repo.Condition, error) {
	id, err := self.resolve.CustomView(ctx, actor, reference)
	if err != nil {
		return nil, err
	}
	view, err := self.repo.Project().View(ctx, id)
	if err != nil {
		return nil, err
	}
	if view.Model != "project" {
		return nil, errdefs.BadRequest("That view shows %ss, not projects", view.Model)
	}
	var stored any
	if len(view.Filter) > 0 {
		if err := json.Unmarshal(view.Filter, &stored); err != nil {
			return nil, err
		}
	}
	condition, err := project_repo.Filter(stored, actor.User)
	if err != nil {
		return nil, err
	}
	conditions := []project_repo.Condition{condition}
	if view.TeamID != nil {
		conditions = append(conditions, project_repo.Condition{
			SQL:  "EXISTS (SELECT 1 FROM project_teams vw WHERE vw.project_id = projects.id AND vw.team_id = ?)",
			Args: []any{*view.TeamID},
		})
	}
	return conditions, nil
}
