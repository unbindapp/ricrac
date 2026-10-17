package customer_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customer"
	"ricrac/ent/customerneed"
	"ricrac/ent/schema/pgarray"
	"ricrac/internal/common/args"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i CustomerRepositoryInterface -p customer_repo -s CustomerRepository -o customer_repository_iface.go
type CustomerRepository struct {
	base *repository.BaseRepository
}

func NewCustomerRepository(db *ent.Client) *CustomerRepository {
	return &CustomerRepository{base: repository.NewBaseRepository(db)}
}

// Load is the customers with what their answer shows (owner, status, tier), in the order of ids.
func (self *CustomerRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Customer, error) {
	customers, err := self.base.Client(tx).Customer.Query().Where(customer.IDIn(ids...)).
		WithOwner().WithStatus().WithTier().All(ctx)
	if err != nil {
		return nil, err
	}
	return inOrder(ids, customers, func(c *ent.Customer) uuid.UUID { return c.ID }), nil
}

// NeedCounts is how many needs not archived each customer has.
func (self *CustomerRepository) NeedCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		CustomerID uuid.UUID `json:"customer_id"`
		Count      int       `json:"count"`
	}
	err := self.base.Client(tx).CustomerNeed.Query().
		Where(customerneed.CustomerIDIn(ids...), customerneed.ArchivedAtIsNil()).
		GroupBy(customerneed.FieldCustomerID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := map[uuid.UUID]int{}
	for _, row := range rows {
		counts[row.CustomerID] = row.Count
	}
	return counts, nil
}

type ListFilter struct {
	Workspace       uuid.UUID
	IncludeArchived bool
	Status          *uuid.UUID
	Tier            *uuid.UUID
	Owner           *uuid.UUID
	// An ILIKE pattern on the name or a domain.
	Pattern *string
	Limit   int64
	Offset  int64
}

// List is the customers by name.
func (self *CustomerRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Customer.Query().Where(customer.WorkspaceID(filter.Workspace))
	if !filter.IncludeArchived {
		query = query.Where(customer.ArchivedAtIsNil())
	}
	if filter.Status != nil {
		query = query.Where(customer.StatusID(*filter.Status))
	}
	if filter.Tier != nil {
		query = query.Where(customer.TierID(*filter.Tier))
	}
	if filter.Owner != nil {
		query = query.Where(customer.OwnerID(*filter.Owner))
	}
	if filter.Pattern != nil {
		query = query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw("("+s.C(customer.FieldName)+" ILIKE ? OR EXISTS (SELECT 1 FROM unnest("+
				s.C(customer.FieldDomains)+") d WHERE d ILIKE ?))", *filter.Pattern, *filter.Pattern))
		})
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(customer.FieldID))
		s.OrderExpr(entsql.Expr("lower(" + s.C(customer.FieldName) + ")"))
		s.OrderBy(s.C(customer.FieldID))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// DomainOwner is a customer other than except that has one of domains: its name and the domain.
func (self *CustomerRepository) DomainOwner(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, except *uuid.UUID, domains []string) (string, string, bool, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT c.name, d FROM customers c, unnest(c.domains) d
		 WHERE c.workspace_id = $1 AND c.id IS DISTINCT FROM $2 AND d = ANY($3) LIMIT 1`,
		workspace, except, domains)
	if err != nil {
		return "", "", false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", "", false, rows.Err()
	}
	var name, domain string
	err = rows.Scan(&name, &domain)
	return name, domain, err == nil, err
}

func (self *CustomerRepository) Create(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, slug, name string) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Customer.Create().SetWorkspaceID(workspace).SetSlugID(slug).SetName(name).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// UpdateInput leaves nil fields and fields not Set as they are.
type UpdateInput struct {
	Name        *string
	Domains     []string
	ExternalIDs []string
	LogoURL     args.Nullable[string]
	Owner       args.Nullable[uuid.UUID]
	Status      args.Nullable[uuid.UUID]
	Tier        args.Nullable[uuid.UUID]
	Revenue     args.Nullable[int64]
	Size        args.Nullable[int]
}

func (self *CustomerRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input UpdateInput, now time.Time) error {
	update := self.base.Client(tx).Customer.UpdateOneID(id).SetUpdatedAt(now)
	if input.Name != nil {
		update.SetName(*input.Name)
	}
	if input.Domains != nil {
		update.SetDomains(pgarray.Text(input.Domains))
	}
	if input.ExternalIDs != nil {
		update.SetExternalIds(pgarray.Text(input.ExternalIDs))
	}
	setOrClear(input.LogoURL, update.SetLogoURL, update.ClearLogoURL)
	setOrClear(input.Owner, update.SetOwnerID, update.ClearOwnerID)
	setOrClear(input.Status, update.SetStatusID, update.ClearStatusID)
	setOrClear(input.Tier, update.SetTierID, update.ClearTierID)
	setOrClear(input.Revenue, update.SetRevenue, update.ClearRevenue)
	setOrClear(input.Size, update.SetSize, update.ClearSize)
	return update.Exec(ctx)
}

// setOrClear sets a value given, clears a null one and leaves one left out.
func setOrClear[T, U any](value args.Nullable[T], set func(T) U, clear func() U) {
	switch {
	case !value.Set:
	case value.Valid:
		set(value.Value)
	default:
		clear()
	}
}

// Archive archives the customer unless it already is.
func (self *CustomerRepository) Archive(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Customer.Update().Where(customer.ID(id), customer.ArchivedAtIsNil()).
		SetArchivedAt(now).SetUpdatedAt(now).Exec(ctx)
}

func inOrder[T any](ids []uuid.UUID, items []T, id func(T) uuid.UUID) []T {
	byID := make(map[uuid.UUID]T, len(items))
	for _, item := range items {
		byID[id(item)] = item
	}
	ordered := make([]T, 0, len(ids))
	for _, key := range ids {
		if item, ok := byID[key]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}
