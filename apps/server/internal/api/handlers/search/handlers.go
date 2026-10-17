package search_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	oapi.Register(reg, oapi.Operation{
		Name: "search", Access: auth.Read, Routes: []string{"GET /search"},
		Description: "Search issues, projects, documents and initiatives (and comments, when asked for) by words, best matches " +
			"first, titles before text. An issue identifier like ENG-12 finds that issue. Each result has its type, title, " +
			"link and a snippet of the matching text with matches in **bold**.",
	}, srv.SearchService.Search)
}
