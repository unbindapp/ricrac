package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type IssueRelease struct {
	ent.Schema
}

func (IssueRelease) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("issue_id", uuid.UUID{}),
		field.UUID("release_id", uuid.UUID{}),
	}
}

func (IssueRelease) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("issue", Issue.Type).Unique().Required().Field("issue_id"),
		edge.To("release", Release.Type).Unique().Required().Field("release_id"),
	}
}

func (IssueRelease) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_releases"}, field.ID("issue_id", "release_id")}
}
