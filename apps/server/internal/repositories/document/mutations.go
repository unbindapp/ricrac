package document_repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	repository "ricrac/internal/repositories"
)

// Parents is where a document belongs: one of them is set.
type Parents struct {
	Project, Initiative, Team, Issue, Cycle, Release *uuid.UUID
}

type CreateInput struct {
	Workspace uuid.UUID
	SlugID    string
	Title     string
	Content   string
	Icon      *string
	Color     *string
	Parents   Parents
	Creator   uuid.UUID
}

func (self *DocumentRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	p := input.Parents
	created, err := self.base.Client(tx).Document.Create().
		SetWorkspaceID(input.Workspace).SetSlugID(input.SlugID).SetTitle(input.Title).SetContent(input.Content).
		SetNillableIcon(input.Icon).SetNillableColor(input.Color).
		SetNillableProjectID(p.Project).SetNillableInitiativeID(p.Initiative).SetNillableTeamID(p.Team).
		SetNillableIssueID(p.Issue).SetNillableCycleID(p.Cycle).SetNillableReleaseID(p.Release).
		SetCreatorID(input.Creator).SetUpdatedByID(input.Creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *DocumentRepository) SetText(ctx context.Context, tx repository.TxInterface, id uuid.UUID, title, content string) error {
	return self.base.Client(tx).Document.UpdateOneID(id).SetTitle(title).SetContent(content).Exec(ctx)
}

func (self *DocumentRepository) SetIcon(ctx context.Context, tx repository.TxInterface, id uuid.UUID, icon *string) error {
	update := self.base.Client(tx).Document.UpdateOneID(id)
	if icon == nil {
		return update.ClearIcon().Exec(ctx)
	}
	return update.SetIcon(*icon).Exec(ctx)
}

func (self *DocumentRepository) SetColor(ctx context.Context, tx repository.TxInterface, id uuid.UUID, color *string) error {
	update := self.base.Client(tx).Document.UpdateOneID(id)
	if color == nil {
		return update.ClearColor().Exec(ctx)
	}
	return update.SetColor(*color).Exec(ctx)
}

// SetParents moves the document to where parents says, clearing the others.
func (self *DocumentRepository) SetParents(ctx context.Context, tx repository.TxInterface, id uuid.UUID, parents Parents) error {
	update := self.base.Client(tx).Document.UpdateOneID(id).
		ClearProjectID().ClearInitiativeID().ClearTeamID().ClearIssueID().ClearCycleID().ClearReleaseID()
	setParents(update, parents)
	return update.Exec(ctx)
}

func setParents(update *ent.DocumentUpdateOne, p Parents) {
	if p.Project != nil {
		update.SetProjectID(*p.Project)
	}
	if p.Initiative != nil {
		update.SetInitiativeID(*p.Initiative)
	}
	if p.Team != nil {
		update.SetTeamID(*p.Team)
	}
	if p.Issue != nil {
		update.SetIssueID(*p.Issue)
	}
	if p.Cycle != nil {
		update.SetCycleID(*p.Cycle)
	}
	if p.Release != nil {
		update.SetReleaseID(*p.Release)
	}
}

// Touch records that user changed the document now.
func (self *DocumentRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time, user uuid.UUID) error {
	return self.base.Client(tx).Document.UpdateOneID(id).SetUpdatedAt(now).SetUpdatedByID(user).Exec(ctx)
}

// SetTrashed moves the document to the trash (now) or out of it, as a change by user.
func (self *DocumentRepository) SetTrashed(ctx context.Context, tx repository.TxInterface, id uuid.UUID, trashed bool, now time.Time, user uuid.UUID) error {
	update := self.base.Client(tx).Document.UpdateOneID(id).SetUpdatedAt(now).SetUpdatedByID(user)
	if trashed {
		update.SetTrashedAt(now)
	} else {
		update.ClearTrashedAt()
	}
	return update.Exec(ctx)
}

func (self *DocumentRepository) AddVersion(ctx context.Context, tx repository.TxInterface, document uuid.UUID, title, content string, author *uuid.UUID) error {
	return self.base.Client(tx).DocumentVersion.Create().SetDocumentID(document).SetTitle(title).SetContent(content).
		SetNillableAuthorID(author).Exec(ctx)
}
