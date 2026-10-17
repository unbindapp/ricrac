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
	initiative_repo "ricrac/internal/repositories/initiative"
	issue_repo "ricrac/internal/repositories/issue"
	label_repo "ricrac/internal/repositories/label"
	milestone_repo "ricrac/internal/repositories/milestone"
	project_repo "ricrac/internal/repositories/project"
	resolve_repo "ricrac/internal/repositories/resolve"
	seed_repo "ricrac/internal/repositories/seed"
	statusupdate_repo "ricrac/internal/repositories/statusupdate"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i RepositoriesInterface -p repositories -s Repositories -o repositories_iface.go
type Repositories struct {
	db           *ent.Client
	base         *repository.BaseRepository
	access       access_repo.AccessRepositoryInterface
	resolve      resolve_repo.ResolveRepositoryInterface
	events       events_repo.EventsRepositoryInterface
	label        label_repo.LabelRepositoryInterface
	issue        issue_repo.IssueRepositoryInterface
	seed         seed_repo.SeedRepositoryInterface
	comment      comment_repo.CommentRepositoryInterface
	attachment   attachment_repo.AttachmentRepositoryInterface
	project      project_repo.ProjectRepositoryInterface
	milestone    milestone_repo.MilestoneRepositoryInterface
	statusUpdate statusupdate_repo.StatusUpdateRepositoryInterface
	initiative   initiative_repo.InitiativeRepositoryInterface
}

func NewRepositories(db *ent.Client) *Repositories {
	return &Repositories{
		db:           db,
		base:         repository.NewBaseRepository(db),
		access:       access_repo.NewAccessRepository(db),
		resolve:      resolve_repo.NewResolveRepository(db),
		events:       events_repo.NewEventsRepository(db),
		label:        label_repo.NewLabelRepository(db),
		issue:        issue_repo.NewIssueRepository(db),
		seed:         seed_repo.NewSeedRepository(db),
		comment:      comment_repo.NewCommentRepository(db),
		attachment:   attachment_repo.NewAttachmentRepository(db),
		project:      project_repo.NewProjectRepository(db),
		milestone:    milestone_repo.NewMilestoneRepository(db),
		statusUpdate: statusupdate_repo.NewStatusUpdateRepository(db),
		initiative:   initiative_repo.NewInitiativeRepository(db),
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

func (r *Repositories) Project() project_repo.ProjectRepositoryInterface {
	return r.project
}

func (r *Repositories) Milestone() milestone_repo.MilestoneRepositoryInterface {
	return r.milestone
}

func (r *Repositories) StatusUpdate() statusupdate_repo.StatusUpdateRepositoryInterface {
	return r.statusUpdate
}

func (r *Repositories) Initiative() initiative_repo.InitiativeRepositoryInterface {
	return r.initiative
}
