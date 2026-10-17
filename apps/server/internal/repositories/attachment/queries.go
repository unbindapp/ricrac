package attachment_repo

import (
	"context"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/attachment"
	"ricrac/ent/file"
	repository "ricrac/internal/repositories"
)

func withAnswer(q *ent.AttachmentQuery) *ent.AttachmentQuery {
	return q.WithCreator().WithIssue(func(q *ent.IssueQuery) { q.WithTeam().WithState() })
}

func (self *AttachmentRepository) IssueOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	found, err := self.base.DB.Attachment.Get(ctx, id)
	if err != nil {
		return uuid.Nil, err
	}
	return found.IssueID, nil
}

// List is the issue's attachments with what their answer shows, oldest first.
func (self *AttachmentRepository) List(ctx context.Context, issue uuid.UUID, limit, offset int64) ([]*ent.Attachment, error) {
	return withAnswer(self.base.DB.Attachment.Query()).Where(attachment.IssueID(issue)).
		Order(ent.Asc(attachment.FieldCreatedAt), ent.Asc(attachment.FieldID)).
		Limit(int(limit)).Offset(int(offset)).All(ctx)
}

func (self *AttachmentRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).Attachment.Delete().Where(attachment.ID(id)).Exec(ctx)
	return err
}

// StoredFile is a file of the workspace without its bytes, nil when there's none, and whether it
// was uploaded.
func (self *AttachmentRepository) StoredFile(ctx context.Context, workspace, id uuid.UUID) (*ent.File, bool, error) {
	found, err := self.base.DB.File.Query().Where(file.ID(id), file.WorkspaceID(workspace)).
		Select(file.FieldID, file.FieldWorkspaceID, file.FieldName, file.FieldContentType, file.FieldSize).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	uploaded, err := self.base.DB.File.Query().Where(file.ID(id), file.DataNotNil()).Exist(ctx)
	return found, uploaded, err
}

// UploadedFile is a file with its bytes, nil when there's none or it wasn't uploaded yet.
func (self *AttachmentRepository) UploadedFile(ctx context.Context, id uuid.UUID) (*ent.File, error) {
	found, err := self.base.DB.File.Query().Where(file.ID(id), file.DataNotNil()).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}
