package statusupdate_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/comment"
	"ricrac/ent/initiative"
	"ricrac/ent/project"
	"ricrac/ent/reaction"
	"ricrac/ent/statusupdate"
	repository "ricrac/internal/repositories"
	project_repo "ricrac/internal/repositories/project"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i StatusUpdateRepositoryInterface -p statusupdate_repo -s StatusUpdateRepository -o statusupdate_repository_iface.go
type StatusUpdateRepository struct {
	base *repository.BaseRepository
}

func NewStatusUpdateRepository(db *ent.Client) *StatusUpdateRepository {
	return &StatusUpdateRepository{base: repository.NewBaseRepository(db)}
}

// Load is the updates with what their answer shows, in the order of ids.
func (self *StatusUpdateRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.StatusUpdate, error) {
	updates, err := self.base.Client(tx).StatusUpdate.Query().Where(statusupdate.IDIn(ids...)).
		WithProject().WithInitiative().WithUser().
		WithReactions(func(q *ent.ReactionQuery) { q.WithUser().Order(ent.Asc(reaction.FieldCreatedAt)) }).
		All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.StatusUpdate, len(updates))
	for _, u := range updates {
		byID[u.ID] = u
	}
	ordered := make([]*ent.StatusUpdate, 0, len(ids))
	for _, id := range ids {
		if u, ok := byID[id]; ok {
			ordered = append(ordered, u)
		}
	}
	return ordered, nil
}

// CommentCounts is how many comments each update has.
func (self *StatusUpdateRepository) CommentCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		StatusUpdateID uuid.UUID `json:"status_update_id"`
		Count          int       `json:"count"`
	}
	err := self.base.Client(tx).Comment.Query().Where(comment.StatusUpdateIDIn(ids...)).
		GroupBy(comment.FieldStatusUpdateID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.StatusUpdateID] = row.Count
	}
	return counts, nil
}

func newestFirst(s *entsql.Selector) {
	s.OrderBy(entsql.Desc(s.C(statusupdate.FieldCreatedAt)), s.C(statusupdate.FieldID))
}

// Latest is the newest update on the project or initiative.
func (self *StatusUpdateRepository) Latest(ctx context.Context, tx repository.TxInterface, project, initiative *uuid.UUID) (*ent.StatusUpdate, error) {
	query := self.base.Client(tx).StatusUpdate.Query()
	if project != nil {
		query.Where(statusupdate.ProjectID(*project))
	} else {
		query.Where(statusupdate.ProjectIDIsNil())
	}
	if initiative != nil {
		query.Where(statusupdate.InitiativeID(*initiative))
	} else {
		query.Where(statusupdate.InitiativeIDIsNil())
	}
	found, err := query.Modify(newestFirst).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

type ListFilter struct {
	Workspace uuid.UUID
	// The caller's teams: updates on projects in none of them are left out.
	Teams []uuid.UUID
	// Guests don't see updates on initiatives.
	SeesInitiatives bool
	ID              *uuid.UUID
	Project         *uuid.UUID
	Initiative      *uuid.UUID
	// project or initiative.
	Type      *string
	User      *uuid.UUID
	CreatedAt *time.Time
	Limit     int64
	Offset    int64
}

// List is the updates the filter keeps, newest first.
func (self *StatusUpdateRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	onProject := statusupdate.And(statusupdate.ProjectIDNotNil(),
		statusupdate.HasProjectWith(project.TrashedAtIsNil(), project_repo.Visible(filter.Teams)))
	onInitiative := statusupdate.And(statusupdate.InitiativeIDNotNil(), func(s *entsql.Selector) {
		if !filter.SeesInitiatives {
			s.Where(entsql.False())
		}
	}, statusupdate.HasInitiativeWith(initiative.TrashedAtIsNil()))
	query := self.base.DB.StatusUpdate.Query().Where(statusupdate.WorkspaceID(filter.Workspace), statusupdate.Or(onProject, onInitiative))
	if filter.ID != nil {
		query.Where(statusupdate.ID(*filter.ID))
	}
	if filter.Project != nil {
		query.Where(statusupdate.ProjectID(*filter.Project))
	}
	if filter.Initiative != nil {
		query.Where(statusupdate.InitiativeID(*filter.Initiative))
	}
	if filter.Type != nil && *filter.Type == "project" {
		query.Where(statusupdate.ProjectIDNotNil())
	}
	if filter.Type != nil && *filter.Type == "initiative" {
		query.Where(statusupdate.InitiativeIDNotNil())
	}
	if filter.User != nil {
		query.Where(statusupdate.UserID(*filter.User))
	}
	if filter.CreatedAt != nil {
		query.Where(statusupdate.CreatedAtGTE(*filter.CreatedAt))
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(statusupdate.FieldID))
		newestFirst(s)
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// Get is an update of the workspace.
func (self *StatusUpdateRepository) Get(ctx context.Context, workspace, id uuid.UUID) (*ent.StatusUpdate, error) {
	found, err := self.base.DB.StatusUpdate.Query().Where(statusupdate.ID(id), statusupdate.WorkspaceID(workspace)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// GetForUpdate locks the update until the transaction ends.
func (self *StatusUpdateRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.StatusUpdate, error) {
	return self.base.Client(tx).StatusUpdate.Query().Where(statusupdate.ID(id)).ForUpdate().Only(ctx)
}

type CreateInput struct {
	Workspace  uuid.UUID
	Project    *uuid.UUID
	Initiative *uuid.UUID
	User       uuid.UUID
	Body       string
	Health     string
}

func (self *StatusUpdateRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).StatusUpdate.Create().SetWorkspaceID(input.Workspace).
		SetNillableProjectID(input.Project).SetNillableInitiativeID(input.Initiative).SetUserID(input.User).
		SetBody(input.Body).SetHealth(input.Health).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// Update is the update's update builder, to set fields on.
func (self *StatusUpdateRepository) Update(tx repository.TxInterface, id uuid.UUID) *ent.StatusUpdateUpdateOne {
	return self.base.Client(tx).StatusUpdate.UpdateOneID(id)
}

func (self *StatusUpdateRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).StatusUpdate.DeleteOneID(id).Exec(ctx)
}

// SyncHealth copies the newest update's health onto the project or initiative (null when none
// is left).
func (self *StatusUpdateRepository) SyncHealth(ctx context.Context, tx repository.TxInterface, project, initiative *uuid.UUID) error {
	if project == nil && initiative == nil {
		return nil
	}
	latest, err := self.Latest(ctx, tx, project, initiative)
	if err != nil {
		return err
	}
	var health *string
	if latest != nil {
		health = &latest.Health
	}
	if project != nil {
		update := self.base.Client(tx).Project.UpdateOneID(*project)
		if health == nil {
			return update.ClearHealth().Exec(ctx)
		}
		return update.SetHealth(*health).Exec(ctx)
	}
	update := self.base.Client(tx).Initiative.UpdateOneID(*initiative)
	if health == nil {
		return update.ClearHealth().Exec(ctx)
	}
	return update.SetHealth(*health).Exec(ctx)
}

// ProjectAudience is who hears about a project's updates: its members, subscribers and lead.
func (self *StatusUpdateRepository) ProjectAudience(ctx context.Context, tx repository.TxInterface, project uuid.UUID) ([]uuid.UUID, error) {
	return self.ids(ctx, tx, `SELECT user_id FROM project_members WHERE project_id = $1
		UNION SELECT user_id FROM project_subscribers WHERE project_id = $1
		UNION SELECT lead_id FROM projects WHERE id = $1 AND lead_id IS NOT NULL`, project)
}

// InitiativeOwner is who hears about an initiative's updates.
func (self *StatusUpdateRepository) InitiativeOwner(ctx context.Context, tx repository.TxInterface, initiative uuid.UUID) ([]uuid.UUID, error) {
	return self.ids(ctx, tx, "SELECT owner_id FROM initiatives WHERE id = $1 AND owner_id IS NOT NULL", initiative)
}

func (self *StatusUpdateRepository) ids(ctx context.Context, tx repository.TxInterface, query string, args ...any) ([]uuid.UUID, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
