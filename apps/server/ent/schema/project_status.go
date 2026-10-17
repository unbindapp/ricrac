package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectStatus struct {
	ent.Schema
}

func (ProjectStatus) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("name"),
		field.Text("type"),
		field.Text("color"),
		field.Text("description").Optional().Nillable(),
		field.Float("position"),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (ProjectStatus) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("project_statuses").Field("workspace_id").Unique().Required(),
		edge.To("projects", Project.Type),
	}
}

func (ProjectStatus) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_statuses"}}
}
