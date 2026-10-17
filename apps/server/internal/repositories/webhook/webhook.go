package webhook_repo

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/schema/pgarray"
	"ricrac/ent/webhook"
	"ricrac/ent/webhookdelivery"
	"ricrac/internal/common/args"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i WebhookRepositoryInterface -p webhook_repo -s WebhookRepository -o webhook_repository_iface.go
type WebhookRepository struct {
	base *repository.BaseRepository
}

func NewWebhookRepository(db *ent.Client) *WebhookRepository {
	return &WebhookRepository{base: repository.NewBaseRepository(db)}
}

// Load is the webhook with its team and creator.
func (self *WebhookRepository) Load(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Webhook, error) {
	found, err := self.base.Client(tx).Webhook.Query().Where(webhook.ID(id)).WithTeam().WithCreator().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// List is a page of the workspace's webhooks, oldest first, with their team and creator.
func (self *WebhookRepository) List(ctx context.Context, workspace uuid.UUID, limit, offset int64) ([]*ent.Webhook, error) {
	return self.base.DB.Webhook.Query().Where(webhook.WorkspaceID(workspace)).WithTeam().WithCreator().
		Order(webhook.ByCreatedAt(), webhook.ByID()).Limit(int(limit)).Offset(int(offset)).All(ctx)
}

type CreateInput struct {
	Workspace     uuid.UUID
	Team          *uuid.UUID
	URL           string
	Label         *string
	Secret        string
	ResourceTypes []string
	Enabled       bool
	Creator       uuid.UUID
}

func (self *WebhookRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Webhook.Create().SetWorkspaceID(input.Workspace).SetNillableTeamID(input.Team).
		SetURL(input.URL).SetNillableLabel(input.Label).SetSecret(input.Secret).
		SetResourceTypes(pgarray.Text(input.ResourceTypes)).SetEnabled(input.Enabled).SetCreatorID(input.Creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// UpdateInput leaves nil fields and fields not Set as they are.
type UpdateInput struct {
	URL           *string
	Label         args.Nullable[string]
	ResourceTypes []string
	Team          args.Nullable[uuid.UUID]
	Enabled       *bool
}

func (self *WebhookRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input UpdateInput, now time.Time) error {
	update := self.base.Client(tx).Webhook.Update().Where(webhook.ID(id)).SetNillableURL(input.URL).
		SetNillableEnabled(input.Enabled).SetUpdatedAt(now)
	if input.ResourceTypes != nil {
		update.SetResourceTypes(pgarray.Text(input.ResourceTypes))
	}
	setOrClear(input.Label, update.SetLabel, update.ClearLabel)
	setOrClear(input.Team, update.SetTeamID, update.ClearTeamID)
	return update.Exec(ctx)
}

func (self *WebhookRepository) SetSecret(ctx context.Context, tx repository.TxInterface, id uuid.UUID, secret string, now time.Time) error {
	return self.base.Client(tx).Webhook.Update().Where(webhook.ID(id)).SetSecret(secret).SetUpdatedAt(now).Exec(ctx)
}

func (self *WebhookRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).Webhook.Delete().Where(webhook.ID(id)).Exec(ctx)
	return err
}

func (self *WebhookRepository) Get(ctx context.Context, id uuid.UUID) (*ent.Webhook, error) {
	return self.base.DB.Webhook.Get(ctx, id)
}

// pendingAttempt reads next_attempt_at only while the delivery is pending: a ping waits for
// 'infinity', which doesn't scan.
func pendingAttempt(s *entsql.Selector) {
	columns := make([]string, 0, len(webhookdelivery.Columns))
	for _, column := range webhookdelivery.Columns {
		if column == webhookdelivery.FieldNextAttemptAt {
			columns = append(columns, "CASE WHEN "+s.C(webhookdelivery.FieldDeliveredAt)+" IS NULL AND "+
				s.C(webhookdelivery.FieldFailedAt)+" IS NULL AND "+s.C(column)+" <> 'infinity' THEN "+s.C(column)+" END AS "+column)
			continue
		}
		columns = append(columns, s.C(column))
	}
	s.Select(columns...)
}

// Deliveries is a page of the webhook's deliveries, newest first.
func (self *WebhookRepository) Deliveries(ctx context.Context, id uuid.UUID, limit, offset int64) ([]*ent.WebhookDelivery, error) {
	return self.base.DB.WebhookDelivery.Query().Where(webhookdelivery.WebhookID(id)).
		Modify(func(s *entsql.Selector) {
			pendingAttempt(s)
			s.OrderBy(entsql.Desc(s.C(webhookdelivery.FieldCreatedAt)), s.C(webhookdelivery.FieldID))
			s.Limit(int(limit)).Offset(int(offset))
		}).All(ctx)
}

// LastDeliveries is the webhook's newest deliveries.
func (self *WebhookRepository) LastDeliveries(ctx context.Context, id uuid.UUID, count int) ([]*ent.WebhookDelivery, error) {
	return self.base.DB.WebhookDelivery.Query().Where(webhookdelivery.WebhookID(id)).
		Modify(func(s *entsql.Selector) {
			pendingAttempt(s)
			s.OrderBy(entsql.Desc(s.C(webhookdelivery.FieldCreatedAt)))
			s.Limit(count)
		}).All(ctx)
}

// QueuePing adds a delivery that is sent right away and never retried.
func (self *WebhookRepository) QueuePing(ctx context.Context, id uuid.UUID, payload json.RawMessage) (uuid.UUID, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		"INSERT INTO webhook_deliveries (webhook_id, payload, next_attempt_at) VALUES ($1, $2, 'infinity') RETURNING id", id, string(payload))
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	var delivery uuid.UUID
	if rows.Next() {
		err = rows.Scan(&delivery)
	}
	if err == nil {
		err = rows.Err()
	}
	return delivery, err
}

// setOrClear sets a value given, clears a null one and leaves one left out.
func setOrClear[T, U any](value args.Nullable[T], set func(T) U, clear func() U) {
	switch {
	case !value.Set:
	case value.Valid:
		set(value.Value)
	default:
		clear()
	}
}
