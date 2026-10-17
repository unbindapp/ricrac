package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectLabelLink struct {
	ent.Schema
}

func (ProjectLabelLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("label_id", uuid.UUID{}),
	}
}

func (ProjectLabelLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("project", Project.Type).Unique().Required().Field("project_id"),
		edge.To("label", ProjectLabel.Type).Unique().Required().Field("label_id"),
	}
}

func (ProjectLabelLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_label_links"}, field.ID("project_id", "label_id")}
}
