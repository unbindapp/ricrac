package comment_service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	comment_repo "ricrac/internal/repositories/comment"
	events_service "ricrac/internal/services/events"
)

type ListInput struct {
	targetInput
	Limit  *int64  `json:"limit" doc:"How many threads to return, 50 by default, at most 250."`
	Cursor *string `json:"cursor" doc:"The endCursor of the previous page."`
}

type SaveInput struct {
	ID   *string `json:"id" doc:"Only to edit one of your comments: its id. Omit to create a comment."`
	Body *string `json:"body" doc:"Markdown. Mention people with @displayName, @email or their linked GitHub login; they are notified."`
	targetInput
	ParentID      *string `json:"parentId" doc:"The comment to reply to; the reply joins its thread."`
	QuotedText    *string `json:"quotedText" doc:"The passage of the document or description the comment is about."`
	SuggestedText *string `json:"suggestedText" doc:"With documentId and quotedText: text to put in place of the quoted passage, which must occur once in the document. accept_suggestion applies it."`
}

type IDInput struct {
	ID string `json:"id"`
}

func (self *CommentService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.CommentThread], error) {
	if actor.Visitor && input.IssueID == nil {
		return args.PageOf[models.CommentThread]{}, errdefs.Unauthorized()
	}
	target, err := self.namedTarget(ctx, actor, input.targetInput)
	if err != nil {
		return args.PageOf[models.CommentThread]{}, err
	}
	if target == nil {
		return args.PageOf[models.CommentThread]{}, errdefs.BadRequest("Pass one of %s", targetNames)
	}
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	offset, err := page.Offset()
	if err != nil {
		return args.PageOf[models.CommentThread]{}, err
	}
	comments, err := self.repo.Comment().Threads(ctx, nil, target.Column, target.ID, new(page.Size()+1), offset)
	if err != nil {
		return args.PageOf[models.CommentThread]{}, err
	}
	threads := make([]models.CommentThread, len(comments))
	for i, c := range comments {
		threads[i] = models.CommentThreadOf(c)
	}
	return args.WrapOf(page, threads)
}

func (self *CommentService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.CommentDetail, error) {
	body, err := utils.NonEmpty(utils.OrEmpty(input.Body), "body")
	if err != nil {
		return models.CommentDetail{}, err
	}
	target, err := self.namedTarget(ctx, actor, input.targetInput)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if input.ID != nil {
		if target != nil || input.ParentID != nil || input.QuotedText != nil || input.SuggestedText != nil {
			return models.CommentDetail{}, errdefs.BadRequest("Only the body of a comment can be changed")
		}
		return self.edit(ctx, actor, *input.ID, body)
	}
	var root *uuid.UUID
	if input.ParentID != nil {
		parent, err := self.Load(ctx, actor, *input.ParentID)
		if err != nil {
			return models.CommentDetail{}, err
		}
		if parent.DeletedAt != nil {
			return models.CommentDetail{}, errDeleted()
		}
		parentTarget, err := TargetOf(parent)
		if err != nil {
			return models.CommentDetail{}, err
		}
		if target != nil && *target != parentTarget {
			return models.CommentDetail{}, errdefs.BadRequest("The parent comment is on something else")
		}
		target = &parentTarget
		root = parent.ParentID
		if root == nil {
			root = &parent.ID
		}
	}
	if target == nil {
		return models.CommentDetail{}, errdefs.BadRequest("Pass what to comment on (%s) or parentId", targetNames)
	}
	var quoted *string
	if input.QuotedText != nil && strings.TrimSpace(*input.QuotedText) != "" {
		quoted = new(strings.TrimSpace(*input.QuotedText))
	}
	if input.SuggestedText != nil {
		if err := self.checkSuggestion(ctx, *target, root, quoted); err != nil {
			return models.CommentDetail{}, err
		}
	}
	var value models.CommentDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		_, value, err = self.create(ctx, tx, actor, newComment{target: *target, root: root, body: body, quoted: quoted, suggested: input.SuggestedText})
		return err
	})
	return value, err
}

// checkSuggestion refuses a suggestion that isn't a new thread on a document quoting a passage it
// has once.
func (self *CommentService) checkSuggestion(ctx context.Context, target Target, root *uuid.UUID, quoted *string) error {
	if target.Column != "document_id" || root != nil {
		return errdefs.BadRequest("Only a new comment on a document suggests text; pass documentId, quotedText and suggestedText")
	}
	if quoted == nil {
		return errdefs.BadRequest("Pass quotedText: the passage suggestedText replaces")
	}
	content, err := self.repo.Comment().DocumentContent(ctx, nil, target.ID)
	if err != nil {
		return err
	}
	return args.CheckPassage(content, *quoted)
}

// Create adds a comment (a reply when root is set) by the actor, notifying and subscribing the
// people it concerns.
func (self *CommentService) Create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, target Target, root *uuid.UUID, body string, quoted *string) (uuid.UUID, models.CommentDetail, error) {
	return self.create(ctx, tx, actor, newComment{target: target, root: root, body: body, quoted: quoted})
}

type newComment struct {
	target            Target
	root              *uuid.UUID
	body              string
	quoted, suggested *string
}

func (self *CommentService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, c newComment) (uuid.UUID, models.CommentDetail, error) {
	target, root, body := c.target, c.root, c.body
	id, err := self.repo.Comment().Create(ctx, tx, comment_repo.CreateInput{
		Workspace: actor.Workspace, Column: target.Column, Target: target.ID, Root: root, User: actor.ActorID(), Body: body,
		Quoted: c.quoted, Suggested: c.suggested,
	})
	if err != nil {
		return uuid.Nil, models.CommentDetail{}, err
	}
	about, err := self.about(ctx, tx, target)
	if err != nil {
		return uuid.Nil, models.CommentDetail{}, err
	}
	mentioned, err := self.events.Mentions(ctx, actor, body)
	if err != nil {
		return uuid.Nil, models.CommentDetail{}, err
	}
	var participants []uuid.UUID
	if root != nil {
		if participants, err = self.repo.Comment().Participants(ctx, tx, *root); err != nil {
			return uuid.Nil, models.CommentDetail{}, err
		}
	}
	audience, err := self.audience(ctx, tx, actor, target, id, mentioned)
	if err != nil {
		return uuid.Nil, models.CommentDetail{}, err
	}
	name, err := self.actorName(ctx, actor)
	if err != nil {
		return uuid.Nil, models.CommentDetail{}, err
	}
	replied := slices.DeleteFunc(participants, func(u uuid.UUID) bool { return slices.Contains(mentioned, u) })
	others := slices.DeleteFunc(audience, func(u uuid.UUID) bool {
		return slices.Contains(mentioned, u) || slices.Contains(replied, u)
	})
	commented := fmt.Sprintf("%s commented on %s", name, about.label)
	if c.suggested != nil {
		about.kinds[0], commented = "documentSuggestion", fmt.Sprintf("%s suggested an edit to %s", name, about.label)
	}
	for _, n := range []struct {
		users   []uuid.UUID
		kind    string
		summary string
	}{
		{mentioned, about.kinds[2], fmt.Sprintf("%s mentioned you in a comment on %s", name, about.label)},
		{replied, about.kinds[1], fmt.Sprintf("%s replied to a thread on %s", name, about.label)},
		{others, about.kinds[0], commented},
	} {
		notice := about.notice
		notice.Kind, notice.Summary, notice.Comment = n.kind, n.summary, &id
		if err := self.events.Notify(ctx, tx, actor, n.users, notice); err != nil {
			return uuid.Nil, models.CommentDetail{}, err
		}
	}
	value, err := self.detail(ctx, tx, id)
	if err != nil {
		return uuid.Nil, models.CommentDetail{}, err
	}
	return id, value, self.events.Emit(ctx, tx, actor, "Comment", "create", value, nil, about.team)
}

// audience is who hears about a new comment besides the mentioned and the thread: an issue's or
// discussion's subscribers (subscribing the actor and the mentioned first), a document's creator
// or a status update's author.
func (self *CommentService) audience(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, target Target, comment uuid.UUID, mentioned []uuid.UUID) ([]uuid.UUID, error) {
	switch target.Column {
	case "issue_id":
		if err := self.events.Subscribe(ctx, tx, target.ID, append([]uuid.UUID{actor.User}, mentioned...)); err != nil {
			return nil, err
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return nil, err
		}
		if err := self.repo.Comment().TouchIssue(ctx, tx, target.ID, now); err != nil {
			return nil, err
		}
		audience, err := self.events.IssueSubscribers(ctx, tx, target.ID)
		if err != nil {
			return nil, err
		}
		return audience, self.events.QueueCommentForGitHub(ctx, tx, actor, target.ID, comment, "comment_created")
	case "discussion_id", "discussion_thread_id":
		discussion, err := self.discussionOf(ctx, tx, target)
		if err != nil {
			return nil, err
		}
		return self.discussionAudience(ctx, tx, actor, discussion, mentioned)
	case "document_id":
		creator, err := self.repo.Comment().DocumentCreator(ctx, tx, target.ID)
		if err != nil || creator == nil {
			return nil, err
		}
		return []uuid.UUID{*creator}, nil
	case "status_update_id":
		update, err := self.repo.Comment().StatusUpdate(ctx, tx, target.ID)
		if err != nil || update == nil || update.UserID == nil {
			return nil, err
		}
		return []uuid.UUID{*update.UserID}, nil
	}
	return nil, nil
}

// discussionOf is the discussion a comment on a discussion or one of its threads is in.
func (self *CommentService) discussionOf(ctx context.Context, tx repository.TxInterface, target Target) (uuid.UUID, error) {
	if target.Column == "discussion_id" {
		return target.ID, nil
	}
	thread, err := self.repo.Discussion().Thread(ctx, tx, target.ID)
	if err != nil {
		return uuid.Nil, err
	}
	return thread.DiscussionID, nil
}

// discussionAudience subscribes the actor and the mentioned to the discussion, marks it active and
// answers its subscribers.
func (self *CommentService) discussionAudience(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, discussion uuid.UUID, mentioned []uuid.UUID) ([]uuid.UUID, error) {
	users := slices.DeleteFunc(append([]uuid.UUID{actor.User}, mentioned...), func(u uuid.UUID) bool { return u == uuid.Nil })
	if err := self.repo.Discussion().Subscribe(ctx, tx, discussion, users); err != nil {
		return nil, err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := self.repo.Discussion().Bump(ctx, tx, discussion, now); err != nil {
		return nil, err
	}
	return self.repo.Discussion().SubscriberIDs(ctx, tx, discussion)
}

func (self *CommentService) edit(ctx context.Context, actor *auth.Actor, reference, body string) (models.CommentDetail, error) {
	row, err := self.Load(ctx, actor, reference)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if row.DeletedAt != nil {
		return models.CommentDetail{}, errDeleted()
	}
	if row.UserID == nil || *row.UserID != actor.User {
		return models.CommentDetail{}, errdefs.Forbidden("Only its author can edit a comment")
	}
	var value models.CommentDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err = self.Change(ctx, tx, actor, row, body)
		return err
	})
	return value, err
}

// Change rewrites a comment's body, notifying people newly mentioned.
func (self *CommentService) Change(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Comment, body string) (models.CommentDetail, error) {
	before, err := self.detail(ctx, tx, row.ID)
	if err != nil {
		return models.CommentDetail{}, err
	}
	target, err := TargetOf(row)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if inForum(target) && body != row.Body {
		written := row.CreatedAt
		if row.EditedAt != nil {
			written = *row.EditedAt
		}
		if err := self.repo.Comment().AddVersion(ctx, tx, row.WorkspaceID, row.ID, row.Body, written); err != nil {
			return models.CommentDetail{}, err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if err := self.repo.Comment().SetBody(ctx, tx, row.ID, body, now); err != nil {
		return models.CommentDetail{}, err
	}
	previous, err := self.events.Mentions(ctx, actor, row.Body)
	if err != nil {
		return models.CommentDetail{}, err
	}
	current, err := self.events.Mentions(ctx, actor, body)
	if err != nil {
		return models.CommentDetail{}, err
	}
	mentioned := slices.DeleteFunc(current, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	about, err := self.about(ctx, tx, target)
	if err != nil {
		return models.CommentDetail{}, err
	}
	if inForum(target) {
		discussion, err := self.discussionOf(ctx, tx, target)
		if err != nil {
			return models.CommentDetail{}, err
		}
		if err := self.repo.Discussion().Subscribe(ctx, tx, discussion, mentioned); err != nil {
			return models.CommentDetail{}, err
		}
	}
	if target.Column == "issue_id" {
		if err := self.events.Subscribe(ctx, tx, target.ID, mentioned); err != nil {
			return models.CommentDetail{}, err
		}
		if err := self.events.QueueCommentForGitHub(ctx, tx, actor, target.ID, row.ID, "comment_edited"); err != nil {
			return models.CommentDetail{}, err
		}
	}
	name, err := self.actorName(ctx, actor)
	if err != nil {
		return models.CommentDetail{}, err
	}
	notice := about.notice
	notice.Kind = about.kinds[2]
	notice.Summary = fmt.Sprintf("%s mentioned you in a comment on %s", name, about.label)
	notice.Comment = &row.ID
	if err := self.events.Notify(ctx, tx, actor, mentioned, notice); err != nil {
		return models.CommentDetail{}, err
	}
	value, err := self.detail(ctx, tx, row.ID)
	if err != nil {
		return models.CommentDetail{}, err
	}
	updatedFrom := events_service.M{"body": before.Body, "editedAt": before.EditedAt}
	return value, self.events.Emit(ctx, tx, actor, "Comment", "update", value, updatedFrom, about.team)
}

// about is how notifications about comments on a target read and what they point at.
type about struct {
	// The kinds for a new comment, a reply in your thread, and a mention.
	kinds  [3]string
	label  string
	notice events_service.Notice
	team   *uuid.UUID
}

func (self *CommentService) about(ctx context.Context, tx repository.TxInterface, target Target) (about, error) {
	id := target.ID
	projectKinds := [3]string{"projectNewComment", "projectCommentReply", "projectCommentMention"}
	discussionKinds := [3]string{"discussionNewComment", "discussionCommentReply", "discussionCommentMention"}
	switch target.Column {
	case "issue_id":
		name, err := self.repo.Issue().Identifier(ctx, tx, id)
		if err != nil {
			return about{}, err
		}
		team, err := self.repo.Comment().IssueTeam(ctx, tx, id)
		return about{[3]string{"issueNewComment", "issueCommentReply", "issueCommentMention"}, name, events_service.Notice{Issue: &id}, team}, err
	case "project_id":
		name, err := self.repo.Comment().ProjectName(ctx, tx, id)
		return about{projectKinds, name, events_service.Notice{Project: &id}, nil}, err
	case "milestone_id":
		milestone, err := self.repo.Comment().Milestone(ctx, tx, id)
		if err != nil {
			return about{}, err
		}
		project := milestone.Edges.Project
		if project == nil {
			return about{projectKinds, milestone.Name, events_service.Notice{}, nil}, nil
		}
		return about{projectKinds, milestone.Name + " (" + project.Name + ")", events_service.Notice{Project: &project.ID}, nil}, nil
	case "initiative_id":
		name, err := self.repo.Comment().InitiativeName(ctx, tx, id)
		return about{[3]string{"initiativeNewComment", "initiativeCommentReply", "initiativeCommentMention"}, name, events_service.Notice{Initiative: &id}, nil}, err
	case "document_id":
		name, err := self.repo.Comment().DocumentTitle(ctx, tx, id)
		return about{[3]string{"documentNewComment", "documentCommentReply", "documentCommentMention"}, name, events_service.Notice{Document: &id}, nil}, err
	case "discussion_id":
		discussion, err := self.repo.Discussion().Get(ctx, tx, id)
		if err != nil {
			return about{}, err
		}
		return about{discussionKinds, discussion.Title, events_service.Notice{Discussion: &id}, discussion.TeamID}, nil
	case "discussion_thread_id":
		thread, err := self.repo.Discussion().Thread(ctx, tx, id)
		if err != nil {
			return about{}, err
		}
		discussion := thread.Edges.Discussion
		label := fmt.Sprintf("%s in %s", thread.Title, discussion.Title)
		notice := events_service.Notice{Discussion: &discussion.ID, DiscussionThread: &id}
		return about{discussionKinds, label, notice, discussion.TeamID}, nil
	}
	update, err := self.repo.Comment().StatusUpdate(ctx, tx, id)
	if err != nil || update == nil {
		return about{}, err
	}
	name := ""
	kinds := [3]string{"initiativeUpdateNewComment", "initiativeUpdateCommentReply", "initiativeUpdateCommentMention"}
	switch {
	case update.Edges.Project != nil:
		kinds = [3]string{"projectUpdateNewComment", "projectUpdateCommentReply", "projectUpdateCommentMention"}
		name = update.Edges.Project.Name
	case update.Edges.Initiative != nil:
		name = update.Edges.Initiative.Name
	}
	notice := events_service.Notice{StatusUpdate: &id, Project: update.ProjectID, Initiative: update.InitiativeID}
	return about{kinds, "the update on " + name, notice, nil}, nil
}

func (self *CommentService) actorName(ctx context.Context, actor *auth.Actor) (string, error) {
	if actor.User == uuid.Nil {
		return "Someone on GitHub", nil
	}
	return self.repo.Comment().UserName(ctx, actor.User)
}
