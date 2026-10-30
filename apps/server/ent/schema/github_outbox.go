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

type GithubOutbox struct {
	ent.Schema
}

func (GithubOutbox) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(),
		field.UUID("repo_sync_id", uuid.UUID{}),
		field.Int("github_number").SchemaType(integer),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("comment_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("github_comment_id").Optional().Nillable(),
		field.UUID("milestone_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("kind"),
		field.JSON("changes", json.RawMessage{}).Optional(),
		field.Int("attempts").SchemaType(integer).Optional(),
		field.Time("next_attempt_at").Optional(),
		field.UUID("act_as", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (GithubOutbox) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("repo_sync", GithubRepoSync.Type).Ref("github_outbox").Field("repo_sync_id").Unique().Required(),
		edge.From("issue", Issue.Type).Ref("github_outbox").Field("issue_id").Unique(),
		edge.From("comment", Comment.Type).Ref("github_outbox").Field("comment_id").Unique(),
		edge.From("milestone", ProjectMilestone.Type).Ref("github_outbox").Field("milestone_id").Unique(),
	}
}

func (GithubOutbox) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_outbox"}}
}
