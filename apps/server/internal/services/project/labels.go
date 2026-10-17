package project_service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	project_repo "ricrac/internal/repositories/project"
)

type ListLabelsInput struct {
	IncludeRetired bool    `json:"includeRetired,omitempty"`
	Query          *string `json:"query" doc:"Part of the name."`
	Limit          *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor         *string `json:"cursor" doc:"The endCursor of the previous page."`
}

type SaveLabelInput struct {
	ID          *string               `json:"id" doc:"Only to change a label: its id, name or Group/Name. Omit to create one."`
	Name        *string               `json:"name"`
	Color       *string               `json:"color" doc:"Hex color like #bb87fc."`
	Description args.Nullable[string] `json:"description" doc:"null clears it."`
	IsGroup     *bool                 `json:"isGroup" doc:"Whether this is a group holding other labels (projects can't have a group itself)."`
	Parent      args.Nullable[string] `json:"parent" doc:"The group (name or id) this label sits in; null takes it out."`
}

func (self *ProjectService) labelValues(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.ProjectLabel, error) {
	labels, err := self.repo.Project().LoadLabels(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.Project().LabelProjectCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.ProjectLabel, len(labels))
	for i, l := range labels {
		values[i] = models.ProjectLabelOf(l, counts[l.ID])
	}
	return values, nil
}

func (self *ProjectService) labelValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.ProjectLabel, error) {
	values, err := self.labelValues(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.ProjectLabel{}, err
	}
	if len(values) == 0 {
		return models.ProjectLabel{}, errdefs.NotFound("No such project label")
	}
	return values[0], nil
}

func (self *ProjectService) ListLabels(ctx context.Context, actor *auth.Actor, input *ListLabelsInput) (args.PageOf[models.ProjectLabel], error) {
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	var like *string
	if input.Query != nil {
		like = new("%" + utils.EscapeLike(strings.TrimSpace(*input.Query)) + "%")
	}
	offset, err := page.Offset()
	if err != nil {
		return args.PageOf[models.ProjectLabel]{}, err
	}
	ids, err := self.repo.Project().Labels(ctx, project_repo.LabelFilter{
		Workspace: actor.Workspace, IncludeRetired: input.IncludeRetired, Pattern: like, Limit: page.Size() + 1, Offset: offset,
	})
	if err != nil {
		return args.PageOf[models.ProjectLabel]{}, err
	}
	values, err := self.labelValues(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.ProjectLabel]{}, err
	}
	return args.WrapOf(page, values)
}

// labelID is a project label or group by id, name or Group/Name, retired ones included.
func (self *ProjectService) labelID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	rows, err := self.repo.Project().AllLabels(ctx, actor.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, ok := utils.AsUUID(value); ok {
		for _, r := range rows {
			if r.ID == id {
				return r.ID, nil
			}
		}
	}
	group, name := utils.SplitGroup(value)
	var matches []project_repo.LabelRow
	for _, r := range rows {
		if strings.EqualFold(r.Name, name) && (group == nil || r.Parent != nil && strings.EqualFold(*r.Parent, *group)) {
			matches = append(matches, r)
		}
	}
	switch {
	case len(matches) == 1 || len(matches) > 1 && group != nil:
		return matches[0].ID, nil
	case len(matches) == 0:
		var names []string
		for _, r := range rows[:min(len(rows), 30)] {
			if r.Parent != nil {
				names = append(names, *r.Parent+"/"+r.Name)
			} else {
				names = append(names, r.Name)
			}
		}
		return uuid.Nil, errdefs.BadRequest("No project label \"%s\". Options: %s.", value, strings.Join(names, ", "))
	}
	return uuid.Nil, errdefs.BadRequest("\"%s\" matches labels in several groups; name it as Group/%s", value, name)
}

func (self *ProjectService) SaveLabel(ctx context.Context, actor *auth.Actor, input *SaveLabelInput) (models.ProjectLabel, error) {
	if err := actor.RequireMember(); err != nil {
		return models.ProjectLabel{}, err
	}
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.labelID(ctx, actor, *input.ID)
		if err != nil {
			return models.ProjectLabel{}, err
		}
		existing = &id
	}
	var value models.ProjectLabel
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		if existing != nil {
			id = *existing
		} else {
			name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
			if err != nil {
				return err
			}
			color, err := colorOrPick(input.Color, name)
			if err != nil {
				return err
			}
			if id, err = self.repo.Project().CreateLabel(ctx, tx, actor.Workspace, name, color, actor.User); err != nil {
				return err
			}
		}
		if err := self.applyLabel(ctx, tx, actor, id, input); err != nil {
			return err
		}
		var err error
		if value, err = self.labelValue(ctx, tx, id); err != nil {
			return err
		}
		action := "create"
		if existing != nil {
			action = "update"
		}
		return self.events.Emit(ctx, tx, actor, "ProjectLabel", action, value, nil, nil)
	})
	return value, err
}

func (self *ProjectService) applyLabel(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input *SaveLabelInput) error {
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := self.repo.Project().UpdateLabel(tx, id).SetName(name).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Color != nil {
		color, err := utils.CheckColor(*input.Color)
		if err != nil {
			return err
		}
		if err := self.repo.Project().UpdateLabel(tx, id).SetColor(color).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Description.Set {
		update := self.repo.Project().UpdateLabel(tx, id)
		if err := setOrClear(update, input.Description.Ptr(), (*ent.ProjectLabelUpdateOne).SetDescription, (*ent.ProjectLabelUpdateOne).ClearDescription).Exec(ctx); err != nil {
			return err
		}
	}
	if input.IsGroup != nil {
		if err := self.repo.Project().UpdateLabel(tx, id).SetIsGroup(*input.IsGroup).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Parent.Set {
		parent, err := optionalRef(ctx, actor, input.Parent, self.labelID)
		if err != nil {
			return err
		}
		update := self.repo.Project().UpdateLabel(tx, id)
		if err := setOrClear(update, parent, (*ent.ProjectLabelUpdateOne).SetParentID, (*ent.ProjectLabelUpdateOne).ClearParentID).Exec(ctx); err != nil {
			return err
		}
	}
	shape, err := self.repo.Project().LabelShape(ctx, tx, id)
	if err != nil {
		return err
	}
	switch {
	case shape.ParentIsGroup != nil && !*shape.ParentIsGroup:
		return errdefs.BadRequest("A label's parent must be a group (a label with isGroup)")
	case shape.IsGroup && shape.ParentIsGroup != nil:
		return errdefs.BadRequest("Groups don't nest; a group can't have a parent")
	case shape.IsGroup && shape.InUse:
		return errdefs.BadRequest("Projects use this label, so it can't become a group")
	case !shape.IsGroup && shape.HasChildren:
		return errdefs.BadRequest("This group has labels in it, so it must stay a group")
	}
	clash, err := self.repo.Project().LabelNameClash(ctx, tx, id)
	if err != nil {
		return err
	}
	if clash {
		return errdefs.Conflict("There is already a project label with that name")
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Project().UpdateLabel(tx, id).SetUpdatedAt(now).Exec(ctx)
}

func (self *ProjectService) markLabel(ctx context.Context, actor *auth.Actor, reference string, change func(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error) (models.ProjectLabel, error) {
	if err := actor.RequireMember(); err != nil {
		return models.ProjectLabel{}, err
	}
	id, err := self.labelID(ctx, actor, reference)
	if err != nil {
		return models.ProjectLabel{}, err
	}
	var value models.ProjectLabel
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := change(ctx, tx, id, now); err != nil {
			return err
		}
		if value, err = self.labelValue(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "ProjectLabel", "update", value, nil, nil)
	})
	return value, err
}

func (self *ProjectService) RetireLabel(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ProjectLabel, error) {
	return self.markLabel(ctx, actor, input.ID, self.repo.Project().RetireLabel)
}

func (self *ProjectService) RestoreLabel(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ProjectLabel, error) {
	return self.markLabel(ctx, actor, input.ID, self.repo.Project().RestoreLabel)
}

func (self *ProjectService) DeleteLabel(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedProjectLabel, error) {
	if err := actor.RequireMember(); err != nil {
		return models.DeletedProjectLabel{}, err
	}
	id, err := self.labelID(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedProjectLabel{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.labelValue(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := self.repo.Project().DeleteLabel(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "ProjectLabel", "remove", value, nil, nil)
	})
	if err != nil {
		return models.DeletedProjectLabel{}, err
	}
	return models.DeletedProjectLabel{Deleted: true, ID: id.String()}, nil
}
