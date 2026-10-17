package attachment_repo

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/attachment"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AttachmentRepositoryInterface -p attachment_repo -s AttachmentRepository -o attachment_repository_iface.go
type AttachmentRepository struct {
	base *repository.BaseRepository
}

func NewAttachmentRepository(db *ent.Client) *AttachmentRepository {
	return &AttachmentRepository{base: repository.NewBaseRepository(db)}
}

// Load is the attachment with what its answer shows.
func (self *AttachmentRepository) Load(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Attachment, error) {
	return self.base.Client(tx).Attachment.Query().Where(attachment.ID(id)).
		WithCreator().WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() }).Only(ctx)
}

type UpsertInput struct {
	Workspace, Issue uuid.UUID
	URL, Title       string
	Subtitle         *string
	// Nil keeps what an existing attachment has ({} for a new one).
	Metadata   json.RawMessage
	IconURL    *string
	SourceType string
	Creator    *uuid.UUID
}

// Upsert attaches a link to an issue, or updates the attachment with the same URL, saying
// whether it created one.
func (self *AttachmentRepository) Upsert(ctx context.Context, tx repository.TxInterface, input UpsertInput) (uuid.UUID, bool, error) {
	var metadata any
	if input.Metadata != nil {
		metadata = string(input.Metadata)
	}
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`INSERT INTO attachments (workspace_id, issue_id, url, title, subtitle, metadata, icon_url, source_type, creator_id)
		 VALUES ($1, $2, $3, $4, $5, coalesce($6::jsonb, '{}'), $7, $8, $9)
		 ON CONFLICT (issue_id, url) DO UPDATE SET title = EXCLUDED.title,
			subtitle = coalesce($5, attachments.subtitle), metadata = coalesce($6, attachments.metadata),
			icon_url = coalesce($7, attachments.icon_url), updated_at = now()
		 RETURNING id, xmax = 0`,
		input.Workspace, input.Issue, input.URL, input.Title, input.Subtitle, metadata, input.IconURL, input.SourceType, input.Creator)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer rows.Close()
	var id uuid.UUID
	var created bool
	if rows.Next() {
		err = rows.Scan(&id, &created)
	}
	if err == nil {
		err = rows.Err()
	}
	return id, created, err
}
