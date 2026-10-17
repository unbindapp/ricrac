package release_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	release_repo "ricrac/internal/repositories/release"
)

type ListNotesInput struct {
	Pipeline *string `json:"pipeline" doc:"Pipeline id, slug or name."`
	args.Page
}

type SaveNoteInput struct {
	ID               *string         `json:"id" doc:"Only to update release notes: their id or slug. Omit to create them."`
	Pipeline         *string         `json:"pipeline" doc:"The pipeline (id, slug or name). Required to create."`
	Title            *string         `json:"title" doc:"Defaults to the newest release's name and version."`
	Content          *string         `json:"content" doc:"Markdown."`
	Patch            []args.TextEdit `json:"patch" doc:"Edits to the current content instead of the whole text, applied in order, all or none."`
	Releases         []string        `json:"releases" doc:"The releases covered (id, slug or name), replacing the current ones."`
	RangeFromRelease *string         `json:"rangeFromRelease" doc:"With rangeToRelease: cover every release of the pipeline created from this one to that one, inclusive."`
	RangeToRelease   *string         `json:"rangeToRelease"`
}

func (self *ReleaseService) ListNotes(ctx context.Context, actor *auth.Actor, input *ListNotesInput) (args.PageOf[models.ReleaseNote], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.ReleaseNote]{}, err
	}
	filter := release_repo.NoteFilter{Workspace: actor.Workspace, Teams: teams}
	if input.Pipeline != nil {
		id, err := self.pipelineID(ctx, actor, *input.Pipeline)
		if err != nil {
			return args.PageOf[models.ReleaseNote]{}, err
		}
		filter.Pipeline = &id
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.ReleaseNote]{}, err
	}
	filter.Limit, filter.Offset = input.Size()+1, offset
	ids, err := self.releases().Notes(ctx, filter)
	if err != nil {
		return args.PageOf[models.ReleaseNote]{}, err
	}
	notes, err := self.noteValues(ctx, nil, ids, false)
	if err != nil {
		return args.PageOf[models.ReleaseNote]{}, err
	}
	return args.WrapOf(input.Page, notes)
}

func (self *ReleaseService) GetNote(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ReleaseNote, error) {
	id, err := self.noteID(ctx, actor, input.ID)
	if err != nil {
		return models.ReleaseNote{}, err
	}
	return self.noteValue(ctx, nil, id)
}

// coveredReleases is the releases a note covers, oldest first, checking they are in the
// pipeline; nil when neither releases nor a range was given.
func (self *ReleaseService) coveredReleases(ctx context.Context, actor *auth.Actor, pipeline uuid.UUID, input *SaveNoteInput) ([]uuid.UUID, error) {
	inPipeline := func(value string) (uuid.UUID, error) {
		id, err := self.releaseID(ctx, actor, value)
		if err != nil {
			return uuid.Nil, err
		}
		of, err := self.releases().ReleasePipeline(ctx, nil, id)
		if err != nil {
			return uuid.Nil, err
		}
		if of != pipeline {
			return uuid.Nil, errdefs.BadRequest("The release %q is in another pipeline", value)
		}
		return id, nil
	}
	if (input.RangeFromRelease == nil) != (input.RangeToRelease == nil) {
		return nil, errdefs.BadRequest("Pass both rangeFromRelease and rangeToRelease")
	}
	hasRange := input.RangeFromRelease != nil
	var from, to uuid.UUID
	if hasRange {
		var err error
		if from, err = inPipeline(*input.RangeFromRelease); err != nil {
			return nil, err
		}
		if to, err = inPipeline(*input.RangeToRelease); err != nil {
			return nil, err
		}
	}
	ids := []uuid.UUID{}
	switch {
	case input.Releases != nil && hasRange:
		return nil, errdefs.BadRequest("Pass releases or a range, not both")
	case input.Releases == nil && !hasRange:
		return nil, nil
	case input.Releases != nil:
		for _, value := range input.Releases {
			id, err := inPipeline(value)
			if err != nil {
				return nil, err
			}
			ids = utils.AppendUnique(ids, id)
		}
	default:
		var err error
		if ids, err = self.releases().ReleasesBetween(ctx, pipeline, from, to); err != nil {
			return nil, err
		}
	}
	ordered, err := self.releases().OldestFirst(ctx, ids)
	if err != nil {
		return nil, err
	}
	if ordered == nil {
		ordered = []uuid.UUID{}
	}
	return ordered, nil
}

func (self *ReleaseService) defaultNoteTitle(ctx context.Context, tx repository.TxInterface, releases []uuid.UUID) (*string, error) {
	newest, err := self.releases().Newest(ctx, tx, releases)
	if err != nil || newest == nil {
		return nil, err
	}
	if newest.Version != nil && !strings.Contains(newest.Name, *newest.Version) {
		return new(fmt.Sprintf("%s (%s)", newest.Name, *newest.Version)), nil
	}
	return &newest.Name, nil
}

func (self *ReleaseService) SaveNote(ctx context.Context, actor *auth.Actor, input *SaveNoteInput) (models.ReleaseNote, error) {
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.noteID(ctx, actor, *input.ID)
		if err != nil {
			return models.ReleaseNote{}, err
		}
		existing = &id
	}
	var pipeline uuid.UUID
	var err error
	switch {
	case existing == nil && input.Pipeline != nil:
		if pipeline, err = self.pipelineID(ctx, actor, *input.Pipeline); err != nil {
			return models.ReleaseNote{}, err
		}
	case existing == nil:
		return models.ReleaseNote{}, errdefs.BadRequest("pipeline is required to create release notes")
	default:
		if pipeline, err = self.releases().NotePipeline(ctx, *existing); err != nil {
			return models.ReleaseNote{}, err
		}
		if input.Pipeline != nil {
			other, err := self.pipelineID(ctx, actor, *input.Pipeline)
			if err != nil {
				return models.ReleaseNote{}, err
			}
			if other != pipeline {
				return models.ReleaseNote{}, errdefs.BadRequest("Release notes can't move to another pipeline")
			}
		}
	}
	releases, err := self.coveredReleases(ctx, actor, pipeline, input)
	if err != nil {
		return models.ReleaseNote{}, err
	}
	var title *string
	if input.Title != nil {
		t, err := utils.NonEmpty(*input.Title, "title")
		if err != nil {
			return models.ReleaseNote{}, err
		}
		title = &t
	}
	var value models.ReleaseNote
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var before *any
		var current string
		var err error
		if existing != nil {
			id = *existing
			note, err := self.noteValue(ctx, tx, id)
			if err != nil {
				return err
			}
			before = new(any(note))
			current = note.Content.Value
		} else {
			if title == nil {
				if title, err = self.defaultNoteTitle(ctx, tx, releases); err != nil {
					return err
				}
			}
			if title == nil {
				return errdefs.BadRequest("Pass a title, or the releases the notes cover to name them after the newest")
			}
			id, err = self.releases().CreateNote(ctx, tx, release_repo.CreateNoteInput{
				Workspace: actor.Workspace, Pipeline: pipeline, Slug: utils.SlugID(), Title: *title, Creator: actor.User,
			})
			if err != nil {
				return err
			}
		}
		content, err := args.NewContent(current, input.Content, input.Patch)
		if err != nil {
			return err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		err = self.releases().UpdateNote(ctx, tx, id, release_repo.NoteUpdate{Title: title, Content: content, Releases: releases}, now)
		if err != nil {
			return err
		}
		if value, err = self.noteValue(ctx, tx, id); err != nil {
			return err
		}
		return self.emitCreatedOrChanged(ctx, tx, actor, "ReleaseNote", before, value)
	})
	if err != nil {
		return models.ReleaseNote{}, err
	}
	return value, nil
}

func (self *ReleaseService) DeleteNote(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedReleaseNote, error) {
	id, err := self.noteID(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedReleaseNote{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.noteValue(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := self.releases().DeleteNote(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "ReleaseNote", "remove", value, nil, nil)
	})
	if err != nil {
		return models.DeletedReleaseNote{}, err
	}
	return models.DeletedReleaseNote{ID: id.String(), Deleted: true}, nil
}
