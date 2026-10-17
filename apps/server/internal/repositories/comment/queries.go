package comment_repo

import (
	"context"
	"fmt"
	"slices"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/comment"
	"ricrac/ent/issue"
	"ricrac/ent/projectmilestone"
	"ricrac/ent/reaction"
	"ricrac/ent/statusupdate"
	"ricrac/ent/user"
	repository "ricrac/internal/repositories"
)

// Find is a comment of the workspace, nil when there's none.
func (self *CommentRepository) Find(ctx context.Context, workspace, id uuid.UUID) (*ent.Comment, error) {
	found, err := self.base.DB.Comment.Query().Where(comment.ID(id), comment.WorkspaceID(workspace)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// Load is the comment with what its detailed answer shows: what it is on and its workspace.
func (self *CommentRepository) Load(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Comment, error) {
	found, err := WithAnswer(self.base.Client(tx).Comment.Query()).Where(comment.ID(id)).
		WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() }).
		WithProject().WithInitiative().WithDocument().WithMilestone().WithWorkspace().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// Participants is everyone who wrote in the thread that starts with root.
func (self *CommentRepository) Participants(ctx context.Context, tx repository.TxInterface, root uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		UserID uuid.UUID `json:"user_id"`
	}
	err := self.base.Client(tx).Comment.Query().
		Where(comment.Or(comment.ID(root), comment.ParentID(root)), comment.UserIDNotNil()).
		Unique(true).Select(comment.FieldUserID).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	users := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		users[i] = row.UserID
	}
	return users, nil
}

func (self *CommentRepository) ReplyCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).Comment.Query().Where(comment.ParentID(id)).Count(ctx)
}

// StatusUpdate is the status update with its project and initiative, nil when there's none.
func (self *CommentRepository) StatusUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.StatusUpdate, error) {
	found, err := self.base.Client(tx).StatusUpdate.Query().Where(statusupdate.ID(id)).WithProject().WithInitiative().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// IssueTeam is the issue's team, nil when there's no such issue.
func (self *CommentRepository) IssueTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*uuid.UUID, error) {
	found, err := self.base.Client(tx).Issue.Query().Where(issue.ID(id)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &found.TeamID, nil
}

func (self *CommentRepository) UserName(ctx context.Context, id uuid.UUID) (string, error) {
	found, err := self.base.DB.User.Query().Where(user.ID(id)).Only(ctx)
	if err != nil {
		return "", err
	}
	return found.Name, nil
}

func (self *CommentRepository) ProjectName(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (string, error) {
	found, err := self.base.Client(tx).Project.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return found.Name, nil
}

func (self *CommentRepository) InitiativeName(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (string, error) {
	found, err := self.base.Client(tx).Initiative.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return found.Name, nil
}

func (self *CommentRepository) DocumentTitle(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (string, error) {
	found, err := self.base.Client(tx).Document.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return found.Title, nil
}

// Milestone is the milestone with its project.
func (self *CommentRepository) Milestone(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.ProjectMilestone, error) {
	return self.base.Client(tx).ProjectMilestone.Query().Where(projectmilestone.ID(id)).WithProject().Only(ctx)
}

// WithReactionAnswer loads what a reaction's answer shows (see models.ReactionOf).
func WithReactionAnswer(q *ent.ReactionQuery) *ent.ReactionQuery {
	return q.WithUser().WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() })
}

func (self *CommentRepository) Reaction(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Reaction, error) {
	return WithReactionAnswer(self.base.Client(tx).Reaction.Query()).Where(reaction.ID(id)).Only(ctx)
}

// ReactionTargets is the columns reactions can be on.
var ReactionTargets = []string{"issue_id", "comment_id", "status_update_id"}

// FindReaction is the user's reaction with emoji on the thing whose id is in column, nil when
// there's none.
func (self *CommentRepository) FindReaction(ctx context.Context, tx repository.TxInterface, userID uuid.UUID, emoji, column string, id uuid.UUID) (*ent.Reaction, error) {
	if !slices.Contains(ReactionTargets, column) {
		return nil, fmt.Errorf("reactions can't be on %q", column)
	}
	found, err := WithReactionAnswer(self.base.Client(tx).Reaction.Query()).
		Where(reaction.UserID(userID), reaction.Emoji(emoji), func(s *entsql.Selector) { s.Where(entsql.EQ(s.C(column), id)) }).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// OwnReaction is the user's reaction in the workspace with the team of the issue it is on (or of
// the comment's issue), nil when there's none.
func (self *CommentRepository) OwnReaction(ctx context.Context, workspace, userID, id uuid.UUID) (*ent.Reaction, *uuid.UUID, error) {
	found, err := self.base.DB.Reaction.Query().
		Where(reaction.ID(id), reaction.WorkspaceID(workspace), reaction.UserID(userID)).
		WithIssue().WithComment(func(q *ent.CommentQuery) { q.WithIssue() }).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	switch {
	case found.Edges.Issue != nil:
		return found, &found.Edges.Issue.TeamID, nil
	case found.Edges.Comment != nil && found.Edges.Comment.Edges.Issue != nil:
		return found, &found.Edges.Comment.Edges.Issue.TeamID, nil
	}
	return found, nil, nil
}
