package customer_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customer"
	"ricrac/ent/customerstatus"
	"ricrac/ent/customertier"
	"ricrac/internal/common/args"
	repository "ricrac/internal/repositories"
)

// OptionKind is customer statuses or tiers, which have the same shape.
type OptionKind int

const (
	Statuses OptionKind = iota
	Tiers
)

type Option struct {
	ID          uuid.UUID
	Name        string
	Color       string
	Description *string
	Position    float64
}

func statusOption(s *ent.CustomerStatus) Option {
	return Option{ID: s.ID, Name: s.Name, Color: s.Color, Description: s.Description, Position: s.Position}
}

func tierOption(t *ent.CustomerTier) Option {
	return Option{ID: t.ID, Name: t.Name, Color: t.Color, Description: t.Description, Position: t.Position}
}

func optionsOf[T any](items []T, convert func(T) Option) []Option {
	options := make([]Option, len(items))
	for i, item := range items {
		options[i] = convert(item)
	}
	return options
}

func byPosition(s *entsql.Selector) {
	s.OrderBy(s.C("position"))
}

func byPositionAndName(s *entsql.Selector) {
	s.OrderBy(s.C("position"), s.C("name"))
}

// Named is every option of the workspace by position, for finding one by name.
func (self *CustomerRepository) Named(ctx context.Context, kind OptionKind, workspace uuid.UUID) ([]Option, error) {
	client := self.base.DB
	if kind == Tiers {
		tiers, err := client.CustomerTier.Query().Where(customertier.WorkspaceID(workspace)).Order(byPosition).All(ctx)
		return optionsOf(tiers, tierOption), err
	}
	statuses, err := client.CustomerStatus.Query().Where(customerstatus.WorkspaceID(workspace)).Order(byPosition).All(ctx)
	return optionsOf(statuses, statusOption), err
}

// Options is a page of the workspace's options by position, then name.
func (self *CustomerRepository) Options(ctx context.Context, kind OptionKind, workspace uuid.UUID, limit, offset int64) ([]Option, error) {
	client := self.base.DB
	if kind == Tiers {
		tiers, err := client.CustomerTier.Query().Where(customertier.WorkspaceID(workspace)).Order(byPositionAndName).
			Limit(int(limit)).Offset(int(offset)).All(ctx)
		return optionsOf(tiers, tierOption), err
	}
	statuses, err := client.CustomerStatus.Query().Where(customerstatus.WorkspaceID(workspace)).Order(byPositionAndName).
		Limit(int(limit)).Offset(int(offset)).All(ctx)
	return optionsOf(statuses, statusOption), err
}

func (self *CustomerRepository) GetOption(ctx context.Context, tx repository.TxInterface, kind OptionKind, id uuid.UUID) (Option, error) {
	client := self.base.Client(tx)
	if kind == Tiers {
		tier, err := client.CustomerTier.Get(ctx, id)
		if err != nil {
			return Option{}, err
		}
		return tierOption(tier), nil
	}
	status, err := client.CustomerStatus.Get(ctx, id)
	if err != nil {
		return Option{}, err
	}
	return statusOption(status), nil
}

func optionColumn(kind OptionKind) string {
	if kind == Tiers {
		return customer.FieldTierID
	}
	return customer.FieldStatusID
}

// CustomerCounts is how many customers not archived have each option.
func (self *CustomerRepository) CustomerCounts(ctx context.Context, tx repository.TxInterface, kind OptionKind, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	column := optionColumn(kind)
	var rows []struct {
		Option uuid.UUID `json:"option"`
		Count  int       `json:"count"`
	}
	err := self.base.Client(tx).Customer.Query().
		Where(customer.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(entsql.In(s.C(column), anys(ids)...))
		}).
		Modify(func(s *entsql.Selector) {
			s.Select(entsql.As(s.C(column), "option"), entsql.As(entsql.Count("*"), "count")).GroupBy(s.C(column))
		}).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := map[uuid.UUID]int{}
	for _, row := range rows {
		counts[row.Option] = row.Count
	}
	return counts, nil
}

// CustomersWith is how many customers, archived ones too, have the option.
func (self *CustomerRepository) CustomersWith(ctx context.Context, tx repository.TxInterface, kind OptionKind, id uuid.UUID) (int, error) {
	query := self.base.Client(tx).Customer.Query()
	if kind == Tiers {
		return query.Where(customer.TierID(id)).Count(ctx)
	}
	return query.Where(customer.StatusID(id)).Count(ctx)
}

// OptionNameTaken says whether an option other than except has this name, in any case.
func (self *CustomerRepository) OptionNameTaken(ctx context.Context, tx repository.TxInterface, kind OptionKind, workspace uuid.UUID, name string, except *uuid.UUID) (bool, error) {
	sameName := func(s *entsql.Selector) {
		s.Where(repository.Raw("lower("+s.C("name")+") = lower(?)", name))
		if except != nil {
			s.Where(entsql.NEQ(s.C("id"), *except))
		}
	}
	client := self.base.Client(tx)
	if kind == Tiers {
		return client.CustomerTier.Query().Where(customertier.WorkspaceID(workspace), sameName).Exist(ctx)
	}
	return client.CustomerStatus.Query().Where(customerstatus.WorkspaceID(workspace), sameName).Exist(ctx)
}

// CreateOption adds an option after the others.
func (self *CustomerRepository) CreateOption(ctx context.Context, tx repository.TxInterface, kind OptionKind, workspace uuid.UUID, name, color string) (uuid.UUID, error) {
	client := self.base.Client(tx)
	var last []struct {
		Max *float64 `json:"max"`
	}
	var err error
	if kind == Tiers {
		err = client.CustomerTier.Query().Where(customertier.WorkspaceID(workspace)).Aggregate(ent.Max(customertier.FieldPosition)).Scan(ctx, &last)
	} else {
		err = client.CustomerStatus.Query().Where(customerstatus.WorkspaceID(workspace)).Aggregate(ent.Max(customerstatus.FieldPosition)).Scan(ctx, &last)
	}
	if err != nil {
		return uuid.Nil, err
	}
	position := 0.0
	if len(last) > 0 && last[0].Max != nil {
		position = *last[0].Max + 1
	}
	if kind == Tiers {
		created, err := client.CustomerTier.Create().SetWorkspaceID(workspace).SetName(name).SetColor(color).SetPosition(position).Save(ctx)
		if err != nil {
			return uuid.Nil, err
		}
		return created.ID, nil
	}
	created, err := client.CustomerStatus.Create().SetWorkspaceID(workspace).SetName(name).SetColor(color).SetPosition(position).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// OptionUpdate leaves nil fields and a Description not Set as they are.
type OptionUpdate struct {
	Name        *string
	Color       *string
	Description args.Nullable[string]
	Position    *float64
}

func (self *CustomerRepository) UpdateOption(ctx context.Context, tx repository.TxInterface, kind OptionKind, id uuid.UUID, input OptionUpdate) error {
	client := self.base.Client(tx)
	if kind == Tiers {
		update := client.CustomerTier.UpdateOneID(id).SetNillableName(input.Name).SetNillableColor(input.Color).SetNillablePosition(input.Position)
		setOrClear(input.Description, update.SetDescription, update.ClearDescription)
		return update.Exec(ctx)
	}
	update := client.CustomerStatus.UpdateOneID(id).SetNillableName(input.Name).SetNillableColor(input.Color).SetNillablePosition(input.Position)
	setOrClear(input.Description, update.SetDescription, update.ClearDescription)
	return update.Exec(ctx)
}

func (self *CustomerRepository) DeleteOption(ctx context.Context, tx repository.TxInterface, kind OptionKind, id uuid.UUID) error {
	client := self.base.Client(tx)
	if kind == Tiers {
		return client.CustomerTier.DeleteOneID(id).Exec(ctx)
	}
	return client.CustomerStatus.DeleteOneID(id).Exec(ctx)
}

func anys(ids []uuid.UUID) []any {
	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}
	return values
}
