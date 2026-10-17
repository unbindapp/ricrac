package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type CustomerNeed struct {
	ent.Schema
}

func (CustomerNeed) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("customer_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("body").Optional().Nillable(),
		field.Int("priority").SchemaType(integer).Optional(),
		field.Text("attachment_url").Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (CustomerNeed) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("customer_needs").Field("workspace_id").Unique().Required(),
		edge.From("customer", Customer.Type).Ref("customer_needs").Field("customer_id").Unique(),
		edge.From("issue", Issue.Type).Ref("customer_needs").Field("issue_id").Unique(),
		edge.From("project", Project.Type).Ref("customer_needs").Field("project_id").Unique(),
		edge.From("creator", User.Type).Ref("customer_needs").Field("creator_id").Unique(),
	}
}

func (CustomerNeed) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "customer_needs"}}
}
