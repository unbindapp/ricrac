package comment_service

import (
	"context"

	"ricrac/internal/auth"
	"ricrac/internal/models"
)

func (self *CommentService) History(ctx context.Context, actor *auth.Actor, input *IDInput) (models.MessageHistory, error) {
	row, err := self.Load(ctx, actor, input.ID)
	if err != nil {
		return models.MessageHistory{}, err
	}
	if row.DeletedAt != nil {
		return models.MessageHistory{}, errDeleted()
	}
	versions, err := self.repo.Comment().Versions(ctx, nil, row.ID)
	if err != nil {
		return models.MessageHistory{}, err
	}
	return models.MessageHistoryOf(versions, row.Body, row.EditedAt, row.CreatedAt), nil
}
