package initiative_repo

import (
	"context"
	"database/sql"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/initiative"
	"ricrac/ent/initiativeproject"
	"ricrac/ent/project"
	repository "ricrac/internal/repositories"
	project_repo "ricrac/internal/repositories/project"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i InitiativeRepositoryInterface -p initiative_repo -s InitiativeRepository -o initiative_repository_iface.go
type InitiativeRepository struct {
	base *repository.BaseRepository
}

func NewInitiativeRepository(db *ent.Client) *InitiativeRepository {
	return &InitiativeRepository{base: repository.NewBaseRepository(db)}
}

func scan(ctx context.Context, client *ent.Client, read func(rows *sql.Rows) error, query string, args ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := read(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Load is the initiatives with what their answer shows, in the order of ids.
func (self *InitiativeRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Initiative, error) {
	initiatives, err := self.base.Client(tx).Initiative.Query().Where(initiative.IDIn(ids...)).
		WithWorkspace().WithOwner().WithCreator().WithParent().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Initiative, len(initiatives))
	for _, i := range initiatives {
		byID[i.ID] = i
	}
	ordered := make([]*ent.Initiative, 0, len(ids))
	for _, id := range ids {
		if i, ok := byID[id]; ok {
			ordered = append(ordered, i)
		}
	}
	return ordered, nil
}

// ProjectCounts is how many projects not in the trash each initiative has.
func (self *InitiativeRepository) ProjectCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	counts := map[uuid.UUID]int{}
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return err
		}
		counts[id] = n
		return nil
	}, `SELECT ip.initiative_id, count(*) FROM initiative_projects ip JOIN projects ipp ON ipp.id = ip.project_id
		WHERE ip.initiative_id = ANY($1) AND ipp.trashed_at IS NULL GROUP BY ip.initiative_id`, ids)
	return counts, err
}

// Progress is each initiative's progress as viewer sees it (initiative_progress).
func (self *InitiativeRepository) Progress(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, viewer uuid.UUID) (map[uuid.UUID]float64, error) {
	progress := map[uuid.UUID]float64{}
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error {
		var id uuid.UUID
		var value float64
		if err := rows.Scan(&id, &value); err != nil {
			return err
		}
		progress[id] = value
		return nil
	}, "SELECT id, initiative_progress(id, $2) FROM unnest($1::uuid[]) AS id", ids, viewer)
	return progress, err
}

type ListFilter struct {
	Workspace       uuid.UUID
	IncludeArchived bool
	IncludeTrashed  bool
	Status          *string
	Owner           *uuid.UUID
	TopLevel        bool
	Parent          *uuid.UUID
	// An ILIKE pattern on the name, summary or description.
	Pattern *string
	Limit   int64
	Offset  int64
}

// List is the initiatives the filter keeps, most recently updated first.
func (self *InitiativeRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Initiative.Query().Where(initiative.WorkspaceID(filter.Workspace))
	if !filter.IncludeArchived {
		query.Where(initiative.ArchivedAtIsNil())
	}
	if !filter.IncludeTrashed {
		query.Where(initiative.TrashedAtIsNil())
	}
	if filter.Status != nil {
		query.Where(initiative.Status(*filter.Status))
	}
	if filter.Owner != nil {
		query.Where(initiative.OwnerID(*filter.Owner))
	}
	if filter.TopLevel {
		query.Where(initiative.ParentIDIsNil())
	}
	if filter.Parent != nil {
		query.Where(initiative.ParentID(*filter.Parent))
	}
	return query.Modify(func(s *entsql.Selector) {
		if filter.Pattern != nil {
			s.Where(repository.Raw("("+s.C(initiative.FieldName)+" ILIKE ? OR "+s.C(initiative.FieldSummary)+" ILIKE ? OR "+
				s.C(initiative.FieldContent)+" ILIKE ?)", *filter.Pattern, *filter.Pattern, *filter.Pattern))
		}
		s.Select(s.C(initiative.FieldID))
		s.OrderBy(entsql.Desc(s.C(initiative.FieldUpdatedAt)))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// Projects is the initiative's projects not in the trash, in one of teams, in their order, with
// their status and lead.
func (self *InitiativeRepository) Projects(ctx context.Context, id uuid.UUID, teams []uuid.UUID) ([]*ent.Project, error) {
	return self.base.DB.Project.Query().
		Where(project.TrashedAtIsNil(), project_repo.Visible(teams)).
		WithStatus().WithLead().
		Modify(func(s *entsql.Selector) {
			links := entsql.Table(initiativeproject.Table).As("ip")
			s.Join(links).On(links.C(initiativeproject.FieldProjectID), s.C(project.FieldID))
			s.Where(entsql.EQ(links.C(initiativeproject.FieldInitiativeID), id))
			s.OrderBy(links.C(initiativeproject.FieldSortOrder), s.C(project.FieldName))
		}).All(ctx)
}

// Children is the initiative's sub-initiatives not in the trash, in their order, with their
// owners.
func (self *InitiativeRepository) Children(ctx context.Context, id uuid.UUID) ([]*ent.Initiative, error) {
	return self.base.DB.Initiative.Query().Where(initiative.ParentID(id), initiative.TrashedAtIsNil()).WithOwner().
		Order(ent.Asc(initiative.FieldSortOrder), ent.Asc(initiative.FieldName)).All(ctx)
}

type CreateInput struct {
	Workspace uuid.UUID
	SlugID    string
	Name      string
	Creator   uuid.UUID
}

// Create adds an initiative after the workspace's others in manual order.
func (self *InitiativeRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	client := self.base.Client(tx)
	var last float64
	err := scan(ctx, client, func(rows *sql.Rows) error { return rows.Scan(&last) },
		"SELECT coalesce(max(sort_order), 0) FROM initiatives WHERE workspace_id = $1", input.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	created, err := client.Initiative.Create().SetWorkspaceID(input.Workspace).SetSlugID(input.SlugID).SetName(input.Name).
		SetCreatorID(input.Creator).SetSortOrder(last + 1000).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// GetForUpdate locks the initiative until the transaction ends.
func (self *InitiativeRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Initiative, error) {
	return self.base.Client(tx).Initiative.Query().Where(initiative.ID(id)).ForUpdate().Only(ctx)
}

func (self *InitiativeRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Initiative, error) {
	return self.base.Client(tx).Initiative.Get(ctx, id)
}

// Update is the initiative's update builder, to set fields on.
func (self *InitiativeRepository) Update(tx repository.TxInterface, id uuid.UUID) *ent.InitiativeUpdateOne {
	return self.base.Client(tx).Initiative.UpdateOneID(id)
}

// InOwnTree says whether initiative is parent or one of its ancestors.
func (self *InitiativeRepository) InOwnTree(ctx context.Context, tx repository.TxInterface, parent, id uuid.UUID) (bool, error) {
	var loops bool
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error { return rows.Scan(&loops) },
		`WITH RECURSIVE up AS (SELECT id, parent_id FROM initiatives WHERE id = $1
		 UNION SELECT i.id, i.parent_id FROM initiatives i JOIN up ON i.id = up.parent_id)
		 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, parent, id)
	return loops, err
}

// UnlinkProjects takes the projects out of the initiative.
func (self *InitiativeRepository) UnlinkProjects(ctx context.Context, tx repository.TxInterface, id uuid.UUID, projects []uuid.UUID) error {
	_, err := self.base.Client(tx).InitiativeProject.Delete().
		Where(initiativeproject.InitiativeID(id), initiativeproject.ProjectIDIn(projects...)).Exec(ctx)
	return err
}

// KeepProjects takes the projects not in projects out of the initiative.
func (self *InitiativeRepository) KeepProjects(ctx context.Context, tx repository.TxInterface, id uuid.UUID, projects []uuid.UUID) error {
	_, err := self.base.Client(tx).InitiativeProject.Delete().
		Where(initiativeproject.InitiativeID(id), initiativeproject.ProjectIDNotIn(projects...)).Exec(ctx)
	return err
}

// PurgeTrash deletes the initiatives trashed more than 30 days ago.
func (self *InitiativeRepository) PurgeTrash(ctx context.Context) (int, error) {
	return self.base.DB.Initiative.Delete().Where(func(s *entsql.Selector) {
		s.Where(repository.Raw(s.C(initiative.FieldTrashedAt) + " < now() - interval '30 days'"))
	}).Exec(ctx)
}

// Trashed is the most recently trashed initiative of the workspace whose id or name is value, or
// whose slug id is slug.
func (self *InitiativeRepository) Trashed(ctx context.Context, workspace uuid.UUID, value, slug string) (uuid.UUID, bool, error) {
	found, err := self.base.DB.Initiative.Query().
		Where(initiative.WorkspaceID(workspace), initiative.TrashedAtNotNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("("+s.C(initiative.FieldID)+"::text = ? OR "+s.C(initiative.FieldSlugID)+" = ? OR lower("+
				s.C(initiative.FieldName)+") = lower(?))", value, slug, value))
		}).
		Order(ent.Desc(initiative.FieldTrashedAt)).First(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return found.ID, true, nil
}
