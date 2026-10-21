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

type Issue struct {
	ent.Schema
}

func (Issue) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}),
		field.Int("number").SchemaType(integer),
		field.Other("previous_identifiers", pgarray.Text{}).SchemaType(textArray).Optional(),
		field.Text("title"),
		field.Text("description").Optional(),
		field.UUID("state_id", uuid.UUID{}),
		field.Int("priority").SchemaType(integer).Optional(),
		field.Float("estimate").Optional().Nillable(),
		field.UUID("assignee_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("milestone_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("cycle_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("due_date").SchemaType(date).Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Float("sub_issue_sort_order").Optional().Nillable(),
		field.Text("sla_type").Optional().Nillable(),
		field.Time("sla_started_at").Optional().Nillable(),
		field.Time("sla_breaches_at").Optional().Nillable(),
		field.Time("snoozed_until").Optional().Nillable(),
		field.UUID("snoozed_by_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("canceled_at").Optional().Nillable(),
		field.Time("started_triage_at").Optional().Nillable(),
		field.Time("triaged_at").Optional().Nillable(),
		field.Time("auto_closed_at").Optional().Nillable(),
		field.Time("auto_archived_at").Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("trashed_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Issue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("issues").Field("workspace_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("issues").Field("team_id").Unique().Required(),
		edge.From("state", WorkflowState.Type).Ref("issues").Field("state_id").Unique().Required(),
		edge.From("assignee", User.Type).Ref("issues_by_assignee").Field("assignee_id").Unique(),
		edge.From("creator", User.Type).Ref("issues_by_creator").Field("creator_id").Unique(),
		edge.From("project", Project.Type).Ref("issues").Field("project_id").Unique(),
		edge.From("milestone", ProjectMilestone.Type).Ref("issues").Field("milestone_id").Unique(),
		edge.From("cycle", Cycle.Type).Ref("issues").Field("cycle_id").Unique(),
		edge.To("children", Issue.Type).From("parent").Field("parent_id").Unique(),
		edge.From("snoozed_by", User.Type).Ref("issues_by_snoozed_by").Field("snoozed_by_id").Unique(),
		edge.To("issue_relations_by_issue", IssueRelation.Type),
		edge.To("issue_relations_by_related_issue", IssueRelation.Type),
		edge.To("issue_history", IssueHistory.Type),
		edge.To("attachments", Attachment.Type),
		edge.To("documents", Document.Type),
		edge.To("comments", Comment.Type),
		edge.To("discussion_threads", DiscussionThread.Type),
		edge.To("reactions", Reaction.Type),
		edge.To("customer_needs", CustomerNeed.Type),
		edge.To("notifications", Notification.Type),
		edge.To("github_outbox", GithubOutbox.Type),
		edge.To("labels", IssueLabel.Type).Through("issue_label_links", IssueLabelLink.Type),
		edge.To("releases", Release.Type).Through("issue_releases", IssueRelease.Type),
		edge.To("shared_with", User.Type).Through("issue_shares", IssueShare.Type),
		edge.To("subscribers", User.Type).Through("issue_subscribers", IssueSubscriber.Type),
	}
}

func (Issue) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issues"}}
}
