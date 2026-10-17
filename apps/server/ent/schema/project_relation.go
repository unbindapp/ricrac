package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectRelation struct {
	ent.Schema
}

func (ProjectRelation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("related_project_id", uuid.UUID{}),
		field.Text("type"),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (ProjectRelation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).Ref("project_relations_by_project").Field("project_id").Unique().Required(),
		edge.From("related_project", Project.Type).Ref("project_relations_by_related_project").Field("related_project_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("project_relations").Field("creator_id").Unique(),
	}
}

func (ProjectRelation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_relations"}}
}
