package milestones_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	milestones := srv.MilestoneService
	oapi.Register(reg, oapi.Operation{
		Name: "list_milestones", Access: auth.Read, Routes: []string{"GET /projects/{project}/milestones", "GET /milestones"},
		Description: "List a project's milestones in order, or without project the workspace's own (in no project, open ones " +
			"first; those of synced GitHub repositories among them), each with its issue counts and progress (completed issues / " +
			"(all - canceled)).",
	}, milestones.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_milestone", Access: auth.Read, Routes: []string{"GET /milestones/{id}"},
		Description: "Read a milestone by id or name (with its project for one of a project's), with its progress and its issues.",
	}, milestones.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_milestone", Access: auth.Write, Routes: []string{"POST /milestones", "PATCH /milestones/{id}"},
		Description: "Create a milestone (omit id; name is required) in a project, or without project one of the workspace's " +
			"own, or change one (pass its id or name). Names are unique within a project and among the workspace's own. Put " +
			"issues in it with save_issue (milestone). Milestones on issues of synced GitHub repositories are GitHub's " +
			"milestones too: changes, closing and deleting go both ways.",
	}, milestones.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_milestone", Access: auth.Write, Routes: []string{"DELETE /milestones/{id}"},
		Description: "Delete a milestone, on GitHub too when it is synced there. Its issues stay, without a milestone.",
	}, milestones.Delete)
}
