package team_repo

import (
	"context"
	"database/sql"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/issue"
	"ricrac/ent/issuelabel"
	"ricrac/ent/team"
	"ricrac/ent/teammember"
	"ricrac/ent/user"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
	state_repo "ricrac/internal/repositories/state"
)

func one(rows *sql.Rows, dest ...any) error {
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
	}
	return rows.Err()
}

func ordered(teams []*ent.Team, ids []uuid.UUID) []*ent.Team {
	byID := make(map[uuid.UUID]*ent.Team, len(teams))
	for _, t := range teams {
		byID[t.ID] = t
	}
	found := make([]*ent.Team, 0, len(ids))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			found = append(found, t)
		}
	}
	return found
}

func (self *TeamRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Team, error) {
	return self.base.Client(tx).Team.Get(ctx, id)
}

// Lock locks the team until the transaction ends.
func (self *TeamRepository) Lock(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).Team.Query().Where(team.ID(id)).ForUpdate().IDs(ctx)
	return err
}

// Load is the teams with their workspace and parent, in the order of ids.
func (self *TeamRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Team, error) {
	teams, err := self.base.Client(tx).Team.Query().Where(team.IDIn(ids...)).WithWorkspace().WithParent().All(ctx)
	if err != nil {
		return nil, err
	}
	return ordered(teams, ids), nil
}

// LoadDetail is the team with what its full answer shows: workspace, parent, the states its
// settings name, active states in board order, members by name, the unarchived sub-teams among
// visible or public ones by name, and the current cycle.
func (self *TeamRepository) LoadDetail(ctx context.Context, tx repository.TxInterface, id uuid.UUID, visible []uuid.UUID) (*ent.Team, error) {
	return self.base.Client(tx).Team.Query().Where(team.ID(id)).
		WithWorkspace().WithParent().
		WithAutoCloseState().WithDefaultIssueState().WithGithubPrOpenedState().WithGithubPrMergedState().
		WithWorkflowStates(func(q *ent.WorkflowStateQuery) {
			q.Where(workflowstate.ArchivedAtIsNil()).Order(state_repo.BoardOrder)
		}).
		WithTeamMembers(func(q *ent.TeamMemberQuery) {
			q.WithUser().Order(func(s *entsql.Selector) {
				users := entsql.Table(user.Table)
				s.Join(users).On(s.C(teammember.FieldUserID), users.C(user.FieldID))
				s.OrderExpr(entsql.Expr("lower(" + users.C(user.FieldName) + ")"))
			})
		}).
		WithChildren(func(q *ent.TeamQuery) {
			q.Where(team.ArchivedAtIsNil(), team.Or(team.IDIn(visible...), team.Private(false))).Order(ent.Asc(team.FieldName))
		}).
		WithCycles(func(q *ent.CycleQuery) {
			q.Where(cycle.ArchivedAtIsNil(), func(s *entsql.Selector) {
				s.Where(repository.Raw(s.C(cycle.FieldStartsAt) + " <= now() AND " + s.C(cycle.FieldEndsAt) + " > now()"))
			}).Order(ent.Desc(cycle.FieldStartsAt)).Limit(1)
		}).
		Only(ctx)
}

// MemberCounts is how many members each team has.
func (self *TeamRepository) MemberCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		TeamID uuid.UUID `json:"team_id"`
		Count  int       `json:"count"`
	}
	err := self.base.Client(tx).TeamMember.Query().Where(teammember.TeamIDIn(ids...)).
		GroupBy(teammember.FieldTeamID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.TeamID] = row.Count
	}
	return counts, nil
}

// MemberOf is which of the teams the user is a member of.
func (self *TeamRepository) MemberOf(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, user uuid.UUID) (map[uuid.UUID]bool, error) {
	members, err := self.base.Client(tx).TeamMember.Query().Where(teammember.TeamIDIn(ids...), teammember.UserID(user)).All(ctx)
	if err != nil {
		return nil, err
	}
	in := make(map[uuid.UUID]bool, len(members))
	for _, m := range members {
		in[m.TeamID] = true
	}
	return in, nil
}

// OpenIssueCounts is how many open issues, not archived or trashed, each team has.
func (self *TeamRepository) OpenIssueCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		TeamID uuid.UUID `json:"team_id"`
		Count  int       `json:"count"`
	}
	err := self.base.Client(tx).Issue.Query().
		Where(issue.TeamIDIn(ids...), issue.TrashedAtIsNil(), issue.ArchivedAtIsNil(),
			issue.HasStateWith(workflowstate.TypeNotIn("completed", "canceled"))).
		GroupBy(issue.FieldTeamID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.TeamID] = row.Count
	}
	return counts, nil
}

// LabelCount is how many unretired labels that aren't groups the team has.
func (self *TeamRepository) LabelCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).IssueLabel.Query().
		Where(issuelabel.TeamID(id), issuelabel.RetiredAtIsNil(), issuelabel.IsGroup(false)).Count(ctx)
}

// Users is the users of ids, in their order.
func (self *TeamRepository) Users(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.User, error) {
	users, err := self.base.Client(tx).User.Query().Where(user.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	found := make([]*ent.User, 0, len(ids))
	for _, id := range ids {
		if u, ok := byID[id]; ok {
			found = append(found, u)
		}
	}
	return found, nil
}

type ListFilter struct {
	Teams           []uuid.UUID
	IncludeArchived bool
	// Only teams this user is a member of, when set.
	Member *uuid.UUID
	// An ILIKE pattern on the name or key.
	Pattern *string
	Limit   int64
	Offset  int64
}

// List is the teams among Teams, by name.
func (self *TeamRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	query := self.base.DB.Team.Query().Where(team.IDIn(filter.Teams...))
	if !filter.IncludeArchived {
		query = query.Where(team.ArchivedAtIsNil())
	}
	if filter.Member != nil {
		query = query.Where(team.HasTeamMembersWith(teammember.UserID(*filter.Member)))
	}
	return query.Modify(func(s *entsql.Selector) {
		if filter.Pattern != nil {
			s.Where(repository.Raw(s.C(team.FieldName)+" ILIKE ? OR "+s.C(team.FieldKey)+" ILIKE ?", *filter.Pattern, *filter.Pattern))
		}
		s.Select(s.C(team.FieldID))
		s.OrderExpr(entsql.Expr("lower(" + s.C(team.FieldName) + ")"))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

type Membership struct {
	Private bool
	Member  bool
	Owner   bool
}

// Membership is whether the team is private and the user is a member or owner of it.
func (self *TeamRepository) Membership(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID) (Membership, error) {
	found, err := self.base.Client(tx).Team.Query().Where(team.ID(id)).
		WithTeamMembers(func(q *ent.TeamMemberQuery) { q.Where(teammember.UserID(user)) }).Only(ctx)
	if err != nil {
		return Membership{}, err
	}
	m := Membership{Private: found.Private}
	if len(found.Edges.TeamMembers) > 0 {
		m.Member = true
		m.Owner = found.Edges.TeamMembers[0].Owner
	}
	return m, nil
}

// FindUnarchived is an unarchived team of the workspace by id, key or name, whatever its
// privacy: the id first, then the key.
func (self *TeamRepository) FindUnarchived(ctx context.Context, workspace uuid.UUID, value string) (uuid.UUID, bool, error) {
	ids, err := self.base.DB.Team.Query().Where(team.WorkspaceID(workspace), team.ArchivedAtIsNil()).
		Modify(func(s *entsql.Selector) {
			id, key, name := s.C(team.FieldID)+"::text", s.C(team.FieldKey), s.C(team.FieldName)
			s.Where(repository.Raw(id+" = ? OR upper("+key+") = upper(?) OR lower("+name+") = lower(?)", value, value, value))
			s.Select(s.C(team.FieldID))
			s.OrderExpr(repository.RawExpr(id+" = ? DESC", value))
			s.OrderExpr(repository.RawExpr("upper("+key+") = upper(?) DESC", value))
			s.Limit(1)
		}).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}

// KeyTakenBy is the name of another unarchived team of the workspace with the key.
func (self *TeamRepository) KeyTakenBy(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, key string, except *uuid.UUID) (string, bool, error) {
	query := self.base.Client(tx).Team.Query().
		Where(team.WorkspaceID(workspace), team.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("upper("+s.C(team.FieldKey)+") = ?", key))
		})
	if except != nil {
		query = query.Where(team.IDNEQ(*except))
	}
	found, err := query.First(ctx)
	if ent.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return found.Name, true, nil
}

// NameTaken says whether another unarchived team of the workspace has the name, in any case.
func (self *TeamRepository) NameTaken(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, name string, except *uuid.UUID) (bool, error) {
	query := self.base.Client(tx).Team.Query().
		Where(team.WorkspaceID(workspace), team.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(team.FieldName)+") = lower(?)", name))
		})
	if except != nil {
		query = query.Where(team.IDNEQ(*except))
	}
	return query.Exist(ctx)
}

// InOwnTree says whether team is parent or one of its ancestors.
func (self *TeamRepository) InOwnTree(ctx context.Context, tx repository.TxInterface, parent, id uuid.UUID) (bool, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`WITH RECURSIVE up AS (SELECT id, parent_id FROM teams WHERE id = $1
		 UNION SELECT t.id, t.parent_id FROM teams t JOIN up ON t.id = up.parent_id)
		 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, parent, id)
	if err != nil {
		return false, err
	}
	var loops bool
	return loops, one(rows, &loops)
}

// Blind is the names of users who can't see the team: neither its members nor, for a public
// team, members of the workspace who aren't guests.
func (self *TeamRepository) Blind(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) ([]string, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT u.name FROM users u JOIN teams t ON t.id = $1
		 JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = u.id
		 WHERE u.id = ANY($2) AND NOT ((NOT t.private AND m.role <> 'guest')
			OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = u.id))`, id, users)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// IssuesInStates is the issues in any of the states.
func (self *TeamRepository) IssuesInStates(ctx context.Context, tx repository.TxInterface, states []uuid.UUID) ([]uuid.UUID, error) {
	return self.base.Client(tx).Issue.Query().Where(issue.StateIDIn(states...)).IDs(ctx)
}

// ArchivedTriageState is the team's most recently archived triage state, when it has no active
// one.
func (self *TeamRepository) ArchivedTriageState(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (uuid.UUID, bool, error) {
	client := self.base.Client(tx)
	active, err := client.WorkflowState.Query().
		Where(workflowstate.TeamID(id), workflowstate.Type("triage"), workflowstate.ArchivedAtIsNil()).Exist(ctx)
	if err != nil || active {
		return uuid.Nil, false, err
	}
	ids, err := client.WorkflowState.Query().
		Where(workflowstate.TeamID(id), workflowstate.Type("triage"), workflowstate.ArchivedAtNotNil()).
		Order(ent.Desc(workflowstate.FieldArchivedAt)).Limit(1).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}
