package account_repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"ricrac/ent/schema/pgarray"
	"ricrac/ent/team"
	"ricrac/ent/teammember"
	"ricrac/ent/workspace"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

func (self *AccountRepository) URLKeyTaken(ctx context.Context, tx repository.TxInterface, urlKey string) (bool, error) {
	return self.base.Client(tx).Workspace.Query().Where(workspace.URLKey(urlKey)).Exist(ctx)
}

// CreateWorkspace creates the workspace with owner as its owner.
func (self *AccountRepository) CreateWorkspace(ctx context.Context, tx repository.TxInterface, name, urlKey string, owner uuid.UUID) (uuid.UUID, error) {
	client := self.base.Client(tx)
	created, err := client.Workspace.Create().SetName(name).SetURLKey(urlKey).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, client.WorkspaceMember.Create().SetWorkspaceID(created.ID).SetUserID(owner).SetRole("owner").Exec(ctx)
}

// DeleteWorkspace deletes the workspace when its URL key is urlKey, saying how many it deleted.
func (self *AccountRepository) DeleteWorkspace(ctx context.Context, tx repository.TxInterface, id uuid.UUID, urlKey string) (int, error) {
	return self.base.Client(tx).Workspace.Delete().Where(workspace.ID(id), workspace.URLKey(urlKey)).Exec(ctx)
}

// LockOwners is the workspace's active owners, locked until tx ends.
func (self *AccountRepository) LockOwners(ctx context.Context, tx repository.TxInterface, workspaceID uuid.UUID) ([]uuid.UUID, error) {
	members, err := self.base.Client(tx).WorkspaceMember.Query().
		Where(workspacemember.WorkspaceID(workspaceID), workspacemember.Role("owner"), workspacemember.Active(true)).
		ForUpdate().
		All(ctx)
	if err != nil {
		return nil, err
	}
	owners := make([]uuid.UUID, len(members))
	for i, m := range members {
		owners[i] = m.UserID
	}
	return owners, nil
}

// Leave makes the user an inactive member of the workspace and takes them out of its teams.
func (self *AccountRepository) Leave(ctx context.Context, tx repository.TxInterface, workspaceID, user uuid.UUID) error {
	client := self.base.Client(tx)
	err := client.WorkspaceMember.Update().
		Where(workspacemember.WorkspaceID(workspaceID), workspacemember.UserID(user)).
		SetActive(false).
		Exec(ctx)
	if err != nil {
		return err
	}
	_, err = client.TeamMember.Delete().
		Where(teammember.UserID(user), teammember.HasTeamWith(team.WorkspaceID(workspaceID))).
		Exec(ctx)
	return err
}

// DropWorkspaceAccess takes the workspace out of the keys and OAuth grants and codes that list
// it (of one user, or everyone's), deleting those left without a workspace.
func (self *AccountRepository) DropWorkspaceAccess(ctx context.Context, tx repository.TxInterface, workspaceID uuid.UUID, user *uuid.UUID) error {
	client := self.base.Client(tx)
	for _, table := range []string{"api_keys", "oauth_grants", "oauth_codes"} {
		_, err := client.ExecContext(ctx, fmt.Sprintf(
			"DELETE FROM %s WHERE workspace_ids = ARRAY[$1]::uuid[] AND ($2::uuid IS NULL OR user_id = $2)", table), workspaceID, user)
		if err != nil {
			return err
		}
		_, err = client.ExecContext(ctx, fmt.Sprintf(
			`UPDATE %s SET workspace_ids = array_remove(workspace_ids, $1)
			 WHERE $1 = ANY(workspace_ids) AND ($2::uuid IS NULL OR user_id = $2)`, table), workspaceID, user)
		if err != nil {
			return err
		}
	}
	return nil
}

type TakenInvitation struct {
	Workspace uuid.UUID
	Role      string
	Teams     []uuid.UUID
}

// TakeInvitation deletes the user's unexpired invitation and returns it, nil when there's none.
func (self *AccountRepository) TakeInvitation(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID) (*TakenInvitation, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`DELETE FROM invitations i USING users u WHERE i.id = $1 AND u.id = $2 AND i.email = u.email AND i.expires_at > now()
		 RETURNING i.workspace_id, i.role, i.team_ids`, id, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var taken TakenInvitation
	var teams pgarray.UUID
	if err := rows.Scan(&taken.Workspace, &taken.Role, &teams); err != nil {
		return nil, err
	}
	taken.Teams = teams
	return &taken, rows.Err()
}

// Join makes the user an active member of the workspace with role, and of the teams.
func (self *AccountRepository) Join(ctx context.Context, tx repository.TxInterface, workspaceID, user uuid.UUID, role string, teams []uuid.UUID) error {
	client := self.base.Client(tx)
	_, err := client.ExecContext(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (workspace_id, user_id) DO UPDATE SET active = true, role = excluded.role`,
		workspaceID, user, role)
	if err != nil {
		return err
	}
	_, err = client.ExecContext(ctx,
		"INSERT INTO team_members (team_id, user_id) SELECT unnest($1::uuid[]), $2 ON CONFLICT DO NOTHING", teams, user)
	return err
}

// DeclineInvitation deletes the invitation when it is the user's.
func (self *AccountRepository) DeclineInvitation(ctx context.Context, id, user uuid.UUID) error {
	_, err := self.base.DB.ExecContext(ctx,
		"DELETE FROM invitations i USING users u WHERE i.id = $1 AND u.id = $2 AND i.email = u.email", id, user)
	return err
}

// ActiveIn is which of the workspaces the user is an active member of.
func (self *AccountRepository) ActiveIn(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
	}
	err := self.base.DB.WorkspaceMember.Query().
		Where(workspacemember.UserID(user), workspacemember.Active(true), workspacemember.WorkspaceIDIn(workspaces...)).
		Select(workspacemember.FieldWorkspaceID).
		Scan(ctx, &rows)
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.WorkspaceID
	}
	return ids, err
}

// AdminIn says whether the user is an owner or admin of one of the workspaces (any when nil).
func (self *AccountRepository) AdminIn(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID) (bool, error) {
	query := self.base.DB.WorkspaceMember.Query().
		Where(workspacemember.UserID(user), workspacemember.Active(true), workspacemember.RoleIn("owner", "admin"))
	if workspaces != nil {
		query.Where(workspacemember.WorkspaceIDIn(workspaces...))
	}
	return query.Exist(ctx)
}
