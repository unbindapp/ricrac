package state_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	state_repo "ricrac/internal/repositories/state"
	events_service "ricrac/internal/services/events"
)

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to change a status: its id or name. Omit to create one."`
	Team        *string               `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id."`
	Name        *string               `json:"name" doc:"Required to create; unique in the workflow."`
	Type        *string               `json:"type" doc:"triage, backlog, unstarted, started, completed or canceled. Required to create; changes only while no issues are in the status."`
	Color       *string               `json:"color" doc:"Hex color like #f2c94c; defaults to the type's color."`
	Description args.Nullable[string] `json:"description" doc:"Null clears it."`
	Position    *float64              `json:"position" doc:"Order within its type on the board; lower comes first. New statuses go last."`
}

func (self *StateService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.IssueStatus, error) {
	var value models.IssueStatus
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var state, team uuid.UUID
		var before *models.IssueStatus
		var err error
		if input.ID != nil {
			if state, team, err = self.find(ctx, actor, *input.ID, input.Team); err != nil {
				return err
			}
			if err := self.teams.RequireCanEdit(ctx, tx, actor, team); err != nil {
				return err
			}
			current, err := self.value(ctx, tx, state)
			if err != nil {
				return err
			}
			before = &current
			if err := self.update(ctx, tx, state, team, input); err != nil {
				return err
			}
		} else {
			if team, err = self.resolve.TeamOrDefault(ctx, actor, input.Team); err != nil {
				return err
			}
			if err := self.teams.RequireCanEdit(ctx, tx, actor, team); err != nil {
				return err
			}
			if state, err = self.create(ctx, tx, team, input); err != nil {
				return err
			}
		}
		if err := self.checkWorkflow(ctx, tx, team); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, state); err != nil {
			return err
		}
		if before == nil {
			return self.events.Emit(ctx, tx, actor, "WorkflowState", "create", value, nil, &team)
		}
		updatedFrom := events_service.ChangedFields(*before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		return self.events.Emit(ctx, tx, actor, "WorkflowState", "update", value, updatedFrom, &team)
	})
	if err != nil {
		return models.IssueStatus{}, err
	}
	return value, nil
}

func (self *StateService) checkNameFree(ctx context.Context, tx repository.TxInterface, team uuid.UUID, state *uuid.UUID, name string) error {
	taken, err := self.repo.State().NameTaken(ctx, tx, team, name, state)
	if err != nil {
		return err
	}
	if taken {
		return errdefs.BadRequest("The team already has a status named %q", name)
	}
	return nil
}

func (self *StateService) checkSingleTriage(ctx context.Context, tx repository.TxInterface, team uuid.UUID, state *uuid.UUID) error {
	exists, err := self.repo.State().HasTriage(ctx, tx, team, state)
	if err != nil {
		return err
	}
	if exists {
		return errdefs.BadRequest("The team already has a triage status; a team has one")
	}
	return nil
}

func (self *StateService) create(ctx context.Context, tx repository.TxInterface, team uuid.UUID, input *SaveInput) (uuid.UUID, error) {
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	if input.Type == nil {
		return uuid.Nil, errdefs.BadRequest("type is required: triage, backlog, unstarted, started, completed or canceled")
	}
	kind, err := checkType(*input.Type)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.checkNameFree(ctx, tx, team, nil, name); err != nil {
		return uuid.Nil, err
	}
	if kind == "triage" {
		if err := self.checkSingleTriage(ctx, tx, team, nil); err != nil {
			return uuid.Nil, err
		}
	}
	color := defaultColor(kind)
	if input.Color != nil {
		if color, err = utils.CheckColor(*input.Color); err != nil {
			return uuid.Nil, err
		}
	}
	var position float64
	if input.Position != nil {
		position = *input.Position
	} else if position, err = self.repo.State().NextPosition(ctx, tx, team, kind); err != nil {
		return uuid.Nil, err
	}
	return self.repo.State().Create(ctx, tx, state_repo.CreateInput{
		Team: team, Name: name, Type: kind, Color: color, Description: utils.Trimmed(input.Description.Ptr()), Position: position,
	})
}

func (self *StateService) update(ctx context.Context, tx repository.TxInterface, state, team uuid.UUID, input *SaveInput) error {
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := self.checkNameFree(ctx, tx, team, &state, name); err != nil {
			return err
		}
		if err := self.repo.State().SetName(ctx, tx, state, name); err != nil {
			return err
		}
	}
	if input.Color != nil {
		color, err := utils.CheckColor(*input.Color)
		if err != nil {
			return err
		}
		if err := self.repo.State().SetColor(ctx, tx, state, color); err != nil {
			return err
		}
	}
	if input.Description.Set {
		if err := self.repo.State().SetDescription(ctx, tx, state, utils.Trimmed(input.Description.Ptr())); err != nil {
			return err
		}
	}
	if input.Position != nil {
		if err := self.repo.State().SetPosition(ctx, tx, state, *input.Position); err != nil {
			return err
		}
	}
	if input.Type != nil {
		if err := self.changeType(ctx, tx, state, team, *input.Type); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.State().Touch(ctx, tx, state, now)
}

func (self *StateService) changeType(ctx context.Context, tx repository.TxInterface, state, team uuid.UUID, value string) error {
	kind, err := checkType(value)
	if err != nil {
		return err
	}
	current, err := self.repo.State().Get(ctx, tx, state)
	if err != nil {
		return err
	}
	issues, err := self.repo.State().AllIssueCount(ctx, tx, state)
	if err != nil {
		return err
	}
	if kind == current.Type {
		return nil
	}
	if issues != 0 {
		return errdefs.BadRequest("%d issues are in this status, so its type can't change; move them or create a new status", issues)
	}
	if kind == "triage" {
		if err := self.checkSingleTriage(ctx, tx, team, &state); err != nil {
			return err
		}
	}
	return self.repo.State().SetType(ctx, tx, state, kind)
}

type DeleteInput struct {
	ID           string  `json:"id" doc:"The status's id or name."`
	Team         *string `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id."`
	MoveIssuesTo *string `json:"moveIssuesTo" doc:"Status (name, type or id) to move its issues to. Required when issues are in it or when it is the default or auto-close status."`
}

func (self *StateService) Delete(ctx context.Context, actor *auth.Actor, input *DeleteInput) (models.DeletedIssueStatus, error) {
	state, team, err := self.find(ctx, actor, input.ID, input.Team)
	if err != nil {
		return models.DeletedIssueStatus{}, err
	}
	var deleted models.DeletedIssueStatus
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.teams.RequireCanEdit(ctx, tx, actor, team); err != nil {
			return err
		}
		current, err := self.repo.State().GetForUpdate(ctx, tx, state)
		if err != nil {
			return err
		}
		t := current.Edges.Team
		if current.ArchivedAt != nil {
			return errdefs.BadRequest("The status %s is already archived", current.Name)
		}
		if current.Type == "triage" && t.TriageEnabled {
			return errdefs.BadRequest("Triage is on for this team; turn it off with save_team (triageEnabled: false) instead")
		}
		var replacement *uuid.UUID
		if input.MoveIssuesTo != nil {
			target, err := self.teams.FindState(ctx, tx, team, *input.MoveIssuesTo)
			if err != nil {
				return err
			}
			if target == state {
				return errdefs.BadRequest("moveIssuesTo must be another status")
			}
			replacement = &target
		}
		waiting, err := self.repo.State().Issues(ctx, tx, state)
		if err != nil {
			return err
		}
		need := func(why string) error {
			return errdefs.BadRequest("%s; pass moveIssuesTo (a status of the same team)", why)
		}
		if len(waiting) > 0 && replacement == nil {
			return need(fmt.Sprintf("%d issues are in %s", len(waiting), current.Name))
		}
		for _, setting := range []struct {
			on   bool
			what string
			set  func(*ent.TeamUpdateOne, uuid.UUID) *ent.TeamUpdateOne
		}{
			{t.DefaultIssueStateID != nil && *t.DefaultIssueStateID == state, "default", (*ent.TeamUpdateOne).SetDefaultIssueStateID},
			{t.AutoCloseStateID != nil && *t.AutoCloseStateID == state, "auto-close", (*ent.TeamUpdateOne).SetAutoCloseStateID},
		} {
			if !setting.on {
				continue
			}
			if replacement == nil {
				return need(fmt.Sprintf("%s is the team's %s status", current.Name, setting.what))
			}
			if err := self.repo.Team().Update(ctx, tx, team, func(u *ent.TeamUpdateOne) { setting.set(u, *replacement) }); err != nil {
				return err
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.State().Archive(ctx, tx, state, now); err != nil {
			return err
		}
		if err := self.checkWorkflow(ctx, tx, team); err != nil {
			return err
		}
		if replacement != nil {
			for _, issue := range waiting {
				if err := self.issues.SetState(ctx, tx, actor, issue, *replacement, "updated"); err != nil {
					return err
				}
			}
		}
		value, err := self.value(ctx, tx, state)
		if err != nil {
			return err
		}
		if err := self.events.Emit(ctx, tx, actor, "WorkflowState", "remove", value, nil, &team); err != nil {
			return err
		}
		deleted = models.DeletedIssueStatus{IssueStatus: value, MovedIssues: len(waiting)}
		if replacement == nil {
			return nil
		}
		moved, err := self.repo.State().Get(ctx, tx, *replacement)
		if err != nil {
			return err
		}
		deleted.MovedTo = models.StateRefOf(moved)
		return nil
	})
	if err != nil {
		return models.DeletedIssueStatus{}, err
	}
	return deleted, nil
}
