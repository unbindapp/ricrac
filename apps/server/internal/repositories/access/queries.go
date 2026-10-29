package access_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/apikey"
	"ricrac/ent/oauthtoken"
	"ricrac/ent/team"
	"ricrac/ent/teammember"
	"ricrac/ent/workspace"
	"ricrac/ent/workspacemember"
	"ricrac/internal/auth"
)

// Reachable is the workspaces of workspaces (all when nil) the user is an active member of, by name.
func (self *AccessRepository) Reachable(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID) ([]auth.Reached, error) {
	var rows []struct {
		ID     uuid.UUID `json:"id"`
		URLKey string    `json:"url_key"`
		Name   string    `json:"name"`
		Role   string    `json:"role"`
	}
	query := self.base.DB.Workspace.Query().Modify(func(s *entsql.Selector) {
		members := entsql.Table(workspacemember.Table)
		s.Join(members).On(s.C(workspace.FieldID), members.C(workspacemember.FieldWorkspaceID))
		s.Where(entsql.And(entsql.EQ(members.C(workspacemember.FieldUserID), user), entsql.EQ(members.C(workspacemember.FieldActive), true)))
		if workspaces != nil {
			s.Where(entsql.In(s.C(workspace.FieldID), anys(workspaces)...))
		}
		s.Select(s.C(workspace.FieldID), s.C(workspace.FieldURLKey), s.C(workspace.FieldName), members.C(workspacemember.FieldRole))
		s.OrderExpr(entsql.Expr("lower(" + s.C(workspace.FieldName) + ")"))
	})
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	reached := make([]auth.Reached, len(rows))
	for i, row := range rows {
		reached[i] = auth.Reached{ID: row.ID, URLKey: row.URLKey, Name: row.Name, Role: row.Role}
	}
	return reached, nil
}

// Role is the user's role in the workspace while they're an active member.
func (self *AccessRepository) Role(ctx context.Context, workspaceID, user uuid.UUID) (string, bool, error) {
	member, err := self.base.DB.WorkspaceMember.Query().
		Where(workspacemember.WorkspaceID(workspaceID), workspacemember.UserID(user), workspacemember.Active(true)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return member.Role, true, nil
}

// PublicWorkspace is the public workspace with this URL key or id, uuid.Nil when there's none.
func (self *AccessRepository) PublicWorkspace(ctx context.Context, named string) (uuid.UUID, error) {
	match := workspace.URLKey(named)
	if id, err := uuid.Parse(named); err == nil {
		match = workspace.Or(match, workspace.ID(id))
	}
	ids, err := self.base.DB.Workspace.Query().Where(match, workspace.Public(true)).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, err
	}
	return ids[0], nil
}

// VisibleTeams is the teams of the workspace whose issues the user sees: public ones unless
// they're a guest, the ones they belong to, or every team for Ricrac itself.
func (self *AccessRepository) VisibleTeams(ctx context.Context, workspaceID, user uuid.UUID, guest, everything bool) ([]uuid.UUID, error) {
	return self.base.DB.Team.Query().Where(team.WorkspaceID(workspaceID), func(s *entsql.Selector) {
		if everything {
			return
		}
		members := entsql.Table(teammember.Table)
		belongs := entsql.Exists(entsql.Select(members.C(teammember.FieldTeamID)).From(members).
			Where(entsql.And(entsql.ColumnsEQ(members.C(teammember.FieldTeamID), s.C(team.FieldID)), entsql.EQ(members.C(teammember.FieldUserID), user))))
		if guest {
			s.Where(belongs)
			return
		}
		s.Where(entsql.Or(entsql.EQ(s.C(team.FieldPrivate), false), belongs))
	}).IDs(ctx)
}

// APIKeyCaller is the caller of an API key by the hash of its secret, nil when there's none.
// It notes the key was used, at most once a minute.
func (self *AccessRepository) APIKeyCaller(ctx context.Context, hash string) (*auth.Caller, error) {
	key, err := self.base.DB.APIKey.Query().Where(apikey.KeyHash(hash)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = self.base.DB.ExecContext(ctx,
		"UPDATE api_keys SET last_used_at = now() WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')", key.ID)
	if err != nil {
		return nil, err
	}
	caller := &auth.Caller{User: key.UserID, Scopes: auth.ParseScopes(key.Scopes)}
	if key.WorkspaceIds != nil {
		caller.Workspaces = []uuid.UUID(*key.WorkspaceIds)
	}
	return caller, nil
}

// AccessTokenCaller is the caller of an unexpired OAuth access token by the hash of its secret,
// nil when there's none. It notes the grant was used, at most once a minute.
func (self *AccessRepository) AccessTokenCaller(ctx context.Context, hash string) (*auth.Caller, error) {
	token, err := self.base.DB.OAuthToken.Query().
		Where(oauthtoken.ID(hash), oauthtoken.Kind("access")).
		WithGrant().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if token.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	grant := token.Edges.Grant
	_, err = self.base.DB.ExecContext(ctx,
		"UPDATE oauth_grants SET last_used_at = now() WHERE id = $1 AND last_used_at < now() - interval '1 minute'", grant.ID)
	if err != nil {
		return nil, err
	}
	caller := &auth.Caller{User: grant.UserID, Scopes: auth.ParseScopes(grant.Scopes)}
	if grant.WorkspaceIds != nil {
		caller.Workspaces = []uuid.UUID(*grant.WorkspaceIds)
	}
	return caller, nil
}

func anys(ids []uuid.UUID) []any {
	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}
	return values
}
