package access_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/repositories/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AccessServiceInterface -p access_service -s AccessService -o access_service_iface.go
type AccessService struct {
	cfg  config.Config
	repo repositories.RepositoriesInterface
}

func NewAccessService(cfg config.Config, repo repositories.RepositoriesInterface) *AccessService {
	return &AccessService{cfg: cfg, repo: repo}
}

// Reachable is the workspaces the caller's token reaches that the user is still an active
// member of, by name.
func (self *AccessService) Reachable(ctx context.Context, caller *auth.Caller) ([]auth.Reached, error) {
	return self.repo.Access().Reachable(ctx, caller.User, caller.Workspaces)
}

// Pick is the caller in one workspace: the one named (URL key or id), else the one the links
// among the arguments point to, else the only one the token reaches.
func (self *AccessService) Pick(ctx context.Context, caller *auth.Caller, named string, args map[string]any) (*auth.Actor, error) {
	reachable, err := self.Reachable(ctx, caller)
	if err != nil {
		return nil, err
	}
	if len(reachable) == 0 {
		return nil, errdefs.Forbidden("This token doesn't reach any workspace you're a member of")
	}
	var chosen *auth.Reached
	if name := strings.TrimSpace(named); name != "" {
		key := strings.ToLower(name)
		for i := range reachable {
			if reachable[i].URLKey == key || reachable[i].ID.String() == key {
				chosen = &reachable[i]
			}
		}
		if chosen == nil {
			return nil, errdefs.BadRequest("No workspace %q here; use one of %s", name, choices(reachable))
		}
	} else {
		chosen = linked(reachable, args, self.cfg.PublicURL)
		if chosen == nil && len(reachable) == 1 {
			chosen = &reachable[0]
		}
		if chosen == nil {
			return nil, errdefs.BadRequest("This token reaches several workspaces: pass \"workspace\" with one of %s", choices(reachable))
		}
	}
	role, _ := auth.ParseRole(chosen.Role)
	return auth.NewActor(caller.User, chosen.ID, role, caller.Scopes, nil, self.visibleTeams), nil
}

// Load is user in the workspace, or nil when they aren't an active member.
func (self *AccessService) Load(ctx context.Context, user, workspace uuid.UUID, scopes []auth.Access) (*auth.Actor, error) {
	value, found, err := self.repo.Access().Role(ctx, workspace, user)
	if err != nil || !found {
		return nil, err
	}
	role, ok := auth.ParseRole(value)
	if !ok {
		return nil, nil
	}
	return auth.NewActor(user, workspace, role, scopes, nil, self.visibleTeams), nil
}

// System is Ricrac itself acting in a workspace, as actor (a member, or uuid.Nil for nobody),
// seeing every team.
func (self *AccessService) System(workspace, actor uuid.UUID, origin auth.Origin) *auth.Actor {
	return auth.NewActor(actor, workspace, auth.Owner, auth.EveryAccess, &origin, self.visibleTeams)
}

// Visitor is someone who isn't a member reading the public workspace named (URL key or id), nil
// when no such workspace is public.
func (self *AccessService) Visitor(ctx context.Context, named string) (*auth.Actor, error) {
	name := strings.ToLower(strings.TrimSpace(named))
	if name == "" {
		return nil, nil
	}
	workspace, err := self.repo.Access().PublicWorkspace(ctx, name)
	if err != nil || workspace == uuid.Nil {
		return nil, err
	}
	return auth.NewVisitor(workspace, self.visibleTeams), nil
}

func (self *AccessService) visibleTeams(ctx context.Context, actor *auth.Actor) ([]uuid.UUID, error) {
	guest := actor.Role == auth.Guest && !actor.Visitor
	return self.repo.Access().VisibleTeams(ctx, actor.Workspace, actor.User, guest, actor.Origin != nil)
}

// Authenticate is the caller of a bearer token (an API key or an OAuth access token), nil when
// it isn't a valid one.
func (self *AccessService) Authenticate(ctx context.Context, token string) (*auth.Caller, error) {
	if token == "" {
		return nil, nil
	}
	if strings.HasPrefix(token, auth.APIKeyPrefix) {
		return self.repo.Access().APIKeyCaller(ctx, auth.SecretHash(token))
	}
	return self.repo.Access().AccessTokenCaller(ctx, auth.SecretHash(token))
}

func choices(reachable []auth.Reached) string {
	names := make([]string, len(reachable))
	for i, w := range reachable {
		names[i] = fmt.Sprintf("%q (%s)", w.URLKey, w.Name)
	}
	return strings.Join(names, ", ")
}

// linked is the workspace of the links passed as arguments, like
// https://ricrac.example/acme/issue/ENG-1, when they all point to the same one.
func linked(reachable []auth.Reached, args map[string]any, base string) *auth.Reached {
	prefix := base + "/"
	var found *auth.Reached
	for _, value := range args {
		items, ok := value.([]any)
		if !ok {
			items = []any{value}
		}
		for _, item := range items {
			link, ok := item.(string)
			if !ok || strings.ContainsAny(link, " \t\n\r") {
				continue
			}
			rest, ok := strings.CutPrefix(link, prefix)
			if !ok {
				continue
			}
			key, _, _ := strings.Cut(rest, "/")
			for i := range reachable {
				if reachable[i].URLKey != key {
					continue
				}
				if found != nil && found.ID != reachable[i].ID {
					return nil
				}
				found = &reachable[i]
			}
		}
	}
	return found
}
