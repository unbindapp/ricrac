package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type DocumentVersion struct {
	ent.Schema
}

func (DocumentVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(),
		field.UUID("document_id", uuid.UUID{}),
		field.Text("title"),
		field.Text("content"),
		field.UUID("author_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (DocumentVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("document", Document.Type).Ref("document_versions").Field("document_id").Unique().Required(),
		edge.From("author", User.Type).Ref("document_versions").Field("author_id").Unique(),
	}
}

func (DocumentVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "document_versions"}}
}
