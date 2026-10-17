// Package automation_service is what happens on its own every minute: auto-archive, auto-close,
// the trash, snoozes, due dates and cleanups.
package automation_service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AutomationServiceInterface -p automation_service -s AutomationService -o automation_service_iface.go
type AutomationService struct {
	repo repositories.RepositoriesInterface
}

func NewAutomationService(repo repositories.RepositoriesInterface) *AutomationService {
	return &AutomationService{repo: repo}
}

// Run runs each automation in its own transaction.
func (self *AutomationService) Run(ctx context.Context) error {
	steps := []func(context.Context, repository.TxInterface) error{
		self.autoArchive, self.autoClose, self.emptyTrash, self.wakeSnoozed, self.notifyDue, self.cleanUp,
	}
	for _, step := range steps {
		if err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error { return step(ctx, tx) }); err != nil {
			return err
		}
	}
	return nil
}

func (self *AutomationService) autoArchive(ctx context.Context, tx repository.TxInterface) error {
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Automation().AutoArchive(ctx, tx, now)
}

func (self *AutomationService) autoClose(ctx context.Context, tx repository.TxInterface) error {
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	closable, err := self.repo.Automation().Closable(ctx, tx)
	if err != nil || len(closable) == 0 {
		return err
	}
	var ids []uuid.UUID
	for _, c := range closable {
		ids = append(ids, c.From, c.To)
	}
	states, err := self.repo.Automation().States(ctx, tx, ids)
	if err != nil {
		return err
	}
	for _, c := range closable {
		changes, err := json.Marshal(map[string]any{"state": map[string]any{
			"from": models.StateRefOf(states[c.From]), "to": models.StateRefOf(states[c.To]),
		}})
		if err != nil {
			return err
		}
		if err := self.repo.Automation().AutoClose(ctx, tx, c.Issue, c.To, changes, now); err != nil {
			return err
		}
	}
	return nil
}

func (self *AutomationService) emptyTrash(ctx context.Context, tx repository.TxInterface) error {
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Automation().EmptyTrash(ctx, tx, now)
}

func (self *AutomationService) wakeSnoozed(ctx context.Context, tx repository.TxInterface) error {
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Automation().WakeSnoozed(ctx, tx, now)
}

func (self *AutomationService) notifyDue(ctx context.Context, tx repository.TxInterface) error {
	return self.repo.Automation().NotifyDue(ctx, tx)
}

func (self *AutomationService) cleanUp(ctx context.Context, tx repository.TxInterface) error {
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Automation().CleanUp(ctx, tx, now)
}
