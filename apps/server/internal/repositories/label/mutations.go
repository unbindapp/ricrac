package label_repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent/issuelabel"
	repository "ricrac/internal/repositories"
)

type CreateInput struct {
	Workspace uuid.UUID
	Team      *uuid.UUID
	Name      string
	Color     string
	Creator   uuid.UUID
}

func (self *LabelRepository) Create(ctx context.Context, tx repository.TxInterface, input CreateInput) (uuid.UUID, error) {
	created, err := self.base.Client(tx).IssueLabel.Create().
		SetWorkspaceID(input.Workspace).SetNillableTeamID(input.Team).SetName(input.Name).SetColor(input.Color).
		SetCreatorID(input.Creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

type UpdateInput struct {
	Name      string
	Parent    *uuid.UUID
	IsGroup   bool
	GroupType string
}

func (self *LabelRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input UpdateInput) error {
	update := self.base.Client(tx).IssueLabel.UpdateOneID(id).SetName(input.Name).SetIsGroup(input.IsGroup).SetGroupType(input.GroupType)
	if input.Parent == nil {
		update.ClearParentID()
	} else {
		update.SetParentID(*input.Parent)
	}
	return update.Exec(ctx)
}

func (self *LabelRepository) SetColor(ctx context.Context, tx repository.TxInterface, id uuid.UUID, color string) error {
	return self.base.Client(tx).IssueLabel.UpdateOneID(id).SetColor(color).Exec(ctx)
}

func (self *LabelRepository) SetDescription(ctx context.Context, tx repository.TxInterface, id uuid.UUID, description *string) error {
	update := self.base.Client(tx).IssueLabel.UpdateOneID(id)
	if description == nil {
		return update.ClearDescription().Exec(ctx)
	}
	return update.SetDescription(*description).Exec(ctx)
}

func (self *LabelRepository) Touch(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).IssueLabel.UpdateOneID(id).SetUpdatedAt(now).Exec(ctx)
}

// SetRetired retires (from now on, unless already retired) or restores the label and the
// labels of its group.
func (self *LabelRepository) SetRetired(ctx context.Context, tx repository.TxInterface, id uuid.UUID, retired bool, now time.Time) error {
	labels, err := self.base.Client(tx).IssueLabel.Query().Where(issuelabel.Or(issuelabel.ID(id), issuelabel.ParentID(id))).All(ctx)
	if err != nil {
		return err
	}
	for _, l := range labels {
		update := self.base.Client(tx).IssueLabel.UpdateOne(l).SetUpdatedAt(now)
		switch {
		case !retired:
			update.ClearRetiredAt()
		case l.RetiredAt == nil:
			update.SetRetiredAt(now)
		}
		if err := update.Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (self *LabelRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).IssueLabel.DeleteOneID(id).Exec(ctx)
}
