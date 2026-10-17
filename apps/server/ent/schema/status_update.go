package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type StatusUpdate struct {
	ent.Schema
}

func (StatusUpdate) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("initiative_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("body"),
		field.Text("health"),
		field.Time("edited_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (StatusUpdate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("status_updates").Field("workspace_id").Unique().Required(),
		edge.From("project", Project.Type).Ref("status_updates").Field("project_id").Unique(),
		edge.From("initiative", Initiative.Type).Ref("status_updates").Field("initiative_id").Unique(),
		edge.From("user", User.Type).Ref("status_updates").Field("user_id").Unique(),
		edge.To("comments", Comment.Type),
		edge.To("reactions", Reaction.Type),
		edge.To("notifications", Notification.Type),
	}
}

func (StatusUpdate) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "status_updates"}}
}
