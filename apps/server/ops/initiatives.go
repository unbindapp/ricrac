package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Initiatives: company goals that group projects (and other initiatives), with their progress,
// health and status. Guests can't see them.

func initiativeOps() []*Op {
	return []*Op{
		op("list_initiatives", app.Read, []string{"GET /initiatives"},
			"List initiatives, most recently updated first, each with its project count and progress (the average of "+
				"its projects'). Narrow with status (planned, active, completed), owner, parent or query. Archived ones are "+
				"left out unless includeArchived is set.",
			listInitiatives).returns(ResultOf[PageOf[Initiative]]()),
		op("get_initiative", app.Read, []string{"GET /initiatives/{id}"},
			"Read an initiative by id, slug, link or name: its fields and description, projects (with status, "+
				"progress, health and lead), sub-initiatives, latest status update, links and documents.",
			getInitiative).returns(ResultOf[InitiativeDetail]()),
		op("save_initiative", app.Write, []string{"POST /initiatives", "PATCH /initiatives/{id}"},
			"Create an initiative (omit id; name is required) or update one (pass id). Omitted fields stay as they are; "+
				"null clears a field. Add projects with addProjects or setProjects; nest it under another initiative with "+
				"parent.",
			saveInitiative).returns(ResultOf[Initiative]()),
		op("delete_initiative", app.Write, []string{"DELETE /initiatives/{id}"},
			"Move an initiative to the trash; its projects stay. It is deleted for good after 30 days; "+
				"restore_initiative brings it back before then.",
			trashInitiative).returns(ResultOf[Initiative]()),
		op("restore_initiative", app.Write, []string{"POST /initiatives/{id}/restore"},
			"Bring an initiative back from the trash.", restoreInitiative).returns(ResultOf[Initiative]()),
		op("archive_initiative", app.Write, []string{"POST /initiatives/{id}/archive"},
			"Archive an initiative. It stays readable but is left out of lists unless includeArchived is set.", archiveInitiative).returns(ResultOf[Initiative]()),
		op("unarchive_initiative", app.Write, []string{"POST /initiatives/{id}/unarchive"},
			"Bring an archived initiative back.", unarchiveInitiative).returns(ResultOf[Initiative]()),
	}
}

const initiativeFrom = "FROM initiatives i JOIN workspaces w ON w.id = i.workspace_id"

// initiativeJSON is the JSON of the initiative i (with its workspace w joined).
func initiativeJSON(c *app.Ctx, withDescription bool) string {
	description := ""
	if withDescription {
		description = "'description', i.content,"
	}
	return fmt.Sprintf(`jsonb_build_object(
		'id', i.id,
		'slugId', i.slug_id,
		'name', i.name,
		'summary', i.summary,
		%s
		'url', %s || '/' || w.url_key || '/initiative/' || concat_ws('-', nullif(entity_slug(i.name), ''), i.slug_id),
		'icon', i.icon,
		'color', i.color,
		'status', i.status,
		'health', i.health,
		'owner', user_ref(i.owner_id),
		'creator', user_ref(i.creator_id),
		'targetDate', i.target_date,
		'targetDateResolution', i.target_date_resolution,
		'parent', initiative_ref(i.parent_id),
		'projectCount', (SELECT count(*) FROM initiative_projects ip JOIN projects ipp ON ipp.id = ip.project_id
			WHERE ip.initiative_id = i.id AND ipp.trashed_at IS NULL),
		'progress', initiative_progress(i.id, %s),
		'sortOrder', i.sort_order,
		'startedAt', i.started_at,
		'completedAt', i.completed_at,
		'archivedAt', i.archived_at,
		'trashed', i.trashed_at IS NOT NULL,
		'trashedAt', i.trashed_at,
		'createdAt', i.created_at,
		'updatedAt', i.updated_at
	)`, description, sqlLiteral(c.PublicURL()), viewer(c))
}

func initiativeValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE i.id = $1", initiativeJSON(c, true), initiativeFrom), id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such initiative")
	}
	return value, nil
}

/* List, read */

type ListInitiativesArgs struct {
	Status             *string `json:"status" doc:"planned, active or completed."`
	Owner              *string `json:"owner" doc:"Owner (id, name, email or \"me\")."`
	Parent             *string `json:"parent" doc:"Sub-initiatives of this initiative (name, slug or id), or \"null\" for top-level ones."`
	Query              *string `json:"query" doc:"Words in the name, summary or description."`
	IncludeArchived    bool    `json:"includeArchived,omitempty"`
	IncludeTrashed     bool    `json:"includeTrashed,omitempty"`
	IncludeDescription bool    `json:"includeDescription,omitempty" doc:"Include each initiative's description."`
	Limit              *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor             *string `json:"cursor" doc:"The endCursor of the previous page."`
}

func listInitiatives(c *app.Ctx, args ListInitiativesArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	q := app.NewSQL(fmt.Sprintf("SELECT %s %s WHERE i.workspace_id = ", initiativeJSON(c, args.IncludeDescription), initiativeFrom)).Bind(c.Workspace)
	if !args.IncludeArchived {
		q.Push(" AND i.archived_at IS NULL")
	}
	if !args.IncludeTrashed {
		q.Push(" AND i.trashed_at IS NULL")
	}
	if args.Status != nil {
		status, err := checkInitiativeStatus(*args.Status)
		if err != nil {
			return nil, err
		}
		q.Push(" AND i.status = ").Bind(status)
	}
	if args.Owner != nil {
		owner, err := resolveUser(c, *args.Owner)
		if err != nil {
			return nil, err
		}
		q.Push(" AND i.owner_id = ").Bind(owner)
	}
	if args.Parent != nil {
		if isNoneWord(*args.Parent) {
			q.Push(" AND i.parent_id IS NULL")
		} else {
			parent, err := resolveInitiative(c, *args.Parent)
			if err != nil {
				return nil, err
			}
			q.Push(" AND i.parent_id = ").Bind(parent)
		}
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		like := "%" + escapeLike(query) + "%"
		q.Push(" AND (i.name ILIKE ").Bind(like).Push(" OR i.summary ILIKE ").Bind(like).Push(" OR i.content ILIKE ").Bind(like).Push(")")
	}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY i.updated_at DESC LIMIT ").Bind(page.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

func getInitiative(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveInitiative(c, args.ID)
	if err != nil {
		return nil, err
	}
	value, err := initiativeValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	more, err := app.One[M](c, c.DB(), fmt.Sprintf(`SELECT jsonb_build_object(
		'projects', (SELECT coalesce(jsonb_agg(project_ref(p.id) || jsonb_build_object(
				'status', project_status_ref(p.status_id), 'health', p.health, 'lead', user_ref(p.lead_id),
				'targetDate', p.target_date, 'progress', project_issue_stats(p.id, NULL, %[3]s)->'progress')
			ORDER BY ip.sort_order, p.name), '[]')
			FROM initiative_projects ip JOIN projects p ON p.id = ip.project_id
			WHERE ip.initiative_id = $1 AND p.trashed_at IS NULL AND %[1]s),
		'subInitiatives', (SELECT coalesce(jsonb_agg(initiative_ref(c.id) || jsonb_build_object(
				'status', c.status, 'health', c.health, 'owner', user_ref(c.owner_id),
				'progress', initiative_progress(c.id, %[3]s)) ORDER BY c.sort_order, c.name), '[]')
			FROM initiatives c WHERE c.parent_id = $1 AND c.trashed_at IS NULL),
		'links', %[2]s,
		'documents', (SELECT coalesce(jsonb_agg(document_ref(d.id) ORDER BY d.sort_order, d.created_at), '[]')
			FROM documents d WHERE d.initiative_id = $1 AND d.trashed_at IS NULL)
	)`, projectVisible("p", 2), linksSQL("initiative_id"), viewer(c)), id, teams)
	if err != nil {
		return nil, err
	}
	for key, v := range more {
		value[key] = v
	}
	if value["latestUpdate"], err = latestStatusUpdate(c, c.DB(), "initiative_id", id); err != nil {
		return nil, err
	}
	return value, nil
}

/* Save */

type SaveInitiativeArgs struct {
	ID                   *string          `json:"id" doc:"Only to update an initiative: its id, slug, link or name. Omit to create one."`
	Name                 *string          `json:"name" doc:"Required to create."`
	Summary              Nullable[string] `json:"summary" doc:"A one-line summary, at most 255 characters; null clears it."`
	Description          *string          `json:"description" doc:"Markdown."`
	Patch                []TextEdit       `json:"patch" doc:"Edits to the current description instead of the whole text, applied in order, all or none."`
	Icon                 Nullable[string] `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color                Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Status               *string          `json:"status" doc:"planned (the default), active or completed."`
	Owner                Nullable[string] `json:"owner" doc:"Person (id, name, email or \"me\"); null removes the owner."`
	TargetDate           Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	TargetDateResolution Nullable[string] `json:"targetDateResolution" doc:"How precise targetDate is: month, quarter, halfYear or year; null means the exact day."`
	Parent               Nullable[string] `json:"parent" doc:"The initiative (name, slug or id) this one is part of; null makes it top-level."`
	AddProjects          []string         `json:"addProjects" doc:"Projects (names, slugs or ids) to add."`
	RemoveProjects       []string         `json:"removeProjects"`
	SetProjects          []string         `json:"setProjects" doc:"Replace all projects with these."`
	Links                []LinkArg        `json:"links" doc:"Links to add ({url, title}); a link already on the initiative gets the new title."`
	RemoveLinks          []string         `json:"removeLinks" doc:"URLs of links to remove."`
	SortOrder            *float64         `json:"sortOrder" doc:"Position in manually ordered lists."`
}

func checkInitiativeStatus(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !slices.Contains([]string{"planned", "active", "completed"}, lower) {
		return "", app.BadRequest("An initiative's status is planned, active or completed, not \"%s\"", value)
	}
	return lower, nil
}

func saveInitiative(c *app.Ctx, args SaveInitiativeArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := resolveInitiative(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var before M
		var err error
		if existing != nil {
			id = *existing
			if before, err = initiativeValue(c, tx, id); err != nil {
				return err
			}
		} else {
			name, err := nonEmpty(orEmpty(args.Name), "name")
			if err != nil {
				return err
			}
			if id, err = app.One[uuid.UUID](c, tx,
				`INSERT INTO initiatives (workspace_id, slug_id, name, creator_id, sort_order)
				 VALUES ($1, $2, $3, $4, (SELECT coalesce(max(sort_order), 0) + 1000 FROM initiatives WHERE workspace_id = $1))
				 RETURNING id`, c.Workspace, slugID(), name, c.User); err != nil {
				return err
			}
		}
		if err := applyInitiativeChanges(c, tx, id, args); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET updated_at = now() WHERE id = $1", id); err != nil {
			return err
		}
		if value, err = initiativeValue(c, tx, id); err != nil {
			return err
		}
		if before == nil {
			return emit(c, tx, "Initiative", "create", value, nil, nil)
		}
		updatedFrom := changedFields(before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		return emit(c, tx, "Initiative", "update", value, updatedFrom, nil)
	})
	return value, err
}

func applyInitiativeChanges(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveInitiativeArgs) error {
	content, err := app.One[string](c, tx, "SELECT content FROM initiatives WHERE id = $1 FOR UPDATE", id)
	if err != nil {
		return err
	}
	if args.Name != nil {
		name, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET name = $2 WHERE id = $1", id, name); err != nil {
			return err
		}
	}
	if args.Summary.Set {
		var summary *string
		if trimmed := strings.TrimSpace(args.Summary.Value); args.Summary.Valid && trimmed != "" {
			summary = &trimmed
		}
		if summary != nil && len([]rune(*summary)) > 255 {
			return app.BadRequest("summary is at most 255 characters; put the rest in the description")
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET summary = $2 WHERE id = $1", id, summary); err != nil {
			return err
		}
	}
	updated, err := newContent(content, args.Description, args.Patch)
	if err != nil {
		return err
	}
	if updated != nil {
		if err := app.Exec(c, tx, "UPDATE initiatives SET content = $2 WHERE id = $1", id, *updated); err != nil {
			return err
		}
	}
	if args.Icon.Set {
		var icon *string
		if args.Icon.Valid && strings.TrimSpace(args.Icon.Value) != "" {
			icon = &args.Icon.Value
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET icon = $2 WHERE id = $1", id, icon); err != nil {
			return err
		}
	}
	if args.Color.Set {
		var color *string
		if args.Color.Valid {
			checked, err := checkColor(args.Color.Value)
			if err != nil {
				return err
			}
			color = &checked
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET color = $2 WHERE id = $1", id, color); err != nil {
			return err
		}
	}
	if args.Status != nil {
		status, err := checkInitiativeStatus(*args.Status)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			`UPDATE initiatives SET status = $2,
				started_at = CASE WHEN $2 = 'planned' THEN NULL WHEN $2 = 'active' THEN coalesce(started_at, now())
								  ELSE started_at END,
				completed_at = CASE WHEN $2 = 'completed' THEN coalesce(completed_at, now()) END
			 WHERE id = $1`, id, status); err != nil {
			return err
		}
	}
	if args.Owner.Set {
		owner, err := optionalRef(c, args.Owner, resolveUser)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET owner_id = $2 WHERE id = $1", id, owner); err != nil {
			return err
		}
	}
	if args.TargetDate.Set {
		date, err := optionalDate(args.TargetDate)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			`UPDATE initiatives SET target_date = $2,
				target_date_resolution = CASE WHEN $2::date IS NULL THEN NULL ELSE target_date_resolution END
			 WHERE id = $1`, id, date); err != nil {
			return err
		}
	}
	if args.TargetDateResolution.Set {
		resolution, err := checkResolution(args.TargetDateResolution)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE initiatives SET target_date_resolution = CASE WHEN target_date IS NULL THEN NULL ELSE $2 END WHERE id = $1", id, resolution); err != nil {
			return err
		}
	}
	if args.Parent.Set {
		if err := initiativeParent(c, tx, id, args.Parent); err != nil {
			return err
		}
	}
	if err := initiativeProjects(c, tx, id, args.SetProjects, args.AddProjects, args.RemoveProjects); err != nil {
		return err
	}
	if err := saveLinks(c, tx, "initiative_id", id, args.Links, args.RemoveLinks); err != nil {
		return err
	}
	if args.SortOrder != nil {
		return app.Exec(c, tx, "UPDATE initiatives SET sort_order = $2 WHERE id = $1", id, *args.SortOrder)
	}
	return nil
}

func initiativeParent(c *app.Ctx, tx pgx.Tx, id uuid.UUID, value Nullable[string]) error {
	parent, err := optionalRef(c, value, resolveInitiative)
	if err != nil {
		return err
	}
	if parent != nil {
		loops, err := app.One[bool](c, tx,
			`WITH RECURSIVE up AS (SELECT id, parent_id FROM initiatives WHERE id = $1
			 UNION SELECT i.id, i.parent_id FROM initiatives i JOIN up ON i.id = up.parent_id)
			 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, *parent, id)
		if err != nil {
			return err
		}
		if loops {
			return app.BadRequest("An initiative can't be part of itself or of its own sub-initiatives")
		}
	}
	return app.Exec(c, tx, "UPDATE initiatives SET parent_id = $2 WHERE id = $1", id, parent)
}

func initiativeProjects(c *app.Ctx, tx pgx.Tx, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return app.BadRequest("Pass either setProjects or addProjects/removeProjects, not both")
	}
	link := func(projects []uuid.UUID) error {
		for _, project := range projects {
			if err := linkInitiatives(c, tx, project, []uuid.UUID{id}); err != nil {
				return err
			}
		}
		return nil
	}
	if set != nil {
		ids, err := resolveProjects(c, set)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM initiative_projects WHERE initiative_id = $1 AND NOT (project_id = ANY($2))", id, ids); err != nil {
			return err
		}
		if err := link(ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := resolveProjects(c, add)
		if err != nil {
			return err
		}
		if err := link(ids); err != nil {
			return err
		}
	}
	if remove == nil {
		return nil
	}
	ids, err := resolveProjects(c, remove)
	if err != nil {
		return err
	}
	return app.Exec(c, tx, "DELETE FROM initiative_projects WHERE initiative_id = $1 AND project_id = ANY($2)", id, ids)
}

/* Trash and archive */

func markInitiative(c *app.Ctx, id uuid.UUID, sql, action string) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, sql, id); err != nil {
			return err
		}
		var err error
		if value, err = initiativeValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Initiative", action, value, nil, nil)
	})
	return value, err
}

func trashInitiative(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveInitiative(c, args.ID)
	if err != nil {
		return nil, err
	}
	return markInitiative(c, id, "UPDATE initiatives SET trashed_at = now(), updated_at = now() WHERE id = $1 AND trashed_at IS NULL", "remove")
}

func restoreInitiative(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := findTrashed(c, "initiatives", "initiative", args.ID)
	if err != nil {
		return nil, err
	}
	return markInitiative(c, id, "UPDATE initiatives SET trashed_at = NULL, updated_at = now() WHERE id = $1", "update")
}

func archiveInitiative(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveInitiative(c, args.ID)
	if err != nil {
		return nil, err
	}
	return markInitiative(c, id, "UPDATE initiatives SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL", "update")
}

func unarchiveInitiative(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveInitiative(c, args.ID)
	if err != nil {
		return nil, err
	}
	return markInitiative(c, id, "UPDATE initiatives SET archived_at = NULL, updated_at = now() WHERE id = $1", "update")
}

/* Result shapes */

type Initiative struct {
	ID                   string         `json:"id"`
	SlugID               string         `json:"slugId"`
	Name                 string         `json:"name"`
	Summary              *string        `json:"summary"`
	Description          string         `json:"description,omitempty" doc:"Markdown; in lists only with includeDescription."`
	URL                  string         `json:"url"`
	Icon                 *string        `json:"icon"`
	Color                *string        `json:"color"`
	Status               string         `json:"status" enum:"planned,active,completed"`
	Health               *string        `json:"health" enum:"onTrack,atRisk,offTrack"`
	Owner                *UserRef       `json:"owner"`
	Creator              *UserRef       `json:"creator"`
	TargetDate           *string        `json:"targetDate"`
	TargetDateResolution *string        `json:"targetDateResolution" enum:"month,quarter,halfYear,year"`
	Parent               *InitiativeRef `json:"parent"`
	ProjectCount         int            `json:"projectCount"`
	Progress             float64        `json:"progress" doc:"The average progress of its projects, from 0 to 1."`
	SortOrder            float64        `json:"sortOrder"`
	StartedAt            *string        `json:"startedAt"`
	CompletedAt          *string        `json:"completedAt"`
	ArchivedAt           *string        `json:"archivedAt"`
	Trashed              bool           `json:"trashed"`
	TrashedAt            *string        `json:"trashedAt"`
	CreatedAt            string         `json:"createdAt"`
	UpdatedAt            string         `json:"updatedAt"`
}

type InitiativeDetail struct {
	Initiative
	Projects       []InitiativeProject `json:"projects"`
	SubInitiatives []SubInitiative     `json:"subInitiatives"`
	Links          []EntityLink        `json:"links"`
	Documents      []DocumentRef       `json:"documents"`
	LatestUpdate   *StatusUpdate       `json:"latestUpdate"`
}

type InitiativeProject struct {
	ProjectRef
	Status     ProjectStatusRef `json:"status"`
	Health     *string          `json:"health" enum:"onTrack,atRisk,offTrack"`
	Lead       *UserRef         `json:"lead"`
	TargetDate *string          `json:"targetDate"`
	Progress   float64          `json:"progress"`
}

type SubInitiative struct {
	InitiativeRef
	Status   string   `json:"status" enum:"planned,active,completed"`
	Health   *string  `json:"health" enum:"onTrack,atRisk,offTrack"`
	Owner    *UserRef `json:"owner"`
	Progress float64  `json:"progress"`
}
