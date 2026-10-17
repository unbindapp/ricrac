package models

import (
	"encoding/json"

	"ricrac/ent"
	"ricrac/internal/common/args"
)

type Favorite struct {
	ID        string         `json:"id"`
	Type      string         `json:"type" enum:"issue,project,initiative,cycle,customView,document,label,team,user,release"`
	Folder    *string        `json:"folder"`
	SortOrder float64        `json:"sortOrder"`
	CreatedAt string         `json:"createdAt"`
	Target    FavoriteTarget `json:"target" doc:"What it points at, in the short form of its type."`
}

// FavoriteTarget is the short form of whatever a favorite points at.
type FavoriteTarget struct {
	Value any
}

func (t FavoriteTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Value)
}

func (FavoriteTarget) JSONSchema() M {
	return M{"anyOf": []any{
		args.ResultOf[IssueRef](), args.ResultOf[ProjectRef](), args.ResultOf[InitiativeRef](), args.ResultOf[FavoriteCycle](),
		args.ResultOf[FavoriteCustomView](), args.ResultOf[DocumentRef](), args.ResultOf[LabelRef](), args.ResultOf[TeamRef](),
		args.ResultOf[UserRef](), args.ResultOf[ReleaseRef](),
	}}
}

type FavoriteCycle struct {
	CycleRef
	Team TeamRef `json:"team"`
}

type FavoriteCustomView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type RemovedFavorite struct {
	Removed bool   `json:"removed"`
	ID      string `json:"id"`
}

// FavoriteTypeOf is the API's name of a stored favorite type.
func FavoriteTypeOf(stored string) string {
	if stored == "custom_view" {
		return "customView"
	}
	return stored
}

func FavoriteOf(f *ent.Favorite, target any) Favorite {
	return Favorite{
		ID: ID(f.ID), Type: FavoriteTypeOf(f.Type), Folder: f.Folder, SortOrder: f.SortOrder, CreatedAt: Time(f.CreatedAt),
		Target: FavoriteTarget{Value: target},
	}
}

// FavoriteCycleOf needs the cycle's team loaded.
func FavoriteCycleOf(c *ent.Cycle) FavoriteCycle {
	return FavoriteCycle{CycleRef: *CycleRefOf(c), Team: *TeamRefOf(c.Edges.Team)}
}

func FavoriteCustomViewOf(v *ent.CustomView) FavoriteCustomView {
	return FavoriteCustomView{ID: ID(v.ID), Name: v.Name, SlugID: v.SlugID}
}
