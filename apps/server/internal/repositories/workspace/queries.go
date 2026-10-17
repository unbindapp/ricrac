package workspace_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/auditentry"
	"ricrac/ent/invitation"
	"ricrac/ent/issue"
	"ricrac/ent/predicate"
	"ricrac/ent/team"
	"ricrac/ent/teammember"
	"ricrac/ent/user"
	"ricrac/ent/workflowstate"
	"ricrac/ent/workspace"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

func (self *WorkspaceRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Workspace, error) {
	return self.base.Client(tx).Workspace.Get(ctx, id)
}

// GetForUpdate locks the workspace until the transaction ends.
func (self *WorkspaceRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Workspace, error) {
	return self.base.Client(tx).Workspace.Query().Where(workspace.ID(id)).ForUpdate().Only(ctx)
}

// URLKeyTaken says whether another workspace has this URL key.
func (self *WorkspaceRepository) URLKeyTaken(ctx context.Context, tx repository.TxInterface, key string, except uuid.UUID) (bool, error) {
	return self.base.Client(tx).Workspace.Query().Where(workspace.URLKey(key), workspace.IDNEQ(except)).Exist(ctx)
}

// Teams is the unarchived teams among ids, by name.
func (self *WorkspaceRepository) Teams(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Team, error) {
	return self.base.Client(tx).Team.Query().Where(team.IDIn(ids...), team.ArchivedAtIsNil()).Order(ent.Asc(team.FieldName)).All(ctx)
}

// TeamsByID is the teams among ids, archived ones too, by name.
func (self *WorkspaceRepository) TeamsByID(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Team, error) {
	return self.base.Client(tx).Team.Query().Where(team.IDIn(ids...)).Order(ent.Asc(team.FieldName)).All(ctx)
}

func (self *WorkspaceRepository) ActiveMemberCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).WorkspaceMember.Query().Where(workspacemember.WorkspaceID(id), workspacemember.Active(true)).Count(ctx)
}

// IssueCount is how many issues of teams the workspace has outside the trash.
func (self *WorkspaceRepository) IssueCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID, teams []uuid.UUID) (int, error) {
	return self.base.Client(tx).Issue.Query().
		Where(issue.WorkspaceID(id), issue.TeamIDIn(teams...), issue.TrashedAtIsNil()).Count(ctx)
}

type MemberFilter struct {
	Workspace       uuid.UUID
	IncludeInactive bool
	// An ILIKE pattern on the name, display name or email.
	Pattern *string
	Team    *uuid.UUID
	Limit   int64
	Offset  int64
}

// ListMembers is the members' user ids, by name then email.
func (self *WorkspaceRepository) ListMembers(ctx context.Context, filter MemberFilter) ([]uuid.UUID, error) {
	query := self.base.DB.User.Query().
		Where(user.HasWorkspaceMembersWith(memberIn(filter.Workspace, filter.IncludeInactive)...))
	if filter.Team != nil {
		query = query.Where(user.HasTeamMembersWith(teammember.TeamID(*filter.Team)))
	}
	return query.Modify(func(s *entsql.Selector) {
		if filter.Pattern != nil {
			s.Where(repository.Raw(s.C(user.FieldName)+" ILIKE ? OR "+s.C(user.FieldDisplayName)+" ILIKE ? OR "+s.C(user.FieldEmail)+" ILIKE ?",
				*filter.Pattern, *filter.Pattern, *filter.Pattern))
		}
		s.Select(s.C(user.FieldID))
		s.OrderExpr(entsql.Expr("lower(" + s.C(user.FieldName) + ")"))
		s.OrderBy(s.C(user.FieldEmail))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

func memberIn(id uuid.UUID, includeInactive bool) []predicate.WorkspaceMember {
	preds := []predicate.WorkspaceMember{workspacemember.WorkspaceID(id)}
	if !includeInactive {
		preds = append(preds, workspacemember.Active(true))
	}
	return preds
}

// Memberships is the memberships of users in the workspace with their user, in the order of
// users.
func (self *WorkspaceRepository) Memberships(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) ([]*ent.WorkspaceMember, error) {
	members, err := self.base.Client(tx).WorkspaceMember.Query().
		Where(workspacemember.WorkspaceID(id), workspacemember.UserIDIn(users...)).WithUser().All(ctx)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uuid.UUID]*ent.WorkspaceMember, len(members))
	for _, m := range members {
		byUser[m.UserID] = m
	}
	ordered := make([]*ent.WorkspaceMember, 0, len(users))
	for _, u := range users {
		if m, ok := byUser[u]; ok {
			ordered = append(ordered, m)
		}
	}
	return ordered, nil
}

// MemberTeams is, for each of users, their unarchived teams of the workspace among visible, by
// name.
func (self *WorkspaceRepository) MemberTeams(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users, visible []uuid.UUID) (map[uuid.UUID][]*ent.Team, error) {
	teams, err := self.base.Client(tx).Team.Query().
		Where(team.WorkspaceID(id), team.ArchivedAtIsNil(), team.IDIn(visible...), team.HasTeamMembersWith(teammember.UserIDIn(users...))).
		WithTeamMembers(func(q *ent.TeamMemberQuery) { q.Where(teammember.UserIDIn(users...)) }).
		Order(ent.Asc(team.FieldName)).All(ctx)
	if err != nil {
		return nil, err
	}
	byUser := map[uuid.UUID][]*ent.Team{}
	for _, t := range teams {
		for _, m := range t.Edges.TeamMembers {
			byUser[m.UserID] = append(byUser[m.UserID], t)
		}
	}
	return byUser, nil
}

// OpenAssignedCount is how many open issues of the workspace, not archived or trashed, are
// assigned to the user.
func (self *WorkspaceRepository) OpenAssignedCount(ctx context.Context, tx repository.TxInterface, id, assignee uuid.UUID) (int, error) {
	return self.base.Client(tx).Issue.Query().
		Where(issue.WorkspaceID(id), issue.AssigneeID(assignee), issue.TrashedAtIsNil(), issue.ArchivedAtIsNil(),
			issue.HasStateWith(workflowstate.TypeNotIn("completed", "canceled"))).Count(ctx)
}

// HasMember says whether the user is or was a member of the workspace.
func (self *WorkspaceRepository) HasMember(ctx context.Context, id, user uuid.UUID) (bool, error) {
	return self.base.DB.WorkspaceMember.Query().Where(workspacemember.WorkspaceID(id), workspacemember.UserID(user)).Exist(ctx)
}

// RoleForUpdate is the member's role, locking the membership until the transaction ends.
func (self *WorkspaceRepository) RoleForUpdate(ctx context.Context, tx repository.TxInterface, id, user uuid.UUID) (string, error) {
	member, err := self.base.Client(tx).WorkspaceMember.Query().
		Where(workspacemember.WorkspaceID(id), workspacemember.UserID(user)).ForUpdate().Only(ctx)
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

func (self *WorkspaceRepository) ActiveOwners(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).WorkspaceMember.Query().
		Where(workspacemember.WorkspaceID(id), workspacemember.Role("owner"), workspacemember.Active(true)).Count(ctx)
}

// TimezoneKnown says whether Postgres knows the IANA timezone.
func (self *WorkspaceRepository) TimezoneKnown(ctx context.Context, tx repository.TxInterface, zone string) (bool, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)", zone)
	if err != nil {
		return false, err
	}
	var known bool
	return known, scan(rows, &known)
}

func (self *WorkspaceRepository) User(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.User, error) {
	return self.base.Client(tx).User.Get(ctx, id)
}

// HasActiveMemberWithEmail says whether an active member has this email, in any case.
func (self *WorkspaceRepository) HasActiveMemberWithEmail(ctx context.Context, tx repository.TxInterface, id uuid.UUID, email string) (bool, error) {
	return self.base.Client(tx).User.Query().
		Where(user.HasWorkspaceMembersWith(workspacemember.WorkspaceID(id), workspacemember.Active(true)),
			func(s *entsql.Selector) { s.Where(repository.Raw("lower("+s.C(user.FieldEmail)+") = ?", email)) }).
		Exist(ctx)
}

// ListInvitations is the ids of the workspace's unexpired invitations, newest first.
func (self *WorkspaceRepository) ListInvitations(ctx context.Context, id uuid.UUID, limit, offset int64) ([]uuid.UUID, error) {
	return self.base.DB.Invitation.Query().
		Where(invitation.WorkspaceID(id), func(s *entsql.Selector) { s.Where(repository.Raw(s.C(invitation.FieldExpiresAt) + " > now()")) }).
		Order(ent.Desc(invitation.FieldCreatedAt)).Limit(int(limit)).Offset(int(offset)).IDs(ctx)
}

// Invitations is the invitations with their inviter, in the order of ids.
func (self *WorkspaceRepository) Invitations(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Invitation, error) {
	invitations, err := self.base.Client(tx).Invitation.Query().Where(invitation.IDIn(ids...)).WithInviter().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Invitation, len(invitations))
	for _, i := range invitations {
		byID[i.ID] = i
	}
	ordered := make([]*ent.Invitation, 0, len(ids))
	for _, id := range ids {
		if i, ok := byID[id]; ok {
			ordered = append(ordered, i)
		}
	}
	return ordered, nil
}

func (self *WorkspaceRepository) Invitation(ctx context.Context, id uuid.UUID) (*ent.Invitation, error) {
	return self.base.DB.Invitation.Get(ctx, id)
}

func (self *WorkspaceRepository) DeleteInvitation(ctx context.Context, id uuid.UUID) error {
	_, err := self.base.DB.Invitation.Delete().Where(invitation.ID(id)).Exec(ctx)
	return err
}

// AuditEntries is the workspace's audit log with its actors, newest first, of kind when set.
func (self *WorkspaceRepository) AuditEntries(ctx context.Context, id uuid.UUID, kind *string, limit, offset int64) ([]*ent.AuditEntry, error) {
	query := self.base.DB.AuditEntry.Query().Where(auditentry.WorkspaceID(id))
	if kind != nil {
		query = query.Where(auditentry.Type(*kind))
	}
	return query.WithActor().Order(ent.Desc(auditentry.FieldID)).Limit(int(limit)).Offset(int(offset)).All(ctx)
}
