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
		Description: "List the teams you can see (private teams only if you're in them) with key, name, whether you're a member, " +
			"member and open issue counts and whether cycles and triage are on. Narrow with query (name or key) or mine.",
	}, teams.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_team", Access: auth.Read, Routes: []string{"GET /teams/{id}"},
		Description: "Read a team by key, name or id: all its settings (estimates, cycles, triage, auto-archive and auto-close), " +
			"its statuses in order, members (with owners), sub-teams, current cycle, default status and who is on triage.",
	}, teams.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_team", Access: auth.Write, Routes: []string{"POST /teams", "PATCH /teams/{id}"},
		Description: "Create a team (omit id; name is required, key defaults to its initials) or change one (id = key, name or " +
			"id). Omitted fields stay as they are; null clears a nullable field. People are id, name, email or \"me\"; " +
			"statuses are name, type or id. Changing the key renames every issue (ENG-1 becomes NEW-1); old " +
			"identifiers keep working. Workspace admins, team owners and members of the team (or anyone, for a public " +
			"team) may change it; guests can't.",
	}, teams.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_team", Access: auth.Write, Routes: []string{"DELETE /teams/{id}"},
		Description: "Archive a team (workspace admins and team owners). It disappears from lists and its key becomes free; its " +
			"issues stay readable.",
	}, teams.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "join_team", Access: auth.Write, Routes: []string{"POST /teams/{id}/join"},
		Description: "Join a team yourself. Private teams can only be joined by workspace admins; others need a member to add them " +
			"(save_team addMembers).",
	}, teams.Join)
	oapi.Register(reg, oapi.Operation{
		Name: "leave_team", Access: auth.Write, Routes: []string{"POST /teams/{id}/leave"},
		Description: "Leave a team. The last member of a private team can't leave it.",
	}, teams.Leave)
	oapi.Register(reg, oapi.Operation{
		Name: "get_triage_responsibility", Access: auth.Read, Routes: []string{"GET /teams/{id}/triage"},
		Description: "Who handles the team's triage: the action (none, notify or assign), who is responsible now and, with a " +
			"rotation, the upcoming shifts (filter them with user).",
	}, teams.TriageResponsibility)
}
