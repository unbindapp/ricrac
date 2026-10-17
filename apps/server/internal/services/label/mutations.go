package label_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	label_repo "ricrac/internal/repositories/label"
	events_service "ricrac/internal/services/events"
)

func (self *LabelService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.IssueLabel, error) {
	if err := actor.RequireMember(); err != nil {
		return models.IssueLabel{}, err
	}
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team)
	if err != nil {
		return models.IssueLabel{}, err
	}
	var value models.IssueLabel
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var label uuid.UUID
		var before *models.IssueLabel
		if input.ID != nil {
			if label, err = self.find(ctx, actor, *input.ID, team); err != nil {
				return err
			}
			current, err := self.value(ctx, tx, label)
			if err != nil {
				return err
			}
			before = &current
		} else if label, err = self.create(ctx, tx, actor, team, input); err != nil {
			return err
		}
		if err := self.apply(ctx, tx, actor, label, input); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, label); err != nil {
			return err
		}
		stored, err := self.repo.Label().Get(ctx, tx, label)
		if err != nil {
			return err
		}
		if before == nil {
			return self.events.Emit(ctx, tx, actor, "IssueLabel", "create", value, nil, stored.TeamID)
		}
		updatedFrom := events_service.ChangedFields(*before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Label().Touch(ctx, tx, label, now); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "IssueLabel", "update", value, updatedFrom, stored.TeamID)
	})
	if err != nil {
		return models.IssueLabel{}, err
	}
	return value, nil
}

func (self *LabelService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team *uuid.UUID, input *SaveInput) (uuid.UUID, error) {
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	color := utils.PickColor(name)
	if input.Color != nil {
		color = *input.Color
	}
	if color, err = utils.CheckColor(color); err != nil {
		return uuid.Nil, err
	}
	return self.repo.Label().Create(ctx, tx, label_repo.CreateInput{Workspace: actor.Workspace, Team: team, Name: name, Color: color, Creator: actor.User})
}

func (self *LabelService) apply(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, label uuid.UUID, input *SaveInput) error {
	current, err := self.repo.Label().GetForUpdate(ctx, tx, label)
	if err != nil {
		return err
	}
	name := current.Name
	if input.Name != nil {
		if name, err = utils.NonEmpty(*input.Name, "name"); err != nil {
			return err
		}
	}
	parent := current.ParentID
	if input.Parent.Set {
		parent = nil
		if input.Parent.Valid {
			id, err := self.checkParent(ctx, tx, actor, label, current.TeamID, input.Parent.Value)
			if err != nil {
				return err
			}
			parent = &id
		}
	}
	isGroup := current.IsGroup
	if input.IsGroup != nil {
		isGroup = *input.IsGroup
	}
	if isGroup && !current.IsGroup {
		used, err := self.repo.Label().OnIssues(ctx, tx, label)
		if err != nil {
			return err
		}
		if used {
			return errdefs.BadRequest("%s is on issues, and a group can't be; create a new group instead", name)
		}
	}
	if !isGroup && current.IsGroup {
		hasChildren, err := self.repo.Label().HasChildren(ctx, tx, label)
		if err != nil {
			return err
		}
		if hasChildren {
			return errdefs.BadRequest("%s still holds labels; move or delete them first", name)
		}
	}
	groupType := current.GroupType
	if input.GroupType != nil {
		if !isGroup {
			return errdefs.BadRequest("groupType only applies to groups (isGroup: true)")
		}
		if *input.GroupType != "singleSelect" && *input.GroupType != "multiSelect" {
			return errdefs.BadRequest("groupType is singleSelect or multiSelect")
		}
		groupType = *input.GroupType
	}
	taken, err := self.repo.Label().NameTaken(ctx, tx, actor.Workspace, current.TeamID, parent, name, label)
	if err != nil {
		return err
	}
	if taken {
		place := "the workspace"
		if parent != nil {
			place = "this group"
		} else if current.TeamID != nil {
			place = "this team"
		}
		return errdefs.BadRequest("There already is a label named %q in %s", name, place)
	}
	err = self.repo.Label().Update(ctx, tx, label, label_repo.UpdateInput{Name: name, Parent: parent, IsGroup: isGroup, GroupType: groupType})
	if err != nil {
		return err
	}
	if input.Color != nil {
		color, err := utils.CheckColor(*input.Color)
		if err != nil {
			return err
		}
		if err := self.repo.Label().SetColor(ctx, tx, label, color); err != nil {
			return err
		}
	}
	if input.Description.Set {
		if err := self.repo.Label().SetDescription(ctx, tx, label, utils.Trimmed(input.Description.Ptr())); err != nil {
			return err
		}
	}
	if isGroup {
		if err := self.checkSingleSelect(ctx, tx, label); err != nil {
			return err
		}
	}
	if parent != nil {
		return self.checkSingleSelect(ctx, tx, *parent)
	}
	return nil
}

// checkParent is the group reference names, if label may go in it: same team or a workspace
// group, and not one of its own descendants.
func (self *LabelService) checkParent(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, label uuid.UUID, team *uuid.UUID, reference string) (uuid.UUID, error) {
	parent, err := self.find(ctx, actor, reference, team)
	if err != nil {
		return uuid.Nil, err
	}
	group, err := self.repo.Label().Get(ctx, tx, parent)
	if err != nil {
		return uuid.Nil, err
	}
	if !group.IsGroup {
		return uuid.Nil, errdefs.BadRequest("%s isn't a group; make it one with isGroup: true", group.Name)
	}
	if group.TeamID != nil && (team == nil || *group.TeamID != *team) {
		return uuid.Nil, errdefs.BadRequest("A label's group must be a workspace group or a group of the same team")
	}
	loops, err := self.repo.Label().InOwnTree(ctx, tx, parent, label)
	if err != nil {
		return uuid.Nil, err
	}
	if loops {
		return uuid.Nil, errdefs.BadRequest("A label can't be in its own group or in a group inside it")
	}
	return parent, nil
}

// checkSingleSelect checks that issues have at most one label of a single-select group.
func (self *LabelService) checkSingleSelect(ctx context.Context, tx repository.TxInterface, group uuid.UUID) error {
	clash, found, err := self.repo.Label().SingleSelectClash(ctx, tx, group)
	if err != nil {
		return err
	}
	if found {
		return errdefs.BadRequest("Some issues would have several labels of the single-select group %s; make it multiSelect or remove them first", clash)
	}
	return nil
}

func (self *LabelService) setRetired(ctx context.Context, actor *auth.Actor, input *LabelInput, retired bool) (models.IssueLabel, error) {
	if err := actor.RequireMember(); err != nil {
		return models.IssueLabel{}, err
	}
	label, err := self.labelOf(ctx, actor, input)
	if err != nil {
		return models.IssueLabel{}, err
	}
	var value models.IssueLabel
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		before, err := self.value(ctx, tx, label)
		if err != nil {
			return err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Label().SetRetired(ctx, tx, label, retired, now); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, label); err != nil {
			return err
		}
		stored, err := self.repo.Label().Get(ctx, tx, label)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "IssueLabel", "update", value, events_service.ChangedFields(before, value), stored.TeamID)
	})
	if err != nil {
		return models.IssueLabel{}, err
	}
	return value, nil
}

func (self *LabelService) Retire(ctx context.Context, actor *auth.Actor, input *LabelInput) (models.IssueLabel, error) {
	return self.setRetired(ctx, actor, input, true)
}

func (self *LabelService) Restore(ctx context.Context, actor *auth.Actor, input *LabelInput) (models.IssueLabel, error) {
	return self.setRetired(ctx, actor, input, false)
}

func (self *LabelService) Delete(ctx context.Context, actor *auth.Actor, input *LabelInput) (models.DeletedIssueLabel, error) {
	if err := actor.RequireMember(); err != nil {
		return models.DeletedIssueLabel{}, err
	}
	label, err := self.labelOf(ctx, actor, input)
	if err != nil {
		return models.DeletedIssueLabel{}, err
	}
	var issues int
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.value(ctx, tx, label)
		if err != nil {
			return err
		}
		stored, err := self.repo.Label().Get(ctx, tx, label)
		if err != nil {
			return err
		}
		if issues, err = self.repo.Label().IssuesWithLabelOrGroup(ctx, tx, label); err != nil {
			return err
		}
		if err := self.repo.Label().Delete(ctx, tx, label); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "IssueLabel", "remove", value, nil, stored.TeamID)
	})
	if err != nil {
		return models.DeletedIssueLabel{}, err
	}
	return models.DeletedIssueLabel{Deleted: label.String(), RemovedFromIssues: issues}, nil
}
