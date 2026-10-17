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

type Invitation struct {
	ent.Schema
}

func (Invitation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("email"),
		field.Text("role"),
		field.Other("team_ids", pgarray.UUID{}).SchemaType(uuidArray).Optional(),
		field.UUID("inviter_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("expires_at"),
	}
}

func (Invitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("invitations").Field("workspace_id").Unique().Required(),
		edge.From("inviter", User.Type).Ref("invitations").Field("inviter_id").Unique(),
	}
}

func (Invitation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invitations"}}
}
