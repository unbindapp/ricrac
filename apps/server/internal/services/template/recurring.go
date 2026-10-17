package template_service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

// addMonths adds months as chrono does, keeping the day unless the month is shorter.
func addMonths(at time.Time, months int) time.Time {
	first := time.Date(at.Year(), at.Month()+time.Month(months), 1, 0, 0, 0, 0, at.Location())
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(at.Day(), last), at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), at.Location())
}

func nextRun(at time.Time, every uint32, unit string) *time.Time {
	var next time.Time
	switch unit {
	case "day":
		next = at.AddDate(0, 0, int(every))
	case "week":
		next = at.AddDate(0, 0, 7*int(every))
	case "month":
		next = addMonths(at, int(every))
	default:
		return nil
	}
	return &next
}

// RunRecurring creates the issues of recurring templates that are due. Each template is claimed
// and moved to its next time first, so a slow or failing run never creates the same issue twice.
func (self *TemplateService) RunRecurring(ctx context.Context) error {
	var runs []*ent.Template
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var err error
		if runs, err = self.repo.Template().Due(ctx, tx); err != nil {
			return err
		}
		for _, run := range runs {
			var next *time.Time
			var recurrence Recurrence
			if json.Unmarshal(run.Recurrence, &recurrence) == nil && run.NextRunAt != nil {
				next = nextRun(*run.NextRunAt, recurrence.Every, recurrence.Unit)
				for next != nil && !next.After(time.Now()) {
					next = nextRun(*next, recurrence.Every, recurrence.Unit)
				}
			}
			if err := self.repo.Template().SetNextRun(ctx, tx, run.ID, next); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.CreatorID == nil {
			continue
		}
		if err := self.createIssue(ctx, run); err != nil {
			return err
		}
	}
	return nil
}

// createIssue creates an issue from the template as its creator, logging why it couldn't.
func (self *TemplateService) createIssue(ctx context.Context, run *ent.Template) error {
	actor, err := self.access.Load(ctx, *run.CreatorID, run.WorkspaceID, []auth.Access{auth.Read, auth.Write})
	if err != nil || actor == nil {
		return err
	}
	raw, err := self.repo.Template().Data(ctx, nil, run.ID)
	if err != nil {
		return err
	}
	var data models.M
	_ = json.Unmarshal(raw, &data)
	input := models.M{"template": run.ID.String()}
	if _, ok := data["title"]; !ok {
		input["title"] = run.Name
	}
	if _, err := self.ops.Call(ctx, actor, "save_issue", input); err != nil {
		slog.Warn(fmt.Sprintf("recurring issue from template %s failed: %s", run.Name, errdefs.As(err).Message))
	}
	return nil
}
