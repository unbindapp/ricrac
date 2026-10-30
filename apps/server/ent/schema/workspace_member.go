package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type WorkspaceMember struct {
	ent.Schema
}

func (WorkspaceMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Text("role"),
		field.Bool("active").Optional(),
		field.Time("joined_at").Optional(),
		field.Text("theme").Optional().Nillable(),
		field.Bool("assistant").Optional(),
	}
}

func (WorkspaceMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("workspace", Workspace.Type).Unique().Required().Field("workspace_id"),
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
	}
}

func (WorkspaceMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "workspace_members"}, field.ID("workspace_id", "user_id")}
}
