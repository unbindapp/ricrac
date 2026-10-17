package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ReleasePipelineTeam struct {
	ent.Schema
}

func (ReleasePipelineTeam) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("pipeline_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}),
	}
}

func (ReleasePipelineTeam) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("pipeline", ReleasePipeline.Type).Unique().Required().Field("pipeline_id"),
		edge.To("team", Team.Type).Unique().Required().Field("team_id"),
	}
}

func (ReleasePipelineTeam) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "release_pipeline_teams"}, field.ID("pipeline_id", "team_id")}
}
