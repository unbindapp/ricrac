package milestones_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	milestones := srv.MilestoneService
	oapi.Register(reg, oapi.Operation{
		Name: "list_milestones", Access: auth.Read, Routes: []string{"GET /projects/{project}/milestones"},
		Description: "List a project's milestones in order, each with its issue counts and progress (completed issues / (all - " +
			"canceled)).",
	}, milestones.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_milestone", Access: auth.Read, Routes: []string{"GET /milestones/{id}"},
		Description: "Read a milestone by id (or by name with its project), with its progress and its issues.",
	}, milestones.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_milestone", Access: auth.Write, Routes: []string{"POST /milestones", "PATCH /milestones/{id}"},
		Description: "Create a milestone in a project (omit id; project and name are required) or change one (pass its id, or " +
			"its name with the project). Names are unique within a project. Put issues in it with save_issue " +
			"(milestone).",
	}, milestones.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_milestone", Access: auth.Write, Routes: []string{"DELETE /milestones/{id}"},
		Description: "Delete a milestone. Its issues stay in the project, without a milestone.",
	}, milestones.Delete)
}
