package templates_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	templates := srv.TemplateService
	oapi.Register(reg, oapi.Operation{
		Name: "list_templates", Access: auth.Read, Routes: []string{"GET /templates"},
		Description: "List the issue, project and document templates (without their data; read one with get_template). " +
			"save_issue, save_project and save_document take a template's name or id.",
	}, templates.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_template", Access: auth.Read, Routes: []string{"GET /templates/{id}"},
		Description: "Read a template by id or name (with type when names repeat across types), including the data it fills in.",
	}, templates.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_template", Access: auth.Write, Routes: []string{"POST /templates", "PATCH /templates/{id}"},
		Description: "Create a template (omit id; name, type and data are required) or update one (pass id). data holds what " +
			"it fills in, by type. issue: title, description, team, state, priority, estimate, assignee, labels, " +
			"project, cycle, dueDate and subIssues (a list of objects with save_issue's fields). project: name, " +
			"summary, description, state, priority, lead, labels, teams, milestones ([{name, description, " +
			"targetDate}]) and issues (save_issue objects). document: title, content. An issue template with a " +
			"recurrence creates an issue from itself on that schedule (recurring issues), as its creator.",
	}, templates.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_template", Access: auth.Write, Routes: []string{"DELETE /templates/{id}"},
		Description: "Delete a template. Things made from it keep what it filled in.",
	}, templates.Delete)
}
