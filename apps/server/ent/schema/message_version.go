package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type MessageVersion struct {
	ent.Schema
}

func (MessageVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("comment_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("discussion_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("body"),
		field.Time("written_at"),
		field.Time("created_at").Optional(),
	}
}

func (MessageVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("message_versions").Field("workspace_id").Unique().Required(),
		edge.From("comment", Comment.Type).Ref("versions").Field("comment_id").Unique(),
		edge.From("discussion", Discussion.Type).Ref("versions").Field("discussion_id").Unique(),
	}
}

func (MessageVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "message_versions"}}
}
