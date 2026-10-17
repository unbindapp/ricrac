package release_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/release"
	"ricrac/ent/releasepipeline"
	"ricrac/ent/releasepipelineteam"
	"ricrac/ent/releasestage"
	repository "ricrac/internal/repositories"
	resolve_repo "ricrac/internal/repositories/resolve"
)

func liveStages(q *ent.ReleaseStageQuery) {
	q.Where(releasestage.ArchivedAtIsNil()).Order(releasestage.ByPosition())
}

// LoadPipelines is the pipelines with their stages (in order) and teams, in the order of ids.
func (self *ReleaseRepository) LoadPipelines(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.ReleasePipeline, error) {
	pipelines, err := self.base.Client(tx).ReleasePipeline.Query().Where(releasepipeline.IDIn(ids...)).
		WithReleaseStages(liveStages).WithTeams().All(ctx)
	if err != nil {
		return nil, err
	}
	return inOrder(ids, pipelines, func(p *ent.ReleasePipeline) uuid.UUID { return p.ID }), nil
}

// ReleaseCounts is how many releases not archived each pipeline has.
func (self *ReleaseRepository) ReleaseCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []countRow
	err := self.base.Client(tx).Release.Query().Where(release.PipelineIDIn(ids...), release.ArchivedAtIsNil()).
		Modify(func(s *entsql.Selector) {
			s.Select(entsql.As(s.C(release.FieldPipelineID), "id"), entsql.As(entsql.Count("*"), "count")).
				GroupBy(s.C(release.FieldPipelineID))
		}).Scan(ctx, &rows)
	return countsOf(rows), err
}

type PipelineFilter struct {
	Workspace       uuid.UUID
	Teams           []uuid.UUID
	IncludeArchived bool
	// An ILIKE pattern on the name.
	Pattern      *string
	Kind         *string
	IsProduction *bool
	Team         *uuid.UUID
	Limit        int64
	Offset       int64
}

// Pipelines is the pipelines the teams see, by name.
func (self *ReleaseRepository) Pipelines(ctx context.Context, filter PipelineFilter) ([]uuid.UUID, error) {
	query := self.base.DB.ReleasePipeline.Query().
		Where(releasepipeline.WorkspaceID(filter.Workspace), pipelineVisible(filter.Teams))
	if !filter.IncludeArchived {
		query = query.Where(releasepipeline.ArchivedAtIsNil())
	}
	if filter.Pattern != nil {
		query = query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw(s.C(releasepipeline.FieldName)+" ILIKE ?", *filter.Pattern))
		})
	}
	if filter.Kind != nil {
		query = query.Where(releasepipeline.Type(*filter.Kind))
	}
	if filter.IsProduction != nil {
		query = query.Where(releasepipeline.IsProduction(*filter.IsProduction))
	}
	if filter.Team != nil {
		query = query.Where(releasepipeline.HasReleasePipelineTeamsWith(releasepipelineteam.TeamID(*filter.Team)))
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(releasepipeline.FieldID))
		s.OrderExpr(entsql.Expr("lower(" + s.C(releasepipeline.FieldName) + ")"))
		s.OrderBy(s.C(releasepipeline.FieldID))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// PipelineVisible says whether the pipeline has no teams or one of teams.
func (self *ReleaseRepository) PipelineVisible(ctx context.Context, id uuid.UUID, teams []uuid.UUID) (bool, error) {
	return self.base.DB.ReleasePipeline.Query().Where(releasepipeline.ID(id), pipelineVisible(teams)).Exist(ctx)
}

func (self *ReleaseRepository) CreatePipeline(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, slug, name string) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ReleasePipeline.Create().SetWorkspaceID(workspace).SetSlugID(slug).SetName(name).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// PipelineUpdate leaves nil fields as they are.
type PipelineUpdate struct {
	Name         *string
	Kind         *string
	IsProduction *bool
}

func (self *ReleaseRepository) UpdatePipeline(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input PipelineUpdate, now time.Time) error {
	return self.base.Client(tx).ReleasePipeline.UpdateOneID(id).SetNillableName(input.Name).SetNillableType(input.Kind).
		SetNillableIsProduction(input.IsProduction).SetUpdatedAt(now).Exec(ctx)
}

func (self *ReleaseRepository) ClearPipelineTeams(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID) error {
	_, err := self.base.Client(tx).ReleasePipelineTeam.Delete().Where(releasepipelineteam.PipelineID(pipeline)).Exec(ctx)
	return err
}

// AddPipelineTeams adds the teams the pipeline doesn't have yet.
func (self *ReleaseRepository) AddPipelineTeams(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID, teams []uuid.UUID) error {
	if len(teams) == 0 {
		return nil
	}
	client := self.base.Client(tx)
	builders := make([]*ent.ReleasePipelineTeamCreate, len(teams))
	for i, team := range teams {
		builders[i] = client.ReleasePipelineTeam.Create().SetPipelineID(pipeline).SetTeamID(team)
	}
	return client.ReleasePipelineTeam.CreateBulk(builders...).OnConflict().DoNothing().Exec(ctx)
}

func (self *ReleaseRepository) RemovePipelineTeams(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID, teams []uuid.UUID) error {
	_, err := self.base.Client(tx).ReleasePipelineTeam.Delete().
		Where(releasepipelineteam.PipelineID(pipeline), releasepipelineteam.TeamIDIn(teams...)).Exec(ctx)
	return err
}

// ArchivePipeline archives the pipeline unless it already is.
func (self *ReleaseRepository) ArchivePipeline(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).ReleasePipeline.Update().Where(releasepipeline.ID(id), releasepipeline.ArchivedAtIsNil()).
		SetArchivedAt(now).SetUpdatedAt(now).Exec(ctx)
}

// Stages is the pipeline's stages not archived, in order.
func (self *ReleaseRepository) Stages(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID) ([]resolve_repo.Typed, error) {
	stages, err := self.base.Client(tx).ReleaseStage.Query().
		Where(releasestage.PipelineID(pipeline), releasestage.ArchivedAtIsNil()).Order(releasestage.ByPosition()).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]resolve_repo.Typed, len(stages))
	for i, s := range stages {
		rows[i] = resolve_repo.Typed{ID: s.ID, Name: s.Name, Type: s.Type}
	}
	return rows, nil
}

// StagePipeline is the pipeline of the stage, if it is in the workspace.
func (self *ReleaseRepository) StagePipeline(ctx context.Context, stage, workspace uuid.UUID) (uuid.UUID, bool, error) {
	found, err := self.base.DB.ReleaseStage.Query().
		Where(releasestage.ID(stage), releasestage.HasPipelineWith(releasepipeline.WorkspaceID(workspace))).Only(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return found.PipelineID, true, nil
}

func (self *ReleaseRepository) GetStage(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.ReleaseStage, error) {
	return self.base.Client(tx).ReleaseStage.Get(ctx, id)
}

// CreateStage adds a stage at position, or after the others without one.
func (self *ReleaseRepository) CreateStage(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID, name, kind, color string, position *float64) (uuid.UUID, error) {
	client := self.base.Client(tx)
	if position == nil {
		var last []struct {
			Max *float64 `json:"max"`
		}
		err := client.ReleaseStage.Query().Where(releasestage.PipelineID(pipeline)).
			Aggregate(ent.Max(releasestage.FieldPosition)).Scan(ctx, &last)
		if err != nil {
			return uuid.Nil, err
		}
		next := 0.0
		if len(last) > 0 && last[0].Max != nil {
			next = *last[0].Max + 1
		}
		position = &next
	}
	created, err := client.ReleaseStage.Create().SetPipelineID(pipeline).SetName(name).SetType(kind).SetColor(color).
		SetPosition(*position).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// StageUpdate leaves nil fields as they are.
type StageUpdate struct {
	Name     *string
	Kind     *string
	Color    *string
	Position *float64
}

func (self *ReleaseRepository) UpdateStage(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input StageUpdate) error {
	return self.base.Client(tx).ReleaseStage.UpdateOneID(id).SetNillableName(input.Name).SetNillableType(input.Kind).
		SetNillableColor(input.Color).SetNillablePosition(input.Position).Exec(ctx)
}

// CountStages is how many stages not archived the pipeline has.
func (self *ReleaseRepository) CountStages(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID) (int, error) {
	return self.base.Client(tx).ReleaseStage.Query().Where(releasestage.PipelineID(pipeline), releasestage.ArchivedAtIsNil()).Count(ctx)
}

// CountInStage is how many releases, archived ones too, are in the stage.
func (self *ReleaseRepository) CountInStage(ctx context.Context, tx repository.TxInterface, stage uuid.UUID) (int, error) {
	return self.base.Client(tx).Release.Query().Where(release.StageID(stage)).Count(ctx)
}

// MoveReleases moves every release of a stage to another, as it is.
func (self *ReleaseRepository) MoveReleases(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Release.Update().Where(release.StageID(from)).SetStageID(to).SetUpdatedAt(now).Exec(ctx)
}

func (self *ReleaseRepository) DeleteStage(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).ReleaseStage.DeleteOneID(id).Exec(ctx)
}

// FirstPlannedStage is the first planned stage of the pipeline, else its first stage.
func (self *ReleaseRepository) FirstPlannedStage(ctx context.Context, tx repository.TxInterface, pipeline uuid.UUID) (uuid.UUID, bool, error) {
	ids, err := self.base.Client(tx).ReleaseStage.Query().
		Where(releasestage.PipelineID(pipeline), releasestage.ArchivedAtIsNil()).
		Modify(func(s *entsql.Selector) {
			s.Select(s.C(releasestage.FieldID))
			s.OrderExpr(entsql.Expr(s.C(releasestage.FieldType) + " <> 'planned'"))
			s.OrderBy(s.C(releasestage.FieldPosition))
			s.Limit(1)
		}).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}
