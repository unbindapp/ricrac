package workspace_repo

import (
	"database/sql"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i WorkspaceRepositoryInterface -p workspace_repo -s WorkspaceRepository -o workspace_repository_iface.go
type WorkspaceRepository struct {
	base *repository.BaseRepository
}

func NewWorkspaceRepository(db *ent.Client) *WorkspaceRepository {
	return &WorkspaceRepository{base: repository.NewBaseRepository(db)}
}

func scan(rows *sql.Rows, dest ...any) error {
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
	}
	return rows.Err()
}
