package models

import (
	"github.com/google/uuid"

	"ricrac/ent"
)

type Board struct {
	ID          string        `json:"id"`
	SlugID      string        `json:"slugId"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	Columns     []BoardColumn `json:"columns" doc:"In board order."`
	Creator     *UserRef      `json:"creator"`
	URL         string        `json:"url"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
}

type BoardColumn struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Color      *string `json:"color"`
	IssueCount int     `json:"issueCount"`
}

type BoardRef struct {
	ID     string `json:"id"`
	SlugID string `json:"slugId"`
	Name   string `json:"name"`
}

type BoardColumnRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BoardCard struct {
	Board     BoardRef       `json:"board"`
	Column    BoardColumnRef `json:"column"`
	SortOrder float64        `json:"sortOrder" doc:"Order in the column, lowest first."`
	Issue     Issue          `json:"issue"`
}

type DeletedBoard struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}

type RemovedFromBoard struct {
	Removed bool     `json:"removed" doc:"false when the issue wasn't on the board."`
	Board   BoardRef `json:"board"`
	Issue   IssueRef `json:"issue"`
}

// BoardOf needs the board's workspace, creator and columns loaded; counts are issues per column.
func BoardOf(b *ent.Board, counts map[uuid.UUID]int, publicURL string) Board {
	columns := make([]BoardColumn, len(b.Edges.Columns))
	for i, c := range b.Edges.Columns {
		columns[i] = BoardColumn{ID: ID(c.ID), Name: c.Name, Color: c.Color, IssueCount: counts[c.ID]}
	}
	return Board{
		ID: ID(b.ID), SlugID: b.SlugID, Name: b.Name, Description: b.Description, Columns: columns,
		Creator: UserRefOf(b.Edges.Creator), URL: sluggedURL(publicURL, b.Edges.Workspace.URLKey, "board", b.Name, b.SlugID),
		CreatedAt: Time(b.CreatedAt), UpdatedAt: Time(b.UpdatedAt),
	}
}

func BoardRefOf(b *ent.Board) BoardRef {
	return BoardRef{ID: ID(b.ID), SlugID: b.SlugID, Name: b.Name}
}

func BoardColumnRefOf(c *ent.BoardColumn) BoardColumnRef {
	return BoardColumnRef{ID: ID(c.ID), Name: c.Name}
}
