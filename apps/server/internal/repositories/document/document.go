package document_repo

import (
	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i DocumentRepositoryInterface -p document_repo -s DocumentRepository -o document_repository_iface.go
type DocumentRepository struct {
	base *repository.BaseRepository
}

func NewDocumentRepository(db *ent.Client) *DocumentRepository {
	return &DocumentRepository{base: repository.NewBaseRepository(db)}
}
