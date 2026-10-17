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
		field.UUID("project_id", uuid.UUID{}),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.Time("target_date").SchemaType(date).Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (ProjectMilestone) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).Ref("project_milestones").Field("project_id").Unique().Required(),
		edge.To("issues", Issue.Type),
		edge.To("comments", Comment.Type),
	}
}

func (ProjectMilestone) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "project_milestones"}}
}
