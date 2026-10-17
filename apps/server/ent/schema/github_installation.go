package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type GithubInstallation struct {
	ent.Schema
}

func (GithubInstallation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("account_login"),
		field.Text("account_type"),
		field.UUID("installer_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("suspended_at").Optional().Nillable(),
		field.Text("last_event").Optional().Nillable(),
		field.Time("last_event_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (GithubInstallation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("github_installations").Field("workspace_id").Unique().Required(),
		edge.From("installer", User.Type).Ref("github_installations").Field("installer_id").Unique(),
		edge.To("github_repo_syncs", GithubRepoSync.Type),
	}
}

func (GithubInstallation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "github_installations"}}
}
