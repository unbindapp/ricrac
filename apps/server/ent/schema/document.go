package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Document struct {
	ent.Schema
}

func (Document) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("workspace_id", uuid.UUID{}),
		field.Text("slug_id"),
		field.Text("title"),
		field.Text("content").Optional(),
		field.Text("icon").Optional().Nillable(),
		field.Text("color").Optional().Nillable(),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("initiative_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("team_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("issue_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("cycle_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("release_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("creator_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("updated_by_id", uuid.UUID{}).Optional().Nillable(),
		field.Float("sort_order").Optional(),
		field.Time("trashed_at").Optional().Nillable(),
		field.Time("created_at").Optional(),
		field.Time("updated_at").Optional(),
	}
}

func (Document) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("workspace", Workspace.Type).Ref("documents").Field("workspace_id").Unique().Required(),
		edge.From("project", Project.Type).Ref("documents").Field("project_id").Unique(),
		edge.From("initiative", Initiative.Type).Ref("documents").Field("initiative_id").Unique(),
		edge.From("team", Team.Type).Ref("documents").Field("team_id").Unique(),
		edge.From("issue", Issue.Type).Ref("documents").Field("issue_id").Unique(),
		edge.From("cycle", Cycle.Type).Ref("documents").Field("cycle_id").Unique(),
		edge.From("release", Release.Type).Ref("documents").Field("release_id").Unique(),
		edge.From("creator", User.Type).Ref("documents_by_creator").Field("creator_id").Unique(),
		edge.From("updated_by", User.Type).Ref("documents_by_updated_by").Field("updated_by_id").Unique(),
		edge.To("document_versions", DocumentVersion.Type),
		edge.To("comments", Comment.Type),
		edge.To("notifications", Notification.Type),
	}
}

func (Document) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "documents"}}
}
