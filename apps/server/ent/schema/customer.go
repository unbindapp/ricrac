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

type Customer struct {
	ent.Schema
}

func (Customer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("name"),
		field.Other("domains", pgarray.Text{}).SchemaType(textArray).Optional(),
		field.Other("external_ids", pgarray.Text{}).SchemaType(textArray).Optional(),
		field.Text("logo_url").Optional().Nillable(),
		field.UUID("owner_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("status_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("tier_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("revenue").Optional().Nillable(),
		field.Int("size").SchemaType(integer).Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Customer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("customers").Field("workspace_id").Unique().Required(),
		edge.From("owner", User.Type).Ref("customers").Field("owner_id").Unique(),
		edge.From("status", CustomerStatus.Type).Ref("customers").Field("status_id").Unique(),
		edge.From("tier", CustomerTier.Type).Ref("customers").Field("tier_id").Unique(),
		edge.To("customer_needs", CustomerNeed.Type),
	}
}

func (Customer) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "customers"}}
}
