package release_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issue"
	"ricrac/ent/issuerelease"
	"ricrac/ent/issueshare"
	"ricrac/ent/release"
	"ricrac/ent/releasenote"
	"ricrac/ent/releasestage"
	"ricrac/ent/team"
	"ricrac/internal/common/args"
	repository "ricrac/internal/repositories"
)

// LoadReleases is the releases with their pipeline, stage and creator, in the order of ids.
func (self *ReleaseRepository) LoadReleases(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Release, error) {
	releases, err := self.base.Client(tx).Release.Query().Where(release.IDIn(ids...)).
		WithPipeline().WithStage().WithCreator().All(ctx)
	if err != nil {
		return nil, err
	}
	return inOrder(ids, releases, func(r *ent.Release) uuid.UUID { return r.ID }), nil
}

// IssueCounts is how many issues not in the trash each release has.
func (self *ReleaseRepository) IssueCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []countRow
	err := self.base.Client(tx).IssueRelease.Query().
		Where(issuerelease.ReleaseIDIn(ids...), issuerelease.HasIssueWith(issue.TrashedAtIsNil())).
		Modify(func(s *entsql.Selector) {
			s.Select(entsql.As(s.C(issuerelease.FieldReleaseID), "id"), entsql.As(entsql.Count("*"), "count")).
				GroupBy(s.C(issuerelease.FieldReleaseID))
		}).Scan(ctx, &rows)
	return countsOf(rows), err
}

type ReleaseFilter struct {
	Workspace       uuid.UUID
	Teams           []uuid.UUID
	IncludeArchived bool
	Pipeline        *uuid.UUID
	Stage           *uuid.UUID
	StageType       *string
	// ILIKE patterns on the name or version, and on the commit SHA.
	Pattern       *string
	CommitPattern *string
	// createdAt, updatedAt or targetDate.
	OrderBy string
	Limit   int64
	Offset  int64
}

// Releases is the releases of pipelines the teams see, in the order asked for.
func (self *ReleaseRepository) Releases(ctx context.Context, filter ReleaseFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Release.Query().
		Where(release.WorkspaceID(filter.Workspace), release.HasPipelineWith(pipelineVisible(filter.Teams)))
	if !filter.IncludeArchived {
		query = query.Where(release.ArchivedAtIsNil())
	}
	if filter.Pipeline != nil {
		query = query.Where(release.PipelineID(*filter.Pipeline))
	}
	if filter.Stage != nil {
		query = query.Where(release.StageID(*filter.Stage))
	}
	if filter.StageType != nil {
		query = query.Where(release.HasStageWith(releasestage.Type(*filter.StageType)))
	}
	if filter.Pattern != nil {
		query = query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw("("+s.C(release.FieldName)+" ILIKE ? OR "+s.C(release.FieldVersion)+" ILIKE ? OR "+
				s.C(release.FieldCommitSha)+" ILIKE ?)", *filter.Pattern, *filter.Pattern, *filter.CommitPattern))
		})
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(release.FieldID))
		switch filter.OrderBy {
		case "updatedAt":
			s.OrderBy(entsql.Desc(s.C(release.FieldUpdatedAt)))
		case "targetDate":
			s.OrderExpr(entsql.Expr(s.C(release.FieldTargetDate) + " ASC NULLS LAST"))
			s.OrderBy(entsql.Desc(s.C(release.FieldCreatedAt)))
		default:
			s.OrderBy(entsql.Desc(s.C(release.FieldCreatedAt)))
		}
		s.OrderBy(s.C(release.FieldID))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

func (self *ReleaseRepository) ReleasePipeline(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (uuid.UUID, error) {
	found, err := self.base.Client(tx).Release.Get(ctx, id)
	if err != nil {
		return uuid.Nil, err
	}
	return found.PipelineID, nil
}

type CreateReleaseInput struct {
	Workspace uuid.UUID
	Pipeline  uuid.UUID
	Slug      string
	Name      string
	Creator   uuid.UUID
	// When it was created, for imports; now without.
	CreatedAt *time.Time
}

func (self *ReleaseRepository) CreateRelease(ctx context.Context, tx repository.TxInterface, input CreateReleaseInput, now time.Time) (uuid.UUID, error) {
	createdAt := now
	if input.CreatedAt != nil {
		createdAt = *input.CreatedAt
	}
	created, err := self.base.Client(tx).Release.Create().SetWorkspaceID(input.Workspace).SetPipelineID(input.Pipeline).
		SetSlugID(input.Slug).SetName(input.Name).SetCreatorID(input.Creator).SetCreatedAt(createdAt).SetUpdatedAt(createdAt).
		Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// MoveToStage moves a release to a stage, stamping or clearing startedAt, completedAt and
// canceledAt by the stage's type. Staying in the same stage changes nothing.
func (self *ReleaseRepository) MoveToStage(ctx context.Context, tx repository.TxInterface, id, stage uuid.UUID, now time.Time) error {
	client := self.base.Client(tx)
	current, err := client.Release.Get(ctx, id)
	if err != nil {
		return err
	}
	target, err := client.ReleaseStage.Get(ctx, stage)
	if err != nil {
		return err
	}
	if current.StageID != nil && *current.StageID == stage {
		return nil
	}
	update := client.Release.UpdateOne(current).SetStageID(stage)
	switch target.Type {
	case "planned":
		update.ClearStartedAt()
	case "started", "completed":
		update.SetStartedAt(orNow(current.StartedAt, now))
	}
	setOrClear(stampedIf(target.Type == "completed", current.CompletedAt, now), update.SetCompletedAt, update.ClearCompletedAt)
	setOrClear(stampedIf(target.Type == "canceled", current.CanceledAt, now), update.SetCanceledAt, update.ClearCanceledAt)
	return update.Exec(ctx)
}

func orNow(value *time.Time, now time.Time) time.Time {
	if value != nil {
		return *value
	}
	return now
}

// stampedIf is the time kept (or now) when stamped, else null.
func stampedIf(stamped bool, value *time.Time, now time.Time) args.Nullable[time.Time] {
	if !stamped {
		return args.Nullable[time.Time]{Set: true}
	}
	return args.Nullable[time.Time]{Set: true, Valid: true, Value: orNow(value, now)}
}

// ReleaseUpdate leaves nil fields and fields not Set as they are.
type ReleaseUpdate struct {
	Name        *string
	Version     args.Nullable[string]
	Description args.Nullable[string]
	CommitSHA   args.Nullable[string]
	StartDate   args.Nullable[time.Time]
	TargetDate  args.Nullable[time.Time]
	StartedAt   args.Nullable[time.Time]
	CompletedAt args.Nullable[time.Time]
	// Touch sets updatedAt to now.
	Touch bool
}

func (self *ReleaseRepository) UpdateRelease(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input ReleaseUpdate, now time.Time) error {
	update := self.base.Client(tx).Release.UpdateOneID(id).SetNillableName(input.Name)
	setOrClear(input.Version, update.SetVersion, update.ClearVersion)
	setOrClear(input.Description, update.SetDescription, update.ClearDescription)
	setOrClear(input.CommitSHA, update.SetCommitSha, update.ClearCommitSha)
	setOrClear(input.StartDate, update.SetStartDate, update.ClearStartDate)
	setOrClear(input.TargetDate, update.SetTargetDate, update.ClearTargetDate)
	setOrClear(input.StartedAt, update.SetStartedAt, update.ClearStartedAt)
	setOrClear(input.CompletedAt, update.SetCompletedAt, update.ClearCompletedAt)
	if input.Touch {
		update.SetUpdatedAt(now)
	}
	return update.Exec(ctx)
}

// ArchiveRelease archives the release unless it already is.
func (self *ReleaseRepository) ArchiveRelease(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Release.Update().Where(release.ID(id), release.ArchivedAtIsNil()).
		SetArchivedAt(now).SetUpdatedAt(now).Exec(ctx)
}

// ReleaseIssues is the issues not in the trash in the release that are in one of teams or
// shared with user, with their team, state and assignee, by identifier.
func (self *ReleaseRepository) ReleaseIssues(ctx context.Context, id uuid.UUID, teams []uuid.UUID, user uuid.UUID) ([]*ent.Issue, error) {
	return self.base.DB.Issue.Query().
		Where(issue.HasIssueReleasesWith(issuerelease.ReleaseID(id)), issue.TrashedAtIsNil(),
			issue.Or(issue.TeamIDIn(teams...), issue.HasIssueSharesWith(issueshare.UserID(user)))).
		WithTeam().WithState().WithAssignee().
		Order(issue.ByTeamField(team.FieldKey), issue.ByNumber()).
		All(ctx)
}

// NotesCovering is the release notes that cover the release, newest first.
func (self *ReleaseRepository) NotesCovering(ctx context.Context, id uuid.UUID) ([]*ent.ReleaseNote, error) {
	return self.base.DB.ReleaseNote.Query().
		Where(func(s *entsql.Selector) {
			s.Where(repository.Raw("? = ANY("+s.C(releasenote.FieldReleaseIds)+")", id))
		}).
		Order(releasenote.ByCreatedAt(entsql.OrderDesc())).All(ctx)
}

// setOrClear sets a value given, clears a null one and leaves one left out.
func setOrClear[T, U any](value args.Nullable[T], set func(T) U, clear func() U) {
	switch {
	case !value.Set:
	case value.Valid:
		set(value.Value)
	default:
		clear()
	}
}
