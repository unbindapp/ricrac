package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type DiscussionSubscriber struct {
	ent.Schema
}

func (DiscussionSubscriber) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("discussion_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
	}
}

func (DiscussionSubscriber) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("discussion", Discussion.Type).Unique().Required().Field("discussion_id"),
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
	}
}

func (DiscussionSubscriber) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "discussion_subscribers"}, field.ID("discussion_id", "user_id")}
}
