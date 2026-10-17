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
	Body *string `json:"body" doc:"Markdown. Mention people with @displayName or @email; they are notified."`
	targetInput
	ParentID   *string `json:"parentId" doc:"The comment to reply to; the reply joins its thread."`
	QuotedText *string `json:"quotedText" doc:"The passage of the document or description the comment is about."`
}

type IDInput struct {
	ID string `json:"id"`
}

func (self *CommentService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.CommentThread], error) {
	target, err := self.namedTarget(ctx, actor, input.targetInput)
	if err != nil {
		return args.PageOf[models.CommentThread]{}, err
	}
	if target == nil {
		return args.PageOf[models.CommentThread]{}, errdefs.BadRequest("Pass one of issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId")
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
		if target != nil || input.ParentID != nil || input.QuotedText != nil {
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
		return models.CommentDetail{}, errdefs.BadRequest("Pass what to comment on (issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId) or parentId")
	}
	var quoted *string
	if input.QuotedText != nil && strings.TrimSpace(*input.QuotedText) != "" {
		quoted = new(strings.TrimSpace(*input.QuotedText))
	}
	var value models.CommentDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		_, value, err = self.Create(ctx, tx, actor, *target, root, body, quoted)
		return err
	})
	return value, err
}

// Create adds a comment (a reply when root is set) by the actor, notifying and subscribing the
// people it concerns.
func (self *CommentService) Create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, target Target, root *uuid.UUID, body string, quoted *string) (uuid.UUID, models.CommentDetail, error) {
	id, err := self.repo.Comment().Create(ctx, tx, comment_repo.CreateInput{
		Workspace: actor.Workspace, Column: target.Column, Target: target.ID, Root: root, User: actor.ActorID(), Body: body, Quoted: quoted,
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
	for _, n := range []struct {
		users   []uuid.UUID
		kind    string
		summary string
	}{
		{mentioned, about.kinds[2], fmt.Sprintf("%s mentioned you in a comment on %s", name, about.label)},
		{replied, about.kinds[1], fmt.Sprintf("%s replied to a thread on %s", name, about.label)},
		{others, about.kinds[0], fmt.Sprintf("%s commented on %s", name, about.label)},
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

// audience is who hears about a new comment besides the mentioned and the thread: an issue's
// subscribers (subscribing the actor and the mentioned first) or a status update's author.
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
	case "status_update_id":
		update, err := self.repo.Comment().StatusUpdate(ctx, tx, target.ID)
		if err != nil || update == nil || update.UserID == nil {
			return nil, err
		}
		return []uuid.UUID{*update.UserID}, nil
	}
	return nil, nil
}

func (self *CommentService) edit(ctx context.Context, actor *auth.Actor, reference, body string) (models.CommentDetail, error) {
	row, err := self.Load(ctx, actor, reference)
	if err != nil {
		return models.CommentDetail{}, err
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
	target, err := TargetOf(row)
	if err != nil {
		return models.CommentDetail{}, err
	}
	about, err := self.about(ctx, tx, target)
	if err != nil {
		return models.CommentDetail{}, err
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
		return about{projectKinds, milestone.Name + " (" + project.Name + ")", events_service.Notice{Project: &project.ID}, nil}, nil
	case "initiative_id":
		name, err := self.repo.Comment().InitiativeName(ctx, tx, id)
		return about{[3]string{"initiativeNewComment", "initiativeCommentReply", "initiativeCommentMention"}, name, events_service.Notice{Initiative: &id}, nil}, err
	case "document_id":
		name, err := self.repo.Comment().DocumentTitle(ctx, tx, id)
		return about{[3]string{"documentNewComment", "documentCommentReply", "documentCommentMention"}, name, events_service.Notice{Document: &id}, nil}, err
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
