package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type WorkflowState struct {
	ent.Schema
}

func (WorkflowState) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("team_id", uuid.UUID{}),
		field.Text("name"),
		field.Text("type"),
		field.Text("color"),
		field.Text("description").Optional().Nillable(),
		field.Float("position"),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (WorkflowState) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("team", Team.Type).Ref("workflow_states").Field("team_id").Unique().Required(),
		edge.To("teams_by_auto_close_state", Team.Type),
		edge.To("teams_by_default_issue_state", Team.Type),
		edge.To("issues", Issue.Type),
		edge.To("teams_by_github_pr_opened_state", Team.Type),
		edge.To("teams_by_github_pr_merged_state", Team.Type),
	}
}

func (WorkflowState) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "workflow_states"}}
}
