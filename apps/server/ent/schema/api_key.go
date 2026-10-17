package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"ricrac/ent/schema/pgarray"
)

type APIKey struct {
	ent.Schema
}

func (APIKey) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("user_id", uuid.UUID{}),
		field.Text("label"),
		field.Text("key_hash"),
		field.Text("key_hint"),
		field.Other("scopes", pgarray.Text{}).SchemaType(textArray),
		field.Time("created_at").Optional(),
		field.Time("last_used_at").Optional().Nillable(),
		field.Other("workspace_ids", pgarray.UUID{}).SchemaType(uuidArray).Optional().Nillable(),
	}
}

func (APIKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("api_keys").Field("user_id").Unique().Required(),
	}
}

func (APIKey) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "api_keys"}}
}
