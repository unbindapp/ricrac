package issue_service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	issue_repo "ricrac/internal/repositories/issue"
)

func eq(value any) M {
	return M{"eq": value}
}

// isNullWord says whether a reference means none.
func isNullWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "null", "none", "unassigned", "no project":
		return true
	}
	return false
}

// simpleFilters is the list arguments as filter parts.
func (self *IssueService) simpleFilters(ctx context.Context, actor *auth.Actor, input *ListInput) ([]any, error) {
	var parts []any
	var team *uuid.UUID
	if input.Team != nil {
		id, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return nil, err
		}
		team = &id
		if input.IncludeSubTeams {
			ids, err := self.repo.Issue().SubTeams(ctx, id)
			if err != nil {
				return nil, err
			}
			list := make([]any, len(ids))
			for i, id := range ids {
				list[i] = id.String()
			}
			parts = append(parts, M{"team": M{"id": M{"in": list}}})
		} else {
			parts = append(parts, M{"team": M{"id": eq(id.String())}})
		}
	}
	userPart := func(value *string, wrap func(M) M) error {
		if value == nil {
			return nil
		}
		user, err := self.resolve.User(ctx, actor, *value)
		if err != nil {
			return err
		}
		parts = append(parts, wrap(M{"id": eq(user.String())}))
		return nil
	}
	if input.Assignee != nil && isNullWord(*input.Assignee) {
		parts = append(parts, M{"assignee": M{"null": true}})
	} else if err := userPart(input.Assignee, func(f M) M { return M{"assignee": f} }); err != nil {
		return nil, err
	}
	if err := userPart(input.Creator, func(f M) M { return M{"creator": f} }); err != nil {
		return nil, err
	}
	if err := userPart(input.Subscriber, func(f M) M { return M{"subscribers": M{"some": f}} }); err != nil {
		return nil, err
	}
	if input.State != nil {
		value := *input.State
		id, isID := utils.AsUUID(value)
		switch {
		case team != nil:
			state, err := self.resolve.State(ctx, actor, *team, value)
			if err != nil {
				return nil, err
			}
			parts = append(parts, M{"state": M{"id": eq(state.String())}})
		case isID:
			parts = append(parts, M{"state": M{"id": eq(id.String())}})
		default:
			parts = append(parts, M{"or": []any{M{"state": M{"name": eq(value)}}, M{"state": M{"type": eq(value)}}}})
		}
	}
	if input.Label != nil {
		value := *input.Label
		if id, ok := utils.AsUUID(value); ok {
			parts = append(parts, M{"labels": M{"some": M{"id": eq(id.String())}}})
		} else {
			if _, err := self.resolve.Labels(ctx, actor, team, []string{value}); err != nil {
				return nil, err
			}
			group, name := utils.SplitGroup(value)
			if group != nil {
				parts = append(parts, M{"labels": M{"some": M{"name": eq(name), "parent": M{"name": eq(*group)}}}})
			} else {
				parts = append(parts, M{"labels": M{"some": M{"name": eq(name)}}})
			}
		}
	}
	var project *uuid.UUID
	if input.Project != nil {
		if isNullWord(*input.Project) {
			parts = append(parts, M{"project": M{"null": true}})
		} else {
			id, err := self.resolve.Project(ctx, actor, *input.Project)
			if err != nil {
				return nil, err
			}
			project = &id
			parts = append(parts, M{"project": M{"id": eq(id.String())}})
		}
	}
	if input.Milestone != nil {
		id, err := self.resolve.Milestone(ctx, actor, project, *input.Milestone)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"projectMilestone": M{"id": eq(id.String())}})
	}
	if input.Cycle != nil {
		value := strings.TrimSpace(*input.Cycle)
		lower := strings.ToLower(value)
		_, isID := utils.AsUUID(value)
		number, numErr := strconv.ParseInt(value, 10, 64)
		switch {
		case team != nil:
			id, err := self.resolve.Cycle(ctx, actor, team, value)
			if err != nil {
				return nil, err
			}
			parts = append(parts, M{"cycle": M{"id": eq(id.String())}})
		case lower == "current" || lower == "active":
			parts = append(parts, M{"cycle": M{"isActive": eq(true)}})
		case isID:
			parts = append(parts, M{"cycle": M{"id": eq(value)}})
		case numErr == nil:
			parts = append(parts, M{"cycle": M{"number": eq(float64(number))}})
		default:
			parts = append(parts, M{"cycle": M{"name": eq(*input.Cycle)}})
		}
	}
	if input.Release != nil {
		id, err := self.resolve.Release(ctx, actor, *input.Release)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"releases": M{"some": M{"id": eq(id.String())}}})
	}
	if input.ParentID != nil {
		id, err := self.resolve.Issue(ctx, actor, *input.ParentID)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"parent": M{"id": eq(id.String())}})
	}
	if input.Priority != nil {
		priority, err := utils.CheckPriority(*input.Priority)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"priority": eq(float64(priority))})
	}
	for _, f := range []struct {
		field string
		value *string
		op    string
	}{{"createdAt", input.CreatedAt, "gte"}, {"updatedAt", input.UpdatedAt, "gte"}, {"dueDate", input.DueBefore, "lte"}} {
		if f.value == nil {
			continue
		}
		at, err := utils.ParseDatetime(*f.value)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{f.field: M{f.op: at.Format(time.RFC3339Nano)}})
	}
	return parts, nil
}

func (self *IssueService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Issue], error) {
	parts, err := self.simpleFilters(ctx, actor, input)
	if err != nil {
		return args.PageOf[models.Issue]{}, err
	}
	if input.CustomView != nil {
		viewParts, err := self.viewFilter(ctx, actor, *input.CustomView)
		if err != nil {
			return args.PageOf[models.Issue]{}, err
		}
		parts = append(parts, viewParts...)
	}
	if input.Filter != nil {
		parts = append(parts, input.Filter)
	}
	orderBy := "updatedAt"
	if input.OrderBy != nil {
		orderBy = *input.OrderBy
	}
	if _, ok := issue_repo.Orders[orderBy]; !ok {
		return args.PageOf[models.Issue]{}, errdefs.BadRequest("orderBy %q isn't one of updatedAt, createdAt, priority, dueDate, manual, identifier", orderBy)
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Issue]{}, err
	}
	filter, err := CompileFilter(M{"and": parts}, actor.User)
	if err != nil {
		return args.PageOf[models.Issue]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Issue]{}, err
	}
	ids, err := self.repo.Issue().List(ctx, issue_repo.ListQuery{
		Workspace: actor.Workspace, Teams: teams, Viewer: actor.User, IncludeArchived: input.IncludeArchived,
		IncludeTrashed: input.IncludeTrashed, Search: strings.TrimSpace(utils.OrEmpty(input.Query)), Filter: filter,
		OrderBy: orderBy, Limit: input.Size() + 1, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.Issue]{}, err
	}
	issues, err := self.Values(ctx, nil, ids, input.IncludeDescription)
	if err != nil {
		return args.PageOf[models.Issue]{}, err
	}
	return args.WrapOf(input.Page, issues)
}

// viewFilter is the filter parts of an issue view.
func (self *IssueService) viewFilter(ctx context.Context, actor *auth.Actor, reference string) ([]any, error) {
	id, err := self.resolve.CustomView(ctx, actor, reference)
	if err != nil {
		return nil, err
	}
	view, err := self.repo.Issue().View(ctx, id)
	if err != nil {
		return nil, err
	}
	if view.Model != "issue" {
		return nil, errdefs.BadRequest("That view shows %ss, not issues", view.Model)
	}
	var filter any
	if len(view.Filter) > 0 {
		if err := json.Unmarshal(view.Filter, &filter); err != nil {
			return nil, err
		}
	}
	parts := []any{filter}
	if view.TeamID != nil {
		parts = append(parts, M{"team": M{"id": eq(view.TeamID.String())}})
	}
	return parts, nil
}
