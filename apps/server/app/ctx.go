package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Role int

const (
	Guest Role = iota
	Member
	Admin
	Owner
)

func ParseRole(value string) (Role, bool) {
	switch value {
	case "guest":
		return Guest, true
	case "member":
		return Member, true
	case "admin":
		return Admin, true
	case "owner":
		return Owner, true
	}
	return Guest, false
}

func (r Role) String() string {
	return [...]string{"guest", "member", "admin", "owner"}[r]
}

// Access is what a token may do. Sessions and keys without scopes may do everything their role
// allows.
type Access int

const (
	Read Access = iota
	Write
	AdminAccess
)

func (a Access) String() string {
	return [...]string{"read", "write", "admin"}[a]
}

var AllScopes = []string{"read", "write", "admin"}

var EveryAccess = []Access{Read, Write, AdminAccess}

func ParseScopes(scopes []string) []Access {
	var found []Access
	for _, scope := range scopes {
		switch scope {
		case "read":
			found = append(found, Read)
		case "write":
			found = append(found, Write)
		case "admin":
			found = append(found, AdminAccess)
		}
	}
	return found
}

func scopeError(access Access) *Error {
	return Forbidden("This token lacks the %q scope", access.String())
}

// Caller is who is calling and which workspaces their token reaches. Each operation then runs
// in one of them, as a Ctx.
type Caller struct {
	context.Context
	State  *State
	User   uuid.UUID
	Scopes []Access
	// Nil reaches every workspace the user belongs to, including ones they join later.
	Workspaces []uuid.UUID
}

// Reached is a workspace a caller can act in.
type Reached struct {
	ID     uuid.UUID
	URLKey string
	Name   string
	Role   string
}

// SessionCaller is a dashboard session: every workspace, everything the user's roles allow.
func SessionCaller(ctx context.Context, state *State, user uuid.UUID) *Caller {
	return &Caller{Context: ctx, State: state, User: user, Scopes: EveryAccess}
}

func (c *Caller) Require(access Access) error {
	if !slices.Contains(c.Scopes, access) {
		return scopeError(access)
	}
	return nil
}

func (c *Caller) DB() *pgxpool.Pool {
	return c.State.DB
}

// Reachable is the workspaces the token reaches that the user is still an active member of,
// by name.
func (c *Caller) Reachable() ([]Reached, error) {
	return Rows[Reached](c, c.State.DB,
		`SELECT w.id, w.url_key, w.name, m.role FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
		 WHERE m.user_id = $1 AND m.active AND ($2::uuid[] IS NULL OR w.id = ANY($2)) ORDER BY lower(w.name)`,
		c.User, c.Workspaces)
}

// Ctx is the caller in one workspace: the one named (URL key or id), else the one the links
// among the arguments point to, else the only one the token reaches.
func (c *Caller) Ctx(named string, args map[string]any) (*Ctx, error) {
	reachable, err := c.Reachable()
	if err != nil {
		return nil, err
	}
	if len(reachable) == 0 {
		return nil, Forbidden("This token doesn't reach any workspace you're a member of")
	}
	var chosen *Reached
	if name := strings.TrimSpace(named); name != "" {
		key := strings.ToLower(name)
		for i := range reachable {
			if reachable[i].URLKey == key || reachable[i].ID.String() == key {
				chosen = &reachable[i]
			}
		}
		if chosen == nil {
			return nil, BadRequest("No workspace %q here; use one of %s", name, choices(reachable))
		}
	} else {
		chosen = linked(reachable, args, c.State.Config.PublicURL)
		if chosen == nil && len(reachable) == 1 {
			chosen = &reachable[0]
		}
		if chosen == nil {
			return nil, BadRequest("This token reaches several workspaces: pass \"workspace\" with one of %s", choices(reachable))
		}
	}
	role, _ := ParseRole(chosen.Role)
	return &Ctx{Context: c.Context, State: c.State, User: c.User, Workspace: chosen.ID, Role: role, Scopes: c.Scopes}, nil
}

func choices(reachable []Reached) string {
	names := make([]string, len(reachable))
	for i, w := range reachable {
		names[i] = fmt.Sprintf("%q (%s)", w.URLKey, w.Name)
	}
	return strings.Join(names, ", ")
}

// linked is the workspace of the links passed as arguments, like
// https://ricrac.example/acme/issue/ENG-1, when they all point to the same one.
func linked(reachable []Reached, args map[string]any, base string) *Reached {
	prefix := base + "/"
	var found *Reached
	for _, value := range args {
		items, ok := value.([]any)
		if !ok {
			items = []any{value}
		}
		for _, item := range items {
			link, ok := item.(string)
			if !ok || strings.ContainsAny(link, " \t\n\r") {
				continue
			}
			rest, ok := strings.CutPrefix(link, prefix)
			if !ok {
				continue
			}
			key, _, _ := strings.Cut(rest, "/")
			for i := range reachable {
				if reachable[i].URLKey != key {
					continue
				}
				if found != nil && found.ID != reachable[i].ID {
					return nil
				}
				found = &reachable[i]
			}
		}
	}
	return found
}

// Ctx is who is calling in which workspace. Every operation runs with one, whether it came
// through the REST API, MCP or the dashboard.
type Ctx struct {
	context.Context
	State     *State
	User      uuid.UUID
	Workspace uuid.UUID
	Role      Role
	Scopes    []Access

	teamsOnce sync.Once
	teams     []uuid.UUID
	teamsErr  error
}

// LoadCtx is the context of user in workspace, or nil when they aren't an active member.
func LoadCtx(ctx context.Context, state *State, user, workspace uuid.UUID, scopes []Access) (*Ctx, error) {
	role, found, err := Maybe[string](ctx, state.DB,
		"SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND active", workspace, user)
	if err != nil || !found {
		return nil, err
	}
	parsed, ok := ParseRole(role)
	if !ok {
		return nil, nil
	}
	return &Ctx{Context: ctx, State: state, User: user, Workspace: workspace, Role: parsed, Scopes: scopes}, nil
}

// Fresh is the same caller with nothing cached, as a new request would see it.
func (c *Ctx) Fresh() *Ctx {
	return &Ctx{Context: c.Context, State: c.State, User: c.User, Workspace: c.Workspace, Role: c.Role, Scopes: c.Scopes}
}

func (c *Ctx) DB() *pgxpool.Pool {
	return c.State.DB
}

func (c *Ctx) PublicURL() string {
	return c.State.Config.PublicURL
}

func (c *Ctx) Allows(access Access) bool {
	return slices.Contains(c.Scopes, access)
}

// Caller is the same caller, reaching only this workspace.
func (c *Ctx) Caller() *Caller {
	return &Caller{Context: c.Context, State: c.State, User: c.User, Scopes: c.Scopes, Workspaces: []uuid.UUID{c.Workspace}}
}

func (c *Ctx) Require(access Access) error {
	if !c.Allows(access) {
		return scopeError(access)
	}
	if access == AdminAccess {
		return c.RequireAdmin()
	}
	return nil
}

func (c *Ctx) RequireAdmin() error {
	if c.Role < Admin {
		return Forbidden("Only workspace admins can do this")
	}
	return nil
}

// RequireMember keeps guests, who only work inside the teams they were added to, out.
func (c *Ctx) RequireMember() error {
	if c.Role == Guest {
		return Forbidden("Guests can't do this")
	}
	return nil
}

// TeamIDs is the teams whose issues this user can see: every public team (except for guests)
// and the private teams they belong to. Archived teams are included so their issues stay
// readable.
func (c *Ctx) TeamIDs() ([]uuid.UUID, error) {
	c.teamsOnce.Do(func() {
		c.teams, c.teamsErr = All[uuid.UUID](c, c.State.DB,
			`SELECT t.id FROM teams t WHERE t.workspace_id = $1 AND (
				($2 AND NOT t.private)
				OR EXISTS (SELECT 1 FROM team_members m WHERE m.team_id = t.id AND m.user_id = $3)
			 )`, c.Workspace, c.Role != Guest, c.User)
		if c.teams == nil {
			c.teams = []uuid.UUID{}
		}
	})
	return c.teams, c.teamsErr
}

func (c *Ctx) CanSeeTeam(team uuid.UUID) (bool, error) {
	ids, err := c.TeamIDs()
	return slices.Contains(ids, team), err
}
