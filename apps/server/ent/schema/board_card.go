package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type BoardCard struct {
	ent.Schema
}

func (BoardCard) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("board_id", uuid.UUID{}),
		field.UUID("column_id", uuid.UUID{}),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("discussion_id", uuid.UUID{}).Optional().Nillable(),
		field.Float("sort_order"),
		field.Time("created_at").Optional(),
	}
}

func (BoardCard) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("board", Board.Type).Ref("board_cards").Field("board_id").Unique().Required(),
		edge.From("column", BoardColumn.Type).Ref("board_cards").Field("column_id").Unique().Required(),
		edge.From("issue", Issue.Type).Ref("board_cards").Field("issue_id").Unique(),
		edge.From("discussion", Discussion.Type).Ref("board_cards").Field("discussion_id").Unique(),
	}
}

func (BoardCard) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "board_cards"}}
}
