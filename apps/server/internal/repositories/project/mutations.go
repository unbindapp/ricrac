package project_repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/entitylink"
	"ricrac/ent/initiativeproject"
	"ricrac/ent/project"
	"ricrac/ent/projectlabellink"
	"ricrac/ent/projectmember"
	"ricrac/ent/projectrelation"
	"ricrac/ent/projectteam"
	repository "ricrac/internal/repositories"
)

type CreateInput struct {
	Workspace uuid.UUID
	SlugID    string
	Name      string
	Status    uuid.UUID
	Creator   uuid.UUID
}

// Create adds a project after the workspace's others in manual order.
func (self *ProjectRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	client := self.base.Client(tx)
	var last float64
	err := scan(ctx, client, func(rows *sql.Rows) error { return rows.Scan(&last) },
		"SELECT coalesce(max(sort_order), 0) FROM projects WHERE workspace_id = $1", input.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	created, err := client.Project.Create().SetWorkspaceID(input.Workspace).SetSlugID(input.SlugID).SetName(input.Name).
		SetStatusID(input.Status).SetCreatorID(input.Creator).SetSortOrder(last + 1000).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// Update is the project's update builder, to set fields on.
func (self *ProjectRepository) Update(tx repository.TxInterface, id uuid.UUID) *ent.ProjectUpdateOne {
	return self.base.Client(tx).Project.UpdateOneID(id)
}

type Stamps struct {
	StartedAt   *time.Time
	CompletedAt *time.Time
	CanceledAt  *time.Time
}

func (self *ProjectRepository) SetStamps(ctx context.Context, tx repository.TxInterface, id uuid.UUID, stamps Stamps) error {
	return self.base.Client(tx).Project.UpdateOneID(id).
		SetNillableStartedAt(stamps.StartedAt).SetNillableCompletedAt(stamps.CompletedAt).SetNillableCanceledAt(stamps.CanceledAt).
		Modify(func(u *entsql.UpdateBuilder) {
			if stamps.StartedAt == nil {
				u.SetNull(project.FieldStartedAt)
			}
			if stamps.CompletedAt == nil {
				u.SetNull(project.FieldCompletedAt)
			}
			if stamps.CanceledAt == nil {
				u.SetNull(project.FieldCanceledAt)
			}
		}).Exec(ctx)
}

func (self *ProjectRepository) Subscribe(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	creates := make([]*ent.ProjectSubscriberCreate, len(users))
	for i, u := range users {
		creates[i] = self.base.Client(tx).ProjectSubscriber.Create().SetProjectID(id).SetUserID(u)
	}
	return self.base.Client(tx).ProjectSubscriber.CreateBulk(creates...).OnConflict().DoNothing().Exec(ctx)
}

func (self *ProjectRepository) AddMembers(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	creates := make([]*ent.ProjectMemberCreate, len(users))
	for i, u := range users {
		creates[i] = self.base.Client(tx).ProjectMember.Create().SetProjectID(id).SetUserID(u)
	}
	return self.base.Client(tx).ProjectMember.CreateBulk(creates...).OnConflict().DoNothing().Exec(ctx)
}

func (self *ProjectRepository) RemoveMembers(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	_, err := self.base.Client(tx).ProjectMember.Delete().Where(projectmember.ProjectID(id), projectmember.UserIDIn(users...)).Exec(ctx)
	return err
}

func (self *ProjectRepository) AddTeams(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) error {
	if len(teams) == 0 {
		return nil
	}
	creates := make([]*ent.ProjectTeamCreate, len(teams))
	for i, t := range teams {
		creates[i] = self.base.Client(tx).ProjectTeam.Create().SetProjectID(id).SetTeamID(t)
	}
	return self.base.Client(tx).ProjectTeam.CreateBulk(creates...).OnConflict().DoNothing().Exec(ctx)
}

func (self *ProjectRepository) RemoveTeams(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) error {
	_, err := self.base.Client(tx).ProjectTeam.Delete().Where(projectteam.ProjectID(id), projectteam.TeamIDIn(teams...)).Exec(ctx)
	return err
}

// KeepTeams removes the project's teams that aren't in teams.
func (self *ProjectRepository) KeepTeams(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) error {
	_, err := self.base.Client(tx).ProjectTeam.Delete().Where(projectteam.ProjectID(id), projectteam.TeamIDNotIn(teams...)).Exec(ctx)
	return err
}

func (self *ProjectRepository) TeamCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).ProjectTeam.Query().Where(projectteam.ProjectID(id)).Count(ctx)
}

func (self *ProjectRepository) HasTeam(ctx context.Context, tx repository.TxInterface, id, teamID uuid.UUID) (bool, error) {
	return self.base.Client(tx).ProjectTeam.Query().Where(projectteam.ProjectID(id), projectteam.TeamID(teamID)).Exist(ctx)
}

// DropLeadTeamIfGone clears the lead team when it's no longer one of the project's teams.
func (self *ProjectRepository) DropLeadTeamIfGone(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).Project.Update().
		Where(project.ID(id), project.LeadTeamIDNotNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw(s.C(project.FieldLeadTeamID)+" NOT IN (SELECT team_id FROM project_teams WHERE project_id = ?)", id))
		}).
		ClearLeadTeamID().Save(ctx)
	return err
}

func (self *ProjectRepository) AddLabels(ctx context.Context, tx repository.TxInterface, id uuid.UUID, labels []uuid.UUID) error {
	if len(labels) == 0 {
		return nil
	}
	creates := make([]*ent.ProjectLabelLinkCreate, len(labels))
	for i, l := range labels {
		creates[i] = self.base.Client(tx).ProjectLabelLink.Create().SetProjectID(id).SetLabelID(l)
	}
	return self.base.Client(tx).ProjectLabelLink.CreateBulk(creates...).OnConflict().DoNothing().Exec(ctx)
}

func (self *ProjectRepository) RemoveLabels(ctx context.Context, tx repository.TxInterface, id uuid.UUID, labels []uuid.UUID) error {
	_, err := self.base.Client(tx).ProjectLabelLink.Delete().Where(projectlabellink.ProjectID(id), projectlabellink.LabelIDIn(labels...)).Exec(ctx)
	return err
}

// KeepLabels removes the project's labels that aren't in labels.
func (self *ProjectRepository) KeepLabels(ctx context.Context, tx repository.TxInterface, id uuid.UUID, labels []uuid.UUID) error {
	_, err := self.base.Client(tx).ProjectLabelLink.Delete().Where(projectlabellink.ProjectID(id), projectlabellink.LabelIDNotIn(labels...)).Exec(ctx)
	return err
}

// LinkInitiatives adds the project to each initiative, after the initiative's other projects.
func (self *ProjectRepository) LinkInitiatives(ctx context.Context, tx repository.TxInterface, id uuid.UUID, initiatives []uuid.UUID) error {
	client := self.base.Client(tx)
	for _, initiative := range initiatives {
		var last float64
		err := scan(ctx, client, func(rows *sql.Rows) error { return rows.Scan(&last) },
			"SELECT coalesce(max(sort_order), 0) FROM initiative_projects WHERE initiative_id = $1", initiative)
		if err != nil {
			return err
		}
		err = client.InitiativeProject.Create().SetInitiativeID(initiative).SetProjectID(id).SetSortOrder(last + 1).
			OnConflict().DoNothing().Exec(ctx)
		if err != nil && !isNoRows(err) {
			return err
		}
	}
	return nil
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// UnlinkInitiatives takes the project out of the initiatives.
func (self *ProjectRepository) UnlinkInitiatives(ctx context.Context, tx repository.TxInterface, id uuid.UUID, initiatives []uuid.UUID) error {
	_, err := self.base.Client(tx).InitiativeProject.Delete().
		Where(initiativeproject.ProjectID(id), initiativeproject.InitiativeIDIn(initiatives...)).Exec(ctx)
	return err
}

// KeepInitiatives takes the project out of the initiatives not in initiatives.
func (self *ProjectRepository) KeepInitiatives(ctx context.Context, tx repository.TxInterface, id uuid.UUID, initiatives []uuid.UUID) error {
	_, err := self.base.Client(tx).InitiativeProject.Delete().
		Where(initiativeproject.ProjectID(id), initiativeproject.InitiativeIDNotIn(initiatives...)).Exec(ctx)
	return err
}

// Relate adds a relation from one project to another unless there is one (either way, for
// related).
func (self *ProjectRepository) Relate(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, kind string, creator uuid.UUID) error {
	exists, err := self.base.Client(tx).ProjectRelation.Query().Where(relationBetween(from, to, kind)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	return self.base.Client(tx).ProjectRelation.Create().SetProjectID(from).SetRelatedProjectID(to).SetType(kind).SetCreatorID(creator).Exec(ctx)
}

// Unrelate removes the relation from one project to another (either way, for related).
func (self *ProjectRepository) Unrelate(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, kind string) error {
	_, err := self.base.Client(tx).ProjectRelation.Delete().Where(relationBetween(from, to, kind)).Exec(ctx)
	return err
}

func relationBetween(from, to uuid.UUID, kind string) func(*entsql.Selector) {
	forward := projectrelation.And(projectrelation.ProjectID(from), projectrelation.RelatedProjectID(to))
	if kind != "related" {
		return projectrelation.And(projectrelation.Type(kind), forward)
	}
	backward := projectrelation.And(projectrelation.ProjectID(to), projectrelation.RelatedProjectID(from))
	return projectrelation.And(projectrelation.Type(kind), projectrelation.Or(forward, backward))
}

// Relabel sets the label of the link with this URL on the project or initiative whose id is in
// column; false when there is none.
func (self *ProjectRepository) Relabel(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID, url, label string) (bool, error) {
	n, err := self.base.Client(tx).EntityLink.Update().
		Where(func(s *entsql.Selector) { s.Where(entsql.EQ(s.C(column), id)) }, entitylink.URL(url)).
		SetLabel(label).Save(ctx)
	return n > 0, err
}

// AddLink adds a link to the project or initiative whose id is in column, after its others.
func (self *ProjectRepository) AddLink(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, column string, id uuid.UUID, url, label string, creator uuid.UUID) error {
	client := self.base.Client(tx)
	var last float64
	err := scan(ctx, client, func(rows *sql.Rows) error { return rows.Scan(&last) },
		"SELECT coalesce(max(sort_order), 0) FROM entity_links WHERE "+column+" = $1", id)
	if err != nil {
		return err
	}
	create := client.EntityLink.Create().SetWorkspaceID(workspace).SetURL(url).SetLabel(label).SetCreatorID(creator).SetSortOrder(last + 1)
	if column == entitylink.FieldInitiativeID {
		create.SetInitiativeID(id)
	} else {
		create.SetProjectID(id)
	}
	return create.Exec(ctx)
}

// PurgeTrash deletes the projects trashed more than 30 days ago.
func (self *ProjectRepository) PurgeTrash(ctx context.Context) (int, error) {
	return self.base.DB.Project.Delete().Where(func(s *entsql.Selector) {
		s.Where(repository.Raw(s.C(project.FieldTrashedAt) + " < now() - interval '30 days'"))
	}).Exec(ctx)
}

// RemoveLinks removes the links with these URLs from the project or initiative whose id is in
// column.
func (self *ProjectRepository) RemoveLinks(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID, urls []string) error {
	_, err := self.base.Client(tx).EntityLink.Delete().
		Where(func(s *entsql.Selector) { s.Where(entsql.EQ(s.C(column), id)) }, entitylink.URLIn(urls...)).Exec(ctx)
	return err
}
