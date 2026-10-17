package oauth_repo

import (
	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i OAuthRepositoryInterface -p oauth_repo -s OAuthRepository -o oauth_repository_iface.go
type OAuthRepository struct {
	base *repository.BaseRepository
}

func NewOAuthRepository(db *ent.Client) *OAuthRepository {
	return &OAuthRepository{base: repository.NewBaseRepository(db)}
}
