package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Notification struct {
	ent.Schema
}

func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("actor_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("type"),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("comment_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("initiative_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("document_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("status_update_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("discussion_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("discussion_thread_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("summary"),
		field.Time("read_at").Optional().Nillable(),
		field.Time("snoozed_until").Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("emailed_at").Optional().Nillable(),
	}
}

func (Notification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("notifications").Field("workspace_id").Unique().Required(),
		edge.From("user", User.Type).Ref("notifications_by_user").Field("user_id").Unique().Required(),
		edge.From("actor", User.Type).Ref("notifications_by_actor").Field("actor_id").Unique(),
		edge.From("issue", Issue.Type).Ref("notifications").Field("issue_id").Unique(),
		edge.From("comment", Comment.Type).Ref("notifications").Field("comment_id").Unique(),
		edge.From("project", Project.Type).Ref("notifications").Field("project_id").Unique(),
		edge.From("initiative", Initiative.Type).Ref("notifications").Field("initiative_id").Unique(),
		edge.From("document", Document.Type).Ref("notifications").Field("document_id").Unique(),
		edge.From("status_update", StatusUpdate.Type).Ref("notifications").Field("status_update_id").Unique(),
		edge.From("discussion", Discussion.Type).Ref("notifications").Field("discussion_id").Unique(),
		edge.From("discussion_thread", DiscussionThread.Type).Ref("notifications").Field("discussion_thread_id").Unique(),
	}
}

func (Notification) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "notifications"}}
}
