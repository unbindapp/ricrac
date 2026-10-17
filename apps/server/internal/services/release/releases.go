package release_service

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	release_repo "ricrac/internal/repositories/release"
)

type ListReleasesInput struct {
	Pipeline        *string `json:"pipeline" doc:"Pipeline id, slug or name."`
	Stage           *string `json:"stage" doc:"Stage id, type (planned, started, completed, canceled) or name (with pipeline)."`
	Query           *string `json:"query" doc:"Words in the name or version, or the start of the commit SHA."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	OrderBy         *string `json:"orderBy" doc:"createdAt (default), updatedAt or targetDate."`
	args.Page
}

type SaveReleaseInput struct {
	ID          *string               `json:"id" doc:"Only to update a release: its id, slug or name. Omit to create one."`
	Name        *string               `json:"name" doc:"Required to create."`
	Pipeline    *string               `json:"pipeline" doc:"The pipeline (id, slug or name). Required to create; releases can't move between pipelines."`
	Version     args.Nullable[string] `json:"version" doc:"e.g. 1.4.0; null clears it."`
	Description args.Nullable[string] `json:"description" doc:"Markdown; null clears it."`
	CommitSHA   args.Nullable[string] `json:"commitSha" doc:"The commit the release was built from; null clears it."`
	Stage       *string               `json:"stage" doc:"Stage of the pipeline: name, type (planned, started, completed, canceled) or id."`
	StartDate   args.Nullable[string] `json:"startDate" doc:"YYYY-MM-DD; null clears it."`
	TargetDate  args.Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	StartedAt   args.Nullable[string] `json:"startedAt" doc:"ISO 8601 time it started; null clears it."`
	CompletedAt args.Nullable[string] `json:"completedAt" doc:"ISO 8601 time it shipped; null clears it."`
	CreatedAt   *string               `json:"createdAt" doc:"When it was created (ISO 8601), for imports. Only on create."`
}

var releaseOrders = []string{"createdAt", "updatedAt", "targetDate"}

func (self *ReleaseService) ListReleases(ctx context.Context, actor *auth.Actor, input *ListReleasesInput) (args.PageOf[models.Release], error) {
	orderBy := "createdAt"
	if input.OrderBy != nil {
		orderBy = *input.OrderBy
	}
	if !slices.Contains(releaseOrders, orderBy) {
		return args.PageOf[models.Release]{}, errdefs.BadRequest("orderBy %q isn't one of createdAt, updatedAt, targetDate", orderBy)
	}
	var pipeline *uuid.UUID
	if input.Pipeline != nil {
		id, err := self.pipelineID(ctx, actor, *input.Pipeline)
		if err != nil {
			return args.PageOf[models.Release]{}, err
		}
		pipeline = &id
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Release]{}, err
	}
	filter := release_repo.ReleaseFilter{
		Workspace: actor.Workspace, Teams: teams, IncludeArchived: input.IncludeArchived, Pipeline: pipeline, OrderBy: orderBy,
	}
	if input.Stage != nil {
		value := *input.Stage
		lower := strings.ToLower(strings.TrimSpace(value))
		id, isID := utils.AsUUID(value)
		switch {
		case pipeline != nil:
			stage, err := self.stage(ctx, nil, *pipeline, value)
			if err != nil {
				return args.PageOf[models.Release]{}, err
			}
			filter.Stage = &stage
		case isID:
			filter.Stage = &id
		case slices.Contains(stageTypes, lower):
			filter.StageType = &lower
		default:
			return args.PageOf[models.Release]{}, errdefs.BadRequest("Name the pipeline too to filter by stage name, or pass a stage type or id")
		}
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Pattern = new("%" + utils.EscapeLike(query) + "%")
		filter.CommitPattern = new(strings.ReplaceAll(query, "%", "") + "%")
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Release]{}, err
	}
	filter.Limit, filter.Offset = input.Size()+1, offset
	ids, err := self.releases().Releases(ctx, filter)
	if err != nil {
		return args.PageOf[models.Release]{}, err
	}
	releases, err := self.releaseValues(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.Release]{}, err
	}
	return args.WrapOf(input.Page, releases)
}

func (self *ReleaseService) GetRelease(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ReleaseDetail, error) {
	id, err := self.releaseID(ctx, actor, input.ID)
	if err != nil {
		return models.ReleaseDetail{}, err
	}
	release, err := self.releaseValue(ctx, nil, id)
	if err != nil {
		return models.ReleaseDetail{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.ReleaseDetail{}, err
	}
	issues, err := self.releases().ReleaseIssues(ctx, id, teams, actor.User)
	if err != nil {
		return models.ReleaseDetail{}, err
	}
	notes, err := self.releases().NotesCovering(ctx, id)
	if err != nil {
		return models.ReleaseDetail{}, err
	}
	detail := models.ReleaseDetail{Release: release, Issues: []models.ReleaseIssue{}, ReleaseNotes: []models.ReleaseNoteSummary{}}
	for _, i := range issues {
		detail.Issues = append(detail.Issues, models.ReleaseIssueOf(i))
	}
	for _, n := range notes {
		detail.ReleaseNotes = append(detail.ReleaseNotes, models.ReleaseNoteSummaryOf(n))
	}
	return detail, nil
}

// parseNullable parses a nullable text with parse, keeping left out and null as they are.
func parseNullable(value args.Nullable[string], parse func(string) (time.Time, error)) (args.Nullable[time.Time], error) {
	if !value.Valid {
		return args.Nullable[time.Time]{Set: value.Set}, nil
	}
	at, err := parse(value.Value)
	if err != nil {
		return args.Nullable[time.Time]{}, err
	}
	return args.Nullable[time.Time]{Set: true, Valid: true, Value: at}, nil
}

func (self *ReleaseService) SaveRelease(ctx context.Context, actor *auth.Actor, input *SaveReleaseInput) (models.Release, error) {
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.releaseID(ctx, actor, *input.ID)
		if err != nil {
			return models.Release{}, err
		}
		existing = &id
	}
	if existing != nil && input.CreatedAt != nil {
		return models.Release{}, errdefs.BadRequest("createdAt can only be set when creating a release")
	}
	var pipeline uuid.UUID
	var err error
	switch {
	case existing == nil && input.Pipeline != nil:
		if pipeline, err = self.pipelineID(ctx, actor, *input.Pipeline); err != nil {
			return models.Release{}, err
		}
	case existing == nil:
		return models.Release{}, errdefs.BadRequest("pipeline is required to create a release")
	default:
		if pipeline, err = self.releases().ReleasePipeline(ctx, nil, *existing); err != nil {
			return models.Release{}, err
		}
		if input.Pipeline != nil {
			other, err := self.pipelineID(ctx, actor, *input.Pipeline)
			if err != nil {
				return models.Release{}, err
			}
			if other != pipeline {
				return models.Release{}, errdefs.BadRequest("A release can't move to another pipeline")
			}
		}
	}
	var name *string
	if input.Name != nil {
		n, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return models.Release{}, err
		}
		name = &n
	}
	var createdAt *time.Time
	if input.CreatedAt != nil {
		at, err := utils.ParseDatetime(*input.CreatedAt)
		if err != nil {
			return models.Release{}, err
		}
		createdAt = &at
	}
	startDate, err := parseNullable(input.StartDate, utils.ParseDate)
	if err != nil {
		return models.Release{}, err
	}
	targetDate, err := parseNullable(input.TargetDate, utils.ParseDate)
	if err != nil {
		return models.Release{}, err
	}
	startedAt, err := parseNullable(input.StartedAt, utils.ParseDatetime)
	if err != nil {
		return models.Release{}, err
	}
	completedAt, err := parseNullable(input.CompletedAt, utils.ParseDatetime)
	if err != nil {
		return models.Release{}, err
	}
	version, commitSHA := trimNullable(input.Version), trimNullable(input.CommitSHA)
	var value models.Release
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var newStage *uuid.UUID
		switch {
		case input.Stage != nil:
			stage, err := self.stage(ctx, tx, pipeline, *input.Stage)
			if err != nil {
				return err
			}
			newStage = &stage
		case existing == nil:
			stage, found, err := self.releases().FirstPlannedStage(ctx, tx, pipeline)
			if err != nil {
				return err
			}
			if !found {
				return errdefs.BadRequest("The pipeline has no stages; add one with save_release_stage")
			}
			newStage = &stage
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		var id uuid.UUID
		var before *any
		if existing != nil {
			id = *existing
			current, err := self.releaseValue(ctx, tx, id)
			if err != nil {
				return err
			}
			before = new(any(current))
		} else {
			if name == nil {
				return errdefs.BadRequest("name is required to create a release")
			}
			id, err = self.releases().CreateRelease(ctx, tx, release_repo.CreateReleaseInput{
				Workspace: actor.Workspace, Pipeline: pipeline, Slug: utils.SlugID(), Name: *name, Creator: actor.User, CreatedAt: createdAt,
			}, now)
			if err != nil {
				return err
			}
		}
		if newStage != nil {
			if err := self.releases().MoveToStage(ctx, tx, id, *newStage, now); err != nil {
				return err
			}
		}
		err = self.releases().UpdateRelease(ctx, tx, id, release_repo.ReleaseUpdate{
			Name: name, Version: version, Description: input.Description, CommitSHA: commitSHA, StartDate: startDate,
			TargetDate: targetDate, StartedAt: startedAt, CompletedAt: completedAt, Touch: existing != nil,
		}, now)
		if err != nil {
			return err
		}
		if value, err = self.releaseValue(ctx, tx, id); err != nil {
			return err
		}
		return self.emitCreatedOrChanged(ctx, tx, actor, "Release", before, value)
	})
	if err != nil {
		return models.Release{}, err
	}
	return value, nil
}

func (self *ReleaseService) DeleteRelease(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Release, error) {
	id, err := self.releaseID(ctx, actor, input.ID)
	if err != nil {
		return models.Release{}, err
	}
	var value models.Release
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.releases().ArchiveRelease(ctx, tx, id, now); err != nil {
			return err
		}
		if value, err = self.releaseValue(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Release", "remove", value, nil, nil)
	})
	if err != nil {
		return models.Release{}, err
	}
	return value, nil
}
