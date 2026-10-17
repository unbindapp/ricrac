package initiative_service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	project_service "ricrac/internal/services/project"
)

// mark changes the initiative with change (given the transaction's now) and emits action.
func (self *InitiativeService) mark(ctx context.Context, actor *auth.Actor, id uuid.UUID, action string, change func(i *ent.Initiative, update *ent.InitiativeUpdateOne, now time.Time) bool) (models.Initiative, error) {
	var value models.Initiative
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		initiative, err := self.repo.Initiative().Get(ctx, tx, id)
		if err != nil {
			return err
		}
		update := self.repo.Initiative().Update(tx, id)
		if change(initiative, update, now) {
			if err := update.SetUpdatedAt(now).Exec(ctx); err != nil {
				return err
			}
		}
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Initiative", action, value, nil, nil)
	})
	return value, err
}

func (self *InitiativeService) find(ctx context.Context, actor *auth.Actor, input *IDInput) (uuid.UUID, error) {
	if err := actor.RequireMember(); err != nil {
		return uuid.Nil, err
	}
	return self.resolve.Initiative(ctx, actor, input.ID)
}

func (self *InitiativeService) Trash(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Initiative, error) {
	id, err := self.find(ctx, actor, input)
	if err != nil {
		return models.Initiative{}, err
	}
	return self.mark(ctx, actor, id, "remove", func(i *ent.Initiative, update *ent.InitiativeUpdateOne, now time.Time) bool {
		update.SetTrashedAt(now)
		return i.TrashedAt == nil
	})
}

func (self *InitiativeService) Restore(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Initiative, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Initiative{}, err
	}
	id, found, err := self.repo.Initiative().Trashed(ctx, actor.Workspace, strings.TrimSpace(input.ID), project_service.TrashedSlugTail(input.ID))
	if err != nil {
		return models.Initiative{}, err
	}
	if !found {
		return models.Initiative{}, errdefs.NotFound("No %s \"%s\" in the trash", "initiative", strings.TrimSpace(input.ID))
	}
	return self.mark(ctx, actor, id, "update", func(i *ent.Initiative, update *ent.InitiativeUpdateOne, now time.Time) bool {
		update.ClearTrashedAt()
		return true
	})
}

func (self *InitiativeService) Archive(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Initiative, error) {
	id, err := self.find(ctx, actor, input)
	if err != nil {
		return models.Initiative{}, err
	}
	return self.mark(ctx, actor, id, "update", func(i *ent.Initiative, update *ent.InitiativeUpdateOne, now time.Time) bool {
		update.SetArchivedAt(now)
		return i.ArchivedAt == nil
	})
}

func (self *InitiativeService) Unarchive(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Initiative, error) {
	id, err := self.find(ctx, actor, input)
	if err != nil {
		return models.Initiative{}, err
	}
	return self.mark(ctx, actor, id, "update", func(i *ent.Initiative, update *ent.InitiativeUpdateOne, now time.Time) bool {
		update.ClearArchivedAt()
		return true
	})
}
