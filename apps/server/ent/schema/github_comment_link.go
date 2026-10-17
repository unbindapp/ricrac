package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type GithubCommentLink struct {
	ent.Schema
}

func (GithubCommentLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).StorageKey("comment_id").Immutable(),
		field.UUID("issue_id", uuid.UUID{}),
		field.Int64("github_id"),
		field.Bool("posted").Optional(),
		field.Time("created_at").Optional(),
	}
}

func (GithubCommentLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("issue", GithubIssueLink.Type).Ref("github_comment_links").Field("issue_id").Unique().Required(),
	}
}

func (GithubCommentLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_comment_links"}}
}
