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

type OAuthClient struct {
	ent.Schema
}

func (OAuthClient) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").Immutable(),
		field.Text("secret_hash").Optional().Nillable(),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Text("homepage_url").Optional().Nillable(),
		field.Other("redirect_uris", pgarray.Text{}).SchemaType(textArray),
		field.UUID("owner_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("workspace_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("fetched_at").Optional().Nillable(),
	}
}

func (OAuthClient) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("oauth_clients").Field("owner_id").Unique(),
		edge.From("workspace", Workspace.Type).Ref("oauth_clients").Field("workspace_id").Unique(),
		edge.To("oauth_grants", OAuthGrant.Type),
		edge.To("oauth_codes", OAuthCode.Type),
	}
}

func (OAuthClient) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "oauth_clients"}}
}
