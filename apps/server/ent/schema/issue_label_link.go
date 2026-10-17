package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type IssueLabelLink struct {
	ent.Schema
}

func (IssueLabelLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("issue_id", uuid.UUID{}),
		field.UUID("label_id", uuid.UUID{}),
	}
}

func (IssueLabelLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("issue", Issue.Type).Unique().Required().Field("issue_id"),
		edge.To("label", IssueLabel.Type).Unique().Required().Field("label_id"),
	}
}

func (IssueLabelLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_label_links"}, field.ID("issue_id", "label_id")}
}
