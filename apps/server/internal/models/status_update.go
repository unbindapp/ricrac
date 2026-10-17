package models

import "ricrac/ent"

type StatusUpdate struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type" enum:"project,initiative"`
	Project      *ProjectRef            `json:"project"`
	Initiative   *InitiativeRef         `json:"initiative"`
	Body         string                 `json:"body"`
	Health       string                 `json:"health" enum:"onTrack,atRisk,offTrack"`
	User         *UserRef               `json:"user"`
	Reactions    []StatusUpdateReaction `json:"reactions"`
	CommentCount int                    `json:"commentCount"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
	EditedAt     *string                `json:"editedAt"`
}

type StatusUpdateReaction struct {
	Emoji string   `json:"emoji"`
	User  *UserRef `json:"user"`
}

type DeletedStatusUpdate struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// StatusUpdateOf needs the update's project, initiative, user and reactions (oldest first, with
// their users) loaded.
func StatusUpdateOf(u *ent.StatusUpdate, commentCount int) StatusUpdate {
	e := u.Edges
	kind := "initiative"
	if u.ProjectID != nil {
		kind = "project"
	}
	reactions := make([]StatusUpdateReaction, len(e.Reactions))
	for i, r := range e.Reactions {
		reactions[i] = StatusUpdateReaction{Emoji: r.Emoji, User: UserRefOf(r.Edges.User)}
	}
	return StatusUpdate{
		ID: ID(u.ID), Type: kind, Project: ProjectRefOf(e.Project), Initiative: InitiativeRefOf(e.Initiative), Body: u.Body,
		Health: u.Health, User: UserRefOf(e.User), Reactions: reactions, CommentCount: commentCount,
		CreatedAt: Time(u.CreatedAt), UpdatedAt: Time(u.UpdatedAt), EditedAt: TimePtr(u.EditedAt),
	}
}
