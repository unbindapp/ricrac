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

type Discussion struct {
	ent.Schema
}

func (Discussion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("title"),
		field.Text("body").Optional(),
		field.Text("thumbnail_url").Optional().Nillable(),
		field.Other("tags", pgarray.Text{}).SchemaType(textArray).Optional(),
		field.UUID("team_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("pinned_at").Optional().Nillable(),
		field.Time("closed_at").Optional().Nillable(),
		field.Time("trashed_at").Optional().Nillable(),
		field.Time("last_activity_at").Optional(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Discussion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("discussions").Field("workspace_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("discussions").Field("team_id").Unique(),
		edge.From("creator", User.Type).Ref("discussions_by_creator").Field("creator_id").Unique(),
		edge.To("threads", DiscussionThread.Type),
		edge.To("comments", Comment.Type),
		edge.To("notifications", Notification.Type),
		edge.To("subscribers", User.Type).Through("discussion_subscribers", DiscussionSubscriber.Type),
	}
}

func (Discussion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "discussions"}}
}
