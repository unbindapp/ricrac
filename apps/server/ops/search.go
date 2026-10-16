package ops

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/app"
)

// Full-text search across issues, projects, documents, initiatives and comments, limited to what
// the caller can see.

func searchOps() []*Op {
	return []*Op{
		op("search", app.Read, []string{"GET /search"},
			"Search issues, projects, documents and initiatives (and comments, when asked for) by words, best matches "+
				"first, titles before text. An issue identifier like ENG-12 finds that issue. Each result has its type, title, "+
				"link and a snippet of the matching text with matches in **bold**.",
			search),
	}
}

var searchTypes = []string{"issue", "project", "document", "initiative", "comment"}

type SearchArgs struct {
	Query           string   `json:"query" doc:"Words to find; \"quoted phrases\", OR and -excluded words work too."`
	Types           []string `json:"types" doc:"What to search: issue, project, document, initiative, comment. All but comment by default."`
	Team            *string  `json:"team" doc:"Only things of this team (key, name or id)."`
	IncludeArchived bool     `json:"includeArchived,omitempty" doc:"Also archived issues, projects and initiatives."`
	Limit           *int64   `json:"limit" doc:"How many results, 20 by default, at most 50."`
}

// searchArgs starts every query, so the arguments are bound the same way for all of them.
const searchArgs = `WITH a AS (SELECT $1::uuid AS ws, $2::uuid[] AS teams, $3::uuid AS me, $4::uuid AS team,
	$5::bool AS archived, $6::text AS base, $7::text AS pattern, $8::text AS key, $9::int AS num,
	websearch_to_tsquery('english', $10) || websearch_to_tsquery('simple', $10) AS ts)`

const searchHeadline = `'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**'`

func searchSlug(name string) string {
	return fmt.Sprintf(`trim(both '-' from left(trim(both '-' from regexp_replace(lower(%s), '[^a-z0-9]+', '-', 'g')), 60))`, name)
}

func searchIssueVisible(i string) string {
	return strings.ReplaceAll(`(I.workspace_id = a.ws AND I.trashed_at IS NULL AND (I.team_id = ANY(a.teams)
		OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = I.id AND sh.user_id = a.me)))`, "I.", i+".")
}

func searchProjectVisible(p string) string {
	return strings.ReplaceAll(`(P.trashed_at IS NULL AND EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = P.id AND pt.team_id = ANY(a.teams)))`,
		"P.", p+".")
}

func searchIssueSQL() string {
	return fmt.Sprintf(`SELECT jsonb_build_object('type', 'issue', 'id', i.id, 'title', i.title, 'identifier', t.key || '-' || i.number,
			'url', a.base || '/' || w.url_key || '/issue/' || t.key || '-' || i.number,
			'snippet', left(ts_headline('english', coalesce(nullif(i.description, ''), i.title), a.ts, %s), 200),
			'rank', ts_rank(i.search, a.ts), 'state', state_ref(i.state_id), 'team', team_ref(i.team_id),
			'updatedAt', i.updated_at) AS v,
			coalesce(upper(t.key) = a.key AND i.number = a.num, false) AS exact,
			(i.title ILIKE a.pattern OR to_tsvector('simple', i.title) @@ a.ts) AS title_match
		 FROM a, issues i JOIN teams t ON t.id = i.team_id JOIN workspaces w ON w.id = i.workspace_id
		 WHERE %s AND (a.archived OR i.archived_at IS NULL) AND (a.team IS NULL OR i.team_id = a.team)
		 AND (i.search @@ a.ts OR i.title ILIKE a.pattern OR (upper(t.key) = a.key AND i.number = a.num))`,
		searchHeadline, searchIssueVisible("i"))
}

func searchProjectSQL() string {
	return fmt.Sprintf(`SELECT jsonb_build_object('type', 'project', 'id', p.id, 'title', p.name,
			'url', a.base || '/' || w.url_key || '/project/' || %s || '-' || p.slug_id,
			'snippet', left(ts_headline('english', coalesce(nullif(p.content, ''), nullif(p.summary, ''), p.name), a.ts, %s), 200),
			'rank', ts_rank(p.search, a.ts), 'status', project_status_ref(p.status_id), 'updatedAt', p.updated_at) AS v,
			false AS exact, (p.name ILIKE a.pattern OR to_tsvector('simple', p.name) @@ a.ts) AS title_match
		 FROM a, projects p JOIN workspaces w ON w.id = p.workspace_id
		 WHERE p.workspace_id = a.ws AND %s AND (a.archived OR p.archived_at IS NULL)
		 AND (a.team IS NULL OR EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = p.id AND pt.team_id = a.team))
		 AND (p.search @@ a.ts OR p.name ILIKE a.pattern)`,
		searchSlug("p.name"), searchHeadline, searchProjectVisible("p"))
}

func searchDocumentSQL() string {
	return fmt.Sprintf(`SELECT jsonb_build_object('type', 'document', 'id', d.id, 'title', d.title,
			'url', a.base || '/' || w.url_key || '/document/' || %s || '-' || d.slug_id,
			'snippet', left(ts_headline('english', coalesce(nullif(d.content, ''), d.title), a.ts, %s), 200),
			'rank', ts_rank(d.search, a.ts), 'project', project_ref(d.project_id), 'updatedAt', d.updated_at) AS v,
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
		searchSlug("d.title"), searchHeadline, searchProjectVisible("p"), searchIssueVisible("i"))
}

func searchInitiativeSQL() string {
	vector := `(setweight(to_tsvector('simple', n.name), 'A') ||
		setweight(to_tsvector('english', coalesce(n.summary, '') || ' ' || n.content), 'B'))`
	return fmt.Sprintf(`SELECT jsonb_build_object('type', 'initiative', 'id', n.id, 'title', n.name,
			'url', a.base || '/' || w.url_key || '/initiative/' || %s || '-' || n.slug_id,
			'snippet', left(ts_headline('english', coalesce(nullif(n.content, ''), nullif(n.summary, ''), n.name), a.ts, %s), 200),
			'rank', ts_rank(%s, a.ts), 'status', n.status, 'updatedAt', n.updated_at) AS v,
			false AS exact, (n.name ILIKE a.pattern OR to_tsvector('simple', n.name) @@ a.ts) AS title_match
		 FROM a, initiatives n JOIN workspaces w ON w.id = n.workspace_id
		 WHERE n.workspace_id = a.ws AND n.trashed_at IS NULL AND (a.archived OR n.archived_at IS NULL)
		 AND (a.team IS NULL OR EXISTS (SELECT 1 FROM initiative_projects ip JOIN project_teams pt ON pt.project_id = ip.project_id
			 WHERE ip.initiative_id = n.id AND pt.team_id = a.team))
		 AND (%s @@ a.ts OR n.name ILIKE a.pattern)`,
		searchSlug("n.name"), searchHeadline, vector, vector)
}

func searchCommentSQL() string {
	return fmt.Sprintf(`SELECT jsonb_build_object('type', 'comment', 'id', c.id, 'title', i.title, 'identifier', t.key || '-' || i.number,
			'url', a.base || '/' || w.url_key || '/issue/' || t.key || '-' || i.number || '#comment-' || c.id,
			'snippet', left(ts_headline('english', c.body, a.ts, %s), 200),
			'rank', ts_rank(c.search, a.ts), 'issue', issue_ref(i.id), 'user', user_ref(c.user_id), 'updatedAt', c.updated_at) AS v,
			false AS exact, false AS title_match
		 FROM a, comments c JOIN issues i ON i.id = c.issue_id JOIN teams t ON t.id = i.team_id
			JOIN workspaces w ON w.id = i.workspace_id
		 WHERE %s AND (a.archived OR i.archived_at IS NULL) AND (a.team IS NULL OR i.team_id = a.team)
		 AND (c.search @@ a.ts OR c.body ILIKE a.pattern)`,
		searchHeadline, searchIssueVisible("i"))
}

func parseSearchTypes(types []string) ([]string, error) {
	if len(types) == 0 {
		return searchTypes[:4], nil
	}
	var chosen []string
	for _, kind := range types {
		lower := strings.ToLower(strings.TrimSpace(kind))
		index := slices.IndexFunc(searchTypes, func(t string) bool { return t == lower || t+"s" == lower })
		if index < 0 {
			return nil, app.BadRequest("%q isn't a type to search; use %s", kind, strings.Join(searchTypes, ", "))
		}
		if !slices.Contains(chosen, searchTypes[index]) {
			chosen = append(chosen, searchTypes[index])
		}
	}
	return chosen, nil
}

type searchHit struct {
	Value      M
	Exact      bool
	TitleMatch bool
}

func search(c *app.Ctx, args SearchArgs) (any, error) {
	query, err := nonEmpty(args.Query, "query")
	if err != nil {
		return nil, err
	}
	types, err := parseSearchTypes(args.Types)
	if err != nil {
		return nil, err
	}
	var team *uuid.UUID
	if args.Team != nil {
		id, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		team = &id
	}
	limit := int64(20)
	if args.Limit != nil {
		limit = min(max(*args.Limit, 1), 50)
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	var key *string
	var number *int
	if k, n, ok := parseIdentifier(query); ok {
		key, number = &k, &n
	}
	var found []searchHit
	for _, kind := range types {
		sql := map[string]func() string{
			"issue":      searchIssueSQL,
			"project":    searchProjectSQL,
			"document":   searchDocumentSQL,
			"initiative": searchInitiativeSQL,
			"comment":    searchCommentSQL,
		}[kind]()
		rows, err := app.Rows[searchHit](c, c.DB(), fmt.Sprintf(
			`%s SELECT v, exact, title_match FROM (%s) s
			 ORDER BY exact DESC, title_match DESC, (v->>'rank')::float8 DESC LIMIT $11`, searchArgs, sql),
			c.Workspace, teams, c.User, team, args.IncludeArchived, c.PublicURL(), "%"+escapeLike(query)+"%",
			key, number, query, limit)
		if err != nil {
			return nil, err
		}
		found = append(found, rows...)
	}
	rank := func(v M) float64 {
		r, _ := v["rank"].(float64)
		return r
	}
	updated := func(v M) string {
		u, _ := v["updatedAt"].(string)
		return u
	}
	flag := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(found, func(a, b searchHit) int {
		return cmp.Or(
			cmp.Compare(flag(b.Exact), flag(a.Exact)),
			cmp.Compare(flag(b.TitleMatch), flag(a.TitleMatch)),
			cmp.Compare(rank(b.Value), rank(a.Value)),
			cmp.Compare(updated(b.Value), updated(a.Value)),
		)
	})
	nodes := []any{}
	for _, hit := range found[:min(len(found), int(limit))] {
		nodes = append(nodes, hit.Value)
	}
	return M{"nodes": nodes}, nil
}
