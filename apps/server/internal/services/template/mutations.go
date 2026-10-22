package template_service

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

func (self *TemplateService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.TemplateDetail, error) {
	if err := actor.RequireMember(); err != nil {
		return models.TemplateDetail{}, err
	}
	var team *uuid.UUID
	if input.Team.Valid {
		id, err := self.resolve.Team(ctx, actor, input.Team.Value)
		if err != nil {
			return models.TemplateDetail{}, err
		}
		team = &id
	}
	var value models.TemplateDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		action := "create"
		if input.ID != nil {
			action = "update"
			if id, err = self.find(ctx, actor, input.Type, *input.ID); err != nil {
				return err
			}
		} else if id, err = self.create(ctx, tx, actor, input); err != nil {
			return err
		}
		if err := self.apply(ctx, tx, id, team, input); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Template", action, value, nil, teamOf(value))
	})
	return value, err
}

func (self *TemplateService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	if input.Type == nil {
		return uuid.Nil, errdefs.BadRequest("type is required: issue, project or document")
	}
	kind, err := checkType(*input.Type)
	if err != nil {
		return uuid.Nil, err
	}
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	if input.Data == nil {
		return uuid.Nil, errdefs.BadRequest("data is required: what the template fills in")
	}
	return self.repo.Template().Create(ctx, tx, actor.Workspace, kind, name, actor.User)
}

func (self *TemplateService) apply(ctx context.Context, tx repository.TxInterface, id uuid.UUID, team *uuid.UUID, input *SaveInput) error {
	current, err := self.repo.Template().GetForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := self.repo.Template().SetName(ctx, tx, id, name); err != nil {
			return err
		}
	}
	if input.Description.Set {
		if err := self.repo.Template().SetDescription(ctx, tx, id, utils.Trimmed(input.Description.Ptr())); err != nil {
			return err
		}
	}
	if input.Team.Set {
		if err := self.repo.Template().SetTeam(ctx, tx, id, team); err != nil {
			return err
		}
	}
	if input.Recurrence.Set {
		if err := self.setRecurrence(ctx, tx, id, current.Type, input.Recurrence.Ptr()); err != nil {
			return err
		}
	}
	if input.Data != nil {
		if err := checkData(current.Type, input.Data); err != nil {
			return err
		}
		data, err := json.Marshal(input.Data)
		if err != nil {
			return err
		}
		if err := self.repo.Template().SetData(ctx, tx, id, data); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Template().Touch(ctx, tx, id, now)
}

func (self *TemplateService) setRecurrence(ctx context.Context, tx repository.TxInterface, id uuid.UUID, kind string, r *Recurrence) error {
	if r == nil {
		return self.repo.Template().SetRecurrence(ctx, tx, id, nil, nil)
	}
	if kind != "issue" {
		return errdefs.BadRequest("Only issue templates recur")
	}
	if r.Every < 1 || r.Every > 365 || !slices.Contains([]string{"day", "week", "month"}, r.Unit) {
		return errdefs.BadRequest("recurrence is {every: 1-365, unit: day|week|month, startsAt?}")
	}
	starts := time.Now().UTC()
	if r.StartsAt != nil {
		var err error
		if starts, err = utils.ParseDatetime(*r.StartsAt); err != nil {
			return err
		}
	}
	stored, err := json.Marshal(models.M{"every": r.Every, "unit": r.Unit, "startsAt": starts})
	if err != nil {
		return err
	}
	return self.repo.Template().SetRecurrence(ctx, tx, id, stored, &starts)
}

func (self *TemplateService) Delete(ctx context.Context, actor *auth.Actor, input *DeleteInput) (models.DeletedTemplate, error) {
	if err := actor.RequireMember(); err != nil {
		return models.DeletedTemplate{}, err
	}
	id, err := self.find(ctx, actor, input.Type, input.ID)
	if err != nil {
		return models.DeletedTemplate{}, err
	}
	var value models.TemplateDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		if err := self.repo.Template().Delete(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Template", "remove", value, nil, teamOf(value))
	})
	if err != nil {
		return models.DeletedTemplate{}, err
	}
	return models.DeletedTemplate{Deleted: true, ID: id.String(), Name: value.Name}, nil
}
