package labels_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	labels := srv.LabelService
	oapi.Register(reg, oapi.Operation{
		Name: "list_issue_labels", Access: auth.Read, Routes: []string{"GET /labels"},
		Description: "List issue labels. With team: that team's labels and the workspace labels (the ones its issues can use); " +
			"without: workspace labels and the labels of every team you can see. Groups (isGroup) hold child labels; an " +
			"issue takes one child of a singleSelect group. Retired labels are left out unless includeRetired is set.",
	}, labels.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_issue_label", Access: auth.Read, Routes: []string{"GET /labels/{id}"},
		Description: "Read an issue label by id or name (Group/Label for a label in a group; pass team to pick a team's label).",
	}, labels.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_issue_label", Access: auth.Write, Routes: []string{"POST /labels", "PATCH /labels/{id}"},
		Description: "Create an issue label (omit id; name is required; with team it's that team's label, without it a workspace " +
			"label) or change one (id = id or name). Set isGroup to make a group that holds labels, and parent to put a " +
			"label in a group (null takes it out). Names are unique within their team (or the workspace) and group. " +
			"Omitted fields stay as they are; null clears a field.",
	}, labels.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "retire_issue_label", Access: auth.Write, Routes: []string{"POST /labels/{id}/retire"},
		Description: "Retire a label (and the labels of a group): it stays on the issues that have it but can't be added anymore. " +
			"restore_issue_label brings it back.",
	}, labels.Retire)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_issue_label", Access: auth.Write, Routes: []string{"POST /labels/{id}/restore"},
		Description: "Bring back a retired label (and the labels of a group).",
	}, labels.Restore)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_issue_label", Access: auth.Write, Routes: []string{"DELETE /labels/{id}"},
		Description: "Delete a label for good and take it off every issue; deleting a group deletes its labels too. Prefer " +
			"retire_issue_label to keep it on existing issues.",
	}, labels.Delete)
}
