package comments_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	comments := srv.CommentService
	oapi.Register(reg, oapi.Operation{
		Name: "list_comments", Access: auth.Read, Routes: []string{"GET /comments"},
		Description: "List the comment threads on one issue, project, initiative, document, milestone or status update, oldest " +
			"first. Each thread is its first comment with its replies, reactions and whether it was resolved.",
	}, comments.List)
	oapi.Register(reg, oapi.Operation{
		Name: "save_comment", Access: auth.Write, Routes: []string{"POST /comments", "PATCH /comments/{id}"},
		Description: "Comment on an issue, project, initiative, document, milestone or status update (pass exactly one of them), " +
			"or reply to a comment with parentId. Pass id and body to edit your own comment. The body is Markdown; " +
			"mention people with @displayName or @email to notify them. Commenting on an issue subscribes you to it and " +
			"notifies its subscribers.",
	}, comments.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_comment", Access: auth.Write, Routes: []string{"DELETE /comments/{id}"},
		Description: "Delete a comment (your own, or any as a workspace admin). Deleting a thread's first comment deletes its replies.",
	}, comments.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "resolve_comment", Access: auth.Write, Routes: []string{"POST /comments/{id}/resolve"},
		Description: "Mark a comment thread as resolved. Pass the thread's first comment.",
	}, comments.Resolve)
	oapi.Register(reg, oapi.Operation{
		Name: "unresolve_comment", Access: auth.Write, Routes: []string{"POST /comments/{id}/unresolve"},
		Description: "Reopen a resolved comment thread.",
	}, comments.Unresolve)
	oapi.Register(reg, oapi.Operation{
		Name: "add_reaction", Access: auth.Write, Routes: []string{"POST /reactions"},
		Description: "React with an emoji to an issue, a comment or a status update (exactly one). The emoji is a unicode emoji " +
			"(👍) or a shortcode (+1, :tada:). Reacting twice with the same emoji changes nothing.",
	}, comments.AddReaction)
	oapi.Register(reg, oapi.Operation{
		Name: "remove_reaction", Access: auth.Write, Routes: []string{"DELETE /reactions/{id}", "POST /reactions/remove"},
		Description: "Take back one of your reactions: by its id, or by emoji and the issue, comment or status update.",
	}, comments.RemoveReaction)
}
