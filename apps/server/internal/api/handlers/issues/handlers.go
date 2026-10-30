package issues_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	issues := srv.IssueService
	oapi.Register(reg, oapi.Operation{
		Name: "list_issues", Access: auth.Read, Public: true, Routes: []string{"GET /issues"},
		Description: "List issues, newest update first. Narrow with the simple arguments (team, assignee \"me\", state, label, " +
			"project, cycle, ...) and/or a `filter` object for anything more (and/or, comparators, relations). Archived " +
			"and trashed issues are left out unless asked for. Descriptions are left out unless includeDescription is set.",
	}, issues.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_issue", Access: auth.Read, Public: true, Routes: []string{"GET /issues/{id}"},
		Description: "Read an issue by identifier (ENG-123), id or link: its fields, description, sub-issues, relations " +
			"(blocks, blocked by, related, duplicates), attachments, subscribers and releases. Set includeComments or " +
			"includeHistory to also get its comment threads or change history.",
	}, issues.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_issue", Access: auth.Write, Routes: []string{"POST /issues", "PATCH /issues/{id}"},
		Description: "Create an issue (omit id; title is required unless a template provides it; team is optional) or update " +
			"one (pass id). Omitted fields stay as they are; null clears a field. People are given as id, name, email or " +
			"\"me\"; states as name, type or id; labels, projects, cycles and milestones as name or id. Use patch to edit " +
			"part of the description without resending it. Earlier identifiers of an issue keep working. Returns the issue as get_issue does, with relations and sub-issues. " +
			"When the user wants to review an issue before it is saved, pass preview: true first: it answers the issue as it would be, " +
			"and where and as whom it would open on GitHub, without saving anything; show it, then save again without preview once they confirm.",
	}, issues.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "update_issues", Access: auth.Write, Routes: []string{"PATCH /issues"},
		Description: "Apply the same change to several issues at once, e.g. move them to a state, assign them, add a label or put " +
			"them in a cycle. `changes` takes the fields of save_issue (without id). Stops at the first issue that fails; " +
			"issues before it keep their change.",
	}, issues.UpdateMany)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_issue", Access: auth.Write, Routes: []string{"DELETE /issues/{id}"},
		Description: "Move an issue to the trash. It disappears from lists and is deleted for good after 30 days; restore_issue " +
			"brings it back before then.",
	}, issues.Trash)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_issue", Access: auth.Write, Routes: []string{"POST /issues/{id}/restore"},
		Description: "Bring an issue back from the trash.",
	}, issues.Restore)
	oapi.Register(reg, oapi.Operation{
		Name: "archive_issue", Access: auth.Write, Routes: []string{"POST /issues/{id}/archive"},
		Description: "Archive an issue (and its sub-issues). Archived issues are kept and readable but left out of lists unless " +
			"includeArchived is set. Closed issues are archived automatically after the team's auto-archive period.",
	}, issues.Archive)
	oapi.Register(reg, oapi.Operation{
		Name: "unarchive_issue", Access: auth.Write, Routes: []string{"POST /issues/{id}/unarchive"},
		Description: "Bring an archived issue back.",
	}, issues.Unarchive)
	oapi.Register(reg, oapi.Operation{
		Name: "get_issue_history", Access: auth.Read, Public: true, Routes: []string{"GET /issues/{id}/history"},
		Description: "The change history of an issue, oldest first: who changed which field from what to what, and when.",
	}, issues.History)
	oapi.Register(reg, oapi.Operation{
		Name: "share_issue", Access: auth.Write, Routes: []string{"POST /issues/{id}/shares"},
		Description: "Give a workspace member who can't see the workspace's issues (a guest) access " +
			"to this one issue. Only when the user asks to share it.",
	}, issues.Share)
	oapi.Register(reg, oapi.Operation{
		Name: "unshare_issue", Access: auth.Write, Routes: []string{"DELETE /issues/{id}/shares/{user}"},
		Description: "Take back access to an issue given with share_issue.",
	}, issues.Unshare)
}

func RegisterImportHandlers(srv *server.Server, reg *oapi.Registry) {
	oapi.Register(reg, oapi.Operation{
		Name: "import_issues", Access: auth.Write, Routes: []string{"POST /issues/import"},
		Description: "Import issues from another tracker in one go (workspace admins). An issue can keep its number, so ENG-42 " +
			"stays ENG-42 (the number must be free; gaps are fine), along with its creator, dates, status, " +
			"labels, assignee, project, links, subscribers and comments with their authors and dates. Issues point at " +
			"each other (parent, relatedTo, blocks, blockedBy, duplicateOf) by identifier, within the import or already " +
			"in Ricrac. Pass the issues inline, or upload them as a JSON file with prepare_file_upload and pass its " +
			"fileId, which keeps long descriptions out of the conversation. All or nothing, and nobody is notified. " +
			"dryRun checks everything and reports without importing.",
	}, srv.IssueService.Import)
}
