package document_service

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	document_repo "ricrac/internal/repositories/document"
)

var listOrders = map[string]string{"updatedAt": "updated_at", "createdAt": "created_at"}

func (self *DocumentService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Document], error) {
	orderBy := "updatedAt"
	if input.OrderBy != nil {
		orderBy = *input.OrderBy
	}
	order, ok := listOrders[orderBy]
	if !ok {
		return args.PageOf[models.Document]{}, errdefs.BadRequest("orderBy %q isn't updatedAt or createdAt", orderBy)
	}
	parents, err := self.listParents(ctx, actor, input)
	if err != nil {
		return args.PageOf[models.Document]{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Document]{}, err
	}
	filter := document_repo.ListFilter{
		Workspace: actor.Workspace, Teams: teams, Viewer: actor.User, Parents: parents,
		IncludeTrashed: input.IncludeTrashed, OrderBy: order,
	}
	if input.Creator != nil {
		creator, err := self.resolve.User(ctx, actor, *input.Creator)
		if err != nil {
			return args.PageOf[models.Document]{}, err
		}
		filter.Creator = &creator
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Query = &query
	}
	if filter.Offset, err = input.Offset(); err != nil {
		return args.PageOf[models.Document]{}, err
	}
	filter.Limit = input.Size() + 1
	ids, err := self.repo.Document().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.Document]{}, err
	}
	documents, err := self.values(ctx, nil, ids, input.IncludeContent)
	if err != nil {
		return args.PageOf[models.Document]{}, err
	}
	return args.WrapOf(input.Page, documents)
}

// listParents is the parents a list is narrowed to.
func (self *DocumentService) listParents(ctx context.Context, actor *auth.Actor, input *ListInput) ([]document_repo.Parent, error) {
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return nil, err
	}
	var parents []document_repo.Parent
	for _, p := range []struct {
		column  string
		value   *string
		resolve func(context.Context, *auth.Actor, string) (uuid.UUID, error)
	}{
		{"project_id", input.Project, self.resolve.Project},
		{"initiative_id", input.Initiative, self.resolve.Initiative},
		{"issue_id", input.Issue, self.resolve.Issue},
		{"release_id", input.Release, self.resolve.Release},
	} {
		if p.value == nil {
			continue
		}
		id, err := p.resolve(ctx, actor, *p.value)
		if err != nil {
			return nil, err
		}
		parents = append(parents, document_repo.Parent{Column: p.column, ID: id})
	}
	switch {
	case input.Cycle != nil:
		id, err := self.resolve.Cycle(ctx, actor, team, *input.Cycle)
		if err != nil {
			return nil, err
		}
		parents = append(parents, document_repo.Parent{Column: "cycle_id", ID: id})
	case team != nil:
		parents = append(parents, document_repo.Parent{Column: "team_id", ID: *team})
	}
	return parents, nil
}

func (self *DocumentService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DocumentDetail, error) {
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return models.DocumentDetail{}, err
	}
	return self.value(ctx, nil, id)
}

func (self *DocumentService) ListVersions(ctx context.Context, actor *auth.Actor, input *IDInput) ([]models.DocumentVersionSummary, error) {
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return nil, err
	}
	versions, err := self.repo.Document().Versions(ctx, id)
	if err != nil {
		return nil, err
	}
	summaries := make([]models.DocumentVersionSummary, len(versions))
	for i, v := range versions {
		summaries[i] = models.DocumentVersionSummaryOf(v)
	}
	return summaries, nil
}

func (self *DocumentService) GetVersion(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DocumentVersion, error) {
	notFound := errdefs.NotFound("No document version %q", strings.TrimSpace(input.ID))
	id, err := strconv.ParseInt(strings.TrimSpace(input.ID), 10, 64)
	if err != nil {
		return models.DocumentVersion{}, notFound
	}
	version, err := self.repo.Document().VersionInWorkspace(ctx, id, actor.Workspace)
	if err != nil {
		return models.DocumentVersion{}, err
	}
	if version == nil {
		return models.DocumentVersion{}, notFound
	}
	visible, err := self.resolve.CanSeeDocument(ctx, actor, version.DocumentID)
	if err != nil {
		return models.DocumentVersion{}, err
	}
	if !visible {
		return models.DocumentVersion{}, notFound
	}
	return models.DocumentVersionOf(version), nil
}
