package github_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/githubusertoken"
	"ricrac/ent/user"
	"ricrac/ent/workspace"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i GitHubRepositoryInterface -p github_repo -s GitHubRepository -o github_repository_iface.go
type GitHubRepository struct {
	base *repository.BaseRepository
}

func NewGitHubRepository(db *ent.Client) *GitHubRepository {
	return &GitHubRepository{base: repository.NewBaseRepository(db)}
}

func now() entsql.Querier {
	return entsql.Expr("now()")
}

// UserLogin is the GitHub login a user linked.
func (self *GitHubRepository) UserLogin(ctx context.Context, id uuid.UUID) (*string, error) {
	found, err := self.base.DB.User.Query().Where(user.ID(id)).Select(user.FieldGithubLogin).Only(ctx)
	if err != nil {
		return nil, err
	}
	return found.GithubLogin, nil
}

// ClearUserLogin unlinks a user's GitHub account and forgets its token.
func (self *GitHubRepository) ClearUserLogin(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	client := self.base.Client(tx)
	if _, err := client.GithubUserToken.Delete().Where(githubusertoken.ID(id)).Exec(ctx); err != nil {
		return err
	}
	return client.User.Update().Where(user.ID(id)).ClearGithubLogin().Exec(ctx)
}

// SetUserLogin links a GitHub login to a user, taking it and its token from anyone who had it.
func (self *GitHubRepository) SetUserLogin(ctx context.Context, tx repository.TxInterface, id uuid.UUID, login string) error {
	client := self.base.Client(tx)
	previous, err := client.User.Query().Where(user.GithubLogin(login), user.IDNEQ(id)).IDs(ctx)
	if err != nil {
		return err
	}
	for _, other := range previous {
		if err := self.ClearUserLogin(ctx, tx, other); err != nil {
			return err
		}
	}
	return client.User.Update().Where(user.ID(id)).SetGithubLogin(login).Exec(ctx)
}

func (self *GitHubRepository) WorkspaceURLKey(ctx context.Context, id uuid.UUID) (string, error) {
	found, err := self.base.DB.Workspace.Query().Where(workspace.ID(id)).Select(workspace.FieldURLKey).Only(ctx)
	if err != nil {
		return "", err
	}
	return found.URLKey, nil
}
