package customer_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	customer_repo "ricrac/internal/repositories/customer"
)

func (self *CustomerService) ListNeeds(ctx context.Context, actor *auth.Actor, input *ListNeedsInput) (args.PageOf[models.CustomerNeed], error) {
	if err := actor.RequireMember(); err != nil {
		return args.PageOf[models.CustomerNeed]{}, err
	}
	viewer, err := self.viewer(ctx, actor)
	if err != nil {
		return args.PageOf[models.CustomerNeed]{}, err
	}
	filter := customer_repo.NeedFilter{Workspace: actor.Workspace, Viewer: viewer}
	if input.Customer != nil {
		id, err := self.customerID(ctx, actor, *input.Customer)
		if err != nil {
			return args.PageOf[models.CustomerNeed]{}, err
		}
		filter.Customer = &id
	}
	if input.Issue != nil {
		id, err := self.resolve.Issue(ctx, actor, *input.Issue)
		if err != nil {
			return args.PageOf[models.CustomerNeed]{}, err
		}
		filter.Issue = &id
	}
	if input.Project != nil {
		id, err := self.resolve.Project(ctx, actor, *input.Project)
		if err != nil {
			return args.PageOf[models.CustomerNeed]{}, err
		}
		filter.Project = &id
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.CustomerNeed]{}, err
	}
	filter.Limit, filter.Offset = input.Size()+1, offset
	ids, err := self.customers().Needs(ctx, filter)
	if err != nil {
		return args.PageOf[models.CustomerNeed]{}, err
	}
	needs, err := self.needValues(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.CustomerNeed]{}, err
	}
	return args.WrapOf(input.Page, needs)
}

func (self *CustomerService) needID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	if err := actor.RequireMember(); err != nil {
		return uuid.Nil, err
	}
	id, err := self.resolve.ByID(ctx, actor, "customer_needs", "customer need", value)
	if err != nil {
		return uuid.Nil, err
	}
	viewer, err := self.viewer(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	visible, err := self.customers().NeedVisible(ctx, id, viewer)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, errdefs.NotFound("No customer need %q", strings.TrimSpace(value))
	}
	return id, nil
}

func (self *CustomerService) SaveNeed(ctx context.Context, actor *auth.Actor, input *SaveNeedInput) (models.CustomerNeed, error) {
	if err := actor.RequireMember(); err != nil {
		return models.CustomerNeed{}, err
	}
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.needID(ctx, actor, *input.ID)
		if err != nil {
			return models.CustomerNeed{}, err
		}
		existing = &id
	}
	if input.Issue != nil && input.Project != nil {
		return models.CustomerNeed{}, errdefs.BadRequest("A need is for an issue or a project, not both")
	}
	if existing == nil && input.Issue == nil && input.Project == nil {
		return models.CustomerNeed{}, errdefs.BadRequest("Pass the issue or the project the customer needs")
	}
	var customer, issue, project *uuid.UUID
	switch {
	case input.Customer != nil:
		id, err := self.customerID(ctx, actor, *input.Customer)
		if err != nil {
			return models.CustomerNeed{}, err
		}
		customer = &id
	case existing == nil:
		return models.CustomerNeed{}, errdefs.BadRequest("customer is required to create a need")
	}
	if input.Issue != nil {
		id, err := self.resolve.Issue(ctx, actor, *input.Issue)
		if err != nil {
			return models.CustomerNeed{}, err
		}
		issue = &id
	}
	if input.Project != nil {
		id, err := self.resolve.Project(ctx, actor, *input.Project)
		if err != nil {
			return models.CustomerNeed{}, err
		}
		project = &id
	}
	if input.Priority != nil && (*input.Priority < 0 || *input.Priority > 1) {
		return models.CustomerNeed{}, errdefs.BadRequest("priority is 0 (normal) or 1 (important)")
	}
	var priority *int
	if input.Priority != nil {
		priority = new(int(*input.Priority))
	}
	var value models.CustomerNeed
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var before *any
		if existing != nil {
			id = *existing
			current, err := self.needValue(ctx, tx, id)
			if err != nil {
				return err
			}
			before = new(any(current))
		} else {
			var err error
			if id, err = self.customers().CreateNeed(ctx, tx, actor.Workspace, actor.User); err != nil {
				return err
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		err = self.customers().UpdateNeed(ctx, tx, id, customer_repo.NeedUpdate{
			Customer: customer, Target: issue != nil || project != nil, Issue: issue, Project: project, Body: input.Body,
			Priority: priority, AttachmentURL: trimNullable(input.AttachmentURL),
		}, now)
		if err != nil {
			return err
		}
		if value, err = self.needValue(ctx, tx, id); err != nil {
			return err
		}
		team, err := self.customers().NeedTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		return self.emitCreatedOrChanged(ctx, tx, actor, "CustomerNeed", before, value, team)
	})
	if err != nil {
		return models.CustomerNeed{}, err
	}
	return value, nil
}

func (self *CustomerService) DeleteNeed(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedCustomerNeed, error) {
	id, err := self.needID(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedCustomerNeed{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.needValue(ctx, tx, id)
		if err != nil {
			return err
		}
		team, err := self.customers().NeedTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := self.customers().DeleteNeed(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "CustomerNeed", "remove", value, nil, team)
	})
	if err != nil {
		return models.DeletedCustomerNeed{}, err
	}
	return models.DeletedCustomerNeed{ID: id.String(), Deleted: true}, nil
}
