package attachment_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

type CreateInput struct {
	Issue    string   `json:"issue" doc:"Issue identifier (ENG-123), id or link."`
	URL      string   `json:"url" doc:"The link to attach."`
	Title    string   `json:"title"`
	Subtitle *string  `json:"subtitle" doc:"A second line, e.g. a PR's state or a file's size."`
	Metadata models.M `json:"metadata" doc:"Any JSON object to keep with the attachment."`
	IconURL  *string  `json:"iconUrl" doc:"Link to an icon to show with it."`
}

type IDInput struct {
	ID string `json:"id"`
}

type ListInput struct {
	Issue string `json:"issue" doc:"Issue identifier (ENG-123), id or link."`
	args.Page
}

func (self *AttachmentService) Create(ctx context.Context, actor *auth.Actor, input *CreateInput) (models.Attachment, error) {
	issue, err := self.resolve.Issue(ctx, actor, input.Issue)
	if err != nil {
		return models.Attachment{}, err
	}
	var value models.Attachment
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var metadata any
		if input.Metadata != nil {
			metadata = input.Metadata
		}
		id, err := self.Upsert(ctx, tx, actor, issue, input.URL, input.Title, input.Subtitle, metadata, input.IconURL)
		if err != nil {
			return err
		}
		value, err = self.Value(ctx, tx, id)
		return err
	})
	return value, err
}

// find is an attachment of the workspace on an issue the caller can see, and the issue.
func (self *AttachmentService) find(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, uuid.UUID, error) {
	id, err := self.resolve.ByID(ctx, actor, "attachments", "attachment", value)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	issue, err := self.repo.Attachment().IssueOf(ctx, id)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	visible, err := self.resolve.CanSeeIssue(ctx, actor, issue)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, uuid.Nil, errdefs.NotFound("No attachment %q", strings.TrimSpace(value))
	}
	return id, issue, nil
}

func (self *AttachmentService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Attachment, error) {
	id, _, err := self.find(ctx, actor, input.ID)
	if err != nil {
		return models.Attachment{}, err
	}
	return self.Value(ctx, nil, id)
}

func (self *AttachmentService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Attachment], error) {
	issue, err := self.resolve.Issue(ctx, actor, input.Issue)
	if err != nil {
		return args.PageOf[models.Attachment]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Attachment]{}, err
	}
	found, err := self.repo.Attachment().List(ctx, issue, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.Attachment]{}, err
	}
	attachments := make([]models.Attachment, len(found))
	for i, a := range found {
		attachments[i] = models.AttachmentOf(a)
	}
	return args.WrapOf(input.Page, attachments)
}

func (self *AttachmentService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Deleted, error) {
	id, issue, err := self.find(ctx, actor, input.ID)
	if err != nil {
		return models.Deleted{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.Value(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := self.repo.Attachment().Delete(ctx, tx, id); err != nil {
			return err
		}
		row, err := self.repo.Issue().Get(ctx, tx, issue)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Attachment", "remove", value, nil, &row.TeamID)
	})
	if err != nil {
		return models.Deleted{}, err
	}
	return models.Deleted{Deleted: models.ID(id)}, nil
}
