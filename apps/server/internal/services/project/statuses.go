package project_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

type ListStatusesInput struct {
	Limit  *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor *string `json:"cursor" doc:"The endCursor of the previous page."`
}

type SaveStatusInput struct {
	ID          *string               `json:"id" doc:"Only to change a status: its id or name. Omit to create one."`
	Name        *string               `json:"name"`
	Type        *string               `json:"type" doc:"backlog, planned, started, paused, completed or canceled. Required to create."`
	Color       *string               `json:"color" doc:"Hex color like #f2c94c."`
	Description args.Nullable[string] `json:"description" doc:"null clears it."`
	Position    *float64              `json:"position" doc:"Order among the statuses (lower comes first). New statuses go last."`
}

type DeleteStatusInput struct {
	ID             string  `json:"id" doc:"The status's id or name."`
	MoveProjectsTo *string `json:"moveProjectsTo" doc:"The status (name, type or id) its projects move to; required when projects use it."`
}

func (self *ProjectService) statusValues(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.ProjectStatus, error) {
	values := make([]models.ProjectStatus, 0, len(ids))
	counts, err := self.repo.Project().StatusProjectCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		status, err := self.repo.Project().Status(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		values = append(values, models.ProjectStatusOf(status, counts[id]))
	}
	return values, nil
}

func (self *ProjectService) ListStatuses(ctx context.Context, actor *auth.Actor, input *ListStatusesInput) (args.PageOf[models.ProjectStatus], error) {
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	offset, err := page.Offset()
	if err != nil {
		return args.PageOf[models.ProjectStatus]{}, err
	}
	statuses, err := self.repo.Project().Statuses(ctx, actor.Workspace, page.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.ProjectStatus]{}, err
	}
	ids := make([]uuid.UUID, len(statuses))
	for i, s := range statuses {
		ids[i] = s.ID
	}
	values, err := self.statusValues(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.ProjectStatus]{}, err
	}
	return args.WrapOf(page, values)
}

func checkStatusType(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !slices.Contains(projectStatusTypes, lower) {
		return "", errdefs.BadRequest("A project status type is backlog, planned, started, paused, completed or canceled, not \"%s\"", value)
	}
	return lower, nil
}

func colorOrPick(color *string, name string) (string, error) {
	if color == nil {
		return utils.PickColor(name), nil
	}
	return utils.CheckColor(*color)
}

func (self *ProjectService) SaveStatus(ctx context.Context, actor *auth.Actor, input *SaveStatusInput) (models.ProjectStatus, error) {
	if err := actor.RequireAdmin(); err != nil {
		return models.ProjectStatus{}, err
	}
	var value models.ProjectStatus
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		if input.ID != nil {
			id, err = self.resolve.ProjectStatus(ctx, actor, *input.ID)
		} else {
			id, err = self.createStatus(ctx, tx, actor, input)
		}
		if err != nil {
			return err
		}
		if input.Name != nil {
			name, err := utils.NonEmpty(*input.Name, "name")
			if err != nil {
				return err
			}
			taken, err := self.repo.Project().StatusNameTaken(ctx, tx, actor.Workspace, id, name)
			if err != nil {
				return err
			}
			if taken {
				return errdefs.Conflict("There is already a project status \"%s\"", name)
			}
			if err := self.repo.Project().UpdateStatus(tx, id).SetName(name).Exec(ctx); err != nil {
				return err
			}
		}
		if input.Type != nil {
			kind, err := checkStatusType(*input.Type)
			if err != nil {
				return err
			}
			if err := self.repo.Project().UpdateStatus(tx, id).SetType(kind).Exec(ctx); err != nil {
				return err
			}
		}
		if input.Color != nil {
			color, err := utils.CheckColor(*input.Color)
			if err != nil {
				return err
			}
			if err := self.repo.Project().UpdateStatus(tx, id).SetColor(color).Exec(ctx); err != nil {
				return err
			}
		}
		if input.Description.Set {
			update := self.repo.Project().UpdateStatus(tx, id)
			if err := setOrClear(update, input.Description.Ptr(), (*ent.ProjectStatusUpdateOne).SetDescription, (*ent.ProjectStatusUpdateOne).ClearDescription).Exec(ctx); err != nil {
				return err
			}
		}
		if input.Position != nil {
			if err := self.repo.Project().UpdateStatus(tx, id).SetPosition(*input.Position).Exec(ctx); err != nil {
				return err
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Project().UpdateStatus(tx, id).SetUpdatedAt(now).Exec(ctx); err != nil {
			return err
		}
		values, err := self.statusValues(ctx, tx, []uuid.UUID{id})
		if err != nil {
			return err
		}
		value = values[0]
		return nil
	})
	return value, err
}

func (self *ProjectService) createStatus(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveStatusInput) (uuid.UUID, error) {
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	if input.Type == nil {
		return uuid.Nil, errdefs.BadRequest("type is required")
	}
	kind, err := checkStatusType(*input.Type)
	if err != nil {
		return uuid.Nil, err
	}
	color, err := colorOrPick(input.Color, name)
	if err != nil {
		return uuid.Nil, err
	}
	return self.repo.Project().CreateStatus(ctx, tx, actor.Workspace, name, kind, color)
}

func (self *ProjectService) DeleteStatus(ctx context.Context, actor *auth.Actor, input *DeleteStatusInput) (models.DeletedProjectStatus, error) {
	if err := actor.RequireAdmin(); err != nil {
		return models.DeletedProjectStatus{}, err
	}
	id, err := self.resolve.ProjectStatus(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedProjectStatus{}, err
	}
	var projects []uuid.UUID
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		remaining, err := self.repo.Project().OtherStatuses(ctx, tx, actor.Workspace, id)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return errdefs.BadRequest("The workspace keeps at least one project status")
		}
		if projects, err = self.repo.Project().ProjectsInStatus(ctx, tx, id); err != nil {
			return err
		}
		if len(projects) > 0 {
			if input.MoveProjectsTo == nil {
				return errdefs.BadRequest("%d projects use this status; pass moveProjectsTo", len(projects))
			}
			target, err := self.resolve.ProjectStatus(ctx, actor, *input.MoveProjectsTo)
			if err != nil {
				return err
			}
			if target == id {
				return errdefs.BadRequest("moveProjectsTo must be another status")
			}
			now, err := self.repo.Now(ctx, tx)
			if err != nil {
				return err
			}
			if err := self.repo.Project().MoveProjects(ctx, tx, id, target, now); err != nil {
				return err
			}
			for _, project := range projects {
				if err := self.stampStatus(ctx, tx, project); err != nil {
					return err
				}
			}
		}
		return self.repo.Project().DeleteStatus(ctx, tx, id)
	})
	if err != nil {
		return models.DeletedProjectStatus{}, err
	}
	return models.DeletedProjectStatus{Deleted: true, ID: id.String(), MovedProjects: len(projects)}, nil
}
