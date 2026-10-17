package project_service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

// mark changes the project with change (given the transaction's now) and emits action.
func (self *ProjectService) mark(ctx context.Context, actor *auth.Actor, id uuid.UUID, action string, change func(p *ent.Project, update *ent.ProjectUpdateOne, now time.Time) bool) (models.Project, error) {
	var value models.Project
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		p, err := self.repo.Project().Get(ctx, tx, id)
		if err != nil {
			return err
		}
		update := self.repo.Project().Update(tx, id)
		if change(p, update, now) {
			if err := update.SetUpdatedAt(now).Exec(ctx); err != nil {
				return err
			}
		}
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		team, err := self.repo.Project().WebhookTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Project", action, value, nil, team)
	})
	return value, err
}

func (self *ProjectService) Trash(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Project, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Project{}, err
	}
	id, err := self.resolve.Project(ctx, actor, input.ID)
	if err != nil {
		return models.Project{}, err
	}
	return self.mark(ctx, actor, id, "remove", func(p *ent.Project, update *ent.ProjectUpdateOne, now time.Time) bool {
		update.SetTrashedAt(now)
		return p.TrashedAt == nil
	})
}

func (self *ProjectService) Restore(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Project, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Project{}, err
	}
	id, found, err := self.repo.Project().Trashed(ctx, actor.Workspace, strings.TrimSpace(input.ID), TrashedSlugTail(input.ID))
	if err != nil {
		return models.Project{}, err
	}
	if !found {
		return models.Project{}, errdefs.NotFound("No %s \"%s\" in the trash", "project", strings.TrimSpace(input.ID))
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.Project{}, err
	}
	visible, err := self.repo.Project().IsVisible(ctx, id, teams)
	if err != nil {
		return models.Project{}, err
	}
	if !visible {
		return models.Project{}, errdefs.NotFound("No project \"%s\" in the trash", strings.TrimSpace(input.ID))
	}
	return self.mark(ctx, actor, id, "update", func(p *ent.Project, update *ent.ProjectUpdateOne, now time.Time) bool {
		update.ClearTrashedAt()
		return true
	})
}

func (self *ProjectService) Archive(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Project, error) {
	id, err := self.resolve.Project(ctx, actor, input.ID)
	if err != nil {
		return models.Project{}, err
	}
	return self.mark(ctx, actor, id, "update", func(p *ent.Project, update *ent.ProjectUpdateOne, now time.Time) bool {
		update.SetArchivedAt(now)
		return p.ArchivedAt == nil
	})
}

func (self *ProjectService) Unarchive(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Project, error) {
	id, err := self.resolve.Project(ctx, actor, input.ID)
	if err != nil {
		return models.Project{}, err
	}
	return self.mark(ctx, actor, id, "update", func(p *ent.Project, update *ent.ProjectUpdateOne, now time.Time) bool {
		update.ClearArchivedAt()
		return true
	})
}

// TrashedSlugTail is the id a slug, link or path ends in: my-project-0a1b2c3d4e5f.
func TrashedSlugTail(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.IndexAny(value, "?#"); i >= 0 {
		value = value[:i]
	}
	segment := strings.TrimRight(value, "/")
	segment = segment[strings.LastIndex(segment, "/")+1:]
	i := strings.LastIndex(segment, "-")
	if i < 0 {
		return segment
	}
	if tail := segment[i+1:]; len(tail) == 12 && utils.IsHex(tail) {
		return tail
	}
	return segment
}
