package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"ricrac/ent/schema/pgarray"
)

type ReleaseNote struct {
	ent.Schema
}

func (ReleaseNote) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("pipeline_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("title"),
		field.Text("content").Optional(),
		field.Other("release_ids", pgarray.UUID{}).SchemaType(uuidArray).Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (ReleaseNote) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("release_notes").Field("workspace_id").Unique().Required(),
		edge.From("pipeline", ReleasePipeline.Type).Ref("release_notes").Field("pipeline_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("release_notes").Field("creator_id").Unique(),
	}
}

func (ReleaseNote) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "release_notes"}}
}
