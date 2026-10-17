// Package release_service is releases: pipelines (continuous or scheduled, with stages from
// planned to completed), the releases that move through them with the issues they ship, and
// release notes. Pipelines can be limited to teams; a pipeline with teams is only visible to
// those who see one of them.
package release_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	release_repo "ricrac/internal/repositories/release"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ReleaseServiceInterface -p release_service -s ReleaseService -o release_service_iface.go
type ReleaseService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewReleaseService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *ReleaseService {
	return &ReleaseService{repo: repo, resolve: resolve, events: events}
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

type IDInput struct {
	ID string `json:"id"`
}

func (self *ReleaseService) releases() release_repo.ReleaseRepositoryInterface {
	return self.repo.Release()
}

func (self *ReleaseService) checkPipelineVisible(ctx context.Context, actor *auth.Actor, pipeline uuid.UUID, what, value string) error {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return err
	}
	visible, err := self.releases().PipelineVisible(ctx, pipeline, teams)
	if err != nil {
		return err
	}
	if !visible {
		return errdefs.NotFound("No %s %q", what, strings.TrimSpace(value))
	}
	return nil
}

func (self *ReleaseService) pipelineID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	id, err := self.resolve.Pipeline(ctx, actor, value)
	if err != nil {
		return uuid.Nil, err
	}
	return id, self.checkPipelineVisible(ctx, actor, id, "release pipeline", value)
}

func (self *ReleaseService) releaseID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	id, err := self.resolve.Release(ctx, actor, value)
	if err != nil {
		return uuid.Nil, err
	}
	pipeline, err := self.releases().ReleasePipeline(ctx, nil, id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, self.checkPipelineVisible(ctx, actor, pipeline, "release", value)
}

// idOrSlug is the id or slug in a value that may be a link or a title-<slug> path segment.
func idOrSlug(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	segment := value[strings.LastIndex(value, "/")+1:]
	if _, ok := utils.AsUUID(segment); ok {
		return segment
	}
	i := strings.LastIndex(segment, "-")
	if i < 0 {
		return segment
	}
	if tail := segment[i+1:]; len(tail) == 12 && utils.IsHex(tail) {
		return tail
	}
	return segment
}

func (self *ReleaseService) noteID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	key := idOrSlug(value)
	var id *uuid.UUID
	if parsed, ok := utils.AsUUID(key); ok {
		id = &parsed
	}
	note, err := self.releases().NoteByKey(ctx, actor.Workspace, id, key)
	if err != nil {
		return uuid.Nil, err
	}
	if note == nil {
		return uuid.Nil, errdefs.NotFound("No release note %q; use its id or slug", strings.TrimSpace(value))
	}
	return note.ID, self.checkPipelineVisible(ctx, actor, note.PipelineID, "release note", value)
}

func oneOf(value string, options []string, what string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !slices.Contains(options, value) {
		return "", errdefs.BadRequest("%s is one of %s", what, strings.Join(options, ", "))
	}
	return value, nil
}

// trimNullable trims a nullable text, turning a blank one into null.
func trimNullable(value args.Nullable[string]) args.Nullable[string] {
	if !value.Valid {
		return value
	}
	if trimmed := strings.TrimSpace(value.Value); trimmed != "" {
		return args.Nullable[string]{Set: true, Valid: true, Value: trimmed}
	}
	return args.Nullable[string]{Set: true}
}

// emitCreatedOrChanged emits create without a previous value, else update when something changed.
func (self *ReleaseService) emitCreatedOrChanged(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, kind string, before *any, value any) error {
	if before == nil {
		return self.events.Emit(ctx, tx, actor, kind, "create", value, nil, nil)
	}
	from := events_service.ChangedFields(*before, value)
	if len(from) == 0 {
		return nil
	}
	return self.events.Emit(ctx, tx, actor, kind, "update", value, from, nil)
}

func (self *ReleaseService) pipelineValues(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, withStages, withTeams bool) ([]models.ReleasePipeline, error) {
	pipelines, err := self.releases().LoadPipelines(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.releases().ReleaseCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.ReleasePipeline, len(pipelines))
	for i, p := range pipelines {
		values[i] = models.ReleasePipelineOf(p, counts[p.ID], withStages, withTeams)
	}
	return values, nil
}

func (self *ReleaseService) pipelineValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.ReleasePipeline, error) {
	values, err := self.pipelineValues(ctx, tx, []uuid.UUID{id}, true, true)
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	if len(values) == 0 {
		return models.ReleasePipeline{}, errdefs.NotFound("No such release pipeline")
	}
	return values[0], nil
}

func (self *ReleaseService) releaseValues(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.Release, error) {
	releases, err := self.releases().LoadReleases(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.releases().IssueCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.Release, len(releases))
	for i, r := range releases {
		values[i] = models.ReleaseOf(r, counts[r.ID])
	}
	return values, nil
}

func (self *ReleaseService) releaseValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.Release, error) {
	values, err := self.releaseValues(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.Release{}, err
	}
	if len(values) == 0 {
		return models.Release{}, errdefs.NotFound("No such release")
	}
	return values[0], nil
}

func (self *ReleaseService) noteValues(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, withContent bool) ([]models.ReleaseNote, error) {
	notes, err := self.releases().LoadNotes(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	covered, err := self.releases().CoveredReleases(ctx, tx, notes)
	if err != nil {
		return nil, err
	}
	values := make([]models.ReleaseNote, len(notes))
	for i, n := range notes {
		values[i] = models.ReleaseNoteOf(n, covered[n.ID], withContent)
	}
	return values, nil
}

func (self *ReleaseService) noteValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.ReleaseNote, error) {
	values, err := self.noteValues(ctx, tx, []uuid.UUID{id}, true)
	if err != nil {
		return models.ReleaseNote{}, err
	}
	if len(values) == 0 {
		return models.ReleaseNote{}, errdefs.NotFound("No such release note")
	}
	return values[0], nil
}
