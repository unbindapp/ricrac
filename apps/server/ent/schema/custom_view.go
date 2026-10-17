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

type CustomView struct {
	ent.Schema
}

func (CustomView) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("owner_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("slug_id"),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Text("icon").Optional().Nillable(),
		field.Text("color").Optional().Nillable(),
		field.Text("model").Optional(),
		field.JSON("filter", json.RawMessage{}).Optional(),
		field.JSON("display", json.RawMessage{}).Optional(),
		field.Bool("shared").Optional(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (CustomView) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("custom_views").Field("workspace_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("custom_views").Field("team_id").Unique(),
		edge.From("owner", User.Type).Ref("custom_views").Field("owner_id").Unique(),
	}
}

func (CustomView) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "custom_views"}}
}
