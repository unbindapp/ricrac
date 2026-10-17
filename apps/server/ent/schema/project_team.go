package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectTeam struct {
	ent.Schema
}

func (ProjectTeam) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}),
	}
}

func (ProjectTeam) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("project", Project.Type).Unique().Required().Field("project_id"),
		edge.To("team", Team.Type).Unique().Required().Field("team_id"),
	}
}

func (ProjectTeam) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_teams"}, field.ID("project_id", "team_id")}
}
