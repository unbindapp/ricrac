package template_service

import (
	"reflect"

	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	issue_service "ricrac/internal/services/issue"
)

type issueTemplateData struct {
	Title       *string                   `json:"title"`
	Description *string                   `json:"description"`
	Team        *string                   `json:"team"`
	State       *string                   `json:"state"`
	Priority    *int32                    `json:"priority"`
	Estimate    *float64                  `json:"estimate"`
	Assignee    *string                   `json:"assignee"`
	Labels      []string                  `json:"labels"`
	Project     *string                   `json:"project"`
	Cycle       *string                   `json:"cycle"`
	DueDate     *string                   `json:"dueDate"`
	SubIssues   []issue_service.SaveInput `json:"subIssues"`
}

type milestoneTemplateData struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	TargetDate  *string `json:"targetDate"`
}

type projectTemplateData struct {
	Name        *string                   `json:"name"`
	Summary     *string                   `json:"summary"`
	Description *string                   `json:"description"`
	State       *string                   `json:"state"`
	Priority    *int32                    `json:"priority"`
	Lead        *string                   `json:"lead"`
	Labels      []string                  `json:"labels"`
	Teams       []string                  `json:"teams"`
	Milestones  []milestoneTemplateData   `json:"milestones"`
	Issues      []issue_service.SaveInput `json:"issues"`
}

type documentTemplateData struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

// checkData checks that data only holds what a template of kind fills in.
func checkData(kind string, data any) error {
	if _, ok := data.(args.M); !ok {
		return errdefs.BadRequest("data must be an object")
	}
	t := reflect.TypeFor[documentTemplateData]()
	switch kind {
	case "issue":
		t = reflect.TypeFor[issueTemplateData]()
	case "project":
		t = reflect.TypeFor[projectTemplateData]()
	}
	if err := args.CheckShape(data, t); err != nil {
		return errdefs.BadRequest("Invalid %s template data: %s", kind, err)
	}
	if err := args.CheckDecodes(data, t); err != nil {
		return errdefs.BadRequest("Invalid %s template data: %s", kind, err)
	}
	return nil
}
