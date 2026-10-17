package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Email struct {
	ent.Schema
}

func (Email) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Text("recipient"),
		field.JSON("content", json.RawMessage{}),
		field.Int("attempts").SchemaType(integer).Optional(),
		field.Text("error").Optional().Nillable(),
		field.Time("send_after").Optional(),
		field.Time("sent_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (Email) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "emails"}}
}
