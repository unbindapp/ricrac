package documents_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	documents := srv.DocumentService
	oapi.Register(reg, oapi.Operation{
		Name: "list_documents", Access: auth.Read, Routes: []string{"GET /documents"},
		Description: "List documents, most recently updated first. Narrow to those of a project, initiative, team, issue, " +
			"cycle or release, by creator, or by words in the title or content. Contents are left out unless " +
			"includeContent is set; read one with get_document.",
	}, documents.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_document", Access: auth.Read, Routes: []string{"GET /documents/{id}"},
		Description: "Read a document by id, slug, link or title: its Markdown content, where it belongs and how many earlier " +
			"versions are kept.",
	}, documents.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_document", Access: auth.Write, Routes: []string{"POST /documents", "PATCH /documents/{id}"},
		Description: "Create a document (omit id; title and exactly one of project, initiative, team, issue, cycle or release " +
			"are required, unless a template gives the title) or update one (pass id). Use patch to edit part of the " +
			"content without resending it. Passing a parent on update moves the document there. People mentioned " +
			"with @displayName or @email are notified. The previous content is kept as a version.",
	}, documents.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_document", Access: auth.Write, Routes: []string{"DELETE /documents/{id}"},
		Description: "Move a document to the trash. It is deleted for good after 30 days; restore_document brings it back.",
	}, documents.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_document", Access: auth.Write, Routes: []string{"POST /documents/{id}/restore"},
		Description: "Bring a document back from the trash.",
	}, documents.Restore)
	oapi.Register(reg, oapi.Operation{
		Name: "list_document_versions", Access: auth.Read, Routes: []string{"GET /documents/{id}/versions"},
		Description: "The earlier versions of a document, newest first (without their content; read one with " +
			"get_document_version).",
	}, documents.ListVersions)
	oapi.Register(reg, oapi.Operation{
		Name: "get_document_version", Access: auth.Read, Routes: []string{"GET /document-versions/{id}"},
		Description: "Read an earlier version of a document: its title and content as they were.",
	}, documents.GetVersion)
	oapi.Register(reg, oapi.Operation{
		Name: "accept_suggestion", Access: auth.Write, Routes: []string{"POST /suggestions/{id}/accept"},
		Description: "Accept a suggested edit on a document (a comment with suggestedText): its text replaces the passage it " +
			"quotes, the previous content is kept as a version, and the suggestion's thread is resolved.",
	}, documents.AcceptSuggestion)
	oapi.Register(reg, oapi.Operation{
		Name: "reject_suggestion", Access: auth.Write, Routes: []string{"POST /suggestions/{id}/reject"},
		Description: "Reject a suggested edit on a document, leaving the document as it is and resolving the suggestion's thread.",
	}, documents.RejectSuggestion)
}
