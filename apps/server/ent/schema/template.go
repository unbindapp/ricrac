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

type Template struct {
	ent.Schema
}

func (Template) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("type"),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.JSON("data", json.RawMessage{}).Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
		field.JSON("recurrence", json.RawMessage{}).Optional(),
		field.Time("next_run_at").Optional().Nillable(),
	}
}

func (Template) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("templates").Field("workspace_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("templates").Field("team_id").Unique(),
		edge.From("creator", User.Type).Ref("templates").Field("creator_id").Unique(),
	}
}

func (Template) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "templates"}}
}
