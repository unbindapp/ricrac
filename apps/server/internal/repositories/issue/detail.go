package issue_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/attachment"
	"ricrac/ent/comment"
	"ricrac/ent/document"
	"ricrac/ent/githubissuelink"
	"ricrac/ent/issue"
	"ricrac/ent/issuehistory"
	"ricrac/ent/issuerelation"
	"ricrac/ent/issuerelease"
	"ricrac/ent/issueshare"
	"ricrac/ent/issuesubscriber"
	"ricrac/ent/reaction"
	repository "ricrac/internal/repositories"
)

// Children is the sub-issues not in the trash of the teams given, in their order.
func (self *IssueRepository) Children(ctx context.Context, tx repository.TxInterface, parent uuid.UUID, teams []uuid.UUID) ([]*ent.Issue, error) {
	return self.base.Client(tx).Issue.Query().
		Where(issue.ParentID(parent), issue.TrashedAtIsNil(), issue.TeamIDIn(teams...)).
		WithTeam().WithState().WithAssignee().
		Order(issue.BySubIssueSortOrder(entsql.OrderNullsLast()), issue.ByCreatedAt()).
		All(ctx)
}

// Relations is the relations from and to the issue, oldest first, with both issues' teams and
// states.
func (self *IssueRepository) Relations(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.IssueRelation, error) {
	withRef := func(q *ent.IssueQuery) { q.WithTeam().WithState() }
	return self.base.Client(tx).IssueRelation.Query().
		Where(issuerelation.Or(issuerelation.IssueID(id), issuerelation.RelatedIssueID(id))).
		WithIssue(withRef).WithRelatedIssue(withRef).
		Order(issuerelation.ByCreatedAt()).
		All(ctx)
}

// Attachments is the issue's attachments, oldest first, with their creators.
func (self *IssueRepository) Attachments(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.Attachment, error) {
	return self.base.Client(tx).Attachment.Query().Where(attachment.IssueID(id)).WithCreator().
		Order(attachment.ByCreatedAt()).All(ctx)
}

// GitHubIssue is the GitHub issue the issue syncs with and its repository, nil when none.
func (self *IssueRepository) GitHubIssue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.GithubIssueLink, error) {
	found, err := self.base.Client(tx).GithubIssueLink.Query().Where(githubissuelink.ID(id)).WithRepoSync().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (self *IssueRepository) Subscribers(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.IssueSubscriber, error) {
	return self.base.Client(tx).IssueSubscriber.Query().Where(issuesubscriber.IssueID(id)).WithUser().All(ctx)
}

func (self *IssueRepository) Releases(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.IssueRelease, error) {
	return self.base.Client(tx).IssueRelease.Query().Where(issuerelease.IssueID(id)).WithRelease().All(ctx)
}

// Documents is the issue's documents not in the trash.
func (self *IssueRepository) Documents(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.Document, error) {
	return self.base.Client(tx).Document.Query().Where(document.IssueID(id), document.TrashedAtIsNil()).All(ctx)
}

func (self *IssueRepository) Shares(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.IssueShare, error) {
	return self.base.Client(tx).IssueShare.Query().Where(issueshare.IssueID(id)).WithUser().All(ctx)
}

func (self *IssueRepository) Reactions(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.Reaction, error) {
	return self.base.Client(tx).Reaction.Query().Where(reaction.IssueID(id)).WithUser().All(ctx)
}

// CommentCount is how many comments and replies the issue has.
func (self *IssueRepository) CommentCount(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	return self.base.Client(tx).Comment.Query().Where(comment.IssueID(id)).Count(ctx)
}

// History is the issue's history, oldest first, with the actors.
func (self *IssueRepository) History(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.IssueHistory, error) {
	return self.base.Client(tx).IssueHistory.Query().Where(issuehistory.IssueID(id)).WithActor().
		Order(issuehistory.ByID()).All(ctx)
}
