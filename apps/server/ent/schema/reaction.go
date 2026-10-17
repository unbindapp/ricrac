package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Reaction struct {
	ent.Schema
}

func (Reaction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Text("emoji"),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("comment_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("status_update_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (Reaction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("reactions").Field("workspace_id").Unique().Required(),
		edge.From("user", User.Type).Ref("reactions").Field("user_id").Unique().Required(),
		edge.From("issue", Issue.Type).Ref("reactions").Field("issue_id").Unique(),
		edge.From("comment", Comment.Type).Ref("reactions").Field("comment_id").Unique(),
		edge.From("status_update", StatusUpdate.Type).Ref("reactions").Field("status_update_id").Unique(),
	}
}

func (Reaction) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "reactions"}}
}
