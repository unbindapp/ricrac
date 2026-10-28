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

type SaveThreadInput struct {
	ID         *string               `json:"id" doc:"Only to update a thread: its id. Omit to start one."`
	Discussion *string               `json:"discussion" doc:"To start a thread: the discussion's id, slug, link or title."`
	Kind       *string               `json:"kind" enum:"bug,design,question,idea,task,general" doc:"What the thread is about; general by default."`
	Title      *string               `json:"title"`
	Body       *string               `json:"body" doc:"Markdown. Mention people with @displayName or @email; they are notified and subscribed to the discussion."`
	Patch      []args.TextEdit       `json:"patch" doc:"Edits to the current body instead of the whole text, applied in order, all or none."`
	Issue      args.Nullable[string] `json:"issue" doc:"The issue tracking it (ENG-123, id or link); null unlinks it."`
	Resolved   *bool                 `json:"resolved" doc:"Resolve the thread, or reopen it with false."`
}

var threadKinds = []string{"bug", "design", "question", "idea", "task", "general"}

func checkKind(kind *string) (*string, error) {
	if kind == nil {
		return nil, nil
	}
	value := strings.ToLower(strings.TrimSpace(*kind))
	if !slices.Contains(threadKinds, value) {
		return nil, errdefs.BadRequest("kind %q isn't one of %s", *kind, strings.Join(threadKinds, ", "))
	}
	return &value, nil
}

// thread is a thread of a discussion the caller can see.
func (self *DiscussionService) thread(ctx context.Context, actor *auth.Actor, value string) (*ent.DiscussionThread, error) {
	notFound := errdefs.NotFound("No discussion thread %q", strings.TrimSpace(value))
	id, ok := utils.AsUUID(utils.LastSegment(value))
	if !ok {
		return nil, notFound
	}
	found, err := self.repo.Discussion().FindThread(ctx, actor.Workspace, id)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, notFound
	}
	visible, err := self.resolve.CanSeeDiscussion(ctx, actor, found.DiscussionID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, notFound
	}
	return found, nil
}

func (self *DiscussionService) threadValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.DiscussionThreadDetail, error) {
	found, err := self.repo.Discussion().Thread(ctx, tx, id)
	if err != nil {
		return models.DiscussionThreadDetail{}, err
	}
	comments, err := self.repo.Comment().Threads(ctx, tx, "discussion_thread_id", id, nil, 0)
	if err != nil {
		return models.DiscussionThreadDetail{}, err
	}
	detail := models.DiscussionThreadDetail{Comments: make([]models.CommentThread, len(comments))}
	count := 0
	for i, c := range comments {
		detail.Comments[i] = models.CommentThreadOf(c)
		for _, written := range append([]*ent.Comment{c}, c.Edges.Children...) {
			if written.DeletedAt == nil {
				count++
			}
		}
	}
	detail.DiscussionThread = models.DiscussionThreadOf(found, self.cfg.PublicURL, count)
	return detail, nil
}

func (self *DiscussionService) GetThread(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DiscussionThreadDetail, error) {
	found, err := self.thread(ctx, actor, input.ID)
	if err != nil {
		return models.DiscussionThreadDetail{}, err
	}
	return self.threadValue(ctx, nil, found.ID)
}

func (self *DiscussionService) SaveThread(ctx context.Context, actor *auth.Actor, input *SaveThreadInput) (models.DiscussionThreadDetail, error) {
	kind, err := checkKind(input.Kind)
	if err != nil {
		return models.DiscussionThreadDetail{}, err
	}
	var issue *uuid.UUID
	if input.Issue.Valid {
		id, err := self.resolve.Issue(ctx, actor, input.Issue.Value)
		if err != nil {
			return models.DiscussionThreadDetail{}, err
		}
		issue = &id
	}
	var value models.DiscussionThreadDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		action := "create"
		if input.ID != nil {
			action = "update"
			id, err = self.updateThread(ctx, tx, actor, input, kind, issue)
		} else {
			id, err = self.createThread(ctx, tx, actor, input, kind, issue)
		}
		if err != nil {
			return err
		}
		if value, err = self.threadValue(ctx, tx, id); err != nil {
			return err
		}
		return self.emitThread(ctx, tx, actor, action, value.DiscussionThread)
	})
	return value, err
}

func (self *DiscussionService) emitThread(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, action string, value models.DiscussionThread) error {
	discussion, err := self.repo.Discussion().Get(ctx, tx, uuid.MustParse(value.Discussion.ID))
	if err != nil {
		return err
	}
	return self.events.Emit(ctx, tx, actor, "DiscussionThread", action, value, nil, discussion.TeamID)
}

func (self *DiscussionService) createThread(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveThreadInput, kind *string, issue *uuid.UUID) (uuid.UUID, error) {
	if input.Discussion == nil {
		return uuid.Nil, errdefs.BadRequest("Pass discussion: the discussion to start the thread in")
	}
	if input.Resolved != nil || input.Patch != nil {
		return uuid.Nil, errdefs.BadRequest("resolved and patch only apply when updating a thread")
	}
	discussion, err := self.find(ctx, actor, *input.Discussion, false)
	if err != nil {
		return uuid.Nil, err
	}
	title, err := utils.NonEmpty(utils.OrEmpty(input.Title), "title")
	if err != nil {
		return uuid.Nil, err
	}
	if kind == nil {
		kind = new("general")
	}
	body := utils.OrEmpty(input.Body)
	id, err := self.repo.Discussion().CreateThread(ctx, tx, discussion_repo.ThreadInput{
		Workspace: actor.Workspace, Discussion: discussion, Kind: *kind, Title: title, Body: body, Issue: issue, Creator: actor.User,
	})
	if err != nil {
		return uuid.Nil, err
	}
	mentioned, err := self.events.Mentions(ctx, actor, body)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.subscribe(ctx, tx, discussion, append([]uuid.UUID{actor.User}, mentioned...)...); err != nil {
		return uuid.Nil, err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.repo.Discussion().Bump(ctx, tx, discussion, now); err != nil {
		return uuid.Nil, err
	}
	return id, self.notifyThread(ctx, tx, actor, discussion, id, title, mentioned)
}

// notifyThread tells the discussion's subscribers about a new thread, and the mentioned that they
// were.
func (self *DiscussionService) notifyThread(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, discussion, thread uuid.UUID, title string, mentioned []uuid.UUID) error {
	found, err := self.repo.Discussion().Get(ctx, tx, discussion)
	if err != nil {
		return err
	}
	subscribers, err := self.repo.Discussion().SubscriberIDs(ctx, tx, discussion)
	if err != nil {
		return err
	}
	name, err := self.actorName(ctx, actor)
	if err != nil {
		return err
	}
	notice := events_service.Notice{Discussion: &discussion, DiscussionThread: &thread}
	mention := notice
	mention.Kind, mention.Summary = "discussionMention", fmt.Sprintf("%s mentioned you in %s (%s)", name, title, found.Title)
	if err := self.events.Notify(ctx, tx, actor, mentioned, mention); err != nil {
		return err
	}
	others := slices.DeleteFunc(subscribers, func(u uuid.UUID) bool { return slices.Contains(mentioned, u) })
	notice.Kind, notice.Summary = "discussionNewThread", fmt.Sprintf("%s started a thread in %s: %s", name, found.Title, title)
	return self.events.Notify(ctx, tx, actor, others, notice)
}

func (self *DiscussionService) updateThread(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveThreadInput, kind *string, issue *uuid.UUID) (uuid.UUID, error) {
	if input.Discussion != nil {
		return uuid.Nil, errdefs.BadRequest("A thread stays in its discussion; leave discussion out when updating")
	}
	current, err := self.thread(ctx, actor, *input.ID)
	if err != nil {
		return uuid.Nil, err
	}
	rewrites := input.Title != nil || input.Body != nil || input.Patch != nil
	isAuthor := current.CreatorID != nil && *current.CreatorID == actor.User
	if rewrites && !isAuthor && actor.Role < auth.Admin {
		return uuid.Nil, errdefs.Forbidden("Only its author or a workspace admin can change a thread's title or body")
	}
	changes := discussion_repo.ThreadChanges{Kind: kind, SetIssue: input.Issue.Set, Issue: issue}
	if input.Title != nil {
		title, err := utils.NonEmpty(*input.Title, "title")
		if err != nil {
			return uuid.Nil, err
		}
		changes.Title = &title
	}
	if changes.Body, err = args.NewContent(current.Body, input.Body, input.Patch); err != nil {
		return uuid.Nil, err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.repo.Discussion().UpdateThread(ctx, tx, current.ID, changes, now); err != nil {
		return uuid.Nil, err
	}
	if changes.Body != nil {
		if err := self.notifyThreadMentions(ctx, tx, actor, current, *changes.Body); err != nil {
			return uuid.Nil, err
		}
	}
	if input.Resolved != nil && *input.Resolved != (current.ResolvedAt != nil) {
		if err := self.setResolved(ctx, tx, actor, current, *input.Resolved, now); err != nil {
			return uuid.Nil, err
		}
	}
	return current.ID, self.repo.Discussion().Bump(ctx, tx, current.DiscussionID, now)
}

func (self *DiscussionService) notifyThreadMentions(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, thread *ent.DiscussionThread, body string) error {
	mentioned, err := self.newlyMentioned(ctx, actor, thread.Body, body)
	if err != nil || len(mentioned) == 0 {
		return err
	}
	if err := self.subscribe(ctx, tx, thread.DiscussionID, mentioned...); err != nil {
		return err
	}
	found, err := self.repo.Discussion().Get(ctx, tx, thread.DiscussionID)
	if err != nil {
		return err
	}
	name, err := self.actorName(ctx, actor)
	if err != nil {
		return err
	}
	return self.events.Notify(ctx, tx, actor, mentioned, events_service.Notice{
		Kind: "discussionMention", Summary: fmt.Sprintf("%s mentioned you in %s (%s)", name, thread.Title, found.Title),
		Discussion: &thread.DiscussionID, DiscussionThread: &thread.ID,
	})
}

// setResolved resolves or reopens the thread; its author hears it was resolved.
func (self *DiscussionService) setResolved(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, thread *ent.DiscussionThread, resolved bool, now time.Time) error {
	if err := self.repo.Discussion().SetThreadResolved(ctx, tx, thread.ID, resolved, actor.User, now); err != nil {
		return err
	}
	if !resolved || thread.CreatorID == nil {
		return nil
	}
	found, err := self.repo.Discussion().Get(ctx, tx, thread.DiscussionID)
	if err != nil {
		return err
	}
	name, err := self.actorName(ctx, actor)
	if err != nil {
		return err
	}
	return self.events.Notify(ctx, tx, actor, []uuid.UUID{*thread.CreatorID}, events_service.Notice{
		Kind: "discussionThreadResolved", Summary: fmt.Sprintf("%s resolved %s in %s", name, thread.Title, found.Title),
		Discussion: &thread.DiscussionID, DiscussionThread: &thread.ID,
	})
}

func (self *DiscussionService) DeleteThread(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedDiscussionThread, error) {
	current, err := self.thread(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedDiscussionThread{}, err
	}
	if (current.CreatorID == nil || *current.CreatorID != actor.User) && actor.Role < auth.Admin {
		return models.DeletedDiscussionThread{}, errdefs.Forbidden("Only its author or a workspace admin can delete a thread")
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.threadValue(ctx, tx, current.ID)
		if err != nil {
			return err
		}
		if err := self.repo.Discussion().DeleteThread(ctx, tx, current.ID); err != nil {
			return err
		}
		return self.emitThread(ctx, tx, actor, "remove", value.DiscussionThread)
	})
	return models.DeletedDiscussionThread{Deleted: models.ID(current.ID)}, err
}
