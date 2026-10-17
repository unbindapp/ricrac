package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Cycle struct {
	ent.Schema
}

func (Cycle) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("team_id", uuid.UUID{}),
		field.Int("number").SchemaType(integer),
		field.Text("name").Optional().Nillable(),
		field.Text("description").Optional().Nillable(),
		field.Time("starts_at"),
		field.Time("ends_at"),
		field.Time("completed_at").Optional().Nillable(),
		field.JSON("history", json.RawMessage{}).Optional(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Cycle) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("team", Team.Type).Ref("cycles").Field("team_id").Unique().Required(),
		edge.To("issues", Issue.Type),
		edge.To("documents", Document.Type),
	}
}

func (Cycle) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "cycles"}}
}
