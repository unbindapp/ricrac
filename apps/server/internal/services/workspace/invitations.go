package workspace_service

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	workspace_repo "ricrac/internal/repositories/workspace"
)

type ListInvitationsInput struct {
	args.Page
}

func (self *WorkspaceService) invitations(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.Invitation, error) {
	invitations, err := self.repo.Workspace().Invitations(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.Invitation, len(invitations))
	for i, invitation := range invitations {
		teams, err := self.repo.Workspace().TeamsByID(ctx, tx, invitation.TeamIds)
		if err != nil {
			return nil, err
		}
		values[i] = models.InvitationOf(invitation, teams)
	}
	return values, nil
}

func (self *WorkspaceService) ListInvitations(ctx context.Context, actor *auth.Actor, input *ListInvitationsInput) (args.PageOf[models.Invitation], error) {
	if err := actor.RequireMember(); err != nil {
		return args.PageOf[models.Invitation]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Invitation]{}, err
	}
	ids, err := self.repo.Workspace().ListInvitations(ctx, actor.Workspace, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.Invitation]{}, err
	}
	values, err := self.invitations(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.Invitation]{}, err
	}
	return args.WrapOf(input.Page, values)
}

type InviteInput struct {
	Email string   `json:"email" doc:"Their email address; they sign in with the Google account of this email."`
	Role  *string  `json:"role" doc:"member (default), admin (only admins invite admins) or guest."`
	Teams []string `json:"teams" doc:"Optional: everyone joins the workspace's team. Guests see only the teams named here, the workspace's team by default (keys, names or ids)."`
}

func validEmail(email string) bool {
	local, domain, found := strings.Cut(email, "@")
	return found && local != "" && strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") &&
		!strings.HasSuffix(domain, ".") && strings.IndexFunc(email, unicode.IsSpace) < 0
}

// invitedRole is the role an invitation gives, if the actor may give it.
func invitedRole(actor *auth.Actor, value *string) (auth.Role, error) {
	switch name := strings.TrimSpace(utils.OrEmpty(value)); {
	case value == nil || name == "member":
		return auth.Member, nil
	case name == "guest":
		return auth.Guest, nil
	case name == "admin":
		if err := actor.Require(auth.AdminAccess); err != nil {
			return auth.Guest, errdefs.Forbidden("Only admins can invite admins")
		}
		return auth.Admin, nil
	default:
		return auth.Guest, errdefs.BadRequest("role is member, admin or guest, not %q", name)
	}
}

func (self *WorkspaceService) Invite(ctx context.Context, actor *auth.Actor, input *InviteInput) (models.Invitation, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Invitation{}, err
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(email) {
		return models.Invitation{}, errdefs.BadRequest("%q isn't an email address", email)
	}
	role, err := invitedRole(actor, input.Role)
	if err != nil {
		return models.Invitation{}, err
	}
	teams := []uuid.UUID{}
	for _, team := range input.Teams {
		id, err := self.resolve.Team(ctx, actor, team)
		if err != nil {
			return models.Invitation{}, err
		}
		if !slices.Contains(teams, id) {
			teams = append(teams, id)
		}
	}
	if role == auth.Guest && len(teams) == 0 {
		team, err := self.resolve.DefaultTeam(ctx, actor)
		if err != nil {
			return models.Invitation{}, err
		}
		teams = append(teams, team)
	}
	var value models.Invitation
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		member, err := self.repo.Workspace().HasActiveMemberWithEmail(ctx, tx, actor.Workspace, email)
		if err != nil {
			return err
		}
		if member {
			return errdefs.Conflict("%s is already a member of this workspace", email)
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		id, err := self.repo.Workspace().UpsertInvitation(ctx, tx, workspace_repo.InvitationInput{
			Workspace: actor.Workspace, Email: email, Role: role.String(), Teams: teams, Inviter: actor.User, Now: now,
		})
		if err != nil {
			return err
		}
		if err := self.events.Audit(ctx, tx, actor, "invitation.created", M{"email": email, "role": role.String()}); err != nil {
			return err
		}
		if err := self.emails.QueueInvitation(ctx, tx, actor, email, role); err != nil {
			return err
		}
		values, err := self.invitations(ctx, tx, []uuid.UUID{id})
		if err != nil {
			return err
		}
		value = values[0]
		return nil
	})
	if err != nil {
		return models.Invitation{}, err
	}
	return value, nil
}

func (self *WorkspaceService) DeleteInvitation(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Deleted, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Deleted{}, err
	}
	id, err := self.resolve.ByID(ctx, actor, "invitations", "invitation", input.ID)
	if err != nil {
		return models.Deleted{}, err
	}
	invitation, err := self.repo.Workspace().Invitation(ctx, id)
	if err != nil {
		return models.Deleted{}, err
	}
	if (invitation.InviterID == nil || *invitation.InviterID != actor.User) && actor.Role < auth.Admin {
		return models.Deleted{}, errdefs.Forbidden("Only admins can withdraw invitations someone else sent")
	}
	if err := self.repo.Workspace().DeleteInvitation(ctx, id); err != nil {
		return models.Deleted{}, err
	}
	return models.Deleted{Deleted: models.ID(id)}, nil
}
