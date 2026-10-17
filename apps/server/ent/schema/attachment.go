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

type Attachment struct {
	ent.Schema
}

func (Attachment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("issue_id", uuid.UUID{}),
		field.Text("url"),
		field.Text("title"),
		field.Text("subtitle").Optional().Nillable(),
		field.Text("icon_url").Optional().Nillable(),
		field.JSON("metadata", json.RawMessage{}).Optional(),
		field.Text("source_type").Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Attachment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("attachments").Field("workspace_id").Unique().Required(),
		edge.From("issue", Issue.Type).Ref("attachments").Field("issue_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("attachments").Field("creator_id").Unique(),
	}
}

func (Attachment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "attachments"}}
}
