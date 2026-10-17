package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Project struct {
	ent.Schema
}

func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("name"),
		field.Text("summary").Optional().Nillable(),
		field.Text("content").Optional(),
		field.Text("icon").Optional().Nillable(),
		field.Text("color").Optional().Nillable(),
		field.UUID("status_id", uuid.UUID{}),
		field.Int("priority").SchemaType(integer).Optional(),
		field.UUID("lead_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("lead_team_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("start_date").SchemaType(date).Optional().Nillable(),
		field.Text("start_date_resolution").Optional().Nillable(),
		field.Time("target_date").SchemaType(date).Optional().Nillable(),
		field.Text("target_date_resolution").Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Text("health").Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("canceled_at").Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("trashed_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("projects").Field("workspace_id").Unique().Required(),
		edge.From("status", ProjectStatus.Type).Ref("projects").Field("status_id").Unique().Required(),
		edge.From("lead", User.Type).Ref("projects_by_lead").Field("lead_id").Unique(),
		edge.From("lead_team", Team.Type).Ref("projects_by_lead_team").Field("lead_team_id").Unique(),
		edge.From("creator", User.Type).Ref("projects_by_creator").Field("creator_id").Unique(),
		edge.To("project_relations_by_project", ProjectRelation.Type),
		edge.To("project_relations_by_related_project", ProjectRelation.Type),
		edge.To("project_milestones", ProjectMilestone.Type),
		edge.To("status_updates", StatusUpdate.Type),
		edge.To("entity_links", EntityLink.Type),
		edge.To("issues", Issue.Type),
		edge.To("documents", Document.Type),
		edge.To("comments", Comment.Type),
		edge.To("customer_needs", CustomerNeed.Type),
		edge.To("notifications", Notification.Type),
		edge.To("github_repo_syncs", GithubRepoSync.Type),
		edge.From("initiatives", Initiative.Type).Ref("projects").Through("initiative_projects", InitiativeProject.Type),
		edge.To("labels", ProjectLabel.Type).Through("project_label_links", ProjectLabelLink.Type),
		edge.To("members", User.Type).Through("project_members", ProjectMember.Type),
		edge.To("subscribers", User.Type).Through("project_subscribers", ProjectSubscriber.Type),
		edge.To("teams", Team.Type).Through("project_teams", ProjectTeam.Type),
	}
}

func (Project) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "projects"}}
}
