package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type WebhookDelivery struct {
	ent.Schema
}

func (WebhookDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("webhook_id", uuid.UUID{}),
		field.JSON("payload", json.RawMessage{}),
		field.Int("attempts").SchemaType(integer).Optional(),
		field.Time("next_attempt_at").Optional(),
		field.Int("status_code").SchemaType(integer).Optional().Nillable(),
		field.Text("error").Optional().Nillable(),
		field.Time("delivered_at").Optional().Nillable(),
		field.Time("failed_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (WebhookDelivery) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("webhook", Webhook.Type).Ref("webhook_deliveries").Field("webhook_id").Unique().Required(),
	}
}

func (WebhookDelivery) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "webhook_deliveries"}}
}
