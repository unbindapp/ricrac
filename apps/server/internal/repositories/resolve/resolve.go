package resolve_repo

import (
	"context"
	"fmt"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/document"
	"ricrac/ent/githubissuelink"
	"ricrac/ent/githubreposync"
	"ricrac/ent/issue"
	"ricrac/ent/issuelabel"
	"ricrac/ent/issueshare"
	"ricrac/ent/predicate"
	"ricrac/ent/project"
	"ricrac/ent/projectlabel"
	"ricrac/ent/projectmilestone"
	"ricrac/ent/projectstatus"
	"ricrac/ent/projectteam"
	"ricrac/ent/team"
	"ricrac/ent/template"
	"ricrac/ent/user"
	"ricrac/ent/workflowstate"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

// The lookups that turn what agents pass into ids: every row a name could match, for the
// resolve service to pick from.

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ResolveRepositoryInterface -p resolve_repo -s ResolveRepository -o resolve_repository_iface.go
type ResolveRepository struct {
	base *repository.BaseRepository
}

func NewResolveRepository(db *ent.Client) *ResolveRepository {
	return &ResolveRepository{base: repository.NewBaseRepository(db)}
}

type Member struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// Members is the workspace's active members.
func (self *ResolveRepository) Members(ctx context.Context, workspace uuid.UUID) ([]Member, error) {
	var rows []Member
	err := self.base.DB.User.Query().Modify(func(s *entsql.Selector) {
		members := entsql.Table(workspacemember.Table)
		s.Join(members).On(s.C(user.FieldID), members.C(workspacemember.FieldUserID)).
			Where(entsql.And(entsql.EQ(members.C(workspacemember.FieldWorkspaceID), workspace), entsql.EQ(members.C(workspacemember.FieldActive), true)))
		s.Select(s.C(user.FieldID), s.C(user.FieldName), s.C(user.FieldDisplayName), s.C(user.FieldEmail))
	}).Scan(ctx, &rows)
	return rows, err
}

type Keyed struct {
	ID            uuid.UUID
	Key           string
	Name          string
	PreviousKeys  []string
	PreviousNames []string
	MergedIDs     []uuid.UUID
}

// Teams is the unarchived teams among ids, by name.
func (self *ResolveRepository) Teams(ctx context.Context, ids []uuid.UUID) ([]Keyed, error) {
	teams, err := self.base.DB.Team.Query().Where(team.IDIn(ids...), team.ArchivedAtIsNil()).Order(ent.Asc(team.FieldName)).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Keyed, len(teams))
	for i, t := range teams {
		rows[i] = Keyed{ID: t.ID, Key: t.Key, Name: t.Name, PreviousKeys: t.PreviousKeys, PreviousNames: t.PreviousNames, MergedIDs: t.MergedTeamIds}
	}
	return rows, nil
}

// Aliases is what the workspace's teams, statuses and labels were also called: the ids of those
// merged into them (lowercase), and the earlier keys (uppercase) and names (lowercase) of teams.
type Aliases struct {
	IDs   map[string]string
	Keys  map[string]string
	Names map[string]string
}

func (self *ResolveRepository) Aliases(ctx context.Context, workspace uuid.UUID) (Aliases, error) {
	aliases := Aliases{IDs: map[string]string{}, Keys: map[string]string{}, Names: map[string]string{}}
	rows, err := self.base.DB.QueryContext(ctx,
		`SELECT 'id', unnest(merged_team_ids)::text, id::text FROM teams WHERE workspace_id = $1
		 UNION ALL SELECT 'key', upper(unnest(previous_keys)), key FROM teams WHERE workspace_id = $1 AND archived_at IS NULL
		 UNION ALL SELECT 'name', lower(unnest(previous_names)), name FROM teams WHERE workspace_id = $1 AND archived_at IS NULL
		 UNION ALL SELECT 'id', unnest(s.merged_ids)::text, s.id::text FROM workflow_states s JOIN teams t ON t.id = s.team_id
		 	WHERE t.workspace_id = $1
		 UNION ALL SELECT 'id', unnest(merged_ids)::text, id::text FROM issue_labels WHERE workspace_id = $1`, workspace)
	if err != nil {
		return aliases, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, alias, current string
		if err := rows.Scan(&kind, &alias, &current); err != nil {
			return aliases, err
		}
		switch kind {
		case "key":
			aliases.Keys[alias] = current
		case "name":
			aliases.Names[alias] = current
		default:
			aliases.IDs[alias] = current
		}
	}
	return aliases, rows.Err()
}

type Typed struct {
	ID        uuid.UUID
	Name      string
	Type      string
	MergedIDs []uuid.UUID
}

// States is the team's unarchived workflow states, in their order.
func (self *ResolveRepository) States(ctx context.Context, teamID uuid.UUID) ([]Typed, error) {
	states, err := self.base.DB.WorkflowState.Query().
		Where(workflowstate.TeamID(teamID), workflowstate.ArchivedAtIsNil()).
		Order(ent.Asc(workflowstate.FieldPosition)).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Typed, len(states))
	for i, s := range states {
		rows[i] = Typed{ID: s.ID, Name: s.Name, Type: s.Type, MergedIDs: s.MergedIds}
	}
	return rows, nil
}

// ProjectStatuses is the workspace's unarchived project statuses, in their order.
func (self *ResolveRepository) ProjectStatuses(ctx context.Context, workspace uuid.UUID) ([]Typed, error) {
	statuses, err := self.base.DB.ProjectStatus.Query().
		Where(projectstatus.WorkspaceID(workspace), projectstatus.ArchivedAtIsNil()).
		Order(ent.Asc(projectstatus.FieldPosition)).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Typed, len(statuses))
	for i, s := range statuses {
		rows[i] = Typed{ID: s.ID, Name: s.Name, Type: s.Type}
	}
	return rows, nil
}

func firstID(ctx context.Context, ids []uuid.UUID, err error) (uuid.UUID, bool, error) {
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}

func (self *ResolveRepository) IssueByID(ctx context.Context, workspace, id uuid.UUID) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Issue.Query().Where(issue.ID(id), issue.WorkspaceID(workspace)).IDs(ctx)
	return firstID(ctx, ids, err)
}

// IssueByNumber is the issue numbered number in the team keyed key; a team in use wins over an
// archived one with the same key.
func (self *ResolveRepository) IssueByNumber(ctx context.Context, workspace uuid.UUID, key string, number int) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Issue.Query().
		Where(issue.WorkspaceID(workspace), issue.Number(number)).
		Modify(func(s *entsql.Selector) {
			teams := entsql.Table(team.Table)
			s.Join(teams).On(s.C(issue.FieldTeamID), teams.C(team.FieldID)).
				Where(repository.Raw("upper("+teams.C(team.FieldKey)+") = ?", key))
			s.OrderExpr(entsql.Expr(teams.C(team.FieldArchivedAt) + " IS NOT NULL"))
			s.Limit(1)
		}).IDs(ctx)
	return firstID(ctx, ids, err)
}

func (self *ResolveRepository) IssueByPreviousIdentifier(ctx context.Context, workspace uuid.UUID, identifier string) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Issue.Query().
		Where(issue.WorkspaceID(workspace), func(s *entsql.Selector) {
			s.Where(repository.Raw("? = ANY("+s.C(issue.FieldPreviousIdentifiers)+")", identifier))
		}).IDs(ctx)
	return firstID(ctx, ids, err)
}

// IssueSyncedAs is the workspace's issue synced with this GitHub issue.
func (self *ResolveRepository) IssueSyncedAs(ctx context.Context, workspace uuid.UUID, repo string, number int) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.GithubIssueLink.Query().
		Where(githubissuelink.GithubNumber(number), githubissuelink.HasRepoSyncWith(
			githubreposync.WorkspaceID(workspace), githubreposync.FullNameEqualFold(repo),
		)).IDs(ctx)
	return firstID(ctx, ids, err)
}

// IssueVisible says whether the issue is in one of teams or shared with the user.
func (self *ResolveRepository) IssueVisible(ctx context.Context, id uuid.UUID, teams []uuid.UUID, viewer uuid.UUID, withTrashed bool) (bool, error) {
	query := self.base.DB.Issue.Query().
		Where(issue.ID(id), issue.Or(issue.TeamIDIn(teams...), issue.HasIssueSharesWith(issueshare.UserID(viewer))))
	if !withTrashed {
		query = query.Where(issue.TrashedAtIsNil())
	}
	return query.Exist(ctx)
}

type LabelRow struct {
	ID        uuid.UUID
	Name      string
	Parent    *string
	TeamID    *uuid.UUID
	IsGroup   bool
	MergedIDs []uuid.UUID
}

func labelRows(labels []*ent.IssueLabel) []LabelRow {
	rows := make([]LabelRow, len(labels))
	for i, l := range labels {
		rows[i] = LabelRow{ID: l.ID, Name: l.Name, TeamID: l.TeamID, IsGroup: l.IsGroup, MergedIDs: l.MergedIds}
		if l.Edges.Parent != nil {
			rows[i].Parent = &l.Edges.Parent.Name
		}
	}
	return rows
}

// IssueLabels is the unretired labels of the workspace and of the team, if any.
func (self *ResolveRepository) IssueLabels(ctx context.Context, workspace uuid.UUID, teamID *uuid.UUID) ([]LabelRow, error) {
	scope := []predicate.IssueLabel{issuelabel.TeamIDIsNil()}
	if teamID != nil {
		scope = append(scope, issuelabel.TeamID(*teamID))
	}
	labels, err := self.base.DB.IssueLabel.Query().
		Where(issuelabel.WorkspaceID(workspace), issuelabel.RetiredAtIsNil(), issuelabel.Or(scope...)).
		WithParent().All(ctx)
	if err != nil {
		return nil, err
	}
	return labelRows(labels), nil
}

// AllIssueLabels is every label (retired too) of the workspace and of teams, by name.
func (self *ResolveRepository) AllIssueLabels(ctx context.Context, workspace uuid.UUID, teams []uuid.UUID) ([]LabelRow, error) {
	labels, err := self.base.DB.IssueLabel.Query().
		Where(issuelabel.WorkspaceID(workspace), issuelabel.Or(issuelabel.TeamIDIsNil(), issuelabel.TeamIDIn(teams...))).
		WithParent().
		Order(func(s *entsql.Selector) { s.OrderExpr(entsql.Expr("lower(" + s.C(issuelabel.FieldName) + ")")) }).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return labelRows(labels), nil
}

// ProjectLabels is the workspace's unretired project labels that aren't groups.
func (self *ResolveRepository) ProjectLabels(ctx context.Context, workspace uuid.UUID) ([]LabelRow, error) {
	labels, err := self.base.DB.ProjectLabel.Query().
		Where(projectlabel.WorkspaceID(workspace), projectlabel.RetiredAtIsNil(), projectlabel.IsGroup(false)).
		WithParent().All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]LabelRow, len(labels))
	for i, l := range labels {
		rows[i] = LabelRow{ID: l.ID, Name: l.Name}
		if l.Edges.Parent != nil {
			rows[i].Parent = &l.Edges.Parent.Name
		}
	}
	return rows, nil
}

type Named struct {
	ID   uuid.UUID
	Name string
}

// ProjectVisible is a project of one of teams.
func ProjectVisible(teams []uuid.UUID) predicate.Project {
	return project.HasProjectTeamsWith(projectteam.TeamIDIn(teams...))
}

// Projects is the workspace's projects of teams that aren't trashed, the ones in use and most
// recently changed first.
func (self *ResolveRepository) Projects(ctx context.Context, workspace uuid.UUID, teams []uuid.UUID) ([]Named, error) {
	projects, err := self.base.DB.Project.Query().
		Where(project.WorkspaceID(workspace), ProjectVisible(teams), project.TrashedAtIsNil()).
		Order(func(s *entsql.Selector) { s.OrderExpr(entsql.Expr(s.C(project.FieldArchivedAt) + " IS NOT NULL")) },
			ent.Desc(project.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Named, len(projects))
	for i, p := range projects {
		rows[i] = Named{ID: p.ID, Name: p.Name}
	}
	return rows, nil
}

func (self *ResolveRepository) ProjectBySlug(ctx context.Context, workspace uuid.UUID, slug string) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Project.Query().Where(project.WorkspaceID(workspace), project.SlugID(slug)).IDs(ctx)
	return firstID(ctx, ids, err)
}

// MilestoneProject is the project of a milestone of the workspace.
func (self *ResolveRepository) MilestoneProject(ctx context.Context, workspace, id uuid.UUID) (uuid.UUID, bool, error) {
	milestone, err := self.base.DB.ProjectMilestone.Query().
		Where(projectmilestone.ID(id), projectmilestone.HasProjectWith(project.WorkspaceID(workspace))).Only(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return milestone.ProjectID, true, nil
}

// Milestones is the project's milestones in their order.
func (self *ResolveRepository) Milestones(ctx context.Context, projectID uuid.UUID) ([]Named, error) {
	milestones, err := self.base.DB.ProjectMilestone.Query().
		Where(projectmilestone.ProjectID(projectID)).Order(ent.Asc(projectmilestone.FieldSortOrder)).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Named, len(milestones))
	for i, m := range milestones {
		rows[i] = Named{ID: m.ID, Name: m.Name}
	}
	return rows, nil
}

func (self *ResolveRepository) CycleTeam(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	found, err := self.base.DB.Cycle.Query().Where(cycle.ID(id)).Only(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return found.TeamID, true, nil
}

// RelativeCycle is the team's current, next or previous unarchived cycle.
func (self *ResolveRepository) RelativeCycle(ctx context.Context, teamID uuid.UUID, which string) (uuid.UUID, bool, error) {
	query := self.base.DB.Cycle.Query().Where(cycle.TeamID(teamID), cycle.ArchivedAtIsNil())
	now := func(s *entsql.Selector, condition string) { s.Where(entsql.ExprP(condition)) }
	switch which {
	case "current":
		query = query.Where(func(s *entsql.Selector) {
			now(s, s.C(cycle.FieldStartsAt)+" <= now() AND "+s.C(cycle.FieldEndsAt)+" > now()")
		}).Order(ent.Desc(cycle.FieldStartsAt))
	case "next":
		query = query.Where(func(s *entsql.Selector) { now(s, s.C(cycle.FieldStartsAt)+" > now()") }).Order(ent.Asc(cycle.FieldStartsAt))
	case "previous":
		query = query.Where(func(s *entsql.Selector) { now(s, s.C(cycle.FieldEndsAt)+" <= now()") }).Order(ent.Desc(cycle.FieldEndsAt))
	default:
		return uuid.Nil, false, fmt.Errorf("no relative cycle %q", which)
	}
	ids, err := query.Limit(1).IDs(ctx)
	return firstID(ctx, ids, err)
}

func (self *ResolveRepository) CycleByNumber(ctx context.Context, teamID uuid.UUID, number int) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Cycle.Query().Where(cycle.TeamID(teamID), cycle.Number(number)).IDs(ctx)
	return firstID(ctx, ids, err)
}

func (self *ResolveRepository) CycleByName(ctx context.Context, teamID uuid.UUID, name string) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Cycle.Query().Where(cycle.TeamID(teamID), cycle.NameEqualFold(name)).Limit(1).IDs(ctx)
	return firstID(ctx, ids, err)
}

type Slugged struct {
	ID   uuid.UUID
	Name string
	Slug string
}

// Slugged is the rows of a table with a workspace, a slug and a name (nameColumn), the most
// recently changed first. extra narrows them with SQL on the table's columns.
func (self *ResolveRepository) Slugged(ctx context.Context, table, nameColumn string, workspace uuid.UUID, extra string, args ...any) ([]Slugged, error) {
	query := fmt.Sprintf("SELECT id, %s, slug_id FROM %s WHERE workspace_id = $1 %s ORDER BY updated_at DESC", nameColumn, table, extra)
	rows, err := self.base.DB.QueryContext(ctx, query, append([]any{workspace}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []Slugged
	for rows.Next() {
		var row Slugged
		if err := rows.Scan(&row.ID, &row.Name, &row.Slug); err != nil {
			return nil, err
		}
		found = append(found, row)
	}
	return found, rows.Err()
}

type DocumentOwners struct {
	Team, Project, Issue, Cycle *uuid.UUID
}

func (self *ResolveRepository) DocumentOwners(ctx context.Context, id uuid.UUID) (DocumentOwners, bool, error) {
	found, err := self.base.DB.Document.Query().Where(document.ID(id)).Only(ctx)
	if ent.IsNotFound(err) {
		return DocumentOwners{}, false, nil
	}
	if err != nil {
		return DocumentOwners{}, false, err
	}
	return DocumentOwners{Team: found.TeamID, Project: found.ProjectID, Issue: found.IssueID, Cycle: found.CycleID}, true, nil
}

// CustomerByDomain is the customer with this domain (lowercased) or external id.
func (self *ResolveRepository) CustomerByDomain(ctx context.Context, workspace uuid.UUID, value string) (uuid.UUID, bool, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		`SELECT id FROM customers WHERE workspace_id = $1 AND (lower($2) = ANY(domains) OR $2 = ANY(external_ids)) LIMIT 1`,
		workspace, value)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return uuid.Nil, false, rows.Err()
	}
	var id uuid.UUID
	err = rows.Scan(&id)
	return id, err == nil, err
}

type TemplateRow struct {
	ID   uuid.UUID
	Name string
	Team *uuid.UUID
}

// Templates is the workspace's templates of a type, by name.
func (self *ResolveRepository) Templates(ctx context.Context, workspace uuid.UUID, kind string) ([]TemplateRow, error) {
	templates, err := self.base.DB.Template.Query().
		Where(template.WorkspaceID(workspace), template.Type(kind)).Order(ent.Asc(template.FieldName)).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]TemplateRow, len(templates))
	for i, t := range templates {
		rows[i] = TemplateRow{ID: t.ID, Name: t.Name, Team: t.TeamID}
	}
	return rows, nil
}

// CustomViewsVisibleTo narrows custom views to shared ones and the viewer's own.
func CustomViewsVisibleTo(viewer uuid.UUID) string {
	return fmt.Sprintf("AND (shared OR owner_id = '%s')", viewer)
}

// InWorkspace says whether the table has a row with this id in the workspace.
func (self *ResolveRepository) InWorkspace(ctx context.Context, table string, id, workspace uuid.UUID) (bool, error) {
	if strings.ContainsAny(table, " ;\"'") {
		return false, fmt.Errorf("bad table %q", table)
	}
	rows, err := self.base.DB.QueryContext(ctx,
		fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE id = $1 AND workspace_id = $2)", table), id, workspace)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var exists bool
	if rows.Next() {
		err = rows.Scan(&exists)
	}
	return exists, err
}
