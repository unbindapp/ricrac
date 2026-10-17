package initiatives_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	initiatives := srv.InitiativeService
	oapi.Register(reg, oapi.Operation{
		Name: "list_initiatives", Access: auth.Read, Routes: []string{"GET /initiatives"},
		Description: "List initiatives, most recently updated first, each with its project count and progress (the average of " +
			"its projects'). Narrow with status (planned, active, completed), owner, parent or query. Archived ones are " +
			"left out unless includeArchived is set.",
	}, initiatives.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_initiative", Access: auth.Read, Routes: []string{"GET /initiatives/{id}"},
		Description: "Read an initiative by id, slug, link or name: its fields and description, projects (with status, " +
			"progress, health and lead), sub-initiatives, latest status update, links and documents.",
	}, initiatives.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_initiative", Access: auth.Write, Routes: []string{"POST /initiatives", "PATCH /initiatives/{id}"},
		Description: "Create an initiative (omit id; name is required) or update one (pass id). Omitted fields stay as they are; " +
			"null clears a field. Add projects with addProjects or setProjects; nest it under another initiative with " +
			"parent.",
	}, initiatives.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_initiative", Access: auth.Write, Routes: []string{"DELETE /initiatives/{id}"},
		Description: "Move an initiative to the trash; its projects stay. It is deleted for good after 30 days; " +
			"restore_initiative brings it back before then.",
	}, initiatives.Trash)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_initiative", Access: auth.Write, Routes: []string{"POST /initiatives/{id}/restore"},
		Description: "Bring an initiative back from the trash.",
	}, initiatives.Restore)
	oapi.Register(reg, oapi.Operation{
		Name: "archive_initiative", Access: auth.Write, Routes: []string{"POST /initiatives/{id}/archive"},
		Description: "Archive an initiative. It stays readable but is left out of lists unless includeArchived is set.",
	}, initiatives.Archive)
	oapi.Register(reg, oapi.Operation{
		Name: "unarchive_initiative", Access: auth.Write, Routes: []string{"POST /initiatives/{id}/unarchive"},
		Description: "Bring an archived initiative back.",
	}, initiatives.Unarchive)
}
