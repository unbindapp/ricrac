package discussion_repo

import (
	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i DiscussionRepositoryInterface -p discussion_repo -s DiscussionRepository -o discussion_repository_iface.go
type DiscussionRepository struct {
	base *repository.BaseRepository
}

func NewDiscussionRepository(db *ent.Client) *DiscussionRepository {
	return &DiscussionRepository{base: repository.NewBaseRepository(db)}
}
