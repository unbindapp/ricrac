package customer_service

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	customer_repo "ricrac/internal/repositories/customer"
)

func (self *CustomerService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Customer], error) {
	if err := actor.RequireMember(); err != nil {
		return args.PageOf[models.Customer]{}, err
	}
	filter := customer_repo.ListFilter{Workspace: actor.Workspace, IncludeArchived: input.IncludeArchived}
	if input.Status != nil {
		id, err := self.optionID(ctx, actor, customer_repo.Statuses, *input.Status)
		if err != nil {
			return args.PageOf[models.Customer]{}, err
		}
		filter.Status = &id
	}
	if input.Tier != nil {
		id, err := self.optionID(ctx, actor, customer_repo.Tiers, *input.Tier)
		if err != nil {
			return args.PageOf[models.Customer]{}, err
		}
		filter.Tier = &id
	}
	if input.Owner != nil {
		id, err := self.resolve.User(ctx, actor, *input.Owner)
		if err != nil {
			return args.PageOf[models.Customer]{}, err
		}
		filter.Owner = &id
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Pattern = new("%" + utils.EscapeLike(query) + "%")
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Customer]{}, err
	}
	filter.Limit, filter.Offset = input.Size()+1, offset
	ids, err := self.customers().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.Customer]{}, err
	}
	customers, err := self.values(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.Customer]{}, err
	}
	return args.WrapOf(input.Page, customers)
}

func (self *CustomerService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.CustomerDetail, error) {
	id, err := self.customerID(ctx, actor, input.ID)
	if err != nil {
		return models.CustomerDetail{}, err
	}
	customer, err := self.value(ctx, nil, id)
	if err != nil {
		return models.CustomerDetail{}, err
	}
	viewer, err := self.viewer(ctx, actor)
	if err != nil {
		return models.CustomerDetail{}, err
	}
	needIDs, err := self.customers().CustomerNeeds(ctx, id, viewer)
	if err != nil {
		return models.CustomerDetail{}, err
	}
	needs, err := self.needValues(ctx, nil, needIDs)
	if err != nil {
		return models.CustomerDetail{}, err
	}
	return models.CustomerDetail{Customer: customer, Needs: needs}, nil
}

func customerDomain(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	host, found := strings.CutPrefix(lower, "https://")
	if !found {
		host = strings.TrimPrefix(lower, "http://")
	}
	host, _, _ = strings.Cut(host, "/")
	if !strings.Contains(host, ".") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") ||
		strings.ContainsFunc(host, unicode.IsSpace) {
		return "", errdefs.BadRequest("%q isn't a domain like acme.com", value)
	}
	return host, nil
}

func (self *CustomerService) domains(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, customer *uuid.UUID, values []string) ([]string, error) {
	domains := []string{}
	for _, value := range values {
		domain, err := customerDomain(value)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(domains, domain) {
			domains = append(domains, domain)
		}
	}
	name, domain, taken, err := self.customers().DomainOwner(ctx, tx, actor.Workspace, customer, domains)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errdefs.Conflict("%s already belongs to the customer %s", domain, name)
	}
	return domains, nil
}

func (self *CustomerService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.Customer, error) {
	if err := actor.RequireMember(); err != nil {
		return models.Customer{}, err
	}
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.customerID(ctx, actor, *input.ID)
		if err != nil {
			return models.Customer{}, err
		}
		existing = &id
	}
	owner, err := resolveNullable(input.Owner, func(v string) (uuid.UUID, error) { return self.resolve.User(ctx, actor, v) })
	if err != nil {
		return models.Customer{}, err
	}
	status, err := resolveNullable(input.Status, func(v string) (uuid.UUID, error) {
		return self.optionID(ctx, actor, customer_repo.Statuses, v)
	})
	if err != nil {
		return models.Customer{}, err
	}
	tier, err := resolveNullable(input.Tier, func(v string) (uuid.UUID, error) {
		return self.optionID(ctx, actor, customer_repo.Tiers, v)
	})
	if err != nil {
		return models.Customer{}, err
	}
	var name *string
	if input.Name != nil {
		n, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return models.Customer{}, err
		}
		name = &n
	}
	var externalIDs []string
	if input.ExternalIDs != nil {
		externalIDs = []string{}
		for _, id := range input.ExternalIDs {
			id = strings.TrimSpace(id)
			if id != "" && !slices.Contains(externalIDs, id) {
				externalIDs = append(externalIDs, id)
			}
		}
	}
	logoURL := trimNullable(input.LogoURL)
	if input.Revenue.Valid && input.Revenue.Value < 0 {
		return models.Customer{}, errdefs.BadRequest("revenue can't be negative")
	}
	if input.Size.Valid && input.Size.Value < 0 {
		return models.Customer{}, errdefs.BadRequest("size can't be negative")
	}
	size := args.Nullable[int]{Set: input.Size.Set, Valid: input.Size.Valid, Value: int(input.Size.Value)}
	var value models.Customer
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var domains []string
		if input.Domains != nil {
			var err error
			if domains, err = self.domains(ctx, tx, actor, existing, input.Domains); err != nil {
				return err
			}
		}
		var id uuid.UUID
		var before *any
		if existing != nil {
			id = *existing
			current, err := self.value(ctx, tx, id)
			if err != nil {
				return err
			}
			before = new(any(current))
		} else {
			if name == nil {
				return errdefs.BadRequest("name is required to create a customer")
			}
			var err error
			if id, err = self.customers().Create(ctx, tx, actor.Workspace, utils.SlugID(), *name); err != nil {
				return err
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		err = self.customers().Update(ctx, tx, id, customer_repo.UpdateInput{
			Name: name, Domains: domains, ExternalIDs: externalIDs, LogoURL: logoURL, Owner: owner, Status: status,
			Tier: tier, Revenue: input.Revenue, Size: size,
		}, now)
		if err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.emitCreatedOrChanged(ctx, tx, actor, "Customer", before, value, nil)
	})
	if err != nil {
		return models.Customer{}, err
	}
	return value, nil
}

func (self *CustomerService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Customer, error) {
	id, err := self.customerID(ctx, actor, input.ID)
	if err != nil {
		return models.Customer{}, err
	}
	var value models.Customer
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.customers().Archive(ctx, tx, id, now); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Customer", "remove", value, nil, nil)
	})
	if err != nil {
		return models.Customer{}, err
	}
	return value, nil
}
