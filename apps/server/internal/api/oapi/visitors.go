package oapi

import "ricrac/internal/common/errdefs"

// hiddenFromVisitors is what visitors of public workspaces get in place of these fields,
// wherever an answer has them: no emails, and nothing of the areas only members see.
var hiddenFromVisitors = map[string]any{
	"email":             "",
	"project":           nil,
	"projectMilestone":  nil,
	"cycle":             nil,
	"currentCycle":      nil,
	"subscribers":       []any{},
	"sharedWith":        []any{},
	"releases":          []any{},
	"documents":         []any{},
	"members":           []any{},
	"triageResponsible": []any{},
	"responsible":       []any{},
}

func forVisitors(value any) any {
	plain := errdefs.Sorted(value)
	hide(plain)
	return plain
}

func hide(value any) {
	switch v := value.(type) {
	case M:
		for key, child := range v {
			if hidden, ok := hiddenFromVisitors[key]; ok {
				v[key] = hidden
				continue
			}
			if key == "changes" {
				dropHiddenChanges(child)
			}
			hide(child)
		}
	case []any:
		for _, child := range v {
			hide(child)
		}
	}
}

// dropHiddenChanges leaves changes to hidden fields out of an issue's history, rather than
// showing them as cleared.
func dropHiddenChanges(value any) {
	changes, ok := value.(M)
	if !ok {
		return
	}
	for key := range changes {
		if _, hidden := hiddenFromVisitors[key]; hidden {
			delete(changes, key)
		}
	}
}
