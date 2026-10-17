package attachments_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	attachments := srv.AttachmentService
	oapi.Register(reg, oapi.Operation{
		Name: "create_attachment", Access: auth.Write, Routes: []string{"POST /attachments"},
		Description: "Attach a link (pull request, design, doc, error report, ...) to an issue. Attaching a URL the issue already " +
			"has updates that attachment instead. The source (github, figma, slack, ...) is recognized from the URL.",
	}, attachments.Create)
	oapi.Register(reg, oapi.Operation{
		Name: "get_attachment", Access: auth.Read, Routes: []string{"GET /attachments/{id}"},
		Description: "Read an attachment by id.",
	}, attachments.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "list_attachments", Access: auth.Read, Routes: []string{"GET /issues/{issue}/attachments"},
		Description: "List an issue's attachments, oldest first.",
	}, attachments.List)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_attachment", Access: auth.Write, Routes: []string{"DELETE /attachments/{id}"},
		Description: "Remove an attachment from its issue.",
	}, attachments.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "prepare_file_upload", Access: auth.Write, Routes: []string{"POST /files"},
		Description: "Get a one-time URL to upload a file (screenshot, log, recording, ...) to, valid for 15 minutes. Upload it " +
			"by running the returned `curl` command with <file> replaced by the local path (an HTTP PUT of the raw bytes). " +
			"Then embed the assetUrl in Markdown (`![name](assetUrl)` for images, `[name](assetUrl)` otherwise) in a " +
			"description or comment, or attach it to an issue with create_attachment_from_upload. Agents that can't run " +
			"commands use upload_file instead.",
	}, attachments.PrepareFileUpload)
	oapi.Register(reg, oapi.Operation{
		Name: "upload_file", Access: auth.Write, Routes: []string{"POST /files/inline"},
		Description: "Upload a small file (up to 5 MB) as base64 in the arguments, for agents that can't run commands; otherwise " +
			"prefer prepare_file_upload. Embed the returned assetUrl in Markdown (`![name](assetUrl)`) or attach it to an " +
			"issue with create_attachment_from_upload.",
	}, attachments.UploadFile)
	oapi.Register(reg, oapi.Operation{
		Name: "create_attachment_from_upload", Access: auth.Write, Routes: []string{"POST /attachments/from-upload"},
		Description: "Attach an uploaded file (from prepare_file_upload or upload_file) to an issue.",
	}, attachments.CreateFromUpload)
}
