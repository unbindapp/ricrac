package boards_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	boards := srv.BoardService
	oapi.Register(reg, oapi.Operation{
		Name: "list_boards", Access: auth.Read, Routes: []string{"GET /boards"},
		Description: "List the workspace's boards: kanban boards with columns of their own (like Ideas, Design, Ready), " +
			"apart from the workflow statuses, with how many issues and discussions each column holds.",
	}, boards.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_board", Access: auth.Read, Routes: []string{"GET /boards/{id}"},
		Description: "Read a board by id, slug, link or name: its columns in order. list_board_issues and list_board_discussions list its cards.",
	}, boards.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_board", Access: auth.Write, Routes: []string{"POST /boards", "PATCH /boards/{id}"},
		Description: "Create a board (omit id; name is required) or update one (pass id). columns replaces all its " +
			"columns, in order: pass a column's id to rename it, its name to keep it.",
	}, boards.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_board", Access: auth.Write, Routes: []string{"DELETE /boards/{id}"},
		Description: "Delete a board. Its issues and discussions stay; only their places on the board go.",
	}, boards.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "list_board_issues", Access: auth.Read, Routes: []string{"GET /boards/{board}/issues"},
		Description: "List the issues on a board, column by column in their order, each with its column and sortOrder. " +
			"Trashed and archived issues are left out.",
	}, boards.ListIssues)
	oapi.Register(reg, oapi.Operation{
		Name: "place_on_board", Access: auth.Write, Routes: []string{"PUT /boards/{board}/issues/{issue}"},
		Description: "Put an issue on a board in a column, or move it to another column or place. An issue is on a board " +
			"at most once. To add an idea, create it with save_issue (githubRepository \"none\" keeps it off GitHub) " +
			"and place it.",
	}, boards.Place)
	oapi.Register(reg, oapi.Operation{
		Name: "remove_from_board", Access: auth.Write, Routes: []string{"DELETE /boards/{board}/issues/{issue}"},
		Description: "Take an issue off a board. The issue stays as it is.",
	}, boards.Remove)
	oapi.Register(reg, oapi.Operation{
		Name: "list_board_discussions", Access: auth.Read, Routes: []string{"GET /boards/{board}/discussions"},
		Description: "List the forum discussions on a board, column by column in their order, each with its column and " +
			"sortOrder, which orders them among the column's issues too. Trashed discussions are left out.",
	}, boards.ListDiscussions)
	oapi.Register(reg, oapi.Operation{
		Name: "place_discussion_on_board", Access: auth.Write, Routes: []string{"PUT /boards/{board}/discussions/{discussion}"},
		Description: "Put a forum discussion on a board in a column, next to its issues, or move it to another column or " +
			"place. A discussion is on a board at most once.",
	}, boards.PlaceDiscussion)
	oapi.Register(reg, oapi.Operation{
		Name: "remove_discussion_from_board", Access: auth.Write, Routes: []string{"DELETE /boards/{board}/discussions/{discussion}"},
		Description: "Take a forum discussion off a board. The discussion stays as it is.",
	}, boards.RemoveDiscussion)
}
