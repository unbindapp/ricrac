package comment_repo

import (
	"context"
	"fmt"
	"slices"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/comment"
	"ricrac/ent/reaction"
	repository "ricrac/internal/repositories"
)

// Targets is the columns comments can be on.
var Targets = []string{"issue_id", "project_id", "initiative_id", "document_id", "milestone_id", "status_update_id"}

func oldestFirst(q *ent.CommentQuery) {
	q.Order(ent.Asc(comment.FieldCreatedAt), ent.Asc(comment.FieldID))
}

// WithAnswer loads what a comment's answer shows (see models.CommentOf).
func WithAnswer(q *ent.CommentQuery) *ent.CommentQuery {
	return q.WithUser().WithResolvingUser().
		WithReactions(func(q *ent.ReactionQuery) { q.WithUser().Order(ent.Asc(reaction.FieldCreatedAt)) })
}

// Threads is the comment threads on the thing whose id is in column (e.g. issue_id), oldest
// first, each with its replies; a page of them when limit is set.
func (self *CommentRepository) Threads(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID, limit *int64, offset int64) ([]*ent.Comment, error) {
	if !slices.Contains(Targets, column) {
		return nil, fmt.Errorf("comments can't be on %q", column)
	}
	query := WithAnswer(self.base.Client(tx).Comment.Query()).
		Where(comment.ParentIDIsNil(), func(s *entsql.Selector) { s.Where(entsql.EQ(s.C(column), id)) }).
		WithChildren(func(q *ent.CommentQuery) { oldestFirst(WithAnswer(q)) })
	oldestFirst(query)
	if limit != nil {
		query = query.Limit(int(*limit))
	}
	if offset > 0 {
		query = query.Offset(int(offset))
	}
	return query.All(ctx)
}
