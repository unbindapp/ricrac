package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type AuditEntry struct {
	ent.Schema
}

func (AuditEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("actor_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("type"),
		field.JSON("metadata", json.RawMessage{}).Optional(),
		field.Time("created_at").Optional(),
	}
}

func (AuditEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("audit_entries").Field("workspace_id").Unique().Required(),
		edge.From("actor", User.Type).Ref("audit_entries").Field("actor_id").Unique(),
	}
}

func (AuditEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "audit_entries"}}
}
