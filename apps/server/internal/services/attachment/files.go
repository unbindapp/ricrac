package attachment_service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	attachment_repo "ricrac/internal/repositories/attachment"
)

const inlineLimit = 5 * 1024 * 1024

type PrepareUploadInput struct {
	Name        string `json:"name" doc:"The file name, e.g. screenshot.png."`
	ContentType string `json:"contentType" doc:"Its MIME type, e.g. image/png."`
	Size        int64  `json:"size" doc:"Its size in bytes."`
}

type UploadInput struct {
	Name        string `json:"name" doc:"The file name, e.g. screenshot.png."`
	ContentType string `json:"contentType" doc:"Its MIME type, e.g. image/png."`
	Base64      string `json:"base64" doc:"The file's bytes in base64 (a data: URL works too)."`
}

type FromUploadInput struct {
	Issue  string  `json:"issue" doc:"Issue identifier (ENG-123), id or link."`
	FileID string  `json:"fileId" doc:"The fileId from prepare_file_upload or upload_file."`
	Title  *string `json:"title" doc:"Defaults to the file name."`
}

func checkFileName(name string) (string, error) {
	name, err := utils.NonEmpty(name, "name")
	if err != nil {
		return "", err
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name[strings.LastIndexAny(name, `/\`)+1:])
	if name == "" || utf8.RuneCountInString(name) > 255 {
		return "", errdefs.BadRequest("name is a file name of 1 to 255 characters")
	}
	return name, nil
}

func checkContentType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	part := func(p string) bool {
		if p == "" {
			return false
		}
		for _, b := range []byte(p) {
			if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || strings.IndexByte("!#$&^_.+-", b) >= 0) {
				return false
			}
		}
		return true
	}
	kind, sub, found := strings.Cut(value, "/")
	if !found || len(value) > 127 || !part(kind) || !part(sub) {
		return "", errdefs.BadRequest("%q isn't a content type; use one like image/png, text/plain or application/pdf", value)
	}
	return value, nil
}

func fileMarkdown(name, contentType, url string) string {
	label := strings.NewReplacer("[", "", "]", "").Replace(name)
	if strings.HasPrefix(contentType, "image/") {
		return fmt.Sprintf("![%s](%s)", label, url)
	}
	return fmt.Sprintf("[%s](%s)", label, url)
}

func decodeBase64(value string) ([]byte, error) {
	data := strings.TrimSpace(value)
	if head, rest, found := strings.Cut(data, ";base64,"); found && strings.HasPrefix(head, "data:") {
		data = rest
	}
	data = strings.Join(strings.Fields(data), "")
	data = strings.NewReplacer("-", "+", "_", "/").Replace(data)
	if bytes, err := base64.StdEncoding.DecodeString(data); err == nil {
		return bytes, nil
	}
	if bytes, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(data, "=")); err == nil {
		return bytes, nil
	}
	return nil, errdefs.BadRequest("base64 isn't valid base64")
}

func humanSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func (self *AttachmentService) PrepareFileUpload(ctx context.Context, actor *auth.Actor, input *PrepareUploadInput) (models.PreparedUpload, error) {
	name, err := checkFileName(input.Name)
	if err != nil {
		return models.PreparedUpload{}, err
	}
	contentType, err := checkContentType(input.ContentType)
	if err != nil {
		return models.PreparedUpload{}, err
	}
	limit := self.cfg.MaxUploadSize
	if input.Size <= 0 || input.Size > limit {
		return models.PreparedUpload{}, errdefs.BadRequest("size is the file's size in bytes, at most %d", limit)
	}
	token := auth.RandomSecret("rr_up_")
	now, err := self.repo.Now(ctx, nil)
	if err != nil {
		return models.PreparedUpload{}, err
	}
	created, err := self.repo.Attachment().PrepareUpload(ctx, attachment_repo.FileInput{
		Workspace: actor.Workspace, Uploader: actor.User, Name: name, ContentType: contentType, Size: input.Size,
	}, auth.SecretHash(token), now.Add(15*time.Minute))
	if err != nil {
		return models.PreparedUpload{}, err
	}
	uploadURL := fmt.Sprintf("%s/uploads/%s", self.cfg.PublicURL, token)
	asset := AssetURL(self.cfg.PublicURL, created.ID)
	return models.PreparedUpload{
		FileID:    models.ID(created.ID),
		UploadURL: uploadURL,
		AssetURL:  asset,
		Markdown:  fileMarkdown(name, contentType, asset),
		ExpiresAt: created.UploadExpiresAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		Curl:      fmt.Sprintf("curl -sS --fail-with-body -X PUT -H 'content-type: %s' --data-binary @<file> '%s'", contentType, uploadURL),
	}, nil
}

func (self *AttachmentService) UploadFile(ctx context.Context, actor *auth.Actor, input *UploadInput) (models.UploadedFile, error) {
	name, err := checkFileName(input.Name)
	if err != nil {
		return models.UploadedFile{}, err
	}
	contentType, err := checkContentType(input.ContentType)
	if err != nil {
		return models.UploadedFile{}, err
	}
	bytes, err := decodeBase64(input.Base64)
	if err != nil {
		return models.UploadedFile{}, err
	}
	limit := min(int64(inlineLimit), self.cfg.MaxUploadSize)
	if len(bytes) == 0 || int64(len(bytes)) > limit {
		return models.UploadedFile{}, errdefs.BadRequest("The file must have 1 to %d bytes; upload bigger ones with prepare_file_upload", limit)
	}
	id, err := self.repo.Attachment().CreateFile(ctx, attachment_repo.FileInput{
		Workspace: actor.Workspace, Uploader: actor.User, Name: name, ContentType: contentType, Size: int64(len(bytes)),
	}, bytes)
	if err != nil {
		return models.UploadedFile{}, err
	}
	asset := AssetURL(self.cfg.PublicURL, id)
	return models.UploadedFile{
		FileID: models.ID(id), AssetURL: asset, Markdown: fileMarkdown(name, contentType, asset),
		Name: name, ContentType: contentType, Size: len(bytes),
	}, nil
}

func (self *AttachmentService) CreateFromUpload(ctx context.Context, actor *auth.Actor, input *FromUploadInput) (models.Attachment, error) {
	issue, err := self.resolve.Issue(ctx, actor, input.Issue)
	if err != nil {
		return models.Attachment{}, err
	}
	notFound := errdefs.NotFound("No uploaded file %q", strings.TrimSpace(input.FileID))
	file, ok := utils.AsUUID(input.FileID)
	if !ok {
		return models.Attachment{}, notFound
	}
	stored, uploaded, err := self.repo.Attachment().StoredFile(ctx, actor.Workspace, file)
	if err != nil {
		return models.Attachment{}, err
	}
	if stored == nil {
		return models.Attachment{}, notFound
	}
	if !uploaded {
		return models.Attachment{}, errdefs.BadRequest("The file hasn't been uploaded yet; PUT it to its uploadUrl first")
	}
	title := stored.Name
	if input.Title != nil && strings.TrimSpace(*input.Title) != "" {
		title = *input.Title
	}
	subtitle := fmt.Sprintf("%s, %s", stored.ContentType, humanSize(stored.Size))
	metadata := models.M{"fileId": file, "name": stored.Name, "contentType": stored.ContentType, "size": stored.Size}
	var value models.Attachment
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		id, err := self.Upsert(ctx, tx, actor, issue, AssetURL(self.cfg.PublicURL, file), title, &subtitle, metadata, nil)
		if err != nil {
			return err
		}
		value, err = self.Value(ctx, tx, id)
		return err
	})
	return value, err
}

// ReceiveUpload stores the bytes PUT to a one-time upload URL.
func (self *AttachmentService) ReceiveUpload(ctx context.Context, token string, data []byte) (models.ReceivedUpload, error) {
	id, found, err := self.repo.Attachment().Upload(ctx, auth.SecretHash(token), data)
	if err != nil {
		return models.ReceivedUpload{}, err
	}
	if !found {
		return models.ReceivedUpload{}, errdefs.NotFound("This upload URL is unknown, expired or already used; get a new one with prepare_file_upload")
	}
	return models.ReceivedUpload{AssetURL: AssetURL(self.cfg.PublicURL, id), FileID: models.ID(id), Size: len(data)}, nil
}

// File is an uploaded file with its bytes, nil when there's none.
func (self *AttachmentService) File(ctx context.Context, id uuid.UUID) (*ent.File, error) {
	return self.repo.Attachment().UploadedFile(ctx, id)
}
