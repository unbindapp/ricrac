package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectMember struct {
	ent.Schema
}

func (ProjectMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
	}
}

func (ProjectMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("project", Project.Type).Unique().Required().Field("project_id"),
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
	}
}

func (ProjectMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_members"}, field.ID("project_id", "user_id")}
}
