package discussion_repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/discussionsubscriber"
	"ricrac/ent/discussionthread"
	repository "ricrac/internal/repositories"
)

type CreateInput struct {
	Workspace uuid.UUID
	SlugID    string
	Title     string
	Body      string
	Team      *uuid.UUID
	Creator   uuid.UUID
}

func (self *DiscussionRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Discussion.Create().
		SetWorkspaceID(input.Workspace).SetSlugID(input.SlugID).SetTitle(input.Title).SetBody(input.Body).
		SetNillableTeamID(input.Team).SetCreatorID(input.Creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *DiscussionRepository) SetText(ctx context.Context, tx repository.TxInterface, id uuid.UUID, title, body string) error {
	return self.base.Client(tx).Discussion.UpdateOneID(id).SetTitle(title).SetBody(body).Exec(ctx)
}

func (self *DiscussionRepository) SetTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID, team *uuid.UUID) error {
	update := self.base.Client(tx).Discussion.UpdateOneID(id)
	if team == nil {
		return update.ClearTeamID().Exec(ctx)
	}
	return update.SetTeamID(*team).Exec(ctx)
}

// SetPinned pins the discussion at now, or unpins it when now is nil.
func (self *DiscussionRepository) SetPinned(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now *time.Time) error {
	update := self.base.Client(tx).Discussion.UpdateOneID(id)
	if now == nil {
		return update.ClearPinnedAt().Exec(ctx)
	}
	return update.SetPinnedAt(*now).Exec(ctx)
}

// SetClosed closes the discussion at now, or reopens it when now is nil.
func (self *DiscussionRepository) SetClosed(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now *time.Time) error {
	update := self.base.Client(tx).Discussion.UpdateOneID(id)
	if now == nil {
		return update.ClearClosedAt().Exec(ctx)
	}
	return update.SetClosedAt(*now).Exec(ctx)
}

func (self *DiscussionRepository) SetTrashed(ctx context.Context, tx repository.TxInterface, id uuid.UUID, trashed bool, now time.Time) error {
	update := self.base.Client(tx).Discussion.UpdateOneID(id).SetUpdatedAt(now)
	if trashed {
		return update.SetTrashedAt(now).Exec(ctx)
	}
	return update.ClearTrashedAt().Exec(ctx)
}

func (self *DiscussionRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Discussion.UpdateOneID(id).SetUpdatedAt(now).SetLastActivityAt(now).Exec(ctx)
}

// Bump marks activity in the discussion, such as a comment, without it being edited.
func (self *DiscussionRepository) Bump(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Discussion.UpdateOneID(id).SetLastActivityAt(now).Exec(ctx)
}

func (self *DiscussionRepository) Subscribe(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	creates := make([]*ent.DiscussionSubscriberCreate, len(users))
	for i, u := range users {
		creates[i] = self.base.Client(tx).DiscussionSubscriber.Create().SetDiscussionID(id).SetUserID(u)
	}
	return self.base.Client(tx).DiscussionSubscriber.CreateBulk(creates...).OnConflict().DoNothing().Exec(ctx)
}

func (self *DiscussionRepository) Unsubscribe(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	_, err := self.base.Client(tx).DiscussionSubscriber.Delete().
		Where(discussionsubscriber.DiscussionID(id), discussionsubscriber.UserIDIn(users...)).Exec(ctx)
	return err
}

type ThreadInput struct {
	Workspace  uuid.UUID
	Discussion uuid.UUID
	Kind       string
	Title      string
	Body       string
	Issue      *uuid.UUID
	Creator    uuid.UUID
}

func (self *DiscussionRepository) CreateThread(ctx context.Context, tx repository.TxInterface, input ThreadInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).DiscussionThread.Create().
		SetWorkspaceID(input.Workspace).SetDiscussionID(input.Discussion).SetKind(input.Kind).SetTitle(input.Title).
		SetBody(input.Body).SetNillableIssueID(input.Issue).SetCreatorID(input.Creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// ThreadChanges is what changes on a thread; nil fields stay as they are.
type ThreadChanges struct {
	Kind, Title, Body *string
	// Set with Issue nil unlinks the issue.
	SetIssue bool
	Issue    *uuid.UUID
}

func (self *DiscussionRepository) UpdateThread(ctx context.Context, tx repository.TxInterface, id uuid.UUID, changes ThreadChanges, now time.Time) error {
	update := self.base.Client(tx).DiscussionThread.UpdateOneID(id).SetUpdatedAt(now).
		SetNillableKind(changes.Kind).SetNillableTitle(changes.Title).SetNillableBody(changes.Body)
	switch {
	case !changes.SetIssue:
	case changes.Issue == nil:
		update.ClearIssueID()
	default:
		update.SetIssueID(*changes.Issue)
	}
	return update.Exec(ctx)
}

// SetThreadResolved resolves the thread or reopens it.
func (self *DiscussionRepository) SetThreadResolved(ctx context.Context, tx repository.TxInterface, id uuid.UUID, resolved bool, by uuid.UUID, now time.Time) error {
	update := self.base.Client(tx).DiscussionThread.UpdateOneID(id).SetUpdatedAt(now)
	if !resolved {
		return update.ClearResolvedAt().ClearResolvingUserID().Exec(ctx)
	}
	return update.SetResolvedAt(now).SetResolvingUserID(by).Exec(ctx)
}

func (self *DiscussionRepository) DeleteThread(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).DiscussionThread.Delete().Where(discussionthread.ID(id)).Exec(ctx)
	return err
}
