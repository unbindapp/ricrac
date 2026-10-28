// Package discussion_service is the forum: discussions, the smaller threads inside them (a bug, a
// design check, a question) and who follows them. Comments on either go through comment_service.
package discussion_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i DiscussionServiceInterface -p discussion_service -s DiscussionService -o discussion_service_iface.go
type DiscussionService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewDiscussionService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *DiscussionService {
	return &DiscussionService{cfg: cfg, repo: repo, resolve: resolve, events: events}
}

type IDInput struct {
	ID string `json:"id"`
}

func (self *DiscussionService) values(ctx context.Context, tx repository.TxInterface, viewer uuid.UUID, ids []uuid.UUID, withBody bool) ([]models.Discussion, error) {
	discussions, err := self.repo.Discussion().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.Discussion().Counts(ctx, tx, ids, viewer)
	if err != nil {
		return nil, err
	}
	values := make([]models.Discussion, len(discussions))
	for i, d := range discussions {
		values[i] = models.DiscussionOf(d, self.cfg.PublicURL, counts[d.ID], withBody)
	}
	return values, nil
}

// value is the discussion as reading or saving it answers, with its threads and subscribers.
func (self *DiscussionService) value(ctx context.Context, tx repository.TxInterface, viewer, id uuid.UUID) (models.DiscussionDetail, error) {
	values, err := self.values(ctx, tx, viewer, []uuid.UUID{id}, true)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	if len(values) == 0 {
		return models.DiscussionDetail{}, errdefs.NotFound("No such discussion")
	}
	threads, err := self.repo.Discussion().Threads(ctx, tx, id)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	threadIDs := make([]uuid.UUID, len(threads))
	for i, t := range threads {
		threadIDs[i] = t.ID
	}
	comments, err := self.repo.Discussion().ThreadCommentCounts(ctx, tx, threadIDs)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	detail := models.DiscussionDetail{Discussion: values[0], Threads: make([]models.DiscussionThread, len(threads)), Subscribers: []models.UserRef{}}
	for i, t := range threads {
		detail.Threads[i] = models.DiscussionThreadOf(t, self.cfg.PublicURL, comments[t.ID])
	}
	subscribers, err := self.repo.Discussion().Subscribers(ctx, tx, id)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	for _, u := range subscribers {
		detail.Subscribers = append(detail.Subscribers, *models.UserRefOf(u))
	}
	return detail, nil
}

// find is a discussion the caller can see, by id, slug, link or title; with trashed, only among
// those in the trash.
func (self *DiscussionService) find(ctx context.Context, actor *auth.Actor, value string, trashed bool) (uuid.UUID, error) {
	if !trashed {
		return self.resolve.Discussion(ctx, actor, value)
	}
	value = strings.TrimSpace(value)
	var asID *uuid.UUID
	if id, ok := utils.AsUUID(value); ok {
		asID = &id
	}
	notFound := errdefs.NotFound("No discussion %q in the trash", value)
	id, found, err := self.repo.Discussion().Trashed(ctx, actor.Workspace, asID, utils.SlugTail(value), value)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, notFound
	}
	visible, err := self.resolve.CanSeeDiscussion(ctx, actor, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, notFound
	}
	return id, nil
}

// subscribe follows the discussion for the users, leaving out the nobody GitHub acts as.
func (self *DiscussionService) subscribe(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users ...uuid.UUID) error {
	users = slices.DeleteFunc(users, func(u uuid.UUID) bool { return u == uuid.Nil })
	return self.repo.Discussion().Subscribe(ctx, tx, id, users)
}

// newlyMentioned is the members mentioned in after and not in before.
func (self *DiscussionService) newlyMentioned(ctx context.Context, actor *auth.Actor, before, after string) ([]uuid.UUID, error) {
	previous, err := self.events.Mentions(ctx, actor, before)
	if err != nil {
		return nil, err
	}
	mentioned, err := self.events.Mentions(ctx, actor, after)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(mentioned, func(u uuid.UUID) bool { return slices.Contains(previous, u) }), nil
}

func (self *DiscussionService) actorName(ctx context.Context, actor *auth.Actor) (string, error) {
	return self.repo.Comment().UserName(ctx, actor.User)
}
