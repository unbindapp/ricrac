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

type OAuthCode struct {
	ent.Schema
}

func (OAuthCode) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").StorageKey("code_hash").Immutable(),
		field.Text("client_id"),
		field.UUID("user_id", uuid.UUID{}),
		field.Other("scopes", pgarray.Text{}).SchemaType(textArray),
		field.Text("redirect_uri"),
		field.Text("code_challenge").Optional().Nillable(),
		field.Text("resource").Optional().Nillable(),
		field.Time("expires_at"),
		field.Other("workspace_ids", pgarray.UUID{}).SchemaType(uuidArray).Optional().Nillable(),
	}
}

func (OAuthCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("client", OAuthClient.Type).Ref("oauth_codes").Field("client_id").Unique().Required(),
		edge.From("user", User.Type).Ref("oauth_codes").Field("user_id").Unique().Required(),
	}
}

func (OAuthCode) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "oauth_codes"}}
}
