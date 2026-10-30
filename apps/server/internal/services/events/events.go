// Package events_service writes what happens around a change: webhook deliveries, issue history,
// inbox notifications, subscriptions, the audit log and changes queued for GitHub. Everything is
// written in the change's transaction, so it only exists if the change does.
package events_service

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
	events_repo "ricrac/internal/repositories/events"
	"ricrac/internal/repositories/repositories"
)

type M = map[string]any

// Waker wakes the background senders when there is work for them.
type Waker interface {
	WakeDeliveries()
	WakeGitHub()
}

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i EventsServiceInterface -p events_service -s EventsService -o events_service_iface.go
type EventsService struct {
	repo  repositories.RepositoriesInterface
	waker Waker
}

func NewEventsService(repo repositories.RepositoriesInterface, waker Waker) *EventsService {
	return &EventsService{repo: repo, waker: waker}
}

func (self *EventsService) events() events_repo.EventsRepositoryInterface {
	return self.repo.Events()
}

// Emit queues a webhook delivery for every enabled webhook of the workspace that listens to
// kind (Issue, Comment, Project, ...) on this team, or on every team. data is the thing as the
// API answers it; updatedFrom the fields it changed, with their previous values.
func (self *EventsService) Emit(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, kind, action string, data any, updatedFrom M, team *uuid.UUID) error {
	hooks, err := self.events().Webhooks(ctx, tx, actor.Workspace, kind, team)
	if err != nil || len(hooks) == 0 {
		return err
	}
	var url any
	if object, ok := asObject(data); ok {
		url = object["url"]
	}
	for _, hook := range hooks {
		payload := M{
			"action":      action,
			"type":        kind,
			"data":        data,
			"url":         url,
			"actor":       M{"id": actor.ActorID()},
			"createdAt":   time.Now().UTC(),
			"workspaceId": actor.Workspace,
			"webhookId":   hook,
		}
		if updatedFrom != nil {
			payload["updatedFrom"] = updatedFrom
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := self.events().QueueDelivery(ctx, tx, hook, encoded); err != nil {
			return err
		}
	}
	self.waker.WakeDeliveries()
	return nil
}

// History adds an entry to an issue's history. changes maps fields to {from, to}; a change from
// GitHub also has github: {label, url}. Changes to issues linked to GitHub issues are queued for
// GitHub.
func (self *EventsService) History(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue uuid.UUID, action string, changes M) error {
	if actor.Origin != nil {
		changes = maps.Clone(changes)
		changes["github"] = M{"label": actor.Origin.Label, "url": actor.Origin.URL}
	}
	encoded, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	if err := self.events().AddHistory(ctx, tx, issue, actor.ActorID(), action, encoded); err != nil {
		return err
	}
	return self.queueIssueForGitHub(ctx, tx, actor, issue, changes, encoded)
}

// pushedFields are the issue fields GitHub has too.
var pushedFields = []string{"title", "description", "state", "labels", "assignee", "projectMilestone", "priority", "estimate", "dueDate", "cycle"}

func mirrorsGitHub(actor *auth.Actor) bool {
	return actor.Origin != nil && actor.Origin.FromGitHubIssue
}

func (self *EventsService) queueIssueForGitHub(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue uuid.UUID, changes M, encoded json.RawMessage) error {
	pushed := slices.ContainsFunc(pushedFields, func(field string) bool { _, ok := changes[field]; return ok })
	if mirrorsGitHub(actor) || !pushed {
		return nil
	}
	queued, err := self.events().QueueIssueForGitHub(ctx, tx, issue, encoded)
	if err != nil {
		return err
	}
	if queued {
		self.waker.WakeGitHub()
	}
	return nil
}

// QueueCommentForGitHub queues a new comment on a linked issue (kind comment_created), or the
// edit or deletion of a comment the app posted.
func (self *EventsService) QueueCommentForGitHub(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue, comment uuid.UUID, kind string) error {
	if mirrorsGitHub(actor) {
		return nil
	}
	queued, err := self.events().QueueCommentForGitHub(ctx, tx, issue, comment, kind)
	if err != nil {
		return err
	}
	if queued {
		self.waker.WakeGitHub()
	}
	return nil
}

// QueueMilestoneForGitHub queues a milestone's change (kind milestone) or deletion
// (milestone_deleted) for each GitHub milestone it is linked to.
func (self *EventsService) QueueMilestoneForGitHub(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, milestone uuid.UUID, kind string) error {
	if mirrorsGitHub(actor) {
		return nil
	}
	queued, err := self.events().QueueMilestoneForGitHub(ctx, tx, milestone, kind, nil)
	if err != nil {
		return err
	}
	if queued {
		self.waker.WakeGitHub()
	}
	return nil
}

// QueueNewIssueForGitHub queues an issue created in Ricrac for the team's synced repository it
// opens in on GitHub (see save_issue's githubRepository), if any.
func (self *EventsService) QueueNewIssueForGitHub(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue, team uuid.UUID, repository *string) error {
	if mirrorsGitHub(actor) || repository != nil && utils.IsNoneWord(*repository) {
		return nil
	}
	var actAs *uuid.UUID
	if actor.User != uuid.Nil {
		actAs = &actor.User
	}
	queued, err := self.events().QueueNewIssueForGitHub(ctx, tx, issue, team, repository, actAs)
	if err != nil {
		return err
	}
	if queued {
		self.waker.WakeGitHub()
		return nil
	}
	if repository == nil {
		return nil
	}
	names, err := self.events().IssueCreatingRepos(ctx, tx, team)
	if err != nil {
		return err
	}
	return errdefs.BadRequest("No synced repository %q opens the team's issues on GitHub. %s", *repository, utils.Options(names))
}

func (self *EventsService) Audit(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, kind string, metadata any) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return self.events().AddAudit(ctx, tx, actor.Workspace, actor.ActorID(), kind, encoded)
}

// Notice is what an inbox notification is about. Unset targets stay empty.
type Notice struct {
	// issueAssignedToYou, issueMention, issueComment, issueCommentReply, issueStatusChanged,
	// issueNewComment, issuePriorityChanged, issueBlocking, issueUnblocked, issueDue,
	// issueCreated (triage), issueSubscribed, projectUpdateCreated, documentMention,
	// discussionNewThread, ...
	Kind             string
	Summary          string
	Issue            *uuid.UUID
	Comment          *uuid.UUID
	Project          *uuid.UUID
	Initiative       *uuid.UUID
	Document         *uuid.UUID
	StatusUpdate     *uuid.UUID
	Discussion       *uuid.UUID
	DiscussionThread *uuid.UUID
}

// Notify notifies each of users once, except the actor, who doesn't need to hear about their
// own change, and anyone no longer in the workspace.
func (self *EventsService) Notify(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, users []uuid.UUID, notice Notice) error {
	var seen []uuid.UUID
	for _, user := range users {
		if user == actor.User || slices.Contains(seen, user) {
			continue
		}
		seen = append(seen, user)
		member, err := self.events().IsActiveMember(ctx, tx, actor.Workspace, user)
		if err != nil {
			return err
		}
		if !member {
			continue
		}
		err = self.events().AddNotification(ctx, tx, events_repo.Notification{
			Workspace: actor.Workspace, User: user, Actor: actor.ActorID(), Kind: notice.Kind, Summary: notice.Summary,
			Issue: notice.Issue, Comment: notice.Comment, Project: notice.Project, Initiative: notice.Initiative,
			Document: notice.Document, StatusUpdate: notice.StatusUpdate, Discussion: notice.Discussion,
			DiscussionThread: notice.DiscussionThread,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (self *EventsService) IssueSubscribers(ctx context.Context, tx repository.TxInterface, issue uuid.UUID) ([]uuid.UUID, error) {
	return self.events().IssueSubscribers(ctx, tx, issue)
}

func (self *EventsService) Subscribe(ctx context.Context, tx repository.TxInterface, issue uuid.UUID, users []uuid.UUID) error {
	users = slices.DeleteFunc(slices.Clone(users), func(u uuid.UUID) bool { return u == uuid.Nil })
	return self.events().Subscribe(ctx, tx, issue, users)
}

// Mentions is the members mentioned in Markdown as @displayName, @email or @ + the part of their
// email before the @.
func (self *EventsService) Mentions(ctx context.Context, actor *auth.Actor, text string) ([]uuid.UUID, error) {
	handles := utils.MentionHandles(text)
	if len(handles) == 0 {
		return nil, nil
	}
	return self.events().Mentioned(ctx, actor.Workspace, handles)
}

// ChangedFields is the fields of before (as JSON) that after changed, with their values in
// before; updatedAt doesn't count.
func ChangedFields(before, after any) M {
	from, _ := asObject(before)
	to, _ := asObject(after)
	changed := M{}
	for key, value := range from {
		if key == "updatedAt" {
			continue
		}
		if next, ok := to[key]; !ok || string(errdefs.Marshal(next)) != string(errdefs.Marshal(value)) {
			changed[key] = value
		}
	}
	return changed
}

func asObject(value any) (M, bool) {
	if object, ok := value.(M); ok {
		return object, true
	}
	var object M
	if err := json.Unmarshal(errdefs.Marshal(value), &object); err != nil || object == nil {
		return nil, false
	}
	return object, true
}
