package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ReleasePipeline struct {
	ent.Schema
}

func (ReleasePipeline) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("name"),
		field.Text("type").Optional(),
		field.Bool("is_production").Optional(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (ReleasePipeline) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("release_pipelines").Field("workspace_id").Unique().Required(),
		edge.To("release_stages", ReleaseStage.Type),
		edge.To("releases", Release.Type),
		edge.To("release_notes", ReleaseNote.Type),
		edge.To("teams", Team.Type).Through("release_pipeline_teams", ReleasePipelineTeam.Type),
	}
}

func (ReleasePipeline) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "release_pipelines"}}
}
