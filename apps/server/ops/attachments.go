package ops

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Attachments (links on issues, unique per URL) and file uploads, which agents embed in Markdown
// or attach to issues.

const inlineLimit = 5 * 1024 * 1024

func attachmentOps() []*Op {
	return []*Op{
		op("create_attachment", app.Write, []string{"POST /attachments"},
			"Attach a link (pull request, design, doc, error report, ...) to an issue. Attaching a URL the issue already "+
				"has updates that attachment instead. The source (github, figma, slack, ...) is recognized from the URL.",
			createAttachment),
		op("get_attachment", app.Read, []string{"GET /attachments/{id}"}, "Read an attachment by id.", getAttachment),
		op("list_attachments", app.Read, []string{"GET /issues/{issue}/attachments"},
			"List an issue's attachments, oldest first.", listAttachments),
		op("delete_attachment", app.Write, []string{"DELETE /attachments/{id}"}, "Remove an attachment from its issue.", deleteAttachment),
		op("prepare_file_upload", app.Write, []string{"POST /files"},
			"Get a one-time URL to upload a file (screenshot, log, recording, ...) to, valid for 15 minutes. Upload it "+
				"by running the returned `curl` command with <file> replaced by the local path (an HTTP PUT of the raw bytes). "+
				"Then embed the assetUrl in Markdown (`![name](assetUrl)` for images, `[name](assetUrl)` otherwise) in a "+
				"description or comment, or attach it to an issue with create_attachment_from_upload. Agents that can't run "+
				"commands use upload_file instead.",
			prepareFileUpload),
		op("upload_file", app.Write, []string{"POST /files/inline"},
			"Upload a small file (up to 5 MB) as base64 in the arguments, for agents that can't run commands; otherwise "+
				"prefer prepare_file_upload. Embed the returned assetUrl in Markdown (`![name](assetUrl)`) or attach it to an "+
				"issue with create_attachment_from_upload.",
			uploadFile),
		op("create_attachment_from_upload", app.Write, []string{"POST /attachments/from-upload"},
			"Attach an uploaded file (from prepare_file_upload or upload_file) to an issue.", createAttachmentFromUpload),
	}
}

const attachmentJSON = `jsonb_build_object('id', a.id, 'url', a.url, 'title', a.title, 'subtitle', a.subtitle,
	'sourceType', a.source_type, 'metadata', a.metadata, 'iconUrl', a.icon_url, 'creator', user_ref(a.creator_id),
	'issue', issue_ref(a.issue_id), 'createdAt', a.created_at, 'updatedAt', a.updated_at)`

func attachmentValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, "SELECT "+attachmentJSON+" FROM attachments a WHERE a.id = $1", id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such attachment")
	}
	return value, nil
}

// AssetURL is where a file is served from; Markdown and attachments point here.
func AssetURL(publicURL string, id uuid.UUID) string {
	return fmt.Sprintf("%s/files/%s", publicURL, id)
}

// urlHost is the host of a URL, lowercase, without credentials or port.
func urlHost(url string) string {
	_, rest, found := strings.Cut(url, "://")
	if !found {
		return ""
	}
	authority := rest
	if i := strings.IndexAny(authority, "/?#"); i >= 0 {
		authority = authority[:i]
	}
	host := authority[strings.LastIndex(authority, "@")+1:]
	if ipv6, ok := strings.CutPrefix(host, "["); ok {
		host, _, _ = strings.Cut(ipv6, "]")
	} else {
		host, _, _ = strings.Cut(host, ":")
	}
	return strings.ToLower(host)
}

func sourceType(url, publicURL string) string {
	name := urlHost(url)
	if name == "" {
		return "link"
	}
	_, rest, _ := strings.Cut(url, "://")
	path := ""
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		path = rest[i:]
	}
	if urlHost(publicURL) == name && strings.HasPrefix(path, "/files/") {
		return "upload"
	}
	sources := [][2]string{
		{"github.com", "github"}, {"githubusercontent.com", "github"}, {"gitlab.com", "gitlab"}, {"figma.com", "figma"},
		{"slack.com", "slack"}, {"sentry.io", "sentry"}, {"notion.so", "notion"}, {"notion.site", "notion"}, {"loom.com", "loom"},
	}
	for _, s := range sources {
		if name == s[0] || strings.HasSuffix(name, "."+s[0]) {
			return s[1]
		}
	}
	if strings.HasPrefix(name, "gitlab.") {
		return "gitlab"
	}
	return "link"
}

// upsertAttachment attaches a link to an issue, or updates the attachment with the same URL.
func upsertAttachment(c *app.Ctx, tx pgx.Tx, issue uuid.UUID, url, title string, subtitle *string, metadata any) (uuid.UUID, error) {
	return saveAttachment(c, tx, issue, url, title, subtitle, metadata, nil)
}

func saveAttachment(c *app.Ctx, tx pgx.Tx, issue uuid.UUID, url, title string, subtitle *string, metadata any, iconURL *string) (uuid.UUID, error) {
	url = strings.TrimSpace(url)
	if !(strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")) || urlHost(url) == "" {
		return uuid.Nil, app.BadRequest("%q isn't a link; attachments need an http(s) URL", url)
	}
	title, err := nonEmpty(title, "title")
	if err != nil {
		return uuid.Nil, err
	}
	if metadata != nil {
		if _, ok := metadata.(M); !ok {
			return uuid.Nil, app.BadRequest("metadata must be an object")
		}
	}
	var trimmedSubtitle *string
	if subtitle != nil && strings.TrimSpace(*subtitle) != "" {
		trimmedSubtitle = new(strings.TrimSpace(*subtitle))
	}
	type saved struct {
		ID      uuid.UUID
		Created bool
	}
	row, _, err := app.Row[saved](c, tx,
		`INSERT INTO attachments (workspace_id, issue_id, url, title, subtitle, metadata, icon_url, source_type, creator_id)
		 VALUES ($1, $2, $3, $4, $5, coalesce($6, '{}'), $7, $8, $9)
		 ON CONFLICT (issue_id, url) DO UPDATE SET title = EXCLUDED.title,
			subtitle = coalesce($5, attachments.subtitle), metadata = coalesce($6, attachments.metadata),
			icon_url = coalesce($7, attachments.icon_url), updated_at = now()
		 RETURNING id, xmax = 0`,
		c.Workspace, issue, url, title, trimmedSubtitle, metadata, iconURL, sourceType(url, c.PublicURL()), c.User)
	if err != nil {
		return uuid.Nil, err
	}
	value, err := attachmentValue(c, tx, row.ID)
	if err != nil {
		return uuid.Nil, err
	}
	team, err := app.One[uuid.UUID](c, tx, "SELECT team_id FROM issues WHERE id = $1", issue)
	if err != nil {
		return uuid.Nil, err
	}
	action := "update"
	if row.Created {
		action = "create"
	}
	return row.ID, emit(c, tx, "Attachment", action, value, nil, &team)
}

type CreateAttachmentArgs struct {
	Issue    string  `json:"issue" doc:"Issue identifier (ENG-123), id or link."`
	URL      string  `json:"url" doc:"The link to attach."`
	Title    string  `json:"title"`
	Subtitle *string `json:"subtitle" doc:"A second line, e.g. a PR's state or a file's size."`
	Metadata M       `json:"metadata" doc:"Any JSON object to keep with the attachment."`
	IconURL  *string `json:"iconUrl" doc:"Link to an icon to show with it."`
}

func createAttachment(c *app.Ctx, args CreateAttachmentArgs) (any, error) {
	issue, err := resolveIssue(c, args.Issue)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var metadata any
		if args.Metadata != nil {
			metadata = args.Metadata
		}
		id, err := saveAttachment(c, tx, issue, args.URL, args.Title, args.Subtitle, metadata, args.IconURL)
		if err != nil {
			return err
		}
		value, err = attachmentValue(c, tx, id)
		return err
	})
	return value, err
}

// findAttachment is an attachment of the workspace on an issue the caller can see.
func findAttachment(c *app.Ctx, value string) (uuid.UUID, uuid.UUID, error) {
	id, err := resolveByID(c, "attachments", "attachment", value)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	issue, err := app.One[uuid.UUID](c, c.DB(), "SELECT issue_id FROM attachments WHERE id = $1", id)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	visible, err := canSeeIssue(c, issue)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, uuid.Nil, app.NotFound("No attachment %q", strings.TrimSpace(value))
	}
	return id, issue, nil
}

func getAttachment(c *app.Ctx, args IDArgs) (any, error) {
	id, _, err := findAttachment(c, args.ID)
	if err != nil {
		return nil, err
	}
	return attachmentValue(c, c.DB(), id)
}

type ListAttachmentsArgs struct {
	Issue string `json:"issue" doc:"Issue identifier (ENG-123), id or link."`
	Page
}

func listAttachments(c *app.Ctx, args ListAttachmentsArgs) (any, error) {
	issue, err := resolveIssue(c, args.Issue)
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		"SELECT "+attachmentJSON+" FROM attachments a WHERE a.issue_id = $1 ORDER BY a.created_at, a.id LIMIT $2 OFFSET $3",
		issue, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func deleteAttachment(c *app.Ctx, args IDArgs) (any, error) {
	id, issue, err := findAttachment(c, args.ID)
	if err != nil {
		return nil, err
	}
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := attachmentValue(c, tx, id)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM attachments WHERE id = $1", id); err != nil {
			return err
		}
		team, err := app.One[uuid.UUID](c, tx, "SELECT team_id FROM issues WHERE id = $1", issue)
		if err != nil {
			return err
		}
		return emit(c, tx, "Attachment", "remove", value, nil, &team)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": id}, nil
}

/* Uploads */

func checkFileName(name string) (string, error) {
	name, err := nonEmpty(name, "name")
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
		return "", app.BadRequest("name is a file name of 1 to 255 characters")
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
		return "", app.BadRequest("%q isn't a content type; use one like image/png, text/plain or application/pdf", value)
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

type PrepareFileUploadArgs struct {
	Name        string `json:"name" doc:"The file name, e.g. screenshot.png."`
	ContentType string `json:"contentType" doc:"Its MIME type, e.g. image/png."`
	Size        int64  `json:"size" doc:"Its size in bytes."`
}

func prepareFileUpload(c *app.Ctx, args PrepareFileUploadArgs) (any, error) {
	name, err := checkFileName(args.Name)
	if err != nil {
		return nil, err
	}
	contentType, err := checkContentType(args.ContentType)
	if err != nil {
		return nil, err
	}
	limit := c.State.Config.MaxUploadSize
	if args.Size <= 0 || args.Size > limit {
		return nil, app.BadRequest("size is the file's size in bytes, at most %d", limit)
	}
	token := app.RandomSecret("rr_up_")
	type created struct {
		ID        uuid.UUID
		ExpiresAt time.Time
	}
	row, _, err := app.Row[created](c, c.DB(),
		`INSERT INTO files (workspace_id, uploader_id, name, content_type, size, upload_hash, upload_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, now() + interval '15 minutes') RETURNING id, upload_expires_at`,
		c.Workspace, c.User, name, contentType, args.Size, app.SecretHash(token))
	if err != nil {
		return nil, err
	}
	uploadURL := fmt.Sprintf("%s/uploads/%s", c.PublicURL(), token)
	asset := AssetURL(c.PublicURL(), row.ID)
	return M{
		"fileId":    row.ID,
		"uploadUrl": uploadURL,
		"assetUrl":  asset,
		"markdown":  fileMarkdown(name, contentType, asset),
		"expiresAt": row.ExpiresAt.UTC(),
		"curl":      fmt.Sprintf("curl -sS --fail-with-body -X PUT -H 'content-type: %s' --data-binary @<file> '%s'", contentType, uploadURL),
	}, nil
}

type UploadFileArgs struct {
	Name        string `json:"name" doc:"The file name, e.g. screenshot.png."`
	ContentType string `json:"contentType" doc:"Its MIME type, e.g. image/png."`
	Base64      string `json:"base64" doc:"The file's bytes in base64 (a data: URL works too)."`
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
	return nil, app.BadRequest("base64 isn't valid base64")
}

func uploadFile(c *app.Ctx, args UploadFileArgs) (any, error) {
	name, err := checkFileName(args.Name)
	if err != nil {
		return nil, err
	}
	contentType, err := checkContentType(args.ContentType)
	if err != nil {
		return nil, err
	}
	bytes, err := decodeBase64(args.Base64)
	if err != nil {
		return nil, err
	}
	limit := min(int64(inlineLimit), c.State.Config.MaxUploadSize)
	if len(bytes) == 0 || int64(len(bytes)) > limit {
		return nil, app.BadRequest("The file must have 1 to %d bytes; upload bigger ones with prepare_file_upload", limit)
	}
	id, err := app.One[uuid.UUID](c, c.DB(),
		`INSERT INTO files (workspace_id, uploader_id, name, content_type, size, data)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, c.Workspace, c.User, name, contentType, int64(len(bytes)), bytes)
	if err != nil {
		return nil, err
	}
	asset := AssetURL(c.PublicURL(), id)
	return M{
		"fileId":      id,
		"assetUrl":    asset,
		"markdown":    fileMarkdown(name, contentType, asset),
		"name":        name,
		"contentType": contentType,
		"size":        len(bytes),
	}, nil
}

type CreateAttachmentFromUploadArgs struct {
	Issue  string  `json:"issue" doc:"Issue identifier (ENG-123), id or link."`
	FileID string  `json:"fileId" doc:"The fileId from prepare_file_upload or upload_file."`
	Title  *string `json:"title" doc:"Defaults to the file name."`
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

func createAttachmentFromUpload(c *app.Ctx, args CreateAttachmentFromUploadArgs) (any, error) {
	issue, err := resolveIssue(c, args.Issue)
	if err != nil {
		return nil, err
	}
	notFound := app.NotFound("No uploaded file %q", strings.TrimSpace(args.FileID))
	file, ok := asUUID(args.FileID)
	if !ok {
		return nil, notFound
	}
	type stored struct {
		Name        string
		ContentType string
		Size        int64
		Uploaded    bool
	}
	row, found, err := app.Row[stored](c, c.DB(),
		"SELECT name, content_type, size, data IS NOT NULL FROM files WHERE id = $1 AND workspace_id = $2", file, c.Workspace)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound
	}
	if !row.Uploaded {
		return nil, app.BadRequest("The file hasn't been uploaded yet; PUT it to its uploadUrl first")
	}
	title := row.Name
	if args.Title != nil && strings.TrimSpace(*args.Title) != "" {
		title = *args.Title
	}
	subtitle := fmt.Sprintf("%s, %s", row.ContentType, humanSize(row.Size))
	metadata := M{"fileId": file, "name": row.Name, "contentType": row.ContentType, "size": row.Size}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		id, err := saveAttachment(c, tx, issue, AssetURL(c.PublicURL(), file), title, &subtitle, metadata, nil)
		if err != nil {
			return err
		}
		value, err = attachmentValue(c, tx, id)
		return err
	})
	return value, err
}
