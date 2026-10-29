package models

import (
	"encoding/json"

	"ricrac/ent"
)

type WorkspaceList struct {
	Nodes []ReachableWorkspace `json:"nodes"`
}

type ReachableWorkspace struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URLKey string `json:"urlKey"`
	URL    string `json:"url"`
	Role   string `json:"role" enum:"owner,admin,member,guest"`
}

type Workspace struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	URLKey      string          `json:"urlKey"`
	LogoURL     *string         `json:"logoUrl"`
	URL         string          `json:"url"`
	Theme       string          `json:"theme" enum:"default,dank" doc:"The look of the web app for everyone in the workspace."`
	Public      bool            `json:"public" doc:"Anyone with a link can read its issues without signing in."`
	CreatedAt   string          `json:"createdAt"`
	Role        string          `json:"role" enum:"owner,admin,member,guest" doc:"Your role."`
	Teams       []WorkspaceTeam `json:"teams" doc:"The workspace's team (a workspace has one), when you can see it; its key is the issue prefix."`
	MemberCount int             `json:"memberCount"`
	IssueCount  int             `json:"issueCount" doc:"Issues of the teams you can see, trashed ones left out."`
}

type WorkspaceTeam struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	Private bool   `json:"private"`
}

type WorkspaceUser struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	DisplayName        string    `json:"displayName"`
	Email              string    `json:"email"`
	AvatarURL          *string   `json:"avatarUrl"`
	Timezone           *string   `json:"timezone"`
	Role               string    `json:"role" enum:"owner,admin,member,guest"`
	Active             bool      `json:"active" doc:"False once removed from the workspace."`
	IsMe               bool      `json:"isMe"`
	EmailNotifications *bool     `json:"emailNotifications" doc:"Only shown for yourself."`
	JoinedAt           string    `json:"joinedAt"`
	Teams              []TeamRef `json:"teams" doc:"Their teams that you can see."`
}

type WorkspaceUserDetail struct {
	WorkspaceUser
	OpenIssueCount int `json:"openIssueCount" doc:"Open issues assigned to them."`
}

type RemovedUser struct {
	Removed string `json:"removed"`
}

type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role" enum:"admin,member,guest"`
	Teams     []TeamRef `json:"teams" doc:"The teams they join."`
	Inviter   *UserRef  `json:"inviter"`
	CreatedAt string    `json:"createdAt"`
	ExpiresAt string    `json:"expiresAt"`
}

type AuditEntry struct {
	ID        int      `json:"id"`
	Type      string   `json:"type" doc:"Like member.roleChanged, member.removed, invitation.created, team.archived or workspace.updated."`
	Actor     *UserRef `json:"actor"`
	Metadata  M        `json:"metadata"`
	CreatedAt string   `json:"createdAt"`
}

// WorkspaceOf needs the workspace's visible teams, by name.
func WorkspaceOf(w *ent.Workspace, publicURL, role string, teams []*ent.Team, memberCount, issueCount int) Workspace {
	workspaceTeams := make([]WorkspaceTeam, len(teams))
	for i, t := range teams {
		workspaceTeams[i] = WorkspaceTeam{ID: ID(t.ID), Key: t.Key, Name: t.Name, Private: t.Private}
	}
	return Workspace{
		ID: ID(w.ID), Name: w.Name, URLKey: w.URLKey, LogoURL: w.LogoURL, URL: publicURL + "/" + w.URLKey, Theme: w.Theme,
		Public: w.Public, CreatedAt: Time(w.CreatedAt), Role: role, Teams: workspaceTeams, MemberCount: memberCount, IssueCount: issueCount,
	}
}

// WorkspaceUserOf needs the membership's user loaded; teams are their teams the viewer sees, by
// name.
func WorkspaceUserOf(m *ent.WorkspaceMember, viewer string, teams []*ent.Team) WorkspaceUser {
	u := m.Edges.User
	refs := make([]TeamRef, len(teams))
	for i, t := range teams {
		refs[i] = *TeamRefOf(t)
	}
	value := WorkspaceUser{
		ID: ID(u.ID), Name: u.Name, DisplayName: u.DisplayName, Email: u.Email, AvatarURL: u.AvatarURL,
		Timezone: u.Timezone, Role: m.Role, Active: m.Active, IsMe: ID(u.ID) == viewer, JoinedAt: Time(m.JoinedAt), Teams: refs,
	}
	if value.IsMe {
		value.EmailNotifications = &u.EmailNotifications
	}
	return value
}

// InvitationOf needs the inviter loaded; teams are the ones it names, by name.
func InvitationOf(i *ent.Invitation, teams []*ent.Team) Invitation {
	refs := make([]TeamRef, len(teams))
	for n, t := range teams {
		refs[n] = *TeamRefOf(t)
	}
	return Invitation{
		ID: ID(i.ID), Email: i.Email, Role: i.Role, Teams: refs, Inviter: UserRefOf(i.Edges.Inviter),
		CreatedAt: Time(i.CreatedAt), ExpiresAt: Time(i.ExpiresAt),
	}
}

// AuditEntryOf needs the actor loaded.
func AuditEntryOf(a *ent.AuditEntry) AuditEntry {
	var metadata M
	if len(a.Metadata) > 0 {
		_ = json.Unmarshal(a.Metadata, &metadata)
	}
	return AuditEntry{ID: int(a.ID), Type: a.Type, Actor: UserRefOf(a.Edges.Actor), Metadata: metadata, CreatedAt: Time(a.CreatedAt)}
}
