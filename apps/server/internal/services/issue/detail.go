package issue_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

func (self *IssueService) Get(ctx context.Context, actor *auth.Actor, input *GetInput) (models.IssueDetail, error) {
	id, err := self.resolve.Issue(ctx, actor, input.ID)
	if err != nil {
		return models.IssueDetail{}, err
	}
	return self.Detail(ctx, actor, id, input.IncludeComments, input.IncludeHistory)
}

// Detail is the issue as get_issue answers it, read from the pool.
func (self *IssueService) Detail(ctx context.Context, actor *auth.Actor, id uuid.UUID, withComments, withHistory bool) (models.IssueDetail, error) {
	return self.detailIn(ctx, nil, actor, id, withComments, withHistory)
}

func (self *IssueService) detailIn(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, withComments, withHistory bool) (models.IssueDetail, error) {
	value, err := self.Value(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail := models.IssueDetail{Issue: value}
	children, err := self.repo.Issue().Children(ctx, tx, id, teams)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Children = make([]models.IssueChild, len(children))
	for i, child := range children {
		detail.Children[i] = models.IssueChildOf(child)
	}
	if detail.Relations, err = self.relations(ctx, tx, id); err != nil {
		return models.IssueDetail{}, err
	}
	linked, err := self.repo.Issue().GitHubIssue(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.GitHub = models.GitHubIssueRefOf(linked)
	attachments, err := self.repo.Issue().Attachments(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Attachments = make([]models.IssueAttachment, len(attachments))
	for i, a := range attachments {
		detail.Attachments[i] = models.IssueAttachmentOf(a)
	}
	subscribers, err := self.repo.Issue().Subscribers(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Subscribers = make([]models.UserRef, len(subscribers))
	for i, s := range subscribers {
		detail.Subscribers[i] = *models.UserRefOf(s.Edges.User)
	}
	releases, err := self.repo.Issue().Releases(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Releases = make([]models.ReleaseRef, len(releases))
	for i, r := range releases {
		detail.Releases[i] = *models.ReleaseRefOf(r.Edges.Release)
	}
	documents, err := self.repo.Issue().Documents(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Documents = make([]models.DocumentRef, len(documents))
	for i, d := range documents {
		detail.Documents[i] = *models.DocumentRefOf(d)
	}
	shares, err := self.repo.Issue().Shares(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.SharedWith = make([]models.UserRef, len(shares))
	for i, s := range shares {
		detail.SharedWith[i] = *models.UserRefOf(s.Edges.User)
	}
	reactions, err := self.repo.Issue().Reactions(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Reactions = make([]models.IssueReaction, len(reactions))
	for i, r := range reactions {
		detail.Reactions[i] = models.IssueReaction{Emoji: r.Emoji, User: models.UserRefOf(r.Edges.User)}
	}
	if detail.CommentCount, err = self.repo.Issue().CommentCount(ctx, tx, id); err != nil {
		return models.IssueDetail{}, err
	}
	if withComments {
		threads, err := self.comments.Threads(ctx, tx, "issue_id", id)
		if err != nil {
			return models.IssueDetail{}, err
		}
		detail.Comments = models.CommentThreads(threads)
		if detail.Comments == nil {
			detail.Comments = models.CommentThreads{}
		}
	}
	if withHistory {
		if detail.History, err = self.history(ctx, tx, id); err != nil {
			return models.IssueDetail{}, err
		}
	}
	return detail, nil
}

func (self *IssueService) relations(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.IssueRelations, error) {
	rows, err := self.repo.Issue().Relations(ctx, tx, id)
	if err != nil {
		return models.IssueRelations{}, err
	}
	relations := models.IssueRelations{Blocks: []models.IssueRef{}, BlockedBy: []models.IssueRef{}, Related: []models.IssueRef{}, Duplicates: []models.IssueRef{}}
	for _, r := range rows {
		outgoing := r.IssueID == id
		other := models.IssueRefOf(r.Edges.RelatedIssue)
		if !outgoing {
			other = models.IssueRefOf(r.Edges.Issue)
		}
		switch {
		case r.Type == "blocks" && outgoing:
			relations.Blocks = append(relations.Blocks, *other)
		case r.Type == "blocks":
			relations.BlockedBy = append(relations.BlockedBy, *other)
		case r.Type == "related":
			relations.Related = append(relations.Related, *other)
		case r.Type == "duplicate" && outgoing:
			if relations.DuplicateOf == nil {
				relations.DuplicateOf = other
			}
		case r.Type == "duplicate":
			relations.Duplicates = append(relations.Duplicates, *other)
		}
	}
	return relations, nil
}

func (self *IssueService) history(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.HistoryEntries, error) {
	rows, err := self.repo.Issue().History(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	entries := make(models.HistoryEntries, len(rows))
	for i, h := range rows {
		entries[i] = models.HistoryEntryOf(h)
	}
	return entries, nil
}

func (self *IssueService) History(ctx context.Context, actor *auth.Actor, input *IDInput) (models.IssueHistory, error) {
	id, err := self.resolve.Issue(ctx, actor, input.ID)
	if err != nil {
		return models.IssueHistory{}, err
	}
	identifier, err := self.repo.Issue().Identifier(ctx, nil, id)
	if err != nil {
		return models.IssueHistory{}, err
	}
	entries, err := self.history(ctx, nil, id)
	if err != nil {
		return models.IssueHistory{}, err
	}
	return models.IssueHistory{Identifier: identifier, History: entries}, nil
}
