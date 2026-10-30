package assistant_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/assistantrun"
	"ricrac/ent/workspace"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AssistantRepositoryInterface -p assistant_repo -s AssistantRepository -o assistant_repository_iface.go
type AssistantRepository struct {
	base *repository.BaseRepository
}

func NewAssistantRepository(db *ent.Client) *AssistantRepository {
	return &AssistantRepository{base: repository.NewBaseRepository(db)}
}

// SpentToday is what every question of the last 24 hours cost, in dollars.
func (self *AssistantRepository) SpentToday(ctx context.Context) (float64, error) {
	var rows []struct {
		Sum *float64 `json:"sum"`
	}
	err := self.base.DB.AssistantRun.Query().
		Where(func(s *entsql.Selector) {
			s.Where(repository.Raw(s.C(assistantrun.FieldCreatedAt) + " > now() - interval '24 hours'"))
		}).
		Aggregate(ent.Sum(assistantrun.FieldCostUsd)).Scan(ctx, &rows)
	if err != nil || len(rows) == 0 || rows[0].Sum == nil {
		return 0, err
	}
	return *rows[0].Sum, nil
}

// SessionOwner is who started the conversation and where; found is false for one never started.
func (self *AssistantRepository) SessionOwner(ctx context.Context, session uuid.UUID) (user, workspace uuid.UUID, found bool, err error) {
	run, err := self.base.DB.AssistantRun.Query().Where(assistantrun.SessionID(session)).
		Order(assistantrun.ByCreatedAt()).First(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, false, err
	}
	return run.UserID, run.WorkspaceID, true, nil
}

type Run struct {
	Workspace uuid.UUID
	User      uuid.UUID
	Session   uuid.UUID
	CostUSD   float64
	Turns     int
	Outcome   string
}

func (self *AssistantRepository) Record(ctx context.Context, run Run) error {
	return self.base.DB.AssistantRun.Create().SetWorkspaceID(run.Workspace).SetUserID(run.User).SetSessionID(run.Session).
		SetCostUsd(run.CostUSD).SetTurns(run.Turns).SetOutcome(run.Outcome).Exec(ctx)
}

// Allowed says whether the user may ask in the workspace: it's on there and they're picked.
func (self *AssistantRepository) Allowed(ctx context.Context, workspaceID, user uuid.UUID) (bool, error) {
	return self.base.DB.WorkspaceMember.Query().Where(
		workspacemember.WorkspaceID(workspaceID), workspacemember.UserID(user), workspacemember.Active(true),
		workspacemember.Assistant(true), workspacemember.RoleNEQ("guest"),
		workspacemember.HasWorkspaceWith(workspace.AssistantEnabled(true)),
	).Exist(ctx)
}

// Settings is whether the assistant is on in the workspace and who's picked to use it.
func (self *AssistantRepository) Settings(ctx context.Context, tx repository.TxInterface, workspaceID uuid.UUID) (bool, []uuid.UUID, error) {
	client := self.base.Client(tx)
	found, err := client.Workspace.Get(ctx, workspaceID)
	if err != nil {
		return false, nil, err
	}
	var rows []struct {
		UserID uuid.UUID `json:"user_id"`
	}
	err = client.WorkspaceMember.Query().Where(workspacemember.WorkspaceID(workspaceID), workspacemember.Active(true),
		workspacemember.Assistant(true), workspacemember.RoleNEQ("guest")).
		Select(workspacemember.FieldUserID).Scan(ctx, &rows)
	users := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		users[i] = row.UserID
	}
	return found.AssistantEnabled, users, err
}

func (self *AssistantRepository) SetEnabled(ctx context.Context, tx repository.TxInterface, workspaceID uuid.UUID, enabled bool) error {
	return self.base.Client(tx).Workspace.UpdateOneID(workspaceID).SetAssistantEnabled(enabled).Exec(ctx)
}

// SetUsers picks who may use the assistant, of the workspace's active members who aren't
// guests, and answers how many it picked.
func (self *AssistantRepository) SetUsers(ctx context.Context, tx repository.TxInterface, workspaceID uuid.UUID, users []uuid.UUID) (int, error) {
	client := self.base.Client(tx)
	if err := client.WorkspaceMember.Update().Where(workspacemember.WorkspaceID(workspaceID)).SetAssistant(false).Exec(ctx); err != nil {
		return 0, err
	}
	return client.WorkspaceMember.Update().Where(workspacemember.WorkspaceID(workspaceID), workspacemember.UserIDIn(users...),
		workspacemember.Active(true), workspacemember.RoleNEQ("guest")).SetAssistant(true).Save(ctx)
}
