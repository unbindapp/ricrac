package project_service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/entitylink"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	project_repo "ricrac/internal/repositories/project"
	events_service "ricrac/internal/services/events"
)

type SaveInput struct {
	ID                   *string               `json:"id" doc:"Only to update a project: its id, slug, link or name. Omit to create one."`
	Name                 *string               `json:"name" doc:"Required to create."`
	Summary              args.Nullable[string] `json:"summary" doc:"A one-line summary, at most 255 characters; null clears it."`
	Description          *string               `json:"description" doc:"Markdown. Mention people with @displayName or @email; they are notified."`
	Patch                []args.TextEdit       `json:"patch" doc:"Edits to the current description instead of the whole text, applied in order, all or none."`
	Icon                 args.Nullable[string] `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color                args.Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	State                *string               `json:"state" doc:"Project status name, type (backlog, planned, started, paused, completed, canceled) or id. New projects start in the first backlog status."`
	Priority             *int                  `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Lead                 args.Nullable[string] `json:"lead" doc:"Person (id, name, email or \"me\"); null removes the lead."`
	LeadTeam             args.Nullable[string] `json:"leadTeam" doc:"One of the project's teams (key, name or id) that leads it; null clears it."`
	AddTeams             []string              `json:"addTeams" doc:"Teams (keys, names or ids) to add. A new project gets the workspace's team when no team is named."`
	RemoveTeams          []string              `json:"removeTeams"`
	SetTeams             []string              `json:"setTeams" doc:"Replace all teams with these. A project keeps at least one team."`
	AddMembers           []string              `json:"addMembers" doc:"People to add as members (they are subscribed)."`
	RemoveMembers        []string              `json:"removeMembers"`
	Labels               []string              `json:"labels" doc:"Replace all project labels with these (names or ids; Group/Label for a label in a group)."`
	AddLabels            []string              `json:"addLabels"`
	RemoveLabels         []string              `json:"removeLabels"`
	StartDate            args.Nullable[string] `json:"startDate" doc:"YYYY-MM-DD; null clears it."`
	StartDateResolution  args.Nullable[string] `json:"startDateResolution" doc:"How precise startDate is: month, quarter, halfYear or year; null means the exact day."`
	TargetDate           args.Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	TargetDateResolution args.Nullable[string] `json:"targetDateResolution" doc:"How precise targetDate is: month, quarter, halfYear or year; null means the exact day."`
	Links                []LinkInput           `json:"links" doc:"Links to add ({url, title}); a link already on the project gets the new title."`
	RemoveLinks          []string              `json:"removeLinks" doc:"URLs of links to remove."`
	AddInitiatives       []string              `json:"addInitiatives" doc:"Initiatives (names, slugs or ids) to add the project to."`
	RemoveInitiatives    []string              `json:"removeInitiatives"`
	SetInitiatives       []string              `json:"setInitiatives" doc:"Replace all initiatives with these."`
	Blocks               []string              `json:"blocks" doc:"Projects this one blocks (added to existing dependencies)."`
	BlockedBy            []string              `json:"blockedBy" doc:"Projects blocking this one (added)."`
	RelatedTo            []string              `json:"relatedTo" doc:"Projects related to this one (added)."`
	RemoveBlocks         []string              `json:"removeBlocks"`
	RemoveBlockedBy      []string              `json:"removeBlockedBy"`
	RemoveRelatedTo      []string              `json:"removeRelatedTo"`
	SortOrder            *float64              `json:"sortOrder" doc:"Position in manually ordered lists (lower is higher up)."`
	Template             *string               `json:"template" doc:"Project template name or id, applied on create: it fills in what the arguments don't set and adds its milestones and issues."`
}

// projectTemplate is what a project template fills in on a new project.
type projectTemplate struct {
	Name        *string  `json:"name"`
	Summary     *string  `json:"summary"`
	Description *string  `json:"description"`
	State       *string  `json:"state"`
	Priority    *int     `json:"priority"`
	Lead        *string  `json:"lead"`
	Labels      []string `json:"labels"`
	Teams       []string `json:"teams"`
	Milestones  []struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		TargetDate  *string `json:"targetDate"`
	} `json:"milestones"`
	Issues []any `json:"issues"`
}

func (self *ProjectService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.ProjectDetail, error) {
	var id uuid.UUID
	var templateIssues []M
	if input.ID != nil {
		var err error
		if id, err = self.resolve.Project(ctx, actor, *input.ID); err != nil {
			return models.ProjectDetail{}, err
		}
		if err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error { return self.update(ctx, tx, actor, id, *input) }); err != nil {
			return models.ProjectDetail{}, err
		}
	} else {
		if err := actor.RequireMember(); err != nil {
			return models.ProjectDetail{}, err
		}
		err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
			var err error
			id, templateIssues, err = self.create(ctx, tx, actor, *input)
			return err
		})
		if err != nil {
			return models.ProjectDetail{}, err
		}
	}
	for n, issue := range templateIssues {
		if _, err := self.ops.Call(ctx, actor, "save_issue", issue); err != nil {
			return models.ProjectDetail{}, errdefs.BadRequest("The project was created, but the template's issue %d failed: %s", n+1, errdefs.As(err).Message)
		}
	}
	return self.detail(ctx, nil, actor, id)
}

type M = map[string]any

func firstOf[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func orNullable[T any](current args.Nullable[T], fallback *T) args.Nullable[T] {
	if current.Set || fallback == nil {
		return current
	}
	return args.Nullable[T]{Set: true, Valid: true, Value: *fallback}
}

// applyTemplate fills in what the arguments leave to the template.
func (self *ProjectService) applyTemplate(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (projectTemplate, error) {
	var t projectTemplate
	id, err := self.resolve.Template(ctx, actor, "project", *input.Template)
	if err != nil {
		return t, err
	}
	input.Template = nil
	stored, err := self.repo.Project().Template(ctx, tx, id)
	if err != nil {
		return t, err
	}
	if err := remarshal(stored.Data, &t); err != nil {
		return t, errdefs.BadRequest("The project template is invalid: %s", strings.TrimPrefix(err.Error(), "json: "))
	}
	for _, m := range t.Milestones {
		if m.Name == nil {
			return t, errdefs.BadRequest("The project template is invalid: missing field `name`")
		}
	}
	input.Name = firstOf(input.Name, t.Name)
	input.Summary = orNullable(input.Summary, t.Summary)
	if utils.OrEmpty(input.Description) == "" && input.Patch == nil {
		input.Description = t.Description
	}
	input.State = firstOf(input.State, t.State)
	input.Priority = firstOf(input.Priority, t.Priority)
	input.Lead = orNullable(input.Lead, t.Lead)
	if input.Labels == nil && t.Labels != nil {
		input.AddLabels = append(input.AddLabels, t.Labels...)
	}
	if input.SetTeams == nil && len(input.AddTeams) == 0 {
		switch {
		case t.Teams != nil:
			input.AddTeams = t.Teams
		case stored.TeamID != nil:
			input.AddTeams = []string{stored.TeamID.String()}
		default:
			input.AddTeams = nil
		}
	}
	return t, nil
}

// remarshal decodes stored JSON into target as it would come out of Postgres: through a generic
// value first.
func remarshal(raw json.RawMessage, target any) error {
	var value any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	return json.Unmarshal(errdefs.Marshal(value), target)
}

// create creates the project and its template milestones; it returns the template's issues to
// create (as save_issue arguments) once the project is committed.
func (self *ProjectService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input SaveInput) (uuid.UUID, []M, error) {
	var t projectTemplate
	if input.Template != nil {
		var err error
		if t, err = self.applyTemplate(ctx, tx, actor, &input); err != nil {
			return uuid.Nil, nil, err
		}
	}
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, nil, err
	}
	if len(input.SetTeams) == 0 && len(input.AddTeams) == 0 {
		team, err := self.resolve.DefaultTeam(ctx, actor)
		if err != nil {
			return uuid.Nil, nil, err
		}
		input.SetTeams, input.AddTeams = nil, []string{team.String()}
	}
	var status uuid.UUID
	if input.State != nil {
		status, err = self.resolve.ProjectStatus(ctx, actor, *input.State)
	} else {
		status, err = self.defaultStatus(ctx, tx, actor)
	}
	if err != nil {
		return uuid.Nil, nil, err
	}
	input.State = nil
	id, err := self.repo.Project().Create(ctx, tx, project_repo.CreateInput{
		Workspace: actor.Workspace, SlugID: utils.SlugID(), Name: name, Status: status, Creator: actor.User,
	})
	if err != nil {
		return uuid.Nil, nil, err
	}
	if err := self.stampStatus(ctx, tx, id); err != nil {
		return uuid.Nil, nil, err
	}
	if err := self.repo.Project().Subscribe(ctx, tx, id, []uuid.UUID{actor.User}); err != nil {
		return uuid.Nil, nil, err
	}
	if err := self.apply(ctx, tx, actor, id, input, true); err != nil {
		return uuid.Nil, nil, err
	}
	milestones := map[string]uuid.UUID{}
	for n, m := range t.Milestones {
		var target *time.Time
		if m.TargetDate != nil {
			at, err := utils.ParseDatetime(*m.TargetDate)
			if err != nil {
				return uuid.Nil, nil, err
			}
			target = new(time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC))
		}
		milestoneName, err := utils.NonEmpty(*m.Name, "A template milestone's name")
		if err != nil {
			return uuid.Nil, nil, err
		}
		milestone, err := self.repo.Milestone().CreateFull(ctx, tx, actor.Workspace, id, milestoneName, m.Description, target, float64(n))
		if err != nil {
			return uuid.Nil, nil, err
		}
		key := strings.ToLower(milestoneName)
		if _, taken := milestones[key]; !taken {
			milestones[key] = milestone
		}
	}
	firstTeam, err := self.repo.Project().WebhookTeam(ctx, tx, id)
	if err != nil {
		return uuid.Nil, nil, err
	}
	var issues []M
	for n, item := range t.Issues {
		issue, ok := item.(map[string]any)
		if !ok {
			return uuid.Nil, nil, errdefs.BadRequest("The template's issue %d isn't an object", n+1)
		}
		delete(issue, "id")
		issue["project"] = id.String()
		if _, named := issue["team"]; !named && firstTeam != nil {
			issue["team"] = firstTeam.String()
		}
		if milestone, ok := issue["milestone"].(string); ok {
			if found, ok := milestones[strings.ToLower(strings.TrimSpace(milestone))]; ok {
				issue["milestone"] = found.String()
			}
		}
		issues = append(issues, issue)
	}
	value, err := self.value(ctx, tx, actor, id)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return id, issues, self.events.Emit(ctx, tx, actor, "Project", "create", value, nil, firstTeam)
}

// defaultStatus is the status new projects start in: the first backlog status, else the first
// planned one.
func (self *ProjectService) defaultStatus(ctx context.Context, tx repository.TxInterface, actor *auth.Actor) (uuid.UUID, error) {
	id, found, err := self.repo.Project().DefaultStatus(ctx, tx, actor.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, errdefs.BadRequest("The workspace has no project statuses; add one with save_project_status")
	}
	return id, nil
}

// row is the project, locked until the transaction ends.
func (self *ProjectService) row(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Project, error) {
	row, err := self.repo.Project().GetForUpdate(ctx, tx, id)
	if ent.IsNotFound(err) {
		return nil, errdefs.NotFound("No such project")
	}
	return row, err
}

func (self *ProjectService) update(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput) error {
	if input.Template != nil {
		return errdefs.BadRequest("Templates only apply when creating a project")
	}
	before, err := self.value(ctx, tx, actor, id)
	if err != nil {
		return err
	}
	row, err := self.row(ctx, tx, id)
	if err != nil {
		return err
	}
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := self.repo.Project().Update(tx, id).SetName(name).Exec(ctx); err != nil {
			return err
		}
	}
	if input.State != nil {
		status, err := self.resolve.ProjectStatus(ctx, actor, *input.State)
		if err != nil {
			return err
		}
		if status != row.StatusID {
			if err := self.repo.Project().Update(tx, id).SetStatusID(status).Exec(ctx); err != nil {
				return err
			}
			if err := self.stampStatus(ctx, tx, id); err != nil {
				return err
			}
		}
		input.State = nil
	}
	if err := self.apply(ctx, tx, actor, id, input, false); err != nil {
		return err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	if err := self.repo.Project().Update(tx, id).SetUpdatedAt(now).Exec(ctx); err != nil {
		return err
	}
	after, err := self.value(ctx, tx, actor, id)
	if err != nil {
		return err
	}
	updatedFrom := events_service.ChangedFields(before, after)
	if len(updatedFrom) == 0 {
		return nil
	}
	team, err := self.repo.Project().WebhookTeam(ctx, tx, id)
	if err != nil {
		return err
	}
	return self.events.Emit(ctx, tx, actor, "Project", "update", after, updatedFrom, team)
}

// stampStatus stamps when the project started, completed or was canceled, from the type of its
// status.
func (self *ProjectService) stampStatus(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	p, err := self.repo.Project().Get(ctx, tx, id)
	if err != nil {
		return err
	}
	status, err := self.repo.Project().Status(ctx, tx, p.StatusID)
	if err != nil {
		return err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Project().SetStamps(ctx, tx, id, stamps(p, status.Type, now))
}

// stamps is the times a project in a status of this type has, as of now.
func stamps(p *ent.Project, kind string, now time.Time) project_repo.Stamps {
	var s project_repo.Stamps
	switch kind {
	case "started":
		s.StartedAt = orNow(p.StartedAt, now)
	case "paused", "completed", "canceled":
		s.StartedAt = p.StartedAt
	}
	if kind == "completed" {
		s.CompletedAt = orNow(p.CompletedAt, now)
	}
	if kind == "canceled" {
		s.CanceledAt = orNow(p.CanceledAt, now)
	}
	return s
}

func orNow(at *time.Time, now time.Time) *time.Time {
	if at != nil {
		return at
	}
	return &now
}

// setOrClear sets a nullable column to value, or clears it for nil.
func setOrClear[U, T any](update U, value *T, set func(U, T) U, clear func(U) U) U {
	if value == nil {
		return clear(update)
	}
	return set(update, *value)
}

func samePtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// optionalRef resolves a nullable reference: nil when null or a null word.
func optionalRef(ctx context.Context, actor *auth.Actor, value args.Nullable[string], resolve func(context.Context, *auth.Actor, string) (uuid.UUID, error)) (*uuid.UUID, error) {
	if !value.Valid || utils.IsNoneWord(value.Value) {
		return nil, nil
	}
	id, err := resolve(ctx, actor, value.Value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// apply applies the fields that work the same when creating and updating.
func (self *ProjectService) apply(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput, creating bool) error {
	row, err := self.row(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := self.relations(ctx, tx, actor, id, input); err != nil {
		return err
	}
	if input.Summary.Set {
		var summary *string
		if trimmed := strings.TrimSpace(input.Summary.Value); input.Summary.Valid && trimmed != "" {
			summary = &trimmed
		}
		if summary != nil && len([]rune(*summary)) > 255 {
			return errdefs.BadRequest("summary is at most 255 characters; put the rest in the description")
		}
		if !samePtr(summary, row.Summary) {
			if err := setOrClear(self.repo.Project().Update(tx, id), summary, (*ent.ProjectUpdateOne).SetSummary, (*ent.ProjectUpdateOne).ClearSummary).Exec(ctx); err != nil {
				return err
			}
		}
	}
	content, err := args.NewContent(row.Content, input.Description, input.Patch)
	if err != nil {
		return err
	}
	if content != nil && *content != row.Content {
		if err := self.content(ctx, tx, actor, id, row, *content, creating); err != nil {
			return err
		}
	}
	if input.Icon.Set {
		var icon *string
		if input.Icon.Valid && strings.TrimSpace(input.Icon.Value) != "" {
			icon = &input.Icon.Value
		}
		if !samePtr(icon, row.Icon) {
			if err := setOrClear(self.repo.Project().Update(tx, id), icon, (*ent.ProjectUpdateOne).SetIcon, (*ent.ProjectUpdateOne).ClearIcon).Exec(ctx); err != nil {
				return err
			}
		}
	}
	if input.Color.Set {
		var color *string
		if input.Color.Valid {
			checked, err := utils.CheckColor(input.Color.Value)
			if err != nil {
				return err
			}
			color = &checked
		}
		if !samePtr(color, row.Color) {
			if err := setOrClear(self.repo.Project().Update(tx, id), color, (*ent.ProjectUpdateOne).SetColor, (*ent.ProjectUpdateOne).ClearColor).Exec(ctx); err != nil {
				return err
			}
		}
	}
	if input.Priority != nil {
		priority, err := utils.CheckPriority(*input.Priority)
		if err != nil {
			return err
		}
		if priority != row.Priority {
			if err := self.repo.Project().Update(tx, id).SetPriority(priority).Exec(ctx); err != nil {
				return err
			}
		}
	}
	if input.Lead.Set {
		if err := self.lead(ctx, tx, actor, id, row, input.Lead); err != nil {
			return err
		}
	}
	if err := self.teams(ctx, tx, actor, id, input.SetTeams, input.AddTeams, input.RemoveTeams); err != nil {
		return err
	}
	if input.LeadTeam.Set {
		team, err := optionalRef(ctx, actor, input.LeadTeam, self.resolve.Team)
		if err != nil {
			return err
		}
		if team != nil {
			onProject, err := self.repo.Project().HasTeam(ctx, tx, id, *team)
			if err != nil {
				return err
			}
			if !onProject {
				return errdefs.BadRequest("The lead team must be one of the project's teams; add it with addTeams")
			}
		}
		if err := setOrClear(self.repo.Project().Update(tx, id), team, (*ent.ProjectUpdateOne).SetLeadTeamID, (*ent.ProjectUpdateOne).ClearLeadTeamID).Exec(ctx); err != nil {
			return err
		}
	}
	if input.AddMembers != nil {
		users, err := self.resolve.Users(ctx, actor, input.AddMembers)
		if err != nil {
			return err
		}
		if err := self.repo.Project().AddMembers(ctx, tx, id, users); err != nil {
			return err
		}
		if err := self.repo.Project().Subscribe(ctx, tx, id, users); err != nil {
			return err
		}
	}
	if input.RemoveMembers != nil {
		users, err := self.resolve.Users(ctx, actor, input.RemoveMembers)
		if err != nil {
			return err
		}
		if err := self.repo.Project().RemoveMembers(ctx, tx, id, users); err != nil {
			return err
		}
	}
	if err := self.labels(ctx, tx, actor, id, input.Labels, input.AddLabels, input.RemoveLabels); err != nil {
		return err
	}
	if err := self.dates(ctx, tx, id, "start", input.StartDate, input.StartDateResolution); err != nil {
		return err
	}
	if err := self.dates(ctx, tx, id, "target", input.TargetDate, input.TargetDateResolution); err != nil {
		return err
	}
	if err := self.SaveLinks(ctx, tx, actor, entitylink.FieldProjectID, id, input.Links, input.RemoveLinks); err != nil {
		return err
	}
	if err := self.initiatives(ctx, tx, actor, id, input.SetInitiatives, input.AddInitiatives, input.RemoveInitiatives); err != nil {
		return err
	}
	if input.SortOrder != nil {
		return self.repo.Project().Update(tx, id).SetSortOrder(*input.SortOrder).Exec(ctx)
	}
	return nil
}

// content sets the description and notifies the people newly mentioned in it.
func (self *ProjectService) content(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, row *ent.Project, content string, creating bool) error {
	if err := self.repo.Project().Update(tx, id).SetContent(content).Exec(ctx); err != nil {
		return err
	}
	var previous []uuid.UUID
	if !creating {
		var err error
		if previous, err = self.events.Mentions(ctx, actor, row.Content); err != nil {
			return err
		}
	}
	found, err := self.events.Mentions(ctx, actor, content)
	if err != nil {
		return err
	}
	mentioned := slices.DeleteFunc(found, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	if len(mentioned) == 0 {
		return nil
	}
	if err := self.repo.Project().Subscribe(ctx, tx, id, mentioned); err != nil {
		return err
	}
	return self.events.Notify(ctx, tx, actor, mentioned, events_service.Notice{Kind: "projectMention", Summary: "You were mentioned in the project " + row.Name, Project: &id})
}

func (self *ProjectService) lead(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, row *ent.Project, value args.Nullable[string]) error {
	lead, err := optionalRef(ctx, actor, value, self.resolve.User)
	if err != nil {
		return err
	}
	if samePtr(lead, row.LeadID) {
		return nil
	}
	if err := setOrClear(self.repo.Project().Update(tx, id), lead, (*ent.ProjectUpdateOne).SetLeadID, (*ent.ProjectUpdateOne).ClearLeadID).Exec(ctx); err != nil {
		return err
	}
	if lead == nil {
		return nil
	}
	if err := self.repo.Project().Subscribe(ctx, tx, id, []uuid.UUID{*lead}); err != nil {
		return err
	}
	p, err := self.repo.Project().Get(ctx, tx, id)
	if err != nil {
		return err
	}
	return self.events.Notify(ctx, tx, actor, []uuid.UUID{*lead}, events_service.Notice{Kind: "projectAssigned", Summary: "You are now the lead of the project " + p.Name, Project: &id})
}

func (self *ProjectService) dates(ctx context.Context, tx repository.TxInterface, id uuid.UUID, which string, date, resolution args.Nullable[string]) error {
	if date.Set {
		value, err := utils.OptionalDate(date.Ptr())
		if err != nil {
			return err
		}
		update := self.repo.Project().Update(tx, id)
		switch {
		case which == "start" && value == nil:
			update.ClearStartDate().ClearStartDateResolution()
		case which == "start":
			update.SetStartDate(*value)
		case value == nil:
			update.ClearTargetDate().ClearTargetDateResolution()
		default:
			update.SetTargetDate(*value)
		}
		if err := update.Exec(ctx); err != nil {
			return err
		}
	}
	if !resolution.Set {
		return nil
	}
	value, err := utils.CheckResolution(resolution.Ptr())
	if err != nil {
		return err
	}
	p, err := self.repo.Project().Get(ctx, tx, id)
	if err != nil {
		return err
	}
	update := self.repo.Project().Update(tx, id)
	if which == "start" {
		if p.StartDate == nil || value == nil {
			return update.ClearStartDateResolution().Exec(ctx)
		}
		return update.SetStartDateResolution(*value).Exec(ctx)
	}
	if p.TargetDate == nil || value == nil {
		return update.ClearTargetDateResolution().Exec(ctx)
	}
	return update.SetTargetDateResolution(*value).Exec(ctx)
}

func (self *ProjectService) teams(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return errdefs.BadRequest("Pass either setTeams or addTeams/removeTeams, not both")
	}
	if set == nil && add == nil && remove == nil {
		return nil
	}
	if set != nil {
		ids, err := self.resolveTeams(ctx, actor, set)
		if err != nil {
			return err
		}
		if err := self.repo.Project().KeepTeams(ctx, tx, id, ids); err != nil {
			return err
		}
		if err := self.repo.Project().AddTeams(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := self.resolveTeams(ctx, actor, add)
		if err != nil {
			return err
		}
		if err := self.repo.Project().AddTeams(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if remove != nil {
		ids, err := self.resolveTeams(ctx, actor, remove)
		if err != nil {
			return err
		}
		if err := self.repo.Project().RemoveTeams(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	count, err := self.repo.Project().TeamCount(ctx, tx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return errdefs.BadRequest("A project keeps at least one team")
	}
	return self.repo.Project().DropLeadTeamIfGone(ctx, tx, id)
}

// resolveTeams resolves each team, keeping repeats.
func (self *ProjectService) resolveTeams(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	for _, value := range values {
		id, err := self.resolve.Team(ctx, actor, value)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (self *ProjectService) labels(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return errdefs.BadRequest("Pass either labels or addLabels/removeLabels, not both")
	}
	if set != nil {
		ids, err := self.resolve.ProjectLabels(ctx, actor, set)
		if err != nil {
			return err
		}
		if err := self.repo.Project().KeepLabels(ctx, tx, id, ids); err != nil {
			return err
		}
		if err := self.repo.Project().AddLabels(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := self.resolve.ProjectLabels(ctx, actor, add)
		if err != nil {
			return err
		}
		if err := self.repo.Project().AddLabels(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if remove == nil {
		return nil
	}
	ids, err := self.resolve.ProjectLabels(ctx, actor, remove)
	if err != nil {
		return err
	}
	return self.repo.Project().RemoveLabels(ctx, tx, id, ids)
}

// SaveLinks adds links to a project or initiative (column is project_id or initiative_id); a
// link with the same URL gets the new title. Then it removes the links with the URLs in remove.
func (self *ProjectService) SaveLinks(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, column string, id uuid.UUID, links []LinkInput, remove []string) error {
	for _, link := range links {
		url, err := utils.NonEmpty(link.URL, "A link's url")
		if err != nil {
			return err
		}
		label := strings.TrimSpace(link.Title)
		if label == "" {
			label = url
		}
		relabeled, err := self.repo.Project().Relabel(ctx, tx, column, id, url, label)
		if err != nil {
			return err
		}
		if relabeled {
			continue
		}
		if err := self.repo.Project().AddLink(ctx, tx, actor.Workspace, column, id, url, label, actor.User); err != nil {
			return err
		}
	}
	if len(remove) == 0 {
		return nil
	}
	urls := make([]string, len(remove))
	for i, url := range remove {
		urls[i] = strings.TrimSpace(url)
	}
	return self.repo.Project().RemoveLinks(ctx, tx, column, id, urls)
}

func (self *ProjectService) initiatives(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return errdefs.BadRequest("Pass either setInitiatives or addInitiatives/removeInitiatives, not both")
	}
	if (set != nil || add != nil || remove != nil) && actor.Role == auth.Guest {
		return errdefs.Forbidden("Guests can't change initiatives")
	}
	if set != nil {
		ids, err := self.resolve.Initiatives(ctx, actor, set)
		if err != nil {
			return err
		}
		if err := self.repo.Project().KeepInitiatives(ctx, tx, id, ids); err != nil {
			return err
		}
		if err := self.repo.Project().LinkInitiatives(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := self.resolve.Initiatives(ctx, actor, add)
		if err != nil {
			return err
		}
		if err := self.repo.Project().LinkInitiatives(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if remove == nil {
		return nil
	}
	ids, err := self.resolve.Initiatives(ctx, actor, remove)
	if err != nil {
		return err
	}
	return self.repo.Project().UnlinkInitiatives(ctx, tx, id, ids)
}

func (self *ProjectService) relations(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput) error {
	type change struct {
		list    []string
		kind    string
		reverse bool
	}
	ends := func(other uuid.UUID, reverse bool) (uuid.UUID, uuid.UUID) {
		if reverse {
			return other, id
		}
		return id, other
	}
	for _, add := range []change{{input.Blocks, "blocks", false}, {input.BlockedBy, "blocks", true}, {input.RelatedTo, "related", false}} {
		for _, reference := range add.list {
			other, err := self.resolve.Project(ctx, actor, reference)
			if err != nil {
				return err
			}
			if other == id {
				return errdefs.BadRequest("A project can't depend on or relate to itself")
			}
			from, to := ends(other, add.reverse)
			if err := self.repo.Project().Relate(ctx, tx, from, to, add.kind, actor.User); err != nil {
				return err
			}
		}
	}
	for _, remove := range []change{{input.RemoveBlocks, "blocks", false}, {input.RemoveBlockedBy, "blocks", true}, {input.RemoveRelatedTo, "related", false}} {
		for _, reference := range remove.list {
			other, err := self.resolve.Project(ctx, actor, reference)
			if err != nil {
				return err
			}
			from, to := ends(other, remove.reverse)
			if err := self.repo.Project().Unrelate(ctx, tx, from, to, remove.kind); err != nil {
				return err
			}
		}
	}
	return nil
}
