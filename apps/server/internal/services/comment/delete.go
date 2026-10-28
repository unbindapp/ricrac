package comment_service

import (
	"context"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

func (self *CommentService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedComment, error) {
	row, err := self.Load(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedComment{}, err
	}
	if (row.UserID == nil || *row.UserID != actor.User) && actor.Role < auth.Admin {
		return models.DeletedComment{}, errdefs.Forbidden("Only its author or a workspace admin can delete a comment")
	}
	deleted := models.DeletedComment{Deleted: models.ID(row.ID)}
	if row.DeletedAt != nil {
		return deleted, nil
	}
	target, err := TargetOf(row)
	if err != nil {
		return models.DeletedComment{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if inForum(target) {
			return self.tombstone(ctx, tx, actor, row, target)
		}
		deleted.DeletedReplies, err = self.Remove(ctx, tx, actor, row)
		return err
	})
	if err != nil {
		return models.DeletedComment{}, err
	}
	return deleted, nil
}

// tombstone empties a forum message and marks it deleted, keeping its place and its replies.
func (self *CommentService) tombstone(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Comment, target Target) error {
	value, err := self.detail(ctx, tx, row.ID)
	if err != nil {
		return err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	if err := self.repo.Comment().Tombstone(ctx, tx, row.ID, now); err != nil {
		return err
	}
	team, err := self.teamOf(ctx, tx, target)
	if err != nil {
		return err
	}
	return self.events.Emit(ctx, tx, actor, "Comment", "remove", value, nil, team)
}

// Remove deletes a comment and its replies, answering how many replies went with it.
func (self *CommentService) Remove(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Comment) (int, error) {
	value, err := self.detail(ctx, tx, row.ID)
	if err != nil {
		return 0, err
	}
	replies, err := self.repo.Comment().ReplyCount(ctx, tx, row.ID)
	if err != nil {
		return 0, err
	}
	target, err := TargetOf(row)
	if err != nil {
		return 0, err
	}
	if target.Column == "issue_id" {
		if err := self.events.QueueCommentForGitHub(ctx, tx, actor, target.ID, row.ID, "comment_deleted"); err != nil {
			return 0, err
		}
	}
	if err := self.repo.Comment().Delete(ctx, tx, row.ID); err != nil {
		return 0, err
	}
	team, err := self.teamOf(ctx, tx, target)
	if err != nil {
		return 0, err
	}
	return replies, self.events.Emit(ctx, tx, actor, "Comment", "remove", value, nil, team)
}

func (self *CommentService) Resolve(ctx context.Context, actor *auth.Actor, input *IDInput) (models.CommentDetail, error) {
	return self.setResolved(ctx, actor, input.ID, true)
}

func (self *CommentService) Unresolve(ctx context.Context, actor *auth.Actor, input *IDInput) (models.CommentDetail, error) {
	return self.setResolved(ctx, actor, input.ID, false)
}

func (self *CommentService) setResolved(ctx context.Context, actor *auth.Actor, reference string, resolved bool) (models.CommentDetail, error) {
	row, err := self.Load(ctx, actor, reference)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if row.ParentID != nil {
		return models.CommentDetail{}, errdefs.BadRequest("Only a thread is resolved; pass the thread's first comment (the parentId)")
	}
	var value models.CommentDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Comment().SetResolved(ctx, tx, row.ID, resolved, actor.User, now); err != nil {
			return err
		}
		if value, err = self.detail(ctx, tx, row.ID); err != nil {
			return err
		}
		target, err := TargetOf(row)
		if err != nil {
			return err
		}
		team, err := self.teamOf(ctx, tx, target)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Comment", "update", value, nil, team)
	})
	return value, err
}
