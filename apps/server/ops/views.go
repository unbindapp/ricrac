package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ricrac/app"
)

// Custom views: saved filters of issues (or projects, initiatives), personal or shared with the
// workspace, optionally for one team.

func viewOps() []*Op {
	return []*Op{
		op("list_custom_views", app.Read, []string{"GET /views"},
			"List the custom views the caller can use: those shared with the workspace and their own. Issue views "+
				"apply to list_issues with customView.",
			listViews).returns(ResultOf[PageOf[CustomView]]()),
		op("get_custom_view", app.Read, []string{"GET /views/{id}"},
			"Read a custom view by id, slug, link or name: its filter and display settings.",
			getView).returns(ResultOf[CustomView]()),
		op("save_custom_view", app.Write, []string{"POST /views", "PATCH /views/{id}"},
			"Create a custom view (omit id; name and filter are required) or update one (pass id). Issue views take "+
				"the filter of list_issues, which is checked when saving. Views are personal unless shared; only their "+
				"owner (or an admin, for shared views) can change them.",
			saveView).returns(ResultOf[CustomView]()),
		op("delete_custom_view", app.Write, []string{"DELETE /views/{id}"},
			"Delete a custom view. Only its owner (or an admin, for a shared view) can.",
			deleteView).returns(ResultOf[DeletedCustomView]()),
	}
}

const viewFrom = "FROM custom_views v JOIN workspaces w ON w.id = v.workspace_id"

// viewJSON is the JSON of the view v (with its workspace w joined).
func viewJSON(c *app.Ctx) string {
	return fmt.Sprintf(`jsonb_build_object(
		'id', v.id,
		'slugId', v.slug_id,
		'name', v.name,
		'description', v.description,
		'icon', v.icon,
		'color', v.color,
		'model', v.model,
		'filter', v.filter,
		'display', v.display,
		'shared', v.shared,
		'team', team_ref(v.team_id),
		'owner', user_ref(v.owner_id),
		'url', %s || '/' || w.url_key || '/view/' || concat_ws('-', nullif(trim(both '-' from left(
			trim(both '-' from regexp_replace(lower(v.name), '[^a-z0-9]+', '-', 'g')), 60)), ''), v.slug_id),
		'createdAt', v.created_at,
		'updatedAt', v.updated_at
	)`, sqlLiteral(c.PublicURL()))
}

func viewValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE v.id = $1", viewJSON(c), viewFrom), id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such view")
	}
	return value, nil
}

// findView is a view the caller can use, by id, slug, link or name: shared or theirs, and of a
// team they see.
func findView(c *app.Ctx, value string) (uuid.UUID, error) {
	id, err := resolveCustomView(c, value)
	if err != nil {
		return uuid.Nil, err
	}
	team, err := app.One[*uuid.UUID](c, c.DB(), "SELECT team_id FROM custom_views WHERE id = $1", id)
	if err != nil || team == nil {
		return id, err
	}
	visible, err := c.CanSeeTeam(*team)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, app.NotFound("No view %q", strings.TrimSpace(value))
	}
	return id, nil
}

type ListViewsArgs struct {
	Query  *string `json:"query" doc:"Words in the name or description."`
	Model  *string `json:"model" doc:"issue, project or initiative."`
	Team   *string `json:"team" doc:"Team key, name or id: that team's views."`
	Shared *bool   `json:"shared" doc:"true: only views shared with the workspace; false: only the caller's personal views."`
	Page
}

func listViews(c *app.Ctx, args ListViewsArgs) (any, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL(fmt.Sprintf("SELECT %s %s WHERE v.workspace_id = ", viewJSON(c), viewFrom)).Bind(c.Workspace)
	q.Push(" AND (v.shared OR v.owner_id = ").Bind(c.User)
	q.Push(") AND (v.team_id IS NULL OR v.team_id = ANY(").Bind(teams).Push("))")
	if args.Model != nil {
		model, err := checkViewModel(*args.Model)
		if err != nil {
			return nil, err
		}
		q.Push(" AND v.model = ").Bind(model)
	}
	if args.Team != nil {
		team, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		q.Push(" AND v.team_id = ").Bind(team)
	}
	if args.Shared != nil {
		if *args.Shared {
			q.Push(" AND v.shared")
		} else {
			q.Push(" AND NOT v.shared AND v.owner_id = ").Bind(c.User)
		}
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		like := "%" + escapeLike(query) + "%"
		q.Push(" AND (v.name ILIKE ").Bind(like).Push(" OR v.description ILIKE ").Bind(like).Push(")")
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY lower(v.name), v.created_at LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getView(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findView(c, args.ID)
	if err != nil {
		return nil, err
	}
	return viewValue(c, c.DB(), id)
}

func checkViewModel(model string) (string, error) {
	switch model {
	case "issue", "project", "initiative":
		return model, nil
	}
	return "", app.BadRequest("model %q isn't one of issue, project, initiative", model)
}

type SaveViewArgs struct {
	ID          *string          `json:"id" doc:"Only to update a view: its id, slug, link or name. Omit to create one."`
	Name        *string          `json:"name"`
	Description Nullable[string] `json:"description"`
	Icon        Nullable[string] `json:"icon"`
	Color       Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Model       *string          `json:"model" doc:"What the view lists: issue (default), project or initiative. Only when creating."`
	Filter      any              `json:"filter" doc:"The view's filter object. For issue views, the filter language of list_issues: Fields: id, number, title, description, priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, cycle {id,number,name,isActive}, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, contains, startsWith, endsWith, null. Example: {\"assignee\": {\"isMe\": {\"eq\": true}}, \"state\": {\"type\": {\"in\": [\"started\", \"unstarted\"]}}}"`
	Display     any              `json:"display" doc:"How to show it, e.g. {\"groupBy\": \"state\", \"orderBy\": \"priority\"}."`
	Team        Nullable[string] `json:"team" doc:"Team key, name or id the view belongs to (its issues are that team's); null makes it workspace-wide."`
	Shared      *bool            `json:"shared" doc:"Share the view with the workspace (default false: only the caller sees it)."`
}

// checkIssueFilter compiles and runs the filter, so that mistakes in it surface now rather than
// when the view is used.
func checkIssueFilter(c *app.Ctx, tx pgx.Tx, filter any) error {
	q := app.NewSQL("SELECT 1 FROM issues i WHERE FALSE")
	if err := pushFilter(q, c, "i", filter); err != nil {
		return err
	}
	q.Push(" LIMIT 0")
	err := app.Exec(c, tx, q.String(), q.Args...)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return app.BadRequest("The filter doesn't work: %s. %s", pgErr.Message, filterHelp)
	}
	return err
}

func checkViewObject(value any, what string) error {
	if _, ok := value.(M); !ok {
		return app.BadRequest("%s must be an object", what)
	}
	return nil
}

// checkCanChangeView lets only the owner change a view; admins also change shared ones.
func checkCanChangeView(c *app.Ctx, id uuid.UUID) error {
	type row struct {
		Owner  *uuid.UUID
		Shared bool
	}
	view, _, err := app.Row[row](c, c.DB(), "SELECT owner_id, shared FROM custom_views WHERE id = $1", id)
	if err != nil {
		return err
	}
	if view.Owner != nil && *view.Owner == c.User || view.Shared && c.Role >= app.Admin {
		return nil
	}
	return app.Forbidden("Only the view's owner (or an admin, for a shared view) can change it")
}

func saveView(c *app.Ctx, args SaveViewArgs) (any, error) {
	if args.Shared != nil && *args.Shared {
		if err := c.RequireMember(); err != nil {
			return nil, err
		}
	}
	if args.Display != nil {
		if err := checkViewObject(args.Display, "display"); err != nil {
			return nil, err
		}
	}
	color, err := optionalColor(args.Color)
	if err != nil {
		return nil, err
	}
	var team *uuid.UUID
	if args.Team.Valid {
		id, err := resolveTeam(c, args.Team.Value)
		if err != nil {
			return nil, err
		}
		team = &id
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		action := "create"
		if args.ID != nil {
			action = "update"
			var err error
			if id, err = findView(c, *args.ID); err != nil {
				return err
			}
			if err := checkCanChangeView(c, id); err != nil {
				return err
			}
			if args.Model != nil {
				return app.BadRequest("A view's model can't change; create another view")
			}
		} else {
			name, err := nonEmpty(orEmpty(args.Name), "name")
			if err != nil {
				return err
			}
			model := "issue"
			if args.Model != nil {
				model = *args.Model
			}
			if model, err = checkViewModel(model); err != nil {
				return err
			}
			if id, err = app.One[uuid.UUID](c, tx,
				"INSERT INTO custom_views (workspace_id, owner_id, slug_id, name, model) VALUES ($1, $2, $3, $4, $5) RETURNING id",
				c.Workspace, c.User, slugID(), name, model); err != nil {
				return err
			}
		}
		if args.Name != nil {
			name, err := nonEmpty(*args.Name, "name")
			if err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE custom_views SET name = $2 WHERE id = $1", id, name); err != nil {
				return err
			}
		}
		if args.Filter != nil {
			if err := checkViewObject(args.Filter, "filter"); err != nil {
				return err
			}
			model, err := app.One[string](c, tx, "SELECT model FROM custom_views WHERE id = $1", id)
			if err != nil {
				return err
			}
			if model == "issue" {
				if err := checkIssueFilter(c, tx, args.Filter); err != nil {
					return err
				}
			}
			if err := app.Exec(c, tx, "UPDATE custom_views SET filter = $2 WHERE id = $1", id, args.Filter); err != nil {
				return err
			}
		}
		for _, column := range []struct {
			name  string
			set   bool
			value *string
		}{
			{"description", args.Description.Set, trimmedOrNil(args.Description.Ptr())},
			{"icon", args.Icon.Set, trimmedOrNil(args.Icon.Ptr())},
			{"color", args.Color.Set, color},
		} {
			if !column.set {
				continue
			}
			if err := app.Exec(c, tx, fmt.Sprintf("UPDATE custom_views SET %s = $2 WHERE id = $1", column.name), id, column.value); err != nil {
				return err
			}
		}
		if args.Display != nil {
			if err := app.Exec(c, tx, "UPDATE custom_views SET display = $2 WHERE id = $1", id, args.Display); err != nil {
				return err
			}
		}
		if args.Team.Set {
			if err := app.Exec(c, tx, "UPDATE custom_views SET team_id = $2 WHERE id = $1", id, team); err != nil {
				return err
			}
		}
		if args.Shared != nil {
			if err := app.Exec(c, tx, "UPDATE custom_views SET shared = $2 WHERE id = $1", id, *args.Shared); err != nil {
				return err
			}
		}
		if err := app.Exec(c, tx, "UPDATE custom_views SET updated_at = now() WHERE id = $1", id); err != nil {
			return err
		}
		var err error
		if value, err = viewValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "CustomView", action, value, nil, refUUID(value, "team"))
	})
	return value, err
}

func deleteView(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findView(c, args.ID)
	if err != nil {
		return nil, err
	}
	if err := checkCanChangeView(c, id); err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if value, err = viewValue(c, tx, id); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM favorites WHERE type = 'custom_view' AND target_id = $1", id); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM custom_views WHERE id = $1", id); err != nil {
			return err
		}
		return emit(c, tx, "CustomView", "remove", value, nil, refUUID(value, "team"))
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": true, "id": id, "name": value["name"]}, nil
}

/* Shapes */

type CustomView struct {
	ID          string   `json:"id"`
	SlugID      string   `json:"slugId"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Icon        *string  `json:"icon"`
	Color       *string  `json:"color"`
	Model       string   `json:"model" enum:"issue,project,initiative"`
	Filter      M        `json:"filter"`
	Display     M        `json:"display"`
	Shared      bool     `json:"shared"`
	Team        *TeamRef `json:"team"`
	Owner       *UserRef `json:"owner"`
	URL         string   `json:"url"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type DeletedCustomView struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}
