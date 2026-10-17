package document_repo

import (
	"context"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/document"
	"ricrac/ent/documentversion"
	"ricrac/ent/predicate"
	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
)

// Load is the documents with what their answer shows, in the order of ids.
func (self *DocumentRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Document, error) {
	documents, err := self.base.Client(tx).Document.Query().Where(document.IDIn(ids...)).
		WithWorkspace().WithProject().WithInitiative().WithTeam().
		WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() }).
		WithCycle().WithRelease().WithCreator().WithUpdatedBy().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Document, len(documents))
	for _, d := range documents {
		byID[d.ID] = d
	}
	ordered := make([]*ent.Document, 0, len(ids))
	for _, id := range ids {
		if d, ok := byID[id]; ok {
			ordered = append(ordered, d)
		}
	}
	return ordered, nil
}

func (self *DocumentRepository) VersionCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).DocumentVersion.Query().Where(documentversion.DocumentID(id)).Count(ctx)
}

// Visible is SQL true when the caller sees document d (a table or alias): by its team, project,
// issue or cycle; documents of initiatives and releases are visible to everyone. Its arguments
// follow, for repository.Raw.
func Visible(d string, teams []uuid.UUID, viewer uuid.UUID) (string, []any) {
	sql := fmt.Sprintf(`CASE WHEN %[1]s.team_id IS NOT NULL THEN %[1]s.team_id = ANY(?)
		WHEN %[1]s.project_id IS NOT NULL THEN EXISTS (SELECT 1 FROM projects vp JOIN project_teams vpt ON vpt.project_id = vp.id
			WHERE vp.id = %[1]s.project_id AND vp.trashed_at IS NULL AND vpt.team_id = ANY(?))
		WHEN %[1]s.issue_id IS NOT NULL THEN EXISTS (SELECT 1 FROM issues vi WHERE vi.id = %[1]s.issue_id AND (vi.team_id = ANY(?)
			OR EXISTS (SELECT 1 FROM issue_shares vs WHERE vs.issue_id = vi.id AND vs.user_id = ?)))
		WHEN %[1]s.cycle_id IS NOT NULL THEN EXISTS (SELECT 1 FROM cycles vc WHERE vc.id = %[1]s.cycle_id AND vc.team_id = ANY(?))
		ELSE TRUE END`, d)
	return sql, []any{teams, teams, teams, viewer, teams}
}

// Parent is a column naming what documents belong to, and its value.
type Parent struct {
	Column string
	ID     uuid.UUID
}

type ListFilter struct {
	Workspace uuid.UUID
	// The teams the caller sees, and the caller, who also sees the documents of issues shared with
	// them.
	Teams          []uuid.UUID
	Viewer         uuid.UUID
	Parents        []Parent
	Creator        *uuid.UUID
	IncludeTrashed bool
	// Words to find in the title or content.
	Query *string
	// updated_at or created_at, newest first.
	OrderBy string
	Limit   int64
	Offset  int64
}

func (self *DocumentRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Document.Query().Where(document.WorkspaceID(filter.Workspace), func(s *entsql.Selector) {
		sql, args := Visible(s.TableName(), filter.Teams, filter.Viewer)
		s.Where(repository.Raw(sql, args...))
		for _, p := range filter.Parents {
			s.Where(entsql.EQ(s.C(p.Column), p.ID))
		}
	})
	if filter.Creator != nil {
		query = query.Where(document.CreatorID(*filter.Creator))
	}
	if !filter.IncludeTrashed {
		query = query.Where(document.TrashedAtIsNil())
	}
	if filter.Query != nil {
		words := *filter.Query
		query = query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw("("+s.C("search")+" @@ websearch_to_tsquery('english', ?) OR "+s.C(document.FieldTitle)+" ILIKE ?)",
				words, "%"+utils.EscapeLike(words)+"%"))
		})
	}
	return query.Order(ent.Desc(filter.OrderBy)).Limit(int(filter.Limit)).Offset(int(filter.Offset)).IDs(ctx)
}

// Trashed is the document in the trash most recently trashed whose id (asID), slug id or title
// (in any case) matches.
func (self *DocumentRepository) Trashed(ctx context.Context, workspace uuid.UUID, asID *uuid.UUID, slug, title string) (uuid.UUID, bool, error) {
	matches := []predicate.Document{document.SlugID(slug), func(s *entsql.Selector) {
		s.Where(repository.Raw("lower("+s.C(document.FieldTitle)+") = lower(?)", title))
	}}
	if asID != nil {
		matches = append(matches, document.ID(*asID))
	}
	ids, err := self.base.DB.Document.Query().
		Where(document.WorkspaceID(workspace), document.TrashedAtNotNil(), document.Or(matches...)).
		Order(ent.Desc(document.FieldTrashedAt)).Limit(1).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}

// WebhookTeam is the team a document's webhooks are about: its team, or its issue's or cycle's.
func (self *DocumentRepository) WebhookTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*uuid.UUID, error) {
	found, err := self.base.Client(tx).Document.Query().Where(document.ID(id)).WithIssue().WithCycle().Only(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case found.TeamID != nil:
		return found.TeamID, nil
	case found.Edges.Issue != nil:
		return &found.Edges.Issue.TeamID, nil
	case found.Edges.Cycle != nil:
		return &found.Edges.Cycle.TeamID, nil
	}
	return nil, nil
}

// GetForUpdate locks the document until the transaction ends.
func (self *DocumentRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Document, error) {
	return self.base.Client(tx).Document.Query().Where(document.ID(id)).ForUpdate().Only(ctx)
}

// Version is the document's version with this id, nil when it has none.
func (self *DocumentRepository) Version(ctx context.Context, tx repository.TxInterface, document uuid.UUID, id int64) (*ent.DocumentVersion, error) {
	found, err := self.base.Client(tx).DocumentVersion.Query().
		Where(documentversion.ID(id), documentversion.DocumentID(document)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// HasRecentVersion says whether the document has a version less than 10 minutes old.
func (self *DocumentRepository) HasRecentVersion(ctx context.Context, tx repository.TxInterface, document uuid.UUID) (bool, error) {
	return self.base.Client(tx).DocumentVersion.Query().Where(documentversion.DocumentID(document), func(s *entsql.Selector) {
		s.Where(entsql.ExprP(s.C(documentversion.FieldCreatedAt) + " > now() - interval '10 minutes'"))
	}).Exist(ctx)
}

// Versions is the document's versions with their author, newest first.
func (self *DocumentRepository) Versions(ctx context.Context, document uuid.UUID) ([]*ent.DocumentVersion, error) {
	return self.base.DB.DocumentVersion.Query().Where(documentversion.DocumentID(document)).WithAuthor().
		Order(ent.Desc(documentversion.FieldID)).All(ctx)
}

// VersionInWorkspace is a version of a document of the workspace, with its document and author,
// nil when there's none.
func (self *DocumentRepository) VersionInWorkspace(ctx context.Context, id int64, workspace uuid.UUID) (*ent.DocumentVersion, error) {
	found, err := self.base.DB.DocumentVersion.Query().
		Where(documentversion.ID(id), documentversion.HasDocumentWith(document.WorkspaceID(workspace))).
		WithDocument().WithAuthor().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}
