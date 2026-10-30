package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type GithubRepoSync struct {
	ent.Schema
}

func (GithubRepoSync) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Int64("installation_id"),
		field.Int64("repo_id"),
		field.Text("full_name"),
		field.UUID("team_id", uuid.UUID{}),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.Bool("link_back").Optional(),
		field.Text("backfill_status").Optional().Nillable(),
		field.Bool("backfill_closed").Optional(),
		field.Int("backfill_page").SchemaType(integer).Optional(),
		field.Int("issues_created").SchemaType(integer).Optional(),
		field.Int("issues_updated").SchemaType(integer).Optional(),
		field.Int("comments_created").SchemaType(integer).Optional(),
		field.Int("skipped").SchemaType(integer).Optional(),
		field.Text("last_error").Optional().Nillable(),
		field.Time("last_synced_at").Optional().Nillable(),
		field.Time("milestones_synced_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (GithubRepoSync) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("github_repo_syncs").Field("workspace_id").Unique().Required(),
		edge.From("installation", GithubInstallation.Type).Ref("github_repo_syncs").Field("installation_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("github_repo_syncs").Field("team_id").Unique().Required(),
		edge.From("project", Project.Type).Ref("github_repo_syncs").Field("project_id").Unique(),
		edge.To("github_issue_links", GithubIssueLink.Type),
		edge.To("github_outbox", GithubOutbox.Type),
		edge.To("github_milestone_links", GithubMilestoneLink.Type),
	}
}

func (GithubRepoSync) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_repo_syncs"}}
}
