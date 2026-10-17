package github_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
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

func (self *GitHubRepository) ClearUserLogin(ctx context.Context, id uuid.UUID) error {
	return self.base.DB.User.Update().Where(user.ID(id)).ClearGithubLogin().Exec(ctx)
}

// SetUserLogin links a GitHub login to a user, taking it from anyone who had it.
func (self *GitHubRepository) SetUserLogin(ctx context.Context, tx repository.TxInterface, id uuid.UUID, login string) error {
	client := self.base.Client(tx)
	err := client.User.Update().Where(user.GithubLogin(login), user.IDNEQ(id)).ClearGithubLogin().Exec(ctx)
	if err != nil {
		return err
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
