package states_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	states := srv.StateService
	oapi.Register(reg, oapi.Operation{
		Name: "list_issue_statuses", Access: auth.Read, Routes: []string{"GET /teams/{team}/statuses"},
		Description: "List a team's workflow statuses in board order (triage, backlog, unstarted, started, completed, canceled, " +
			"then position): name, type, color, how many issues are in each and which is the default for new issues.",
	}, states.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_issue_status", Access: auth.Read, Routes: []string{"GET /statuses/{id}"},
		Description: "Read a workflow status by id, or by name or type together with its team.",
	}, states.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_issue_status", Access: auth.Write, Routes: []string{"POST /statuses", "PATCH /statuses/{id}"},
		Description: "Create a workflow status (omit id; team, name and type are required) or change one (id, or name with team). " +
			"Types: triage, backlog, unstarted, started, completed, canceled. Names are unique within the team. The type " +
			"can only change while no issues are in the status. Omitted fields stay as they are; null clears the " +
			"description.",
	}, states.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_issue_status", Access: auth.Write, Routes: []string{"DELETE /statuses/{id}"},
		Description: "Archive a workflow status. If issues are in it, pass moveIssuesTo (a status of the same team) and they move " +
			"there. If it is the team's default or auto-close status, that setting moves to moveIssuesTo too (which then " +
			"must be backlog/unstarted, or canceled). A team keeps at least one open, one completed and one canceled " +
			"status; turn triage off with save_team rather than deleting the Triage status.",
	}, states.Delete)
}
