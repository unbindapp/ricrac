package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type BoardColumn struct {
	ent.Schema
}

func (BoardColumn) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("board_id", uuid.UUID{}),
		field.Text("name"),
		field.Text("color").Optional().Nillable(),
		field.Float("position"),
		field.Time("created_at").Optional(),
	}
}

func (BoardColumn) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("board", Board.Type).Ref("columns").Field("board_id").Unique().Required(),
		edge.To("board_issues", BoardIssue.Type),
	}
}

func (BoardColumn) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "board_columns"}}
}
