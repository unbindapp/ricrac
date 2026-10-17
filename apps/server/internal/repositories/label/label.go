package label_repo

import (
	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i LabelRepositoryInterface -p label_repo -s LabelRepository -o label_repository_iface.go
type LabelRepository struct {
	base *repository.BaseRepository
}

func NewLabelRepository(db *ent.Client) *LabelRepository {
	return &LabelRepository{base: repository.NewBaseRepository(db)}
}
