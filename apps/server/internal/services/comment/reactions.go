package comment_service

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

type ReactionInput struct {
	Emoji          string  `json:"emoji" doc:"A unicode emoji (👍) or a shortcode (+1, :tada:)."`
	IssueID        *string `json:"issueId" doc:"Issue identifier (ENG-123), id or link."`
	CommentID      *string `json:"commentId" doc:"Comment id."`
	StatusUpdateID *string `json:"statusUpdateId" doc:"Status update id."`
}

type RemoveReactionInput struct {
	ID             *string `json:"id" doc:"The reaction's id. Or pass emoji and what it is on."`
	Emoji          *string `json:"emoji"`
	IssueID        *string `json:"issueId"`
	CommentID      *string `json:"commentId"`
	StatusUpdateID *string `json:"statusUpdateId"`
}

// normalizeEmoji keeps shortcodes without colons, lowercase; unicode emoji as they are.
func normalizeEmoji(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	name := trimmed
	if inner, ok := strings.CutPrefix(trimmed, ":"); ok {
		if inner, ok := strings.CutSuffix(inner, ":"); ok {
			name = inner
		}
	}
	shortcode := len(name) <= 50
	ascii := true
	for _, r := range name {
		if r >= utf8.RuneSelf {
			ascii = false
		}
		if !(r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) || r == '_' || r == '+' || r == '-') {
			shortcode = false
		}
	}
	isUnicode := !ascii && utf8.RuneCountInString(name) <= 16 && !strings.ContainsFunc(name, unicode.IsSpace)
	if name == "" || !(shortcode || isUnicode) {
		return "", errdefs.BadRequest("%q isn't an emoji; pass one like 👍 or a shortcode like +1 or :tada:", value)
	}
	if shortcode {
		return strings.ToLower(name), nil
	}
	return name, nil
}

// reactionTarget is the column and id a reaction is on, checked visible, and the issue's team
// for webhooks.
func (self *CommentService) reactionTarget(ctx context.Context, actor *auth.Actor, issue, comment, statusUpdate *string) (string, uuid.UUID, *uuid.UUID, error) {
	var column string
	var id uuid.UUID
	var on *Target
	switch {
	case issue != nil && comment == nil && statusUpdate == nil:
		issueID, err := self.resolve.Issue(ctx, actor, *issue)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		column, id, on = "issue_id", issueID, &Target{"issue_id", issueID}
	case issue == nil && comment != nil && statusUpdate == nil:
		row, err := self.Load(ctx, actor, *comment)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		if row.DeletedAt != nil {
			return "", uuid.Nil, nil, errDeleted()
		}
		target, err := TargetOf(row)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		column, id, on = "comment_id", row.ID, &target
	case issue == nil && comment == nil && statusUpdate != nil:
		updateID, err := self.resolve.ByID(ctx, actor, "status_updates", "status update", *statusUpdate)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		if err := self.checkVisible(ctx, actor, Target{"status_update_id", updateID}); err != nil {
			return "", uuid.Nil, nil, err
		}
		column, id = "status_update_id", updateID
	default:
		return "", uuid.Nil, nil, errdefs.BadRequest("Pass exactly one of issueId, commentId and statusUpdateId")
	}
	if on == nil {
		return column, id, nil, nil
	}
	team, err := self.teamOf(ctx, nil, *on)
	return column, id, team, err
}

func (self *CommentService) AddReaction(ctx context.Context, actor *auth.Actor, input *ReactionInput) (models.Reaction, error) {
	emoji, err := normalizeEmoji(input.Emoji)
	if err != nil {
		return models.Reaction{}, err
	}
	column, id, team, err := self.reactionTarget(ctx, actor, input.IssueID, input.CommentID, input.StatusUpdateID)
	if err != nil {
		return models.Reaction{}, err
	}
	var value models.Reaction
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		inserted, err := self.repo.Comment().AddReaction(ctx, tx, actor.Workspace, actor.User, emoji, column, id)
		if err != nil {
			return err
		}
		reaction, err := self.repo.Comment().FindReaction(ctx, tx, actor.User, emoji, column, id)
		if err != nil {
			return err
		}
		if reaction == nil {
			return errors.New("the reaction just added is missing")
		}
		value = models.ReactionOf(reaction)
		if !inserted {
			return nil
		}
		return self.events.Emit(ctx, tx, actor, "Reaction", "create", value, nil, team)
	})
	return value, err
}

func (self *CommentService) RemoveReaction(ctx context.Context, actor *auth.Actor, input *RemoveReactionInput) (models.RemovedReaction, error) {
	var reaction uuid.UUID
	var team *uuid.UUID
	exists := false
	switch {
	case input.ID != nil && input.Emoji == nil:
		id, ok := utils.AsUUID(*input.ID)
		if !ok {
			return models.RemovedReaction{}, errdefs.NotFound("No reaction %q", *input.ID)
		}
		found, foundTeam, err := self.repo.Comment().OwnReaction(ctx, actor.Workspace, actor.User, id)
		if err != nil {
			return models.RemovedReaction{}, err
		}
		if found != nil {
			reaction, team, exists = found.ID, foundTeam, true
		}
	case input.ID == nil && input.Emoji != nil:
		emoji, err := normalizeEmoji(*input.Emoji)
		if err != nil {
			return models.RemovedReaction{}, err
		}
		column, id, targetTeam, err := self.reactionTarget(ctx, actor, input.IssueID, input.CommentID, input.StatusUpdateID)
		if err != nil {
			return models.RemovedReaction{}, err
		}
		team = targetTeam
		found, err := self.repo.Comment().FindReaction(ctx, nil, actor.User, emoji, column, id)
		if err != nil {
			return models.RemovedReaction{}, err
		}
		if found != nil {
			reaction, exists = found.ID, true
		}
	default:
		return models.RemovedReaction{}, errdefs.BadRequest("Pass the reaction's id, or emoji and what it is on")
	}
	if !exists {
		return models.RemovedReaction{}, errdefs.NotFound("You have no such reaction")
	}
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		found, err := self.repo.Comment().Reaction(ctx, tx, reaction)
		if err != nil {
			return err
		}
		if err := self.repo.Comment().DeleteReaction(ctx, tx, reaction); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Reaction", "remove", models.ReactionOf(found), nil, team)
	})
	if err != nil {
		return models.RemovedReaction{}, err
	}
	return models.RemovedReaction{Removed: models.ID(reaction)}, nil
}
