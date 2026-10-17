package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"ricrac/ent/schema/pgarray"
)

type Webhook struct {
	ent.Schema
}

func (Webhook) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("url"),
		field.Text("label").Optional().Nillable(),
		field.Text("secret"),
		field.Other("resource_types", pgarray.Text{}).SchemaType(textArray),
		field.Bool("enabled").Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Webhook) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("webhooks").Field("workspace_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("webhooks").Field("team_id").Unique(),
		edge.From("creator", User.Type).Ref("webhooks").Field("creator_id").Unique(),
		edge.To("webhook_deliveries", WebhookDelivery.Type),
	}
}

func (Webhook) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "webhooks"}}
}
