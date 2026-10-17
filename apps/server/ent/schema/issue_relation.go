package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type IssueRelation struct {
	ent.Schema
}

func (IssueRelation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("issue_id", uuid.UUID{}),
		field.UUID("related_issue_id", uuid.UUID{}),
		field.Text("type"),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (IssueRelation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("issue", Issue.Type).Ref("issue_relations_by_issue").Field("issue_id").Unique().Required(),
		edge.From("related_issue", Issue.Type).Ref("issue_relations_by_related_issue").Field("related_issue_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("issue_relations").Field("creator_id").Unique(),
	}
}

func (IssueRelation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_relations"}}
}
