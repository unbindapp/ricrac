package favorites_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	favorites := srv.FavoriteService
	oapi.Register(reg, oapi.Operation{
		Name: "list_favorites", Access: auth.Read, Routes: []string{"GET /favorites"},
		Description: "List the caller's favorites by folder and order, each with what it points at. Favorites of things that " +
			"were deleted or can no longer be seen are left out.",
	}, favorites.List)
	oapi.Register(reg, oapi.Operation{
		Name: "add_favorite", Access: auth.Write, Routes: []string{"POST /favorites"},
		Description: "Add something to the caller's favorites: an issue, project, initiative, cycle, customView, document, " +
			"label, team, user or release. Adding a favorite again only updates its folder or order.",
	}, favorites.Add)
	oapi.Register(reg, oapi.Operation{
		Name: "remove_favorite", Access: auth.Write, Routes: []string{"DELETE /favorites/{id}"},
		Description: "Remove one of the caller's favorites, by the favorite's id, or by type and target.",
	}, favorites.Remove)
}
