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

type Team struct {
	ent.Schema
}

func (Team) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("key"),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Text("icon").Optional().Nillable(),
		field.Text("color").Optional().Nillable(),
		field.Bool("private").Optional(),
		field.Text("timezone").Optional(),
		field.Int("issue_count").SchemaType(integer).Optional(),
		field.Text("estimate_type").Optional(),
		field.Bool("estimate_allow_zero").Optional(),
		field.Bool("estimate_extended").Optional(),
		field.Float("default_estimate").Optional().Nillable(),
		field.Bool("cycles_enabled").Optional(),
		field.Int("cycle_duration_weeks").SchemaType(integer).Optional(),
		field.Int("cycle_cooldown_weeks").SchemaType(integer).Optional(),
		field.Int("cycle_start_day").SchemaType(integer).Optional(),
		field.Int("upcoming_cycle_count").SchemaType(integer).Optional(),
		field.Bool("cycle_auto_add_started").Optional(),
		field.Bool("cycle_auto_add_completed").Optional(),
		field.Bool("cycle_rollover").Optional(),
		field.Bool("triage_enabled").Optional(),
		field.Text("triage_action").Optional(),
		field.Other("triage_responsible_ids", pgarray.UUID{}).SchemaType(uuidArray).Optional(),
		field.Int("triage_rotation_weeks").SchemaType(integer).Optional().Nillable(),
		field.Time("triage_rotation_start").SchemaType(date).Optional().Nillable(),
		field.Bool("require_priority_to_leave_triage").Optional(),
		field.Int("auto_archive_months").SchemaType(integer).Optional().Nillable(),
		field.Int("auto_close_months").SchemaType(integer).Optional().Nillable(),
		field.UUID("auto_close_state_id", uuid.UUID{}).Optional().Nillable(),
		field.Bool("auto_close_parent_issues").Optional(),
		field.Bool("auto_close_child_issues").Optional(),
		field.UUID("default_issue_state_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("default_template_member_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("default_template_non_member_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("default_project_template_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
		field.UUID("github_pr_opened_state_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("github_pr_merged_state_id", uuid.UUID{}).Optional().Nillable(),
	}
}

func (Team) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("teams").Field("workspace_id").Unique().Required(),
		edge.To("children", Team.Type).From("parent").Field("parent_id").Unique(),
		edge.From("auto_close_state", WorkflowState.Type).Ref("teams_by_auto_close_state").Field("auto_close_state_id").Unique(),
		edge.From("default_issue_state", WorkflowState.Type).Ref("teams_by_default_issue_state").Field("default_issue_state_id").Unique(),
		edge.From("github_pr_opened_state", WorkflowState.Type).Ref("teams_by_github_pr_opened_state").Field("github_pr_opened_state_id").Unique(),
		edge.From("github_pr_merged_state", WorkflowState.Type).Ref("teams_by_github_pr_merged_state").Field("github_pr_merged_state_id").Unique(),
		edge.To("workflow_states", WorkflowState.Type),
		edge.To("issue_labels", IssueLabel.Type),
		edge.To("projects_by_lead_team", Project.Type),
		edge.To("cycles", Cycle.Type),
		edge.To("issues", Issue.Type),
		edge.To("documents", Document.Type),
		edge.To("custom_views", CustomView.Type),
		edge.To("templates", Template.Type),
		edge.To("webhooks", Webhook.Type),
		edge.To("github_repo_syncs", GithubRepoSync.Type),
		edge.From("projects", Project.Type).Ref("teams").Through("project_teams", ProjectTeam.Type),
		edge.From("release_pipelines", ReleasePipeline.Type).Ref("teams").Through("release_pipeline_teams", ReleasePipelineTeam.Type),
		edge.To("members", User.Type).Through("team_members", TeamMember.Type),
	}
}

func (Team) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "teams"}}
}
