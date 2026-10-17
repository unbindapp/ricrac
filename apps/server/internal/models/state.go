package models

import (
	"ricrac/ent"
)

type IssueStatus struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"type" enum:"triage,backlog,unstarted,started,completed,canceled"`
	Color            string  `json:"color"`
	Description      *string `json:"description"`
	Position         float64 `json:"position" doc:"Order within its type on the board."`
	Team             TeamRef `json:"team"`
	IssueCount       int     `json:"issueCount"`
	IsDefault        bool    `json:"isDefault" doc:"New issues start in it."`
	IsAutoCloseState bool    `json:"isAutoCloseState" doc:"Auto-closed issues go to it."`
	ArchivedAt       *string `json:"archivedAt"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

type DeletedIssueStatus struct {
	IssueStatus
	MovedIssues int       `json:"movedIssues"`
	MovedTo     *StateRef `json:"movedTo"`
}

// IssueStatusOf needs the state's team loaded.
func IssueStatusOf(s *ent.WorkflowState, issueCount int) IssueStatus {
	t := s.Edges.Team
	return IssueStatus{
		ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color, Description: s.Description, Position: s.Position,
		Team: *TeamRefOf(t), IssueCount: issueCount,
		IsDefault:        t.DefaultIssueStateID != nil && *t.DefaultIssueStateID == s.ID,
		IsAutoCloseState: t.AutoCloseStateID != nil && *t.AutoCloseStateID == s.ID,
		ArchivedAt:       TimePtr(s.ArchivedAt), CreatedAt: Time(s.CreatedAt), UpdatedAt: Time(s.UpdatedAt),
	}
}
