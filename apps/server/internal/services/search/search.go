// Package search_service is full-text search across issues, projects, documents, initiatives,
// discussions and comments, limited to what the caller can see.
package search_service

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	"ricrac/internal/repositories/repositories"
	search_repo "ricrac/internal/repositories/search"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i SearchServiceInterface -p search_service -s SearchService -o search_service_iface.go
type SearchService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
}

func NewSearchService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface) *SearchService {
	return &SearchService{cfg: cfg, repo: repo, resolve: resolve}
}

var searchTypes = []string{"issue", "project", "document", "initiative", "discussion", "comment"}

type SearchInput struct {
	Query           string   `json:"query" doc:"Words to find; \"quoted phrases\", OR and -excluded words work too."`
	Types           []string `json:"types" doc:"What to search: issue, project, document, initiative, discussion, comment. All but comment by default."`
	Team            *string  `json:"team" doc:"Only things of this team (key, name or id)."`
	IncludeArchived bool     `json:"includeArchived,omitempty" doc:"Also archived issues, projects and initiatives."`
	Limit           *int64   `json:"limit" doc:"How many results, 20 by default, at most 50."`
}

func parseTypes(types []string) ([]string, error) {
	if len(types) == 0 {
		return searchTypes[:5], nil
	}
	var chosen []string
	for _, kind := range types {
		lower := strings.ToLower(strings.TrimSpace(kind))
		index := slices.IndexFunc(searchTypes, func(t string) bool { return t == lower || t+"s" == lower })
		if index < 0 {
			return nil, errdefs.BadRequest("%q isn't a type to search; use %s", kind, strings.Join(searchTypes, ", "))
		}
		if !slices.Contains(chosen, searchTypes[index]) {
			chosen = append(chosen, searchTypes[index])
		}
	}
	return chosen, nil
}

// found is a hit as results are sorted: exact identifiers, title matches, rank, the most recently
// updated.
type found struct {
	kind       string
	hit        search_repo.Hit
	rank       float64
	updatedAt  string
	exact      bool
	titleMatch bool
}

// jsonRank is a ts_rank as Postgres writes a real in JSON and reads it back as a double.
func jsonRank(rank float32) float64 {
	value, _ := strconv.ParseFloat(strconv.FormatFloat(float64(rank), 'g', -1, 32), 64)
	return value
}

func flag(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (self *SearchService) Search(ctx context.Context, actor *auth.Actor, input *SearchInput) (models.SearchResults, error) {
	query, err := utils.NonEmpty(input.Query, "query")
	if err != nil {
		return models.SearchResults{}, err
	}
	types, err := parseTypes(input.Types)
	if err != nil {
		return models.SearchResults{}, err
	}
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return models.SearchResults{}, err
	}
	limit := int64(20)
	if input.Limit != nil {
		limit = min(max(*input.Limit, 1), 50)
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.SearchResults{}, err
	}
	params := search_repo.Params{
		Workspace: actor.Workspace, Teams: teams, Viewer: actor.User, Team: team, Archived: input.IncludeArchived,
		PublicURL: self.cfg.PublicURL, Query: query, Limit: limit,
	}
	if key, number, ok := utils.ParseIdentifier(query); ok {
		params.Key, params.Number = &key, &number
	}
	var hits []found
	for _, kind := range types {
		rows, err := self.repo.Search().Search(ctx, kind, params)
		if err != nil {
			return models.SearchResults{}, err
		}
		for _, row := range rows {
			hits = append(hits, found{kind: kind, hit: row, rank: jsonRank(row.Rank), updatedAt: models.Time(row.UpdatedAt), exact: row.Exact, titleMatch: row.TitleMatch})
		}
	}
	slices.SortStableFunc(hits, func(a, b found) int {
		return cmp.Or(
			cmp.Compare(flag(b.exact), flag(a.exact)),
			cmp.Compare(flag(b.titleMatch), flag(a.titleMatch)),
			cmp.Compare(b.rank, a.rank),
			cmp.Compare(b.updatedAt, a.updatedAt),
		)
	})
	return self.results(ctx, hits[:min(len(hits), int(limit))])
}

// results loads what the hits point at and shows them.
func (self *SearchService) results(ctx context.Context, hits []found) (models.SearchResults, error) {
	var issueIDs, statusIDs, projectIDs, userIDs []uuid.UUID
	for _, h := range hits {
		switch h.kind {
		case "issue":
			issueIDs = append(issueIDs, h.hit.ID)
		case "project":
			statusIDs = append(statusIDs, *h.hit.Ref)
		case "document":
			if h.hit.Ref != nil {
				projectIDs = append(projectIDs, *h.hit.Ref)
			}
		case "comment":
			issueIDs = append(issueIDs, *h.hit.Ref)
			if h.hit.Ref2 != nil {
				userIDs = append(userIDs, *h.hit.Ref2)
			}
		}
	}
	issues, err := self.repo.Search().Issues(ctx, issueIDs)
	if err != nil {
		return models.SearchResults{}, err
	}
	statuses, err := self.repo.Search().ProjectStatuses(ctx, statusIDs)
	if err != nil {
		return models.SearchResults{}, err
	}
	projects, err := self.repo.Search().Projects(ctx, projectIDs)
	if err != nil {
		return models.SearchResults{}, err
	}
	users, err := self.repo.Search().Users(ctx, userIDs)
	if err != nil {
		return models.SearchResults{}, err
	}
	nodes := make([]models.SearchHit, len(hits))
	for i, h := range hits {
		row := h.hit
		base := models.SearchHitBase{
			ID: models.ID(row.ID), Title: row.Title, URL: row.URL, Snippet: row.Snippet, Rank: h.rank, UpdatedAt: h.updatedAt,
		}
		var value any
		switch h.kind {
		case "issue":
			issue := issues[row.ID]
			value = models.IssueSearchHit{
				Type: "issue", SearchHitBase: base, Identifier: utils.OrEmpty(row.Identifier),
				State: *models.StateRefOf(issue.Edges.State), Team: *models.TeamRefOf(issue.Edges.Team),
			}
		case "project":
			value = models.ProjectSearchHit{Type: "project", SearchHitBase: base, Status: *models.ProjectStatusRefOf(statuses[*row.Ref])}
		case "document":
			hit := models.DocumentSearchHit{Type: "document", SearchHitBase: base}
			if row.Ref != nil {
				hit.Project = models.ProjectRefOf(projects[*row.Ref])
			}
			value = hit
		case "initiative":
			value = models.InitiativeSearchHit{Type: "initiative", SearchHitBase: base, Status: utils.OrEmpty(row.Status)}
		case "discussion":
			value = models.DiscussionSearchHit{Type: "discussion", SearchHitBase: base}
		case "comment":
			hit := models.CommentSearchHit{
				Type: "comment", SearchHitBase: base, Identifier: utils.OrEmpty(row.Identifier), Issue: *models.IssueRefOf(issues[*row.Ref]),
			}
			if row.Ref2 != nil {
				hit.User = models.UserRefOf(users[*row.Ref2])
			}
			value = hit
		}
		nodes[i] = models.SearchHitOf(value)
	}
	return models.SearchResults{Nodes: nodes}, nil
}
