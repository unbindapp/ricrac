package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type GithubIssueLink struct {
	ent.Schema
}

func (GithubIssueLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).StorageKey("issue_id").Immutable(),
		field.UUID("repo_sync_id", uuid.UUID{}),
		field.Int64("github_id"),
		field.Int("github_number").SchemaType(integer),
		field.Text("github_node_id"),
		field.Text("github_url"),
		field.Time("github_updated_at"),
		field.Time("created_at").Optional(),
	}
}

func (GithubIssueLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("repo_sync", GithubRepoSync.Type).Ref("github_issue_links").Field("repo_sync_id").Unique().Required(),
		edge.To("github_comment_links", GithubCommentLink.Type),
	}
}

func (GithubIssueLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_issue_links"}}
}
