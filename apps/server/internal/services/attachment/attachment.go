// Package attachment_service is attachments on issues and uploaded files.
package attachment_service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	attachment_repo "ricrac/internal/repositories/attachment"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AttachmentServiceInterface -p attachment_service -s AttachmentService -o attachment_service_iface.go
type AttachmentService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewAttachmentService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *AttachmentService {
	return &AttachmentService{cfg: cfg, repo: repo, resolve: resolve, events: events}
}

// Value is the attachment as the API answers it, read in tx.
func (self *AttachmentService) Value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.Attachment, error) {
	found, err := self.repo.Attachment().Load(ctx, tx, id)
	if ent.IsNotFound(err) {
		return models.Attachment{}, errdefs.NotFound("No such attachment")
	}
	if err != nil {
		return models.Attachment{}, err
	}
	return models.AttachmentOf(found), nil
}

// Upsert attaches a link to an issue, or updates the attachment with the same URL. metadata is
// nil or a JSON object.
func (self *AttachmentService) Upsert(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue uuid.UUID, url, title string, subtitle *string, metadata any, iconURL *string) (uuid.UUID, error) {
	url = strings.TrimSpace(url)
	if !(strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")) || URLHost(url) == "" {
		return uuid.Nil, errdefs.BadRequest("%q isn't a link; attachments need an http(s) URL", url)
	}
	title, err := utils.NonEmpty(title, "title")
	if err != nil {
		return uuid.Nil, err
	}
	var encoded json.RawMessage
	if metadata != nil {
		if _, ok := metadata.(map[string]any); !ok {
			return uuid.Nil, errdefs.BadRequest("metadata must be an object")
		}
		if encoded, err = json.Marshal(metadata); err != nil {
			return uuid.Nil, err
		}
	}
	var trimmedSubtitle *string
	if subtitle != nil && strings.TrimSpace(*subtitle) != "" {
		trimmedSubtitle = new(strings.TrimSpace(*subtitle))
	}
	id, created, err := self.repo.Attachment().Upsert(ctx, tx, attachment_repo.UpsertInput{
		Workspace: actor.Workspace, Issue: issue, URL: url, Title: title, Subtitle: trimmedSubtitle, Metadata: encoded,
		IconURL: iconURL, SourceType: SourceType(url, self.cfg.PublicURL), Creator: actor.ActorID(),
	})
	if err != nil {
		return uuid.Nil, err
	}
	value, err := self.Value(ctx, tx, id)
	if err != nil {
		return uuid.Nil, err
	}
	row, err := self.repo.Issue().Get(ctx, tx, issue)
	if err != nil {
		return uuid.Nil, err
	}
	action := "update"
	if created {
		action = "create"
	}
	return id, self.events.Emit(ctx, tx, actor, "Attachment", action, value, nil, &row.TeamID)
}

// AssetURL is where a file is served from; Markdown and attachments point here.
func AssetURL(publicURL string, id uuid.UUID) string {
	return fmt.Sprintf("%s/files/%s", publicURL, id)
}

// URLHost is the host of a URL, lowercase, without credentials or port.
func URLHost(url string) string {
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

// SourceType is what kind of link url is: upload, github, gitlab, figma, ... or link.
func SourceType(url, publicURL string) string {
	name := URLHost(url)
	if name == "" {
		return "link"
	}
	_, rest, _ := strings.Cut(url, "://")
	path := ""
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		path = rest[i:]
	}
	if URLHost(publicURL) == name && strings.HasPrefix(path, "/files/") {
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
