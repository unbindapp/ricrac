package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type IssueHistory struct {
	ent.Schema
}

func (IssueHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(),
		field.UUID("issue_id", uuid.UUID{}),
		field.UUID("actor_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("action"),
		field.JSON("changes", json.RawMessage{}).Optional(),
		field.Time("created_at").Optional(),
	}
}

func (IssueHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("issue", Issue.Type).Ref("issue_history").Field("issue_id").Unique().Required(),
		edge.From("actor", User.Type).Ref("issue_history").Field("actor_id").Unique(),
	}
}

func (IssueHistory) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_history"}}
}
