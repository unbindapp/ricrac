package models

import "ricrac/ent"

// CommentDetailOf needs what CommentOf needs, and the comment's workspace, issue (with its team and
// state), project, initiative, document, milestone, discussion and discussion thread (with its
// discussion) loaded.
func CommentDetailOf(c *ent.Comment, publicURL string) CommentDetail {
	e := c.Edges
	detail := CommentDetail{
		Comment: CommentOf(c), ParentID: IDPtr(c.ParentID), Issue: IssueRefOf(e.Issue), Project: ProjectRefOf(e.Project),
		Initiative: InitiativeRefOf(e.Initiative), Document: DocumentRefOf(e.Document), ProjectMilestone: MilestoneRefOf(e.Milestone),
	}
	if c.StatusUpdateID != nil {
		detail.StatusUpdate = &Ref{ID: ID(*c.StatusUpdateID)}
	}
	anchor := "#comment-" + ID(c.ID)
	switch {
	case e.Issue != nil:
		url := IssueURL(publicURL, e.Workspace.URLKey, detail.Issue.Identifier) + anchor
		detail.URL = &url
	case e.Document != nil:
		url := DocumentURL(publicURL, e.Workspace.URLKey, e.Document.Title, e.Document.SlugID) + anchor
		detail.URL = &url
	case e.Discussion != nil:
		detail.Discussion = DiscussionRefOf(e.Discussion)
		url := DiscussionURL(publicURL, e.Workspace.URLKey, e.Discussion.Title, e.Discussion.SlugID) + anchor
		detail.URL = &url
	case e.DiscussionThread != nil:
		discussion := e.DiscussionThread.Edges.Discussion
		detail.Discussion, detail.DiscussionThread = DiscussionRefOf(discussion), DiscussionThreadRefOf(e.DiscussionThread)
		url := DiscussionThreadURL(DiscussionURL(publicURL, e.Workspace.URLKey, discussion.Title, discussion.SlugID), ID(e.DiscussionThread.ID)) + anchor
		detail.URL = &url
	}
	return detail
}

// ReactionOf needs the reaction's user and issue (with its team and state) loaded.
func ReactionOf(r *ent.Reaction) Reaction {
	reaction := Reaction{
		ID: ID(r.ID), Emoji: r.Emoji, User: UserRefOf(r.Edges.User), Issue: IssueRefOf(r.Edges.Issue), CreatedAt: Time(r.CreatedAt),
	}
	if r.CommentID != nil {
		reaction.Comment = &Ref{ID: ID(*r.CommentID)}
	}
	if r.StatusUpdateID != nil {
		reaction.StatusUpdate = &Ref{ID: ID(*r.StatusUpdateID)}
	}
	return reaction
}
