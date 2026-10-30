package workspace_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	workspaces := srv.WorkspaceService
	oapi.RegisterAccount(reg, oapi.Operation{
		Name: "list_workspaces", Access: auth.Read, Routes: []string{"GET /workspaces"},
		Description: "List the workspaces this token reaches: name, URL key, link and your role. When there are several, pass " +
			"the URL key as `workspace` to the other operations.",
	}, workspaces.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_workspace", Access: auth.Read, Public: true, Routes: []string{"GET /workspace"},
		Description: "The workspace: its name, URL key and link, your role, its team (whose key is the issue prefix, ENG in " +
			"ENG-123) and how many members and issues it has.",
	}, workspaces.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "update_workspace", Access: auth.AdminAccess, Routes: []string{"PATCH /workspace"},
		Description: "Rename the workspace, change its URL key (2 to 32 lowercase letters, digits and dashes; links with the old " +
			"key stop working) or its logo. Admins only. Omitted fields stay as they are; null clears the logo.",
	}, workspaces.Update)
	oapi.Register(reg, oapi.Operation{
		Name: "list_users", Access: auth.Read, Routes: []string{"GET /users"},
		Description: "List the workspace's members with their role, teams and whether they are you. Narrow with query (name, " +
			"display name or email) or team; removed members are left out unless includeInactive is set.",
	}, workspaces.ListUsers)
	oapi.Register(reg, oapi.Operation{
		Name: "get_user", Access: auth.Read, Routes: []string{"GET /users/{id}"},
		Description: "Read a member by id, name, display name, email or \"me\": role, teams, timezone and how many open issues " +
			"are assigned to them. Removed members can be read by id.",
	}, workspaces.GetUser)
	oapi.Register(reg, oapi.Operation{
		Name: "update_user", Access: auth.Write, Routes: []string{"PATCH /users/{id}"},
		Description: "Change a member. Anyone may change their own name, displayName, timezone, emailNotifications and theme. Changing a role (owner, admin, " +
			"member, guest) needs an admin; only owners grant or take away owner, and the last owner stays one. Omitted " +
			"fields stay as they are; null clears the timezone.",
	}, workspaces.UpdateUser)
	oapi.Register(reg, oapi.Operation{
		Name: "remove_user", Access: auth.AdminAccess, Routes: []string{"DELETE /users/{id}"},
		Description: "Remove a member from the workspace (admins only): they lose access and leave its team; their issues, " +
			"comments and history stay. The last owner can't be removed. Invite them again to bring them back.",
	}, workspaces.RemoveUser)
	oapi.Register(reg, oapi.Operation{
		Name: "list_invitations", Access: auth.Read, Routes: []string{"GET /invitations"},
		Description: "List pending invitations to the workspace: email, role, teams, who invited and when they expire.",
	}, workspaces.ListInvitations)
	oapi.Register(reg, oapi.Operation{
		Name: "invite_user", Access: auth.Write, Routes: []string{"POST /invitations"},
		Description: "Invite someone by email. role is member (default), admin (only admins invite admins) or guest. Everyone " +
			"but guests joins the workspace's team; guests see only the issues of the teams in teams (optional, the " +
			"workspace's team by default) and those shared with them. The " +
			"invitation expires in 14 days; inviting the same email again renews it. They accept it in the dashboard.",
	}, workspaces.Invite)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_invitation", Access: auth.Write, Routes: []string{"DELETE /invitations/{id}"},
		Description: "Withdraw a pending invitation by id. Admins can withdraw any; members the ones they sent.",
	}, workspaces.DeleteInvitation)
	oapi.Register(reg, oapi.Operation{
		Name: "get_audit_log", Access: auth.AdminAccess, Routes: []string{"GET /audit-log"},
		Description: "The workspace's audit log, newest first (admins only): role changes, removed members, invitations, " +
			"archived teams and workspace changes, with who did it. Filter with type, e.g. member.roleChanged.",
	}, workspaces.AuditLog)
}
