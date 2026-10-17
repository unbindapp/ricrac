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

type OAuthGrant struct {
	ent.Schema
}

func (OAuthGrant) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Text("client_id"),
		field.UUID("user_id", uuid.UUID{}),
		field.Other("scopes", pgarray.Text{}).SchemaType(textArray),
		field.Time("created_at").Optional(),
		field.Time("last_used_at").Optional(),
		field.Other("workspace_ids", pgarray.UUID{}).SchemaType(uuidArray).Optional().Nillable(),
	}
}

func (OAuthGrant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("client", OAuthClient.Type).Ref("oauth_grants").Field("client_id").Unique().Required(),
		edge.From("user", User.Type).Ref("oauth_grants").Field("user_id").Unique().Required(),
		edge.To("oauth_tokens", OAuthToken.Type),
	}
}

func (OAuthGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "oauth_grants"}}
}
