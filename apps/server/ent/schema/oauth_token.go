package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type OAuthToken struct {
	ent.Schema
}

func (OAuthToken) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").StorageKey("token_hash").Immutable(),
		field.Text("kind"),
		field.UUID("grant_id", uuid.UUID{}),
		field.Time("expires_at"),
	}
}

func (OAuthToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("grant", OAuthGrant.Type).Ref("oauth_tokens").Field("grant_id").Unique().Required(),
	}
}

func (OAuthToken) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "oauth_tokens"}}
}
