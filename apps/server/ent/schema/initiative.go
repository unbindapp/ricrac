package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Initiative struct {
	ent.Schema
}

func (Initiative) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("name"),
		field.Text("summary").Optional().Nillable(),
		field.Text("content").Optional(),
		field.Text("icon").Optional().Nillable(),
		field.Text("color").Optional().Nillable(),
		field.Text("status").Optional(),
		field.UUID("owner_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("target_date").SchemaType(date).Optional().Nillable(),
		field.Text("target_date_resolution").Optional().Nillable(),
		field.Text("health").Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("trashed_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Initiative) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("initiatives").Field("workspace_id").Unique().Required(),
		edge.To("children", Initiative.Type).From("parent").Field("parent_id").Unique(),
		edge.From("owner", User.Type).Ref("initiatives_by_owner").Field("owner_id").Unique(),
		edge.From("creator", User.Type).Ref("initiatives_by_creator").Field("creator_id").Unique(),
		edge.To("status_updates", StatusUpdate.Type),
		edge.To("entity_links", EntityLink.Type),
		edge.To("documents", Document.Type),
		edge.To("comments", Comment.Type),
		edge.To("notifications", Notification.Type),
		edge.To("projects", Project.Type).Through("initiative_projects", InitiativeProject.Type),
	}
}

func (Initiative) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "initiatives"}}
}
