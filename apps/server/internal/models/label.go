package models

import (
	"ricrac/ent"
)

type IssueLabel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Color       string            `json:"color"`
	Description *string           `json:"description"`
	IsGroup     bool              `json:"isGroup"`
	GroupType   *string           `json:"groupType" enum:"singleSelect,multiSelect" doc:"For groups."`
	Parent      *IssueLabelParent `json:"parent" doc:"The group it is in."`
	Team        *TeamRef          `json:"team" doc:"Null for a workspace label."`
	Children    *[]LabelRef       `json:"children" doc:"For groups: the labels in it."`
	IssueCount  int               `json:"issueCount"`
	Creator     *UserRef          `json:"creator"`
	RetiredAt   *string           `json:"retiredAt"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

type IssueLabelParent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeletedIssueLabel struct {
	Deleted           string `json:"deleted"`
	RemovedFromIssues int    `json:"removedFromIssues"`
}

// IssueLabelOf needs the label's parent, team, creator and children loaded.
func IssueLabelOf(l *ent.IssueLabel, issueCount int) IssueLabel {
	label := IssueLabel{
		ID: ID(l.ID), Name: l.Name, Color: l.Color, Description: l.Description, IsGroup: l.IsGroup,
		Team: TeamRefOf(l.Edges.Team), IssueCount: issueCount, Creator: UserRefOf(l.Edges.Creator),
		RetiredAt: TimePtr(l.RetiredAt), CreatedAt: Time(l.CreatedAt), UpdatedAt: Time(l.UpdatedAt),
	}
	if l.Edges.Parent != nil {
		label.Parent = &IssueLabelParent{ID: ID(l.Edges.Parent.ID), Name: l.Edges.Parent.Name}
	}
	if l.IsGroup {
		label.GroupType = &l.GroupType
		children := make([]LabelRef, len(l.Edges.Children))
		for i, child := range l.Edges.Children {
			children[i] = LabelRef{ID: ID(child.ID), Name: child.Name, Color: child.Color, Parent: &l.Name}
		}
		label.Children = &children
	}
	return label
}
