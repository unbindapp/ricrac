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

type Workspace struct {
	ent.Schema
}

func (Workspace) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Text("name"),
		field.Text("url_key"),
		field.Text("logo_url").Optional().Nillable(),
		field.JSON("default_team_settings", json.RawMessage{}).Optional(),
		field.Text("project_update_reminder_day").Optional().Nillable(),
		field.Text("theme").Optional(),
		field.Bool("public").Optional(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Workspace) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("invitations", Invitation.Type),
		edge.To("oauth_clients", OAuthClient.Type),
		edge.To("teams", Team.Type),
		edge.To("issue_labels", IssueLabel.Type),
		edge.To("project_statuses", ProjectStatus.Type),
		edge.To("project_labels", ProjectLabel.Type),
		edge.To("projects", Project.Type),
		edge.To("project_milestones", ProjectMilestone.Type),
		edge.To("initiatives", Initiative.Type),
		edge.To("status_updates", StatusUpdate.Type),
		edge.To("entity_links", EntityLink.Type),
		edge.To("release_pipelines", ReleasePipeline.Type),
		edge.To("releases", Release.Type),
		edge.To("release_notes", ReleaseNote.Type),
		edge.To("issues", Issue.Type),
		edge.To("attachments", Attachment.Type),
		edge.To("files", File.Type),
		edge.To("documents", Document.Type),
		edge.To("discussions", Discussion.Type),
		edge.To("discussion_threads", DiscussionThread.Type),
		edge.To("comments", Comment.Type),
		edge.To("message_versions", MessageVersion.Type),
		edge.To("reactions", Reaction.Type),
		edge.To("customer_statuses", CustomerStatus.Type),
		edge.To("customer_tiers", CustomerTier.Type),
		edge.To("customers", Customer.Type),
		edge.To("customer_needs", CustomerNeed.Type),
		edge.To("custom_views", CustomView.Type),
		edge.To("favorites", Favorite.Type),
		edge.To("templates", Template.Type),
		edge.To("notifications", Notification.Type),
		edge.To("webhooks", Webhook.Type),
		edge.To("audit_entries", AuditEntry.Type),
		edge.To("github_installations", GithubInstallation.Type),
		edge.To("github_repo_syncs", GithubRepoSync.Type),
		edge.To("members", User.Type).Through("workspace_members", WorkspaceMember.Type),
	}
}

func (Workspace) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "workspaces"}}
}
