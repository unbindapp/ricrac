package models

import "ricrac/ent"

// CommentOf needs the comment's user, resolving user and reactions (oldest first, with their
// users) loaded.
func CommentOf(c *ent.Comment) Comment {
	reactions := make([]CommentReaction, len(c.Edges.Reactions))
	for i, r := range c.Edges.Reactions {
		reactions[i] = CommentReaction{ID: ID(r.ID), Emoji: r.Emoji, User: UserRefOf(r.Edges.User)}
	}
	return Comment{
		ID: ID(c.ID), Body: c.Body, User: UserRefOf(c.Edges.User), QuotedText: c.QuotedText, Reactions: reactions,
		CreatedAt: Time(c.CreatedAt), UpdatedAt: Time(c.UpdatedAt), EditedAt: TimePtr(c.EditedAt),
		ResolvedAt: TimePtr(c.ResolvedAt), ResolvingUser: UserRefOf(c.Edges.ResolvingUser),
	}
}

// CommentThreadOf needs what CommentOf needs, for the comment and its replies (Children, oldest
// first).
func CommentThreadOf(c *ent.Comment) CommentThread {
	replies := make([]Comment, len(c.Edges.Children))
	for i, reply := range c.Edges.Children {
		replies[i] = CommentOf(reply)
	}
	return CommentThread{Comment: CommentOf(c), Replies: replies}
}
