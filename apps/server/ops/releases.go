package ops

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Releases: pipelines (continuous or scheduled, with stages from planned to completed), the
// releases that move through them with the issues they ship, and release notes. Pipelines can be
// limited to teams; a pipeline with teams is only visible to those who see one of them.

func releaseOps() []*Op {
	return []*Op{
		op("list_release_pipelines", app.Read, []string{"GET /release-pipelines"},
			"List release pipelines by name: type (continuous: every merge ships; scheduled: releases are planned "+
				"ahead), whether they deploy to production and how many releases they have. Set includeStages / "+
				"includeTeams to also get their stages and teams.",
			listPipelines).returns(ResultOf[PageOf[ReleasePipeline]]()),
		op("get_release_pipeline", app.Read, []string{"GET /release-pipelines/{id}"},
			"Read a release pipeline by id, slug or name, with its stages (in order) and teams.",
			getPipeline).returns(ResultOf[ReleasePipeline]()),
		op("save_release_pipeline", app.Write, []string{"POST /release-pipelines", "PATCH /release-pipelines/{id}"},
			"Create a release pipeline (omit id; name is required) or update one (pass id, slug or name). New "+
				"pipelines get the stages Planned, In Progress, Released and Canceled; change them with "+
				"save_release_stage. Teams are given as key, name or id.",
			savePipeline).returns(ResultOf[ReleasePipeline]()),
		op("delete_release_pipeline", app.Write, []string{"DELETE /release-pipelines/{id}"},
			"Archive a release pipeline. Its releases stay readable.",
			deletePipeline).returns(ResultOf[ReleasePipeline]()),
		op("save_release_stage", app.Write, []string{"POST /release-stages", "PATCH /release-stages/{id}"},
			"Add a stage to a release pipeline (omit id; pipeline, name and type are required) or change one (pass its "+
				"id). The type (planned, started, completed, canceled) decides which timestamps releases get when they "+
				"move into it.",
			saveStage).returns(ResultOf[ReleaseStageDetail]()),
		op("delete_release_stage", app.Write, []string{"DELETE /release-stages/{id}"},
			"Delete a release stage. When releases are in it, moveReleasesTo (another stage of the pipeline: name, "+
				"type or id) is required.",
			deleteStage).returns(ResultOf[DeletedReleaseStage]()),
		op("list_releases", app.Read, []string{"GET /releases"},
			"List releases, newest first: name, version, pipeline, stage, dates, commit and how many issues they ship. "+
				"Narrow by pipeline, stage (a type like started works across pipelines; a stage name needs pipeline) or a "+
				"query on the name, version or commit. Archived releases are left out unless includeArchived is set.",
			listReleases).returns(ResultOf[PageOf[Release]]()),
		op("get_release", app.Read, []string{"GET /releases/{id}"},
			"Read a release by id, slug or name, with the issues in it (with their status) and the release notes "+
				"that cover it.",
			getRelease).returns(ResultOf[ReleaseDetail]()),
		op("save_release", app.Write, []string{"POST /releases", "PATCH /releases/{id}"},
			"Create a release (omit id; name and pipeline are required) or update one (pass id, slug or name). "+
				"Omitted fields stay as they are; null clears a field. New releases start in the pipeline's first planned "+
				"stage unless stage is given. Moving to a started, completed or canceled stage stamps startedAt, "+
				"completedAt or canceledAt unless you pass them; moving back to planned clears them. Add issues to a "+
				"release with save_issue (addReleases, removeReleases, setReleases).",
			saveRelease).returns(ResultOf[Release]()),
		op("delete_release", app.Write, []string{"DELETE /releases/{id}"},
			"Archive a release. It stays readable and keeps its issues.",
			deleteRelease).returns(ResultOf[Release]()),
		op("list_release_notes", app.Read, []string{"GET /release-notes"},
			"List release notes, newest first, optionally of one pipeline: title, pipeline, the releases they cover "+
				"and who wrote them. Read the content with get_release_note.",
			listNotes).returns(ResultOf[PageOf[ReleaseNote]]()),
		op("get_release_note", app.Read, []string{"GET /release-notes/{id}"},
			"Read a release note by id or slug: title, Markdown content, pipeline and the releases it covers.",
			getNote).returns(ResultOf[ReleaseNote]()),
		op("save_release_note", app.Write, []string{"POST /release-notes", "PATCH /release-notes/{id}"},
			"Create release notes (omit id; pipeline is required) or update them (pass id or slug). Name the releases "+
				"they cover with releases, or with rangeFromRelease and rangeToRelease for every release of the pipeline "+
				"created between the two, inclusive. The title defaults to the newest release's name and version. Use "+
				"patch to edit part of the content without resending it.",
			saveNote).returns(ResultOf[ReleaseNote]()),
		op("delete_release_note", app.Write, []string{"DELETE /release-notes/{id}"},
			"Delete release notes. The releases stay.",
			deleteNote).returns(ResultOf[DeletedReleaseNote]()),
	}
}

var (
	pipelineTypes = []string{"continuous", "scheduled"}
	stageTypes    = []string{"planned", "started", "completed", "canceled"}
	defaultStages = [][3]string{
		{"Planned", "planned", "#bec2c8"},
		{"In Progress", "started", "#f2c94c"},
		{"Released", "completed", "#5e6ad2"},
		{"Canceled", "canceled", "#95a2b3"},
	}
)

/* Representation, access */

const pipelineJSON = `jsonb_build_object(
	'id', p.id,
	'slugId', p.slug_id,
	'name', p.name,
	'type', p.type,
	'isProduction', p.is_production,
	'releaseCount', (SELECT count(*) FROM releases r WHERE r.pipeline_id = p.id AND r.archived_at IS NULL),
	'archivedAt', p.archived_at,
	'createdAt', p.created_at,
	'updatedAt', p.updated_at
)`

const stageJSON = "jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color, 'position', s.position)"

func pipelineColumns(stages, teams bool) string {
	json := "(" + pipelineJSON
	if stages {
		json += ` || jsonb_build_object('stages', (SELECT coalesce(jsonb_agg(` + stageJSON + ` ORDER BY s.position), '[]')
			FROM release_stages s WHERE s.pipeline_id = p.id AND s.archived_at IS NULL))`
	}
	if teams {
		json += ` || jsonb_build_object('teams', (SELECT coalesce(jsonb_agg(team_ref(pt.team_id)), '[]')
			FROM release_pipeline_teams pt WHERE pt.pipeline_id = p.id))`
	}
	return json + ")"
}

const releaseJSON = `jsonb_build_object(
	'id', r.id,
	'slugId', r.slug_id,
	'name', r.name,
	'version', r.version,
	'description', r.description,
	'commitSha', r.commit_sha,
	'pipeline', release_pipeline_ref(r.pipeline_id),
	'stage', release_stage_ref(r.stage_id),
	'startDate', r.start_date,
	'targetDate', r.target_date,
	'startedAt', r.started_at,
	'completedAt', r.completed_at,
	'canceledAt', r.canceled_at,
	'issueCount', (SELECT count(*) FROM issue_releases ir JOIN issues i ON i.id = ir.issue_id
		WHERE ir.release_id = r.id AND i.trashed_at IS NULL),
	'creator', user_ref(r.creator_id),
	'archivedAt', r.archived_at,
	'createdAt', r.created_at,
	'updatedAt', r.updated_at
)`

func noteJSON(withContent bool) string {
	content := ""
	if withContent {
		content = "'content', n.content,"
	}
	return fmt.Sprintf(`jsonb_build_object(
		'id', n.id,
		'slugId', n.slug_id,
		'title', n.title,
		%s
		'pipeline', release_pipeline_ref(n.pipeline_id),
		'releases', (SELECT coalesce(jsonb_agg(release_ref(r.id) ORDER BY r.created_at), '[]')
			FROM releases r WHERE r.id = ANY(n.release_ids)),
		'creator', user_ref(n.creator_id),
		'createdAt', n.created_at,
		'updatedAt', n.updated_at
	)`, content)
}

// pushPipelineVisible adds the condition that the pipeline in column has no teams or a team the
// caller sees.
func pushPipelineVisible(q *app.SQL, c *app.Ctx, column string) error {
	teams, err := c.TeamIDs()
	if err != nil {
		return err
	}
	q.Push(fmt.Sprintf(` AND (NOT EXISTS (SELECT 1 FROM release_pipeline_teams x WHERE x.pipeline_id = %[1]s)
		OR EXISTS (SELECT 1 FROM release_pipeline_teams x WHERE x.pipeline_id = %[1]s AND x.team_id = ANY(`, column)).Bind(teams)
	q.Push(")))")
	return nil
}

func checkPipelineVisible(c *app.Ctx, pipeline uuid.UUID, what, value string) error {
	q := app.NewSQL("SELECT EXISTS (SELECT 1 FROM release_pipelines p WHERE p.id = ").Bind(pipeline)
	if err := pushPipelineVisible(q, c, "p.id"); err != nil {
		return err
	}
	q.Push(")")
	visible, err := app.One[bool](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return err
	}
	if !visible {
		return app.NotFound("No %s %q", what, strings.TrimSpace(value))
	}
	return nil
}

func pipelineID(c *app.Ctx, value string) (uuid.UUID, error) {
	id, err := resolvePipeline(c, value)
	if err != nil {
		return uuid.Nil, err
	}
	return id, checkPipelineVisible(c, id, "release pipeline", value)
}

func releaseID(c *app.Ctx, value string) (uuid.UUID, error) {
	id, err := resolveRelease(c, value)
	if err != nil {
		return uuid.Nil, err
	}
	pipeline, err := app.One[uuid.UUID](c, c.DB(), "SELECT pipeline_id FROM releases WHERE id = $1", id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, checkPipelineVisible(c, pipeline, "release", value)
}

// idOrSlug is the id or slug in a value that may be a link or a title-<slug> path segment.
func idOrSlug(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	segment := value[strings.LastIndex(value, "/")+1:]
	if _, ok := asUUID(segment); ok {
		return segment
	}
	i := strings.LastIndex(segment, "-")
	if i < 0 {
		return segment
	}
	if tail := segment[i+1:]; len(tail) == 12 && isHex(tail) {
		return tail
	}
	return segment
}

func noteID(c *app.Ctx, value string) (uuid.UUID, error) {
	key := idOrSlug(value)
	var id *uuid.UUID
	if parsed, ok := asUUID(key); ok {
		id = &parsed
	}
	type found struct{ ID, Pipeline uuid.UUID }
	row, ok, err := app.Row[found](c, c.DB(),
		"SELECT id, pipeline_id FROM release_notes WHERE workspace_id = $1 AND (id = $2 OR slug_id = $3)", c.Workspace, id, key)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, app.NotFound("No release note %q; use its id or slug", strings.TrimSpace(value))
	}
	return row.ID, checkPipelineVisible(c, row.Pipeline, "release note", value)
}

func pipelineValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	return app.One[M](c, q, "SELECT "+pipelineColumns(true, true)+" FROM release_pipelines p WHERE p.id = $1", id)
}

func releaseValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	return app.One[M](c, q, "SELECT "+releaseJSON+" FROM releases r WHERE r.id = $1", id)
}

func noteValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	return app.One[M](c, q, "SELECT "+noteJSON(true)+" FROM release_notes n WHERE n.id = $1", id)
}

func oneOf(value string, options []string, what string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !slices.Contains(options, value) {
		return "", app.BadRequest("%s is one of %s", what, strings.Join(options, ", "))
	}
	return value, nil
}

/* Pipelines */

type ListPipelinesArgs struct {
	Query           *string `json:"query" doc:"Words in the name."`
	Kind            *string `json:"type" doc:"continuous or scheduled."`
	IsProduction    *bool   `json:"isProduction"`
	Team            *string `json:"team" doc:"Pipelines of this team (key, name or id)."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	IncludeStages   bool    `json:"includeStages,omitempty" doc:"Include each pipeline's stages."`
	IncludeTeams    bool    `json:"includeTeams,omitempty" doc:"Include each pipeline's teams."`
	Page
}

func listPipelines(c *app.Ctx, args ListPipelinesArgs) (any, error) {
	q := app.NewSQL("SELECT " + pipelineColumns(args.IncludeStages, args.IncludeTeams) +
		" FROM release_pipelines p WHERE p.workspace_id = ").Bind(c.Workspace)
	if err := pushPipelineVisible(q, c, "p.id"); err != nil {
		return nil, err
	}
	if !args.IncludeArchived {
		q.Push(" AND p.archived_at IS NULL")
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		q.Push(" AND p.name ILIKE ").Bind("%" + escapeLike(query) + "%")
	}
	if args.Kind != nil {
		kind, err := oneOf(*args.Kind, pipelineTypes, "type")
		if err != nil {
			return nil, err
		}
		q.Push(" AND p.type = ").Bind(kind)
	}
	if args.IsProduction != nil {
		q.Push(" AND p.is_production = ").Bind(*args.IsProduction)
	}
	if args.Team != nil {
		team, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		q.Push(" AND EXISTS (SELECT 1 FROM release_pipeline_teams t WHERE t.pipeline_id = p.id AND t.team_id = ").Bind(team).Push(")")
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY lower(p.name), p.id LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getPipeline(c *app.Ctx, args IDArgs) (any, error) {
	id, err := pipelineID(c, args.ID)
	if err != nil {
		return nil, err
	}
	return pipelineValue(c, c.DB(), id)
}

type SavePipelineArgs struct {
	ID           *string  `json:"id" doc:"Only to update a pipeline: its id, slug or name. Omit to create one."`
	Name         *string  `json:"name" doc:"Required to create."`
	Kind         *string  `json:"type" doc:"continuous (default: every deploy is a release) or scheduled (releases planned ahead)."`
	IsProduction *bool    `json:"isProduction" doc:"Whether it deploys to production."`
	AddTeams     []string `json:"addTeams" doc:"Teams to add (key, name or id)."`
	RemoveTeams  []string `json:"removeTeams"`
	SetTeams     []string `json:"setTeams" doc:"Replace all teams with these."`
}

// resolveTeamList is nil when the list was left out.
func resolveTeamList(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}
	ids := []uuid.UUID{}
	for _, value := range values {
		id, err := resolveTeam(c, value)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func savePipeline(c *app.Ctx, args SavePipelineArgs) (any, error) {
	if args.SetTeams != nil && (args.AddTeams != nil || args.RemoveTeams != nil) {
		return nil, app.BadRequest("Pass setTeams or addTeams/removeTeams, not both")
	}
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := pipelineID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	var name, kind *string
	if args.Name != nil {
		n, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return nil, err
		}
		name = &n
	}
	if args.Kind != nil {
		k, err := oneOf(*args.Kind, pipelineTypes, "type")
		if err != nil {
			return nil, err
		}
		kind = &k
	}
	set, err := resolveTeamList(c, args.SetTeams)
	if err != nil {
		return nil, err
	}
	add, err := resolveTeamList(c, args.AddTeams)
	if err != nil {
		return nil, err
	}
	remove, err := resolveTeamList(c, args.RemoveTeams)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var before M
		var err error
		if existing != nil {
			id = *existing
			if before, err = pipelineValue(c, tx, id); err != nil {
				return err
			}
		} else {
			if name == nil {
				return app.BadRequest("name is required to create a pipeline")
			}
			if id, err = app.One[uuid.UUID](c, tx,
				"INSERT INTO release_pipelines (workspace_id, slug_id, name) VALUES ($1, $2, $3) RETURNING id",
				c.Workspace, slugID(), *name); err != nil {
				return err
			}
			for position, stage := range defaultStages {
				if err := app.Exec(c, tx,
					"INSERT INTO release_stages (pipeline_id, name, type, color, position) VALUES ($1, $2, $3, $4, $5)",
					id, stage[0], stage[1], stage[2], float64(position)); err != nil {
					return err
				}
			}
		}
		if err := app.Exec(c, tx,
			`UPDATE release_pipelines SET name = coalesce($2, name), type = coalesce($3, type),
				is_production = coalesce($4, is_production), updated_at = now()
			 WHERE id = $1`, id, name, kind, args.IsProduction); err != nil {
			return err
		}
		if set != nil {
			if err := app.Exec(c, tx, "DELETE FROM release_pipeline_teams WHERE pipeline_id = $1", id); err != nil {
				return err
			}
			add = set
		}
		if add != nil {
			if err := app.Exec(c, tx,
				"INSERT INTO release_pipeline_teams (pipeline_id, team_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
				id, add); err != nil {
				return err
			}
		}
		if remove != nil {
			if err := app.Exec(c, tx, "DELETE FROM release_pipeline_teams WHERE pipeline_id = $1 AND team_id = ANY($2)", id, remove); err != nil {
				return err
			}
		}
		if value, err = pipelineValue(c, tx, id); err != nil {
			return err
		}
		return emitCreatedOrChanged(c, tx, "ReleasePipeline", before, value, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deletePipeline(c *app.Ctx, args IDArgs) (any, error) {
	id, err := pipelineID(c, args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx,
			"UPDATE release_pipelines SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL", id); err != nil {
			return err
		}
		var err error
		if value, err = pipelineValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "ReleasePipeline", "remove", value, nil, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

/* Stages */

// releaseStage is a stage of the pipeline by id, name or type (the first of that type).
func releaseStage(c *app.Ctx, q app.Querier, pipeline uuid.UUID, value string) (uuid.UUID, error) {
	rows, err := app.Rows[typed](c, q,
		"SELECT id, name, type FROM release_stages WHERE pipeline_id = $1 AND archived_at IS NULL ORDER BY position", pipeline)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, found := findTyped(rows, value); found {
		return id, nil
	}
	return uuid.Nil, app.BadRequest("The pipeline has no stage %q. Options: %s.", value, strings.Join(typedNames(rows), ", "))
}

// stageByID is a stage by id, with its pipeline, checking the caller sees the pipeline.
func stageByID(c *app.Ctx, value string) (uuid.UUID, uuid.UUID, error) {
	notFound := app.NotFound("No release stage %q; pass its id", strings.TrimSpace(value))
	id, ok := asUUID(value)
	if !ok {
		return uuid.Nil, uuid.Nil, notFound
	}
	pipeline, found, err := app.Maybe[uuid.UUID](c, c.DB(),
		`SELECT s.pipeline_id FROM release_stages s JOIN release_pipelines p ON p.id = s.pipeline_id
		 WHERE s.id = $1 AND p.workspace_id = $2`, id, c.Workspace)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !found {
		return uuid.Nil, uuid.Nil, notFound
	}
	return id, pipeline, checkPipelineVisible(c, pipeline, "release stage", value)
}

type SaveStageArgs struct {
	ID       *string  `json:"id" doc:"Only to update a stage: its id. Omit to create one."`
	Pipeline *string  `json:"pipeline" doc:"The pipeline (id, slug or name). Required to create."`
	Name     *string  `json:"name" doc:"Required to create."`
	Kind     *string  `json:"type" doc:"planned, started, completed or canceled. Required to create."`
	Color    *string  `json:"color" doc:"Hex color like #f2c94c."`
	Position *float64 `json:"position" doc:"Order in the pipeline, lowest first. New stages go last."`
}

func saveStage(c *app.Ctx, args SaveStageArgs) (any, error) {
	var name, kind, color *string
	if args.Name != nil {
		n, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return nil, err
		}
		name = &n
	}
	if args.Kind != nil {
		k, err := oneOf(*args.Kind, stageTypes, "type")
		if err != nil {
			return nil, err
		}
		kind = &k
	}
	if args.Color != nil {
		checked, err := checkColor(*args.Color)
		if err != nil {
			return nil, err
		}
		color = &checked
	}
	var existing *uuid.UUID
	var pipeline uuid.UUID
	switch {
	case args.ID != nil:
		id, of, err := stageByID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		if args.Pipeline != nil {
			other, err := pipelineID(c, *args.Pipeline)
			if err != nil {
				return nil, err
			}
			if other != of {
				return nil, app.BadRequest("A stage can't move to another pipeline")
			}
		}
		existing, pipeline = &id, of
	case args.Pipeline != nil:
		var err error
		if pipeline, err = pipelineID(c, *args.Pipeline); err != nil {
			return nil, err
		}
	default:
		return nil, app.BadRequest("pipeline is required to create a stage")
	}
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		if existing != nil {
			id = *existing
		} else {
			if name == nil {
				return app.BadRequest("name is required to create a stage")
			}
			if kind == nil {
				return app.BadRequest("type is required to create a stage")
			}
			newColor := "#95a2b3"
			for _, stage := range defaultStages {
				if stage[1] == *kind {
					newColor = stage[2]
					break
				}
			}
			if color != nil {
				newColor = *color
			}
			if id, err = app.One[uuid.UUID](c, tx,
				`INSERT INTO release_stages (pipeline_id, name, type, color, position)
				 VALUES ($1, $2, $3, $4, (SELECT coalesce(max(position), -1) + 1 FROM release_stages WHERE pipeline_id = $1))
				 RETURNING id`, pipeline, *name, *kind, newColor); err != nil {
				return err
			}
		}
		if err := app.Exec(c, tx,
			`UPDATE release_stages SET name = coalesce($2, name), type = coalesce($3, type), color = coalesce($4, color),
				position = coalesce($5, position)
			 WHERE id = $1`, id, name, kind, color, args.Position); err != nil {
			return err
		}
		if value, err = app.One[M](c, tx, "SELECT "+stageJSON+" FROM release_stages s WHERE s.id = $1", id); err != nil {
			return err
		}
		value["pipeline"], err = app.One[any](c, tx, "SELECT release_pipeline_ref($1)", pipeline)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

type DeleteStageArgs struct {
	ID             string  `json:"id" doc:"The stage's id."`
	MoveReleasesTo *string `json:"moveReleasesTo" doc:"Where its releases go: another stage of the pipeline (name, type or id). Required when releases are in it."`
}

func deleteStage(c *app.Ctx, args DeleteStageArgs) (any, error) {
	id, pipeline, err := stageByID(c, args.ID)
	if err != nil {
		return nil, err
	}
	var moved int64
	err = inTx(c, func(tx pgx.Tx) error {
		type counts struct{ Stages, Releases int64 }
		count, _, err := app.Row[counts](c, tx,
			`SELECT (SELECT count(*) FROM release_stages WHERE pipeline_id = $1 AND archived_at IS NULL),
				(SELECT count(*) FROM releases WHERE stage_id = $2)`, pipeline, id)
		if err != nil {
			return err
		}
		if count.Stages <= 1 {
			return app.BadRequest("A pipeline needs at least one stage")
		}
		switch {
		case args.MoveReleasesTo != nil:
			target, err := releaseStage(c, tx, pipeline, *args.MoveReleasesTo)
			if err != nil {
				return err
			}
			if target == id {
				return app.BadRequest("moveReleasesTo must be another stage")
			}
			if err := app.Exec(c, tx, "UPDATE releases SET stage_id = $2, updated_at = now() WHERE stage_id = $1", id, target); err != nil {
				return err
			}
			moved = count.Releases
		case count.Releases > 0:
			return app.BadRequest("%d release(s) are in this stage; pass moveReleasesTo with another stage of the pipeline", count.Releases)
		}
		return app.Exec(c, tx, "DELETE FROM release_stages WHERE id = $1", id)
	})
	if err != nil {
		return nil, err
	}
	return M{"id": id, "deleted": true, "releasesMoved": moved}, nil
}

/* Releases */

type ListReleasesArgs struct {
	Pipeline        *string `json:"pipeline" doc:"Pipeline id, slug or name."`
	Stage           *string `json:"stage" doc:"Stage id, type (planned, started, completed, canceled) or name (with pipeline)."`
	Query           *string `json:"query" doc:"Words in the name or version, or the start of the commit SHA."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	OrderBy         *string `json:"orderBy" doc:"createdAt (default), updatedAt or targetDate."`
	Page
}

func listReleases(c *app.Ctx, args ListReleasesArgs) (any, error) {
	orders := map[string]string{
		"createdAt":  "r.created_at DESC",
		"updatedAt":  "r.updated_at DESC",
		"targetDate": "r.target_date ASC NULLS LAST, r.created_at DESC",
	}
	orderBy := "createdAt"
	if args.OrderBy != nil {
		orderBy = *args.OrderBy
	}
	order, ok := orders[orderBy]
	if !ok {
		return nil, app.BadRequest("orderBy %q isn't one of createdAt, updatedAt, targetDate", orderBy)
	}
	var pipeline *uuid.UUID
	if args.Pipeline != nil {
		id, err := pipelineID(c, *args.Pipeline)
		if err != nil {
			return nil, err
		}
		pipeline = &id
	}
	q := app.NewSQL("SELECT " + releaseJSON + " FROM releases r WHERE r.workspace_id = ").Bind(c.Workspace)
	if err := pushPipelineVisible(q, c, "r.pipeline_id"); err != nil {
		return nil, err
	}
	if !args.IncludeArchived {
		q.Push(" AND r.archived_at IS NULL")
	}
	if pipeline != nil {
		q.Push(" AND r.pipeline_id = ").Bind(*pipeline)
	}
	if args.Stage != nil {
		value := *args.Stage
		lower := strings.ToLower(strings.TrimSpace(value))
		id, isID := asUUID(value)
		switch {
		case pipeline != nil:
			stage, err := releaseStage(c, c.DB(), *pipeline, value)
			if err != nil {
				return nil, err
			}
			q.Push(" AND r.stage_id = ").Bind(stage)
		case isID:
			q.Push(" AND r.stage_id = ").Bind(id)
		case slices.Contains(stageTypes, lower):
			q.Push(" AND EXISTS (SELECT 1 FROM release_stages s WHERE s.id = r.stage_id AND s.type = ").Bind(lower).Push(")")
		default:
			return nil, app.BadRequest("Name the pipeline too to filter by stage name, or pass a stage type or id")
		}
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		pattern := "%" + escapeLike(query) + "%"
		q.Push(" AND (r.name ILIKE ").Bind(pattern)
		q.Push(" OR r.version ILIKE ").Bind(pattern)
		q.Push(" OR r.commit_sha ILIKE ").Bind(strings.ReplaceAll(query, "%", "") + "%").Push(")")
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY " + order + ", r.id LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getRelease(c *app.Ctx, args IDArgs) (any, error) {
	id, err := releaseID(c, args.ID)
	if err != nil {
		return nil, err
	}
	value, err := releaseValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	if value["issues"], err = app.One[any](c, c.DB(),
		`SELECT coalesce(jsonb_agg(issue_ref(i.id) || jsonb_build_object('assignee', user_ref(i.assignee_id))
			ORDER BY t.key, i.number), '[]')
		 FROM issue_releases ir JOIN issues i ON i.id = ir.issue_id JOIN teams t ON t.id = i.team_id
		 WHERE ir.release_id = $1 AND i.trashed_at IS NULL AND (i.team_id = ANY($2)
			OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = i.id AND sh.user_id = $3))`,
		id, teams, c.User); err != nil {
		return nil, err
	}
	if value["releaseNotes"], err = app.One[any](c, c.DB(),
		`SELECT coalesce(jsonb_agg(jsonb_build_object('id', n.id, 'slugId', n.slug_id, 'title', n.title,
			'createdAt', n.created_at) ORDER BY n.created_at DESC), '[]')
		 FROM release_notes n WHERE $1 = ANY(n.release_ids)`, id); err != nil {
		return nil, err
	}
	return value, nil
}

type SaveReleaseArgs struct {
	ID          *string          `json:"id" doc:"Only to update a release: its id, slug or name. Omit to create one."`
	Name        *string          `json:"name" doc:"Required to create."`
	Pipeline    *string          `json:"pipeline" doc:"The pipeline (id, slug or name). Required to create; releases can't move between pipelines."`
	Version     Nullable[string] `json:"version" doc:"e.g. 1.4.0; null clears it."`
	Description Nullable[string] `json:"description" doc:"Markdown; null clears it."`
	CommitSHA   Nullable[string] `json:"commitSha" doc:"The commit the release was built from; null clears it."`
	Stage       *string          `json:"stage" doc:"Stage of the pipeline: name, type (planned, started, completed, canceled) or id."`
	StartDate   Nullable[string] `json:"startDate" doc:"YYYY-MM-DD; null clears it."`
	TargetDate  Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	StartedAt   Nullable[string] `json:"startedAt" doc:"ISO 8601 time it started; null clears it."`
	CompletedAt Nullable[string] `json:"completedAt" doc:"ISO 8601 time it shipped; null clears it."`
	CreatedAt   *string          `json:"createdAt" doc:"When it was created (ISO 8601), for imports. Only on create."`
}

// parseNullable parses a nullable text with parse, keeping left out and null as they are.
func parseNullable(value Nullable[string], parse func(string) (time.Time, error)) (Nullable[time.Time], error) {
	if !value.Valid {
		return Nullable[time.Time]{Set: value.Set}, nil
	}
	at, err := parse(value.Value)
	if err != nil {
		return Nullable[time.Time]{}, err
	}
	return some(at), nil
}

func saveRelease(c *app.Ctx, args SaveReleaseArgs) (any, error) {
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := releaseID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	if existing != nil && args.CreatedAt != nil {
		return nil, app.BadRequest("createdAt can only be set when creating a release")
	}
	var pipeline uuid.UUID
	var err error
	switch {
	case existing == nil && args.Pipeline != nil:
		if pipeline, err = pipelineID(c, *args.Pipeline); err != nil {
			return nil, err
		}
	case existing == nil:
		return nil, app.BadRequest("pipeline is required to create a release")
	default:
		if pipeline, err = app.One[uuid.UUID](c, c.DB(), "SELECT pipeline_id FROM releases WHERE id = $1", *existing); err != nil {
			return nil, err
		}
		if args.Pipeline != nil {
			other, err := pipelineID(c, *args.Pipeline)
			if err != nil {
				return nil, err
			}
			if other != pipeline {
				return nil, app.BadRequest("A release can't move to another pipeline")
			}
		}
	}
	var name *string
	if args.Name != nil {
		n, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return nil, err
		}
		name = &n
	}
	var createdAt *time.Time
	if args.CreatedAt != nil {
		at, err := parseDatetime(*args.CreatedAt)
		if err != nil {
			return nil, err
		}
		createdAt = &at
	}
	startDate, err := parseNullable(args.StartDate, parseDate)
	if err != nil {
		return nil, err
	}
	targetDate, err := parseNullable(args.TargetDate, parseDate)
	if err != nil {
		return nil, err
	}
	startedAt, err := parseNullable(args.StartedAt, parseDatetime)
	if err != nil {
		return nil, err
	}
	completedAt, err := parseNullable(args.CompletedAt, parseDatetime)
	if err != nil {
		return nil, err
	}
	version, commitSHA := trimNullable(args.Version), trimNullable(args.CommitSHA)
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var newStage *uuid.UUID
		switch {
		case args.Stage != nil:
			stage, err := releaseStage(c, tx, pipeline, *args.Stage)
			if err != nil {
				return err
			}
			newStage = &stage
		case existing == nil:
			stage, err := firstPlannedStage(c, tx, pipeline)
			if err != nil {
				return err
			}
			newStage = &stage
		}
		var id uuid.UUID
		var before M
		var err error
		if existing != nil {
			id = *existing
			if before, err = releaseValue(c, tx, id); err != nil {
				return err
			}
		} else {
			if name == nil {
				return app.BadRequest("name is required to create a release")
			}
			if id, err = app.One[uuid.UUID](c, tx,
				`INSERT INTO releases (workspace_id, pipeline_id, slug_id, name, creator_id, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, coalesce($6, now()), coalesce($6, now())) RETURNING id`,
				c.Workspace, pipeline, slugID(), *name, c.User, createdAt); err != nil {
				return err
			}
		}
		if newStage != nil {
			if err := moveToStage(c, tx, id, *newStage); err != nil {
				return err
			}
		}
		if err := app.Exec(c, tx,
			`UPDATE releases SET
				name = coalesce($2, name),
				version = CASE WHEN $3 THEN $4 ELSE version END,
				description = CASE WHEN $5 THEN $6 ELSE description END,
				commit_sha = CASE WHEN $7 THEN $8 ELSE commit_sha END,
				start_date = CASE WHEN $9 THEN $10 ELSE start_date END,
				target_date = CASE WHEN $11 THEN $12 ELSE target_date END,
				started_at = CASE WHEN $13 THEN $14 ELSE started_at END,
				completed_at = CASE WHEN $15 THEN $16 ELSE completed_at END,
				updated_at = CASE WHEN $17 THEN now() ELSE updated_at END
			 WHERE id = $1`,
			id, name, version.Set, version.Ptr(), args.Description.Set, args.Description.Ptr(), commitSHA.Set, commitSHA.Ptr(),
			startDate.Set, startDate.Ptr(), targetDate.Set, targetDate.Ptr(), startedAt.Set, startedAt.Ptr(),
			completedAt.Set, completedAt.Ptr(), existing != nil); err != nil {
			return err
		}
		if value, err = releaseValue(c, tx, id); err != nil {
			return err
		}
		return emitCreatedOrChanged(c, tx, "Release", before, value, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func firstPlannedStage(c *app.Ctx, q app.Querier, pipeline uuid.UUID) (uuid.UUID, error) {
	id, found, err := app.Maybe[uuid.UUID](c, q,
		`SELECT id FROM release_stages WHERE pipeline_id = $1 AND archived_at IS NULL
		 ORDER BY type <> 'planned', position LIMIT 1`, pipeline)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, app.BadRequest("The pipeline has no stages; add one with save_release_stage")
	}
	return id, nil
}

// moveToStage moves a release to a stage, stamping or clearing startedAt, completedAt and
// canceledAt by the stage's type. Staying in the same stage changes nothing.
func moveToStage(c *app.Ctx, q app.Querier, release, stage uuid.UUID) error {
	return app.Exec(c, q,
		`UPDATE releases r SET stage_id = s.id,
			started_at = CASE WHEN s.type = 'planned' THEN NULL
				WHEN s.type IN ('started', 'completed') THEN coalesce(r.started_at, now()) ELSE r.started_at END,
			completed_at = CASE WHEN s.type = 'completed' THEN coalesce(r.completed_at, now()) END,
			canceled_at = CASE WHEN s.type = 'canceled' THEN coalesce(r.canceled_at, now()) END
		 FROM release_stages s
		 WHERE r.id = $1 AND s.id = $2 AND r.stage_id IS DISTINCT FROM s.id`, release, stage)
}

func deleteRelease(c *app.Ctx, args IDArgs) (any, error) {
	id, err := releaseID(c, args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx,
			"UPDATE releases SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL", id); err != nil {
			return err
		}
		var err error
		if value, err = releaseValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Release", "remove", value, nil, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

/* Release notes */

type ListNotesArgs struct {
	Pipeline *string `json:"pipeline" doc:"Pipeline id, slug or name."`
	Page
}

func listNotes(c *app.Ctx, args ListNotesArgs) (any, error) {
	q := app.NewSQL("SELECT " + noteJSON(false) + " FROM release_notes n WHERE n.workspace_id = ").Bind(c.Workspace)
	if err := pushPipelineVisible(q, c, "n.pipeline_id"); err != nil {
		return nil, err
	}
	if args.Pipeline != nil {
		id, err := pipelineID(c, *args.Pipeline)
		if err != nil {
			return nil, err
		}
		q.Push(" AND n.pipeline_id = ").Bind(id)
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY n.created_at DESC, n.id LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getNote(c *app.Ctx, args IDArgs) (any, error) {
	id, err := noteID(c, args.ID)
	if err != nil {
		return nil, err
	}
	return noteValue(c, c.DB(), id)
}

type SaveNoteArgs struct {
	ID               *string    `json:"id" doc:"Only to update release notes: their id or slug. Omit to create them."`
	Pipeline         *string    `json:"pipeline" doc:"The pipeline (id, slug or name). Required to create."`
	Title            *string    `json:"title" doc:"Defaults to the newest release's name and version."`
	Content          *string    `json:"content" doc:"Markdown."`
	Patch            []TextEdit `json:"patch" doc:"Edits to the current content instead of the whole text, applied in order, all or none."`
	Releases         []string   `json:"releases" doc:"The releases covered (id, slug or name), replacing the current ones."`
	RangeFromRelease *string    `json:"rangeFromRelease" doc:"With rangeToRelease: cover every release of the pipeline created from this one to that one, inclusive."`
	RangeToRelease   *string    `json:"rangeToRelease"`
}

// coveredReleases is the releases a note covers, oldest first, checking they are in the
// pipeline; nil when neither releases nor a range was given.
func coveredReleases(c *app.Ctx, pipeline uuid.UUID, args SaveNoteArgs) ([]uuid.UUID, error) {
	inPipeline := func(value string) (uuid.UUID, error) {
		id, err := releaseID(c, value)
		if err != nil {
			return uuid.Nil, err
		}
		of, err := app.One[uuid.UUID](c, c.DB(), "SELECT pipeline_id FROM releases WHERE id = $1", id)
		if err != nil {
			return uuid.Nil, err
		}
		if of != pipeline {
			return uuid.Nil, app.BadRequest("The release %q is in another pipeline", value)
		}
		return id, nil
	}
	if (args.RangeFromRelease == nil) != (args.RangeToRelease == nil) {
		return nil, app.BadRequest("Pass both rangeFromRelease and rangeToRelease")
	}
	hasRange := args.RangeFromRelease != nil
	var from, to uuid.UUID
	if hasRange {
		var err error
		if from, err = inPipeline(*args.RangeFromRelease); err != nil {
			return nil, err
		}
		if to, err = inPipeline(*args.RangeToRelease); err != nil {
			return nil, err
		}
	}
	ids := []uuid.UUID{}
	switch {
	case args.Releases != nil && hasRange:
		return nil, app.BadRequest("Pass releases or a range, not both")
	case args.Releases == nil && !hasRange:
		return nil, nil
	case args.Releases != nil:
		for _, value := range args.Releases {
			id, err := inPipeline(value)
			if err != nil {
				return nil, err
			}
			ids = appendUnique(ids, id)
		}
	default:
		var err error
		if ids, err = app.All[uuid.UUID](c, c.DB(),
			`WITH bounds AS (SELECT min(created_at) AS low, max(created_at) AS high FROM releases WHERE id IN ($2, $3))
			 SELECT r.id FROM releases r, bounds b
			 WHERE r.pipeline_id = $1 AND r.created_at BETWEEN b.low AND b.high
				AND (r.archived_at IS NULL OR r.id IN ($2, $3))
			 ORDER BY r.created_at`, pipeline, from, to); err != nil {
			return nil, err
		}
	}
	ordered, err := app.All[uuid.UUID](c, c.DB(), "SELECT id FROM releases WHERE id = ANY($1) ORDER BY created_at, name", ids)
	if err != nil {
		return nil, err
	}
	if ordered == nil {
		ordered = []uuid.UUID{}
	}
	return ordered, nil
}

func defaultNoteTitle(c *app.Ctx, q app.Querier, releases []uuid.UUID) (*string, error) {
	type newest struct {
		Name    string
		Version *string
	}
	row, found, err := app.Row[newest](c, q,
		"SELECT name, version FROM releases WHERE id = ANY($1) ORDER BY created_at DESC LIMIT 1", releases)
	if err != nil || !found {
		return nil, err
	}
	if row.Version != nil && !strings.Contains(row.Name, *row.Version) {
		return new(fmt.Sprintf("%s (%s)", row.Name, *row.Version)), nil
	}
	return &row.Name, nil
}

func saveNote(c *app.Ctx, args SaveNoteArgs) (any, error) {
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := noteID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	var pipeline uuid.UUID
	var err error
	switch {
	case existing == nil && args.Pipeline != nil:
		if pipeline, err = pipelineID(c, *args.Pipeline); err != nil {
			return nil, err
		}
	case existing == nil:
		return nil, app.BadRequest("pipeline is required to create release notes")
	default:
		if pipeline, err = app.One[uuid.UUID](c, c.DB(), "SELECT pipeline_id FROM release_notes WHERE id = $1", *existing); err != nil {
			return nil, err
		}
		if args.Pipeline != nil {
			other, err := pipelineID(c, *args.Pipeline)
			if err != nil {
				return nil, err
			}
			if other != pipeline {
				return nil, app.BadRequest("Release notes can't move to another pipeline")
			}
		}
	}
	releases, err := coveredReleases(c, pipeline, args)
	if err != nil {
		return nil, err
	}
	var title *string
	if args.Title != nil {
		t, err := nonEmpty(*args.Title, "title")
		if err != nil {
			return nil, err
		}
		title = &t
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var before M
		var current string
		var err error
		if existing != nil {
			id = *existing
			if before, err = noteValue(c, tx, id); err != nil {
				return err
			}
			current, _ = before["content"].(string)
		} else {
			if title == nil {
				if title, err = defaultNoteTitle(c, tx, releases); err != nil {
					return err
				}
			}
			if title == nil {
				return app.BadRequest("Pass a title, or the releases the notes cover to name them after the newest")
			}
			if id, err = app.One[uuid.UUID](c, tx,
				`INSERT INTO release_notes (workspace_id, pipeline_id, slug_id, title, creator_id)
				 VALUES ($1, $2, $3, $4, $5) RETURNING id`, c.Workspace, pipeline, slugID(), *title, c.User); err != nil {
				return err
			}
		}
		content, err := newContent(current, args.Content, args.Patch)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			`UPDATE release_notes SET title = coalesce($2, title), content = coalesce($3, content),
				release_ids = coalesce($4, release_ids), updated_at = now()
			 WHERE id = $1`, id, title, content, releases); err != nil {
			return err
		}
		if value, err = noteValue(c, tx, id); err != nil {
			return err
		}
		return emitCreatedOrChanged(c, tx, "ReleaseNote", before, value, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deleteNote(c *app.Ctx, args IDArgs) (any, error) {
	id, err := noteID(c, args.ID)
	if err != nil {
		return nil, err
	}
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := noteValue(c, tx, id)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM release_notes WHERE id = $1", id); err != nil {
			return err
		}
		return emit(c, tx, "ReleaseNote", "remove", value, nil, nil)
	})
	if err != nil {
		return nil, err
	}
	return M{"id": id, "deleted": true}, nil
}

/* Shapes */

type ReleasePipeline struct {
	ID           string         `json:"id"`
	SlugID       string         `json:"slugId"`
	Name         string         `json:"name"`
	Type         string         `json:"type" enum:"continuous,scheduled"`
	IsProduction bool           `json:"isProduction"`
	ReleaseCount int            `json:"releaseCount" doc:"Releases in it, archived ones left out."`
	ArchivedAt   *string        `json:"archivedAt"`
	CreatedAt    string         `json:"createdAt"`
	UpdatedAt    string         `json:"updatedAt"`
	Stages       []ReleaseStage `json:"stages,omitempty" doc:"In order; in lists only with includeStages."`
	Teams        []TeamRef      `json:"teams,omitempty" doc:"In lists only with includeTeams."`
}

type ReleaseStage struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type" enum:"planned,started,completed,canceled"`
	Color    string  `json:"color"`
	Position float64 `json:"position"`
}

type ReleaseStageDetail struct {
	ReleaseStage
	Pipeline ReleasePipelineRef `json:"pipeline"`
}

type DeletedReleaseStage struct {
	ID            string `json:"id"`
	Deleted       bool   `json:"deleted"`
	ReleasesMoved int    `json:"releasesMoved" doc:"Releases moved to moveReleasesTo."`
}

type Release struct {
	ID          string             `json:"id"`
	SlugID      string             `json:"slugId"`
	Name        string             `json:"name"`
	Version     *string            `json:"version"`
	Description *string            `json:"description" doc:"Markdown."`
	CommitSHA   *string            `json:"commitSha"`
	Pipeline    ReleasePipelineRef `json:"pipeline"`
	Stage       *ReleaseStageRef   `json:"stage"`
	StartDate   *string            `json:"startDate"`
	TargetDate  *string            `json:"targetDate"`
	StartedAt   *string            `json:"startedAt"`
	CompletedAt *string            `json:"completedAt"`
	CanceledAt  *string            `json:"canceledAt"`
	IssueCount  int                `json:"issueCount"`
	Creator     *UserRef           `json:"creator"`
	ArchivedAt  *string            `json:"archivedAt"`
	CreatedAt   string             `json:"createdAt"`
	UpdatedAt   string             `json:"updatedAt"`
}

type ReleaseDetail struct {
	Release
	Issues       []ReleaseIssue       `json:"issues" doc:"The issues in it the caller can see."`
	ReleaseNotes []ReleaseNoteSummary `json:"releaseNotes" doc:"Newest first."`
}

type ReleaseIssue struct {
	IssueRef
	Assignee *UserRef `json:"assignee"`
}

type ReleaseNoteSummary struct {
	ID        string `json:"id"`
	SlugID    string `json:"slugId"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

type ReleaseNote struct {
	ID        string             `json:"id"`
	SlugID    string             `json:"slugId"`
	Title     string             `json:"title"`
	Content   string             `json:"content,omitempty" doc:"Markdown; not in lists."`
	Pipeline  ReleasePipelineRef `json:"pipeline"`
	Releases  []ReleaseRef       `json:"releases"`
	Creator   *UserRef           `json:"creator"`
	CreatedAt string             `json:"createdAt"`
	UpdatedAt string             `json:"updatedAt"`
}

type DeletedReleaseNote struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
