package email_repo

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/email"
	"ricrac/ent/notification"
	"ricrac/ent/user"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i EmailRepositoryInterface -p email_repo -s EmailRepository -o email_repository_iface.go
type EmailRepository struct {
	base *repository.BaseRepository
}

func NewEmailRepository(db *ent.Client) *EmailRepository {
	return &EmailRepository{base: repository.NewBaseRepository(db)}
}

// Queue queues an email to go out once the transaction commits.
func (self *EmailRepository) Queue(ctx context.Context, tx repository.TxInterface, to string, content json.RawMessage) error {
	return self.base.Client(tx).Email.Create().SetRecipient(to).SetContent(content).Exec(ctx)
}

// Names is the user's name and the workspace's.
func (self *EmailRepository) Names(ctx context.Context, tx repository.TxInterface, userID, workspace uuid.UUID) (string, string, error) {
	client := self.base.Client(tx)
	inviter, err := client.User.Get(ctx, userID)
	if err != nil {
		return "", "", err
	}
	found, err := client.Workspace.Get(ctx, workspace)
	if err != nil {
		return "", "", err
	}
	return inviter.Name, found.Name, nil
}

// PendingNotifications is the notifications created between after and before that are still
// unread, unarchived, not snoozed and not emailed, of people who get emails, by person then
// time, with what a digest shows of them. They stay locked until the transaction ends; ones
// locked elsewhere are skipped.
func (self *EmailRepository) PendingNotifications(ctx context.Context, tx repository.TxInterface, after, before time.Time) ([]*ent.Notification, error) {
	return self.base.Client(tx).Notification.Query().
		Where(notification.EmailedAtIsNil(), notification.CreatedAtLTE(before), notification.CreatedAtGT(after),
			notification.ReadAtIsNil(), notification.ArchivedAtIsNil(),
			func(s *entsql.Selector) {
				s.Where(repository.Raw(s.C(notification.FieldSnoozedUntil) + " IS NULL OR " + s.C(notification.FieldSnoozedUntil) + " <= now()"))
			},
			notification.HasUserWith(user.EmailNotifications(true))).
		Order(ent.Asc(notification.FieldUserID), ent.Asc(notification.FieldCreatedAt)).
		ForUpdate(entsql.WithLockAction(entsql.SkipLocked), entsql.WithLockTables(notification.Table)).
		WithUser().WithWorkspace().WithActor().
		WithIssue(func(q *ent.IssueQuery) { q.WithTeam() }).
		WithProject().WithInitiative().WithDocument().
		All(ctx)
}

// MarkEmailed marks the notifications created until before as emailed.
func (self *EmailRepository) MarkEmailed(ctx context.Context, tx repository.TxInterface, before, now time.Time) error {
	_, err := self.base.Client(tx).Notification.Update().
		Where(notification.EmailedAtIsNil(), notification.CreatedAtLTE(before)).SetEmailedAt(now).Save(ctx)
	return err
}

// Due is up to 100 unsent emails due now with fewer than attempts attempts, oldest first.
func (self *EmailRepository) Due(ctx context.Context, attempts int) ([]*ent.Email, error) {
	return self.base.DB.Email.Query().
		Where(email.SentAtIsNil(), email.AttemptsLT(attempts), func(s *entsql.Selector) {
			s.Where(repository.Raw(s.C(email.FieldSendAfter) + " <= now()"))
		}).
		Order(ent.Asc(email.FieldCreatedAt)).Limit(100).All(ctx)
}

func (self *EmailRepository) MarkSent(ctx context.Context, id uuid.UUID, now time.Time) error {
	return self.base.DB.Email.UpdateOneID(id).SetSentAt(now).AddAttempts(1).ClearError().Exec(ctx)
}

// MarkFailed notes the failure and puts the next attempt off until retryAt.
func (self *EmailRepository) MarkFailed(ctx context.Context, id uuid.UUID, reason string, retryAt time.Time) error {
	return self.base.DB.Email.UpdateOneID(id).AddAttempts(1).SetError(reason).SetSendAfter(retryAt).Exec(ctx)
}
