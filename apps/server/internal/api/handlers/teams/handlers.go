package teams_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	teams := srv.TeamService
	oapi.Register(reg, oapi.Operation{
		Name: "list_teams", Access: auth.Read, Routes: []string{"GET /teams"},
		Description: "List the workspace's team (a workspace has one) with key, name, whether you're a member, " +
			"member and open issue counts and whether cycles and triage are on. Narrow with query (name or key) or mine.",
	}, teams.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_team", Access: auth.Read, Routes: []string{"GET /teams/{id}"},
		Description: "Read the workspace's team by key, name or id (earlier keys and names, and those of teams merged into it, " +
			"work too): the issue prefix (key), all its settings (estimates, cycles, triage, auto-archive and auto-close), " +
			"its statuses in order, members (with owners), current cycle, default status and who is on triage.",
	}, teams.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_team", Access: auth.Write, Routes: []string{"POST /teams", "PATCH /teams/{id}"},
		Description: "Change the workspace's settings (id = its key, name or id): the issue prefix (key), name, estimates, " +
			"cycles, triage, auto-archive, auto-close, default status and GitHub statuses. A workspace has one team, " +
			"so creating another fails. Omitted fields stay as they are; null clears a nullable field. People are id, " +
			"name, email or \"me\"; statuses are name, type or id. Changing the key renames every issue (ENG-1 becomes " +
			"NEW-1); old identifiers and the old key keep working. addMembers and removeMembers manage which guests " +
			"see the issues; everyone else is a member. Guests can't change it.",
	}, teams.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_team", Access: auth.Write, Routes: []string{"DELETE /teams/{id}"},
		Description: "Archive a team. A workspace has one team, which holds its issues, statuses and labels, so this fails " +
			"for it; delete the workspace instead.",
	}, teams.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "join_team", Access: auth.Write, Routes: []string{"POST /teams/{id}/join"},
		Description: "Join the workspace's team yourself. Everyone but guests is already a member, so this changes nothing for " +
			"them; guests need a member to add them (save_team addMembers).",
	}, teams.Join)
	oapi.Register(reg, oapi.Operation{
		Name: "leave_team", Access: auth.Write, Routes: []string{"POST /teams/{id}/leave"},
		Description: "Leave a team. Everyone in the workspace is on its team, so this fails for it; leave the workspace instead.",
	}, teams.Leave)
	oapi.Register(reg, oapi.Operation{
		Name: "get_triage_responsibility", Access: auth.Read, Routes: []string{"GET /teams/{id}/triage"},
		Description: "Who handles triage: the action (none, notify or assign), who is responsible now and, with a " +
			"rotation, the upcoming shifts (filter them with user).",
	}, teams.TriageResponsibility)
}
