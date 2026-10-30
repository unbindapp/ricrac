package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ProjectMilestone struct {
	ent.Schema
}

func (ProjectMilestone) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Time("target_date").SchemaType(date).Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Time("closed_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (ProjectMilestone) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("project_milestones").Field("workspace_id").Unique().Required(),
		edge.From("project", Project.Type).Ref("project_milestones").Field("project_id").Unique(),
		edge.To("issues", Issue.Type),
		edge.To("comments", Comment.Type),
		edge.To("github_milestone_links", GithubMilestoneLink.Type),
		edge.To("github_outbox", GithubOutbox.Type),
	}
}

func (ProjectMilestone) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_milestones"}}
}
