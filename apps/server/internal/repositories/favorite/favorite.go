package favorite_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customview"
	"ricrac/ent/favorite"
	repository "ricrac/internal/repositories"
	document_repo "ricrac/internal/repositories/document"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i FavoriteRepositoryInterface -p favorite_repo -s FavoriteRepository -o favorite_repository_iface.go
type FavoriteRepository struct {
	base *repository.BaseRepository
}

func NewFavoriteRepository(db *ent.Client) *FavoriteRepository {
	return &FavoriteRepository{base: repository.NewBaseRepository(db)}
}

type ListFilter struct {
	Workspace uuid.UUID
	User      uuid.UUID
	// The teams the user sees.
	Teams  []uuid.UUID
	Limit  int64
	Offset int64
}

// visible is SQL true when what favorite f points at is still there and the user sees it.
func visible(f string, teams []uuid.UUID, user uuid.UUID) (string, []any) {
	documentVisible, documentArgs := document_repo.Visible("d", teams, user)
	sql := `CASE ` + f + `.type
		WHEN 'issue' THEN EXISTS (SELECT 1 FROM issues i WHERE i.id = ` + f + `.target_id AND i.trashed_at IS NULL AND (i.team_id = ANY(?)
			OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = i.id AND s.user_id = ?)))
		WHEN 'project' THEN EXISTS (SELECT 1 FROM projects p JOIN project_teams pt ON pt.project_id = p.id
			WHERE p.id = ` + f + `.target_id AND p.trashed_at IS NULL AND pt.team_id = ANY(?))
		WHEN 'initiative' THEN EXISTS (SELECT 1 FROM initiatives n WHERE n.id = ` + f + `.target_id AND n.trashed_at IS NULL)
		WHEN 'cycle' THEN EXISTS (SELECT 1 FROM cycles c WHERE c.id = ` + f + `.target_id AND c.team_id = ANY(?))
		WHEN 'custom_view' THEN EXISTS (SELECT 1 FROM custom_views v WHERE v.id = ` + f + `.target_id
			AND (v.shared OR v.owner_id = ` + f + `.user_id) AND (v.team_id IS NULL OR v.team_id = ANY(?)))
		WHEN 'document' THEN EXISTS (SELECT 1 FROM documents d WHERE d.id = ` + f + `.target_id AND d.trashed_at IS NULL
			AND ` + documentVisible + `)
		WHEN 'label' THEN EXISTS (SELECT 1 FROM issue_labels l WHERE l.id = ` + f + `.target_id AND (l.team_id IS NULL OR l.team_id = ANY(?)))
		WHEN 'team' THEN ` + f + `.target_id = ANY(?)
		WHEN 'user' THEN EXISTS (SELECT 1 FROM workspace_members m WHERE m.user_id = ` + f + `.target_id
			AND m.workspace_id = ` + f + `.workspace_id AND m.active)
		WHEN 'release' THEN EXISTS (SELECT 1 FROM releases r WHERE r.id = ` + f + `.target_id)
		ELSE FALSE END`
	args := []any{teams, user, teams, teams, teams}
	args = append(args, documentArgs...)
	return sql, append(args, teams, teams)
}

// List is the user's favorites whose target is still there and visible, by folder (none first),
// order and age.
func (self *FavoriteRepository) List(ctx context.Context, filter ListFilter) ([]*ent.Favorite, error) {
	return self.base.DB.Favorite.Query().
		Where(favorite.WorkspaceID(filter.Workspace), favorite.UserID(filter.User), func(s *entsql.Selector) {
			sql, args := visible(s.TableName(), filter.Teams, filter.User)
			s.Where(repository.Raw(sql, args...))
		}).
		Order(favorite.ByFolder(entsql.OrderNullsFirst()), favorite.BySortOrder(), favorite.ByCreatedAt()).
		Limit(int(filter.Limit)).Offset(int(filter.Offset)).All(ctx)
}

func (self *FavoriteRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Favorite, error) {
	return self.base.Client(tx).Favorite.Get(ctx, id)
}

type AddInput struct {
	Workspace uuid.UUID
	User      uuid.UUID
	Type      string
	Target    uuid.UUID
	Folder    *string
	// After the user's other favorites when nil.
	SortOrder *float64
}

// Add adds a favorite, or updates its folder (when given) and order (when given) when the user
// already has it.
func (self *FavoriteRepository) Add(ctx context.Context, tx repository.TxInterface, input AddInput) (uuid.UUID, error) {
	client := self.base.Client(tx)
	order := input.SortOrder
	if order == nil {
		var rows []struct {
			Max *float64 `json:"max"`
		}
		err := client.Favorite.Query().Where(favorite.UserID(input.User), favorite.WorkspaceID(input.Workspace)).
			Aggregate(ent.Max(favorite.FieldSortOrder)).Scan(ctx, &rows)
		if err != nil {
			return uuid.Nil, err
		}
		last := 0.0
		if len(rows) > 0 && rows[0].Max != nil {
			last = *rows[0].Max
		}
		order = new(last + 1000)
	}
	return client.Favorite.Create().SetWorkspaceID(input.Workspace).SetUserID(input.User).SetType(input.Type).
		SetTargetID(input.Target).SetNillableFolder(input.Folder).SetSortOrder(*order).
		OnConflict(entsql.ConflictColumns(favorite.FieldUserID, favorite.FieldType, favorite.FieldTargetID),
			entsql.ResolveWith(func(u *entsql.UpdateSet) {
				u.Set(favorite.FieldFolder, entsql.Expr("coalesce(EXCLUDED.folder, favorites.folder)"))
				if input.SortOrder != nil {
					u.SetExcluded(favorite.FieldSortOrder)
					return
				}
				u.Set(favorite.FieldSortOrder, entsql.Expr("favorites.sort_order"))
			})).
		ID(ctx)
}

// Remove deletes the user's favorite with this id, false when there's none.
func (self *FavoriteRepository) Remove(ctx context.Context, id, user, workspace uuid.UUID) (bool, error) {
	removed, err := self.base.DB.Favorite.Delete().
		Where(favorite.ID(id), favorite.UserID(user), favorite.WorkspaceID(workspace)).Exec(ctx)
	return removed > 0, err
}

// RemoveTarget deletes the user's favorite of a target, answering its id; false when there's none.
func (self *FavoriteRepository) RemoveTarget(ctx context.Context, user, workspace uuid.UUID, kind string, target uuid.UUID) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Favorite.Query().
		Where(favorite.UserID(user), favorite.WorkspaceID(workspace), favorite.Type(kind), favorite.TargetID(target)).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	if _, err := self.base.DB.Favorite.Delete().Where(favorite.IDIn(ids...)).Exec(ctx); err != nil {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}

// ViewTeam is the team of a custom view, nil for a workspace view.
func (self *FavoriteRepository) ViewTeam(ctx context.Context, id uuid.UUID) (*uuid.UUID, error) {
	view, err := self.base.DB.CustomView.Query().Where(customview.ID(id)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return view.TeamID, nil
}
