package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Release struct {
	ent.Schema
}

func (Release) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("pipeline_id", uuid.UUID{}),
		field.UUID("stage_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("slug_id"),
		field.Text("name"),
		field.Text("version").Optional().Nillable(),
		field.Text("description").Optional().Nillable(),
		field.Text("commit_sha").Optional().Nillable(),
		field.Time("start_date").SchemaType(date).Optional().Nillable(),
		field.Time("target_date").SchemaType(date).Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("canceled_at").Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Release) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("releases").Field("workspace_id").Unique().Required(),
		edge.From("pipeline", ReleasePipeline.Type).Ref("releases").Field("pipeline_id").Unique().Required(),
		edge.From("stage", ReleaseStage.Type).Ref("releases").Field("stage_id").Unique(),
		edge.From("creator", User.Type).Ref("releases").Field("creator_id").Unique(),
		edge.To("documents", Document.Type),
		edge.From("issues", Issue.Type).Ref("releases").Through("issue_releases", IssueRelease.Type),
	}
}

func (Release) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "releases"}}
}
