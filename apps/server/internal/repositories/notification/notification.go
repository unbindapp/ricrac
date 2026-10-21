package notification_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/notification"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i NotificationRepositoryInterface -p notification_repo -s NotificationRepository -o notification_repository_iface.go
type NotificationRepository struct {
	base *repository.BaseRepository
}

func NewNotificationRepository(db *ent.Client) *NotificationRepository {
	return &NotificationRepository{base: repository.NewBaseRepository(db)}
}

func withAnswer(q *ent.NotificationQuery) *ent.NotificationQuery {
	return q.WithActor().WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() }).
		WithComment().WithProject().WithInitiative().WithDocument().WithDiscussion().WithDiscussionThread()
}

func newestFirst(q *ent.NotificationQuery) *ent.NotificationQuery {
	return q.Order(ent.Desc(notification.FieldCreatedAt), ent.Asc(notification.FieldID))
}

func awake(s *entsql.Selector) {
	s.Where(repository.Raw("(" + s.C(notification.FieldSnoozedUntil) + " IS NULL OR " + s.C(notification.FieldSnoozedUntil) + " <= now())"))
}

type ListFilter struct {
	User, Workspace uuid.UUID
	UnreadOnly      bool
	IncludeArchived bool
	IncludeSnoozed  bool
	Type            *string
	Limit, Offset   int64
}

// List is the user's notifications in the workspace with what their answer shows, newest first.
func (self *NotificationRepository) List(ctx context.Context, filter ListFilter) ([]*ent.Notification, error) {
	query := withAnswer(self.base.DB.Notification.Query()).
		Where(notification.UserID(filter.User), notification.WorkspaceID(filter.Workspace))
	if filter.UnreadOnly {
		query = query.Where(notification.ReadAtIsNil())
	}
	if !filter.IncludeArchived {
		query = query.Where(notification.ArchivedAtIsNil())
	}
	if !filter.IncludeSnoozed {
		query = query.Where(awake)
	}
	if filter.Type != nil {
		query = query.Where(notification.Type(*filter.Type))
	}
	return newestFirst(query).Limit(int(filter.Limit)).Offset(int(filter.Offset)).All(ctx)
}

// UnreadCount is how many of the user's notifications are unread, not archived and awake.
func (self *NotificationRepository) UnreadCount(ctx context.Context, user, workspace uuid.UUID) (int, error) {
	return self.base.DB.Notification.Query().
		Where(notification.UserID(user), notification.WorkspaceID(workspace), notification.ReadAtIsNil(),
			notification.ArchivedAtIsNil(), awake).
		Count(ctx)
}

// CountOwn is how many of ids are the user's notifications in the workspace.
func (self *NotificationRepository) CountOwn(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, user, workspace uuid.UUID) (int, error) {
	return self.base.Client(tx).Notification.Query().
		Where(notification.IDIn(ids...), notification.UserID(user), notification.WorkspaceID(workspace)).
		Count(ctx)
}

// Mark reads, archives or snoozes the user's notifications among ids (see the actions of
// mark_notification), keeping when they were first read or archived.
func (self *NotificationRepository) Mark(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, user, workspace uuid.UUID, action string, until *time.Time, now time.Time) error {
	update := self.base.Client(tx).Notification.Update().
		Where(notification.IDIn(ids...), notification.UserID(user), notification.WorkspaceID(workspace))
	switch action {
	case "read":
		update = update.Where(notification.ReadAtIsNil()).SetReadAt(now)
	case "unread":
		update = update.ClearReadAt()
	case "archive":
		update = update.Where(notification.ArchivedAtIsNil()).SetArchivedAt(now)
	case "unarchive":
		update = update.ClearArchivedAt()
	case "snooze":
		update = update.SetNillableSnoozedUntil(until)
	case "unsnooze":
		update = update.ClearSnoozedUntil()
	}
	_, err := update.Save(ctx)
	return err
}

// Load is the notifications among ids with what their answer shows, newest first.
func (self *NotificationRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Notification, error) {
	return newestFirst(withAnswer(self.base.Client(tx).Notification.Query()).Where(notification.IDIn(ids...))).All(ctx)
}

// MarkAllRead reads the user's unread notifications, those created before before and about issue
// when given, answering how many.
func (self *NotificationRepository) MarkAllRead(ctx context.Context, user, workspace uuid.UUID, before *time.Time, issue *uuid.UUID, now time.Time) (int, error) {
	update := self.base.DB.Notification.Update().
		Where(notification.UserID(user), notification.WorkspaceID(workspace), notification.ReadAtIsNil())
	if before != nil {
		update = update.Where(notification.CreatedAtLT(*before))
	}
	if issue != nil {
		update = update.Where(notification.IssueID(*issue))
	}
	return update.SetReadAt(now).Save(ctx)
}
