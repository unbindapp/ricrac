package ops

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Customers: the companies a workspace builds for, with their status, tier and owner, and
// customer needs (requests) linking them to issues and projects. Guests can't see them.

func customerOps() []*Op {
	return []*Op{
		op("list_customers", app.Read, []string{"GET /customers"},
			"List customers by name: domains, external ids, owner, status, tier, revenue, size and how many needs "+
				"(requests) they have. Narrow by status, tier, owner or a query matching the name or a domain. Archived "+
				"customers are left out unless includeArchived is set.",
			listCustomers).returns(ResultOf[PageOf[Customer]]()),
		op("get_customer", app.Read, []string{"GET /customers/{id}"},
			"Read a customer by id, slug, name, domain or external id, with their needs: what they asked for, linked to "+
				"an issue or a project.",
			getCustomer).returns(ResultOf[CustomerDetail]()),
		op("save_customer", app.Write, []string{"POST /customers", "PATCH /customers/{id}"},
			"Create a customer (omit id; name is required) or update one (pass id: id, slug, name or domain). Omitted "+
				"fields stay as they are; null clears a field. Domains are unique within the workspace. Status and tier "+
				"take a name or id (see list_customer_statuses and list_customer_tiers); owner a person (id, name, email "+
				"or \"me\").",
			saveCustomer).returns(ResultOf[Customer]()),
		op("delete_customer", app.Write, []string{"DELETE /customers/{id}"},
			"Archive a customer. Their needs stay on the issues and projects they are linked to.",
			deleteCustomer).returns(ResultOf[Customer]()),
		op("list_customer_statuses", app.Read, []string{"GET /customer-statuses"},
			"The customer statuses of the workspace (e.g. Active, Prospect, Churned) in order.",
			func(c *app.Ctx, args ListOptionsArgs) (any, error) { return listOptions(c, customerStatuses, args) }).returns(ResultOf[PageOf[CustomerStatus]]()),
		op("save_customer_status", app.AdminAccess, []string{"POST /customer-statuses", "PATCH /customer-statuses/{id}"},
			"Create a customer status (omit id; name is required) or update one (pass its id or name). Admins only.",
			func(c *app.Ctx, args SaveOptionArgs) (any, error) { return saveOption(c, customerStatuses, args) }).returns(ResultOf[CustomerStatus]()),
		op("delete_customer_status", app.AdminAccess, []string{"DELETE /customer-statuses/{id}"},
			"Delete a customer status (id or name). Customers with it are left without a status. Admins only.",
			func(c *app.Ctx, args IDArgs) (any, error) { return deleteOption(c, customerStatuses, args) }).returns(ResultOf[DeletedCustomerOption]()),
		op("list_customer_tiers", app.Read, []string{"GET /customer-tiers"},
			"The customer tiers of the workspace (e.g. Enterprise, Growth) in order.",
			func(c *app.Ctx, args ListOptionsArgs) (any, error) { return listOptions(c, customerTiers, args) }).returns(ResultOf[PageOf[CustomerTier]]()),
		op("save_customer_tier", app.AdminAccess, []string{"POST /customer-tiers", "PATCH /customer-tiers/{id}"},
			"Create a customer tier (omit id; name is required) or update one (pass its id or name). Admins only.",
			func(c *app.Ctx, args SaveOptionArgs) (any, error) { return saveOption(c, customerTiers, args) }).returns(ResultOf[CustomerTier]()),
		op("delete_customer_tier", app.AdminAccess, []string{"DELETE /customer-tiers/{id}"},
			"Delete a customer tier (id or name). Customers in it are left without a tier. Admins only.",
			func(c *app.Ctx, args IDArgs) (any, error) { return deleteOption(c, customerTiers, args) }).returns(ResultOf[DeletedCustomerOption]()),
		op("list_customer_needs", app.Read, []string{"GET /customer-needs"},
			"List customer needs (requests), newest first, for a customer, an issue or a project: the customer, the "+
				"issue or project, the request text, priority (0 normal, 1 important) and a link to the source.",
			listNeeds).returns(ResultOf[PageOf[CustomerNeed]]()),
		op("save_customer_need", app.Write, []string{"POST /customer-needs", "PATCH /customer-needs/{id}"},
			"Record what a customer asked for (omit id; customer and exactly one of issue or project are required) or "+
				"change a need (pass id). Issues take an identifier like ENG-123; projects a name, slug or id.",
			saveNeed).returns(ResultOf[CustomerNeed]()),
		op("delete_customer_need", app.Write, []string{"DELETE /customer-needs/{id}"},
			"Delete a customer need.",
			deleteNeed).returns(ResultOf[DeletedCustomerNeed]()),
	}
}

/* Representation */

const customerJSON = `jsonb_build_object(
	'id', c.id,
	'slugId', c.slug_id,
	'name', c.name,
	'domains', to_jsonb(c.domains),
	'externalIds', to_jsonb(c.external_ids),
	'logoUrl', c.logo_url,
	'owner', user_ref(c.owner_id),
	'status', customer_status_ref(c.status_id),
	'tier', customer_tier_ref(c.tier_id),
	'revenue', c.revenue,
	'size', c.size,
	'needCount', (SELECT count(*) FROM customer_needs n WHERE n.customer_id = c.id AND n.archived_at IS NULL),
	'archivedAt', c.archived_at,
	'createdAt', c.created_at,
	'updatedAt', c.updated_at
)`

const needJSON = `jsonb_build_object(
	'id', n.id,
	'customer', customer_ref(n.customer_id),
	'issue', issue_ref(n.issue_id),
	'project', project_ref(n.project_id),
	'body', n.body,
	'priority', n.priority,
	'attachmentUrl', n.attachment_url,
	'creator', user_ref(n.creator_id),
	'createdAt', n.created_at,
	'updatedAt', n.updated_at
)`

// pushNeedVisible adds the condition that need n points at an issue or project the caller can see.
func pushNeedVisible(q *app.SQL, c *app.Ctx) error {
	teams, err := c.TeamIDs()
	if err != nil {
		return err
	}
	q.Push(" AND (n.issue_id IS NULL OR EXISTS (SELECT 1 FROM issues i WHERE i.id = n.issue_id AND i.trashed_at IS NULL " +
		"AND (i.team_id = ANY(").Bind(teams)
	q.Push(") OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = i.id AND sh.user_id = ").Bind(c.User)
	q.Push(")))) AND (n.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p JOIN project_teams pt ON pt.project_id = p.id " +
		"WHERE p.id = n.project_id AND p.trashed_at IS NULL AND pt.team_id = ANY(").Bind(teams)
	q.Push(")))")
	return nil
}

func customerValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, "SELECT "+customerJSON+" FROM customers c WHERE c.id = $1", id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such customer")
	}
	return value, nil
}

func needValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, "SELECT "+needJSON+" FROM customer_needs n WHERE n.id = $1", id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such customer need")
	}
	return value, nil
}

func customerID(c *app.Ctx, value string) (uuid.UUID, error) {
	if err := c.RequireMember(); err != nil {
		return uuid.Nil, err
	}
	return resolveCustomer(c, value)
}

// trimNullable trims a nullable text, turning a blank one into null.
func trimNullable(value Nullable[string]) Nullable[string] {
	if !value.Valid {
		return value
	}
	if trimmed := strings.TrimSpace(value.Value); trimmed != "" {
		return some(trimmed)
	}
	return Nullable[string]{Set: true}
}

// emitCreatedOrChanged emits create without a previous value, else update when something changed.
func emitCreatedOrChanged(c *app.Ctx, tx app.Querier, kind string, before, value M, team *uuid.UUID) error {
	if before == nil {
		return emit(c, tx, kind, "create", value, nil, team)
	}
	from := changedFields(before, value)
	if len(from) == 0 {
		return nil
	}
	return emit(c, tx, kind, "update", value, from, team)
}

/* Customers */

type ListCustomersArgs struct {
	Status          *string `json:"status" doc:"Customer status name or id."`
	Tier            *string `json:"tier" doc:"Customer tier name or id."`
	Owner           *string `json:"owner" doc:"Owner: id, name, email or \"me\"."`
	Query           *string `json:"query" doc:"Words in the name or a domain."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	Page
}

func listCustomers(c *app.Ctx, args ListCustomersArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	q := app.NewSQL("SELECT " + customerJSON + " FROM customers c WHERE c.workspace_id = ").Bind(c.Workspace)
	if !args.IncludeArchived {
		q.Push(" AND c.archived_at IS NULL")
	}
	if args.Status != nil {
		id, err := optionID(c, customerStatuses, *args.Status)
		if err != nil {
			return nil, err
		}
		q.Push(" AND c.status_id = ").Bind(id)
	}
	if args.Tier != nil {
		id, err := optionID(c, customerTiers, *args.Tier)
		if err != nil {
			return nil, err
		}
		q.Push(" AND c.tier_id = ").Bind(id)
	}
	if args.Owner != nil {
		id, err := resolveUser(c, *args.Owner)
		if err != nil {
			return nil, err
		}
		q.Push(" AND c.owner_id = ").Bind(id)
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		pattern := "%" + escapeLike(query) + "%"
		q.Push(" AND (c.name ILIKE ").Bind(pattern)
		q.Push(" OR EXISTS (SELECT 1 FROM unnest(c.domains) d WHERE d ILIKE ").Bind(pattern).Push("))")
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY lower(c.name), c.id LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getCustomer(c *app.Ctx, args IDArgs) (any, error) {
	id, err := customerID(c, args.ID)
	if err != nil {
		return nil, err
	}
	value, err := customerValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	q := app.NewSQL("SELECT coalesce(jsonb_agg(" + needJSON + " ORDER BY n.created_at DESC), '[]') FROM customer_needs n " +
		"WHERE n.archived_at IS NULL AND n.customer_id = ").Bind(id)
	if err := pushNeedVisible(q, c); err != nil {
		return nil, err
	}
	if value["needs"], err = app.One[any](c, c.DB(), q.String(), q.Args...); err != nil {
		return nil, err
	}
	return value, nil
}

type SaveCustomerArgs struct {
	ID          *string          `json:"id" doc:"Only to update a customer: id, slug, name or domain. Omit to create one."`
	Name        *string          `json:"name" doc:"Required to create."`
	Domains     []string         `json:"domains" doc:"Replaces the domains, e.g. [\"acme.com\"]. Each domain belongs to one customer."`
	ExternalIDs []string         `json:"externalIds" doc:"Replaces the ids of the customer in other systems (CRM, billing)."`
	LogoURL     Nullable[string] `json:"logoUrl" doc:"Logo image URL; null clears it."`
	Owner       Nullable[string] `json:"owner" doc:"Person responsible for the customer (id, name, email or \"me\"); null clears it."`
	Status      Nullable[string] `json:"status" doc:"Customer status name or id; null clears it."`
	Tier        Nullable[string] `json:"tier" doc:"Customer tier name or id; null clears it."`
	Revenue     Nullable[int64]  `json:"revenue" doc:"Annual revenue, a whole number; null clears it."`
	Size        Nullable[int32]  `json:"size" doc:"Number of employees; null clears it."`
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
		return "", app.BadRequest("%q isn't a domain like acme.com", value)
	}
	return host, nil
}

func customerDomains(c *app.Ctx, tx app.Querier, customer *uuid.UUID, values []string) ([]string, error) {
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
	type taken struct{ Name, Domain string }
	row, found, err := app.Row[taken](c, tx,
		`SELECT c.name, d FROM customers c, unnest(c.domains) d
		 WHERE c.workspace_id = $1 AND c.id IS DISTINCT FROM $2 AND d = ANY($3) LIMIT 1`,
		c.Workspace, customer, domains)
	if err != nil {
		return nil, err
	}
	if found {
		return nil, app.Conflict("%s already belongs to the customer %s", row.Domain, row.Name)
	}
	return domains, nil
}

// resolveNullable resolves a nullable reference: left out stays unset, null stays null.
func resolveNullable(value Nullable[string], resolve func(string) (uuid.UUID, error)) (Nullable[uuid.UUID], error) {
	if !value.Valid {
		return Nullable[uuid.UUID]{Set: value.Set}, nil
	}
	id, err := resolve(value.Value)
	if err != nil {
		return Nullable[uuid.UUID]{}, err
	}
	return some(id), nil
}

func saveCustomer(c *app.Ctx, args SaveCustomerArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := customerID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	owner, err := resolveNullable(args.Owner, func(v string) (uuid.UUID, error) { return resolveUser(c, v) })
	if err != nil {
		return nil, err
	}
	status, err := resolveNullable(args.Status, func(v string) (uuid.UUID, error) { return optionID(c, customerStatuses, v) })
	if err != nil {
		return nil, err
	}
	tier, err := resolveNullable(args.Tier, func(v string) (uuid.UUID, error) { return optionID(c, customerTiers, v) })
	if err != nil {
		return nil, err
	}
	var name *string
	if args.Name != nil {
		n, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return nil, err
		}
		name = &n
	}
	var externalIDs []string
	if args.ExternalIDs != nil {
		externalIDs = []string{}
		for _, id := range args.ExternalIDs {
			id = strings.TrimSpace(id)
			if id != "" && !slices.Contains(externalIDs, id) {
				externalIDs = append(externalIDs, id)
			}
		}
	}
	logoURL := trimNullable(args.LogoURL)
	if args.Revenue.Valid && args.Revenue.Value < 0 {
		return nil, app.BadRequest("revenue can't be negative")
	}
	if args.Size.Valid && args.Size.Value < 0 {
		return nil, app.BadRequest("size can't be negative")
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var domains []string
		if args.Domains != nil {
			var err error
			if domains, err = customerDomains(c, tx, existing, args.Domains); err != nil {
				return err
			}
		}
		var id uuid.UUID
		var before M
		if existing != nil {
			id = *existing
			var err error
			if before, err = customerValue(c, tx, id); err != nil {
				return err
			}
		} else {
			if name == nil {
				return app.BadRequest("name is required to create a customer")
			}
			var err error
			id, err = app.One[uuid.UUID](c, tx,
				"INSERT INTO customers (workspace_id, slug_id, name) VALUES ($1, $2, $3) RETURNING id", c.Workspace, slugID(), *name)
			if err != nil {
				return err
			}
		}
		err := app.Exec(c, tx,
			`UPDATE customers SET
				name = coalesce($2, name),
				domains = coalesce($3, domains),
				external_ids = coalesce($4, external_ids),
				logo_url = CASE WHEN $5 THEN $6 ELSE logo_url END,
				owner_id = CASE WHEN $7 THEN $8 ELSE owner_id END,
				status_id = CASE WHEN $9 THEN $10 ELSE status_id END,
				tier_id = CASE WHEN $11 THEN $12 ELSE tier_id END,
				revenue = CASE WHEN $13 THEN $14 ELSE revenue END,
				size = CASE WHEN $15 THEN $16 ELSE size END,
				updated_at = now()
			 WHERE id = $1`,
			id, name, domains, externalIDs, logoURL.Set, logoURL.Ptr(), owner.Set, owner.Ptr(), status.Set, status.Ptr(),
			tier.Set, tier.Ptr(), args.Revenue.Set, args.Revenue.Ptr(), args.Size.Set, args.Size.Ptr())
		if err != nil {
			return err
		}
		if value, err = customerValue(c, tx, id); err != nil {
			return err
		}
		return emitCreatedOrChanged(c, tx, "Customer", before, value, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deleteCustomer(c *app.Ctx, args IDArgs) (any, error) {
	id, err := customerID(c, args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx,
			"UPDATE customers SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL", id); err != nil {
			return err
		}
		var err error
		if value, err = customerValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Customer", "remove", value, nil, nil)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

/* Statuses and tiers */

// customerOptions says which table: customer statuses and tiers have the same shape.
type customerOptions struct {
	table  string
	what   string
	column string
}

var (
	customerStatuses = customerOptions{table: "customer_statuses", what: "customer status", column: "status_id"}
	customerTiers    = customerOptions{table: "customer_tiers", what: "customer tier", column: "tier_id"}
)

func optionJSON(kind customerOptions) string {
	return fmt.Sprintf(`jsonb_build_object('id', o.id, 'name', o.name, 'color', o.color, 'description', o.description,
		'position', o.position,
		'customerCount', (SELECT count(*) FROM customers c WHERE c.%s = o.id AND c.archived_at IS NULL))`, kind.column)
}

func optionID(c *app.Ctx, kind customerOptions, value string) (uuid.UUID, error) {
	rows, err := app.Rows[named](c, c.DB(),
		fmt.Sprintf("SELECT id, name FROM %s WHERE workspace_id = $1 ORDER BY position", kind.table), c.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, ok := asUUID(value); ok {
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
	return uuid.Nil, app.BadRequest("No %s %q. %s", kind.what, value, list)
}

func optionValue(c *app.Ctx, q app.Querier, kind customerOptions, id uuid.UUID) (any, error) {
	return app.One[any](c, q, fmt.Sprintf("SELECT %s FROM %s o WHERE o.id = $1", optionJSON(kind), kind.table), id)
}

type ListOptionsArgs struct {
	Page
}

func listOptions(c *app.Ctx, kind customerOptions, args ListOptionsArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(), fmt.Sprintf(
		"SELECT %s FROM %s o WHERE o.workspace_id = $1 ORDER BY o.position, o.name LIMIT $2 OFFSET $3",
		optionJSON(kind), kind.table), c.Workspace, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type SaveOptionArgs struct {
	ID          *string          `json:"id" doc:"Only to update one: its id or name. Omit to create one."`
	Name        *string          `json:"name" doc:"Required to create."`
	Color       *string          `json:"color" doc:"Hex color like #4cb782."`
	Description Nullable[string] `json:"description" doc:"null clears it."`
	Position    *float64         `json:"position" doc:"Order among the others, lowest first. New ones go last."`
}

func saveOption(c *app.Ctx, kind customerOptions, args SaveOptionArgs) (any, error) {
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := optionID(c, kind, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	var name, color *string
	if args.Name != nil {
		n, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return nil, err
		}
		name = &n
	}
	if args.Color != nil {
		checked, err := checkColor(*args.Color)
		if err != nil {
			return nil, err
		}
		color = &checked
	}
	var value any
	err := inTx(c, func(tx pgx.Tx) error {
		if name != nil {
			taken, err := app.One[bool](c, tx, fmt.Sprintf(
				"SELECT EXISTS (SELECT 1 FROM %s WHERE workspace_id = $1 AND lower(name) = lower($2) AND id IS DISTINCT FROM $3)",
				kind.table), c.Workspace, *name, existing)
			if err != nil {
				return err
			}
			if taken {
				return app.Conflict("There already is a %s named %s", kind.what, *name)
			}
		}
		var id uuid.UUID
		if existing != nil {
			id = *existing
		} else {
			if name == nil {
				return app.BadRequest("name is required to create one")
			}
			newColor := pickColor(*name)
			if color != nil {
				newColor = *color
			}
			var err error
			id, err = app.One[uuid.UUID](c, tx, fmt.Sprintf(
				`INSERT INTO %[1]s (workspace_id, name, color, position)
				 VALUES ($1, $2, $3, (SELECT coalesce(max(position), -1) + 1 FROM %[1]s WHERE workspace_id = $1))
				 RETURNING id`, kind.table), c.Workspace, *name, newColor)
			if err != nil {
				return err
			}
		}
		description := trimNullable(args.Description)
		err := app.Exec(c, tx, fmt.Sprintf(
			`UPDATE %s SET name = coalesce($2, name), color = coalesce($3, color),
				description = CASE WHEN $4 THEN $5 ELSE description END, position = coalesce($6, position)
			 WHERE id = $1`, kind.table), id, name, color, description.Set, description.Ptr(), args.Position)
		if err != nil {
			return err
		}
		value, err = optionValue(c, tx, kind, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deleteOption(c *app.Ctx, kind customerOptions, args IDArgs) (any, error) {
	id, err := optionID(c, kind, args.ID)
	if err != nil {
		return nil, err
	}
	var cleared int64
	err = inTx(c, func(tx pgx.Tx) error {
		var err error
		if cleared, err = app.One[int64](c, tx, fmt.Sprintf("SELECT count(*) FROM customers WHERE %s = $1", kind.column), id); err != nil {
			return err
		}
		return app.Exec(c, tx, fmt.Sprintf("DELETE FROM %s WHERE id = $1", kind.table), id)
	})
	if err != nil {
		return nil, err
	}
	return M{"id": id, "deleted": true, "customersCleared": cleared}, nil
}

/* Needs */

type ListNeedsArgs struct {
	Customer *string `json:"customer" doc:"Customer id, slug, name or domain."`
	Issue    *string `json:"issue" doc:"Issue identifier (ENG-123) or id."`
	Project  *string `json:"project" doc:"Project name, slug or id."`
	Page
}

func listNeeds(c *app.Ctx, args ListNeedsArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	q := app.NewSQL("SELECT " + needJSON + " FROM customer_needs n WHERE n.archived_at IS NULL AND n.workspace_id = ").Bind(c.Workspace)
	if err := pushNeedVisible(q, c); err != nil {
		return nil, err
	}
	if args.Customer != nil {
		id, err := customerID(c, *args.Customer)
		if err != nil {
			return nil, err
		}
		q.Push(" AND n.customer_id = ").Bind(id)
	}
	if args.Issue != nil {
		id, err := resolveIssue(c, *args.Issue)
		if err != nil {
			return nil, err
		}
		q.Push(" AND n.issue_id = ").Bind(id)
	}
	if args.Project != nil {
		id, err := resolveProject(c, *args.Project)
		if err != nil {
			return nil, err
		}
		q.Push(" AND n.project_id = ").Bind(id)
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY n.created_at DESC, n.id LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type SaveNeedArgs struct {
	ID            *string          `json:"id" doc:"Only to update a need: its id. Omit to create one."`
	Customer      *string          `json:"customer" doc:"Customer id, slug, name or domain. Required to create."`
	Issue         *string          `json:"issue" doc:"The issue (ENG-123 or id) the customer needs. Pass this or project."`
	Project       *string          `json:"project" doc:"The project (name, slug or id) the customer needs. Pass this or issue."`
	Body          *string          `json:"body" doc:"What the customer said, in Markdown."`
	Priority      *int32           `json:"priority" doc:"0 normal, 1 important."`
	AttachmentURL Nullable[string] `json:"attachmentUrl" doc:"Link to where the request came from (an email, a support ticket); null clears it."`
}

func needID(c *app.Ctx, value string) (uuid.UUID, error) {
	if err := c.RequireMember(); err != nil {
		return uuid.Nil, err
	}
	id, err := resolveByID(c, "customer_needs", "customer need", value)
	if err != nil {
		return uuid.Nil, err
	}
	q := app.NewSQL("SELECT EXISTS (SELECT 1 FROM customer_needs n WHERE n.id = ").Bind(id)
	if err := pushNeedVisible(q, c); err != nil {
		return uuid.Nil, err
	}
	q.Push(")")
	visible, err := app.One[bool](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, app.NotFound("No customer need %q", strings.TrimSpace(value))
	}
	return id, nil
}

func needTeam(c *app.Ctx, q app.Querier, need uuid.UUID) (*uuid.UUID, error) {
	team, found, err := app.Maybe[uuid.UUID](c, q,
		"SELECT i.team_id FROM customer_needs n JOIN issues i ON i.id = n.issue_id WHERE n.id = $1", need)
	if err != nil || !found {
		return nil, err
	}
	return &team, nil
}

func saveNeed(c *app.Ctx, args SaveNeedArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := needID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	if args.Issue != nil && args.Project != nil {
		return nil, app.BadRequest("A need is for an issue or a project, not both")
	}
	if existing == nil && args.Issue == nil && args.Project == nil {
		return nil, app.BadRequest("Pass the issue or the project the customer needs")
	}
	var customer, issue, project *uuid.UUID
	switch {
	case args.Customer != nil:
		id, err := customerID(c, *args.Customer)
		if err != nil {
			return nil, err
		}
		customer = &id
	case existing == nil:
		return nil, app.BadRequest("customer is required to create a need")
	}
	if args.Issue != nil {
		id, err := resolveIssue(c, *args.Issue)
		if err != nil {
			return nil, err
		}
		issue = &id
	}
	if args.Project != nil {
		id, err := resolveProject(c, *args.Project)
		if err != nil {
			return nil, err
		}
		project = &id
	}
	if args.Priority != nil && (*args.Priority < 0 || *args.Priority > 1) {
		return nil, app.BadRequest("priority is 0 (normal) or 1 (important)")
	}
	attachmentURL := trimNullable(args.AttachmentURL)
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var before M
		var err error
		if existing != nil {
			id = *existing
			if before, err = needValue(c, tx, id); err != nil {
				return err
			}
		} else if id, err = app.One[uuid.UUID](c, tx,
			"INSERT INTO customer_needs (workspace_id, creator_id) VALUES ($1, $2) RETURNING id", c.Workspace, c.User); err != nil {
			return err
		}
		err = app.Exec(c, tx,
			`UPDATE customer_needs SET
				customer_id = coalesce($2, customer_id),
				issue_id = CASE WHEN $3 THEN $4 ELSE issue_id END,
				project_id = CASE WHEN $3 THEN $5 ELSE project_id END,
				body = coalesce($6, body),
				priority = coalesce($7, priority),
				attachment_url = CASE WHEN $8 THEN $9 ELSE attachment_url END,
				updated_at = now()
			 WHERE id = $1`,
			id, customer, issue != nil || project != nil, issue, project, args.Body, args.Priority,
			attachmentURL.Set, attachmentURL.Ptr())
		if err != nil {
			return err
		}
		if value, err = needValue(c, tx, id); err != nil {
			return err
		}
		team, err := needTeam(c, tx, id)
		if err != nil {
			return err
		}
		return emitCreatedOrChanged(c, tx, "CustomerNeed", before, value, team)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deleteNeed(c *app.Ctx, args IDArgs) (any, error) {
	id, err := needID(c, args.ID)
	if err != nil {
		return nil, err
	}
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := needValue(c, tx, id)
		if err != nil {
			return err
		}
		team, err := needTeam(c, tx, id)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM customer_needs WHERE id = $1", id); err != nil {
			return err
		}
		return emit(c, tx, "CustomerNeed", "remove", value, nil, team)
	})
	if err != nil {
		return nil, err
	}
	return M{"id": id, "deleted": true}, nil
}

/* Shapes */

type Customer struct {
	ID          string             `json:"id"`
	SlugID      string             `json:"slugId"`
	Name        string             `json:"name"`
	Domains     []string           `json:"domains"`
	ExternalIDs []string           `json:"externalIds" doc:"Its ids in other systems (CRM, billing)."`
	LogoURL     *string            `json:"logoUrl"`
	Owner       *UserRef           `json:"owner"`
	Status      *CustomerStatusRef `json:"status"`
	Tier        *CustomerTierRef   `json:"tier"`
	Revenue     *int               `json:"revenue" doc:"Annual revenue."`
	Size        *int               `json:"size" doc:"Number of employees."`
	NeedCount   int                `json:"needCount"`
	ArchivedAt  *string            `json:"archivedAt"`
	CreatedAt   string             `json:"createdAt"`
	UpdatedAt   string             `json:"updatedAt"`
}

type CustomerDetail struct {
	Customer
	Needs []CustomerNeed `json:"needs" doc:"Newest first, on issues and projects the caller can see."`
}

type CustomerNeed struct {
	ID            string       `json:"id"`
	Customer      *CustomerRef `json:"customer"`
	Issue         *IssueRef    `json:"issue"`
	Project       *ProjectRef  `json:"project"`
	Body          *string      `json:"body" doc:"What the customer said, in Markdown."`
	Priority      int          `json:"priority" doc:"0 normal, 1 important."`
	AttachmentURL *string      `json:"attachmentUrl" doc:"Where the request came from."`
	Creator       *UserRef     `json:"creator"`
	CreatedAt     string       `json:"createdAt"`
	UpdatedAt     string       `json:"updatedAt"`
}

type DeletedCustomerNeed struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type CustomerStatus struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Color         string  `json:"color"`
	Description   *string `json:"description"`
	Position      float64 `json:"position"`
	CustomerCount int     `json:"customerCount" doc:"Customers with it, archived ones left out."`
}

type CustomerTier struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Color         string  `json:"color"`
	Description   *string `json:"description"`
	Position      float64 `json:"position"`
	CustomerCount int     `json:"customerCount" doc:"Customers in it, archived ones left out."`
}

// DeletedCustomerOption is what deleting a customer status or tier answers.
type DeletedCustomerOption struct {
	ID               string `json:"id"`
	Deleted          bool   `json:"deleted"`
	CustomersCleared int    `json:"customersCleared" doc:"Customers left without it."`
}
