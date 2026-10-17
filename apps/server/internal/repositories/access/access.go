package access_repo

import (
	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AccessRepositoryInterface -p access_repo -s AccessRepository -o access_repository_iface.go
type AccessRepository struct {
	base *repository.BaseRepository
}

func NewAccessRepository(db *ent.Client) *AccessRepository {
	return &AccessRepository{base: repository.NewBaseRepository(db)}
}
