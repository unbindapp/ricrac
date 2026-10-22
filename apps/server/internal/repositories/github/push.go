package github_repo

import (
	"context"
	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/comment"
	"ricrac/ent/githubcommentlink"
	"ricrac/ent/githubinstallation"
	"ricrac/ent/githuboutbox"
	"ricrac/ent/githubreposync"
	"ricrac/ent/issue"
	"ricrac/ent/user"
	repository "ricrac/internal/repositories"
)

// OutboxEntry is a change queued for GitHub, with the repository it goes to.
type OutboxEntry struct {
	ID            int64
	RepoSync      uuid.UUID
	Number        int
	Issue         *uuid.UUID
	Comment       *uuid.UUID
	GitHubComment *int64
	Kind          string
	Changes       json.RawMessage
	Attempts      int
	Installation  int64
	FullName      string
	// The user a comment was posted as, to edit or delete it as them too.
	ActAs *uuid.UUID
}

// DueOutbox is up to 50 queued changes due now, of installations that aren't suspended, oldest
// first.
func (self *GitHubRepository) DueOutbox(ctx context.Context) ([]OutboxEntry, error) {
	rows, err := self.base.DB.GithubOutbox.Query().
		Where(func(s *entsql.Selector) { s.Where(repository.Raw(s.C(githuboutbox.FieldNextAttemptAt) + " <= now()")) },
			githuboutbox.HasRepoSyncWith(githubreposync.HasInstallationWith(githubinstallation.SuspendedAtIsNil()))).
		WithRepoSync().Order(githuboutbox.ByID()).Limit(50).All(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]OutboxEntry, len(rows))
	for i, row := range rows {
		entries[i] = OutboxEntry{
			ID: row.ID, RepoSync: row.RepoSyncID, Number: row.GithubNumber, Issue: row.IssueID, Comment: row.CommentID,
			GitHubComment: row.GithubCommentID, Kind: row.Kind, Changes: row.Changes, Attempts: row.Attempts,
			Installation: row.Edges.RepoSync.InstallationID, FullName: row.Edges.RepoSync.FullName, ActAs: row.ActAs,
		}
	}
	return entries, nil
}

func (self *GitHubRepository) DeleteOutbox(ctx context.Context, id int64) error {
	_, err := self.base.DB.GithubOutbox.Delete().Where(githuboutbox.ID(id)).Exec(ctx)
	return err
}

// RetryOutbox puts a change off by seconds.
func (self *GitHubRepository) RetryOutbox(ctx context.Context, id int64, seconds float64) error {
	return self.base.DB.GithubOutbox.Update().Where(githuboutbox.ID(id)).AddAttempts(1).
		Modify(func(u *entsql.UpdateBuilder) {
			u.Set(githuboutbox.FieldNextAttemptAt, repository.RawExpr("now() + make_interval(secs => ?)", seconds))
		}).Exec(ctx)
}

// CommentWithAuthor is a comment with its author, nil when there's none.
func (self *GitHubRepository) CommentWithAuthor(ctx context.Context, id uuid.UUID) (*ent.Comment, error) {
	found, err := self.base.DB.Comment.Query().Where(comment.ID(id)).WithUser().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// CommentPosted says whether a comment is linked to a GitHub comment.
func (self *GitHubRepository) CommentPosted(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (bool, error) {
	return self.base.Client(tx).GithubCommentLink.Query().Where(githubcommentlink.ID(id)).Exist(ctx)
}

// LinkPostedComment links a comment the app posted on the linked GitHub issue, as postedAs or
// as itself when nil.
func (self *GitHubRepository) LinkPostedComment(ctx context.Context, tx repository.TxInterface, commentID uuid.UUID, issueID *uuid.UUID, githubID int64, postedAs *uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO github_comment_links (comment_id, issue_id, github_id, posted, posted_as)
		 SELECT $1, issue_id, $3, true, $4 FROM github_issue_links WHERE issue_id = $2 ON CONFLICT DO NOTHING`,
		commentID, issueID, githubID, postedAs)
	return err
}

// Issue is an issue, nil when there's none.
func (self *GitHubRepository) Issue(ctx context.Context, id uuid.UUID) (*ent.Issue, error) {
	found, err := self.base.DB.Issue.Query().Where(issue.ID(id)).Select(issue.FieldTitle, issue.FieldDescription).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// LoginOf is the GitHub login of the user with this id; false when there's no such user.
func (self *GitHubRepository) LoginOf(ctx context.Context, id string) (*string, bool, error) {
	found, err := self.base.DB.User.Query().
		Where(func(s *entsql.Selector) { s.Where(repository.Raw(s.C(user.FieldID)+"::text = ?", id)) }).
		Select(user.FieldGithubLogin).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return found.GithubLogin, true, nil
}
