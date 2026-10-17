package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type TeamMember struct {
	ent.Schema
}

func (TeamMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("team_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Bool("owner").Optional(),
		field.Time("joined_at").Optional(),
	}
}

func (TeamMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("team", Team.Type).Unique().Required().Field("team_id"),
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
	}
}

func (TeamMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "team_members"}, field.ID("team_id", "user_id")}
}
