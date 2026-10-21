package models

import (
	"ricrac/ent"
	"ricrac/internal/common/args"
)

type Notification struct {
	ID               string               `json:"id"`
	Type             string               `json:"type" doc:"What happened, e.g. issueAssignedToYou, issueMention, issueNewComment."`
	Summary          string               `json:"summary"`
	Actor            *UserRef             `json:"actor"`
	Issue            *IssueRef            `json:"issue"`
	Comment          *NotificationComment `json:"comment"`
	Project          *ProjectRef          `json:"project"`
	Initiative       *InitiativeRef       `json:"initiative"`
	Document         *DocumentRef         `json:"document"`
	StatusUpdate     *Ref                 `json:"statusUpdate"`
	Discussion       *DiscussionRef       `json:"discussion"`
	DiscussionThread *DiscussionThreadRef `json:"discussionThread"`
	ReadAt           *string              `json:"readAt"`
	SnoozedUntil     *string              `json:"snoozedUntil"`
	ArchivedAt       *string              `json:"archivedAt"`
	CreatedAt        string               `json:"createdAt"`
}

type NotificationComment struct {
	ID   string `json:"id"`
	Body string `json:"body" doc:"Its first 200 characters."`
}

type Inbox struct {
	args.PageOf[Notification]
	UnreadCount int `json:"unreadCount"`
}

type MarkedNotifications struct {
	Nodes []Notification `json:"nodes"`
}

type MarkedAllRead struct {
	Marked int `json:"marked" doc:"How many were marked read."`
}

// NotificationOf needs the notification's actor, issue (with its team and state), comment,
// project, initiative, document, discussion and discussion thread loaded.
func NotificationOf(n *ent.Notification) Notification {
	e := n.Edges
	notification := Notification{
		ID: ID(n.ID), Type: n.Type, Summary: n.Summary, Actor: UserRefOf(e.Actor), Issue: IssueRefOf(e.Issue),
		Project: ProjectRefOf(e.Project), Initiative: InitiativeRefOf(e.Initiative), Document: DocumentRefOf(e.Document),
		Discussion: DiscussionRefOf(e.Discussion), DiscussionThread: DiscussionThreadRefOf(e.DiscussionThread),
		ReadAt: TimePtr(n.ReadAt), SnoozedUntil: TimePtr(n.SnoozedUntil), ArchivedAt: TimePtr(n.ArchivedAt), CreatedAt: Time(n.CreatedAt),
	}
	if e.Comment != nil {
		body := []rune(e.Comment.Body)
		notification.Comment = &NotificationComment{ID: ID(e.Comment.ID), Body: string(body[:min(len(body), 200)])}
	}
	if n.StatusUpdateID != nil {
		notification.StatusUpdate = &Ref{ID: ID(*n.StatusUpdateID)}
	}
	return notification
}
