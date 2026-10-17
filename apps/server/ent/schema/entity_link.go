package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type EntityLink struct {
	ent.Schema
}

func (EntityLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("initiative_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("url"),
		field.Text("label"),
		field.Float("sort_order").Optional(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Optional(),
	}
}

func (EntityLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("entity_links").Field("workspace_id").Unique().Required(),
		edge.From("project", Project.Type).Ref("entity_links").Field("project_id").Unique(),
		edge.From("initiative", Initiative.Type).Ref("entity_links").Field("initiative_id").Unique(),
		edge.From("creator", User.Type).Ref("entity_links").Field("creator_id").Unique(),
	}
}

func (EntityLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "entity_links"}}
}
