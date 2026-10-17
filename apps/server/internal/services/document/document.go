// Package document_service is documents: Markdown pages attached to a project, initiative, team,
// issue, cycle or release, with earlier versions kept as they are rewritten.
package document_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i DocumentServiceInterface -p document_service -s DocumentService -o document_service_iface.go
type DocumentService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewDocumentService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *DocumentService {
	return &DocumentService{cfg: cfg, repo: repo, resolve: resolve, events: events}
}

type IDInput struct {
	ID string `json:"id"`
}

type ListInput struct {
	Project        *string `json:"project" doc:"Project name, slug or id."`
	Initiative     *string `json:"initiative" doc:"Initiative name, slug or id."`
	Team           *string `json:"team" doc:"Team key, name or id: the team's documents (with cycle, only the team the cycle belongs to)."`
	Issue          *string `json:"issue" doc:"Issue identifier (ENG-123) or id."`
	Cycle          *string `json:"cycle" doc:"Cycle id, or its number, name or \"current\" with team."`
	Release        *string `json:"release" doc:"Release name, slug or id."`
	Creator        *string `json:"creator" doc:"Person who created the documents (id, name, email or \"me\")."`
	Query          *string `json:"query" doc:"Words to find in the title or content."`
	IncludeTrashed bool    `json:"includeTrashed,omitempty"`
	IncludeContent bool    `json:"includeContent,omitempty" doc:"Include each document's content."`
	OrderBy        *string `json:"orderBy" doc:"updatedAt (default) or createdAt, newest first."`
	args.Page
}

type SaveInput struct {
	ID             *string               `json:"id" doc:"Only to update a document: its id, slug, link or title. Omit to create one."`
	Title          *string               `json:"title"`
	Content        *string               `json:"content" doc:"Markdown. Mention people with @displayName or @email; they are notified."`
	Patch          []args.TextEdit       `json:"patch" doc:"Edits to the current content instead of the whole text, applied in order, all or none."`
	Icon           args.Nullable[string] `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color          args.Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Project        *string               `json:"project" doc:"Parent project: name, slug or id."`
	Initiative     *string               `json:"initiative" doc:"Parent initiative: name, slug or id."`
	Team           *string               `json:"team" doc:"Parent team: key, name or id (or the team of the cycle, with cycle)."`
	Issue          *string               `json:"issue" doc:"Parent issue: identifier (ENG-123) or id."`
	Cycle          *string               `json:"cycle" doc:"Parent cycle: id, or number, name or \"current\" with team."`
	Release        *string               `json:"release" doc:"Parent release: name, slug or id."`
	Template       *string               `json:"template" doc:"Document template name or id, applied on create: it gives the title and content the arguments don't."`
	RestoreVersion *int64                `json:"restoreVersion" doc:"An earlier version (its id from list_document_versions) to bring back on update: its title and content replace the current ones, which are kept as a version."`
}

func (self *DocumentService) values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, withContent bool) ([]models.Document, error) {
	documents, err := self.repo.Document().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.Document, len(documents))
	for i, d := range documents {
		values[i] = models.DocumentOf(d, self.cfg.PublicURL, withContent)
	}
	return values, nil
}

// value is the document as the API answers it, with its content and version count, read in tx.
func (self *DocumentService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.DocumentDetail, error) {
	values, err := self.values(ctx, tx, []uuid.UUID{id}, true)
	if err != nil {
		return models.DocumentDetail{}, err
	}
	if len(values) == 0 {
		return models.DocumentDetail{}, errdefs.NotFound("No such document")
	}
	count, err := self.repo.Document().VersionCount(ctx, tx, id)
	if err != nil {
		return models.DocumentDetail{}, err
	}
	return models.DocumentDetail{Document: values[0], VersionCount: count}, nil
}

// find is a document the caller can see, by id, slug, link or title; with trashed, only among
// those in the trash.
func (self *DocumentService) find(ctx context.Context, actor *auth.Actor, value string, trashed bool) (uuid.UUID, error) {
	if !trashed {
		return self.resolve.Document(ctx, actor, value)
	}
	value = strings.TrimSpace(value)
	tail := strings.TrimRight(value, "/")
	tail = tail[strings.LastIndexAny(tail, "/-")+1:]
	var asID *uuid.UUID
	if id, ok := utils.AsUUID(value); ok {
		asID = &id
	}
	notFound := errdefs.NotFound("No document %q in the trash", value)
	id, found, err := self.repo.Document().Trashed(ctx, actor.Workspace, asID, tail, value)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, notFound
	}
	visible, err := self.resolve.CanSeeDocument(ctx, actor, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, notFound
	}
	return id, nil
}
