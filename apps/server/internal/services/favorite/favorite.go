// Package favorite_service is favorites: the caller's own shortcuts to issues, projects, views,
// documents and more, in optional folders.
package favorite_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	favorite_repo "ricrac/internal/repositories/favorite"
	"ricrac/internal/repositories/repositories"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i FavoriteServiceInterface -p favorite_service -s FavoriteService -o favorite_service_iface.go
type FavoriteService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
}

func NewFavoriteService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface) *FavoriteService {
	return &FavoriteService{repo: repo, resolve: resolve}
}

type ListInput struct {
	args.Page
}

type AddInput struct {
	Type      string   `json:"type" doc:"issue, project, initiative, cycle, customView, document, label, team, user or release."`
	Target    string   `json:"target" doc:"What to favorite: its id, or the reference its type takes (ENG-123, a name, a slug, \"me\", ...)."`
	Team      *string  `json:"team" doc:"Team key, name or id, to find a cycle by number or name, or a team's label."`
	Folder    *string  `json:"folder" doc:"Folder name to put it in."`
	SortOrder *float64 `json:"sortOrder" doc:"Position among the favorites (lower is higher up); after the others by default."`
}

type RemoveInput struct {
	ID     *string `json:"id" doc:"The favorite's id; or, with type, what it points at."`
	Type   *string `json:"type" doc:"With target (or id): the type of what the favorite points at."`
	Target *string `json:"target" doc:"With type: what the favorite points at (id or reference)."`
	Team   *string `json:"team" doc:"Team key, name or id, to find a cycle by number or name, or a team's label."`
}

// types is the API's types and how they are stored.
var types = [][2]string{
	{"issue", "issue"},
	{"project", "project"},
	{"initiative", "initiative"},
	{"cycle", "cycle"},
	{"customView", "custom_view"},
	{"document", "document"},
	{"label", "label"},
	{"team", "team"},
	{"user", "user"},
	{"release", "release"},
}

func storedType(kind string) (string, error) {
	names := make([]string, len(types))
	for i, t := range types {
		if strings.EqualFold(t[0], strings.TrimSpace(kind)) {
			return t[1], nil
		}
		names[i] = t[0]
	}
	return "", errdefs.BadRequest("type %q isn't one of %s", kind, strings.Join(names, ", "))
}

func (self *FavoriteService) values(ctx context.Context, tx repository.TxInterface, favorites []*ent.Favorite) ([]models.Favorite, error) {
	targets, err := self.repo.Favorite().Targets(ctx, tx, favorites)
	if err != nil {
		return nil, err
	}
	values := make([]models.Favorite, len(favorites))
	for i, f := range favorites {
		values[i] = models.FavoriteOf(f, targetOf(targets, f))
	}
	return values, nil
}

// targetOf is the short form of what the favorite points at, nil when it is gone.
func targetOf(targets favorite_repo.Targets, f *ent.Favorite) any {
	id := f.TargetID
	switch f.Type {
	case "issue":
		if i, ok := targets.Issues[id]; ok {
			return models.IssueRefOf(i)
		}
	case "project":
		return models.ProjectRefOf(targets.Projects[id])
	case "initiative":
		return models.InitiativeRefOf(targets.Initiatives[id])
	case "cycle":
		if c, ok := targets.Cycles[id]; ok {
			return models.FavoriteCycleOf(c)
		}
	case "custom_view":
		if v, ok := targets.Views[id]; ok {
			return models.FavoriteCustomViewOf(v)
		}
	case "document":
		return models.DocumentRefOf(targets.Documents[id])
	case "label":
		if l, ok := targets.Labels[id]; ok {
			return models.LabelRefOf(l)
		}
	case "team":
		return models.TeamRefOf(targets.Teams[id])
	case "user":
		return models.UserRefOf(targets.Users[id])
	case "release":
		return models.ReleaseRefOf(targets.Releases[id])
	}
	return nil
}

func (self *FavoriteService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Favorite], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Favorite]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Favorite]{}, err
	}
	favorites, err := self.repo.Favorite().List(ctx, favorite_repo.ListFilter{
		Workspace: actor.Workspace, User: actor.User, Teams: teams, Limit: input.Size() + 1, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.Favorite]{}, err
	}
	values, err := self.values(ctx, nil, favorites)
	if err != nil {
		return args.PageOf[models.Favorite]{}, err
	}
	return args.WrapOf(input.Page, values)
}

// target is the id of what target names, for a favorite of kind (as stored).
func (self *FavoriteService) target(ctx context.Context, actor *auth.Actor, kind, target string, teamName *string) (uuid.UUID, error) {
	team, err := self.resolve.OptionalTeam(ctx, actor, teamName)
	if err != nil {
		return uuid.Nil, err
	}
	switch kind {
	case "issue":
		return self.resolve.Issue(ctx, actor, target)
	case "project":
		return self.resolve.Project(ctx, actor, target)
	case "initiative":
		return self.resolve.Initiative(ctx, actor, target)
	case "cycle":
		return self.resolve.Cycle(ctx, actor, team, target)
	case "custom_view":
		return self.view(ctx, actor, target)
	case "document":
		return self.resolve.Document(ctx, actor, target)
	case "label":
		ids, err := self.resolve.Labels(ctx, actor, team, []string{target})
		if err != nil {
			return uuid.Nil, err
		}
		return ids[0], nil
	case "team":
		return self.resolve.Team(ctx, actor, target)
	case "user":
		return self.resolve.User(ctx, actor, target)
	}
	return self.resolve.Release(ctx, actor, target)
}

// view is a custom view the caller can see, by id, slug, link or name.
func (self *FavoriteService) view(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	id, err := self.resolve.CustomView(ctx, actor, value)
	if err != nil {
		return uuid.Nil, err
	}
	team, err := self.repo.Favorite().ViewTeam(ctx, id)
	if err != nil || team == nil {
		return id, err
	}
	visible, err := actor.CanSeeTeam(ctx, *team)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, errdefs.NotFound("No view %q", strings.TrimSpace(value))
	}
	return id, nil
}

func (self *FavoriteService) Add(ctx context.Context, actor *auth.Actor, input *AddInput) (models.Favorite, error) {
	kind, err := storedType(input.Type)
	if err != nil {
		return models.Favorite{}, err
	}
	target, err := self.target(ctx, actor, kind, input.Target, input.Team)
	if err != nil {
		return models.Favorite{}, err
	}
	var value models.Favorite
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		id, err := self.repo.Favorite().Add(ctx, tx, favorite_repo.AddInput{
			Workspace: actor.Workspace, User: actor.User, Type: kind, Target: target,
			Folder: utils.Trimmed(input.Folder), SortOrder: input.SortOrder,
		})
		if err != nil {
			return err
		}
		added, err := self.repo.Favorite().Get(ctx, tx, id)
		if err != nil {
			return err
		}
		values, err := self.values(ctx, tx, []*ent.Favorite{added})
		if err != nil {
			return err
		}
		value = values[0]
		return nil
	})
	return value, err
}

func (self *FavoriteService) Remove(ctx context.Context, actor *auth.Actor, input *RemoveInput) (models.RemovedFavorite, error) {
	if input.ID != nil && input.Type == nil {
		if id, ok := utils.AsUUID(*input.ID); ok {
			removed, err := self.repo.Favorite().Remove(ctx, id, actor.User, actor.Workspace)
			if err != nil {
				return models.RemovedFavorite{}, err
			}
			if removed {
				return models.RemovedFavorite{Removed: true, ID: id.String()}, nil
			}
		}
	}
	reference := input.Target
	if reference == nil {
		reference = input.ID
	}
	if input.Type == nil || reference == nil {
		return models.RemovedFavorite{}, errdefs.NotFound("No such favorite; pass its id, or type and target")
	}
	kind, err := storedType(*input.Type)
	if err != nil {
		return models.RemovedFavorite{}, err
	}
	target, err := self.target(ctx, actor, kind, *reference, input.Team)
	if err != nil {
		return models.RemovedFavorite{}, err
	}
	removed, found, err := self.repo.Favorite().RemoveTarget(ctx, actor.User, actor.Workspace, kind, target)
	if err != nil {
		return models.RemovedFavorite{}, err
	}
	if !found {
		return models.RemovedFavorite{}, errdefs.NotFound("That isn't one of your favorites")
	}
	return models.RemovedFavorite{Removed: true, ID: removed.String()}, nil
}
