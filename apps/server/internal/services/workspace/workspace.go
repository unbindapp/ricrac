// Package workspace_service is the workspace, its members and invitations, and the audit log.
package workspace_service

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	email_service "ricrac/internal/services/email"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
	seed_service "ricrac/internal/services/seed"
)

type M = map[string]any

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i WorkspaceServiceInterface -p workspace_service -s WorkspaceService -o workspace_service_iface.go
type WorkspaceService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
	emails  email_service.EmailServiceInterface
}

func NewWorkspaceService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, emails email_service.EmailServiceInterface) *WorkspaceService {
	return &WorkspaceService{cfg: cfg, repo: repo, resolve: resolve, events: events, emails: emails}
}

type NoInput struct{}

type IDInput struct {
	ID string `json:"id"`
}

func (self *WorkspaceService) List(ctx context.Context, caller *auth.Caller, _ *NoInput) (models.WorkspaceList, error) {
	reachable, err := self.repo.Access().Reachable(ctx, caller.User, caller.Workspaces)
	if err != nil {
		return models.WorkspaceList{}, err
	}
	nodes := []models.ReachableWorkspace{}
	for _, w := range reachable {
		nodes = append(nodes, models.ReachableWorkspace{
			ID: models.ID(w.ID), Name: w.Name, URLKey: w.URLKey, URL: self.cfg.PublicURL + "/" + w.URLKey, Role: w.Role,
		})
	}
	return models.WorkspaceList{Nodes: nodes}, nil
}

func (self *WorkspaceService) value(ctx context.Context, tx repository.TxInterface, actor *auth.Actor) (models.Workspace, error) {
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.Workspace{}, err
	}
	w, err := self.repo.Workspace().Get(ctx, tx, actor.Workspace)
	if err != nil {
		return models.Workspace{}, err
	}
	teams, err := self.repo.Workspace().Teams(ctx, tx, visible)
	if err != nil {
		return models.Workspace{}, err
	}
	members, err := self.repo.Workspace().ActiveMemberCount(ctx, tx, actor.Workspace)
	if err != nil {
		return models.Workspace{}, err
	}
	issues, err := self.repo.Workspace().IssueCount(ctx, tx, actor.Workspace, visible)
	if err != nil {
		return models.Workspace{}, err
	}
	return models.WorkspaceOf(w, self.cfg.PublicURL, actor.Role.String(), teams, members, issues), nil
}

func (self *WorkspaceService) Get(ctx context.Context, actor *auth.Actor, _ *NoInput) (models.Workspace, error) {
	return self.value(ctx, nil, actor)
}

type UpdateInput struct {
	Name    *string               `json:"name" doc:"1 to 80 characters."`
	URLKey  *string               `json:"urlKey" doc:"2 to 32 lowercase letters, digits and dashes; must not be taken by another workspace."`
	LogoURL args.Nullable[string] `json:"logoUrl" doc:"Image URL; null removes the logo."`
	Theme   *string               `json:"theme" enum:"default,dank" doc:"The look of the web app for everyone in the workspace."`
	Public  *bool                 `json:"public" doc:"Let anyone with a link read its issues without signing in."`
}

var themes = []string{"default", "dank"}

func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (self *WorkspaceService) Update(ctx context.Context, actor *auth.Actor, input *UpdateInput) (models.Workspace, error) {
	var value models.Workspace
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		was, err := self.repo.Workspace().GetForUpdate(ctx, tx, actor.Workspace)
		if err != nil {
			return err
		}
		changes := M{}
		if input.Name != nil {
			name, err := utils.NonEmpty(*input.Name, "name")
			if err != nil {
				return err
			}
			if utf8.RuneCountInString(name) > 80 {
				return errdefs.BadRequest("The name is 1 to 80 characters")
			}
			if name != was.Name {
				changes["name"] = M{"from": was.Name, "to": name}
				if err := self.repo.Workspace().SetName(ctx, tx, actor.Workspace, name); err != nil {
					return err
				}
			}
		}
		if input.URLKey != nil {
			key, err := seed_service.CheckURLKey(*input.URLKey)
			if err != nil {
				return err
			}
			if key != was.URLKey {
				taken, err := self.repo.Workspace().URLKeyTaken(ctx, tx, key, actor.Workspace)
				if err != nil {
					return err
				}
				if taken {
					return errdefs.Conflict("The URL key %q is taken", key)
				}
				changes["urlKey"] = M{"from": was.URLKey, "to": key}
				if err := self.repo.Workspace().SetURLKey(ctx, tx, actor.Workspace, key); err != nil {
					return err
				}
			}
		}
		if input.LogoURL.Set {
			logo := utils.Trimmed(input.LogoURL.Ptr())
			if !sameText(logo, was.LogoURL) {
				changes["logoUrl"] = M{"from": was.LogoURL, "to": logo}
				if err := self.repo.Workspace().SetLogoURL(ctx, tx, actor.Workspace, logo); err != nil {
					return err
				}
			}
		}
		if input.Theme != nil {
			if !slices.Contains(themes, *input.Theme) {
				return errdefs.BadRequest("theme %q isn't one of %s", *input.Theme, strings.Join(themes, ", "))
			}
			if *input.Theme != was.Theme {
				changes["theme"] = M{"from": was.Theme, "to": *input.Theme}
				if err := self.repo.Workspace().SetTheme(ctx, tx, actor.Workspace, *input.Theme); err != nil {
					return err
				}
			}
		}
		if input.Public != nil && *input.Public != was.Public {
			changes["public"] = M{"from": was.Public, "to": *input.Public}
			if err := self.repo.Workspace().SetPublic(ctx, tx, actor.Workspace, *input.Public); err != nil {
				return err
			}
		}
		if len(changes) > 0 {
			now, err := self.repo.Now(ctx, tx)
			if err != nil {
				return err
			}
			if err := self.repo.Workspace().Touch(ctx, tx, actor.Workspace, now); err != nil {
				return err
			}
			if err := self.events.Audit(ctx, tx, actor, "workspace.updated", changes); err != nil {
				return err
			}
		}
		value, err = self.value(ctx, tx, actor)
		return err
	})
	if err != nil {
		return models.Workspace{}, err
	}
	return value, nil
}

type AuditLogInput struct {
	Type *string `json:"type" doc:"Only entries of this type, e.g. member.roleChanged, member.removed, invitation.created, team.archived, workspace.updated."`
	args.Page
}

func (self *WorkspaceService) AuditLog(ctx context.Context, actor *auth.Actor, input *AuditLogInput) (args.PageOf[models.AuditEntry], error) {
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.AuditEntry]{}, err
	}
	var kind *string
	if input.Type != nil {
		kind = new(strings.TrimSpace(*input.Type))
	}
	entries, err := self.repo.Workspace().AuditEntries(ctx, actor.Workspace, kind, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.AuditEntry]{}, err
	}
	nodes := make([]models.AuditEntry, len(entries))
	for i, e := range entries {
		nodes[i] = models.AuditEntryOf(e)
	}
	return args.WrapOf(input.Page, nodes)
}
