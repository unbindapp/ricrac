package statusupdates_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	updates := srv.StatusUpdateService
	oapi.Register(reg, oapi.Operation{
		Name: "get_status_updates", Access: auth.Read, Routes: []string{"GET /status-updates"},
		Description: "Status updates, newest first: of one project or initiative, or, without either, the recent updates across " +
			"the projects you can see and the initiatives. Pass id to read one update.",
	}, updates.List)
	oapi.Register(reg, oapi.Operation{
		Name: "save_status_update", Access: auth.Write, Routes: []string{"POST /status-updates", "PATCH /status-updates/{id}"},
		Description: "Post a status update on a project or initiative (omit id; body is required) with its health: onTrack, " +
			"atRisk or offTrack (by default the previous update's, else onTrack). The newest update's health becomes " +
			"the project's or initiative's. Members and subscribers of the project (or the initiative's owner) are " +
			"notified, and so are people @mentioned. Pass id to edit your own update.",
	}, updates.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_status_update", Access: auth.Write, Routes: []string{"DELETE /status-updates/{id}"},
		Description: "Delete a status update (yours, or any as a workspace admin). The health goes back to the newest remaining " +
			"update's.",
	}, updates.Delete)
}
