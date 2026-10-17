package release_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	release_repo "ricrac/internal/repositories/release"
	resolve_service "ricrac/internal/services/resolve"
)

type ListPipelinesInput struct {
	Query           *string `json:"query" doc:"Words in the name."`
	Kind            *string `json:"type" doc:"continuous or scheduled."`
	IsProduction    *bool   `json:"isProduction"`
	Team            *string `json:"team" doc:"Pipelines of this team (key, name or id)."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	IncludeStages   bool    `json:"includeStages,omitempty" doc:"Include each pipeline's stages."`
	IncludeTeams    bool    `json:"includeTeams,omitempty" doc:"Include each pipeline's teams."`
	args.Page
}

type SavePipelineInput struct {
	ID           *string  `json:"id" doc:"Only to update a pipeline: its id, slug or name. Omit to create one."`
	Name         *string  `json:"name" doc:"Required to create."`
	Kind         *string  `json:"type" doc:"continuous (default: every deploy is a release) or scheduled (releases planned ahead)."`
	IsProduction *bool    `json:"isProduction" doc:"Whether it deploys to production."`
	AddTeams     []string `json:"addTeams" doc:"Teams to add (key, name or id)."`
	RemoveTeams  []string `json:"removeTeams"`
	SetTeams     []string `json:"setTeams" doc:"Replace all teams with these."`
}

type SaveStageInput struct {
	ID       *string  `json:"id" doc:"Only to update a stage: its id. Omit to create one."`
	Pipeline *string  `json:"pipeline" doc:"The pipeline (id, slug or name). Required to create."`
	Name     *string  `json:"name" doc:"Required to create."`
	Kind     *string  `json:"type" doc:"planned, started, completed or canceled. Required to create."`
	Color    *string  `json:"color" doc:"Hex color like #f2c94c."`
	Position *float64 `json:"position" doc:"Order in the pipeline, lowest first. New stages go last."`
}

type DeleteStageInput struct {
	ID             string  `json:"id" doc:"The stage's id."`
	MoveReleasesTo *string `json:"moveReleasesTo" doc:"Where its releases go: another stage of the pipeline (name, type or id). Required when releases are in it."`
}

func (self *ReleaseService) ListPipelines(ctx context.Context, actor *auth.Actor, input *ListPipelinesInput) (args.PageOf[models.ReleasePipeline], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.ReleasePipeline]{}, err
	}
	filter := release_repo.PipelineFilter{Workspace: actor.Workspace, Teams: teams, IncludeArchived: input.IncludeArchived}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Pattern = new("%" + utils.EscapeLike(query) + "%")
	}
	if input.Kind != nil {
		kind, err := oneOf(*input.Kind, pipelineTypes, "type")
		if err != nil {
			return args.PageOf[models.ReleasePipeline]{}, err
		}
		filter.Kind = &kind
	}
	filter.IsProduction = input.IsProduction
	if input.Team != nil {
		team, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return args.PageOf[models.ReleasePipeline]{}, err
		}
		filter.Team = &team
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.ReleasePipeline]{}, err
	}
	filter.Limit, filter.Offset = input.Size()+1, offset
	ids, err := self.releases().Pipelines(ctx, filter)
	if err != nil {
		return args.PageOf[models.ReleasePipeline]{}, err
	}
	pipelines, err := self.pipelineValues(ctx, nil, ids, input.IncludeStages, input.IncludeTeams)
	if err != nil {
		return args.PageOf[models.ReleasePipeline]{}, err
	}
	return args.WrapOf(input.Page, pipelines)
}

func (self *ReleaseService) GetPipeline(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ReleasePipeline, error) {
	id, err := self.pipelineID(ctx, actor, input.ID)
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	return self.pipelineValue(ctx, nil, id)
}

// teamList is nil when the list was left out.
func (self *ReleaseService) teamList(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}
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

func (self *ReleaseService) SavePipeline(ctx context.Context, actor *auth.Actor, input *SavePipelineInput) (models.ReleasePipeline, error) {
	if input.SetTeams != nil && (input.AddTeams != nil || input.RemoveTeams != nil) {
		return models.ReleasePipeline{}, errdefs.BadRequest("Pass setTeams or addTeams/removeTeams, not both")
	}
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.pipelineID(ctx, actor, *input.ID)
		if err != nil {
			return models.ReleasePipeline{}, err
		}
		existing = &id
	}
	var name, kind *string
	if input.Name != nil {
		n, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return models.ReleasePipeline{}, err
		}
		name = &n
	}
	if input.Kind != nil {
		k, err := oneOf(*input.Kind, pipelineTypes, "type")
		if err != nil {
			return models.ReleasePipeline{}, err
		}
		kind = &k
	}
	set, err := self.teamList(ctx, actor, input.SetTeams)
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	add, err := self.teamList(ctx, actor, input.AddTeams)
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	remove, err := self.teamList(ctx, actor, input.RemoveTeams)
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	var value models.ReleasePipeline
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var before *any
		if existing != nil {
			id = *existing
			current, err := self.pipelineValue(ctx, tx, id)
			if err != nil {
				return err
			}
			before = new(any(current))
		} else {
			if name == nil {
				return errdefs.BadRequest("name is required to create a pipeline")
			}
			var err error
			if id, err = self.releases().CreatePipeline(ctx, tx, actor.Workspace, utils.SlugID(), *name); err != nil {
				return err
			}
			for position, stage := range defaultStages {
				if _, err := self.releases().CreateStage(ctx, tx, id, stage[0], stage[1], stage[2], new(float64(position))); err != nil {
					return err
				}
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		err = self.releases().UpdatePipeline(ctx, tx, id, release_repo.PipelineUpdate{Name: name, Kind: kind, IsProduction: input.IsProduction}, now)
		if err != nil {
			return err
		}
		teams := add
		if set != nil {
			if err := self.releases().ClearPipelineTeams(ctx, tx, id); err != nil {
				return err
			}
			teams = set
		}
		if teams != nil {
			if err := self.releases().AddPipelineTeams(ctx, tx, id, teams); err != nil {
				return err
			}
		}
		if remove != nil {
			if err := self.releases().RemovePipelineTeams(ctx, tx, id, remove); err != nil {
				return err
			}
		}
		if value, err = self.pipelineValue(ctx, tx, id); err != nil {
			return err
		}
		return self.emitCreatedOrChanged(ctx, tx, actor, "ReleasePipeline", before, value)
	})
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	return value, nil
}

func (self *ReleaseService) DeletePipeline(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ReleasePipeline, error) {
	id, err := self.pipelineID(ctx, actor, input.ID)
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	var value models.ReleasePipeline
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.releases().ArchivePipeline(ctx, tx, id, now); err != nil {
			return err
		}
		if value, err = self.pipelineValue(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "ReleasePipeline", "remove", value, nil, nil)
	})
	if err != nil {
		return models.ReleasePipeline{}, err
	}
	return value, nil
}

// stage is a stage of the pipeline by id, name or type (the first of that type).
func (self *ReleaseService) stage(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID, value string) (uuid.UUID, error) {
	rows, err := self.releases().Stages(ctx, tx, pipeline)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, found := resolve_service.FindTyped(rows, value); found {
		return id, nil
	}
	return uuid.Nil, errdefs.BadRequest("The pipeline has no stage %q. Options: %s.", value, strings.Join(resolve_service.TypedNames(rows), ", "))
}

// stageByID is a stage by id, with its pipeline, checking the caller sees the pipeline.
func (self *ReleaseService) stageByID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, uuid.UUID, error) {
	notFound := errdefs.NotFound("No release stage %q; pass its id", strings.TrimSpace(value))
	id, ok := utils.AsUUID(value)
	if !ok {
		return uuid.Nil, uuid.Nil, notFound
	}
	pipeline, found, err := self.releases().StagePipeline(ctx, id, actor.Workspace)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !found {
		return uuid.Nil, uuid.Nil, notFound
	}
	return id, pipeline, self.checkPipelineVisible(ctx, actor, pipeline, "release stage", value)
}

func (self *ReleaseService) SaveStage(ctx context.Context, actor *auth.Actor, input *SaveStageInput) (models.ReleaseStageDetail, error) {
	var name, kind, color *string
	if input.Name != nil {
		n, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return models.ReleaseStageDetail{}, err
		}
		name = &n
	}
	if input.Kind != nil {
		k, err := oneOf(*input.Kind, stageTypes, "type")
		if err != nil {
			return models.ReleaseStageDetail{}, err
		}
		kind = &k
	}
	if input.Color != nil {
		checked, err := utils.CheckColor(*input.Color)
		if err != nil {
			return models.ReleaseStageDetail{}, err
		}
		color = &checked
	}
	var existing *uuid.UUID
	var pipeline uuid.UUID
	switch {
	case input.ID != nil:
		id, of, err := self.stageByID(ctx, actor, *input.ID)
		if err != nil {
			return models.ReleaseStageDetail{}, err
		}
		if input.Pipeline != nil {
			other, err := self.pipelineID(ctx, actor, *input.Pipeline)
			if err != nil {
				return models.ReleaseStageDetail{}, err
			}
			if other != of {
				return models.ReleaseStageDetail{}, errdefs.BadRequest("A stage can't move to another pipeline")
			}
		}
		existing, pipeline = &id, of
	case input.Pipeline != nil:
		var err error
		if pipeline, err = self.pipelineID(ctx, actor, *input.Pipeline); err != nil {
			return models.ReleaseStageDetail{}, err
		}
	default:
		return models.ReleaseStageDetail{}, errdefs.BadRequest("pipeline is required to create a stage")
	}
	var value models.ReleaseStageDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		if existing != nil {
			id = *existing
		} else {
			if name == nil {
				return errdefs.BadRequest("name is required to create a stage")
			}
			if kind == nil {
				return errdefs.BadRequest("type is required to create a stage")
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
			var err error
			if id, err = self.releases().CreateStage(ctx, tx, pipeline, *name, *kind, newColor, nil); err != nil {
				return err
			}
		}
		err := self.releases().UpdateStage(ctx, tx, id, release_repo.StageUpdate{Name: name, Kind: kind, Color: color, Position: input.Position})
		if err != nil {
			return err
		}
		stage, err := self.releases().GetStage(ctx, tx, id)
		if err != nil {
			return err
		}
		pipelines, err := self.releases().LoadPipelines(ctx, tx, []uuid.UUID{pipeline})
		if err != nil {
			return err
		}
		if len(pipelines) == 0 {
			return errdefs.NotFound("No such release pipeline")
		}
		value = models.ReleaseStageDetail{ReleaseStage: models.ReleaseStageOf(stage), Pipeline: *models.ReleasePipelineRefOf(pipelines[0])}
		return nil
	})
	if err != nil {
		return models.ReleaseStageDetail{}, err
	}
	return value, nil
}

func (self *ReleaseService) DeleteStage(ctx context.Context, actor *auth.Actor, input *DeleteStageInput) (models.DeletedReleaseStage, error) {
	id, pipeline, err := self.stageByID(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedReleaseStage{}, err
	}
	var moved int
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		stages, err := self.releases().CountStages(ctx, tx, pipeline)
		if err != nil {
			return err
		}
		releases, err := self.releases().CountInStage(ctx, tx, id)
		if err != nil {
			return err
		}
		if stages <= 1 {
			return errdefs.BadRequest("A pipeline needs at least one stage")
		}
		switch {
		case input.MoveReleasesTo != nil:
			target, err := self.stage(ctx, tx, pipeline, *input.MoveReleasesTo)
			if err != nil {
				return err
			}
			if target == id {
				return errdefs.BadRequest("moveReleasesTo must be another stage")
			}
			now, err := self.repo.Now(ctx, tx)
			if err != nil {
				return err
			}
			if err := self.releases().MoveReleases(ctx, tx, id, target, now); err != nil {
				return err
			}
			moved = releases
		case releases > 0:
			return errdefs.BadRequest("%d release(s) are in this stage; pass moveReleasesTo with another stage of the pipeline", releases)
		}
		return self.releases().DeleteStage(ctx, tx, id)
	})
	if err != nil {
		return models.DeletedReleaseStage{}, err
	}
	return models.DeletedReleaseStage{ID: id.String(), Deleted: true, ReleasesMoved: moved}, nil
}
