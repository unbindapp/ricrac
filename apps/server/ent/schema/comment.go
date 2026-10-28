package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Comment struct {
	ent.Schema
}

func (Comment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("initiative_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("document_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("milestone_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("status_update_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("discussion_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("discussion_thread_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("body"),
		field.Text("quoted_text").Optional().Nillable(),
		field.Text("suggested_text").Optional().Nillable(),
		field.Text("suggestion_status").Optional().Nillable(),
		field.Time("edited_at").Optional().Nillable(),
		field.Time("deleted_at").Optional().Nillable(),
		field.Time("resolved_at").Optional().Nillable(),
		field.UUID("resolving_user_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Comment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("comments").Field("workspace_id").Unique().Required(),
		edge.From("issue", Issue.Type).Ref("comments").Field("issue_id").Unique(),
		edge.From("project", Project.Type).Ref("comments").Field("project_id").Unique(),
		edge.From("initiative", Initiative.Type).Ref("comments").Field("initiative_id").Unique(),
		edge.From("document", Document.Type).Ref("comments").Field("document_id").Unique(),
		edge.From("milestone", ProjectMilestone.Type).Ref("comments").Field("milestone_id").Unique(),
		edge.From("status_update", StatusUpdate.Type).Ref("comments").Field("status_update_id").Unique(),
		edge.From("discussion", Discussion.Type).Ref("comments").Field("discussion_id").Unique(),
		edge.From("discussion_thread", DiscussionThread.Type).Ref("comments").Field("discussion_thread_id").Unique(),
		edge.To("children", Comment.Type).From("parent").Field("parent_id").Unique(),
		edge.From("user", User.Type).Ref("comments_by_user").Field("user_id").Unique(),
		edge.From("resolving_user", User.Type).Ref("comments_by_resolving_user").Field("resolving_user_id").Unique(),
		edge.To("reactions", Reaction.Type),
		edge.To("notifications", Notification.Type),
		edge.To("github_outbox", GithubOutbox.Type),
		edge.To("versions", MessageVersion.Type),
	}
}

func (Comment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "comments"}}
}
