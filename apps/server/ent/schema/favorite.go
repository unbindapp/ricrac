package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Favorite struct {
	ent.Schema
}

func (Favorite) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Text("type"),
		field.UUID("target_id", uuid.UUID{}),
		field.Text("folder").Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Time("created_at").Optional(),
	}
}

func (Favorite) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("favorites").Field("workspace_id").Unique().Required(),
		edge.From("user", User.Type).Ref("favorites").Field("user_id").Unique().Required(),
	}
}

func (Favorite) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "favorites"}}
}
