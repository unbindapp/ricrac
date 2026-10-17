package account_repo

import (
	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AccountRepositoryInterface -p account_repo -s AccountRepository -o account_repository_iface.go
type AccountRepository struct {
	base *repository.BaseRepository
}

func NewAccountRepository(db *ent.Client) *AccountRepository {
	return &AccountRepository{base: repository.NewBaseRepository(db)}
}
