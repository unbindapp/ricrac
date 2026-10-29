package auth

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"

	"ricrac/internal/common/errdefs"
)

// Caller is who is calling and which workspaces their token reaches. Each operation then runs
// in one of them, as an Actor.
type Caller struct {
	User   uuid.UUID
	Scopes []Access
	// Nil reaches every workspace the user belongs to, including ones they join later.
	Workspaces []uuid.UUID
}

// SessionCaller is a dashboard session: every workspace, everything the user's roles allow.
func SessionCaller(user uuid.UUID) *Caller {
	return &Caller{User: user, Scopes: EveryAccess}
}

func (c *Caller) Require(access Access) error {
	if !slices.Contains(c.Scopes, access) {
		return ScopeError(access)
	}
	return nil
}

// Reached is a workspace a caller can act in.
type Reached struct {
	ID     uuid.UUID
	URLKey string
	Name   string
	Role   string
}

// Origin is where a change Ricrac makes on its own comes from. Issue history records it.
type Origin struct {
	// What happened, e.g. "Pull request #12 merged".
	Label string
	URL   string
	// The change mirrors one made on GitHub, so it isn't sent back there.
	FromGitHubIssue bool
}

// TeamLoader lists the teams whose issues an actor can see.
type TeamLoader func(ctx context.Context, actor *Actor) ([]uuid.UUID, error)

// Actor is who is calling in which workspace. Every operation runs with one, whether it came
// through the REST API, MCP or the dashboard.
type Actor struct {
	// uuid.Nil for changes Ricrac makes itself (see Origin).
	User      uuid.UUID
	Workspace uuid.UUID
	Role      Role
	Scopes    []Access
	// Set for changes that come from elsewhere, like GitHub, rather than a caller.
	Origin *Origin
	// Someone who isn't a member reading a public workspace: no user, reading only.
	Visitor bool

	loadTeams TeamLoader
	teamsOnce sync.Once
	teams     []uuid.UUID
	teamsErr  error
}

func NewActor(user, workspace uuid.UUID, role Role, scopes []Access, origin *Origin, teams TeamLoader) *Actor {
	return &Actor{User: user, Workspace: workspace, Role: role, Scopes: scopes, Origin: origin, loadTeams: teams}
}

// NewVisitor is someone reading a public workspace without being a member of it.
func NewVisitor(workspace uuid.UUID, teams TeamLoader) *Actor {
	actor := NewActor(uuid.Nil, workspace, Guest, []Access{Read}, nil, teams)
	actor.Visitor = true
	return actor
}

// ActorID is the user making the change, nil when Ricrac makes it on its own.
func (a *Actor) ActorID() *uuid.UUID {
	if a.User == uuid.Nil {
		return nil
	}
	return &a.User
}

// Fresh is the same actor with nothing cached, as a new request would see it.
func (a *Actor) Fresh() *Actor {
	fresh := NewActor(a.User, a.Workspace, a.Role, a.Scopes, a.Origin, a.loadTeams)
	fresh.Visitor = a.Visitor
	return fresh
}

func (a *Actor) Allows(access Access) bool {
	return slices.Contains(a.Scopes, access)
}

// Caller is the same caller, reaching only this workspace.
func (a *Actor) Caller() *Caller {
	return &Caller{User: a.User, Scopes: a.Scopes, Workspaces: []uuid.UUID{a.Workspace}}
}

func (a *Actor) Require(access Access) error {
	if !a.Allows(access) {
		return ScopeError(access)
	}
	if access == AdminAccess {
		return a.RequireAdmin()
	}
	return nil
}

func (a *Actor) RequireAdmin() error {
	if a.Role < Admin {
		return errdefs.Forbidden("Only workspace admins can do this")
	}
	return nil
}

// RequireMember keeps guests, who only work inside the teams they were added to, out.
func (a *Actor) RequireMember() error {
	if a.Role == Guest {
		return errdefs.Forbidden("Guests can't do this")
	}
	return nil
}

// TeamIDs is the teams whose issues this user can see: every public team (except for guests)
// and the private teams they belong to; all of them for Ricrac itself (Origin). Archived teams
// are included so their issues stay readable.
func (a *Actor) TeamIDs(ctx context.Context) ([]uuid.UUID, error) {
	a.teamsOnce.Do(func() {
		a.teams, a.teamsErr = a.loadTeams(ctx, a)
		if a.teams == nil {
			a.teams = []uuid.UUID{}
		}
	})
	return a.teams, a.teamsErr
}

func (a *Actor) CanSeeTeam(ctx context.Context, team uuid.UUID) (bool, error) {
	ids, err := a.TeamIDs(ctx)
	return slices.Contains(ids, team), err
}
