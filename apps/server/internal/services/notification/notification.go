// Package notification_service is the inbox: notifications about what happened to the things a
// person is involved in, which they read, archive or snooze.
package notification_service

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	notification_repo "ricrac/internal/repositories/notification"
	"ricrac/internal/repositories/repositories"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i NotificationServiceInterface -p notification_service -s NotificationService -o notification_service_iface.go
type NotificationService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
}

func NewNotificationService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface) *NotificationService {
	return &NotificationService{repo: repo, resolve: resolve}
}

type ListInput struct {
	UnreadOnly      bool    `json:"unreadOnly,omitempty" doc:"Only unread ones."`
	IncludeArchived bool    `json:"includeArchived,omitempty" doc:"Also archived ones."`
	IncludeSnoozed  bool    `json:"includeSnoozed,omitempty" doc:"Also ones snoozed until later."`
	Type            *string `json:"type" doc:"Only this type, e.g. issueAssignedToYou, issueMention, issueNewComment, issueCommentReply, issueStatusChanged."`
	args.Page
}

type MarkInput struct {
	ID     *string  `json:"id" doc:"One notification's id."`
	IDs    []string `json:"ids" doc:"Several notifications' ids."`
	Action string   `json:"action" doc:"read, unread, archive, unarchive, snooze or unsnooze."`
	Until  *string  `json:"until" doc:"For snooze: until when, as ISO 8601 or a duration from now like PT3H or P1D."`
}

type MarkAllReadInput struct {
	Before *string `json:"before" doc:"Only notifications created before this time (ISO 8601 or a duration like -P1D)."`
	Issue  *string `json:"issue" doc:"Only notifications about this issue (identifier, id or link)."`
}

var actions = []string{"read", "unread", "archive", "unarchive", "snooze", "unsnooze"}

func values(found []*ent.Notification) []models.Notification {
	notifications := make([]models.Notification, len(found))
	for i, n := range found {
		notifications[i] = models.NotificationOf(n)
	}
	return notifications
}

func (self *NotificationService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (models.Inbox, error) {
	offset, err := input.Offset()
	if err != nil {
		return models.Inbox{}, err
	}
	found, err := self.repo.Notification().List(ctx, notification_repo.ListFilter{
		User: actor.User, Workspace: actor.Workspace, UnreadOnly: input.UnreadOnly, IncludeArchived: input.IncludeArchived,
		IncludeSnoozed: input.IncludeSnoozed, Type: utils.Trimmed(input.Type), Limit: input.Size() + 1, Offset: offset,
	})
	if err != nil {
		return models.Inbox{}, err
	}
	unread, err := self.repo.Notification().UnreadCount(ctx, actor.User, actor.Workspace)
	if err != nil {
		return models.Inbox{}, err
	}
	page, err := args.WrapOf(input.Page, values(found))
	if err != nil {
		return models.Inbox{}, err
	}
	return models.Inbox{PageOf: page, UnreadCount: unread}, nil
}

// markIDs is the notifications named, each once.
func markIDs(input *MarkInput) ([]uuid.UUID, error) {
	named := input.IDs
	if input.ID != nil {
		named = append([]string{*input.ID}, named...)
	}
	ids := []uuid.UUID{}
	for _, value := range named {
		id, ok := utils.AsUUID(value)
		if !ok {
			return nil, errdefs.NotFound("No notification %q", value)
		}
		ids = utils.AppendUnique(ids, id)
	}
	if len(ids) == 0 || len(ids) > 250 {
		return nil, errdefs.BadRequest("Pass id or ids (at most 250)")
	}
	return ids, nil
}

// snoozeUntil is when a snooze ends, nil for the other actions.
func snoozeUntil(input *MarkInput) (*time.Time, error) {
	switch {
	case input.Action == "snooze" && input.Until != nil:
		at, err := utils.ParseDatetime(*input.Until)
		if err != nil {
			return nil, err
		}
		if !at.After(time.Now()) {
			return nil, errdefs.BadRequest("until must be in the future")
		}
		return &at, nil
	case input.Action == "snooze":
		return nil, errdefs.BadRequest("Pass until to snooze, e.g. PT3H, P1D or an ISO time")
	case input.Until != nil:
		return nil, errdefs.BadRequest("until is only for snooze")
	}
	return nil, nil
}

func (self *NotificationService) Mark(ctx context.Context, actor *auth.Actor, input *MarkInput) (models.MarkedNotifications, error) {
	ids, err := markIDs(input)
	if err != nil {
		return models.MarkedNotifications{}, err
	}
	until, err := snoozeUntil(input)
	if err != nil {
		return models.MarkedNotifications{}, err
	}
	if !slices.Contains(actions, input.Action) {
		return models.MarkedNotifications{}, errdefs.BadRequest("action %q isn't one of read, unread, archive, unarchive, snooze, unsnooze", input.Action)
	}
	var found []*ent.Notification
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		own, err := self.repo.Notification().CountOwn(ctx, tx, ids, actor.User, actor.Workspace)
		if err != nil {
			return err
		}
		if own != len(ids) {
			return errdefs.NotFound("No such notification in your inbox")
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Notification().Mark(ctx, tx, ids, actor.User, actor.Workspace, input.Action, until, now); err != nil {
			return err
		}
		found, err = self.repo.Notification().Load(ctx, tx, ids)
		return err
	})
	if err != nil {
		return models.MarkedNotifications{}, err
	}
	return models.MarkedNotifications{Nodes: values(found)}, nil
}

func (self *NotificationService) MarkAllRead(ctx context.Context, actor *auth.Actor, input *MarkAllReadInput) (models.MarkedAllRead, error) {
	var before *time.Time
	if input.Before != nil {
		at, err := utils.ParseDatetime(*input.Before)
		if err != nil {
			return models.MarkedAllRead{}, err
		}
		before = &at
	}
	var issue *uuid.UUID
	if input.Issue != nil {
		id, err := self.resolve.Issue(ctx, actor, *input.Issue)
		if err != nil {
			return models.MarkedAllRead{}, err
		}
		issue = &id
	}
	now, err := self.repo.Now(ctx, nil)
	if err != nil {
		return models.MarkedAllRead{}, err
	}
	marked, err := self.repo.Notification().MarkAllRead(ctx, actor.User, actor.Workspace, before, issue, now)
	if err != nil {
		return models.MarkedAllRead{}, err
	}
	return models.MarkedAllRead{Marked: marked}, nil
}
