package models

import (
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
)

type SearchResults struct {
	Nodes []SearchHit `json:"nodes"`
}

// SearchHit is one of the kinds of search results, told apart by type.
type SearchHit struct {
	value any
}

func SearchHitOf(value any) SearchHit {
	return SearchHit{value: value}
}

func (h SearchHit) MarshalJSON() ([]byte, error) {
	return errdefs.Marshal(h.value), nil
}

func (SearchHit) JSONSchema() M {
	return M{"anyOf": []any{
		args.ResultOf[IssueSearchHit](), args.ResultOf[ProjectSearchHit](), args.ResultOf[DocumentSearchHit](),
		args.ResultOf[InitiativeSearchHit](), args.ResultOf[DiscussionSearchHit](), args.ResultOf[CommentSearchHit](),
	}}
}

// SearchHitBase is what every kind of search result has.
type SearchHitBase struct {
	ID        string  `json:"id"`
	Title     string  `json:"title" doc:"For comments, the issue's."`
	URL       string  `json:"url"`
	Snippet   string  `json:"snippet" doc:"The matching text, with matches in **bold**."`
	Rank      float64 `json:"rank"`
	UpdatedAt string  `json:"updatedAt"`
}

type IssueSearchHit struct {
	Type string `json:"type" enum:"issue"`
	SearchHitBase
	Identifier string   `json:"identifier"`
	State      StateRef `json:"state"`
	Team       TeamRef  `json:"team"`
}

type ProjectSearchHit struct {
	Type string `json:"type" enum:"project"`
	SearchHitBase
	Status ProjectStatusRef `json:"status"`
}

type DocumentSearchHit struct {
	Type string `json:"type" enum:"document"`
	SearchHitBase
	Project *ProjectRef `json:"project"`
}

type InitiativeSearchHit struct {
	Type string `json:"type" enum:"initiative"`
	SearchHitBase
	Status string `json:"status" enum:"planned,active,completed"`
}

type DiscussionSearchHit struct {
	Type string `json:"type" enum:"discussion"`
	SearchHitBase
}

type CommentSearchHit struct {
	Type string `json:"type" enum:"comment"`
	SearchHitBase
	Identifier string   `json:"identifier" doc:"The commented issue's."`
	Issue      IssueRef `json:"issue"`
	User       *UserRef `json:"user"`
}
