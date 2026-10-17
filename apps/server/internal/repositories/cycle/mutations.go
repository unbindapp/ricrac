package cycle_repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/issue"
	"ricrac/ent/workflowstate"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

type CreateInput struct {
	Team        uuid.UUID
	Name        *string
	Description *string
	Starts      time.Time
	Ends        time.Time
}

// Create adds a cycle numbered after the team's last one.
func (self *CycleRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	number, err := self.LastNumber(ctx, tx, input.Team)
	if err != nil {
		return uuid.Nil, err
	}
	created, err := self.base.Client(tx).Cycle.Create().SetTeamID(input.Team).SetNumber(number + 1).
		SetNillableName(input.Name).SetNillableDescription(input.Description).
		SetStartsAt(input.Starts).SetEndsAt(input.Ends).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// CreateNumbered adds a cycle with its number.
func (self *CycleRepository) CreateNumbered(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, number int, starts, ends time.Time) error {
	return self.base.Client(tx).Cycle.Create().SetTeamID(teamID).SetNumber(number).SetStartsAt(starts).SetEndsAt(ends).Exec(ctx)
}

// LastNumber is the number of the team's last cycle, 0 when it has none.
func (self *CycleRepository) LastNumber(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID) (int, error) {
	var rows []struct {
		Max *int `json:"max"`
	}
	err := self.base.Client(tx).Cycle.Query().Where(cycle.TeamID(teamID)).Aggregate(ent.Max(cycle.FieldNumber)).Scan(ctx, &rows)
	if err != nil || len(rows) == 0 || rows[0].Max == nil {
		return 0, err
	}
	return *rows[0].Max, nil
}

func (self *CycleRepository) SetDates(ctx context.Context, tx repository.TxInterface, id uuid.UUID, starts, ends time.Time) error {
	return self.base.Client(tx).Cycle.UpdateOneID(id).SetStartsAt(starts).SetEndsAt(ends).Exec(ctx)
}

func (self *CycleRepository) SetName(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name *string) error {
	update := self.base.Client(tx).Cycle.UpdateOneID(id)
	if name == nil {
		return update.ClearName().Exec(ctx)
	}
	return update.SetName(*name).Exec(ctx)
}

func (self *CycleRepository) SetDescription(ctx context.Context, tx repository.TxInterface, id uuid.UUID, description *string) error {
	update := self.base.Client(tx).Cycle.UpdateOneID(id)
	if description == nil {
		return update.ClearDescription().Exec(ctx)
	}
	return update.SetDescription(*description).Exec(ctx)
}

func (self *CycleRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Cycle.UpdateOneID(id).SetUpdatedAt(now).Exec(ctx)
}

func (self *CycleRepository) Archive(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Cycle.UpdateOneID(id).SetArchivedAt(now).SetUpdatedAt(now).Exec(ctx)
}

type MoveInput struct {
	From uuid.UUID
	// The cycle they go to; nil takes them out of any.
	To *uuid.UUID
	// Only the issues not in the trash and not completed or canceled.
	UnfinishedOnly bool
	// The history entry's action and actor (nil for Ricrac).
	Action string
	Actor  *uuid.UUID
	Now    time.Time
}

// MoveIssues moves a cycle's issues to another cycle, or out of any, and records it in their
// history.
func (self *CycleRepository) MoveIssues(ctx context.Context, tx repository.TxInterface, input MoveInput) error {
	client := self.base.Client(tx)
	query := client.Issue.Query().Where(issue.CycleID(input.From))
	if input.UnfinishedOnly {
		query = query.Where(issue.TrashedAtIsNil(), issue.HasStateWith(workflowstate.TypeNotIn("completed", "canceled")))
	}
	ids, err := query.IDs(ctx)
	if err != nil || len(ids) == 0 {
		return err
	}
	update := client.Issue.Update().Where(issue.IDIn(ids...)).SetUpdatedAt(input.Now)
	if input.To == nil {
		update.ClearCycleID()
	} else {
		update.SetCycleID(*input.To)
	}
	if err := update.Exec(ctx); err != nil {
		return err
	}
	from, err := client.Cycle.Get(ctx, input.From)
	if err != nil {
		return err
	}
	var to *ent.Cycle
	if input.To != nil {
		if to, err = client.Cycle.Get(ctx, *input.To); err != nil {
			return err
		}
	}
	changes, err := json.Marshal(models.M{"cycle": models.M{"from": models.CycleRefOf(from), "to": models.CycleRefOf(to)}})
	if err != nil {
		return err
	}
	entries := make([]*ent.IssueHistoryCreate, len(ids))
	for i, id := range ids {
		entries[i] = client.IssueHistory.Create().SetIssueID(id).SetNillableActorID(input.Actor).SetAction(input.Action).SetChanges(changes)
	}
	return client.IssueHistory.CreateBulk(entries...).Exec(ctx)
}
