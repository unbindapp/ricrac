package issue_service

import (
	"slices"

	"ricrac/internal/common/errdefs"
)

var fieldsHiddenFromVisitors = []string{"email", "project", "projectMilestone", "cycle", "subscribers"}

// checkVisitorFilters refuses what visitors of public workspaces may not filter by: people by
// name or email (their errors list members), trashed issues and the areas only members see.
func checkVisitorFilters(input *ListInput) error {
	named := []*string{input.Assignee, input.Creator, input.Subscriber, input.Project, input.Milestone, input.Cycle, input.Release, input.CustomView}
	given := slices.ContainsFunc(named, func(value *string) bool { return value != nil })
	if !given && !input.IncludeTrashed && !mentionsHiddenField(input.Filter) {
		return nil
	}
	return errdefs.Forbidden("Sign in to filter by people, projects, milestones, cycles, releases or views, or to see trashed issues")
}

func mentionsHiddenField(filter any) bool {
	switch value := filter.(type) {
	case map[string]any:
		for key, child := range value {
			if slices.Contains(fieldsHiddenFromVisitors, key) || mentionsHiddenField(child) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(value, mentionsHiddenField)
	}
	return false
}
