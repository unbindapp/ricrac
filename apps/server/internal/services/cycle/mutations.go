package cycle_service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	cycle_repo "ricrac/internal/repositories/cycle"
)

func (self *CycleService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.Cycle, error) {
	var value models.Cycle
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		action := "create"
		if input.ID != nil {
			action = "update"
			if id, err = self.find(ctx, actor, input.Team, *input.ID); err != nil {
				return err
			}
			err = self.update(ctx, tx, id, input)
		} else {
			id, err = self.create(ctx, tx, actor, input)
		}
		if err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Cycle", action, value, nil, teamID(value))
	})
	return value, err
}

func teamID(value models.Cycle) *uuid.UUID {
	id, err := uuid.Parse(value.Team.ID)
	if err != nil {
		return nil
	}
	return &id
}

// chronoRFC3339 writes a time as chrono does: +00:00 and as many fractional digits as it needs.
func chronoRFC3339(at time.Time) string {
	at = at.UTC()
	nanos := at.Nanosecond()
	layout := "2006-01-02T15:04:05.000000000"
	switch {
	case nanos == 0:
		layout = "2006-01-02T15:04:05"
	case nanos%1_000_000 == 0:
		layout = "2006-01-02T15:04:05.000"
	case nanos%1_000 == 0:
		layout = "2006-01-02T15:04:05.000000"
	}
	return at.Format(layout) + "+00:00"
}

func (self *CycleService) checkNoOverlap(ctx context.Context, tx repository.TxInterface, team uuid.UUID, except *uuid.UUID, starts, ends time.Time) error {
	if !ends.After(starts) {
		return errdefs.BadRequest("endsAt must be after startsAt")
	}
	clash, err := self.repo.Cycle().Overlapping(ctx, tx, team, except, starts, ends)
	if err != nil {
		return err
	}
	if clash != nil {
		return errdefs.BadRequest("Those dates overlap cycle %d (%s to %s); a team's cycles can't overlap",
			clash.Number, chronoRFC3339(clash.StartsAt), chronoRFC3339(clash.EndsAt))
	}
	return nil
}

func (self *CycleService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	team, err := self.resolve.TeamOrDefault(ctx, actor, input.Team)
	if err != nil {
		return uuid.Nil, err
	}
	if input.StartsAt == nil || input.EndsAt == nil {
		return uuid.Nil, errdefs.BadRequest("startsAt and endsAt are required to create a cycle")
	}
	starts, err := utils.ParseDatetime(*input.StartsAt)
	if err != nil {
		return uuid.Nil, err
	}
	ends, err := utils.ParseDatetime(*input.EndsAt)
	if err != nil {
		return uuid.Nil, err
	}
	enabled, err := self.repo.Cycle().TeamUsesCycles(ctx, tx, team)
	if err != nil {
		return uuid.Nil, err
	}
	if !enabled {
		return uuid.Nil, errdefs.BadRequest("This team doesn't use cycles; turn them on with save_team (cyclesEnabled) first")
	}
	if err := self.checkNoOverlap(ctx, tx, team, nil, starts, ends); err != nil {
		return uuid.Nil, err
	}
	return self.repo.Cycle().Create(ctx, tx, cycle_repo.CreateInput{
		Team: team, Name: utils.Trimmed(input.Name.Ptr()), Description: utils.Trimmed(input.Description.Ptr()), Starts: starts, Ends: ends,
	})
}

func (self *CycleService) update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input *SaveInput) error {
	current, err := self.repo.Cycle().GetForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	if input.StartsAt != nil || input.EndsAt != nil {
		if current.CompletedAt != nil {
			return errdefs.BadRequest("A completed cycle's dates can't change")
		}
		starts, ends := current.StartsAt, current.EndsAt
		if input.StartsAt != nil {
			if starts, err = utils.ParseDatetime(*input.StartsAt); err != nil {
				return err
			}
		}
		if input.EndsAt != nil {
			if ends, err = utils.ParseDatetime(*input.EndsAt); err != nil {
				return err
			}
		}
		if err := self.checkNoOverlap(ctx, tx, current.TeamID, &id, starts, ends); err != nil {
			return err
		}
		if err := self.repo.Cycle().SetDates(ctx, tx, id, starts, ends); err != nil {
			return err
		}
	}
	if input.Name.Set {
		if err := self.repo.Cycle().SetName(ctx, tx, id, utils.Trimmed(input.Name.Ptr())); err != nil {
			return err
		}
	}
	if input.Description.Set {
		if err := self.repo.Cycle().SetDescription(ctx, tx, id, utils.Trimmed(input.Description.Ptr())); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Cycle().Touch(ctx, tx, id, now)
}

func (self *CycleService) Archive(ctx context.Context, actor *auth.Actor, input *ArchiveInput) (models.Cycle, error) {
	id, err := self.find(ctx, actor, input.Team, input.ID)
	if err != nil {
		return models.Cycle{}, err
	}
	var value models.Cycle
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		current, err := self.repo.Cycle().GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if current.ArchivedAt == nil {
			if err := self.archive(ctx, tx, actor, id, current.StartsAt.After(now), current.CompletedAt != nil || !current.EndsAt.After(now), now); err != nil {
				return err
			}
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Cycle", "update", value, nil, &current.TeamID)
	})
	return value, err
}

// archive archives an upcoming or completed cycle; an upcoming one's issues leave it.
func (self *CycleService) archive(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, future, done bool, now time.Time) error {
	if !future && !done {
		return errdefs.BadRequest("The current cycle can't be archived; wait until it is completed")
	}
	if future {
		err := self.repo.Cycle().MoveIssues(ctx, tx, cycle_repo.MoveInput{From: id, Action: "updated", Actor: &actor.User, Now: now})
		if err != nil {
			return err
		}
	}
	return self.repo.Cycle().Archive(ctx, tx, id, now)
}
