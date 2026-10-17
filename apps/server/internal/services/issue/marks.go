package issue_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	events_service "ricrac/internal/services/events"
)

const withSubIssues = `WITH RECURSIVE tree AS (SELECT id FROM issues WHERE id = $1
	UNION SELECT i.id FROM issues i JOIN tree ON i.parent_id = tree.id)`

// mark runs the update (trashing, restoring, archiving or unarchiving), recording action in the
// history of every issue it changed.
func (self *IssueService) mark(ctx context.Context, actor *auth.Actor, reference, sql, action, webhookAction string) (models.Issue, error) {
	id, err := self.resolve.Issue(ctx, actor, reference)
	if err != nil {
		return models.Issue{}, err
	}
	var value models.Issue
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		row, err := self.lockIssue(ctx, tx, id)
		if err != nil {
			return err
		}
		affected, err := self.repo.Issue().Mark(ctx, tx, sql, id)
		if err != nil {
			return err
		}
		for _, issue := range affected {
			if err := self.events.History(ctx, tx, actor, issue, action, M{}); err != nil {
				return err
			}
		}
		if value, err = self.Value(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Issue", webhookAction, value, nil, &row.TeamID)
	})
	return value, err
}

func (self *IssueService) Trash(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Issue, error) {
	return self.mark(ctx, actor, input.ID, withSubIssues+
		" UPDATE issues SET trashed_at = now(), updated_at = now() WHERE id IN (SELECT id FROM tree) AND trashed_at IS NULL RETURNING id",
		"trashed", "remove")
}

func (self *IssueService) Restore(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Issue, error) {
	return self.mark(ctx, actor, input.ID, withSubIssues+
		" UPDATE issues SET trashed_at = NULL, updated_at = now() WHERE id IN (SELECT id FROM tree) AND trashed_at IS NOT NULL RETURNING id",
		"restored", "update")
}

func (self *IssueService) Archive(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Issue, error) {
	return self.mark(ctx, actor, input.ID, withSubIssues+
		" UPDATE issues SET archived_at = now(), updated_at = now() WHERE id IN (SELECT id FROM tree) AND archived_at IS NULL RETURNING id",
		"archived", "update")
}

func (self *IssueService) Unarchive(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Issue, error) {
	return self.mark(ctx, actor, input.ID,
		"UPDATE issues SET archived_at = NULL, auto_archived_at = NULL, updated_at = now() WHERE id = $1 AND archived_at IS NOT NULL RETURNING id",
		"unarchived", "update")
}

func (self *IssueService) Share(ctx context.Context, actor *auth.Actor, input *ShareInput) (models.IssueShare, error) {
	if err := actor.RequireMember(); err != nil {
		return models.IssueShare{}, err
	}
	id, err := self.resolve.Issue(ctx, actor, input.ID)
	if err != nil {
		return models.IssueShare{}, err
	}
	user, err := self.resolve.User(ctx, actor, input.User)
	if err != nil {
		return models.IssueShare{}, err
	}
	var identifier string
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.repo.Issue().Share(ctx, tx, id, user, actor.User); err != nil {
			return err
		}
		if err := self.events.Subscribe(ctx, tx, id, []uuid.UUID{user}); err != nil {
			return err
		}
		var err error
		if identifier, err = self.repo.Issue().Identifier(ctx, tx, id); err != nil {
			return err
		}
		notice := events_service.Notice{Kind: "issueShared", Summary: identifier + " was shared with you", Issue: &id}
		if err := self.events.Notify(ctx, tx, actor, []uuid.UUID{user}, notice); err != nil {
			return err
		}
		return self.events.History(ctx, tx, actor, id, "shared", M{"user": user})
	})
	if err != nil {
		return models.IssueShare{}, err
	}
	return models.IssueShare{Shared: identifier, With: user.String()}, nil
}

func (self *IssueService) Unshare(ctx context.Context, actor *auth.Actor, input *ShareInput) (models.IssueUnshare, error) {
	if err := actor.RequireMember(); err != nil {
		return models.IssueUnshare{}, err
	}
	id, err := self.resolve.Issue(ctx, actor, input.ID)
	if err != nil {
		return models.IssueUnshare{}, err
	}
	user, err := self.resolve.User(ctx, actor, input.User)
	if err != nil {
		return models.IssueUnshare{}, err
	}
	if err := self.repo.Issue().Unshare(ctx, nil, id, user); err != nil {
		return models.IssueUnshare{}, err
	}
	return models.IssueUnshare{Unshared: input.ID, With: user.String()}, nil
}
