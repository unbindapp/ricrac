package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type BoardIssue struct {
	ent.Schema
}

func (BoardIssue) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("board_id", uuid.UUID{}),
		field.UUID("column_id", uuid.UUID{}),
		field.UUID("issue_id", uuid.UUID{}),
		field.Float("sort_order"),
		field.Time("created_at").Optional(),
	}
}

func (BoardIssue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("board", Board.Type).Ref("board_issues").Field("board_id").Unique().Required(),
		edge.From("column", BoardColumn.Type).Ref("board_issues").Field("column_id").Unique().Required(),
		edge.From("issue", Issue.Type).Ref("board_issues").Field("issue_id").Unique().Required(),
	}
}

func (BoardIssue) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "board_issues"}}
}
