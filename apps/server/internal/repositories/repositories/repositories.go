package repositories

import (
	"context"
	"time"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
	access_repo "ricrac/internal/repositories/access"
	attachment_repo "ricrac/internal/repositories/attachment"
	comment_repo "ricrac/internal/repositories/comment"
	events_repo "ricrac/internal/repositories/events"
	issue_repo "ricrac/internal/repositories/issue"
	label_repo "ricrac/internal/repositories/label"
	resolve_repo "ricrac/internal/repositories/resolve"
	seed_repo "ricrac/internal/repositories/seed"
	view_repo "ricrac/internal/repositories/view"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i RepositoriesInterface -p repositories -s Repositories -o repositories_iface.go
type Repositories struct {
	db         *ent.Client
	base       *repository.BaseRepository
	access     access_repo.AccessRepositoryInterface
	resolve    resolve_repo.ResolveRepositoryInterface
	events     events_repo.EventsRepositoryInterface
	label      label_repo.LabelRepositoryInterface
	issue      issue_repo.IssueRepositoryInterface
	seed       seed_repo.SeedRepositoryInterface
	comment    comment_repo.CommentRepositoryInterface
	attachment attachment_repo.AttachmentRepositoryInterface
	view       view_repo.ViewRepositoryInterface
}

func NewRepositories(db *ent.Client) *Repositories {
	return &Repositories{
		db:         db,
		base:       repository.NewBaseRepository(db),
		access:     access_repo.NewAccessRepository(db),
		resolve:    resolve_repo.NewResolveRepository(db),
		events:     events_repo.NewEventsRepository(db),
		label:      label_repo.NewLabelRepository(db),
		issue:      issue_repo.NewIssueRepository(db),
		seed:       seed_repo.NewSeedRepository(db),
		comment:    comment_repo.NewCommentRepository(db),
		attachment: attachment_repo.NewAttachmentRepository(db),
		view:       view_repo.NewViewRepository(db),
	}
}

func (r *Repositories) Ent() *ent.Client {
	return r.db
}

func (r *Repositories) WithTx(ctx context.Context, fn func(tx repository.TxInterface) error) error {
	return r.base.WithTx(ctx, fn)
}

// Now is Postgres' now() in tx: the transaction's start, so what it writes shares one time.
func (r *Repositories) Now(ctx context.Context, tx repository.TxInterface) (time.Time, error) {
	return r.base.Now(ctx, tx)
}

func (r *Repositories) Access() access_repo.AccessRepositoryInterface {
	return r.access
}

func (r *Repositories) Resolve() resolve_repo.ResolveRepositoryInterface {
	return r.resolve
}

func (r *Repositories) Events() events_repo.EventsRepositoryInterface {
	return r.events
}

func (r *Repositories) Label() label_repo.LabelRepositoryInterface {
	return r.label
}

func (r *Repositories) Issue() issue_repo.IssueRepositoryInterface {
	return r.issue
}

func (r *Repositories) Seed() seed_repo.SeedRepositoryInterface {
	return r.seed
}

func (r *Repositories) Comment() comment_repo.CommentRepositoryInterface {
	return r.comment
}

func (r *Repositories) Attachment() attachment_repo.AttachmentRepositoryInterface {
	return r.attachment
}

func (r *Repositories) View() view_repo.ViewRepositoryInterface {
	return r.view
}
