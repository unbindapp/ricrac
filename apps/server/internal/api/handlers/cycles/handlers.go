package cycles_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	cycles := srv.CycleService
	oapi.Register(reg, oapi.Operation{
		Name: "list_cycles", Access: auth.Read, Routes: []string{"GET /teams/{team}/cycles"},
		Description: "List the workspace's cycles with their progress (issue counts by state type, estimate sums, daily history). " +
			"type narrows to the current, previous or next cycle, the upcoming ones or the completed ones. Newest " +
			"first, except upcoming (soonest first). Archived cycles are left out unless includeArchived is set.",
	}, cycles.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_cycle", Access: auth.Read, Routes: []string{"GET /cycles/{id}"},
		Description: "Read a cycle by id, number, name or \"current\" / \"next\" / \"previous\": its " +
			"dates, progress, daily history and issues (unless includeIssues is false).",
	}, cycles.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_cycle", Access: auth.Write, Routes: []string{"POST /cycles", "PATCH /cycles/{id}"},
		Description: "Create a cycle (omit id; startsAt and endsAt are required and cycles must be enabled with save_team) " +
			"or update one's name, description or dates (pass id). Cycles can't overlap, and completed " +
			"cycles can't be moved. With cycles enabled they are created automatically; only create " +
			"one by hand when the user asks.",
	}, cycles.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "archive_cycle", Access: auth.Write, Routes: []string{"POST /cycles/{id}/archive"},
		Description: "Archive an upcoming or completed cycle (not the current one). Issues of an archived upcoming cycle leave it.",
	}, cycles.Archive)
}
