package customers_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	customers := srv.CustomerService
	oapi.Register(reg, oapi.Operation{
		Name: "list_customers", Access: auth.Read, Routes: []string{"GET /customers"},
		Description: "List customers by name: domains, external ids, owner, status, tier, revenue, size and how many needs " +
			"(requests) they have. Narrow by status, tier, owner or a query matching the name or a domain. Archived " +
			"customers are left out unless includeArchived is set.",
	}, customers.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_customer", Access: auth.Read, Routes: []string{"GET /customers/{id}"},
		Description: "Read a customer by id, slug, name, domain or external id, with their needs: what they asked for, linked to " +
			"an issue or a project.",
	}, customers.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_customer", Access: auth.Write, Routes: []string{"POST /customers", "PATCH /customers/{id}"},
		Description: "Create a customer (omit id; name is required) or update one (pass id: id, slug, name or domain). Omitted " +
			"fields stay as they are; null clears a field. Domains are unique within the workspace. Status and tier " +
			"take a name or id (see list_customer_statuses and list_customer_tiers); owner a person (id, name, email " +
			"or \"me\").",
	}, customers.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_customer", Access: auth.Write, Routes: []string{"DELETE /customers/{id}"},
		Description: "Archive a customer. Their needs stay on the issues and projects they are linked to.",
	}, customers.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "list_customer_statuses", Access: auth.Read, Routes: []string{"GET /customer-statuses"},
		Description: "The customer statuses of the workspace (e.g. Active, Prospect, Churned) in order.",
	}, customers.ListStatuses)
	oapi.Register(reg, oapi.Operation{
		Name: "save_customer_status", Access: auth.AdminAccess, Routes: []string{"POST /customer-statuses", "PATCH /customer-statuses/{id}"},
		Description: "Create a customer status (omit id; name is required) or update one (pass its id or name). Admins only.",
	}, customers.SaveStatus)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_customer_status", Access: auth.AdminAccess, Routes: []string{"DELETE /customer-statuses/{id}"},
		Description: "Delete a customer status (id or name). Customers with it are left without a status. Admins only.",
	}, customers.DeleteStatus)
	oapi.Register(reg, oapi.Operation{
		Name: "list_customer_tiers", Access: auth.Read, Routes: []string{"GET /customer-tiers"},
		Description: "The customer tiers of the workspace (e.g. Enterprise, Growth) in order.",
	}, customers.ListTiers)
	oapi.Register(reg, oapi.Operation{
		Name: "save_customer_tier", Access: auth.AdminAccess, Routes: []string{"POST /customer-tiers", "PATCH /customer-tiers/{id}"},
		Description: "Create a customer tier (omit id; name is required) or update one (pass its id or name). Admins only.",
	}, customers.SaveTier)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_customer_tier", Access: auth.AdminAccess, Routes: []string{"DELETE /customer-tiers/{id}"},
		Description: "Delete a customer tier (id or name). Customers in it are left without a tier. Admins only.",
	}, customers.DeleteTier)
	oapi.Register(reg, oapi.Operation{
		Name: "list_customer_needs", Access: auth.Read, Routes: []string{"GET /customer-needs"},
		Description: "List customer needs (requests), newest first, for a customer, an issue or a project: the customer, the " +
			"issue or project, the request text, priority (0 normal, 1 important) and a link to the source.",
	}, customers.ListNeeds)
	oapi.Register(reg, oapi.Operation{
		Name: "save_customer_need", Access: auth.Write, Routes: []string{"POST /customer-needs", "PATCH /customer-needs/{id}"},
		Description: "Record what a customer asked for (omit id; customer and exactly one of issue or project are required) or " +
			"change a need (pass id). Issues take an identifier like ENG-123; projects a name, slug or id.",
	}, customers.SaveNeed)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_customer_need", Access: auth.Write, Routes: []string{"DELETE /customer-needs/{id}"},
		Description: "Delete a customer need.",
	}, customers.DeleteNeed)
}
