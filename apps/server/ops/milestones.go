package ops

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Project milestones: the stages of a project, each with its issues' progress.

func milestoneOps() []*Op {
	return []*Op{
		op("list_milestones", app.Read, []string{"GET /projects/{project}/milestones"},
			"List a project's milestones in order, each with its issue counts and progress (completed issues / (all - "+
				"canceled)).",
			listMilestones).returns(ResultOf[PageOf[Milestone]]()),
		op("get_milestone", app.Read, []string{"GET /milestones/{id}"},
			"Read a milestone by id (or by name with its project), with its progress and its issues.", getMilestone).returns(ResultOf[MilestoneDetail]()),
		op("save_milestone", app.Write, []string{"POST /milestones", "PATCH /milestones/{id}"},
			"Create a milestone in a project (omit id; project and name are required) or change one (pass its id, or "+
				"its name with the project). Names are unique within a project. Put issues in it with save_issue "+
				"(milestone).",
			saveMilestone).returns(ResultOf[Milestone]()),
		op("delete_milestone", app.Write, []string{"DELETE /milestones/{id}"},
			"Delete a milestone. Its issues stay in the project, without a milestone.", deleteMilestone).returns(ResultOf[DeletedMilestone]()),
	}
}

// milestoneJSON is the JSON of the milestone m, with the progress of the issues the caller sees.
func milestoneJSON(c *app.Ctx) string {
	return `(jsonb_build_object('id', m.id, 'name', m.name, 'description', m.description,
	'targetDate', m.target_date, 'sortOrder', m.sort_order, 'project', project_ref(m.project_id),
	'createdAt', m.created_at, 'updatedAt', m.updated_at) || project_issue_stats(NULL, m.id, ` + viewer(c) + `))`
}

const milestoneOrder = "m.sort_order, m.target_date NULLS LAST, m.created_at"

// projectMilestones is a project's milestones in order, as a JSON array.
func projectMilestones(c *app.Ctx, q app.Querier, project uuid.UUID) (any, error) {
	return app.One[any](c, q,
		"SELECT coalesce(jsonb_agg("+milestoneJSON(c)+" ORDER BY "+milestoneOrder+"), '[]') FROM project_milestones m WHERE m.project_id = $1",
		project)
}

func milestoneValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, "SELECT "+milestoneJSON(c)+" FROM project_milestones m WHERE m.id = $1", id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such milestone")
	}
	return value, nil
}

type ListMilestonesArgs struct {
	Project string  `json:"project" doc:"Project name, slug, link or id."`
	Limit   *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor  *string `json:"cursor" doc:"The endCursor of the previous page."`
}

func listMilestones(c *app.Ctx, args ListMilestonesArgs) (any, error) {
	project, err := resolveProject(c, args.Project)
	if err != nil {
		return nil, err
	}
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		"SELECT "+milestoneJSON(c)+" FROM project_milestones m WHERE m.project_id = $1 ORDER BY "+milestoneOrder+" LIMIT $2 OFFSET $3",
		project, page.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

type MilestoneArgs struct {
	ID      string  `json:"id" doc:"The milestone's id, or its name together with project."`
	Project *string `json:"project" doc:"Project name, slug or id; needed when id is a name."`
}

func findMilestone(c *app.Ctx, id string, project *string) (uuid.UUID, error) {
	var projectID *uuid.UUID
	if project != nil {
		found, err := resolveProject(c, *project)
		if err != nil {
			return uuid.Nil, err
		}
		projectID = &found
	}
	return resolveMilestone(c, projectID, id)
}

func getMilestone(c *app.Ctx, args MilestoneArgs) (any, error) {
	id, err := findMilestone(c, args.ID, args.Project)
	if err != nil {
		return nil, err
	}
	value, err := milestoneValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	value["issues"], err = app.One[any](c, c.DB(),
		`SELECT coalesce(jsonb_agg(issue_ref(i.id) || jsonb_build_object('assignee', user_ref(i.assignee_id),
			'priority', i.priority) ORDER BY i.sort_order), '[]')
		 FROM issues i WHERE i.milestone_id = $1 AND i.trashed_at IS NULL AND i.team_id = ANY($2)`, id, teams)
	if err != nil {
		return nil, err
	}
	return value, nil
}

type SaveMilestoneArgs struct {
	ID          *string          `json:"id" doc:"Only to change a milestone: its id, or its name together with project. Omit to create one."`
	Project     *string          `json:"project" doc:"Project name, slug or id. Required to create."`
	Name        *string          `json:"name" doc:"Required to create; unique within the project."`
	Description Nullable[string] `json:"description" doc:"Markdown; null clears it."`
	TargetDate  Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	SortOrder   *float64         `json:"sortOrder" doc:"Position among the project's milestones (lower comes first; ties go by target date)."`
}

func saveMilestone(c *app.Ctx, args SaveMilestoneArgs) (any, error) {
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := findMilestone(c, *args.ID, args.Project)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		if existing != nil {
			id = *existing
		} else if id, err = createMilestone(c, tx, args); err != nil {
			return err
		}
		if err := applyMilestoneChanges(c, tx, id, args); err != nil {
			return err
		}
		if value, err = milestoneValue(c, tx, id); err != nil {
			return err
		}
		project, err := app.One[uuid.UUID](c, tx, "SELECT project_id FROM project_milestones WHERE id = $1", id)
		if err != nil {
			return err
		}
		team, err := webhookTeam(c, tx, project)
		if err != nil {
			return err
		}
		action := "create"
		if existing != nil {
			action = "update"
		}
		return emit(c, tx, "ProjectMilestone", action, value, nil, team)
	})
	return value, err
}

func createMilestone(c *app.Ctx, tx pgx.Tx, args SaveMilestoneArgs) (uuid.UUID, error) {
	if args.Project == nil {
		return uuid.Nil, app.BadRequest("project is required to create a milestone")
	}
	project, err := resolveProject(c, *args.Project)
	if err != nil {
		return uuid.Nil, err
	}
	name, err := nonEmpty(orEmpty(args.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	return app.One[uuid.UUID](c, tx, "INSERT INTO project_milestones (project_id, name) VALUES ($1, $2) RETURNING id", project, name)
}

func applyMilestoneChanges(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveMilestoneArgs) error {
	if args.Name != nil {
		name, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return err
		}
		taken, err := app.One[bool](c, tx,
			`SELECT EXISTS (SELECT 1 FROM project_milestones o JOIN project_milestones m ON m.id = $1
				WHERE o.project_id = m.project_id AND o.id <> m.id AND lower(o.name) = lower($2))`, id, name)
		if err != nil {
			return err
		}
		if taken {
			return app.Conflict("The project already has a milestone \"%s\"", name)
		}
		if err := app.Exec(c, tx, "UPDATE project_milestones SET name = $2 WHERE id = $1", id, name); err != nil {
			return err
		}
	}
	if args.Description.Set {
		if err := app.Exec(c, tx, "UPDATE project_milestones SET description = $2 WHERE id = $1", id, args.Description.Ptr()); err != nil {
			return err
		}
	}
	if args.TargetDate.Set {
		var target *time.Time
		if args.TargetDate.Valid && !isNoneWord(args.TargetDate.Value) {
			date, err := parseDate(args.TargetDate.Value)
			if err != nil {
				return err
			}
			target = &date
		}
		if err := app.Exec(c, tx, "UPDATE project_milestones SET target_date = $2 WHERE id = $1", id, target); err != nil {
			return err
		}
	}
	if args.SortOrder != nil {
		if err := app.Exec(c, tx, "UPDATE project_milestones SET sort_order = $2 WHERE id = $1", id, *args.SortOrder); err != nil {
			return err
		}
	}
	return app.Exec(c, tx, "UPDATE project_milestones SET updated_at = now() WHERE id = $1", id)
}

func deleteMilestone(c *app.Ctx, args MilestoneArgs) (any, error) {
	id, err := findMilestone(c, args.ID, args.Project)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var err error
		if value, err = milestoneValue(c, tx, id); err != nil {
			return err
		}
		project, err := app.One[uuid.UUID](c, tx, "SELECT project_id FROM project_milestones WHERE id = $1", id)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM project_milestones WHERE id = $1", id); err != nil {
			return err
		}
		team, err := webhookTeam(c, tx, project)
		if err != nil {
			return err
		}
		return emit(c, tx, "ProjectMilestone", "remove", value, nil, team)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": true, "id": id, "name": value["name"]}, nil
}

/* Result shapes */

type Milestone struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	TargetDate  *string    `json:"targetDate"`
	SortOrder   float64    `json:"sortOrder"`
	Project     ProjectRef `json:"project"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
	ProjectIssueStats
}

type MilestoneDetail struct {
	Milestone
	Issues []IssueChild `json:"issues"`
}

type DeletedMilestone struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}
