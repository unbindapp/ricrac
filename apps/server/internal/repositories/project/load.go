package project_repo

import (
	"context"
	"database/sql"
	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/document"
	"ricrac/ent/entitylink"
	"ricrac/ent/initiative"
	"ricrac/ent/predicate"
	"ricrac/ent/project"
	"ricrac/ent/projectlabel"
	"ricrac/ent/projectrelation"
	"ricrac/ent/team"
	"ricrac/ent/user"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

// WithAnswer loads what a project's answer shows (see models.ProjectOf).
func WithAnswer(q *ent.ProjectQuery) *ent.ProjectQuery {
	return q.WithWorkspace().WithStatus().WithLead().WithLeadTeam().
		WithTeams(func(q *ent.TeamQuery) { q.Order(ent.Asc(team.FieldKey)) }).
		WithLabels(func(q *ent.ProjectLabelQuery) { q.WithParent().Order(ent.Asc(projectlabel.FieldName)) }).
		WithInitiatives(func(q *ent.InitiativeQuery) {
			q.Where(initiative.TrashedAtIsNil()).Order(ent.Asc(initiative.FieldName))
		})
}

// Visible is the projects in one of teams.
func Visible(teams []uuid.UUID) predicate.Project {
	return project.HasTeamsWith(team.IDIn(teams...))
}

// Load is the projects with what their answer shows, in the order of ids.
func (self *ProjectRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Project, error) {
	projects, err := WithAnswer(self.base.Client(tx).Project.Query().Where(project.IDIn(ids...))).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Project, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}
	ordered := make([]*ent.Project, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			ordered = append(ordered, p)
		}
	}
	return ordered, nil
}

// Stats is each project's issue counts and progress, counting the issues viewer can see
// (project_issue_stats).
func (self *ProjectRepository) Stats(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, viewer uuid.UUID) (map[uuid.UUID]models.ProjectIssueStats, error) {
	return IssueStats(ctx, self.base.Client(tx), "project_issue_stats(id, NULL, $2)", ids, viewer)
}

// IssueStats runs the stats expression for each of ids ($1 is the list, id the current one, $2
// the viewer).
func IssueStats(ctx context.Context, client *ent.Client, expression string, ids []uuid.UUID, viewer uuid.UUID) (map[uuid.UUID]models.ProjectIssueStats, error) {
	stats := make(map[uuid.UUID]models.ProjectIssueStats, len(ids))
	err := scan(ctx, client, func(rows *sql.Rows) error {
		var id uuid.UUID
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		var value models.ProjectIssueStats
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		stats[id] = value
		return nil
	}, "SELECT id, "+expression+" FROM unnest($1::uuid[]) AS id", ids, viewer)
	return stats, err
}

func (self *ProjectRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Project, error) {
	return self.base.Client(tx).Project.Get(ctx, id)
}

// GetForUpdate locks the project until the transaction ends.
func (self *ProjectRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Project, error) {
	return self.base.Client(tx).Project.Query().Where(project.ID(id)).ForUpdate().Only(ctx)
}

// Creator is who created the project, nil when unknown.
func (self *ProjectRepository) Creator(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.User, error) {
	found, err := self.base.Client(tx).Project.Query().Where(project.ID(id)).WithCreator().Only(ctx)
	if err != nil {
		return nil, err
	}
	return found.Edges.Creator, nil
}

// Members is the project's members by name.
func (self *ProjectRepository) Members(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.User, error) {
	return self.base.Client(tx).Project.Query().Where(project.ID(id)).QueryMembers().Order(ent.Asc(user.FieldName)).All(ctx)
}

// Subscribers is the project's subscribers by name.
func (self *ProjectRepository) Subscribers(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.User, error) {
	return self.base.Client(tx).Project.Query().Where(project.ID(id)).QuerySubscribers().Order(ent.Asc(user.FieldName)).All(ctx)
}

// Links is the links on the project or initiative whose id is in column (project_id or
// initiative_id), in their order, with their creators.
func (self *ProjectRepository) Links(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID) ([]*ent.EntityLink, error) {
	return self.base.Client(tx).EntityLink.Query().
		Where(func(s *entsql.Selector) { s.Where(entsql.EQ(s.C(column), id)) }).
		WithCreator().Order(ent.Asc(entitylink.FieldSortOrder), ent.Asc(entitylink.FieldCreatedAt)).All(ctx)
}

// Documents is the documents not in the trash of the project or initiative whose id is in
// column, in their order.
func (self *ProjectRepository) Documents(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID) ([]*ent.Document, error) {
	return self.base.Client(tx).Document.Query().
		Where(func(s *entsql.Selector) { s.Where(entsql.EQ(s.C(column), id)) }, document.TrashedAtIsNil()).
		Order(ent.Asc(document.FieldSortOrder), ent.Asc(document.FieldCreatedAt)).All(ctx)
}

type Relations struct {
	Blocks    []*ent.Project
	BlockedBy []*ent.Project
	Related   []*ent.Project
}

// Relations is the projects not in the trash, in one of teams, that the project blocks, is
// blocked by or relates to, oldest relation first.
func (self *ProjectRepository) Relations(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) (Relations, error) {
	client := self.base.Client(tx)
	shown := []predicate.Project{project.TrashedAtIsNil(), Visible(teams)}
	var found Relations
	blocks, err := client.ProjectRelation.Query().
		Where(projectrelation.ProjectID(id), projectrelation.Type("blocks"), projectrelation.HasRelatedProjectWith(shown...)).
		WithRelatedProject().Order(ent.Asc(projectrelation.FieldCreatedAt)).All(ctx)
	if err != nil {
		return found, err
	}
	for _, r := range blocks {
		found.Blocks = append(found.Blocks, r.Edges.RelatedProject)
	}
	blockedBy, err := client.ProjectRelation.Query().
		Where(projectrelation.RelatedProjectID(id), projectrelation.Type("blocks"), projectrelation.HasProjectWith(shown...)).
		WithProject().Order(ent.Asc(projectrelation.FieldCreatedAt)).All(ctx)
	if err != nil {
		return found, err
	}
	for _, r := range blockedBy {
		found.BlockedBy = append(found.BlockedBy, r.Edges.Project)
	}
	related, err := client.ProjectRelation.Query().
		Where(projectrelation.Type("related"), projectrelation.Or(
			projectrelation.And(projectrelation.ProjectID(id), projectrelation.HasRelatedProjectWith(shown...)),
			projectrelation.And(projectrelation.RelatedProjectID(id), projectrelation.HasProjectWith(shown...)))).
		WithProject().WithRelatedProject().Order(ent.Asc(projectrelation.FieldCreatedAt)).All(ctx)
	if err != nil {
		return found, err
	}
	for _, r := range related {
		other := r.Edges.RelatedProject
		if r.ProjectID != id {
			other = r.Edges.Project
		}
		found.Related = append(found.Related, other)
	}
	return found, nil
}

// IssueCountByStateType is how many issues not in the trash, in one of teams, the project has
// of each status type.
func (self *ProjectRepository) IssueCountByStateType(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) (map[string]int, error) {
	counts := map[string]int{}
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return err
		}
		counts[kind] = n
		return nil
	}, `SELECT s.type, count(*) FROM issues i JOIN workflow_states s ON s.id = i.state_id
		WHERE i.project_id = $1 AND i.trashed_at IS NULL AND i.team_id = ANY($2) GROUP BY s.type`, id, teams)
	return counts, err
}

// Estimate is the sum of the estimates of the project's issues not in the trash nor canceled, in
// one of teams, and of the completed ones.
func (self *ProjectRepository) Estimate(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) (models.ProjectEstimate, error) {
	var estimate models.ProjectEstimate
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error {
		return rows.Scan(&estimate.Total, &estimate.Completed)
	}, `SELECT coalesce(sum(i.estimate), 0), coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0)
		FROM issues i JOIN workflow_states s ON s.id = i.state_id
		WHERE i.project_id = $1 AND i.trashed_at IS NULL AND s.type <> 'canceled' AND i.team_id = ANY($2)`, id, teams)
	return estimate, err
}

// WebhookTeam is the team a project's webhook events are filed under: its lead team, else its
// first team by key.
func (self *ProjectRepository) WebhookTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*uuid.UUID, error) {
	found, err := self.base.Client(tx).Project.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if found.LeadTeamID != nil {
		return found.LeadTeamID, nil
	}
	first, err := self.base.Client(tx).Project.QueryTeams(found).Order(ent.Asc(team.FieldKey)).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &first.ID, nil
}

// Trashed is the most recently trashed project of the workspace whose id or name is value, or
// whose slug id is slug.
func (self *ProjectRepository) Trashed(ctx context.Context, workspace uuid.UUID, value, slug string) (uuid.UUID, bool, error) {
	found, err := self.base.DB.Project.Query().
		Where(project.WorkspaceID(workspace), project.TrashedAtNotNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("("+s.C(project.FieldID)+"::text = ? OR "+s.C(project.FieldSlugID)+" = ? OR lower("+
				s.C(project.FieldName)+") = lower(?))", value, slug, value))
		}).
		Order(ent.Desc(project.FieldTrashedAt)).First(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return found.ID, true, nil
}

// IsVisible says whether the project is in one of teams.
func (self *ProjectRepository) IsVisible(ctx context.Context, id uuid.UUID, teams []uuid.UUID) (bool, error) {
	return self.base.DB.Project.Query().Where(project.ID(id), Visible(teams)).Exist(ctx)
}
