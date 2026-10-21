package search_repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issue"
	"ricrac/ent/project"
	"ricrac/ent/projectstatus"
	"ricrac/ent/user"
	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i SearchRepositoryInterface -p search_repo -s SearchRepository -o search_repository_iface.go
type SearchRepository struct {
	base *repository.BaseRepository
}

func NewSearchRepository(db *ent.Client) *SearchRepository {
	return &SearchRepository{base: repository.NewBaseRepository(db)}
}

// Params is what a search looks for and in what the caller can see.
type Params struct {
	Workspace uuid.UUID
	Teams     []uuid.UUID
	Viewer    uuid.UUID
	Team      *uuid.UUID
	Archived  bool
	PublicURL string
	Query     string
	// The issue identifier the query is, if it is one.
	Key    *string
	Number *int
	Limit  int64
}

// Hit is one match. Ref and Ref2 are, by type: the project's status, the document's project, the
// comment's issue and author.
type Hit struct {
	ID         uuid.UUID
	Title      string
	Identifier *string
	URL        string
	Snippet    string
	Rank       float32
	UpdatedAt  time.Time
	Exact      bool
	TitleMatch bool
	Ref, Ref2  *uuid.UUID
	Status     *string
}

// searchArgs starts every query, so the arguments are bound the same way for all of them.
const searchArgs = `WITH a AS (SELECT $1::uuid AS ws, $2::uuid[] AS teams, $3::uuid AS me, $4::uuid AS team,
	$5::bool AS archived, $6::text AS base, $7::text AS pattern, $8::text AS key, $9::int AS num,
	websearch_to_tsquery('english', $10) || websearch_to_tsquery('simple', $10) AS ts)`

const searchHeadline = `'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**'`

func searchSlug(name string) string {
	return fmt.Sprintf(`trim(both '-' from left(trim(both '-' from regexp_replace(lower(%s), '[^a-z0-9]+', '-', 'g')), 60))`, name)
}

func issueVisible(i string) string {
	return strings.ReplaceAll(`(I.workspace_id = a.ws AND I.trashed_at IS NULL AND (I.team_id = ANY(a.teams)
		OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = I.id AND sh.user_id = a.me)))`, "I.", i+".")
}

func projectVisible(p string) string {
	return strings.ReplaceAll(`(P.trashed_at IS NULL AND EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = P.id AND pt.team_id = ANY(a.teams)))`,
		"P.", p+".")
}

func issueSQL() string {
	return fmt.Sprintf(`SELECT i.id, i.title, t.key || '-' || i.number AS identifier,
			a.base || '/' || w.url_key || '/issue/' || t.key || '-' || i.number AS url,
			left(ts_headline('english', coalesce(nullif(i.description, ''), i.title), a.ts, %s), 200) AS snippet,
			ts_rank(i.search, a.ts) AS rank, i.updated_at, NULL::uuid AS ref, NULL::uuid AS ref2, NULL::text AS status,
			coalesce(upper(t.key) = a.key AND i.number = a.num, false) AS exact,
			(i.title ILIKE a.pattern OR to_tsvector('simple', i.title) @@ a.ts) AS title_match
		 FROM a, issues i JOIN teams t ON t.id = i.team_id JOIN workspaces w ON w.id = i.workspace_id
		 WHERE %s AND (a.archived OR i.archived_at IS NULL) AND (a.team IS NULL OR i.team_id = a.team)
		 AND (i.search @@ a.ts OR i.title ILIKE a.pattern OR (upper(t.key) = a.key AND i.number = a.num))`,
		searchHeadline, issueVisible("i"))
}

func projectSQL() string {
	return fmt.Sprintf(`SELECT p.id, p.name AS title, NULL::text AS identifier,
			a.base || '/' || w.url_key || '/project/' || %s || '-' || p.slug_id AS url,
			left(ts_headline('english', coalesce(nullif(p.content, ''), nullif(p.summary, ''), p.name), a.ts, %s), 200) AS snippet,
			ts_rank(p.search, a.ts) AS rank, p.updated_at, p.status_id AS ref, NULL::uuid AS ref2, NULL::text AS status,
			false AS exact, (p.name ILIKE a.pattern OR to_tsvector('simple', p.name) @@ a.ts) AS title_match
		 FROM a, projects p JOIN workspaces w ON w.id = p.workspace_id
		 WHERE p.workspace_id = a.ws AND %s AND (a.archived OR p.archived_at IS NULL)
		 AND (a.team IS NULL OR EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = p.id AND pt.team_id = a.team))
		 AND (p.search @@ a.ts OR p.name ILIKE a.pattern)`,
		searchSlug("p.name"), searchHeadline, projectVisible("p"))
}

func documentSQL() string {
	return fmt.Sprintf(`SELECT d.id, d.title, NULL::text AS identifier,
			a.base || '/' || w.url_key || '/document/' || %s || '-' || d.slug_id AS url,
			left(ts_headline('english', coalesce(nullif(d.content, ''), d.title), a.ts, %s), 200) AS snippet,
			ts_rank(d.search, a.ts) AS rank, d.updated_at, d.project_id AS ref, NULL::uuid AS ref2, NULL::text AS status,
			false AS exact, (d.title ILIKE a.pattern OR to_tsvector('simple', d.title) @@ a.ts) AS title_match
		 FROM a, documents d JOIN workspaces w ON w.id = d.workspace_id
		 WHERE d.workspace_id = a.ws AND d.trashed_at IS NULL
		 AND CASE WHEN d.team_id IS NOT NULL THEN d.team_id = ANY(a.teams)
			 WHEN d.project_id IS NOT NULL THEN EXISTS (SELECT 1 FROM projects p WHERE p.id = d.project_id AND %s)
			 WHEN d.issue_id IS NOT NULL THEN EXISTS (SELECT 1 FROM issues i WHERE i.id = d.issue_id AND %s)
			 WHEN d.cycle_id IS NOT NULL THEN EXISTS (SELECT 1 FROM cycles c WHERE c.id = d.cycle_id AND c.team_id = ANY(a.teams))
			 ELSE true END
		 AND (a.team IS NULL OR d.team_id = a.team
			 OR EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = d.project_id AND pt.team_id = a.team)
			 OR EXISTS (SELECT 1 FROM issues i WHERE i.id = d.issue_id AND i.team_id = a.team))
		 AND (d.search @@ a.ts OR d.title ILIKE a.pattern)`,
		searchSlug("d.title"), searchHeadline, projectVisible("p"), issueVisible("i"))
}

func initiativeSQL() string {
	vector := `(setweight(to_tsvector('simple', n.name), 'A') ||
		setweight(to_tsvector('english', coalesce(n.summary, '') || ' ' || n.content), 'B'))`
	return fmt.Sprintf(`SELECT n.id, n.name AS title, NULL::text AS identifier,
			a.base || '/' || w.url_key || '/initiative/' || %s || '-' || n.slug_id AS url,
			left(ts_headline('english', coalesce(nullif(n.content, ''), nullif(n.summary, ''), n.name), a.ts, %s), 200) AS snippet,
			ts_rank(%s, a.ts) AS rank, n.updated_at, NULL::uuid AS ref, NULL::uuid AS ref2, n.status,
			false AS exact, (n.name ILIKE a.pattern OR to_tsvector('simple', n.name) @@ a.ts) AS title_match
		 FROM a, initiatives n JOIN workspaces w ON w.id = n.workspace_id
		 WHERE n.workspace_id = a.ws AND n.trashed_at IS NULL AND (a.archived OR n.archived_at IS NULL)
		 AND (a.team IS NULL OR EXISTS (SELECT 1 FROM initiative_projects ip JOIN project_teams pt ON pt.project_id = ip.project_id
			 WHERE ip.initiative_id = n.id AND pt.team_id = a.team))
		 AND (%s @@ a.ts OR n.name ILIKE a.pattern)`,
		searchSlug("n.name"), searchHeadline, vector, vector)
}

func discussionSQL() string {
	return fmt.Sprintf(`SELECT d.id, d.title, NULL::text AS identifier,
			a.base || '/' || w.url_key || '/discussion/' || %s || '-' || d.slug_id AS url,
			left(ts_headline('english', coalesce(nullif(d.body, ''), d.title), a.ts, %s), 200) AS snippet,
			ts_rank(d.search, a.ts) AS rank, d.updated_at, NULL::uuid AS ref, NULL::uuid AS ref2, NULL::text AS status,
			false AS exact, (d.title ILIKE a.pattern OR to_tsvector('simple', d.title) @@ a.ts) AS title_match
		 FROM a, discussions d JOIN workspaces w ON w.id = d.workspace_id
		 WHERE d.workspace_id = a.ws AND d.trashed_at IS NULL AND (d.team_id IS NULL OR d.team_id = ANY(a.teams))
		 AND (a.team IS NULL OR d.team_id = a.team)
		 AND (d.search @@ a.ts OR d.title ILIKE a.pattern)`,
		searchSlug("d.title"), searchHeadline)
}

func commentSQL() string {
	return fmt.Sprintf(`SELECT c.id, i.title, t.key || '-' || i.number AS identifier,
			a.base || '/' || w.url_key || '/issue/' || t.key || '-' || i.number || '#comment-' || c.id AS url,
			left(ts_headline('english', c.body, a.ts, %s), 200) AS snippet,
			ts_rank(c.search, a.ts) AS rank, c.updated_at, i.id AS ref, c.user_id AS ref2, NULL::text AS status,
			false AS exact, false AS title_match
		 FROM a, comments c JOIN issues i ON i.id = c.issue_id JOIN teams t ON t.id = i.team_id
			JOIN workspaces w ON w.id = i.workspace_id
		 WHERE %s AND (a.archived OR i.archived_at IS NULL) AND (a.team IS NULL OR i.team_id = a.team)
		 AND (c.search @@ a.ts OR c.body ILIKE a.pattern)`,
		searchHeadline, issueVisible("i"))
}

var queries = map[string]func() string{
	"issue":      issueSQL,
	"project":    projectSQL,
	"document":   documentSQL,
	"initiative": initiativeSQL,
	"discussion": discussionSQL,
	"comment":    commentSQL,
}

// Search is the best matches of one type (issue, project, document, initiative, discussion or comment):
// exact identifiers, then title matches, then by rank.
func (self *SearchRepository) Search(ctx context.Context, kind string, params Params) ([]Hit, error) {
	query, ok := queries[kind]
	if !ok {
		return nil, fmt.Errorf("can't search %q", kind)
	}
	rows, err := self.base.DB.QueryContext(ctx, fmt.Sprintf(
		`%s SELECT id, title, identifier, url, snippet, rank, updated_at, exact, title_match, ref, ref2, status FROM (%s) s
		 ORDER BY exact DESC, title_match DESC, rank DESC LIMIT $11`, searchArgs, query()),
		params.Workspace, params.Teams, params.Viewer, params.Team, params.Archived, params.PublicURL,
		"%"+utils.EscapeLike(params.Query)+"%", params.Key, params.Number, params.Query, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var hit Hit
		var rank float64
		if err := rows.Scan(&hit.ID, &hit.Title, &hit.Identifier, &hit.URL, &hit.Snippet, &rank, &hit.UpdatedAt,
			&hit.Exact, &hit.TitleMatch, &hit.Ref, &hit.Ref2, &hit.Status); err != nil {
			return nil, err
		}
		hit.Rank = float32(rank)
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

// Issues is the issues with their team and state, by id.
func (self *SearchRepository) Issues(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*ent.Issue, error) {
	found, err := self.base.DB.Issue.Query().Where(issue.IDIn(ids...)).WithTeam().WithState().All(ctx)
	return byID(found, err, func(i *ent.Issue) uuid.UUID { return i.ID })
}

func (self *SearchRepository) ProjectStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*ent.ProjectStatus, error) {
	found, err := self.base.DB.ProjectStatus.Query().Where(projectstatus.IDIn(ids...)).All(ctx)
	return byID(found, err, func(s *ent.ProjectStatus) uuid.UUID { return s.ID })
}

func (self *SearchRepository) Projects(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*ent.Project, error) {
	found, err := self.base.DB.Project.Query().Where(project.IDIn(ids...)).All(ctx)
	return byID(found, err, func(p *ent.Project) uuid.UUID { return p.ID })
}

func (self *SearchRepository) Users(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*ent.User, error) {
	found, err := self.base.DB.User.Query().Where(user.IDIn(ids...)).All(ctx)
	return byID(found, err, func(u *ent.User) uuid.UUID { return u.ID })
}

func byID[T any](found []T, err error, id func(T) uuid.UUID) (map[uuid.UUID]T, error) {
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]T, len(found))
	for _, value := range found {
		byID[id(value)] = value
	}
	return byID, nil
}
