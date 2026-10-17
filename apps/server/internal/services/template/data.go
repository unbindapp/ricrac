package template_service

import (
	"reflect"

	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
)

type issueTemplateData struct {
	Title       *string           `json:"title"`
	Description *string           `json:"description"`
	Team        *string           `json:"team"`
	State       *string           `json:"state"`
	Priority    *int32            `json:"priority"`
	Estimate    *float64          `json:"estimate"`
	Assignee    *string           `json:"assignee"`
	Labels      []string          `json:"labels"`
	Project     *string           `json:"project"`
	Cycle       *string           `json:"cycle"`
	DueDate     *string           `json:"dueDate"`
	SubIssues   []saveIssueFields `json:"subIssues"`
}

type milestoneTemplateData struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	TargetDate  *string `json:"targetDate"`
}

type projectTemplateData struct {
	Name        *string                 `json:"name"`
	Summary     *string                 `json:"summary"`
	Description *string                 `json:"description"`
	State       *string                 `json:"state"`
	Priority    *int32                  `json:"priority"`
	Lead        *string                 `json:"lead"`
	Labels      []string                `json:"labels"`
	Teams       []string                `json:"teams"`
	Milestones  []milestoneTemplateData `json:"milestones"`
	Issues      []saveIssueFields       `json:"issues"`
}

type documentTemplateData struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

type linkFields struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// saveIssueFields is the shape of save_issue's arguments, which sub-issues and project issues
// are written as.
type saveIssueFields struct {
	ID                *string                `json:"id"`
	Team              *string                `json:"team"`
	Title             *string                `json:"title"`
	Description       *string                `json:"description"`
	Patch             []args.TextEdit        `json:"patch"`
	State             *string                `json:"state"`
	Priority          *int                   `json:"priority"`
	Estimate          args.Nullable[float64] `json:"estimate"`
	Assignee          args.Nullable[string]  `json:"assignee"`
	Labels            []string               `json:"labels"`
	AddLabels         []string               `json:"addLabels"`
	RemoveLabels      []string               `json:"removeLabels"`
	Project           args.Nullable[string]  `json:"project"`
	Milestone         args.Nullable[string]  `json:"milestone"`
	Cycle             args.Nullable[string]  `json:"cycle"`
	ParentID          args.Nullable[string]  `json:"parentId"`
	DueDate           args.Nullable[string]  `json:"dueDate"`
	Blocks            []string               `json:"blocks"`
	BlockedBy         []string               `json:"blockedBy"`
	RelatedTo         []string               `json:"relatedTo"`
	RemoveBlocks      []string               `json:"removeBlocks"`
	RemoveBlockedBy   []string               `json:"removeBlockedBy"`
	RemoveRelatedTo   []string               `json:"removeRelatedTo"`
	DuplicateOf       args.Nullable[string]  `json:"duplicateOf"`
	Links             []linkFields           `json:"links"`
	AddSubscribers    []string               `json:"addSubscribers"`
	RemoveSubscribers []string               `json:"removeSubscribers"`
	AddReleases       []string               `json:"addReleases"`
	RemoveReleases    []string               `json:"removeReleases"`
	SetReleases       []string               `json:"setReleases"`
	SnoozedUntil      args.Nullable[string]  `json:"snoozedUntil"`
	SLABreachesAt     args.Nullable[string]  `json:"slaBreachesAt"`
	SLAType           *string                `json:"slaType"`
	SortOrder         *float64               `json:"sortOrder"`
	SubIssueSortOrder *float64               `json:"subIssueSortOrder"`
	Template          *string                `json:"template"`
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
