package comment_repo

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/ent/comment"
	"ricrac/ent/messageversion"
	"ricrac/ent/reaction"
	repository "ricrac/internal/repositories"
)

type CreateInput struct {
	Workspace uuid.UUID
	// The column of what it is on (see Targets) and its id.
	Column string
	Target uuid.UUID
	// The thread's first comment, for a reply.
	Root   *uuid.UUID
	User   *uuid.UUID
	Body   string
	Quoted *string
	// Text suggested in place of Quoted, on a document.
	Suggested *string
}

func (self *CommentRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	if !slices.Contains(Targets, input.Column) {
		return uuid.Nil, fmt.Errorf("comments can't be on %q", input.Column)
	}
	create := self.base.Client(tx).Comment.Create().
		SetWorkspaceID(input.Workspace).SetNillableParentID(input.Root).SetNillableUserID(input.User).
		SetBody(input.Body).SetNillableQuotedText(input.Quoted).SetNillableSuggestedText(input.Suggested)
	if input.Suggested != nil {
		create.SetSuggestionStatus("pending")
	}
	if err := create.Mutation().SetField(input.Column, input.Target); err != nil {
		return uuid.Nil, err
	}
	created, err := create.Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *CommentRepository) TouchIssue(ctx context.Context, tx repository.TxInterface, issue uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Issue.UpdateOneID(issue).SetUpdatedAt(now).Exec(ctx)
}

// SetBody rewrites the comment, marking it edited.
func (self *CommentRepository) SetBody(ctx context.Context, tx repository.TxInterface, id uuid.UUID, body string, now time.Time) error {
	return self.base.Client(tx).Comment.UpdateOneID(id).SetBody(body).SetEditedAt(now).SetUpdatedAt(now).Exec(ctx)
}

// SetResolved resolves the thread (keeping when and by whom it was first resolved) or reopens it.
func (self *CommentRepository) SetResolved(ctx context.Context, tx repository.TxInterface, id uuid.UUID, resolved bool, by uuid.UUID, now time.Time) error {
	current, err := self.base.Client(tx).Comment.Get(ctx, id)
	if err != nil {
		return err
	}
	update := self.base.Client(tx).Comment.UpdateOneID(id).SetUpdatedAt(now)
	switch {
	case !resolved:
		update.ClearResolvedAt().ClearResolvingUserID()
	default:
		if current.ResolvedAt == nil {
			update.SetResolvedAt(now)
		}
		if current.ResolvingUserID == nil {
			update.SetResolvingUserID(by)
		}
	}
	return update.Exec(ctx)
}

// Decide settles a suggestion (accepted or rejected), resolving its thread.
func (self *CommentRepository) Decide(ctx context.Context, tx repository.TxInterface, id uuid.UUID, status string, by uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Comment.UpdateOneID(id).SetSuggestionStatus(status).
		SetResolvedAt(now).SetResolvingUserID(by).SetUpdatedAt(now).Exec(ctx)
}

func (self *CommentRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).Comment.Delete().Where(comment.ID(id)).Exec(ctx)
	return err
}

// Tombstone empties the comment and marks it deleted, dropping its reactions and earlier versions
// and keeping its replies.
func (self *CommentRepository) Tombstone(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	if _, err := self.base.Client(tx).Reaction.Delete().Where(reaction.CommentID(id)).Exec(ctx); err != nil {
		return err
	}
	if _, err := self.base.Client(tx).MessageVersion.Delete().Where(messageversion.CommentID(id)).Exec(ctx); err != nil {
		return err
	}
	return self.base.Client(tx).Comment.UpdateOneID(id).SetBody("").ClearQuotedText().ClearSuggestedText().
		SetDeletedAt(now).SetUpdatedAt(now).Exec(ctx)
}

// AddVersion keeps an earlier body of the comment, as it read from writtenAt on.
func (self *CommentRepository) AddVersion(ctx context.Context, tx repository.TxInterface, workspace, id uuid.UUID, body string, writtenAt time.Time) error {
	return self.base.Client(tx).MessageVersion.Create().
		SetWorkspaceID(workspace).SetCommentID(id).SetBody(body).SetWrittenAt(writtenAt).Exec(ctx)
}

// AddReaction reacts unless the user already reacted so, saying whether it added one.
func (self *CommentRepository) AddReaction(ctx context.Context, tx repository.TxInterface, workspace, userID uuid.UUID, emoji, column string, id uuid.UUID) (bool, error) {
	if !slices.Contains(ReactionTargets, column) {
		return false, fmt.Errorf("reactions can't be on %q", column)
	}
	result, err := self.base.Client(tx).ExecContext(ctx, fmt.Sprintf(
		`INSERT INTO reactions (workspace_id, user_id, emoji, %s) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, column),
		workspace, userID, emoji, id)
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added > 0, err
}

func (self *CommentRepository) DeleteReaction(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).Reaction.Delete().Where(reaction.ID(id)).Exec(ctx)
	return err
}
