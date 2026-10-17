package workspace_service

import (
	"context"
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

type ListUsersInput struct {
	Query           *string `json:"query" doc:"Part of a name, display name or email."`
	IncludeInactive bool    `json:"includeInactive,omitempty" doc:"Also list removed members."`
	Team            *string `json:"team" doc:"Only members of this team (key, name or id)."`
	args.Page
}

// users is the members as the API answers them, in the order of ids, read in tx.
func (self *WorkspaceService) users(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, ids []uuid.UUID) ([]models.WorkspaceUser, error) {
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return nil, err
	}
	members, err := self.repo.Workspace().Memberships(ctx, tx, actor.Workspace, ids)
	if err != nil {
		return nil, err
	}
	teams, err := self.repo.Workspace().MemberTeams(ctx, tx, actor.Workspace, ids, visible)
	if err != nil {
		return nil, err
	}
	values := make([]models.WorkspaceUser, len(members))
	for i, m := range members {
		values[i] = models.WorkspaceUserOf(m, models.ID(actor.User), teams[m.UserID])
	}
	return values, nil
}

func (self *WorkspaceService) ListUsers(ctx context.Context, actor *auth.Actor, input *ListUsersInput) (args.PageOf[models.WorkspaceUser], error) {
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return args.PageOf[models.WorkspaceUser]{}, err
	}
	if _, err := actor.TeamIDs(ctx); err != nil {
		return args.PageOf[models.WorkspaceUser]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.WorkspaceUser]{}, err
	}
	ids, err := self.repo.Workspace().ListMembers(ctx, workspace_repo.MemberFilter{
		Workspace: actor.Workspace, IncludeInactive: input.IncludeInactive, Pattern: utils.LikePattern(input.Query),
		Team: team, Limit: input.Size() + 1, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.WorkspaceUser]{}, err
	}
	users, err := self.users(ctx, nil, actor, ids)
	if err != nil {
		return args.PageOf[models.WorkspaceUser]{}, err
	}
	return args.WrapOf(input.Page, users)
}

// findMember is an active member by id, name, email or "me", or a removed member by id.
func (self *WorkspaceService) findMember(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	found, err := self.resolve.User(ctx, actor, value)
	if err == nil {
		return found, nil
	}
	id, ok := utils.AsUUID(value)
	if !ok {
		return found, err
	}
	removed, queryErr := self.repo.Workspace().HasMember(ctx, actor.Workspace, id)
	if queryErr != nil {
		return uuid.Nil, queryErr
	}
	if !removed {
		return found, err
	}
	return id, nil
}

func (self *WorkspaceService) userValue(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, user uuid.UUID) (models.WorkspaceUserDetail, error) {
	users, err := self.users(ctx, tx, actor, []uuid.UUID{user})
	if err != nil {
		return models.WorkspaceUserDetail{}, err
	}
	if len(users) == 0 {
		return models.WorkspaceUserDetail{}, errdefs.NotFound("No such member")
	}
	open, err := self.repo.Workspace().OpenAssignedCount(ctx, tx, actor.Workspace, user)
	if err != nil {
		return models.WorkspaceUserDetail{}, err
	}
	return models.WorkspaceUserDetail{WorkspaceUser: users[0], OpenIssueCount: open}, nil
}

func (self *WorkspaceService) GetUser(ctx context.Context, actor *auth.Actor, input *IDInput) (models.WorkspaceUserDetail, error) {
	user, err := self.findMember(ctx, actor, input.ID)
	if err != nil {
		return models.WorkspaceUserDetail{}, err
	}
	return self.userValue(ctx, nil, actor, user)
}

type UpdateUserInput struct {
	ID                 string                `json:"id" doc:"The member: id, name, email or \"me\"."`
	Role               *string               `json:"role" doc:"owner, admin, member or guest. Needs an admin; owner changes need an owner."`
	Name               *string               `json:"name" doc:"Your full name."`
	DisplayName        *string               `json:"displayName" doc:"Your short handle, used for @mentions."`
	Timezone           args.Nullable[string] `json:"timezone" doc:"Your IANA timezone, like Europe/Berlin; null clears it."`
	EmailNotifications *bool                 `json:"emailNotifications" doc:"Whether you get unread notifications by email."`
}

func (self *WorkspaceService) UpdateUser(ctx context.Context, actor *auth.Actor, input *UpdateUserInput) (models.WorkspaceUserDetail, error) {
	user, err := self.resolve.User(ctx, actor, input.ID)
	if err != nil {
		return models.WorkspaceUserDetail{}, err
	}
	profileChange := input.Name != nil || input.DisplayName != nil || input.Timezone.Set || input.EmailNotifications != nil
	if profileChange && user != actor.User {
		return models.WorkspaceUserDetail{}, errdefs.Forbidden("You can only change your own name, display name, timezone and email notifications")
	}
	var value models.WorkspaceUserDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if input.Role != nil {
			if err := self.changeRole(ctx, tx, actor, user, *input.Role); err != nil {
				return err
			}
		}
		if input.Name != nil {
			name, err := utils.NonEmpty(*input.Name, "name")
			if err != nil {
				return err
			}
			if err := self.repo.Workspace().SetUserName(ctx, tx, user, name); err != nil {
				return err
			}
		}
		if input.DisplayName != nil {
			displayName, err := utils.NonEmpty(strings.TrimLeft(*input.DisplayName, "@"), "displayName")
			if err != nil {
				return err
			}
			if strings.IndexFunc(displayName, unicode.IsSpace) >= 0 {
				return errdefs.BadRequest("A display name is one word, used for @mentions, like \"ann\"")
			}
			if err := self.repo.Workspace().SetDisplayName(ctx, tx, user, displayName); err != nil {
				return err
			}
		}
		if input.Timezone.Set {
			timezone := utils.Trimmed(input.Timezone.Ptr())
			if timezone != nil {
				if err := self.CheckTimezone(ctx, tx, *timezone); err != nil {
					return err
				}
			}
			if err := self.repo.Workspace().SetTimezone(ctx, tx, user, timezone); err != nil {
				return err
			}
		}
		if input.EmailNotifications != nil {
			if err := self.repo.Workspace().SetEmailNotifications(ctx, tx, user, *input.EmailNotifications); err != nil {
				return err
			}
		}
		var err error
		value, err = self.userValue(ctx, tx, actor, user)
		return err
	})
	if err != nil {
		return models.WorkspaceUserDetail{}, err
	}
	return value, nil
}

// CheckTimezone accepts IANA timezone names Postgres knows, like Europe/Berlin or UTC.
func (self *WorkspaceService) CheckTimezone(ctx context.Context, tx repository.TxInterface, zone string) error {
	known, err := self.repo.Workspace().TimezoneKnown(ctx, tx, zone)
	if err != nil {
		return err
	}
	if !known {
		return errdefs.BadRequest("%q isn't a timezone; use an IANA name like Europe/Berlin or UTC", zone)
	}
	return nil
}

func (self *WorkspaceService) roleOf(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, user uuid.UUID) (auth.Role, error) {
	name, err := self.repo.Workspace().RoleForUpdate(ctx, tx, actor.Workspace, user)
	if err != nil {
		return auth.Guest, err
	}
	role, ok := auth.ParseRole(name)
	if !ok {
		return auth.Guest, errdefs.BadRequest("Unknown role")
	}
	return role, nil
}

func (self *WorkspaceService) changeRole(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, user uuid.UUID, value string) error {
	if err := actor.Require(auth.AdminAccess); err != nil {
		return err
	}
	role, ok := auth.ParseRole(strings.TrimSpace(value))
	if !ok {
		return errdefs.BadRequest("role is owner, admin, member or guest")
	}
	old, err := self.roleOf(ctx, tx, actor, user)
	if err != nil {
		return err
	}
	if (role == auth.Owner || old == auth.Owner) && role != old && actor.Role != auth.Owner {
		return errdefs.Forbidden("Only owners can make someone an owner or take it away")
	}
	if old == auth.Owner && role != auth.Owner {
		owners, err := self.repo.Workspace().ActiveOwners(ctx, tx, actor.Workspace)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return errdefs.BadRequest("The workspace needs an owner; make someone else owner first")
		}
	}
	if role == old {
		return nil
	}
	if err := self.repo.Workspace().SetRole(ctx, tx, actor.Workspace, user, role.String()); err != nil {
		return err
	}
	return self.events.Audit(ctx, tx, actor, "member.roleChanged", M{"user": user, "from": old.String(), "to": role.String()})
}

func (self *WorkspaceService) RemoveUser(ctx context.Context, actor *auth.Actor, input *IDInput) (models.RemovedUser, error) {
	user, err := self.resolve.User(ctx, actor, input.ID)
	if err != nil {
		return models.RemovedUser{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		role, err := self.roleOf(ctx, tx, actor, user)
		if err != nil {
			return err
		}
		if role == auth.Owner {
			if actor.Role != auth.Owner {
				return errdefs.Forbidden("Only owners can remove an owner")
			}
			owners, err := self.repo.Workspace().ActiveOwners(ctx, tx, actor.Workspace)
			if err != nil {
				return err
			}
			if owners <= 1 {
				return errdefs.BadRequest("The last owner can't be removed; make someone else owner first")
			}
		}
		if err := self.repo.Workspace().Deactivate(ctx, tx, actor.Workspace, user); err != nil {
			return err
		}
		removed, err := self.repo.Workspace().User(ctx, tx, user)
		if err != nil {
			return err
		}
		return self.events.Audit(ctx, tx, actor, "member.removed", M{"user": user, "email": removed.Email, "role": role.String()})
	})
	if err != nil {
		return models.RemovedUser{}, err
	}
	return models.RemovedUser{Removed: models.ID(user)}, nil
}
