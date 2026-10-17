package state_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issue"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i StateRepositoryInterface -p state_repo -s StateRepository -o state_repository_iface.go
type StateRepository struct {
	base *repository.BaseRepository
}

func NewStateRepository(db *ent.Client) *StateRepository {
	return &StateRepository{base: repository.NewBaseRepository(db)}
}

// BoardOrder orders states as the board shows them: by type, then position.
func BoardOrder(s *entsql.Selector) {
	s.OrderExpr(entsql.Expr("array_position(ARRAY['triage','backlog','unstarted','started','completed','canceled'], " + s.C(workflowstate.FieldType) + ")"))
	s.OrderBy(s.C(workflowstate.FieldPosition))
}

// Active is the team's unarchived states in board order.
func (self *StateRepository) Active(ctx context.Context, tx repository.TxInterface, team uuid.UUID) ([]*ent.WorkflowState, error) {
	return self.base.Client(tx).WorkflowState.Query().
		Where(workflowstate.TeamID(team), workflowstate.ArchivedAtIsNil()).Order(BoardOrder).All(ctx)
}

type ListFilter struct {
	Team            uuid.UUID
	IncludeArchived bool
	Limit           int64
	Offset          int64
}

// List is the team's states in board order, archived ones last.
func (self *StateRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.WorkflowState.Query().Where(workflowstate.TeamID(filter.Team))
	if !filter.IncludeArchived {
		query = query.Where(workflowstate.ArchivedAtIsNil())
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(workflowstate.FieldID))
		s.OrderExpr(entsql.Expr(s.C(workflowstate.FieldArchivedAt) + " IS NOT NULL"))
		BoardOrder(s)
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// Load is the states with their team, in the order of ids.
func (self *StateRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.WorkflowState, error) {
	states, err := self.base.Client(tx).WorkflowState.Query().Where(workflowstate.IDIn(ids...)).WithTeam().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.WorkflowState, len(states))
	for _, s := range states {
		byID[s.ID] = s
	}
	ordered := make([]*ent.WorkflowState, 0, len(ids))
	for _, id := range ids {
		if s, ok := byID[id]; ok {
			ordered = append(ordered, s)
		}
	}
	return ordered, nil
}

// IssueCounts is how many issues not in the trash are in each state.
func (self *StateRepository) IssueCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		StateID uuid.UUID `json:"state_id"`
		Count   int       `json:"count"`
	}
	err := self.base.Client(tx).Issue.Query().
		Where(issue.StateIDIn(ids...), issue.TrashedAtIsNil()).
		GroupBy(issue.FieldStateID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.StateID] = row.Count
	}
	return counts, nil
}

func (self *StateRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.WorkflowState, error) {
	return self.base.Client(tx).WorkflowState.Get(ctx, id)
}

// GetForUpdate is the state with its team, both locked until the transaction ends.
func (self *StateRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.WorkflowState, error) {
	return self.base.Client(tx).WorkflowState.Query().Where(workflowstate.ID(id)).
		WithTeam(func(q *ent.TeamQuery) { q.ForUpdate() }).ForUpdate().Only(ctx)
}

// TeamOf is the team of a state of the workspace.
func (self *StateRepository) TeamOf(ctx context.Context, workspace, id uuid.UUID) (uuid.UUID, bool, error) {
	state, err := self.base.DB.WorkflowState.Query().Where(workflowstate.ID(id)).WithTeam().Only(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	if state.Edges.Team.WorkspaceID != workspace {
		return uuid.Nil, false, nil
	}
	return state.TeamID, true, nil
}

// NameTaken says whether another unarchived state of the team has the name, in any case.
func (self *StateRepository) NameTaken(ctx context.Context, tx repository.TxInterface, team uuid.UUID, name string, except *uuid.UUID) (bool, error) {
	query := self.base.Client(tx).WorkflowState.Query().
		Where(workflowstate.TeamID(team), workflowstate.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(workflowstate.FieldName)+") = lower(?)", name))
		})
	if except != nil {
		query = query.Where(workflowstate.IDNEQ(*except))
	}
	return query.Exist(ctx)
}

// HasTriage says whether the team has an unarchived triage state other than except.
func (self *StateRepository) HasTriage(ctx context.Context, tx repository.TxInterface, team uuid.UUID, except *uuid.UUID) (bool, error) {
	query := self.base.Client(tx).WorkflowState.Query().
		Where(workflowstate.TeamID(team), workflowstate.Type("triage"), workflowstate.ArchivedAtIsNil())
	if except != nil {
		query = query.Where(workflowstate.IDNEQ(*except))
	}
	return query.Exist(ctx)
}

// NextPosition is one past the last position of the team's unarchived states of the type, or of
// all of them when there's none.
func (self *StateRepository) NextPosition(ctx context.Context, tx repository.TxInterface, team uuid.UUID, kind string) (float64, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT coalesce(max(position) FILTER (WHERE type = $2), max(position), 0) + 1
		 FROM workflow_states WHERE team_id = $1 AND archived_at IS NULL`, team, kind)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var position float64
	if rows.Next() {
		err = rows.Scan(&position)
	}
	if err != nil {
		return 0, err
	}
	return position, rows.Err()
}

type CreateInput struct {
	Team        uuid.UUID
	Name        string
	Type        string
	Color       string
	Description *string
	Position    float64
}

func (self *StateRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).WorkflowState.Create().
		SetTeamID(input.Team).SetName(input.Name).SetType(input.Type).SetColor(input.Color).
		SetNillableDescription(input.Description).SetPosition(input.Position).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *StateRepository) SetName(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name string) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(id).SetName(name).Exec(ctx)
}

func (self *StateRepository) SetColor(ctx context.Context, tx repository.TxInterface, id uuid.UUID, color string) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(id).SetColor(color).Exec(ctx)
}

func (self *StateRepository) SetDescription(ctx context.Context, tx repository.TxInterface, id uuid.UUID, description *string) error {
	update := self.base.Client(tx).WorkflowState.UpdateOneID(id)
	if description == nil {
		return update.ClearDescription().Exec(ctx)
	}
	return update.SetDescription(*description).Exec(ctx)
}

func (self *StateRepository) SetPosition(ctx context.Context, tx repository.TxInterface, id uuid.UUID, position float64) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(id).SetPosition(position).Exec(ctx)
}

func (self *StateRepository) SetType(ctx context.Context, tx repository.TxInterface, id uuid.UUID, kind string) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(id).SetType(kind).Exec(ctx)
}

func (self *StateRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(id).SetUpdatedAt(now).Exec(ctx)
}

func (self *StateRepository) Archive(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).WorkflowState.UpdateOneID(id).SetArchivedAt(now).SetUpdatedAt(now).Exec(ctx)
}

// AllIssueCount is how many issues are in the state, trashed ones too.
func (self *StateRepository) AllIssueCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).Issue.Query().Where(issue.StateID(id)).Count(ctx)
}

// Issues is the issues in the state, by number.
func (self *StateRepository) Issues(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]uuid.UUID, error) {
	return self.base.Client(tx).Issue.Query().Where(issue.StateID(id)).Order(ent.Asc(issue.FieldNumber)).IDs(ctx)
}
