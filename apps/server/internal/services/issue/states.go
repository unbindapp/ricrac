package issue_service

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	issue_repo "ricrac/internal/repositories/issue"
	events_service "ricrac/internal/services/events"
)

type M = map[string]any

// DefaultState is the state new issues of the team start in.
func (self *IssueService) DefaultState(ctx context.Context, tx repository.TxInterface, team uuid.UUID) (uuid.UUID, error) {
	id, found, err := self.repo.Issue().DefaultState(ctx, tx, team)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, errdefs.BadRequest("The team has no statuses")
	}
	return id, nil
}

// FirstStateOfType is the team's first unarchived state of a type, nil when there's none.
func (self *IssueService) FirstStateOfType(ctx context.Context, tx repository.TxInterface, team uuid.UUID, kind string) (*uuid.UUID, error) {
	return self.repo.Issue().FirstStateOfType(ctx, tx, team, kind)
}

func (self *IssueService) StateType(ctx context.Context, tx repository.TxInterface, state uuid.UUID) (string, error) {
	found, err := self.repo.Issue().State(ctx, tx, state)
	if err != nil {
		return "", err
	}
	return found.Type, nil
}

// AllowedEstimates is the estimates a team's scale has.
func AllowedEstimates(kind string, allowZero, extended bool) []float64 {
	scales := map[string][2][]float64{
		"exponential": {{1, 2, 4, 8, 16}, {32, 64}},
		"fibonacci":   {{1, 2, 3, 5, 8}, {13, 21}},
		"linear":      {{1, 2, 3, 4, 5}, {6, 7, 8, 9, 10}},
		"tShirt":      {{1, 2, 3, 5, 8}, {13, 21}},
	}
	values := []float64{}
	if allowZero {
		values = append(values, 0)
	}
	values = append(values, scales[kind][0]...)
	if extended {
		values = append(values, scales[kind][1]...)
	}
	return values
}

// CheckEstimate is the estimate when it's on the team's scale.
func CheckEstimate(team *ent.Team, estimate float64) (float64, error) {
	if team.EstimateType == "notUsed" {
		return 0, errdefs.BadRequest("This team doesn't use estimates; turn them on with save_team (estimateType)")
	}
	allowed := AllowedEstimates(team.EstimateType, team.EstimateAllowZero, team.EstimateExtended)
	if !slices.Contains(allowed, estimate) {
		list := make([]string, len(allowed))
		for i, v := range allowed {
			list[i] = strconv.FormatFloat(v, 'f', -1, 64)
		}
		return 0, errdefs.BadRequest("%s isn't on the team's %s scale; use one of %s",
			strconv.FormatFloat(estimate, 'f', -1, 64), team.EstimateType, strings.Join(list, ", "))
	}
	return estimate, nil
}

// TriageResponsible is what the team does with issues entering triage (triage_action) and who is
// responsible now: everyone listed, or whose turn it is when they rotate.
func (self *IssueService) TriageResponsible(ctx context.Context, tx repository.TxInterface, team uuid.UUID) (string, []uuid.UUID, error) {
	t, err := self.repo.Issue().Team(ctx, tx, team)
	if err != nil {
		return "", nil, err
	}
	ids := []uuid.UUID(t.TriageResponsibleIds)
	if t.TriageRotationWeeks == nil || t.TriageRotationStart == nil || len(ids) == 0 || *t.TriageRotationWeeks <= 0 {
		return t.TriageAction, ids, nil
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	elapsed := max(int(today.Sub(*t.TriageRotationStart).Hours()/24)/7, 0)
	turn := (elapsed / *t.TriageRotationWeeks) % len(ids)
	return t.TriageAction, []uuid.UUID{ids[turn]}, nil
}

type stateChange struct {
	issue, state uuid.UUID
	action       string
}

// SetState moves the issue to state, stamping when it started, completed or left triage, and runs
// what follows: history, notifications, the current cycle, unblocked issues, and closing
// sub-issues or the parent when the team asks for it.
func (self *IssueService) SetState(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue, state uuid.UUID, action string) error {
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	queue := []stateChange{{issue, state, action}}
	for len(queue) > 0 {
		next := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		row, err := self.repo.Issue().GetForUpdate(ctx, tx, next.issue)
		if ent.IsNotFound(err) {
			return errdefs.NotFound("No such issue")
		}
		if err != nil {
			return err
		}
		if row.StateID == next.state {
			continue
		}
		oldState, err := self.repo.Issue().State(ctx, tx, row.StateID)
		if err != nil {
			return err
		}
		newState, err := self.repo.Issue().State(ctx, tx, next.state)
		if err != nil {
			return err
		}
		if err := self.repo.Issue().MoveToState(ctx, tx, next.issue, stamps(row, newState.Type, oldState.Type, next.state, now)); err != nil {
			return err
		}
		changes := M{"state": M{"from": models.StateRefOf(oldState), "to": models.StateRefOf(newState)}}
		if err := self.events.History(ctx, tx, actor, next.issue, next.action, changes); err != nil {
			return err
		}
		identifier, err := self.repo.Issue().Identifier(ctx, tx, next.issue)
		if err != nil {
			return err
		}
		subscribers, err := self.events.IssueSubscribers(ctx, tx, next.issue)
		if err != nil {
			return err
		}
		if row.AssigneeID != nil {
			subscribers = append(subscribers, *row.AssigneeID)
		}
		notice := events_service.Notice{Kind: "issueStatusChanged", Summary: identifier + " moved to " + newState.Name, Issue: &next.issue}
		if err := self.events.Notify(ctx, tx, actor, subscribers, notice); err != nil {
			return err
		}
		if row.CycleID == nil && (newState.Type == "started" || newState.Type == "completed") {
			if err := self.repo.Issue().AddToCurrentCycle(ctx, tx, row.ID, row.TeamID, newState.Type == "started"); err != nil {
				return err
			}
		}
		closed := newState.Type == "completed" || newState.Type == "canceled"
		if closed {
			if err := self.notifyUnblocked(ctx, tx, actor, next.issue); err != nil {
				return err
			}
			team, err := self.repo.Issue().Team(ctx, tx, row.TeamID)
			if err != nil {
				return err
			}
			if team.AutoCloseChildIssues {
				children, err := self.repo.Issue().OpenChildren(ctx, tx, next.issue)
				if err != nil {
					return err
				}
				for _, child := range children {
					target, err := self.repo.Issue().FirstStateOfType(ctx, tx, child.TeamID, newState.Type)
					if err != nil {
						return err
					}
					if target != nil {
						queue = append(queue, stateChange{child.ID, *target, "auto_closed_with_parent"})
					}
				}
			}
		}
		if row.ParentID == nil {
			continue
		}
		parent, err := self.repo.Issue().Get(ctx, tx, *row.ParentID)
		if err != nil {
			return err
		}
		parentState, err := self.repo.Issue().State(ctx, tx, parent.StateID)
		if err != nil {
			return err
		}
		parentTeam, err := self.repo.Issue().Team(ctx, tx, parent.TeamID)
		if err != nil {
			return err
		}
		open, done, err := self.repo.Issue().ChildCounts(ctx, tx, parent.ID)
		if err != nil {
			return err
		}
		if !parentTeam.AutoCloseParentIssues || !closed || open != 0 || done == 0 || parentState.Type == "completed" || parentState.Type == "canceled" {
			continue
		}
		target, err := self.repo.Issue().FirstStateOfType(ctx, tx, parent.TeamID, "completed")
		if err != nil {
			return err
		}
		if target != nil {
			queue = append(queue, stateChange{parent.ID, *target, "auto_completed_by_sub_issues"})
		}
	}
	return nil
}

// stamps is the times a move from oldType to newType sets, as of now.
func stamps(row *ent.Issue, newType, oldType string, state uuid.UUID, now time.Time) issue_repo.StateStamps {
	s := issue_repo.StateStamps{State: state, UpdatedAt: &now, StartedTriageAt: row.StartedTriageAt, TriagedAt: row.TriagedAt}
	switch newType {
	case "started":
		s.StartedAt = row.StartedAt
		if s.StartedAt == nil {
			s.StartedAt = &now
		}
	case "completed", "canceled":
		s.StartedAt = row.StartedAt
	}
	if newType == "completed" {
		s.CompletedAt = &now
	}
	if newType == "canceled" {
		s.CanceledAt = &now
	}
	if newType == "triage" && s.StartedTriageAt == nil {
		s.StartedTriageAt = &now
	}
	if newType != "triage" && oldType == "triage" {
		s.TriagedAt = &now
	}
	return s
}

func (self *IssueService) notifyUnblocked(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue uuid.UUID) error {
	freed, err := self.repo.Issue().Unblocked(ctx, tx, issue)
	if err != nil {
		return err
	}
	for _, blocked := range freed {
		identifier, err := self.repo.Issue().Identifier(ctx, tx, blocked)
		if err != nil {
			return err
		}
		people, err := self.events.IssueSubscribers(ctx, tx, blocked)
		if err != nil {
			return err
		}
		row, err := self.repo.Issue().Get(ctx, tx, blocked)
		if err != nil {
			return err
		}
		if row.AssigneeID != nil {
			people = append(people, *row.AssigneeID)
		}
		notice := events_service.Notice{Kind: "issueUnblocked", Summary: identifier + " is no longer blocked", Issue: &blocked}
		if err := self.events.Notify(ctx, tx, actor, people, notice); err != nil {
			return err
		}
	}
	return nil
}
