package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Templates: what new issues, projects and documents start from, for the workspace or one team.

func templateOps() []*Op {
	return []*Op{
		op("list_templates", app.Read, []string{"GET /templates"},
			"List the issue, project and document templates (without their data; read one with get_template). "+
				"save_issue, save_project and save_document take a template's name or id.",
			listTemplates).returns(ResultOf[PageOf[Template]]()),
		op("get_template", app.Read, []string{"GET /templates/{id}"},
			"Read a template by id or name (with type when names repeat across types), including the data it fills in.",
			getTemplate).returns(ResultOf[TemplateDetail]()),
		op("save_template", app.Write, []string{"POST /templates", "PATCH /templates/{id}"},
			"Create a template (omit id; name, type and data are required) or update one (pass id). data holds what "+
				"it fills in, by type. issue: title, description, team, state, priority, estimate, assignee, labels, "+
				"project, cycle, dueDate and subIssues (a list of objects with save_issue's fields). project: name, "+
				"summary, description, state, priority, lead, labels, teams, milestones ([{name, description, "+
				"targetDate}]) and issues (save_issue objects). document: title, content. An issue template with a "+
				"recurrence creates an issue from itself on that schedule (recurring issues), as its creator.",
			saveTemplate).returns(ResultOf[TemplateDetail]()),
		op("delete_template", app.Write, []string{"DELETE /templates/{id}"},
			"Delete a template. Things made from it keep what it filled in.",
			deleteTemplate).returns(ResultOf[DeletedTemplate]()),
	}
}

const templateJSON = `jsonb_build_object(
	'id', tp.id,
	'name', tp.name,
	'description', tp.description,
	'type', tp.type,
	'team', team_ref(tp.team_id),
	'creator', user_ref(tp.creator_id),
	'recurrence', tp.recurrence,
	'nextRunAt', tp.next_run_at,
	'createdAt', tp.created_at,
	'updatedAt', tp.updated_at
)`

func templateValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q,
		"SELECT "+templateJSON+" || jsonb_build_object('data', tp.data) FROM templates tp WHERE tp.id = $1", id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such template")
	}
	return value, nil
}

func checkTemplateType(kind string) (string, error) {
	switch kind {
	case "issue", "project", "document":
		return kind, nil
	}
	return "", app.BadRequest("type %q isn't one of issue, project, document", kind)
}

// findTemplate is a template the caller can see by id or name, of kind when given.
func findTemplate(c *app.Ctx, kind *string, value string) (uuid.UUID, error) {
	if kind != nil {
		checked, err := checkTemplateType(*kind)
		if err != nil {
			return uuid.Nil, err
		}
		return resolveTemplate(c, checked, value)
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return uuid.Nil, err
	}
	var asID *uuid.UUID
	if id, ok := asUUID(value); ok {
		asID = &id
	}
	type row struct {
		ID   uuid.UUID
		Type string
	}
	rows, err := app.Rows[row](c, c.DB(),
		`SELECT id, type FROM templates WHERE workspace_id = $1 AND (team_id IS NULL OR team_id = ANY($2))
		 AND (id = $3 OR lower(name) = lower($4)) ORDER BY id = $3 DESC`,
		c.Workspace, teams, asID, strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, err
	}
	switch {
	case len(rows) == 0:
		return uuid.Nil, app.NotFound("No template %q; list_templates shows them", strings.TrimSpace(value))
	case len(rows) == 1, asID != nil && *asID == rows[0].ID:
		return rows[0].ID, nil
	}
	types := make([]string, len(rows))
	for i, r := range rows {
		types[i] = r.Type
	}
	return uuid.Nil, app.BadRequest("Several templates are named %q (%s); pass type too", strings.TrimSpace(value), strings.Join(types, ", "))
}

type ListTemplatesArgs struct {
	Type *string `json:"type" doc:"issue, project or document."`
	Team *string `json:"team" doc:"Team key, name or id: the team's templates and the workspace-wide ones."`
	Page
}

func listTemplates(c *app.Ctx, args ListTemplatesArgs) (any, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL("SELECT " + templateJSON + " FROM templates tp WHERE tp.workspace_id = ").Bind(c.Workspace)
	q.Push(" AND (tp.team_id IS NULL OR tp.team_id = ANY(").Bind(teams).Push("))")
	if args.Type != nil {
		kind, err := checkTemplateType(*args.Type)
		if err != nil {
			return nil, err
		}
		q.Push(" AND tp.type = ").Bind(kind)
	}
	if args.Team != nil {
		team, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		q.Push(" AND (tp.team_id IS NULL OR tp.team_id = ").Bind(team).Push(")")
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY tp.type, lower(tp.name) LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type GetTemplateArgs struct {
	ID   string  `json:"id" doc:"The template's id or name."`
	Type *string `json:"type" doc:"issue, project or document, when templates of several types share the name."`
}

func getTemplate(c *app.Ctx, args GetTemplateArgs) (any, error) {
	id, err := findTemplate(c, args.Type, args.ID)
	if err != nil {
		return nil, err
	}
	return templateValue(c, c.DB(), id)
}

/* Save */

type issueTemplateData struct {
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	Team        *string         `json:"team"`
	State       *string         `json:"state"`
	Priority    *int32          `json:"priority"`
	Estimate    *float64        `json:"estimate"`
	Assignee    *string         `json:"assignee"`
	Labels      []string        `json:"labels"`
	Project     *string         `json:"project"`
	Cycle       *string         `json:"cycle"`
	DueDate     *string         `json:"dueDate"`
	SubIssues   []SaveIssueArgs `json:"subIssues"`
}

type milestoneTemplateData struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	TargetDate  *string `json:"targetDate"`
}

type projectTemplateData struct {
	Name        *string                 `json:"name"`
	Summary     *string                 `json:"summary"`
	Description *string                 `json:"description"`
	State       *string                 `json:"state"`
	Priority    *int32                  `json:"priority"`
	Lead        *string                 `json:"lead"`
	Labels      []string                `json:"labels"`
	Teams       []string                `json:"teams"`
	Milestones  []milestoneTemplateData `json:"milestones"`
	Issues      []SaveIssueArgs         `json:"issues"`
}

type documentTemplateData struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

// checkTemplateData checks that data only holds what a template of kind fills in.
func checkTemplateData(kind string, data any) error {
	if _, ok := data.(M); !ok {
		return app.BadRequest("data must be an object")
	}
	t := reflect.TypeFor[documentTemplateData]()
	switch kind {
	case "issue":
		t = reflect.TypeFor[issueTemplateData]()
	case "project":
		t = reflect.TypeFor[projectTemplateData]()
	}
	if err := checkShape(data, t); err != nil {
		return app.BadRequest("Invalid %s template data: %s", kind, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(app.Marshal(data)))
	if err := decoder.Decode(reflect.New(t).Interface()); err != nil {
		return app.BadRequest("Invalid %s template data: %s", kind, describeDecodeError(err))
	}
	return nil
}

// checkShape refuses the unknown and missing fields of the objects in value, as t describes them,
// naming the fields it expected.
func checkShape(value any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Implements(nullableType) {
		return checkShape(value, reflect.Zero(t).Interface().(nullable).innerType())
	}
	switch t.Kind() {
	case reflect.Slice:
		items, _ := value.([]any)
		for _, item := range items {
			if err := checkShape(item, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		object, ok := value.(M)
		if !ok {
			return nil
		}
		fields := map[string]reflect.Type{}
		var names []string
		structFields(t, func(name string, field reflect.StructField, _ bool) {
			fields[name] = field.Type
			names = append(names, "`"+name+"`")
		})
		for _, key := range sortedKeys(object) {
			fieldType, known := fields[key]
			if !known {
				return fmt.Errorf("unknown field `%s`, expected one of %s", key, strings.Join(names, ", "))
			}
			if err := checkShape(object[key], fieldType); err != nil {
				return err
			}
		}
		for _, name := range requiredFields(t) {
			if _, ok := object[name]; !ok {
				return fmt.Errorf("missing field `%s`", name)
			}
		}
	}
	return nil
}

// Recurrence is when a recurring issue template creates its issues.
type Recurrence struct {
	Every    uint32  `json:"every"`
	Unit     string  `json:"unit"`
	StartsAt *string `json:"startsAt"`
}

func (r *Recurrence) UnmarshalJSON(data []byte) error {
	type plain Recurrence
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*plain)(r))
}

type SaveTemplateArgs struct {
	ID          *string              `json:"id" doc:"Only to update a template: its id or name. Omit to create one."`
	Name        *string              `json:"name"`
	Description Nullable[string]     `json:"description"`
	Type        *string              `json:"type" doc:"issue, project or document. Required to create; can't change."`
	Team        Nullable[string]     `json:"team" doc:"Team key, name or id the template belongs to; null makes it workspace-wide."`
	Data        any                  `json:"data" doc:"What the template fills in; the keys depend on the type (see the description)."`
	Recurrence  Nullable[Recurrence] `json:"recurrence" doc:"Issue templates only: create an issue from the template on a schedule, e.g. {\"every\": 1, \"unit\": \"week\",\n\"startsAt\": \"2026-11-02T09:00:00Z\"}. unit is day, week or month; startsAt (ISO 8601, default now) is the\nfirst one. null stops it."`
}

// addMonths adds months as chrono does, keeping the day unless the month is shorter.
func addMonths(at time.Time, months int) time.Time {
	first := time.Date(at.Year(), at.Month()+time.Month(months), 1, 0, 0, 0, 0, at.Location())
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(at.Day(), last), at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), at.Location())
}

func nextRun(at time.Time, every uint32, unit string) *time.Time {
	var next time.Time
	switch unit {
	case "day":
		next = at.AddDate(0, 0, int(every))
	case "week":
		next = at.AddDate(0, 0, 7*int(every))
	case "month":
		next = addMonths(at, int(every))
	default:
		return nil
	}
	return &next
}

// RunRecurringTemplates creates the issues of recurring templates that are due. Each template is
// claimed and moved to its next time first, so a slow or failing run never creates the same issue
// twice.
func RunRecurringTemplates(ctx context.Context, state *app.State) error {
	type due struct {
		ID         uuid.UUID
		Workspace  uuid.UUID
		Creator    *uuid.UUID
		Name       string
		Recurrence []byte
		At         time.Time
	}
	var runs []due
	err := pgx.BeginFunc(ctx, state.DB, func(tx pgx.Tx) error {
		var err error
		runs, err = app.Rows[due](ctx, tx,
			`SELECT id, workspace_id, creator_id, name, recurrence, next_run_at FROM templates
			 WHERE next_run_at <= now() AND type = 'issue' ORDER BY next_run_at LIMIT 20 FOR UPDATE SKIP LOCKED`)
		if err != nil {
			return err
		}
		for _, run := range runs {
			var next *time.Time
			var recurrence Recurrence
			if json.Unmarshal(run.Recurrence, &recurrence) == nil {
				next = nextRun(run.At, recurrence.Every, recurrence.Unit)
				for next != nil && !next.After(time.Now()) {
					next = nextRun(*next, recurrence.Every, recurrence.Unit)
				}
			}
			if err := app.Exec(ctx, tx, "UPDATE templates SET next_run_at = $2 WHERE id = $1", run.ID, next); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Creator == nil {
			continue
		}
		c, err := app.LoadCtx(ctx, state, *run.Creator, run.Workspace, []app.Access{app.Read, app.Write})
		if err != nil {
			return err
		}
		save := Find("save_issue")
		if c == nil || save == nil {
			continue
		}
		data, err := app.One[M](ctx, state.DB, "SELECT data FROM templates WHERE id = $1", run.ID)
		if err != nil {
			return err
		}
		args := M{"template": run.ID.String()}
		if _, ok := data["title"]; !ok {
			args["title"] = run.Name
		}
		if _, err := save.Call(c, args); err != nil {
			slog.Warn(fmt.Sprintf("recurring issue from template %s failed: %s", run.Name, app.AsError(err).Message))
		}
	}
	return nil
}

func saveTemplate(c *app.Ctx, args SaveTemplateArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
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
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		action := "create"
		if args.ID != nil {
			action = "update"
			if id, err = findTemplate(c, args.Type, *args.ID); err != nil {
				return err
			}
		} else {
			if args.Type == nil {
				return app.BadRequest("type is required: issue, project or document")
			}
			kind, err := checkTemplateType(*args.Type)
			if err != nil {
				return err
			}
			name, err := nonEmpty(orEmpty(args.Name), "name")
			if err != nil {
				return err
			}
			if args.Data == nil {
				return app.BadRequest("data is required: what the template fills in")
			}
			if id, err = app.One[uuid.UUID](c, tx,
				"INSERT INTO templates (workspace_id, type, name, creator_id) VALUES ($1, $2, $3, $4) RETURNING id",
				c.Workspace, kind, name, c.User); err != nil {
				return err
			}
		}
		kind, err := app.One[string](c, tx, "SELECT type FROM templates WHERE id = $1 FOR UPDATE", id)
		if err != nil {
			return err
		}
		if args.Name != nil {
			name, err := nonEmpty(*args.Name, "name")
			if err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE templates SET name = $2 WHERE id = $1", id, name); err != nil {
				return err
			}
		}
		if args.Description.Set {
			if err := app.Exec(c, tx, "UPDATE templates SET description = $2 WHERE id = $1", id, trimmedOrNil(args.Description.Ptr())); err != nil {
				return err
			}
		}
		if args.Team.Set {
			if err := app.Exec(c, tx, "UPDATE templates SET team_id = $2 WHERE id = $1", id, team); err != nil {
				return err
			}
		}
		if args.Recurrence.Set {
			var stored any
			var next *time.Time
			if r := args.Recurrence.Ptr(); r != nil {
				if kind != "issue" {
					return app.BadRequest("Only issue templates recur")
				}
				if r.Every < 1 || r.Every > 365 || !slices.Contains([]string{"day", "week", "month"}, r.Unit) {
					return app.BadRequest("recurrence is {every: 1-365, unit: day|week|month, startsAt?}")
				}
				starts := time.Now().UTC()
				if r.StartsAt != nil {
					if starts, err = parseDatetime(*r.StartsAt); err != nil {
						return err
					}
				}
				stored = M{"every": r.Every, "unit": r.Unit, "startsAt": starts}
				next = &starts
			}
			if err := app.Exec(c, tx, "UPDATE templates SET recurrence = $2, next_run_at = $3 WHERE id = $1", id, stored, next); err != nil {
				return err
			}
		}
		if args.Data != nil {
			if err := checkTemplateData(kind, args.Data); err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE templates SET data = $2 WHERE id = $1", id, args.Data); err != nil {
				return err
			}
		}
		teamless, err := app.One[bool](c, tx,
			"SELECT recurrence IS NOT NULL AND team_id IS NULL AND NOT data ? 'team' FROM templates WHERE id = $1", id)
		if err != nil {
			return err
		}
		if teamless {
			return app.BadRequest("A recurring template needs a team (team, or data.team)")
		}
		if err := app.Exec(c, tx, "UPDATE templates SET updated_at = now() WHERE id = $1", id); err != nil {
			return err
		}
		if value, err = templateValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Template", action, value, nil, refUUID(value, "team"))
	})
	return value, err
}

type DeleteTemplateArgs struct {
	ID   string  `json:"id" doc:"The template's id or name."`
	Type *string `json:"type" doc:"issue, project or document, when templates of several types share the name."`
}

func deleteTemplate(c *app.Ctx, args DeleteTemplateArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := findTemplate(c, args.Type, args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if value, err = templateValue(c, tx, id); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM templates WHERE id = $1", id); err != nil {
			return err
		}
		return emit(c, tx, "Template", "remove", value, nil, refUUID(value, "team"))
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": true, "id": id, "name": value["name"]}, nil
}

/* Shapes */

type Template struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	Type        string              `json:"type" enum:"issue,project,document"`
	Team        *TeamRef            `json:"team"`
	Creator     *UserRef            `json:"creator"`
	Recurrence  *TemplateRecurrence `json:"recurrence"`
	NextRunAt   *string             `json:"nextRunAt" doc:"When a recurring template next creates an issue."`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
}

type TemplateRecurrence struct {
	Every    int    `json:"every"`
	Unit     string `json:"unit" enum:"day,week,month"`
	StartsAt string `json:"startsAt"`
}

type TemplateDetail struct {
	Template
	Data M `json:"data" doc:"What the template fills in."`
}

type DeletedTemplate struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}
