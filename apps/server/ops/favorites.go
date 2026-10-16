package ops

import (
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Favorites: the caller's own shortcuts to issues, projects, views, documents and more, in
// optional folders.

func favoriteOps() []*Op {
	return []*Op{
		op("list_favorites", app.Read, []string{"GET /favorites"},
			"List the caller's favorites by folder and order, each with what it points at. Favorites of things that "+
				"were deleted or can no longer be seen are left out.",
			listFavorites).returns(ResultOf[PageOf[Favorite]]()),
		op("add_favorite", app.Write, []string{"POST /favorites"},
			"Add something to the caller's favorites: an issue, project, initiative, cycle, customView, document, "+
				"label, team, user or release. Adding a favorite again only updates its folder or order.",
			addFavorite).returns(ResultOf[Favorite]()),
		op("remove_favorite", app.Write, []string{"DELETE /favorites/{id}"},
			"Remove one of the caller's favorites, by the favorite's id, or by type and target.",
			removeFavorite).returns(ResultOf[RemovedFavorite]()),
	}
}

// favoriteTypes is the API's types and how they are stored.
var favoriteTypes = [][2]string{
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

func storedFavoriteType(kind string) (string, error) {
	names := make([]string, len(favoriteTypes))
	for i, t := range favoriteTypes {
		if strings.EqualFold(t[0], strings.TrimSpace(kind)) {
			return t[1], nil
		}
		names[i] = t[0]
	}
	return "", app.BadRequest("type %q isn't one of %s", kind, strings.Join(names, ", "))
}

const favoriteJSON = `jsonb_build_object(
	'id', f.id,
	'type', CASE f.type WHEN 'custom_view' THEN 'customView' ELSE f.type END,
	'folder', f.folder,
	'sortOrder', f.sort_order,
	'createdAt', f.created_at,
	'target', CASE f.type
		WHEN 'issue' THEN issue_ref(f.target_id)
		WHEN 'project' THEN project_ref(f.target_id)
		WHEN 'initiative' THEN initiative_ref(f.target_id)
		WHEN 'cycle' THEN cycle_ref(f.target_id) || jsonb_build_object('team', (SELECT team_ref(c.team_id) FROM cycles c WHERE c.id = f.target_id))
		WHEN 'custom_view' THEN (SELECT jsonb_build_object('id', v.id, 'name', v.name, 'slugId', v.slug_id) FROM custom_views v WHERE v.id = f.target_id)
		WHEN 'document' THEN document_ref(f.target_id)
		WHEN 'label' THEN label_ref(f.target_id)
		WHEN 'team' THEN team_ref(f.target_id)
		WHEN 'user' THEN user_ref(f.target_id)
		WHEN 'release' THEN release_ref(f.target_id)
	END)`

// visibleFavorites is SELECT <favorite JSON> FROM favorites f WHERE <the caller's, still there
// and visible>.
func visibleFavorites(c *app.Ctx) (*app.SQL, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL("SELECT " + favoriteJSON + " FROM favorites f WHERE f.workspace_id = ").Bind(c.Workspace)
	q.Push(" AND f.user_id = ").Bind(c.User)
	q.Push(` AND CASE f.type
		WHEN 'issue' THEN EXISTS (SELECT 1 FROM issues i WHERE i.id = f.target_id AND i.trashed_at IS NULL AND (i.team_id = ANY(`).Bind(teams)
	q.Push(") OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = i.id AND s.user_id = ").Bind(c.User)
	q.Push(`))) WHEN 'project' THEN EXISTS (SELECT 1 FROM projects p JOIN project_teams pt ON pt.project_id = p.id
			WHERE p.id = f.target_id AND p.trashed_at IS NULL AND pt.team_id = ANY(`).Bind(teams)
	q.Push(`)) WHEN 'initiative' THEN EXISTS (SELECT 1 FROM initiatives n WHERE n.id = f.target_id AND n.trashed_at IS NULL)
		WHEN 'cycle' THEN EXISTS (SELECT 1 FROM cycles c WHERE c.id = f.target_id AND c.team_id = ANY(`).Bind(teams)
	q.Push(`)) WHEN 'custom_view' THEN EXISTS (SELECT 1 FROM custom_views v WHERE v.id = f.target_id
			AND (v.shared OR v.owner_id = f.user_id) AND (v.team_id IS NULL OR v.team_id = ANY(`).Bind(teams)
	q.Push(`))) WHEN 'document' THEN EXISTS (SELECT 1 FROM documents d WHERE d.id = f.target_id AND d.trashed_at IS NULL`)
	pushDocumentVisible(q, c, teams, "d")
	q.Push(`) WHEN 'label' THEN EXISTS (SELECT 1 FROM issue_labels l WHERE l.id = f.target_id AND (l.team_id IS NULL OR l.team_id = ANY(`).Bind(teams)
	q.Push(")))  WHEN 'team' THEN f.target_id = ANY(").Bind(teams)
	q.Push(`) WHEN 'user' THEN EXISTS (SELECT 1 FROM workspace_members m WHERE m.user_id = f.target_id
			AND m.workspace_id = f.workspace_id AND m.active)
		WHEN 'release' THEN EXISTS (SELECT 1 FROM releases r WHERE r.id = f.target_id)
		ELSE FALSE END`)
	return q, nil
}

type ListFavoritesArgs struct {
	Page
}

func listFavorites(c *app.Ctx, args ListFavoritesArgs) (any, error) {
	q, err := visibleFavorites(c)
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY f.folder NULLS FIRST, f.sort_order, f.created_at LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

// favoriteTarget is the id of what target names, for a favorite of kind (as stored).
func favoriteTarget(c *app.Ctx, kind, target string, teamName *string) (uuid.UUID, error) {
	var team *uuid.UUID
	if teamName != nil {
		id, err := resolveTeam(c, *teamName)
		if err != nil {
			return uuid.Nil, err
		}
		team = &id
	}
	switch kind {
	case "issue":
		return resolveIssue(c, target)
	case "project":
		return resolveProject(c, target)
	case "initiative":
		return resolveInitiative(c, target)
	case "cycle":
		return resolveCycle(c, team, target)
	case "custom_view":
		return findView(c, target)
	case "document":
		return resolveDocument(c, target)
	case "label":
		ids, err := resolveLabels(c, team, []string{target})
		if err != nil {
			return uuid.Nil, err
		}
		return ids[0], nil
	case "team":
		return resolveTeam(c, target)
	case "user":
		return resolveUser(c, target)
	}
	return resolveRelease(c, target)
}

type AddFavoriteArgs struct {
	Type      string   `json:"type" doc:"issue, project, initiative, cycle, customView, document, label, team, user or release."`
	Target    string   `json:"target" doc:"What to favorite: its id, or the reference its type takes (ENG-123, a name, a slug, \"me\", ...)."`
	Team      *string  `json:"team" doc:"Team key, name or id, to find a cycle by number or name, or a team's label."`
	Folder    *string  `json:"folder" doc:"Folder name to put it in."`
	SortOrder *float64 `json:"sortOrder" doc:"Position among the favorites (lower is higher up); after the others by default."`
}

func addFavorite(c *app.Ctx, args AddFavoriteArgs) (any, error) {
	kind, err := storedFavoriteType(args.Type)
	if err != nil {
		return nil, err
	}
	target, err := favoriteTarget(c, kind, args.Target, args.Team)
	if err != nil {
		return nil, err
	}
	var value any
	err = inTx(c, func(tx pgx.Tx) error {
		id, err := app.One[uuid.UUID](c, tx,
			`INSERT INTO favorites (workspace_id, user_id, type, target_id, folder, sort_order)
			 VALUES ($1, $2, $3, $4, $5, coalesce($6, (SELECT coalesce(max(sort_order), 0) + 1000 FROM favorites WHERE user_id = $2 AND workspace_id = $1)))
			 ON CONFLICT (user_id, type, target_id) DO UPDATE SET
				folder = coalesce(EXCLUDED.folder, favorites.folder),
				sort_order = coalesce($6, favorites.sort_order)
			 RETURNING id`,
			c.Workspace, c.User, kind, target, trimmedOrNil(args.Folder), args.SortOrder)
		if err != nil {
			return err
		}
		value, err = app.One[any](c, tx, "SELECT "+favoriteJSON+" FROM favorites f WHERE f.id = $1", id)
		return err
	})
	return value, err
}

type RemoveFavoriteArgs struct {
	ID     *string `json:"id" doc:"The favorite's id; or, with type, what it points at."`
	Type   *string `json:"type" doc:"With target (or id): the type of what the favorite points at."`
	Target *string `json:"target" doc:"With type: what the favorite points at (id or reference)."`
	Team   *string `json:"team" doc:"Team key, name or id, to find a cycle by number or name, or a team's label."`
}

func removeFavorite(c *app.Ctx, args RemoveFavoriteArgs) (any, error) {
	if args.ID != nil && args.Type == nil {
		if id, ok := asUUID(*args.ID); ok {
			removed, found, err := app.Maybe[uuid.UUID](c, c.DB(),
				"DELETE FROM favorites WHERE id = $1 AND user_id = $2 AND workspace_id = $3 RETURNING id", id, c.User, c.Workspace)
			if err != nil {
				return nil, err
			}
			if found {
				return M{"removed": true, "id": removed}, nil
			}
		}
	}
	reference := args.Target
	if reference == nil {
		reference = args.ID
	}
	if args.Type == nil || reference == nil {
		return nil, app.NotFound("No such favorite; pass its id, or type and target")
	}
	kind, err := storedFavoriteType(*args.Type)
	if err != nil {
		return nil, err
	}
	target, err := favoriteTarget(c, kind, *reference, args.Team)
	if err != nil {
		return nil, err
	}
	removed, found, err := app.Maybe[uuid.UUID](c, c.DB(),
		"DELETE FROM favorites WHERE user_id = $1 AND workspace_id = $2 AND type = $3 AND target_id = $4 RETURNING id",
		c.User, c.Workspace, kind, target)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("That isn't one of your favorites")
	}
	return M{"removed": true, "id": removed}, nil
}

/* Shapes */

type Favorite struct {
	ID        string         `json:"id"`
	Type      string         `json:"type" enum:"issue,project,initiative,cycle,customView,document,label,team,user,release"`
	Folder    *string        `json:"folder"`
	SortOrder float64        `json:"sortOrder"`
	CreatedAt string         `json:"createdAt"`
	Target    FavoriteTarget `json:"target" doc:"What it points at, in the short form of its type."`
}

// FavoriteTarget is the short form of whatever a favorite points at.
type FavoriteTarget struct{}

func (FavoriteTarget) JSONSchema() M {
	return M{"anyOf": []any{
		ResultOf[IssueRef](), ResultOf[ProjectRef](), ResultOf[InitiativeRef](), ResultOf[FavoriteCycle](),
		ResultOf[FavoriteCustomView](), ResultOf[DocumentRef](), ResultOf[LabelRef](), ResultOf[TeamRef](),
		ResultOf[UserRef](), ResultOf[ReleaseRef](),
	}}
}

type FavoriteCycle struct {
	CycleRef
	Team TeamRef `json:"team"`
}

type FavoriteCustomView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type RemovedFavorite struct {
	Removed bool   `json:"removed"`
	ID      string `json:"id"`
}
