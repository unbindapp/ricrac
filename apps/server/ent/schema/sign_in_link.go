package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type SignInLink struct {
	ent.Schema
}

func (SignInLink) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").StorageKey("token_hash").Immutable(),
		field.Text("email"),
		field.Text("return_to"),
		field.Time("expires_at"),
		field.Time("created_at").Optional(),
	}
}

func (SignInLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sign_in_links"}}
}
