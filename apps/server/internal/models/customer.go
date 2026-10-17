package models

import (
	"ricrac/ent"
)

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

func nonNilStrings[L ~[]string](list L) []string {
	if list == nil {
		return []string{}
	}
	return []string(list)
}

func optionalInt[T int | int64](value *T) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}

// CustomerOf needs the customer's owner, status and tier loaded.
func CustomerOf(c *ent.Customer, needCount int) Customer {
	return Customer{
		ID: ID(c.ID), SlugID: c.SlugID, Name: c.Name, Domains: nonNilStrings(c.Domains), ExternalIDs: nonNilStrings(c.ExternalIds),
		LogoURL: c.LogoURL, Owner: UserRefOf(c.Edges.Owner), Status: CustomerStatusRefOf(c.Edges.Status),
		Tier: CustomerTierRefOf(c.Edges.Tier), Revenue: optionalInt(c.Revenue), Size: optionalInt(c.Size), NeedCount: needCount,
		ArchivedAt: TimePtr(c.ArchivedAt), CreatedAt: Time(c.CreatedAt), UpdatedAt: Time(c.UpdatedAt),
	}
}

// CustomerNeedOf needs the need's customer, issue (with its team and state), project and creator
// loaded.
func CustomerNeedOf(n *ent.CustomerNeed) CustomerNeed {
	return CustomerNeed{
		ID: ID(n.ID), Customer: CustomerRefOf(n.Edges.Customer), Issue: IssueRefOf(n.Edges.Issue),
		Project: ProjectRefOf(n.Edges.Project), Body: n.Body, Priority: n.Priority, AttachmentURL: n.AttachmentURL,
		Creator: UserRefOf(n.Edges.Creator), CreatedAt: Time(n.CreatedAt), UpdatedAt: Time(n.UpdatedAt),
	}
}
