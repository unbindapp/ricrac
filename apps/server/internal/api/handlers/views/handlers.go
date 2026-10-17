package views_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	views := srv.ViewService
	oapi.Register(reg, oapi.Operation{
		Name: "list_custom_views", Access: auth.Read, Routes: []string{"GET /views"},
		Description: "List the custom views the caller can use: those shared with the workspace and their own. Issue views " +
			"apply to list_issues with customView.",
	}, views.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_custom_view", Access: auth.Read, Routes: []string{"GET /views/{id}"},
		Description: "Read a custom view by id, slug, link or name: its filter and display settings.",
	}, views.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_custom_view", Access: auth.Write, Routes: []string{"POST /views", "PATCH /views/{id}"},
		Description: "Create a custom view (omit id; name and filter are required) or update one (pass id). Issue views take " +
			"the filter of list_issues, which is checked when saving. Views are personal unless shared; only their " +
			"owner (or an admin, for shared views) can change them.",
	}, views.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_custom_view", Access: auth.Write, Routes: []string{"DELETE /views/{id}"},
		Description: "Delete a custom view. Only its owner (or an admin, for a shared view) can.",
	}, views.Delete)
}
