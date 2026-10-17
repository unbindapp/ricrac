package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type IssueSubscriber struct {
	ent.Schema
}

func (IssueSubscriber) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("issue_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
	}
}

func (IssueSubscriber) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("issue", Issue.Type).Unique().Required().Field("issue_id"),
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
	}
}

func (IssueSubscriber) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_subscribers"}, field.ID("issue_id", "user_id")}
}
