package template_repo

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/template"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i TemplateRepositoryInterface -p template_repo -s TemplateRepository -o template_repository_iface.go
type TemplateRepository struct {
	base *repository.BaseRepository
}

func NewTemplateRepository(db *ent.Client) *TemplateRepository {
	return &TemplateRepository{base: repository.NewBaseRepository(db)}
}

// Load is the templates with their team and creator, in the order of ids.
func (self *TemplateRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Template, error) {
	templates, err := self.base.Client(tx).Template.Query().Where(template.IDIn(ids...)).WithTeam().WithCreator().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Template, len(templates))
	for _, t := range templates {
		byID[t.ID] = t
	}
	ordered := make([]*ent.Template, 0, len(ids))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			ordered = append(ordered, t)
		}
	}
	return ordered, nil
}

type ListFilter struct {
	Workspace uuid.UUID
	// The teams the caller sees; workspace templates are listed too.
	Teams []uuid.UUID
	Type  *string
	// Only this team's templates and the workspace ones.
	Team   *uuid.UUID
	Limit  int64
	Offset int64
}

// List is the templates by type, then name.
func (self *TemplateRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Template.Query().
		Where(template.WorkspaceID(filter.Workspace), template.Or(template.TeamIDIsNil(), template.TeamIDIn(filter.Teams...)))
	if filter.Type != nil {
		query = query.Where(template.Type(*filter.Type))
	}
	if filter.Team != nil {
		query = query.Where(template.Or(template.TeamIDIsNil(), template.TeamID(*filter.Team)))
	}
	return query.Order(ent.Asc(template.FieldType), func(s *entsql.Selector) {
		s.OrderExpr(entsql.Expr("lower(" + s.C(template.FieldName) + ")"))
	}).Limit(int(filter.Limit)).Offset(int(filter.Offset)).IDs(ctx)
}

// Named is the templates of the workspace and of teams with this id (first) or name, in any case.
func (self *TemplateRepository) Named(ctx context.Context, workspace uuid.UUID, teams []uuid.UUID, asID *uuid.UUID, name string) ([]*ent.Template, error) {
	matches := template.Or(func(s *entsql.Selector) {
		s.Where(repository.Raw("lower("+s.C(template.FieldName)+") = lower(?)", name))
	})
	if asID != nil {
		matches = template.Or(template.ID(*asID), matches)
	}
	query := self.base.DB.Template.Query().
		Where(template.WorkspaceID(workspace), template.Or(template.TeamIDIsNil(), template.TeamIDIn(teams...)), matches)
	if asID != nil {
		query = query.Order(func(s *entsql.Selector) {
			s.OrderExpr(repository.RawExpr(s.C(template.FieldID)+" = ? DESC", *asID))
		})
	}
	return query.All(ctx)
}

// Data is what the template fills in.
func (self *TemplateRepository) Data(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (json.RawMessage, error) {
	found, err := self.base.Client(tx).Template.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return found.Data, nil
}

// GetForUpdate locks the template until the transaction ends.
func (self *TemplateRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Template, error) {
	return self.base.Client(tx).Template.Query().Where(template.ID(id)).ForUpdate().Only(ctx)
}

func (self *TemplateRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Template, error) {
	return self.base.Client(tx).Template.Get(ctx, id)
}

func (self *TemplateRepository) Create(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, kind, name string, creator uuid.UUID) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Template.Create().SetWorkspaceID(workspace).SetType(kind).SetName(name).
		SetCreatorID(creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *TemplateRepository) SetName(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name string) error {
	return self.base.Client(tx).Template.UpdateOneID(id).SetName(name).Exec(ctx)
}

func (self *TemplateRepository) SetDescription(ctx context.Context, tx repository.TxInterface, id uuid.UUID, description *string) error {
	update := self.base.Client(tx).Template.UpdateOneID(id)
	if description == nil {
		return update.ClearDescription().Exec(ctx)
	}
	return update.SetDescription(*description).Exec(ctx)
}

func (self *TemplateRepository) SetTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID, team *uuid.UUID) error {
	update := self.base.Client(tx).Template.UpdateOneID(id)
	if team == nil {
		return update.ClearTeamID().Exec(ctx)
	}
	return update.SetTeamID(*team).Exec(ctx)
}

// SetRecurrence sets when the template creates issues (nil: never) and the next time.
func (self *TemplateRepository) SetRecurrence(ctx context.Context, tx repository.TxInterface, id uuid.UUID, recurrence json.RawMessage, next *time.Time) error {
	update := self.base.Client(tx).Template.UpdateOneID(id)
	if recurrence == nil {
		update.ClearRecurrence()
	} else {
		update.SetRecurrence(recurrence)
	}
	if next == nil {
		update.ClearNextRunAt()
	} else {
		update.SetNextRunAt(*next)
	}
	return update.Exec(ctx)
}

func (self *TemplateRepository) SetData(ctx context.Context, tx repository.TxInterface, id uuid.UUID, data json.RawMessage) error {
	return self.base.Client(tx).Template.UpdateOneID(id).SetData(data).Exec(ctx)
}

func (self *TemplateRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Template.UpdateOneID(id).SetUpdatedAt(now).Exec(ctx)
}

func (self *TemplateRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).Template.DeleteOneID(id).Exec(ctx)
}

// Due is up to 20 issue templates whose next run has come, the earliest first, locked until the
// transaction ends; templates another run has locked are skipped.
func (self *TemplateRepository) Due(ctx context.Context, tx repository.TxInterface) ([]*ent.Template, error) {
	return self.base.Client(tx).Template.Query().Where(template.Type("issue"), func(s *entsql.Selector) {
		s.Where(entsql.ExprP(s.C(template.FieldNextRunAt) + " <= now()"))
	}).Order(ent.Asc(template.FieldNextRunAt)).Limit(20).ForUpdate(entsql.WithLockAction(entsql.SkipLocked)).All(ctx)
}

func (self *TemplateRepository) SetNextRun(ctx context.Context, tx repository.TxInterface, id uuid.UUID, next *time.Time) error {
	update := self.base.Client(tx).Template.UpdateOneID(id)
	if next == nil {
		return update.ClearNextRunAt().Exec(ctx)
	}
	return update.SetNextRunAt(*next).Exec(ctx)
}
