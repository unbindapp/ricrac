package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type DiscussionThread struct {
	ent.Schema
}

func (DiscussionThread) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("discussion_id", uuid.UUID{}),
		field.Text("kind").Optional(),
		field.Text("title"),
		field.Text("body").Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("resolved_at").Optional().Nillable(),
		field.UUID("resolving_user_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (DiscussionThread) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("discussion_threads").Field("workspace_id").Unique().Required(),
		edge.From("discussion", Discussion.Type).Ref("threads").Field("discussion_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("discussion_threads_by_creator").Field("creator_id").Unique(),
		edge.From("resolving_user", User.Type).Ref("discussion_threads_by_resolving_user").Field("resolving_user_id").Unique(),
		edge.From("issue", Issue.Type).Ref("discussion_threads").Field("issue_id").Unique(),
		edge.To("comments", Comment.Type),
		edge.To("notifications", Notification.Type),
	}
}

func (DiscussionThread) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "discussion_threads"}}
}
