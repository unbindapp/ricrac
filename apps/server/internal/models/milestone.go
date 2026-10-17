package models

import "ricrac/ent"

type Milestone struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	TargetDate  *string    `json:"targetDate"`
	SortOrder   float64    `json:"sortOrder"`
	Project     ProjectRef `json:"project"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
	ProjectIssueStats
}

type MilestoneDetail struct {
	Milestone
	Issues []IssueChild `json:"issues"`
}

type DeletedMilestone struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}

// MilestoneOf needs the milestone's project loaded.
func MilestoneOf(m *ent.ProjectMilestone, stats ProjectIssueStats) Milestone {
	return Milestone{
		ID: ID(m.ID), Name: m.Name, Description: m.Description, TargetDate: DatePtr(m.TargetDate), SortOrder: m.SortOrder,
		Project: *ProjectRefOf(m.Edges.Project), CreatedAt: Time(m.CreatedAt), UpdatedAt: Time(m.UpdatedAt),
		ProjectIssueStats: stats,
	}
}

// MilestoneIssueOf needs the issue's team, state and assignee loaded.
func MilestoneIssueOf(i *ent.Issue) IssueChild {
	return IssueChild{
		ID: ID(i.ID), Identifier: Identifier(i.Edges.Team.Key, i.Number), Title: i.Title, State: *StateRefOf(i.Edges.State),
		Assignee: UserRefOf(i.Edges.Assignee), Priority: i.Priority,
	}
}
