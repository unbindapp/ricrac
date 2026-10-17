package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type File struct {
	ent.Schema
}

func (File) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("uploader_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("name"),
		field.Text("content_type"),
		field.Int64("size").Optional(),
		field.Bytes("data").Optional().Nillable(),
		field.Text("upload_hash").Optional().Nillable(),
		field.Time("upload_expires_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (File) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("files").Field("workspace_id").Unique().Required(),
		edge.From("uploader", User.Type).Ref("files").Field("uploader_id").Unique(),
	}
}

func (File) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "files"}}
}
