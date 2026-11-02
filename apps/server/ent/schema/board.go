package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Board struct {
	ent.Schema
}

func (Board) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("name"),
		field.Text("description").Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Board) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("boards").Field("workspace_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("boards").Field("creator_id").Unique(),
		edge.To("columns", BoardColumn.Type),
		edge.To("board_issues", BoardIssue.Type),
	}
}

func (Board) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "boards"}}
}
