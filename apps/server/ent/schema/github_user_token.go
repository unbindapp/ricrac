package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type GithubUserToken struct {
	ent.Schema
}

func (GithubUserToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).StorageKey("user_id").Immutable(),
		field.Text("access_token"),
		field.Time("expires_at").Optional().Nillable(),
		field.Text("refresh_token").Optional().Nillable(),
		field.Time("refresh_expires_at").Optional().Nillable(),
	}
}

func (GithubUserToken) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_user_tokens"}}
}
