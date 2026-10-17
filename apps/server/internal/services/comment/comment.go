// Package comment_service is comments on issues, projects, initiatives, documents, milestones and
// status updates, their threads and reactions.
package comment_service

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	comment_repo "ricrac/internal/repositories/comment"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i CommentServiceInterface -p comment_service -s CommentService -o comment_service_iface.go
type CommentService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewCommentService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *CommentService {
	return &CommentService{cfg: cfg, repo: repo, resolve: resolve, events: events}
}

// Threads is the comment threads on the thing whose id is in column (e.g. issue_id), oldest
// first, each with its replies.
func (self *CommentService) Threads(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID) ([]models.CommentThread, error) {
	if !slices.Contains(comment_repo.Targets, column) {
		return nil, errdefs.BadRequest("Comments can't be on %q", column)
	}
	comments, err := self.repo.Comment().Threads(ctx, tx, column, id, nil, 0)
	if err != nil {
		return nil, err
	}
	threads := make([]models.CommentThread, len(comments))
	for i, c := range comments {
		threads[i] = models.CommentThreadOf(c)
	}
	return threads, nil
}
