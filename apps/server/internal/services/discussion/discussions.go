package discussion_service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	discussion_repo "ricrac/internal/repositories/discussion"
	events_service "ricrac/internal/services/events"
)

type ListInput struct {
	Team           *string `json:"team" doc:"Only the discussions of this team (key, name or id)."`
	Creator        *string `json:"creator" doc:"Person who started the discussions (id, name, email or \"me\")."`
	Tag            *string `json:"tag" doc:"Only the discussions with this tag."`
	Status         *string `json:"status" enum:"open,closed,all" doc:"open (default), closed or all."`
	Query          *string `json:"query" doc:"Words to find in the title or body."`
	Sort           *string `json:"sort" enum:"activity,newest,top" doc:"activity (default): pinned first, then the most recent activity; newest: pinned first, then the newest; top: pinned first, then the most upvoted, ties by recent activity."`
	IncludeTrashed bool    `json:"includeTrashed,omitempty"`
	OnlyTrashed    bool    `json:"onlyTrashed,omitempty" doc:"Only discussions in the trash, the most recently trashed first."`
	IncludeBody    bool    `json:"includeBody,omitempty" doc:"Include each discussion's body."`
	args.Page
}

type SaveInput struct {
	ID                *string               `json:"id" doc:"Only to update a discussion: its id, slug, link or title. Omit to start one."`
	Title             *string               `json:"title"`
	Body              *string               `json:"body" doc:"Markdown. Mention people with @displayName or @email; they are notified and subscribed."`
	Patch             []args.TextEdit       `json:"patch" doc:"Edits to the current body instead of the whole text, applied in order, all or none."`
	Team              args.Nullable[string] `json:"team" doc:"The team it belongs to (key, name or id); only that team's members see it. null opens it to the whole workspace."`
	ThumbnailURL      args.Nullable[string] `json:"thumbnailUrl" doc:"Image URL the forum shows for it, such as an assetUrl from prepare_file_upload; null shows the first image in the body."`
	Tags              []string              `json:"tags" doc:"All of its tags, replacing the ones it has; [] removes them. Lowercased, up to 10 of up to 32 characters."`
	Pinned            *bool                 `json:"pinned" doc:"Pin it to the top of the forum, or unpin it with false."`
	Closed            *bool                 `json:"closed" doc:"Close it once it is settled, or reopen it with false. Closed discussions still take comments and threads."`
	AddSubscribers    []string              `json:"addSubscribers" doc:"People to subscribe: ids, names, emails or \"me\"."`
	RemoveSubscribers []string              `json:"removeSubscribers" doc:"People to unsubscribe: ids, names, emails or \"me\"."`
}

var (
	statuses = []string{"open", "closed", "all"}
	sorts    = []string{"activity", "newest", "top"}
)

func (self *DiscussionService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Discussion], error) {
	status := "open"
	if input.Status != nil {
		status = strings.TrimSpace(*input.Status)
	}
	if !slices.Contains(statuses, status) {
		return args.PageOf[models.Discussion]{}, errdefs.BadRequest("status %q isn't open, closed or all", status)
	}
	sort := "activity"
	if input.Sort != nil {
		sort = strings.TrimSpace(*input.Sort)
	}
	if !slices.Contains(sorts, sort) {
		return args.PageOf[models.Discussion]{}, errdefs.BadRequest("sort %q isn't activity, newest or top", sort)
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.Discussion]{}, err
	}
	filter := discussion_repo.ListFilter{
		Workspace: actor.Workspace, Teams: teams, Status: status, Sort: sort, IncludeTrashed: input.IncludeTrashed, OnlyTrashed: input.OnlyTrashed,
	}
	if filter.Team, err = self.resolve.OptionalTeam(ctx, actor, input.Team); err != nil {
		return args.PageOf[models.Discussion]{}, err
	}
	if input.Creator != nil {
		creator, err := self.resolve.User(ctx, actor, *input.Creator)
		if err != nil {
			return args.PageOf[models.Discussion]{}, err
		}
		filter.Creator = &creator
	}
	if input.Tag != nil {
		tag := normalTag(*input.Tag)
		filter.Tag = &tag
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Query = &query
	}
	if filter.Offset, err = input.Offset(); err != nil {
		return args.PageOf[models.Discussion]{}, err
	}
	filter.Limit = input.Size() + 1
	ids, err := self.repo.Discussion().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.Discussion]{}, err
	}
	discussions, err := self.values(ctx, nil, actor.User, ids, input.IncludeBody)
	if err != nil {
		return args.PageOf[models.Discussion]{}, err
	}
	return args.WrapOf(input.Page, discussions)
}

func (self *DiscussionService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DiscussionDetail, error) {
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	return self.value(ctx, nil, actor.User, id)
}

func (self *DiscussionService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.DiscussionDetail, error) {
	var value models.DiscussionDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		action := "create"
		if input.ID != nil {
			action = "update"
			if id, err = self.find(ctx, actor, *input.ID, false); err != nil {
				return err
			}
			err = self.update(ctx, tx, actor, id, input)
		} else {
			id, err = self.create(ctx, tx, actor, input)
		}
		if err != nil {
			return err
		}
		if err := self.changeSubscribers(ctx, tx, actor, id, input); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, actor.User, id); err != nil {
			return err
		}
		return self.emit(ctx, tx, actor, action, value.Discussion)
	})
	return value, err
}

func (self *DiscussionService) emit(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, action string, value models.Discussion) error {
	var team *uuid.UUID
	if value.Team != nil {
		team = new(uuid.MustParse(value.Team.ID))
	}
	return self.events.Emit(ctx, tx, actor, "Discussion", action, value, nil, team)
}

func (self *DiscussionService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	if input.Patch != nil {
		return uuid.Nil, errdefs.BadRequest("patch only applies when updating; pass body")
	}
	title, err := utils.NonEmpty(utils.OrEmpty(input.Title), "title")
	if err != nil {
		return uuid.Nil, err
	}
	team, err := self.resolve.OptionalTeam(ctx, actor, input.Team.Ptr())
	if err != nil {
		return uuid.Nil, err
	}
	tags, err := checkTags(input.Tags)
	if err != nil {
		return uuid.Nil, err
	}
	body := utils.OrEmpty(input.Body)
	id, err := self.repo.Discussion().Create(ctx, tx, discussion_repo.CreateInput{
		Workspace: actor.Workspace, SlugID: utils.SlugID(), Title: title, Body: body, Team: team, Creator: actor.User,
		Thumbnail: utils.Trimmed(input.ThumbnailURL.Ptr()), Tags: tags,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.subscribe(ctx, tx, id, actor.User); err != nil {
		return uuid.Nil, err
	}
	if err := self.setFlags(ctx, tx, id, input); err != nil {
		return uuid.Nil, err
	}
	return id, self.notifyMentions(ctx, tx, actor, id, title, "", body)
}

func (self *DiscussionService) update(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input *SaveInput) error {
	current, err := self.repo.Discussion().GetForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	rewrites := input.Title != nil || input.Body != nil || input.Patch != nil || input.Team.Set || input.ThumbnailURL.Set
	isAuthor := current.CreatorID != nil && *current.CreatorID == actor.User
	if rewrites && !isAuthor && actor.Role < auth.Admin {
		return errdefs.Forbidden("Only its author or a workspace admin can change a discussion's title, body, thumbnail or team")
	}
	if input.Closed != nil && !mayClose(actor, current) {
		return errdefs.Forbidden("Only the person who started a discussion can close or reopen it")
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	title := current.Title
	if input.Title != nil {
		if title, err = utils.NonEmpty(*input.Title, "title"); err != nil {
			return err
		}
	}
	body := current.Body
	replaced, err := args.NewContent(current.Body, input.Body, input.Patch)
	if err != nil {
		return err
	}
	if replaced != nil {
		body = *replaced
	}
	if title != current.Title || body != current.Body {
		editedAt, err := self.markEdited(ctx, tx, current, body, now)
		if err != nil {
			return err
		}
		if err := self.repo.Discussion().SetText(ctx, tx, id, title, body, editedAt); err != nil {
			return err
		}
		if err := self.notifyMentions(ctx, tx, actor, id, title, current.Body, body); err != nil {
			return err
		}
	}
	if input.Team.Set {
		team, err := self.resolve.OptionalTeam(ctx, actor, input.Team.Ptr())
		if err != nil {
			return err
		}
		if err := self.repo.Discussion().SetTeam(ctx, tx, id, team); err != nil {
			return err
		}
	}
	if input.ThumbnailURL.Set {
		if err := self.repo.Discussion().SetThumbnail(ctx, tx, id, utils.Trimmed(input.ThumbnailURL.Ptr())); err != nil {
			return err
		}
	}
	if input.Tags != nil {
		tags, err := checkTags(input.Tags)
		if err != nil {
			return err
		}
		if err := self.repo.Discussion().SetTags(ctx, tx, id, tags); err != nil {
			return err
		}
	}
	if err := self.setFlags(ctx, tx, id, input); err != nil {
		return err
	}
	return self.repo.Discussion().Touch(ctx, tx, id, now)
}

// mayClose is whether the actor started the discussion, or is an admin once its starter is gone.
func mayClose(actor *auth.Actor, d *ent.Discussion) bool {
	if d.CreatorID == nil {
		return actor.Role >= auth.Admin
	}
	return *d.CreatorID == actor.User
}

// editingSession is how long after an edit further edits count as the same one.
const editingSession = 10 * time.Minute

// markEdited is when the body was edited (nil when it stays), keeping the body it replaces unless
// that was written in the same editing session.
func (self *DiscussionService) markEdited(ctx context.Context, tx repository.TxInterface, current *ent.Discussion, body string, now time.Time) (*time.Time, error) {
	if body == current.Body {
		return nil, nil
	}
	if current.EditedAt != nil && now.Sub(*current.EditedAt) <= editingSession {
		return &now, nil
	}
	written := current.CreatedAt
	if current.EditedAt != nil {
		written = *current.EditedAt
	}
	return &now, self.repo.Discussion().AddVersion(ctx, tx, current.WorkspaceID, current.ID, current.Body, written)
}

type VoteInput struct {
	ID     string `json:"id" doc:"The discussion's id, slug, link or title."`
	Upvote bool   `json:"upvote" doc:"true to upvote it, false to take your upvote back."`
}

func (self *DiscussionService) Vote(ctx context.Context, actor *auth.Actor, input *VoteInput) (models.DiscussionDetail, error) {
	if err := actor.RequireMember(); err != nil {
		return models.DiscussionDetail{}, err
	}
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	var value models.DiscussionDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		changed, err := self.repo.Discussion().SetVote(ctx, tx, id, actor.User, input.Upvote)
		if err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, actor.User, id); err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return self.emit(ctx, tx, actor, "update", value.Discussion)
	})
	return value, err
}

func (self *DiscussionService) History(ctx context.Context, actor *auth.Actor, input *IDInput) (models.MessageHistory, error) {
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return models.MessageHistory{}, err
	}
	found, err := self.repo.Discussion().Get(ctx, nil, id)
	if err != nil {
		return models.MessageHistory{}, err
	}
	versions, err := self.repo.Discussion().Versions(ctx, nil, id)
	if err != nil {
		return models.MessageHistory{}, err
	}
	return models.MessageHistoryOf(versions, found.Body, found.EditedAt, found.CreatedAt), nil
}

// setFlags pins or closes the discussion, or takes that back, as asked.
func (self *DiscussionService) setFlags(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input *SaveInput) error {
	if input.Pinned == nil && input.Closed == nil {
		return nil
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	at := func(on bool) *time.Time {
		if !on {
			return nil
		}
		return &now
	}
	if input.Pinned != nil {
		if err := self.repo.Discussion().SetPinned(ctx, tx, id, at(*input.Pinned)); err != nil {
			return err
		}
	}
	if input.Closed != nil {
		return self.repo.Discussion().SetClosed(ctx, tx, id, at(*input.Closed))
	}
	return nil
}

func (self *DiscussionService) changeSubscribers(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input *SaveInput) error {
	added, err := self.resolve.Users(ctx, actor, input.AddSubscribers)
	if err != nil {
		return err
	}
	removed, err := self.resolve.Users(ctx, actor, input.RemoveSubscribers)
	if err != nil {
		return err
	}
	if err := self.subscribe(ctx, tx, id, added...); err != nil {
		return err
	}
	if len(removed) == 0 {
		return nil
	}
	return self.repo.Discussion().Unsubscribe(ctx, tx, id, removed)
}

// notifyMentions subscribes and notifies the people newly mentioned in the body.
func (self *DiscussionService) notifyMentions(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, title, before, after string) error {
	mentioned, err := self.newlyMentioned(ctx, actor, before, after)
	if err != nil || len(mentioned) == 0 {
		return err
	}
	if err := self.subscribe(ctx, tx, id, mentioned...); err != nil {
		return err
	}
	name, err := self.actorName(ctx, actor)
	if err != nil {
		return err
	}
	return self.events.Notify(ctx, tx, actor, mentioned, events_service.Notice{
		Kind: "discussionMention", Summary: fmt.Sprintf("%s mentioned you in %s", name, title), Discussion: &id,
	})
}

func (self *DiscussionService) setTrashed(ctx context.Context, actor *auth.Actor, id uuid.UUID, trashed bool) (models.DiscussionDetail, error) {
	var value models.DiscussionDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Discussion().SetTrashed(ctx, tx, id, trashed, now); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, actor.User, id); err != nil {
			return err
		}
		action := "update"
		if trashed {
			action = "remove"
		}
		return self.emit(ctx, tx, actor, action, value.Discussion)
	})
	return value, err
}

// checkOwner refuses anyone but the discussion's author and admins.
func (self *DiscussionService) checkOwner(ctx context.Context, actor *auth.Actor, id uuid.UUID) error {
	if actor.Role >= auth.Admin {
		return nil
	}
	found, err := self.repo.Discussion().Get(ctx, nil, id)
	if err != nil {
		return err
	}
	if found.CreatorID == nil || *found.CreatorID != actor.User {
		return errdefs.Forbidden("Only its author or a workspace admin can trash or restore a discussion")
	}
	return nil
}

func (self *DiscussionService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DiscussionDetail, error) {
	id, err := self.find(ctx, actor, input.ID, false)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	if err := self.checkOwner(ctx, actor, id); err != nil {
		return models.DiscussionDetail{}, err
	}
	return self.setTrashed(ctx, actor, id, true)
}

func (self *DiscussionService) Restore(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DiscussionDetail, error) {
	id, err := self.find(ctx, actor, input.ID, true)
	if err != nil {
		return models.DiscussionDetail{}, err
	}
	if err := self.checkOwner(ctx, actor, id); err != nil {
		return models.DiscussionDetail{}, err
	}
	return self.setTrashed(ctx, actor, id, false)
}

const (
	maxTags      = 10
	maxTagLength = 32
)

func normalTag(tag string) string {
	return strings.ToLower(strings.Join(strings.Fields(tag), " "))
}

func checkTags(tags []string) ([]string, error) {
	checked := []string{}
	for _, tag := range tags {
		tag = normalTag(tag)
		if tag == "" || slices.Contains(checked, tag) {
			continue
		}
		if len([]rune(tag)) > maxTagLength {
			return nil, errdefs.BadRequest("tag %q is longer than %d characters", tag, maxTagLength)
		}
		checked = append(checked, tag)
	}
	if len(checked) > maxTags {
		return nil, errdefs.BadRequest("A discussion takes up to %d tags", maxTags)
	}
	return checked, nil
}

type TagsInput struct {
	args.Page
}

func (self *DiscussionService) Tags(ctx context.Context, actor *auth.Actor, input *TagsInput) (args.PageOf[models.DiscussionTag], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.DiscussionTag]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.DiscussionTag]{}, err
	}
	tags, err := self.repo.Discussion().Tags(ctx, actor.Workspace, teams, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.DiscussionTag]{}, err
	}
	return args.WrapOf(input.Page, tags)
}
