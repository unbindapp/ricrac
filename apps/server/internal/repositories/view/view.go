package view_repo

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customview"
	"ricrac/ent/favorite"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ViewRepositoryInterface -p view_repo -s ViewRepository -o view_repository_iface.go
type ViewRepository struct {
	base *repository.BaseRepository
}

func NewViewRepository(db *ent.Client) *ViewRepository {
	return &ViewRepository{base: repository.NewBaseRepository(db)}
}

// Load is the view with what its answer shows.
func (self *ViewRepository) Load(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.CustomView, error) {
	return self.base.Client(tx).CustomView.Query().Where(customview.ID(id)).
		WithWorkspace().WithTeam().WithOwner().Only(ctx)
}

// LoadAll is the views with what their answers show, in the order of ids.
func (self *ViewRepository) LoadAll(ctx context.Context, ids []uuid.UUID) ([]*ent.CustomView, error) {
	views, err := self.base.DB.CustomView.Query().Where(customview.IDIn(ids...)).
		WithWorkspace().WithTeam().WithOwner().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.CustomView, len(views))
	for _, v := range views {
		byID[v.ID] = v
	}
	ordered := make([]*ent.CustomView, 0, len(ids))
	for _, id := range ids {
		if v, ok := byID[id]; ok {
			ordered = append(ordered, v)
		}
	}
	return ordered, nil
}

func (self *ViewRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.CustomView, error) {
	return self.base.Client(tx).CustomView.Get(ctx, id)
}

type ListFilter struct {
	Workspace uuid.UUID
	Viewer    uuid.UUID
	// Views of no team or of these.
	Teams  []uuid.UUID
	Model  *string
	Team   *uuid.UUID
	Shared *bool
	// An ILIKE pattern on the name or description.
	Pattern *string
	Limit   int64
	Offset  int64
}

// List is the views shared with the workspace or the viewer's, by name.
func (self *ViewRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.CustomView.Query().Where(
		customview.WorkspaceID(filter.Workspace),
		customview.Or(customview.Shared(true), customview.OwnerID(filter.Viewer)),
		customview.Or(customview.TeamIDIsNil(), customview.TeamIDIn(filter.Teams...)))
	if filter.Model != nil {
		query = query.Where(customview.Model(*filter.Model))
	}
	if filter.Team != nil {
		query = query.Where(customview.TeamID(*filter.Team))
	}
	if filter.Shared != nil && *filter.Shared {
		query = query.Where(customview.Shared(true))
	}
	if filter.Shared != nil && !*filter.Shared {
		query = query.Where(customview.Not(customview.Shared(true)), customview.OwnerID(filter.Viewer))
	}
	return query.Modify(func(s *entsql.Selector) {
		if filter.Pattern != nil {
			s.Where(repository.Raw(s.C(customview.FieldName)+" ILIKE ? OR "+s.C(customview.FieldDescription)+" ILIKE ?", *filter.Pattern, *filter.Pattern))
		}
		s.Select(s.C(customview.FieldID))
		s.OrderExpr(entsql.Expr("lower(" + s.C(customview.FieldName) + ")"))
		s.OrderExpr(entsql.Expr(s.C(customview.FieldCreatedAt)))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

type NewView struct {
	Workspace, Owner    uuid.UUID
	SlugID, Name, Model string
}

func (self *ViewRepository) Create(ctx context.Context, tx repository.TxInterface, input NewView) (uuid.UUID, error) {
	created, err := self.base.Client(tx).CustomView.Create().SetWorkspaceID(input.Workspace).SetOwnerID(input.Owner).
		SetSlugID(input.SlugID).SetName(input.Name).SetModel(input.Model).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// Change is a change of some of a view's columns.
type Change func(*ent.CustomViewUpdateOne)

func SetName(name string) Change { return func(u *ent.CustomViewUpdateOne) { u.SetName(name) } }

func SetFilter(filter json.RawMessage) Change {
	return func(u *ent.CustomViewUpdateOne) { u.SetFilter(filter) }
}

func SetDisplay(display json.RawMessage) Change {
	return func(u *ent.CustomViewUpdateOne) { u.SetDisplay(display) }
}

func SetDescription(description *string) Change {
	return func(u *ent.CustomViewUpdateOne) {
		if description == nil {
			u.ClearDescription()
			return
		}
		u.SetDescription(*description)
	}
}

func SetIcon(icon *string) Change {
	return func(u *ent.CustomViewUpdateOne) {
		if icon == nil {
			u.ClearIcon()
			return
		}
		u.SetIcon(*icon)
	}
}

func SetColor(color *string) Change {
	return func(u *ent.CustomViewUpdateOne) {
		if color == nil {
			u.ClearColor()
			return
		}
		u.SetColor(*color)
	}
}

func SetTeam(team *uuid.UUID) Change {
	return func(u *ent.CustomViewUpdateOne) {
		if team == nil {
			u.ClearTeamID()
			return
		}
		u.SetTeamID(*team)
	}
}

func SetShared(shared bool) Change { return func(u *ent.CustomViewUpdateOne) { u.SetShared(shared) } }

func Touch(now time.Time) Change { return func(u *ent.CustomViewUpdateOne) { u.SetUpdatedAt(now) } }

func (self *ViewRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, changes ...Change) error {
	update := self.base.Client(tx).CustomView.UpdateOneID(id)
	for _, change := range changes {
		change(update)
	}
	return update.Exec(ctx)
}

// Delete deletes the view and the favorites pointing at it.
func (self *ViewRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	client := self.base.Client(tx)
	if _, err := client.Favorite.Delete().Where(favorite.Type("custom_view"), favorite.TargetID(id)).Exec(ctx); err != nil {
		return err
	}
	return client.CustomView.DeleteOneID(id).Exec(ctx)
}
