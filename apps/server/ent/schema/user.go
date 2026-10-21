package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.Text("google_subject").Optional().Nillable(),
		field.Text("email"),
		field.Text("name"),
		field.Text("display_name"),
		field.Text("avatar_url").Optional().Nillable(),
		field.Text("timezone").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("last_seen_at").Optional(),
		field.Bool("email_notifications").Optional(),
		field.Text("github_login").Optional().Nillable(),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("invitations", Invitation.Type),
		edge.To("api_keys", APIKey.Type),
		edge.To("oauth_clients", OAuthClient.Type),
		edge.To("oauth_grants", OAuthGrant.Type),
		edge.To("oauth_codes", OAuthCode.Type),
		edge.To("issue_labels", IssueLabel.Type),
		edge.To("project_labels", ProjectLabel.Type),
		edge.To("projects_by_lead", Project.Type),
		edge.To("projects_by_creator", Project.Type),
		edge.To("project_relations", ProjectRelation.Type),
		edge.To("initiatives_by_owner", Initiative.Type),
		edge.To("initiatives_by_creator", Initiative.Type),
		edge.To("status_updates", StatusUpdate.Type),
		edge.To("entity_links", EntityLink.Type),
		edge.To("releases", Release.Type),
		edge.To("release_notes", ReleaseNote.Type),
		edge.To("issues_by_assignee", Issue.Type),
		edge.To("issues_by_creator", Issue.Type),
		edge.To("issues_by_snoozed_by", Issue.Type),
		edge.To("issue_relations", IssueRelation.Type),
		edge.To("issue_history", IssueHistory.Type),
		edge.To("attachments", Attachment.Type),
		edge.To("files", File.Type),
		edge.To("documents_by_creator", Document.Type),
		edge.To("discussions_by_creator", Discussion.Type),
		edge.To("discussion_threads_by_creator", DiscussionThread.Type),
		edge.To("discussion_threads_by_resolving_user", DiscussionThread.Type),
		edge.To("documents_by_updated_by", Document.Type),
		edge.To("document_versions", DocumentVersion.Type),
		edge.To("comments_by_user", Comment.Type),
		edge.To("comments_by_resolving_user", Comment.Type),
		edge.To("reactions", Reaction.Type),
		edge.To("customers", Customer.Type),
		edge.To("customer_needs", CustomerNeed.Type),
		edge.To("custom_views", CustomView.Type),
		edge.To("favorites", Favorite.Type),
		edge.To("templates", Template.Type),
		edge.To("notifications_by_user", Notification.Type),
		edge.To("notifications_by_actor", Notification.Type),
		edge.To("webhooks", Webhook.Type),
		edge.To("audit_entries", AuditEntry.Type),
		edge.To("github_installations", GithubInstallation.Type),
		edge.From("shared_issues", Issue.Type).Ref("shared_with").Through("issue_shares", IssueShare.Type),
		edge.From("subscribed_issues", Issue.Type).Ref("subscribers").Through("issue_subscribers", IssueSubscriber.Type),
		edge.From("member_projects", Project.Type).Ref("members").Through("project_members", ProjectMember.Type),
		edge.From("subscribed_projects", Project.Type).Ref("subscribers").Through("project_subscribers", ProjectSubscriber.Type),
		edge.From("subscribed_discussions", Discussion.Type).Ref("subscribers").Through("discussion_subscribers", DiscussionSubscriber.Type),
		edge.From("teams", Team.Type).Ref("members").Through("team_members", TeamMember.Type),
		edge.From("workspaces", Workspace.Type).Ref("members").Through("workspace_members", WorkspaceMember.Type),
	}
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "users"}}
}
