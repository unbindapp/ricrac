package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type InitiativeProject struct {
	ent.Schema
}

func (InitiativeProject) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("initiative_id", uuid.UUID{}),
		field.UUID("project_id", uuid.UUID{}),
		field.Float("sort_order").Optional(),
	}
}

func (InitiativeProject) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("initiative", Initiative.Type).Unique().Required().Field("initiative_id"),
		edge.To("project", Project.Type).Unique().Required().Field("project_id"),
	}
}

func (InitiativeProject) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "initiative_projects"}, field.ID("initiative_id", "project_id")}
}
