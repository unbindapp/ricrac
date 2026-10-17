// Package issue_service is issues: their answers, saving and listing them, state changes and
// what they set off.
package issue_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	attachment_service "ricrac/internal/services/attachment"
	comment_service "ricrac/internal/services/comment"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i IssueServiceInterface -p issue_service -s IssueService -o issue_service_iface.go
type IssueService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface

	attachments attachment_service.AttachmentServiceInterface
	comments    comment_service.CommentServiceInterface
}

func NewIssueService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *IssueService {
	return &IssueService{cfg: cfg, repo: repo, resolve: resolve, events: events}
}

// Values is the issues as lists show them, in the order of ids, read in tx.
func (self *IssueService) Values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, withDescription bool) ([]models.Issue, error) {
	issues, err := self.repo.Issue().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.Issue().SubIssueCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.Issue, len(issues))
	for i, issue := range issues {
		values[i] = models.IssueOf(issue, self.cfg.PublicURL, counts[issue.ID], withDescription)
	}
	return values, nil
}

// Value is one issue with its description, read in tx.
func (self *IssueService) Value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.Issue, error) {
	values, err := self.Values(ctx, tx, []uuid.UUID{id}, true)
	if err != nil {
		return models.Issue{}, err
	}
	if len(values) == 0 {
		return models.Issue{}, errdefs.NotFound("No such issue")
	}
	return values[0], nil
}

// Uses gives the service the attachments and comments that issues show and save.
func (self *IssueService) Uses(attachments attachment_service.AttachmentServiceInterface, comments comment_service.CommentServiceInterface) {
	self.attachments, self.comments = attachments, comments
}
