package attachment_repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
)

type FileInput struct {
	Workspace   uuid.UUID
	Uploader    uuid.UUID
	Name        string
	ContentType string
	Size        int64
}

// PrepareUpload creates a file waiting for its bytes, which come with the token whose hash is
// given until expiresAt.
func (self *AttachmentRepository) PrepareUpload(ctx context.Context, input FileInput, hash string, expiresAt time.Time) (*ent.File, error) {
	return self.base.DB.File.Create().
		SetWorkspaceID(input.Workspace).SetUploaderID(input.Uploader).SetName(input.Name).SetContentType(input.ContentType).
		SetSize(input.Size).SetUploadHash(hash).SetUploadExpiresAt(expiresAt).Save(ctx)
}

func (self *AttachmentRepository) CreateFile(ctx context.Context, input FileInput, data []byte) (uuid.UUID, error) {
	created, err := self.base.DB.File.Create().
		SetWorkspaceID(input.Workspace).SetUploaderID(input.Uploader).SetName(input.Name).SetContentType(input.ContentType).
		SetSize(input.Size).SetData(data).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// Upload stores the bytes of the file waiting for them with the token whose hash is given, once
// and before it expires; false when there's no such file.
func (self *AttachmentRepository) Upload(ctx context.Context, hash string, data []byte) (uuid.UUID, bool, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		`UPDATE files SET data = $2, size = $3, upload_hash = NULL, upload_expires_at = NULL
		 WHERE upload_hash = $1 AND upload_expires_at > now() AND data IS NULL RETURNING id`,
		hash, data, int64(len(data)))
	if err != nil {
		return uuid.Nil, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return uuid.Nil, false, rows.Err()
	}
	var id uuid.UUID
	err = rows.Scan(&id)
	return id, err == nil, err
}
