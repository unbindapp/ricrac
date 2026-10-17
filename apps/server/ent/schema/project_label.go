package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectLabel struct {
	ent.Schema
}

func (ProjectLabel) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Text("color"),
		field.Bool("is_group").Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("retired_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (ProjectLabel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("project_labels").Field("workspace_id").Unique().Required(),
		edge.To("children", ProjectLabel.Type).From("parent").Field("parent_id").Unique(),
		edge.From("creator", User.Type).Ref("project_labels").Field("creator_id").Unique(),
		edge.From("projects", Project.Type).Ref("labels").Through("project_label_links", ProjectLabelLink.Type),
	}
}

func (ProjectLabel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_labels"}}
}
