// Package account_service is the dashboard's account API: who is signed in, their workspaces
// and invitations, API keys, the OAuth apps of a workspace and the apps they connected.
package account_service

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	seed_service "ricrac/internal/services/seed"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AccountServiceInterface -p account_service -s AccountService -o account_service_iface.go
type AccountService struct {
	cfg    config.Config
	repo   repositories.RepositoriesInterface
	access access_service.AccessServiceInterface
	seed   seed_service.SeedServiceInterface
}

func NewAccountService(cfg config.Config, repo repositories.RepositoriesInterface, access access_service.AccessServiceInterface, seed seed_service.SeedServiceInterface) *AccountService {
	return &AccountService{cfg: cfg, repo: repo, access: access, seed: seed}
}

type UpdateMe struct {
	Name        *string `json:"name"`
	DisplayName *string `json:"displayName"`
}

type CreateWorkspace struct {
	Name string `json:"name"`
	// Lowercase letters, digits and dashes, used in links.
	URLKey string `json:"urlKey"`
	// The first team; named after the workspace when left out.
	TeamName *string `json:"teamName"`
	TeamKey  *string `json:"teamKey"`
}

type DeleteWorkspace struct {
	ConfirmURLKey string `json:"confirmUrlKey"`
}

func (self *AccountService) Config() models.AppConfig {
	return models.AppConfig{GoogleConfigured: self.cfg.GoogleConfigured(), PublicURL: self.cfg.PublicURL, MCPURL: self.cfg.MCPURL()}
}

// SignedIn checks that the session's user still exists.
func (self *AccountService) SignedIn(ctx context.Context, user uuid.UUID) error {
	exists, err := self.repo.Account().UserExists(ctx, user)
	if err != nil {
		return err
	}
	if !exists {
		return errdefs.Unauthorized()
	}
	return nil
}

// Member is the user in the workspace named (id or URL key), or their only one when named is
// empty.
func (self *AccountService) Member(ctx context.Context, user uuid.UUID, named string) (*auth.Actor, error) {
	var name *string
	if value := strings.TrimSpace(named); value != "" {
		name = &value
	}
	ids, err := self.repo.Account().MemberWorkspaces(ctx, user, name)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errdefs.Forbidden("No such workspace")
	}
	if len(ids) > 1 {
		return nil, errdefs.BadRequest("Name the workspace in the x-ricrac-workspace header")
	}
	actor, err := self.access.Load(ctx, user, ids[0], auth.EveryAccess)
	if err != nil {
		return nil, err
	}
	if actor == nil {
		return nil, errdefs.Forbidden("Not a member")
	}
	return actor, nil
}

// Me is the signed-in user, nil when they no longer exist.
func (self *AccountService) Me(ctx context.Context, id uuid.UUID) (*models.Me, error) {
	user, err := self.repo.Account().User(ctx, id)
	if err != nil || user == nil {
		return nil, err
	}
	workspaces, err := self.repo.Account().Memberships(ctx, id)
	if err != nil {
		return nil, err
	}
	invitations, err := self.repo.Account().PendingInvitations(ctx, user.Email, id)
	if err != nil {
		return nil, err
	}
	return &models.Me{
		ID: id, Email: user.Email, Name: user.Name, DisplayName: user.DisplayName, AvatarURL: user.AvatarURL, GitHubLogin: user.GithubLogin,
		Workspaces: models.NonNil(workspaces), Invitations: models.NonNil(invitations),
	}, nil
}

func validHandle(handle string) bool {
	return handle != "" && len(handle) <= 40 && strings.IndexFunc(handle, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '.' && c != '_' && c != '-'
	}) < 0
}

func (self *AccountService) UpdateMe(ctx context.Context, user uuid.UUID, input *UpdateMe) error {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len(name) > 100 {
			return errdefs.BadRequest("Name must be 1 to 100 characters")
		}
		if err := self.repo.Account().SetName(ctx, user, name); err != nil {
			return err
		}
	}
	if input.DisplayName == nil {
		return nil
	}
	handle := strings.TrimSpace(*input.DisplayName)
	if !validHandle(handle) {
		return errdefs.BadRequest("Display names are 1 to 40 letters, digits, dots, dashes or underscores")
	}
	return self.repo.Account().SetDisplayName(ctx, user, handle)
}

func (self *AccountService) CreateWorkspace(ctx context.Context, user uuid.UUID, input *CreateWorkspace) (models.WorkspaceSummary, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return models.WorkspaceSummary{}, errdefs.BadRequest("The name is 1 to 80 characters")
	}
	urlKey, err := seed_service.CheckURLKey(input.URLKey)
	if err != nil {
		return models.WorkspaceSummary{}, err
	}
	teamName := name
	if input.TeamName != nil && strings.TrimSpace(*input.TeamName) != "" {
		teamName = strings.TrimSpace(*input.TeamName)
	}
	teamKey := seed_service.TeamKey(teamName)
	if input.TeamKey != nil {
		teamKey = *input.TeamKey
	}
	if teamKey, err = seed_service.CheckTeamKey(teamKey); err != nil {
		return models.WorkspaceSummary{}, err
	}
	var summary models.WorkspaceSummary
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		taken, err := self.repo.Account().URLKeyTaken(ctx, tx, urlKey)
		if err != nil {
			return err
		}
		if taken {
			return errdefs.Conflict("The URL key %q is taken", urlKey)
		}
		id, err := self.repo.Account().CreateWorkspace(ctx, tx, name, urlKey, user)
		if err != nil {
			return err
		}
		if err := self.seed.WorkspaceDefaults(ctx, tx, id); err != nil {
			return err
		}
		if _, err := self.seed.CreateTeam(ctx, tx, id, user, teamName, teamKey); err != nil {
			return err
		}
		summary = models.WorkspaceSummary{ID: id, Name: name, URLKey: urlKey, Role: "owner"}
		return nil
	})
	return summary, err
}

// DeleteWorkspace deletes the workspace and everything in it. Owners only, confirmed by
// repeating its URL key.
func (self *AccountService) DeleteWorkspace(ctx context.Context, user, id uuid.UUID, input *DeleteWorkspace) error {
	actor, err := self.access.Load(ctx, user, id, nil)
	if err != nil {
		return err
	}
	if actor == nil {
		return errdefs.NotFound("No such workspace")
	}
	if actor.Role != auth.Owner {
		return errdefs.Forbidden("Only owners can delete a workspace")
	}
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		deleted, err := self.repo.Account().DeleteWorkspace(ctx, tx, id, strings.ToLower(strings.TrimSpace(input.ConfirmURLKey)))
		if err != nil {
			return err
		}
		if deleted != 1 {
			return errdefs.BadRequest("Type the workspace's URL key to confirm")
		}
		return self.repo.Account().DropWorkspaceAccess(ctx, tx, id, nil)
	})
}

// LeaveWorkspace leaves the workspace. The last owner has to hand ownership over or delete it
// instead.
func (self *AccountService) LeaveWorkspace(ctx context.Context, user, id uuid.UUID) error {
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		owners, err := self.repo.Account().LockOwners(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(owners) == 1 && owners[0] == user {
			return errdefs.BadRequest("You're the only owner: make someone else an owner first, or delete the workspace")
		}
		if err := self.repo.Account().Leave(ctx, tx, id, user); err != nil {
			return err
		}
		return self.repo.Account().DropWorkspaceAccess(ctx, tx, id, &user)
	})
}

func (self *AccountService) AcceptInvitation(ctx context.Context, user, id uuid.UUID) (models.WorkspaceSummary, error) {
	var summary models.WorkspaceSummary
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		invitation, err := self.repo.Account().TakeInvitation(ctx, tx, id, user)
		if err != nil {
			return err
		}
		if invitation == nil {
			return errdefs.NotFound("No such invitation, or it expired")
		}
		if err := self.repo.Account().Join(ctx, tx, invitation.Workspace, user, invitation.Role, invitation.Teams); err != nil {
			return err
		}
		joined, err := self.repo.Account().Membership(ctx, tx, invitation.Workspace, user)
		if joined != nil {
			summary = *joined
		}
		return err
	})
	return summary, err
}

func (self *AccountService) DeclineInvitation(ctx context.Context, user, id uuid.UUID) error {
	return self.repo.Account().DeclineInvitation(ctx, id, user)
}

// CheckWorkspaces checks that a list of workspaces to grant is non-empty and only holds the
// user's. Nil is all of them.
func (self *AccountService) CheckWorkspaces(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID) error {
	if workspaces == nil {
		return nil
	}
	if len(workspaces) == 0 {
		return errdefs.BadRequest("Pick at least one workspace.")
	}
	active, err := self.repo.Account().ActiveIn(ctx, user, workspaces)
	if err != nil {
		return err
	}
	for _, id := range workspaces {
		if !slices.Contains(active, id) {
			return errdefs.BadRequest("You aren't a member of one of those workspaces.")
		}
	}
	return nil
}
