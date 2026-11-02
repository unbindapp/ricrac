package board_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/board"
	"ricrac/ent/boardcolumn"
	"ricrac/ent/boardissue"
	"ricrac/ent/issue"
	"ricrac/ent/issueshare"
	"ricrac/ent/predicate"
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

// DeleteColumns deletes the columns, taking their issues off the board.
func (self *BoardRepository) DeleteColumns(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := self.base.Client(tx).BoardColumn.Delete().Where(boardcolumn.IDIn(ids...)).Exec(ctx)
	return err
}

// Visible narrows board issues to live issues of these teams, and those shared with viewer.
type Visible struct {
	Teams  []uuid.UUID
	Viewer uuid.UUID
}

func (v Visible) issues() predicate.Issue {
	return issue.And(issue.TrashedAtIsNil(), issue.ArchivedAtIsNil(),
		issue.Or(issue.TeamIDIn(v.Teams...), issue.HasIssueSharesWith(issueshare.UserID(v.Viewer))))
}

// Counts is how many visible issues each column of the boards holds.
func (self *BoardRepository) Counts(ctx context.Context, boards []uuid.UUID, visible Visible) (map[uuid.UUID]int, error) {
	var rows []struct {
		ColumnID uuid.UUID `json:"column_id"`
		Count    int       `json:"count"`
	}
	err := self.base.DB.BoardIssue.Query().
		Where(boardissue.BoardIDIn(boards...), boardissue.HasIssueWith(visible.issues())).
		GroupBy(boardissue.FieldColumnID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.ColumnID] = row.Count
	}
	return counts, nil
}

type CardFilter struct {
	Board   uuid.UUID
	Column  *uuid.UUID
	Visible Visible
	Limit   int64
	Offset  int64
}

// Cards is the board's visible issues, column by column in their order.
func (self *BoardRepository) Cards(ctx context.Context, filter CardFilter) ([]*ent.BoardIssue, error) {
	query := self.base.DB.BoardIssue.Query().
		Where(boardissue.BoardID(filter.Board), boardissue.HasIssueWith(filter.Visible.issues()))
	if filter.Column != nil {
		query = query.Where(boardissue.ColumnID(*filter.Column))
	}
	return query.WithColumn().Modify(func(s *entsql.Selector) {
		columns := entsql.Table(boardcolumn.Table).As("c")
		s.Join(columns).On(s.C(boardissue.FieldColumnID), columns.C(boardcolumn.FieldID))
		s.OrderBy(columns.C(boardcolumn.FieldPosition), s.C(boardissue.FieldSortOrder), s.C(boardissue.FieldCreatedAt))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).All(ctx)
}

// Card is where the issue sits on the board, if it does.
func (self *BoardRepository) Card(ctx context.Context, tx repository.TxInterface, boardID, issueID uuid.UUID) (*ent.BoardIssue, error) {
	found, err := self.base.Client(tx).BoardIssue.Query().
		Where(boardissue.BoardID(boardID), boardissue.IssueID(issueID)).WithColumn().Only(ctx)
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
	err := self.base.Client(tx).BoardIssue.Query().Where(boardissue.ColumnID(column)).
		Aggregate(ent.Max(boardissue.FieldSortOrder)).Scan(ctx, &rows)
	if err != nil || len(rows) == 0 || rows[0].Max == nil {
		return 0, err
	}
	return *rows[0].Max, nil
}

// Place puts the issue in the column at sortOrder, moving it there when it is already on the board.
func (self *BoardRepository) Place(ctx context.Context, tx repository.TxInterface, boardID, column, issueID uuid.UUID, sortOrder float64) error {
	return self.base.Client(tx).BoardIssue.Create().
		SetBoardID(boardID).SetColumnID(column).SetIssueID(issueID).SetSortOrder(sortOrder).
		OnConflictColumns(boardissue.FieldBoardID, boardissue.FieldIssueID).
		Update(func(u *ent.BoardIssueUpsert) { u.UpdateColumnID().UpdateSortOrder() }).
		Exec(ctx)
}

// Remove takes the issue off the board, saying whether it was on it.
func (self *BoardRepository) Remove(ctx context.Context, tx repository.TxInterface, boardID, issueID uuid.UUID) (bool, error) {
	removed, err := self.base.Client(tx).BoardIssue.Delete().
		Where(boardissue.BoardID(boardID), boardissue.IssueID(issueID)).Exec(ctx)
	return removed > 0, err
}
