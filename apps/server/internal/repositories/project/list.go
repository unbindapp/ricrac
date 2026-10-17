package project_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customview"
	"ricrac/ent/initiativeproject"
	"ricrac/ent/project"
	"ricrac/ent/projectlabellink"
	"ricrac/ent/projectmember"
	"ricrac/ent/projectstatus"
	"ricrac/ent/projectteam"
	"ricrac/ent/template"
	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
)

type ListFilter struct {
	Workspace       uuid.UUID
	Teams           []uuid.UUID
	IncludeArchived bool
	IncludeTrashed  bool
	// In this team (among others).
	Team        *uuid.UUID
	StatusType  *string
	Status      *uuid.UUID
	WithoutLead bool
	Lead        *uuid.UUID
	Member      *uuid.UUID
	Initiative  *uuid.UUID
	// With one of these labels.
	Labels   []uuid.UUID
	Health   *string
	Priority *int
	// Words in the name, summary or description.
	Query      *string
	Conditions []Condition
	// updatedAt, createdAt, targetDate, priority or manual.
	OrderBy string
	Limit   int64
	Offset  int64
}

var listOrders = map[string]string{
	"updatedAt":  "projects.updated_at DESC",
	"createdAt":  "projects.created_at DESC",
	"targetDate": "projects.target_date ASC NULLS LAST, projects.updated_at DESC",
	"priority":   "CASE WHEN projects.priority = 0 THEN 5 ELSE projects.priority END, projects.updated_at DESC",
	"manual":     "projects.sort_order ASC, projects.created_at",
}

// IsListOrder says whether projects can be listed in this order.
func IsListOrder(name string) bool {
	_, ok := listOrders[name]
	return ok
}

// List is the projects of the workspace in one of Teams that the filter keeps, in its order.
func (self *ProjectRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Project.Query().Where(project.WorkspaceID(filter.Workspace), project.HasProjectTeamsWith(projectteam.TeamIDIn(filter.Teams...)))
	if !filter.IncludeArchived {
		query.Where(project.ArchivedAtIsNil())
	}
	if !filter.IncludeTrashed {
		query.Where(project.TrashedAtIsNil())
	}
	if filter.Team != nil {
		query.Where(project.HasProjectTeamsWith(projectteam.TeamID(*filter.Team)))
	}
	if filter.StatusType != nil {
		query.Where(project.HasStatusWith(projectstatus.Type(*filter.StatusType)))
	}
	if filter.Status != nil {
		query.Where(project.StatusID(*filter.Status))
	}
	if filter.WithoutLead {
		query.Where(project.LeadIDIsNil())
	}
	if filter.Lead != nil {
		query.Where(project.LeadID(*filter.Lead))
	}
	if filter.Member != nil {
		query.Where(project.HasProjectMembersWith(projectmember.UserID(*filter.Member)))
	}
	if filter.Initiative != nil {
		query.Where(project.HasInitiativeProjectsWith(initiativeproject.InitiativeID(*filter.Initiative)))
	}
	if filter.Labels != nil {
		query.Where(project.HasProjectLabelLinksWith(projectlabellink.LabelIDIn(filter.Labels...)))
	}
	if filter.Health != nil {
		query.Where(project.Health(*filter.Health))
	}
	if filter.Priority != nil {
		query.Where(project.Priority(*filter.Priority))
	}
	return query.Modify(func(s *entsql.Selector) {
		if filter.Query != nil {
			s.Where(repository.Raw("(projects.search @@ websearch_to_tsquery('english', ?) OR projects.name ILIKE ?)",
				*filter.Query, "%"+utils.EscapeLike(*filter.Query)+"%"))
		}
		for _, condition := range filter.Conditions {
			s.Where(repository.Raw(condition.SQL, condition.Args...))
		}
		s.Select(s.C(project.FieldID))
		s.OrderExpr(entsql.Expr(listOrders[filter.OrderBy]))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// View is a custom view of the workspace.
func (self *ProjectRepository) View(ctx context.Context, id uuid.UUID) (*ent.CustomView, error) {
	return self.base.DB.CustomView.Query().Where(customview.ID(id)).Only(ctx)
}

func (self *ProjectRepository) Template(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Template, error) {
	return self.base.Client(tx).Template.Query().Where(template.ID(id)).Only(ctx)
}
