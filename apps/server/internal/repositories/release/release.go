package release_repo

import (
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/predicate"
	"ricrac/ent/releasepipeline"
	"ricrac/ent/releasepipelineteam"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ReleaseRepositoryInterface -p release_repo -s ReleaseRepository -o release_repository_iface.go
type ReleaseRepository struct {
	base *repository.BaseRepository
}

func NewReleaseRepository(db *ent.Client) *ReleaseRepository {
	return &ReleaseRepository{base: repository.NewBaseRepository(db)}
}

// pipelineVisible is a pipeline without teams or with one of teams.
func pipelineVisible(teams []uuid.UUID) predicate.ReleasePipeline {
	return releasepipeline.Or(
		releasepipeline.Not(releasepipeline.HasReleasePipelineTeams()),
		releasepipeline.HasReleasePipelineTeamsWith(releasepipelineteam.TeamIDIn(teams...)),
	)
}

func inOrder[T any](ids []uuid.UUID, items []T, id func(T) uuid.UUID) []T {
	byID := make(map[uuid.UUID]T, len(items))
	for _, item := range items {
		byID[id(item)] = item
	}
	ordered := make([]T, 0, len(ids))
	for _, key := range ids {
		if item, ok := byID[key]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

type countRow struct {
	ID    uuid.UUID `json:"id"`
	Count int       `json:"count"`
}

func countsOf(rows []countRow) map[uuid.UUID]int {
	counts := map[uuid.UUID]int{}
	for _, row := range rows {
		counts[row.ID] = row.Count
	}
	return counts
}
