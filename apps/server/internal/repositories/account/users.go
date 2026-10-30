package account_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/invitation"
	"ricrac/ent/user"
	"ricrac/ent/workspace"
	"ricrac/ent/workspacemember"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

func (self *AccountRepository) UserExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return self.base.DB.User.Query().Where(user.ID(id)).Exist(ctx)
}

// User is the user, nil when there's none.
func (self *AccountRepository) User(ctx context.Context, id uuid.UUID) (*ent.User, error) {
	found, err := self.base.DB.User.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (self *AccountRepository) SetName(ctx context.Context, id uuid.UUID, name string) error {
	return self.base.DB.User.Update().Where(user.ID(id)).SetName(name).Exec(ctx)
}

func (self *AccountRepository) SetDisplayName(ctx context.Context, id uuid.UUID, handle string) error {
	return self.base.DB.User.Update().Where(user.ID(id)).SetDisplayName(handle).Exec(ctx)
}

// Memberships is the workspaces the user is an active member of, by name.
func (self *AccountRepository) Memberships(ctx context.Context, id uuid.UUID) ([]models.WorkspaceSummary, error) {
	return self.summaries(ctx, nil, id, nil)
}

// Membership is the user's place in the workspace, active or not; nil when there's none.
func (self *AccountRepository) Membership(ctx context.Context, tx repository.TxInterface, workspaceID, id uuid.UUID) (*models.WorkspaceSummary, error) {
	found, err := self.summaries(ctx, tx, id, &workspaceID)
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return &found[0], nil
}

func (self *AccountRepository) summaries(ctx context.Context, tx repository.TxInterface, id uuid.UUID, only *uuid.UUID) ([]models.WorkspaceSummary, error) {
	var rows []struct {
		ID       uuid.UUID `json:"id"`
		Name     string    `json:"name"`
		URLKey   string    `json:"url_key"`
		Role     string    `json:"role"`
		LogoURL  *string   `json:"logo_url"`
		Theme    string    `json:"theme"`
		OwnTheme *string   `json:"own_theme"`
	}
	query := self.base.Client(tx).Workspace.Query().Modify(func(s *entsql.Selector) {
		members := entsql.Table(workspacemember.Table)
		s.Join(members).On(s.C(workspace.FieldID), members.C(workspacemember.FieldWorkspaceID))
		s.Where(entsql.EQ(members.C(workspacemember.FieldUserID), id))
		if only != nil {
			s.Where(entsql.EQ(s.C(workspace.FieldID), *only))
		} else {
			s.Where(entsql.EQ(members.C(workspacemember.FieldActive), true))
			s.OrderBy(s.C(workspace.FieldName))
		}
		s.Select(s.C(workspace.FieldID), s.C(workspace.FieldName), s.C(workspace.FieldURLKey), members.C(workspacemember.FieldRole),
			s.C(workspace.FieldLogoURL), s.C(workspace.FieldTheme), entsql.As(members.C(workspacemember.FieldTheme), "own_theme"))
	})
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	summaries := make([]models.WorkspaceSummary, len(rows))
	for i, row := range rows {
		summaries[i] = models.WorkspaceSummary{ID: row.ID, Name: row.Name, URLKey: row.URLKey, Role: row.Role, LogoURL: row.LogoURL, Theme: row.Theme, OwnTheme: row.OwnTheme}
	}
	return summaries, nil
}

// PendingInvitations is the unexpired invitations to email, except to workspaces the user is
// already an active member of.
func (self *AccountRepository) PendingInvitations(ctx context.Context, email string, id uuid.UUID) ([]models.PendingInvitation, error) {
	invitations, err := self.base.DB.Invitation.Query().
		Where(invitation.Email(email), future(invitation.FieldExpiresAt), func(s *entsql.Selector) {
			members := entsql.Table(workspacemember.Table)
			s.Where(entsql.NotExists(entsql.Select(members.C(workspacemember.FieldUserID)).From(members).Where(entsql.And(
				entsql.ColumnsEQ(members.C(workspacemember.FieldWorkspaceID), s.C(invitation.FieldWorkspaceID)),
				entsql.EQ(members.C(workspacemember.FieldUserID), id),
				entsql.EQ(members.C(workspacemember.FieldActive), true)))))
		}).
		WithWorkspace().
		WithInviter().
		All(ctx)
	if err != nil {
		return nil, err
	}
	pending := make([]models.PendingInvitation, len(invitations))
	for i, inv := range invitations {
		pending[i] = models.PendingInvitation{ID: inv.ID, WorkspaceName: inv.Edges.Workspace.Name, Role: inv.Role}
		if inv.Edges.Inviter != nil {
			pending[i].Inviter = &inv.Edges.Inviter.Name
		}
	}
	return pending, nil
}

// MemberWorkspaces is the workspaces the user is an active member of, only the one named (id or
// URL key) when named isn't nil.
func (self *AccountRepository) MemberWorkspaces(ctx context.Context, id uuid.UUID, named *string) ([]uuid.UUID, error) {
	return self.base.DB.Workspace.Query().
		Where(workspace.HasWorkspaceMembersWith(workspacemember.UserID(id), workspacemember.Active(true)), func(s *entsql.Selector) {
			if named == nil {
				return
			}
			s.Where(repository.Raw("("+s.C(workspace.FieldID)+"::text = ? OR "+s.C(workspace.FieldURLKey)+" = lower(?))", *named, *named))
		}).
		IDs(ctx)
}

// UpsertGoogleUser refreshes the user with this Google subject, or else this email, or creates
// them, and returns their id.
func (self *AccountRepository) UpsertGoogleUser(ctx context.Context, subject, email, name, displayName string, picture *string) (uuid.UUID, error) {
	existing, err := self.base.DB.User.Query().
		Where(user.Or(user.GoogleSubject(subject), user.Email(email))).
		Modify(func(s *entsql.Selector) {
			s.OrderExpr(repository.RawExpr(s.C(user.FieldGoogleSubject)+" = ? DESC", subject))
		}).
		Limit(1).
		IDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if len(existing) == 0 {
		created, err := self.base.DB.User.Create().SetGoogleSubject(subject).SetEmail(email).SetName(name).SetDisplayName(displayName).
			SetNillableAvatarURL(picture).Save(ctx)
		if err != nil {
			return uuid.Nil, err
		}
		return created.ID, nil
	}
	update := self.base.DB.User.UpdateOneID(existing[0]).SetGoogleSubject(subject).SetEmail(email).
		Modify(func(u *entsql.UpdateBuilder) { u.Set(user.FieldLastSeenAt, entsql.Expr("now()")) })
	if picture != nil {
		update.SetAvatarURL(*picture)
	}
	return existing[0], update.Exec(ctx)
}
