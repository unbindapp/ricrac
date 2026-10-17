package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type IssueLabel struct {
	ent.Schema
}

func (IssueLabel) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("team_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Text("color"),
		field.Bool("is_group").Optional(),
		field.Text("group_type").Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("retired_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (IssueLabel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("issue_labels").Field("workspace_id").Unique().Required(),
		edge.From("team", Team.Type).Ref("issue_labels").Field("team_id").Unique(),
		edge.To("children", IssueLabel.Type).From("parent").Field("parent_id").Unique(),
		edge.From("creator", User.Type).Ref("issue_labels").Field("creator_id").Unique(),
		edge.From("issues", Issue.Type).Ref("labels").Through("issue_label_links", IssueLabelLink.Type),
	}
}

func (IssueLabel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_labels"}}
}
