package projects_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	projects := srv.ProjectService
	oapi.Register(reg, oapi.Operation{
		Name: "list_projects", Access: auth.Read, Routes: []string{"GET /projects"},
		Description: "List projects in teams you can see, most recently updated first. Narrow with team, status (name or type: " +
			"backlog, planned, started, paused, completed, canceled), lead (\"me\"), member, initiative, label, health, " +
			"priority, query, or a saved project view (customView). Archived and trashed projects are left out unless " +
			"asked for; descriptions unless includeDescription is set. Each project has its progress: completed issues " +
			"/ (all - canceled).",
	}, projects.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_project", Access: auth.Read, Routes: []string{"GET /projects/{id}"},
		Description: "Read a project by id, slug, link or name: its fields and description, members, milestones with their " +
			"progress, latest status update, links, dependencies (blocks, blocked by, related), documents, issue counts " +
			"by status type and estimate sums.",
	}, projects.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_project", Access: auth.Write, Routes: []string{"POST /projects", "PATCH /projects/{id}"},
		Description: "Create a project (omit id; name and at least one team via setTeams or addTeams are required) or update one " +
			"(pass id). Omitted fields stay as they are; null clears a field. People are given as id, name, email or " +
			"\"me\"; the status (state) as name, type or id. Use patch to edit part of the description. A project template " +
			"(create only) adds its milestones and issues. A new lead is notified; so are people @mentioned in the " +
			"description.",
	}, projects.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_project", Access: auth.Write, Routes: []string{"DELETE /projects/{id}"},
		Description: "Move a project to the trash. Its issues keep their project. It is deleted for good after 30 days; " +
			"restore_project brings it back before then.",
	}, projects.Trash)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_project", Access: auth.Write, Routes: []string{"POST /projects/{id}/restore"},
		Description: "Bring a project back from the trash.",
	}, projects.Restore)
	oapi.Register(reg, oapi.Operation{
		Name: "archive_project", Access: auth.Write, Routes: []string{"POST /projects/{id}/archive"},
		Description: "Archive a project. It stays readable but is left out of lists unless includeArchived is set.",
	}, projects.Archive)
	oapi.Register(reg, oapi.Operation{
		Name: "unarchive_project", Access: auth.Write, Routes: []string{"POST /projects/{id}/unarchive"},
		Description: "Bring an archived project back.",
	}, projects.Unarchive)
	oapi.Register(reg, oapi.Operation{
		Name: "list_project_statuses", Access: auth.Read, Routes: []string{"GET /project-statuses"},
		Description: "The workspace's project statuses in order, each with its type (backlog, planned, started, paused, " +
			"completed, canceled) and how many projects are in it.",
	}, projects.ListStatuses)
	oapi.Register(reg, oapi.Operation{
		Name: "save_project_status", Access: auth.Write, Routes: []string{"POST /project-statuses", "PATCH /project-statuses/{id}"},
		Description: "Create a project status (omit id; name and type are required) or change one (pass its id or name). " +
			"Workspace admins only.",
	}, projects.SaveStatus)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_project_status", Access: auth.Write, Routes: []string{"DELETE /project-statuses/{id}"},
		Description: "Delete a project status. Projects in it move to moveProjectsTo, which is required when there are any. " +
			"The workspace keeps at least one status. Workspace admins only.",
	}, projects.DeleteStatus)
	oapi.Register(reg, oapi.Operation{
		Name: "list_project_labels", Access: auth.Read, Routes: []string{"GET /project-labels"},
		Description: "The workspace's project labels and label groups, with how many projects use each. Retired labels are " +
			"left out unless includeRetired is set.",
	}, projects.ListLabels)
	oapi.Register(reg, oapi.Operation{
		Name: "save_project_label", Access: auth.Write, Routes: []string{"POST /project-labels", "PATCH /project-labels/{id}"},
		Description: "Create a project label (omit id; name is required) or change one (pass its id or name). A label can sit in " +
			"a group (parent, a label with isGroup); groups don't nest.",
	}, projects.SaveLabel)
	oapi.Register(reg, oapi.Operation{
		Name: "retire_project_label", Access: auth.Write, Routes: []string{"POST /project-labels/{id}/retire"},
		Description: "Retire a project label (and a group's labels): projects keep it, but it can't be added anymore.",
	}, projects.RetireLabel)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_project_label", Access: auth.Write, Routes: []string{"POST /project-labels/{id}/restore"},
		Description: "Bring a retired project label back.",
	}, projects.RestoreLabel)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_project_label", Access: auth.Write, Routes: []string{"DELETE /project-labels/{id}"},
		Description: "Delete a project label for good, removing it from every project.",
	}, projects.DeleteLabel)
}
