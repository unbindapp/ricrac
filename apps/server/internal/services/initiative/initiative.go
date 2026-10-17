// Package initiative_service is initiatives: company goals that group projects (and other
// initiatives), with their progress, health and status. Guests can't see them.
package initiative_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/ent/entitylink"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	initiative_repo "ricrac/internal/repositories/initiative"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	project_service "ricrac/internal/services/project"
	resolve_service "ricrac/internal/services/resolve"
	statusupdate_service "ricrac/internal/services/statusupdate"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i InitiativeServiceInterface -p initiative_service -s InitiativeService -o initiative_service_iface.go
type InitiativeService struct {
	cfg      config.Config
	repo     repositories.RepositoriesInterface
	resolve  resolve_service.ResolveServiceInterface
	events   events_service.EventsServiceInterface
	projects project_service.ProjectServiceInterface
	updates  statusupdate_service.StatusUpdateServiceInterface
}

func NewInitiativeService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, projects project_service.ProjectServiceInterface, updates statusupdate_service.StatusUpdateServiceInterface) *InitiativeService {
	return &InitiativeService{cfg: cfg, repo: repo, resolve: resolve, events: events, projects: projects, updates: updates}
}

type ListInput struct {
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

type SaveInput struct {
	ID                   *string                     `json:"id" doc:"Only to update an initiative: its id, slug, link or name. Omit to create one."`
	Name                 *string                     `json:"name" doc:"Required to create."`
	Summary              args.Nullable[string]       `json:"summary" doc:"A one-line summary, at most 255 characters; null clears it."`
	Description          *string                     `json:"description" doc:"Markdown."`
	Patch                []args.TextEdit             `json:"patch" doc:"Edits to the current description instead of the whole text, applied in order, all or none."`
	Icon                 args.Nullable[string]       `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color                args.Nullable[string]       `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Status               *string                     `json:"status" doc:"planned (the default), active or completed."`
	Owner                args.Nullable[string]       `json:"owner" doc:"Person (id, name, email or \"me\"); null removes the owner."`
	TargetDate           args.Nullable[string]       `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	TargetDateResolution args.Nullable[string]       `json:"targetDateResolution" doc:"How precise targetDate is: month, quarter, halfYear or year; null means the exact day."`
	Parent               args.Nullable[string]       `json:"parent" doc:"The initiative (name, slug or id) this one is part of; null makes it top-level."`
	AddProjects          []string                    `json:"addProjects" doc:"Projects (names, slugs or ids) to add."`
	RemoveProjects       []string                    `json:"removeProjects"`
	SetProjects          []string                    `json:"setProjects" doc:"Replace all projects with these."`
	Links                []project_service.LinkInput `json:"links" doc:"Links to add ({url, title}); a link already on the initiative gets the new title."`
	RemoveLinks          []string                    `json:"removeLinks" doc:"URLs of links to remove."`
	SortOrder            *float64                    `json:"sortOrder" doc:"Position in manually ordered lists."`
}

type IDInput struct {
	ID string `json:"id"`
}

func (self *InitiativeService) values(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, ids []uuid.UUID, withDescription bool) ([]models.Initiative, error) {
	initiatives, err := self.repo.Initiative().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.Initiative().ProjectCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	progress, err := self.repo.Initiative().Progress(ctx, tx, ids, actor.User)
	if err != nil {
		return nil, err
	}
	values := make([]models.Initiative, len(initiatives))
	for i, initiative := range initiatives {
		values[i] = models.InitiativeOf(initiative, self.cfg.PublicURL, counts[initiative.ID], progress[initiative.ID], withDescription)
	}
	return values, nil
}

func (self *InitiativeService) value(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.Initiative, error) {
	values, err := self.values(ctx, tx, actor, []uuid.UUID{id}, true)
	if err != nil {
		return models.Initiative{}, err
	}
	if len(values) == 0 {
		return models.Initiative{}, errdefs.NotFound("No such initiative")
	}
	return values[0], nil
}

func (self *InitiativeService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Initiative], error) {
	none := args.PageOf[models.Initiative]{}
	if err := actor.RequireMember(); err != nil {
		return none, err
	}
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	filter := initiative_repo.ListFilter{Workspace: actor.Workspace, IncludeArchived: input.IncludeArchived, IncludeTrashed: input.IncludeTrashed}
	if input.Status != nil {
		status, err := checkStatus(*input.Status)
		if err != nil {
			return none, err
		}
		filter.Status = &status
	}
	if input.Owner != nil {
		owner, err := self.resolve.User(ctx, actor, *input.Owner)
		if err != nil {
			return none, err
		}
		filter.Owner = &owner
	}
	if input.Parent != nil {
		if utils.IsNoneWord(*input.Parent) {
			filter.TopLevel = true
		} else {
			parent, err := self.resolve.Initiative(ctx, actor, *input.Parent)
			if err != nil {
				return none, err
			}
			filter.Parent = &parent
		}
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Pattern = new("%" + utils.EscapeLike(query) + "%")
	}
	offset, err := page.Offset()
	if err != nil {
		return none, err
	}
	filter.Limit, filter.Offset = page.Size()+1, offset
	ids, err := self.repo.Initiative().List(ctx, filter)
	if err != nil {
		return none, err
	}
	values, err := self.values(ctx, nil, actor, ids, input.IncludeDescription)
	if err != nil {
		return none, err
	}
	return args.WrapOf(page, values)
}

func (self *InitiativeService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.InitiativeDetail, error) {
	if err := actor.RequireMember(); err != nil {
		return models.InitiativeDetail{}, err
	}
	id, err := self.resolve.Initiative(ctx, actor, input.ID)
	if err != nil {
		return models.InitiativeDetail{}, err
	}
	value, err := self.value(ctx, nil, actor, id)
	if err != nil {
		return models.InitiativeDetail{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.InitiativeDetail{}, err
	}
	detail := models.InitiativeDetail{Initiative: value}
	projects, err := self.repo.Initiative().Projects(ctx, id, teams)
	if err != nil {
		return detail, err
	}
	projectIDs := make([]uuid.UUID, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
	}
	stats, err := self.repo.Project().Stats(ctx, nil, projectIDs, actor.User)
	if err != nil {
		return detail, err
	}
	detail.Projects = make([]models.InitiativeProject, len(projects))
	for i, p := range projects {
		detail.Projects[i] = models.InitiativeProjectOf(p, stats[p.ID].Progress)
	}
	children, err := self.repo.Initiative().Children(ctx, id)
	if err != nil {
		return detail, err
	}
	childIDs := make([]uuid.UUID, len(children))
	for i, c := range children {
		childIDs[i] = c.ID
	}
	progress, err := self.repo.Initiative().Progress(ctx, nil, childIDs, actor.User)
	if err != nil {
		return detail, err
	}
	detail.SubInitiatives = make([]models.SubInitiative, len(children))
	for i, c := range children {
		detail.SubInitiatives[i] = models.SubInitiativeOf(c, progress[c.ID])
	}
	if detail.Links, err = self.projects.Links(ctx, nil, entitylink.FieldInitiativeID, id); err != nil {
		return detail, err
	}
	if detail.Documents, err = self.projects.Documents(ctx, nil, entitylink.FieldInitiativeID, id); err != nil {
		return detail, err
	}
	if detail.LatestUpdate, err = self.updates.Latest(ctx, nil, nil, &id); err != nil {
		return detail, err
	}
	return detail, nil
}

func checkStatus(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !slices.Contains([]string{"planned", "active", "completed"}, lower) {
		return "", errdefs.BadRequest("An initiative's status is planned, active or completed, not \"%s\"", value)
	}
	return lower, nil
}

func (self *InitiativeService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.Initiative, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Initiative{}, err
	}
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.resolve.Initiative(ctx, actor, *input.ID)
		if err != nil {
			return models.Initiative{}, err
		}
		existing = &id
	}
	var value models.Initiative
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var before *models.Initiative
		if existing != nil {
			id = *existing
			current, err := self.value(ctx, tx, actor, id)
			if err != nil {
				return err
			}
			before = &current
		} else {
			name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
			if err != nil {
				return err
			}
			if id, err = self.repo.Initiative().Create(ctx, tx, initiative_repo.CreateInput{
				Workspace: actor.Workspace, SlugID: utils.SlugID(), Name: name, Creator: actor.User,
			}); err != nil {
				return err
			}
		}
		if err := self.apply(ctx, tx, actor, id, input); err != nil {
			return err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Initiative().Update(tx, id).SetUpdatedAt(now).Exec(ctx); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		if before == nil {
			return self.events.Emit(ctx, tx, actor, "Initiative", "create", value, nil, nil)
		}
		updatedFrom := events_service.ChangedFields(*before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		return self.events.Emit(ctx, tx, actor, "Initiative", "update", value, updatedFrom, nil)
	})
	if err != nil {
		return models.Initiative{}, err
	}
	return value, nil
}

// setOrClear sets a nullable column to value, or clears it for nil.
func setOrClear[T any](update *ent.InitiativeUpdateOne, value *T, set func(*ent.InitiativeUpdateOne, T) *ent.InitiativeUpdateOne, clear func(*ent.InitiativeUpdateOne) *ent.InitiativeUpdateOne) *ent.InitiativeUpdateOne {
	if value == nil {
		return clear(update)
	}
	return set(update, *value)
}

func (self *InitiativeService) apply(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input *SaveInput) error {
	current, err := self.repo.Initiative().GetForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := self.repo.Initiative().Update(tx, id).SetName(name).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Summary.Set {
		var summary *string
		if trimmed := strings.TrimSpace(input.Summary.Value); input.Summary.Valid && trimmed != "" {
			summary = &trimmed
		}
		if summary != nil && len([]rune(*summary)) > 255 {
			return errdefs.BadRequest("summary is at most 255 characters; put the rest in the description")
		}
		if err := setOrClear(self.repo.Initiative().Update(tx, id), summary, (*ent.InitiativeUpdateOne).SetSummary, (*ent.InitiativeUpdateOne).ClearSummary).Exec(ctx); err != nil {
			return err
		}
	}
	updated, err := args.NewContent(current.Content, input.Description, input.Patch)
	if err != nil {
		return err
	}
	if updated != nil {
		if err := self.repo.Initiative().Update(tx, id).SetContent(*updated).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Icon.Set {
		var icon *string
		if input.Icon.Valid && strings.TrimSpace(input.Icon.Value) != "" {
			icon = &input.Icon.Value
		}
		if err := setOrClear(self.repo.Initiative().Update(tx, id), icon, (*ent.InitiativeUpdateOne).SetIcon, (*ent.InitiativeUpdateOne).ClearIcon).Exec(ctx); err != nil {
			return err
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
		if err := setOrClear(self.repo.Initiative().Update(tx, id), color, (*ent.InitiativeUpdateOne).SetColor, (*ent.InitiativeUpdateOne).ClearColor).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Status != nil {
		if err := self.status(ctx, tx, id, *input.Status); err != nil {
			return err
		}
	}
	if input.Owner.Set {
		var owner *uuid.UUID
		if input.Owner.Valid && !utils.IsNoneWord(input.Owner.Value) {
			found, err := self.resolve.User(ctx, actor, input.Owner.Value)
			if err != nil {
				return err
			}
			owner = &found
		}
		if err := setOrClear(self.repo.Initiative().Update(tx, id), owner, (*ent.InitiativeUpdateOne).SetOwnerID, (*ent.InitiativeUpdateOne).ClearOwnerID).Exec(ctx); err != nil {
			return err
		}
	}
	if input.TargetDate.Set {
		date, err := utils.OptionalDate(input.TargetDate.Ptr())
		if err != nil {
			return err
		}
		update := self.repo.Initiative().Update(tx, id)
		if date == nil {
			update.ClearTargetDateResolution()
		}
		if err := setOrClear(update, date, (*ent.InitiativeUpdateOne).SetTargetDate, (*ent.InitiativeUpdateOne).ClearTargetDate).Exec(ctx); err != nil {
			return err
		}
	}
	if input.TargetDateResolution.Set {
		resolution, err := utils.CheckResolution(input.TargetDateResolution.Ptr())
		if err != nil {
			return err
		}
		initiative, err := self.repo.Initiative().Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if initiative.TargetDate == nil {
			resolution = nil
		}
		if err := setOrClear(self.repo.Initiative().Update(tx, id), resolution, (*ent.InitiativeUpdateOne).SetTargetDateResolution, (*ent.InitiativeUpdateOne).ClearTargetDateResolution).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Parent.Set {
		if err := self.parent(ctx, tx, actor, id, input.Parent); err != nil {
			return err
		}
	}
	if err := self.projectsOf(ctx, tx, actor, id, input.SetProjects, input.AddProjects, input.RemoveProjects); err != nil {
		return err
	}
	if err := self.projects.SaveLinks(ctx, tx, actor, entitylink.FieldInitiativeID, id, input.Links, input.RemoveLinks); err != nil {
		return err
	}
	if input.SortOrder != nil {
		return self.repo.Initiative().Update(tx, id).SetSortOrder(*input.SortOrder).Exec(ctx)
	}
	return nil
}

// status sets the status, stamping when the initiative started and completed.
func (self *InitiativeService) status(ctx context.Context, tx repository.TxInterface, id uuid.UUID, value string) error {
	status, err := checkStatus(value)
	if err != nil {
		return err
	}
	initiative, err := self.repo.Initiative().Get(ctx, tx, id)
	if err != nil {
		return err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	update := self.repo.Initiative().Update(tx, id).SetStatus(status)
	switch {
	case status == "planned":
		update.ClearStartedAt()
	case status == "active" && initiative.StartedAt == nil:
		update.SetStartedAt(now)
	}
	if status != "completed" {
		update.ClearCompletedAt()
	}
	if status == "completed" && initiative.CompletedAt == nil {
		update.SetCompletedAt(now)
	}
	return update.Exec(ctx)
}

func (self *InitiativeService) parent(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, value args.Nullable[string]) error {
	var parent *uuid.UUID
	if value.Valid && !utils.IsNoneWord(value.Value) {
		found, err := self.resolve.Initiative(ctx, actor, value.Value)
		if err != nil {
			return err
		}
		parent = &found
	}
	if parent != nil {
		loops, err := self.repo.Initiative().InOwnTree(ctx, tx, *parent, id)
		if err != nil {
			return err
		}
		if loops {
			return errdefs.BadRequest("An initiative can't be part of itself or of its own sub-initiatives")
		}
	}
	return setOrClear(self.repo.Initiative().Update(tx, id), parent, (*ent.InitiativeUpdateOne).SetParentID, (*ent.InitiativeUpdateOne).ClearParentID).Exec(ctx)
}

func (self *InitiativeService) projectsOf(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return errdefs.BadRequest("Pass either setProjects or addProjects/removeProjects, not both")
	}
	link := func(projects []uuid.UUID) error {
		for _, project := range projects {
			if err := self.repo.Project().LinkInitiatives(ctx, tx, project, []uuid.UUID{id}); err != nil {
				return err
			}
		}
		return nil
	}
	if set != nil {
		ids, err := self.resolve.Projects(ctx, actor, set)
		if err != nil {
			return err
		}
		if err := self.repo.Initiative().KeepProjects(ctx, tx, id, ids); err != nil {
			return err
		}
		if err := link(ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := self.resolve.Projects(ctx, actor, add)
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
	ids, err := self.resolve.Projects(ctx, actor, remove)
	if err != nil {
		return err
	}
	return self.repo.Initiative().UnlinkProjects(ctx, tx, id, ids)
}
