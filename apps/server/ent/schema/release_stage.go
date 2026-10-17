package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ReleaseStage struct {
	ent.Schema
}

func (ReleaseStage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("pipeline_id", uuid.UUID{}),
		field.Text("name"),
		field.Text("type"),
		field.Text("color"),
		field.Float("position"),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (ReleaseStage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("pipeline", ReleasePipeline.Type).Ref("release_stages").Field("pipeline_id").Unique().Required(),
		edge.To("releases", Release.Type),
	}
}

func (ReleaseStage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "release_stages"}}
}
