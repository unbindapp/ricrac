package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type CustomerStatus struct {
	ent.Schema
}

func (CustomerStatus) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("name"),
		field.Text("color"),
		field.Text("description").Optional().Nillable(),
		field.Float("position"),
	}
}

func (CustomerStatus) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("customer_statuses").Field("workspace_id").Unique().Required(),
		edge.To("customers", Customer.Type),
	}
}

func (CustomerStatus) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "customer_statuses"}}
}
