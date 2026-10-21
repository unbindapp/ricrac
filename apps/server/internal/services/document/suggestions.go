package document_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	events_service "ricrac/internal/services/events"
)

type SuggestionInput struct {
	ID string `json:"id" doc:"The suggestion: its comment's id."`
}

func (self *DocumentService) AcceptSuggestion(ctx context.Context, actor *auth.Actor, input *SuggestionInput) (models.CommentDetail, error) {
	return self.decide(ctx, actor, input.ID, "accepted")
}

func (self *DocumentService) RejectSuggestion(ctx context.Context, actor *auth.Actor, input *SuggestionInput) (models.CommentDetail, error) {
	return self.decide(ctx, actor, input.ID, "rejected")
}

// suggestion is a pending suggestion on a document the caller can see.
func (self *DocumentService) suggestion(ctx context.Context, actor *auth.Actor, value string) (*ent.Comment, error) {
	notFound := errdefs.NotFound("No suggestion %q", strings.TrimSpace(value))
	id, ok := utils.AsUUID(value)
	if !ok {
		return nil, notFound
	}
	found, err := self.repo.Comment().Find(ctx, actor.Workspace, id)
	if err != nil {
		return nil, err
	}
	if found == nil || found.SuggestedText == nil || found.DocumentID == nil {
		return nil, notFound
	}
	visible, err := self.resolve.CanSeeDocument(ctx, actor, *found.DocumentID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, notFound
	}
	if status := utils.OrEmpty(found.SuggestionStatus); status != "pending" {
		return nil, errdefs.BadRequest("The suggestion was already %s", status)
	}
	return found, nil
}

// decide accepts the suggestion, putting its text in place of the passage it quotes, or rejects
// it. Either resolves its thread and tells its author.
func (self *DocumentService) decide(ctx context.Context, actor *auth.Actor, reference, status string) (models.CommentDetail, error) {
	row, err := self.suggestion(ctx, actor, reference)
	if err != nil {
		return models.CommentDetail{}, err
	}
	document := *row.DocumentID
	var value models.CommentDetail
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if status == "accepted" {
			if err := self.applySuggestion(ctx, tx, actor, document, utils.OrEmpty(row.QuotedText), *row.SuggestedText); err != nil {
				return err
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Comment().Decide(ctx, tx, row.ID, status, actor.User, now); err != nil {
			return err
		}
		loaded, err := self.repo.Comment().Load(ctx, tx, row.ID)
		if err != nil {
			return err
		}
		value = models.CommentDetailOf(loaded, self.cfg.PublicURL)
		if err := self.notifyDecision(ctx, tx, actor, row, status, loaded.Edges.Document.Title); err != nil {
			return err
		}
		team, err := self.repo.Document().WebhookTeam(ctx, tx, document)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Comment", "update", value, events_service.M{"suggestionStatus": "pending"}, team)
	})
	return value, err
}

func (self *DocumentService) applySuggestion(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, quoted, suggested string) error {
	current, err := self.repo.Document().GetForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	switch count := strings.Count(current.Content, quoted); {
	case count == 0:
		return errdefs.BadRequest("The passage the suggestion replaces is no longer in the document; reject it or edit the document")
	case count > 1:
		return errdefs.BadRequest("The passage the suggestion replaces is in the document %d times now; edit the document instead", count)
	}
	patch := []args.TextEdit{{Op: "replace", OldString: quoted, NewString: suggested}}
	if err := self.update(ctx, tx, actor, id, &SaveInput{Patch: patch}); err != nil {
		return err
	}
	value, err := self.value(ctx, tx, id)
	if err != nil {
		return err
	}
	team, err := self.repo.Document().WebhookTeam(ctx, tx, id)
	if err != nil {
		return err
	}
	return self.events.Emit(ctx, tx, actor, "Document", "update", value, nil, team)
}

func (self *DocumentService) notifyDecision(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Comment, status, title string) error {
	if row.UserID == nil {
		return nil
	}
	name, err := self.repo.Comment().UserName(ctx, actor.User)
	if err != nil {
		return err
	}
	kind := "documentSuggestionAccepted"
	if status == "rejected" {
		kind = "documentSuggestionRejected"
	}
	return self.events.Notify(ctx, tx, actor, []uuid.UUID{*row.UserID}, events_service.Notice{
		Kind: kind, Summary: fmt.Sprintf("%s %s your suggestion on %s", name, status, title), Comment: &row.ID, Document: row.DocumentID,
	})
}
