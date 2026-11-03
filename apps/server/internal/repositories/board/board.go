package board_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/board"
	"ricrac/ent/boardcard"
	"ricrac/ent/boardcolumn"
	"ricrac/ent/discussion"
	"ricrac/ent/issue"
	"ricrac/ent/issueshare"
	"ricrac/ent/predicate"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i BoardRepositoryInterface -p board_repo -s BoardRepository -o board_repository_iface.go
type BoardRepository struct {
	base *repository.BaseRepository
}

func NewBoardRepository(db *ent.Client) *BoardRepository {
	return &BoardRepository{base: repository.NewBaseRepository(db)}
}

func withColumns(q *ent.BoardColumnQuery) { q.Order(ent.Asc(boardcolumn.FieldPosition)) }

// Load is the board with what its answer shows.
func (self *BoardRepository) Load(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Board, error) {
	return self.base.Client(tx).Board.Query().Where(board.ID(id)).
		WithWorkspace().WithCreator().WithColumns(withColumns).Only(ctx)
}

// LoadAll is the boards with what their answers show, in the order of ids.
func (self *BoardRepository) LoadAll(ctx context.Context, ids []uuid.UUID) ([]*ent.Board, error) {
	boards, err := self.base.DB.Board.Query().Where(board.IDIn(ids...)).
		WithWorkspace().WithCreator().WithColumns(withColumns).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Board, len(boards))
	for _, b := range boards {
		byID[b.ID] = b
	}
	ordered := make([]*ent.Board, 0, len(ids))
	for _, id := range ids {
		if b, ok := byID[id]; ok {
			ordered = append(ordered, b)
		}
	}
	return ordered, nil
}

type ListFilter struct {
	Workspace uuid.UUID
	// An ILIKE pattern on the name or description.
	Pattern *string
	Limit   int64
	Offset  int64
}

// List is the workspace's boards, by name.
func (self *BoardRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	return self.base.DB.Board.Query().Where(board.WorkspaceID(filter.Workspace)).
		Modify(func(s *entsql.Selector) {
			if filter.Pattern != nil {
				s.Where(repository.Raw(s.C(board.FieldName)+" ILIKE ? OR "+s.C(board.FieldDescription)+" ILIKE ?", *filter.Pattern, *filter.Pattern))
			}
			s.Select(s.C(board.FieldID))
			s.OrderExpr(entsql.Expr("lower(" + s.C(board.FieldName) + ")"))
			s.OrderExpr(entsql.Expr(s.C(board.FieldCreatedAt)))
			s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
		}).IDs(ctx)
}

type NewBoard struct {
	Workspace, Creator uuid.UUID
	SlugID, Name       string
}

func (self *BoardRepository) Create(ctx context.Context, tx repository.TxInterface, input NewBoard) (uuid.UUID, error) {
	create := self.base.Client(tx).Board.Create().SetWorkspaceID(input.Workspace).SetSlugID(input.SlugID).SetName(input.Name)
	if input.Creator != uuid.Nil {
		create.SetCreatorID(input.Creator)
	}
	created, err := create.Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// Change is a change of some of a board's columns.
type Change func(*ent.BoardUpdateOne)

func SetName(name string) Change { return func(u *ent.BoardUpdateOne) { u.SetName(name) } }

func SetDescription(description *string) Change {
	return func(u *ent.BoardUpdateOne) {
		if description == nil {
			u.ClearDescription()
			return
		}
		u.SetDescription(*description)
	}
}

func Touch(now time.Time) Change { return func(u *ent.BoardUpdateOne) { u.SetUpdatedAt(now) } }

func (self *BoardRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, changes ...Change) error {
	update := self.base.Client(tx).Board.UpdateOneID(id)
	for _, change := range changes {
		change(update)
	}
	return update.Exec(ctx)
}

func (self *BoardRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).Board.DeleteOneID(id).Exec(ctx)
}

func (self *BoardRepository) Columns(ctx context.Context, tx repository.TxInterface, boardID uuid.UUID) ([]*ent.BoardColumn, error) {
	return self.base.Client(tx).BoardColumn.Query().Where(boardcolumn.BoardID(boardID)).
		Order(ent.Asc(boardcolumn.FieldPosition)).All(ctx)
}

func (self *BoardRepository) CreateColumn(ctx context.Context, tx repository.TxInterface, boardID uuid.UUID, name string, color *string, position float64) error {
	return self.base.Client(tx).BoardColumn.Create().SetBoardID(boardID).SetName(name).
		SetNillableColor(color).SetPosition(position).Exec(ctx)
}

func (self *BoardRepository) UpdateColumn(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name string, color *string, position float64) error {
	update := self.base.Client(tx).BoardColumn.UpdateOneID(id).SetName(name).SetPosition(position)
	if color == nil {
		update.ClearColor()
	} else {
		update.SetColor(*color)
	}
	return update.Exec(ctx)
}

// DeleteColumns deletes the columns, taking their cards off the board.
func (self *BoardRepository) DeleteColumns(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := self.base.Client(tx).BoardColumn.Delete().Where(boardcolumn.IDIn(ids...)).Exec(ctx)
	return err
}

// Visible narrows cards to live issues of these teams and those shared with viewer, and to live
// discussions of the workspace or of these teams.
type Visible struct {
	Teams  []uuid.UUID
	Viewer uuid.UUID
}

func (v Visible) cards() predicate.BoardCard {
	issues := issue.And(issue.TrashedAtIsNil(), issue.ArchivedAtIsNil(),
		issue.Or(issue.TeamIDIn(v.Teams...), issue.HasIssueSharesWith(issueshare.UserID(v.Viewer))))
	discussions := discussion.And(discussion.TrashedAtIsNil(),
		discussion.Or(discussion.TeamIDIsNil(), discussion.TeamIDIn(v.Teams...)))
	return boardcard.Or(boardcard.HasIssueWith(issues), boardcard.HasDiscussionWith(discussions))
}

// Subject is what a card holds: an issue or a discussion.
type Subject struct {
	Issue, Discussion *uuid.UUID
}

func IssueSubject(id uuid.UUID) Subject { return Subject{Issue: &id} }

func DiscussionSubject(id uuid.UUID) Subject { return Subject{Discussion: &id} }

func (s Subject) is() predicate.BoardCard {
	if s.Issue != nil {
		return boardcard.IssueID(*s.Issue)
	}
	return boardcard.DiscussionID(*s.Discussion)
}

func (s Subject) column() string {
	if s.Issue != nil {
		return boardcard.FieldIssueID
	}
	return boardcard.FieldDiscussionID
}

// Counts is how many visible issues and discussions each column of the boards holds.
func (self *BoardRepository) Counts(ctx context.Context, boards []uuid.UUID, visible Visible) (map[uuid.UUID]models.BoardColumnCounts, error) {
	var rows []struct {
		ColumnID uuid.UUID `json:"column_id"`
		Issues   int       `json:"issues"`
		Count    int       `json:"count"`
	}
	err := self.base.DB.BoardCard.Query().
		Where(boardcard.BoardIDIn(boards...), visible.cards()).
		GroupBy(boardcard.FieldColumnID).
		Aggregate(ent.Count(), func(s *entsql.Selector) string {
			return entsql.As("count("+s.C(boardcard.FieldIssueID)+")", "issues")
		}).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]models.BoardColumnCounts, len(rows))
	for _, row := range rows {
		counts[row.ColumnID] = models.BoardColumnCounts{Issues: row.Issues, Discussions: row.Count - row.Issues}
	}
	return counts, nil
}

type CardFilter struct {
	Board  uuid.UUID
	Column *uuid.UUID
	// Discussions lists the discussion cards instead of the issue cards.
	Discussions bool
	Visible     Visible
	Limit       int64
	Offset      int64
}

// Cards is the board's visible issues or discussions, column by column in their order.
func (self *BoardRepository) Cards(ctx context.Context, filter CardFilter) ([]*ent.BoardCard, error) {
	kind := boardcard.IssueIDNotNil()
	if filter.Discussions {
		kind = boardcard.DiscussionIDNotNil()
	}
	query := self.base.DB.BoardCard.Query().Where(boardcard.BoardID(filter.Board), kind, filter.Visible.cards())
	if filter.Column != nil {
		query = query.Where(boardcard.ColumnID(*filter.Column))
	}
	return query.WithColumn().Modify(func(s *entsql.Selector) {
		columns := entsql.Table(boardcolumn.Table).As("c")
		s.Join(columns).On(s.C(boardcard.FieldColumnID), columns.C(boardcolumn.FieldID))
		s.OrderBy(columns.C(boardcolumn.FieldPosition), s.C(boardcard.FieldSortOrder), s.C(boardcard.FieldCreatedAt))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).All(ctx)
}

// Card is where the subject sits on the board, if it does.
func (self *BoardRepository) Card(ctx context.Context, tx repository.TxInterface, boardID uuid.UUID, subject Subject) (*ent.BoardCard, error) {
	found, err := self.base.Client(tx).BoardCard.Query().
		Where(boardcard.BoardID(boardID), subject.is()).WithColumn().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// LastSortOrder is the highest sort order in the column, 0 when it is empty.
func (self *BoardRepository) LastSortOrder(ctx context.Context, tx repository.TxInterface, column uuid.UUID) (float64, error) {
	var rows []struct {
		Max *float64 `json:"max"`
	}
	err := self.base.Client(tx).BoardCard.Query().Where(boardcard.ColumnID(column)).
		Aggregate(ent.Max(boardcard.FieldSortOrder)).Scan(ctx, &rows)
	if err != nil || len(rows) == 0 || rows[0].Max == nil {
		return 0, err
	}
	return *rows[0].Max, nil
}

// Place puts the subject in the column at sortOrder, moving it there when it is already on the board.
func (self *BoardRepository) Place(ctx context.Context, tx repository.TxInterface, boardID, column uuid.UUID, subject Subject, sortOrder float64) error {
	return self.base.Client(tx).BoardCard.Create().
		SetBoardID(boardID).SetColumnID(column).SetNillableIssueID(subject.Issue).SetNillableDiscussionID(subject.Discussion).
		SetSortOrder(sortOrder).
		OnConflictColumns(boardcard.FieldBoardID, subject.column()).
		Update(func(u *ent.BoardCardUpsert) { u.UpdateColumnID().UpdateSortOrder() }).
		Exec(ctx)
}

// Remove takes the subject off the board, saying whether it was on it.
func (self *BoardRepository) Remove(ctx context.Context, tx repository.TxInterface, boardID uuid.UUID, subject Subject) (bool, error) {
	removed, err := self.base.Client(tx).BoardCard.Delete().
		Where(boardcard.BoardID(boardID), subject.is()).Exec(ctx)
	return removed > 0, err
}
