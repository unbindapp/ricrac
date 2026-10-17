package customer_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	customer_repo "ricrac/internal/repositories/customer"
)

func optionWhat(kind customer_repo.OptionKind) string {
	if kind == customer_repo.Tiers {
		return "customer tier"
	}
	return "customer status"
}

// optionID is a customer status or tier by id or name.
func (self *CustomerService) optionID(ctx context.Context, actor *auth.Actor, kind customer_repo.OptionKind, value string) (uuid.UUID, error) {
	rows, err := self.customers().Named(ctx, kind, actor.Workspace)
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
	names := make([]string, len(rows))
	for i, r := range rows {
		if strings.EqualFold(r.Name, value) {
			return r.ID, nil
		}
		names[i] = r.Name
	}
	list := "There are none."
	if len(names) > 0 {
		list = fmt.Sprintf("Options: %s.", strings.Join(names, ", "))
	}
	return uuid.Nil, errdefs.BadRequest("No %s %q. %s", optionWhat(kind), value, list)
}

// optionValues answers statuses; tiers have the same shape.
func (self *CustomerService) optionValues(ctx context.Context, tx repository.TxInterface, kind customer_repo.OptionKind, options []customer_repo.Option) ([]models.CustomerStatus, error) {
	ids := make([]uuid.UUID, len(options))
	for i, o := range options {
		ids[i] = o.ID
	}
	counts, err := self.customers().CustomerCounts(ctx, tx, kind, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.CustomerStatus, len(options))
	for i, o := range options {
		values[i] = models.CustomerStatus{
			ID: models.ID(o.ID), Name: o.Name, Color: o.Color, Description: o.Description, Position: o.Position,
			CustomerCount: counts[o.ID],
		}
	}
	return values, nil
}

func (self *CustomerService) listOptions(ctx context.Context, actor *auth.Actor, kind customer_repo.OptionKind, input *ListOptionsInput) (args.PageOf[models.CustomerStatus], error) {
	if err := actor.RequireMember(); err != nil {
		return args.PageOf[models.CustomerStatus]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.CustomerStatus]{}, err
	}
	options, err := self.customers().Options(ctx, kind, actor.Workspace, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.CustomerStatus]{}, err
	}
	values, err := self.optionValues(ctx, nil, kind, options)
	if err != nil {
		return args.PageOf[models.CustomerStatus]{}, err
	}
	return args.WrapOf(input.Page, values)
}

func (self *CustomerService) saveOption(ctx context.Context, actor *auth.Actor, kind customer_repo.OptionKind, input *SaveOptionInput) (models.CustomerStatus, error) {
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.optionID(ctx, actor, kind, *input.ID)
		if err != nil {
			return models.CustomerStatus{}, err
		}
		existing = &id
	}
	var name, color *string
	if input.Name != nil {
		n, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return models.CustomerStatus{}, err
		}
		name = &n
	}
	if input.Color != nil {
		checked, err := utils.CheckColor(*input.Color)
		if err != nil {
			return models.CustomerStatus{}, err
		}
		color = &checked
	}
	var value models.CustomerStatus
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if name != nil {
			taken, err := self.customers().OptionNameTaken(ctx, tx, kind, actor.Workspace, *name, existing)
			if err != nil {
				return err
			}
			if taken {
				return errdefs.Conflict("There already is a %s named %s", optionWhat(kind), *name)
			}
		}
		var id uuid.UUID
		if existing != nil {
			id = *existing
		} else {
			if name == nil {
				return errdefs.BadRequest("name is required to create one")
			}
			newColor := utils.PickColor(*name)
			if color != nil {
				newColor = *color
			}
			var err error
			if id, err = self.customers().CreateOption(ctx, tx, kind, actor.Workspace, *name, newColor); err != nil {
				return err
			}
		}
		err := self.customers().UpdateOption(ctx, tx, kind, id, customer_repo.OptionUpdate{
			Name: name, Color: color, Description: trimNullable(input.Description), Position: input.Position,
		})
		if err != nil {
			return err
		}
		option, err := self.customers().GetOption(ctx, tx, kind, id)
		if err != nil {
			return err
		}
		values, err := self.optionValues(ctx, tx, kind, []customer_repo.Option{option})
		if err != nil {
			return err
		}
		value = values[0]
		return nil
	})
	if err != nil {
		return models.CustomerStatus{}, err
	}
	return value, nil
}

func (self *CustomerService) deleteOption(ctx context.Context, actor *auth.Actor, kind customer_repo.OptionKind, input *IDInput) (models.DeletedCustomerOption, error) {
	id, err := self.optionID(ctx, actor, kind, input.ID)
	if err != nil {
		return models.DeletedCustomerOption{}, err
	}
	var cleared int
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var err error
		if cleared, err = self.customers().CustomersWith(ctx, tx, kind, id); err != nil {
			return err
		}
		return self.customers().DeleteOption(ctx, tx, kind, id)
	})
	if err != nil {
		return models.DeletedCustomerOption{}, err
	}
	return models.DeletedCustomerOption{ID: id.String(), Deleted: true, CustomersCleared: cleared}, nil
}

func tiers(statuses []models.CustomerStatus) []models.CustomerTier {
	converted := make([]models.CustomerTier, len(statuses))
	for i, s := range statuses {
		converted[i] = models.CustomerTier(s)
	}
	return converted
}

func (self *CustomerService) ListStatuses(ctx context.Context, actor *auth.Actor, input *ListOptionsInput) (args.PageOf[models.CustomerStatus], error) {
	return self.listOptions(ctx, actor, customer_repo.Statuses, input)
}

func (self *CustomerService) SaveStatus(ctx context.Context, actor *auth.Actor, input *SaveOptionInput) (models.CustomerStatus, error) {
	return self.saveOption(ctx, actor, customer_repo.Statuses, input)
}

func (self *CustomerService) DeleteStatus(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedCustomerOption, error) {
	return self.deleteOption(ctx, actor, customer_repo.Statuses, input)
}

func (self *CustomerService) ListTiers(ctx context.Context, actor *auth.Actor, input *ListOptionsInput) (args.PageOf[models.CustomerTier], error) {
	page, err := self.listOptions(ctx, actor, customer_repo.Tiers, input)
	if err != nil {
		return args.PageOf[models.CustomerTier]{}, err
	}
	return args.PageOf[models.CustomerTier]{Nodes: tiers(page.Nodes), PageInfo: page.PageInfo}, nil
}

func (self *CustomerService) SaveTier(ctx context.Context, actor *auth.Actor, input *SaveOptionInput) (models.CustomerTier, error) {
	value, err := self.saveOption(ctx, actor, customer_repo.Tiers, input)
	if err != nil {
		return models.CustomerTier{}, err
	}
	return models.CustomerTier(value), nil
}

func (self *CustomerService) DeleteTier(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedCustomerOption, error) {
	return self.deleteOption(ctx, actor, customer_repo.Tiers, input)
}
