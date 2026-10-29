package workspace_repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/invitation"
	"ricrac/ent/schema/pgarray"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

func (self *WorkspaceRepository) SetName(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name string) error {
	return self.base.Client(tx).Workspace.UpdateOneID(id).SetName(name).Exec(ctx)
}

func (self *WorkspaceRepository) SetURLKey(ctx context.Context, tx repository.TxInterface, id uuid.UUID, key string) error {
	return self.base.Client(tx).Workspace.UpdateOneID(id).SetURLKey(key).Exec(ctx)
}

func (self *WorkspaceRepository) SetLogoURL(ctx context.Context, tx repository.TxInterface, id uuid.UUID, logo *string) error {
	update := self.base.Client(tx).Workspace.UpdateOneID(id)
	if logo == nil {
		return update.ClearLogoURL().Exec(ctx)
	}
	return update.SetLogoURL(*logo).Exec(ctx)
}

func (self *WorkspaceRepository) SetTheme(ctx context.Context, tx repository.TxInterface, id uuid.UUID, theme string) error {
	return self.base.Client(tx).Workspace.UpdateOneID(id).SetTheme(theme).Exec(ctx)
}

func (self *WorkspaceRepository) SetPublic(ctx context.Context, tx repository.TxInterface, id uuid.UUID, public bool) error {
	return self.base.Client(tx).Workspace.UpdateOneID(id).SetPublic(public).Exec(ctx)
}

func (self *WorkspaceRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Workspace.UpdateOneID(id).SetUpdatedAt(now).Exec(ctx)
}

func (self *WorkspaceRepository) SetRole(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID, role string) error {
	_, err := self.base.Client(tx).WorkspaceMember.Update().
		Where(workspacemember.WorkspaceID(id), workspacemember.UserID(user)).SetRole(role).Save(ctx)
	return err
}

// JoinTeam adds the user to the workspace's team.
func (self *WorkspaceRepository) JoinTeam(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id) SELECT t.id, $2 FROM teams t WHERE t.workspace_id = $1 AND t.archived_at IS NULL
		 ON CONFLICT DO NOTHING`, id, user)
	return err
}

func (self *WorkspaceRepository) SetUserName(ctx context.Context, tx repository.TxInterface, user uuid.UUID, name string) error {
	return self.base.Client(tx).User.UpdateOneID(user).SetName(name).Exec(ctx)
}

func (self *WorkspaceRepository) SetDisplayName(ctx context.Context, tx repository.TxInterface, user uuid.UUID, displayName string) error {
	return self.base.Client(tx).User.UpdateOneID(user).SetDisplayName(displayName).Exec(ctx)
}

func (self *WorkspaceRepository) SetTimezone(ctx context.Context, tx repository.TxInterface, user uuid.UUID, timezone *string) error {
	update := self.base.Client(tx).User.UpdateOneID(user)
	if timezone == nil {
		return update.ClearTimezone().Exec(ctx)
	}
	return update.SetTimezone(*timezone).Exec(ctx)
}

func (self *WorkspaceRepository) SetEmailNotifications(ctx context.Context, tx repository.TxInterface, user uuid.UUID, on bool) error {
	return self.base.Client(tx).User.UpdateOneID(user).SetEmailNotifications(on).Exec(ctx)
}

// Deactivate takes the member out of the workspace: inactive, off its teams and their triage.
func (self *WorkspaceRepository) Deactivate(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID) error {
	client := self.base.Client(tx)
	if _, err := client.WorkspaceMember.Update().
		Where(workspacemember.WorkspaceID(id), workspacemember.UserID(user)).SetActive(false).Save(ctx); err != nil {
		return err
	}
	if _, err := client.ExecContext(ctx,
		"DELETE FROM team_members tm USING teams t WHERE t.id = tm.team_id AND t.workspace_id = $1 AND tm.user_id = $2",
		id, user); err != nil {
		return err
	}
	_, err := client.ExecContext(ctx,
		`UPDATE teams SET triage_responsible_ids = array_remove(triage_responsible_ids, $2)
		 WHERE workspace_id = $1 AND $2 = ANY(triage_responsible_ids)`, id, user)
	return err
}

type InvitationInput struct {
	Workspace uuid.UUID
	Email     string
	Role      string
	Teams     []uuid.UUID
	Inviter   uuid.UUID
	Now       time.Time
}

// UpsertInvitation invites the email, or renews its invitation, for 14 days from now.
func (self *WorkspaceRepository) UpsertInvitation(ctx context.Context, tx repository.TxInterface, input InvitationInput) (uuid.UUID, error) {
	return self.base.Client(tx).Invitation.Create().
		SetWorkspaceID(input.Workspace).SetEmail(input.Email).SetRole(input.Role).SetTeamIds(pgarray.UUID(input.Teams)).
		SetInviterID(input.Inviter).SetCreatedAt(input.Now).SetExpiresAt(input.Now.AddDate(0, 0, 14)).
		OnConflictColumns(invitation.FieldWorkspaceID, invitation.FieldEmail).
		Update(func(u *ent.InvitationUpsert) {
			u.UpdateRole().UpdateTeamIds().UpdateInviterID().UpdateCreatedAt().UpdateExpiresAt()
		}).ID(ctx)
}
