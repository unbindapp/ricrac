package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Status updates on projects and initiatives: a written update with a health (on track, at risk,
// off track). The newest update's health is the project's or initiative's health.

func statusUpdateOps() []*Op {
	return []*Op{
		op("get_status_updates", app.Read, []string{"GET /status-updates"},
			"Status updates, newest first: of one project or initiative, or, without either, the recent updates across "+
				"the projects you can see and the initiatives. Pass id to read one update.",
			listStatusUpdates).returns(ResultOf[PageOf[StatusUpdate]]()),
		op("save_status_update", app.Write, []string{"POST /status-updates", "PATCH /status-updates/{id}"},
			"Post a status update on a project or initiative (omit id; body is required) with its health: onTrack, "+
				"atRisk or offTrack (by default the previous update's, else onTrack). The newest update's health becomes "+
				"the project's or initiative's. Members and subscribers of the project (or the initiative's owner) are "+
				"notified, and so are people @mentioned. Pass id to edit your own update.",
			saveStatusUpdate).returns(ResultOf[StatusUpdate]()),
		op("delete_status_update", app.Write, []string{"DELETE /status-updates/{id}"},
			"Delete a status update (yours, or any as a workspace admin). The health goes back to the newest remaining "+
				"update's.",
			deleteStatusUpdate).returns(ResultOf[DeletedStatusUpdate]()),
	}
}

// statusUpdateJSON is the JSON of the status update u.
const statusUpdateJSON = `jsonb_build_object('id', u.id,
	'type', CASE WHEN u.project_id IS NOT NULL THEN 'project' ELSE 'initiative' END,
	'project', project_ref(u.project_id), 'initiative', initiative_ref(u.initiative_id),
	'body', u.body, 'health', u.health, 'user', user_ref(u.user_id),
	'reactions', (SELECT coalesce(jsonb_agg(jsonb_build_object('emoji', r.emoji, 'user', user_ref(r.user_id))
		ORDER BY r.created_at), '[]') FROM reactions r WHERE r.status_update_id = u.id),
	'commentCount', (SELECT count(*) FROM comments c WHERE c.status_update_id = u.id),
	'createdAt', u.created_at, 'updatedAt', u.updated_at, 'editedAt', u.edited_at)`

// latestStatusUpdate is the newest update on the thing whose id is in column (project_id or
// initiative_id), or nil.
func latestStatusUpdate(c *app.Ctx, q app.Querier, column string, id uuid.UUID) (any, error) {
	value, _, err := app.Maybe[any](c, q, fmt.Sprintf(
		"SELECT %s FROM status_updates u WHERE u.%s = $1 ORDER BY u.created_at DESC, u.id LIMIT 1", statusUpdateJSON, column), id)
	return value, err
}

func statusUpdateValue(c *app.Ctx, q app.Querier, id uuid.UUID) (any, error) {
	return app.One[any](c, q, "SELECT "+statusUpdateJSON+" FROM status_updates u WHERE u.id = $1", id)
}

// syncHealth copies the newest update's health onto the project or initiative (null when none is
// left).
func syncHealth(c *app.Ctx, tx pgx.Tx, t updateTarget) error {
	table, column, id := "projects", "project_id", t.project
	if id == nil {
		table, column, id = "initiatives", "initiative_id", t.initiative
	}
	if id == nil {
		return nil
	}
	return app.Exec(c, tx, fmt.Sprintf(
		`UPDATE %s SET health = (SELECT health FROM status_updates WHERE %s = $1
			ORDER BY created_at DESC, id LIMIT 1) WHERE id = $1`, table, column), *id)
}

/* List */

type ListStatusUpdatesArgs struct {
	ID         *string `json:"id" doc:"One status update by id."`
	Project    *string `json:"project" doc:"Project name, slug, link or id."`
	Initiative *string `json:"initiative" doc:"Initiative name, slug, link or id."`
	Type       *string `json:"type" doc:"project or initiative updates only."`
	User       *string `json:"user" doc:"Author (id, name, email or \"me\")."`
	CreatedAt  *string `json:"createdAt" doc:"Created after: ISO 8601 date/time or a duration like -P1W."`
	Limit      *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor     *string `json:"cursor" doc:"The endCursor of the previous page."`
}

func listStatusUpdates(c *app.Ctx, args ListStatusUpdatesArgs) (any, error) {
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL("SELECT " + statusUpdateJSON + " FROM status_updates u WHERE u.workspace_id = ").Bind(c.Workspace)
	q.Push(` AND ((u.project_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = u.project_id
		AND p.trashed_at IS NULL AND EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = p.id AND pt.team_id = ANY(`).Bind(teams)
	q.Push(")))) OR (u.initiative_id IS NOT NULL AND ").Bind(c.Role != app.Guest)
	q.Push(" AND EXISTS (SELECT 1 FROM initiatives i WHERE i.id = u.initiative_id AND i.trashed_at IS NULL)))")
	if args.ID != nil {
		id, ok := asUUID(*args.ID)
		if !ok {
			return nil, app.NotFound("No status update \"%s\"", strings.TrimSpace(*args.ID))
		}
		q.Push(" AND u.id = ").Bind(id)
	}
	if args.Project != nil {
		project, err := resolveProject(c, *args.Project)
		if err != nil {
			return nil, err
		}
		q.Push(" AND u.project_id = ").Bind(project)
	}
	if args.Initiative != nil {
		initiative, err := resolveInitiative(c, *args.Initiative)
		if err != nil {
			return nil, err
		}
		q.Push(" AND u.initiative_id = ").Bind(initiative)
	}
	if args.Type != nil {
		switch *args.Type {
		case "project":
			q.Push(" AND u.project_id IS NOT NULL")
		case "initiative":
			q.Push(" AND u.initiative_id IS NOT NULL")
		default:
			return nil, app.BadRequest("type is project or initiative, not \"%s\"", *args.Type)
		}
	}
	if args.User != nil {
		user, err := resolveUser(c, *args.User)
		if err != nil {
			return nil, err
		}
		q.Push(" AND u.user_id = ").Bind(user)
	}
	if args.CreatedAt != nil {
		after, err := parseDatetime(*args.CreatedAt)
		if err != nil {
			return nil, err
		}
		q.Push(" AND u.created_at >= ").Bind(after)
	}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY u.created_at DESC, u.id LIMIT ").Bind(page.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

/* Save */

type SaveStatusUpdateArgs struct {
	ID         *string `json:"id" doc:"Only to edit an update: its id. Omit to post one."`
	Type       *string `json:"type" doc:"project or initiative; follows from which of them is given."`
	Project    *string `json:"project" doc:"Project name, slug, link or id."`
	Initiative *string `json:"initiative" doc:"Initiative name, slug, link or id."`
	Body       *string `json:"body" doc:"Markdown. Mention people with @displayName or @email; they are notified. Required to post."`
	Health     *string `json:"health" doc:"onTrack, atRisk or offTrack."`
}

// updateTarget is the project or initiative an update is about.
type updateTarget struct {
	project    *uuid.UUID
	initiative *uuid.UUID
	name       string
}

func findUpdateTarget(c *app.Ctx, args SaveStatusUpdateArgs) (updateTarget, error) {
	kind := orEmpty(args.Type)
	switch {
	case args.Project != nil && args.Initiative != nil:
		return updateTarget{}, app.BadRequest("Pass either project or initiative, not both")
	case args.Project != nil && (args.Type == nil || kind == "project"):
		id, err := resolveProject(c, *args.Project)
		if err != nil {
			return updateTarget{}, err
		}
		name, err := app.One[string](c, c.DB(), "SELECT name FROM projects WHERE id = $1", id)
		return updateTarget{project: &id, name: name}, err
	case args.Initiative != nil && (args.Type == nil || kind == "initiative"):
		if err := c.RequireMember(); err != nil {
			return updateTarget{}, err
		}
		id, err := resolveInitiative(c, *args.Initiative)
		if err != nil {
			return updateTarget{}, err
		}
		name, err := app.One[string](c, c.DB(), "SELECT name FROM initiatives WHERE id = $1", id)
		return updateTarget{initiative: &id, name: name}, err
	case args.Project == nil && args.Initiative == nil:
		return updateTarget{}, app.BadRequest("Name the project or initiative the update is about")
	}
	return updateTarget{}, app.BadRequest("type \"%s\" doesn't match; it is project with project, initiative with initiative", kind)
}

func saveStatusUpdate(c *app.Ctx, args SaveStatusUpdateArgs) (any, error) {
	if args.ID != nil {
		return editStatusUpdate(c, *args.ID, args)
	}
	return createStatusUpdate(c, args)
}

func createStatusUpdate(c *app.Ctx, args SaveStatusUpdateArgs) (any, error) {
	target, err := findUpdateTarget(c, args)
	if err != nil {
		return nil, err
	}
	body, err := nonEmpty(orEmpty(args.Body), "body")
	if err != nil {
		return nil, err
	}
	var value any
	err = inTx(c, func(tx pgx.Tx) error {
		var health string
		var err error
		if args.Health != nil {
			if health, err = checkHealth(*args.Health); err != nil {
				return err
			}
		} else {
			previous, found, err := app.Maybe[string](c, tx,
				`SELECT health FROM status_updates WHERE project_id IS NOT DISTINCT FROM $1
				 AND initiative_id IS NOT DISTINCT FROM $2 ORDER BY created_at DESC, id LIMIT 1`, target.project, target.initiative)
			if err != nil {
				return err
			}
			health = "onTrack"
			if found {
				health = previous
			}
		}
		id, err := app.One[uuid.UUID](c, tx,
			`INSERT INTO status_updates (workspace_id, project_id, initiative_id, user_id, body, health)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, c.Workspace, target.project, target.initiative, c.User, body, health)
		if err != nil {
			return err
		}
		if err := syncHealth(c, tx, target); err != nil {
			return err
		}
		mentioned, err := mentions(c, body)
		if err != nil {
			return err
		}
		if err := notifyUpdateMentions(c, tx, target, id, mentioned); err != nil {
			return err
		}
		var audience []uuid.UUID
		if target.project != nil {
			audience, err = app.All[uuid.UUID](c, tx,
				`SELECT user_id FROM project_members WHERE project_id = $1
				 UNION SELECT user_id FROM project_subscribers WHERE project_id = $1
				 UNION SELECT lead_id FROM projects WHERE id = $1 AND lead_id IS NOT NULL`, *target.project)
		} else {
			audience, err = app.All[uuid.UUID](c, tx,
				"SELECT owner_id FROM initiatives WHERE id = $1 AND owner_id IS NOT NULL", target.initiative)
		}
		if err != nil {
			return err
		}
		audience = slices.DeleteFunc(audience, func(u uuid.UUID) bool { return slices.Contains(mentioned, u) })
		kind := "initiativeUpdateCreated"
		if target.project != nil {
			kind = "projectUpdateCreated"
		}
		if err := notify(c, tx, audience, Notice{Kind: kind, Summary: "New update on " + target.name,
			Project: target.project, Initiative: target.initiative, StatusUpdate: &id}); err != nil {
			return err
		}
		if value, err = statusUpdateValue(c, tx, id); err != nil {
			return err
		}
		return emitStatusUpdate(c, tx, target, "create", value)
	})
	return value, err
}

func notifyUpdateMentions(c *app.Ctx, tx pgx.Tx, target updateTarget, update uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	return notify(c, tx, users, Notice{Kind: "projectUpdateMention", Summary: "You were mentioned in an update on " + target.name,
		Project: target.project, Initiative: target.initiative, StatusUpdate: &update})
}

func emitStatusUpdate(c *app.Ctx, tx pgx.Tx, target updateTarget, action string, value any) error {
	if target.project == nil {
		return emit(c, tx, "InitiativeUpdate", action, value, nil, nil)
	}
	team, err := webhookTeam(c, tx, *target.project)
	if err != nil {
		return err
	}
	return emit(c, tx, "ProjectUpdate", action, value, nil, team)
}

// findStatusUpdate is the update by id, if the caller can see what it is about, with its author.
func findStatusUpdate(c *app.Ctx, reference string) (uuid.UUID, *uuid.UUID, updateTarget, error) {
	notFound := app.NotFound("No status update \"%s\"", strings.TrimSpace(reference))
	id, ok := asUUID(reference)
	if !ok {
		return uuid.Nil, nil, updateTarget{}, notFound
	}
	type row struct{ Project, Initiative, Author *uuid.UUID }
	r, found, err := app.Row[row](c, c.DB(),
		"SELECT project_id, initiative_id, user_id FROM status_updates WHERE id = $1 AND workspace_id = $2", id, c.Workspace)
	if err != nil {
		return uuid.Nil, nil, updateTarget{}, err
	}
	if !found {
		return uuid.Nil, nil, updateTarget{}, notFound
	}
	var lookup SaveStatusUpdateArgs
	if r.Project != nil {
		lookup.Project = new(r.Project.String())
	}
	if r.Initiative != nil {
		lookup.Initiative = new(r.Initiative.String())
	}
	target, err := findUpdateTarget(c, lookup)
	if err != nil {
		return uuid.Nil, nil, updateTarget{}, notFound
	}
	return id, r.Author, target, nil
}

func editStatusUpdate(c *app.Ctx, reference string, args SaveStatusUpdateArgs) (any, error) {
	if args.Project != nil || args.Initiative != nil || args.Type != nil {
		return nil, app.BadRequest("An update stays on its project or initiative; pass only body and health")
	}
	id, author, target, err := findStatusUpdate(c, reference)
	if err != nil {
		return nil, err
	}
	if author == nil || *author != c.User {
		return nil, app.Forbidden("Only the author can edit a status update")
	}
	var value any
	err = inTx(c, func(tx pgx.Tx) error {
		oldBody, err := app.One[string](c, tx, "SELECT body FROM status_updates WHERE id = $1 FOR UPDATE", id)
		if err != nil {
			return err
		}
		if args.Body != nil {
			if err := editStatusUpdateBody(c, tx, target, id, oldBody, *args.Body); err != nil {
				return err
			}
		}
		if args.Health != nil {
			health, err := checkHealth(*args.Health)
			if err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE status_updates SET health = $2 WHERE id = $1", id, health); err != nil {
				return err
			}
		}
		if err := app.Exec(c, tx, "UPDATE status_updates SET edited_at = now(), updated_at = now() WHERE id = $1", id); err != nil {
			return err
		}
		if err := syncHealth(c, tx, target); err != nil {
			return err
		}
		if value, err = statusUpdateValue(c, tx, id); err != nil {
			return err
		}
		return emitStatusUpdate(c, tx, target, "update", value)
	})
	return value, err
}

func editStatusUpdateBody(c *app.Ctx, tx pgx.Tx, target updateTarget, id uuid.UUID, oldBody, newBody string) error {
	body, err := nonEmpty(newBody, "body")
	if err != nil {
		return err
	}
	if err := app.Exec(c, tx, "UPDATE status_updates SET body = $2 WHERE id = $1", id, body); err != nil {
		return err
	}
	previous, err := mentions(c, oldBody)
	if err != nil {
		return err
	}
	found, err := mentions(c, body)
	if err != nil {
		return err
	}
	mentioned := slices.DeleteFunc(found, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	return notifyUpdateMentions(c, tx, target, id, mentioned)
}

func deleteStatusUpdate(c *app.Ctx, args IDArgs) (any, error) {
	id, author, target, err := findStatusUpdate(c, args.ID)
	if err != nil {
		return nil, err
	}
	if (author == nil || *author != c.User) && c.Role < app.Admin {
		return nil, app.Forbidden("Only the author or a workspace admin can delete a status update")
	}
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := statusUpdateValue(c, tx, id)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM status_updates WHERE id = $1", id); err != nil {
			return err
		}
		if err := syncHealth(c, tx, target); err != nil {
			return err
		}
		return emitStatusUpdate(c, tx, target, "remove", value)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": true, "id": id}, nil
}

/* Result shapes */

type StatusUpdate struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type" enum:"project,initiative"`
	Project      *ProjectRef            `json:"project"`
	Initiative   *InitiativeRef         `json:"initiative"`
	Body         string                 `json:"body"`
	Health       string                 `json:"health" enum:"onTrack,atRisk,offTrack"`
	User         *UserRef               `json:"user"`
	Reactions    []StatusUpdateReaction `json:"reactions"`
	CommentCount int                    `json:"commentCount"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
	EditedAt     *string                `json:"editedAt"`
}

type StatusUpdateReaction struct {
	Emoji string   `json:"emoji"`
	User  *UserRef `json:"user"`
}

type DeletedStatusUpdate struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}
