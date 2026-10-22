package github_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/githubusertoken"
	repository "ricrac/internal/repositories"
)

// SealedToken is a GitHub user token with its secrets sealed.
type SealedToken struct {
	AccessToken    string
	ExpiresAt      *time.Time
	RefreshToken   *string
	RefreshExpires *time.Time
}

// UserToken is the GitHub token a user linked, nil when there's none.
func (self *GitHubRepository) UserToken(ctx context.Context, user uuid.UUID) (*SealedToken, error) {
	found, err := self.base.DB.GithubUserToken.Get(ctx, user)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &SealedToken{found.AccessToken, found.ExpiresAt, found.RefreshToken, found.RefreshExpiresAt}, nil
}

func (self *GitHubRepository) HasUserToken(ctx context.Context, user uuid.UUID) (bool, error) {
	return self.base.DB.GithubUserToken.Query().Where(githubusertoken.ID(user)).Exist(ctx)
}

func (self *GitHubRepository) SaveUserToken(ctx context.Context, tx repository.TxInterface, user uuid.UUID, token SealedToken) error {
	return self.base.Client(tx).GithubUserToken.Create().SetID(user).SetAccessToken(token.AccessToken).
		SetNillableExpiresAt(token.ExpiresAt).SetNillableRefreshToken(token.RefreshToken).SetNillableRefreshExpiresAt(token.RefreshExpires).
		OnConflict(entsql.ConflictColumns(githubusertoken.FieldID)).
		UpdateAccessToken().UpdateExpiresAt().UpdateRefreshToken().UpdateRefreshExpiresAt().Exec(ctx)
}

func (self *GitHubRepository) DeleteUserToken(ctx context.Context, tx repository.TxInterface, user uuid.UUID) error {
	_, err := self.base.Client(tx).GithubUserToken.Delete().Where(githubusertoken.ID(user)).Exec(ctx)
	return err
}

// LockIssueComments holds the GitHub comments of an issue until tx ends, so a comment being
// posted can't be imported back before it is linked.
func (self *GitHubRepository) LockIssueComments(ctx context.Context, tx repository.TxInterface, issue uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", issue)
	return err
}
