package discussions_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	discussions := srv.DiscussionService
	oapi.Register(reg, oapi.Operation{
		Name: "list_discussions", Access: auth.Read, Routes: []string{"GET /discussions"},
		Description: "List the forum's discussions: pinned ones first, then the most recently active. Open ones by default; " +
			"narrow by team, starter or words in the title or body. Each has its thread and comment counts; bodies are " +
			"left out unless includeBody is set.",
	}, discussions.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_discussion", Access: auth.Read, Routes: []string{"GET /discussions/{id}"},
		Description: "Read a discussion by id, slug, link or title: its Markdown body, its threads (bug, design, question, " +
			"idea, task or general; open or resolved) and its subscribers. Read its comments with list_comments " +
			"(discussionId) and a thread's with get_discussion_thread.",
	}, discussions.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_discussion", Access: auth.Write, Routes: []string{"POST /discussions", "PATCH /discussions/{id}"},
		Description: "Start a discussion in the forum (omit id; title is required) or update one (pass id). Use patch to edit " +
			"part of the body. Pass team to keep it to a team's members. Pin, close, subscribe and unsubscribe people. " +
			"Starting, commenting and being mentioned subscribe you; subscribers hear about new threads and comments. " +
			"Only the author or an admin changes the title, body or team.",
	}, discussions.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_discussion", Access: auth.Write, Routes: []string{"DELETE /discussions/{id}"},
		Description: "Move a discussion to the trash (your own, or any as an admin). It is deleted for good after 30 days; " +
			"restore_discussion brings it back.",
	}, discussions.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "restore_discussion", Access: auth.Write, Routes: []string{"POST /discussions/{id}/restore"},
		Description: "Bring a discussion back from the trash.",
	}, discussions.Restore)
	oapi.Register(reg, oapi.Operation{
		Name: "get_discussion_thread", Access: auth.Read, Routes: []string{"GET /discussion-threads/{id}"},
		Description: "Read a thread of a discussion with its comments, oldest first, each with its replies.",
	}, discussions.GetThread)
	oapi.Register(reg, oapi.Operation{
		Name: "save_discussion_thread", Access: auth.Write, Routes: []string{"POST /discussion-threads", "PATCH /discussion-threads/{id}"},
		Description: "Start a smaller thread inside a discussion for one bug, design check, question, idea or task (omit id; " +
			"discussion and title are required) or update one (pass id): change its kind, link the issue tracking it, " +
			"resolve or reopen it. The discussion's subscribers hear about new threads; a thread's author hears when it " +
			"is resolved. Comment on a thread with save_comment (discussionThreadId).",
	}, discussions.SaveThread)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_discussion_thread", Access: auth.Write, Routes: []string{"DELETE /discussion-threads/{id}"},
		Description: "Delete a thread and its comments (your own, or any as an admin).",
	}, discussions.DeleteThread)
}
