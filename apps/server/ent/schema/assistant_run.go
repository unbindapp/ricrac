package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// AssistantRun is one question to the assistant: who asked, in which conversation, and its cost.
type AssistantRun struct {
	ent.Schema
}

func (AssistantRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("session_id", uuid.UUID{}),
		field.Float("cost_usd"),
		field.Int("turns"),
		field.Text("outcome"),
		field.Time("created_at").Optional(),
	}
}

func (AssistantRun) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "assistant_runs"}}
}
