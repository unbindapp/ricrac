package repositories

import (
	"context"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
	access_repo "ricrac/internal/repositories/access"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i RepositoriesInterface -p repositories -s Repositories -o repositories_iface.go
type Repositories struct {
	db     *ent.Client
	base   *repository.BaseRepository
	access access_repo.AccessRepositoryInterface
}

func NewRepositories(db *ent.Client) *Repositories {
	return &Repositories{
		db:     db,
		base:   repository.NewBaseRepository(db),
		access: access_repo.NewAccessRepository(db),
	}
}

func (r *Repositories) Ent() *ent.Client {
	return r.db
}

func (r *Repositories) WithTx(ctx context.Context, fn func(tx repository.TxInterface) error) error {
	return r.base.WithTx(ctx, fn)
}

func (r *Repositories) Base() *repository.BaseRepository {
	return r.base
}

func (r *Repositories) Access() access_repo.AccessRepositoryInterface {
	return r.access
}
