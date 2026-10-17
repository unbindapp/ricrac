package release_repo

import (
	"context"
	"slices"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/release"
	"ricrac/ent/releasenote"
	"ricrac/ent/schema/pgarray"
	repository "ricrac/internal/repositories"
)

// LoadNotes is the release notes with their pipeline and creator, in the order of ids.
func (self *ReleaseRepository) LoadNotes(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.ReleaseNote, error) {
	notes, err := self.base.Client(tx).ReleaseNote.Query().Where(releasenote.IDIn(ids...)).
		WithPipeline().WithCreator().All(ctx)
	if err != nil {
		return nil, err
	}
	return inOrder(ids, notes, func(n *ent.ReleaseNote) uuid.UUID { return n.ID }), nil
}

// CoveredReleases is the releases each note covers, oldest first.
func (self *ReleaseRepository) CoveredReleases(ctx context.Context, tx repository.TxInterface, notes []*ent.ReleaseNote) (map[uuid.UUID][]*ent.Release, error) {
	var ids []uuid.UUID
	for _, n := range notes {
		ids = append(ids, n.ReleaseIds...)
	}
	releases, err := self.base.Client(tx).Release.Query().Where(release.IDIn(ids...)).
		Order(release.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, err
	}
	covered := map[uuid.UUID][]*ent.Release{}
	for _, n := range notes {
		for _, r := range releases {
			if slices.Contains(n.ReleaseIds, r.ID) {
				covered[n.ID] = append(covered[n.ID], r)
			}
		}
	}
	return covered, nil
}

type NoteFilter struct {
	Workspace uuid.UUID
	Teams     []uuid.UUID
	Pipeline  *uuid.UUID
	Limit     int64
	Offset    int64
}

// Notes is the release notes of pipelines the teams see, newest first.
func (self *ReleaseRepository) Notes(ctx context.Context, filter NoteFilter) ([]uuid.UUID, error) {
	query := self.base.DB.ReleaseNote.Query().
		Where(releasenote.WorkspaceID(filter.Workspace), releasenote.HasPipelineWith(pipelineVisible(filter.Teams)))
	if filter.Pipeline != nil {
		query = query.Where(releasenote.PipelineID(*filter.Pipeline))
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(releasenote.FieldID))
		s.OrderBy(entsql.Desc(s.C(releasenote.FieldCreatedAt)), s.C(releasenote.FieldID))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// NoteByKey is the note of the workspace with this id or slug.
func (self *ReleaseRepository) NoteByKey(ctx context.Context, workspace uuid.UUID, id *uuid.UUID, slug string) (*ent.ReleaseNote, error) {
	match := releasenote.SlugID(slug)
	if id != nil {
		match = releasenote.Or(releasenote.ID(*id), match)
	}
	found, err := self.base.DB.ReleaseNote.Query().Where(releasenote.WorkspaceID(workspace), match).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// ReleasesBetween is the releases of the pipeline created from one to the other (in either
// order), inclusive, oldest first; archived ones only when they are one of the two.
func (self *ReleaseRepository) ReleasesBetween(ctx context.Context, pipeline, from, to uuid.UUID) ([]uuid.UUID, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		`WITH bounds AS (SELECT min(created_at) AS low, max(created_at) AS high FROM releases WHERE id IN ($2, $3))
		 SELECT r.id FROM releases r, bounds b
		 WHERE r.pipeline_id = $1 AND r.created_at BETWEEN b.low AND b.high
			AND (r.archived_at IS NULL OR r.id IN ($2, $3))
		 ORDER BY r.created_at`, pipeline, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// OldestFirst is the releases by creation, then name.
func (self *ReleaseRepository) OldestFirst(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return self.base.DB.Release.Query().Where(release.IDIn(ids...)).
		Order(release.ByCreatedAt(), release.ByName()).IDs(ctx)
}

// Newest is the newest of the releases.
func (self *ReleaseRepository) Newest(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (*ent.Release, error) {
	found, err := self.base.Client(tx).Release.Query().Where(release.IDIn(ids...)).
		Order(release.ByCreatedAt(entsql.OrderDesc())).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (self *ReleaseRepository) NotePipeline(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	found, err := self.base.DB.ReleaseNote.Get(ctx, id)
	if err != nil {
		return uuid.Nil, err
	}
	return found.PipelineID, nil
}

type CreateNoteInput struct {
	Workspace uuid.UUID
	Pipeline  uuid.UUID
	Slug      string
	Title     string
	Creator   uuid.UUID
}

func (self *ReleaseRepository) CreateNote(ctx context.Context, tx repository.TxInterface, input CreateNoteInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ReleaseNote.Create().SetWorkspaceID(input.Workspace).SetPipelineID(input.Pipeline).
		SetSlugID(input.Slug).SetTitle(input.Title).SetCreatorID(input.Creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// NoteUpdate leaves nil fields as they are.
type NoteUpdate struct {
	Title    *string
	Content  *string
	Releases []uuid.UUID
}

func (self *ReleaseRepository) UpdateNote(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input NoteUpdate, now time.Time) error {
	update := self.base.Client(tx).ReleaseNote.UpdateOneID(id).SetNillableTitle(input.Title).SetNillableContent(input.Content).
		SetUpdatedAt(now)
	if input.Releases != nil {
		update.SetReleaseIds(pgarray.UUID(input.Releases))
	}
	return update.Exec(ctx)
}

func (self *ReleaseRepository) DeleteNote(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).ReleaseNote.DeleteOneID(id).Exec(ctx)
}
