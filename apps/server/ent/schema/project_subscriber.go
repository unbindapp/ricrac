package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectSubscriber struct {
	ent.Schema
}

func (ProjectSubscriber) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
	}
}

func (ProjectSubscriber) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("project", Project.Type).Unique().Required().Field("project_id"),
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
	}
}

func (ProjectSubscriber) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_subscribers"}, field.ID("project_id", "user_id")}
}
