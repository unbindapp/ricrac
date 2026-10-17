package customer_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customerneed"
	"ricrac/ent/issue"
	"ricrac/ent/issueshare"
	"ricrac/ent/predicate"
	"ricrac/ent/project"
	"ricrac/ent/team"
	"ricrac/internal/common/args"
	repository "ricrac/internal/repositories"
)

// Viewer is who reads needs: the teams they see and who they are, for issues shared with them.
type Viewer struct {
	Teams []uuid.UUID
	User  uuid.UUID
}

// needVisible is a need on an issue or project the viewer can see, or on neither.
func needVisible(viewer Viewer) predicate.CustomerNeed {
	return customerneed.And(
		customerneed.Or(customerneed.IssueIDIsNil(), customerneed.HasIssueWith(issue.TrashedAtIsNil(),
			issue.Or(issue.TeamIDIn(viewer.Teams...), issue.HasIssueSharesWith(issueshare.UserID(viewer.User))))),
		customerneed.Or(customerneed.ProjectIDIsNil(), customerneed.HasProjectWith(project.TrashedAtIsNil(),
			project.HasTeamsWith(team.IDIn(viewer.Teams...)))),
	)
}

// LoadNeeds is the needs with what their answer shows (customer, issue, project, creator), in
// the order of ids.
func (self *CustomerRepository) LoadNeeds(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.CustomerNeed, error) {
	needs, err := self.base.Client(tx).CustomerNeed.Query().Where(customerneed.IDIn(ids...)).
		WithCustomer().WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() }).WithProject().WithCreator().
		All(ctx)
	if err != nil {
		return nil, err
	}
	return inOrder(ids, needs, func(n *ent.CustomerNeed) uuid.UUID { return n.ID }), nil
}

type NeedFilter struct {
	Workspace uuid.UUID
	Viewer    Viewer
	Customer  *uuid.UUID
	Issue     *uuid.UUID
	Project   *uuid.UUID
	Limit     int64
	Offset    int64
}

// Needs is the needs not archived the viewer can see, newest first.
func (self *CustomerRepository) Needs(ctx context.Context, filter NeedFilter) ([]uuid.UUID, error) {
	query := self.base.DB.CustomerNeed.Query().
		Where(customerneed.ArchivedAtIsNil(), customerneed.WorkspaceID(filter.Workspace), needVisible(filter.Viewer))
	if filter.Customer != nil {
		query = query.Where(customerneed.CustomerID(*filter.Customer))
	}
	if filter.Issue != nil {
		query = query.Where(customerneed.IssueID(*filter.Issue))
	}
	if filter.Project != nil {
		query = query.Where(customerneed.ProjectID(*filter.Project))
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(customerneed.FieldID))
		s.OrderBy(entsql.Desc(s.C(customerneed.FieldCreatedAt)), s.C(customerneed.FieldID))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// CustomerNeeds is the customer's needs not archived the viewer can see, newest first.
func (self *CustomerRepository) CustomerNeeds(ctx context.Context, customer uuid.UUID, viewer Viewer) ([]uuid.UUID, error) {
	return self.base.DB.CustomerNeed.Query().
		Where(customerneed.ArchivedAtIsNil(), customerneed.CustomerID(customer), needVisible(viewer)).
		Order(customerneed.ByCreatedAt(entsql.OrderDesc())).IDs(ctx)
}

func (self *CustomerRepository) NeedVisible(ctx context.Context, id uuid.UUID, viewer Viewer) (bool, error) {
	return self.base.DB.CustomerNeed.Query().Where(customerneed.ID(id), needVisible(viewer)).Exist(ctx)
}

func (self *CustomerRepository) CreateNeed(ctx context.Context, tx repository.TxInterface, workspace, creator uuid.UUID) (uuid.UUID, error) {
	created, err := self.base.Client(tx).CustomerNeed.Create().SetWorkspaceID(workspace).SetCreatorID(creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// NeedUpdate leaves nil fields and fields not Set as they are. With Target, the need is for
// Issue or Project (the other one is cleared).
type NeedUpdate struct {
	Customer      *uuid.UUID
	Target        bool
	Issue         *uuid.UUID
	Project       *uuid.UUID
	Body          *string
	Priority      *int
	AttachmentURL args.Nullable[string]
}

func (self *CustomerRepository) UpdateNeed(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input NeedUpdate, now time.Time) error {
	update := self.base.Client(tx).CustomerNeed.UpdateOneID(id).
		SetNillableCustomerID(input.Customer).SetNillableBody(input.Body).SetNillablePriority(input.Priority).SetUpdatedAt(now)
	if input.Target {
		setOrClear(nullable(input.Issue), update.SetIssueID, update.ClearIssueID)
		setOrClear(nullable(input.Project), update.SetProjectID, update.ClearProjectID)
	}
	setOrClear(input.AttachmentURL, update.SetAttachmentURL, update.ClearAttachmentURL)
	return update.Exec(ctx)
}

func nullable[T any](value *T) args.Nullable[T] {
	if value == nil {
		return args.Nullable[T]{Set: true}
	}
	return args.Nullable[T]{Set: true, Valid: true, Value: *value}
}

// NeedTeam is the team of the need's issue.
func (self *CustomerRepository) NeedTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*uuid.UUID, error) {
	need, err := self.base.Client(tx).CustomerNeed.Query().Where(customerneed.ID(id)).WithIssue().Only(ctx)
	if err != nil || need.Edges.Issue == nil {
		return nil, err
	}
	return &need.Edges.Issue.TeamID, nil
}

func (self *CustomerRepository) DeleteNeed(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).CustomerNeed.DeleteOneID(id).Exec(ctx)
}
