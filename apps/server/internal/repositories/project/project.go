package project_repo

import (
	"context"
	"database/sql"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ProjectRepositoryInterface -p project_repo -s ProjectRepository -o project_repository_iface.go
type ProjectRepository struct {
	base *repository.BaseRepository
}

func NewProjectRepository(db *ent.Client) *ProjectRepository {
	return &ProjectRepository{base: repository.NewBaseRepository(db)}
}

// scan runs a query and hands each row to read.
func scan(ctx context.Context, client *ent.Client, read func(rows *sql.Rows) error, query string, args ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := read(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
