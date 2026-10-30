package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type GithubMilestoneLink struct {
	ent.Schema
}

func (GithubMilestoneLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("milestone_id", uuid.UUID{}),
		field.UUID("repo_sync_id", uuid.UUID{}),
		field.Int64("github_id"),
		field.Int("github_number").SchemaType(integer),
		field.Text("github_url"),
		field.Time("github_updated_at"),
		field.Time("created_at").Optional(),
	}
}

func (GithubMilestoneLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("milestone", ProjectMilestone.Type).Ref("github_milestone_links").Field("milestone_id").Unique().Required(),
		edge.From("repo_sync", GithubRepoSync.Type).Ref("github_milestone_links").Field("repo_sync_id").Unique().Required(),
	}
}

func (GithubMilestoneLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_milestone_links"}}
}
