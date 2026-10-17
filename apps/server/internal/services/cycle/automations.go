package cycle_service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
	cycle_repo "ricrac/internal/repositories/cycle"
)

// RunAutomations creates upcoming cycles, completes ended ones (rolling unfinished issues over)
// and records each running cycle's daily progress. Runs every minute.
func (self *CycleService) RunAutomations(ctx context.Context) error {
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		locked, err := self.repo.Cycle().LockAutomations(ctx, tx)
		if err != nil || !locked {
			return err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		today := now.Format(time.DateOnly)
		teams, err := self.repo.Cycle().TeamsWithCycles(ctx, tx)
		if err != nil {
			return err
		}
		ids := []uuid.UUID{}
		for _, team := range teams {
			if err := self.createCycles(ctx, tx, team, now); err != nil {
				return err
			}
			if err := self.completeEnded(ctx, tx, team, today, now); err != nil {
				return err
			}
			ids = append(ids, team.ID)
		}
		return self.repo.Cycle().RecordProgress(ctx, tx, ids, today)
	})
}

// firstStart is 00:00 UTC of the most recent startDay (0 = Sunday) on or before today.
func firstStart(today time.Time, startDay int) time.Time {
	back := ((int(today.Weekday())-startDay)%7 + 7) % 7
	return time.Date(today.Year(), today.Month(), today.Day()-back, 0, 0, 0, 0, time.UTC)
}

const week = 7 * 24 * time.Hour

// createCycles adds cycles after the team's last one until there is a current one (unless it is
// cooling down) and as many upcoming ones as the team wants.
func (self *CycleService) createCycles(ctx context.Context, tx repository.TxInterface, team *ent.Team, now time.Time) error {
	duration := time.Duration(max(team.CycleDurationWeeks, 1)) * week
	step := duration + time.Duration(max(team.CycleCooldownWeeks, 0))*week
	end, number, upcoming, err := self.repo.Cycle().Latest(ctx, tx, team.ID, now)
	if err != nil {
		return err
	}
	start := firstStart(now, team.CycleStartDay)
	if end != nil {
		start = end.Add(step - duration)
	}
	for !start.Add(duration).After(now) {
		start = start.Add(step)
	}
	for {
		future := start.After(now)
		if future && upcoming >= max(team.UpcomingCycleCount, 0) {
			return nil
		}
		number++
		if err := self.repo.Cycle().CreateNumbered(ctx, tx, team.ID, number, start, start.Add(duration)); err != nil {
			return err
		}
		if future {
			upcoming++
		}
		start = start.Add(step)
	}
}

// completeEnded completes the team's ended cycles: unfinished issues move to the next cycle (or
// out of any cycle when the team doesn't roll over), with a last snapshot of the counts first.
func (self *CycleService) completeEnded(ctx context.Context, tx repository.TxInterface, team *ent.Team, today string, now time.Time) error {
	cycles, err := self.repo.Cycle().Ended(ctx, tx, team.ID)
	if err != nil {
		return err
	}
	for _, cycle := range cycles {
		var next *uuid.UUID
		if team.CycleRollover {
			if next, err = self.repo.Cycle().NextAfter(ctx, tx, team.ID, cycle.EndsAt); err != nil {
				return err
			}
		}
		if err := self.repo.Cycle().Complete(ctx, tx, cycle.ID, today); err != nil {
			return err
		}
		err := self.repo.Cycle().MoveIssues(ctx, tx, cycle_repo.MoveInput{
			From: cycle.ID, To: next, UnfinishedOnly: true, Action: "cycle_rollover", Now: now,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
