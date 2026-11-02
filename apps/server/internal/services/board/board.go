// Package board_service is boards: columns of one's own choosing that issues are placed in,
// apart from their workflow status.
package board_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	board_repo "ricrac/internal/repositories/board"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	resolve_service "ricrac/internal/services/resolve"
)

var defaultColumns = []string{"Ideas", "Backlog", "Doing", "Done"}

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i BoardServiceInterface -p board_service -s BoardService -o board_service_iface.go
type BoardService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
	issues  issue_service.IssueServiceInterface
}

func NewBoardService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, issues issue_service.IssueServiceInterface) *BoardService {
	return &BoardService{cfg: cfg, repo: repo, resolve: resolve, events: events, issues: issues}
}

type ListInput struct {
	Query *string `json:"query" doc:"Words in the name or description."`
	args.Page
}

type IDInput struct {
	ID string `json:"id" doc:"The board's id, slug, link or name."`
}

type ColumnInput struct {
	ID    *string `json:"id" doc:"Only to keep or rename a column: its id. Without it, a column of the same name is kept, else a new one is added."`
	Name  string  `json:"name"`
	Color *string `json:"color" doc:"Hex color like #5e6ad2."`
}

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to update a board: its id, slug, link or name. Omit to create one."`
	Name        *string               `json:"name" doc:"Required to create."`
	Description args.Nullable[string] `json:"description"`
	Columns     []ColumnInput         `json:"columns" doc:"All the board's columns, in order. Columns left out are removed and their issues taken off the board (the issues stay). New boards start with Ideas, Backlog, Doing and Done."`
}

type ListIssuesInput struct {
	Board  string  `json:"board" doc:"The board's id, slug, link or name."`
	Column *string `json:"column" doc:"Only this column (name or id)."`
	args.Page
}

type PlaceInput struct {
	Board     string   `json:"board" doc:"The board's id, slug, link or name."`
	Issue     string   `json:"issue" doc:"The issue: identifier like ENG-123, id or link."`
	Column    string   `json:"column" doc:"The column (name or id) to put it in."`
	SortOrder *float64 `json:"sortOrder" doc:"Order in the column, lowest first. By default it keeps its place when already in the column, else goes last."`
}

type RemoveInput struct {
	Board string `json:"board" doc:"The board's id, slug, link or name."`
	Issue string `json:"issue" doc:"The issue: identifier like ENG-123, id or link."`
}

func visibleTo(ctx context.Context, actor *auth.Actor) (board_repo.Visible, error) {
	teams, err := actor.TeamIDs(ctx)
	return board_repo.Visible{Teams: teams, Viewer: actor.User}, err
}

func (self *BoardService) values(ctx context.Context, actor *auth.Actor, boards []*ent.Board) ([]models.Board, error) {
	visible, err := visibleTo(ctx, actor)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(boards))
	for i, b := range boards {
		ids[i] = b.ID
	}
	counts, err := self.repo.Board().Counts(ctx, ids, visible)
	if err != nil {
		return nil, err
	}
	values := make([]models.Board, len(boards))
	for i, b := range boards {
		values[i] = models.BoardOf(b, counts, self.cfg.PublicURL)
	}
	return values, nil
}

func (self *BoardService) value(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.Board, error) {
	board, err := self.repo.Board().Load(ctx, tx, id)
	if ent.IsNotFound(err) {
		return models.Board{}, errdefs.NotFound("No such board")
	}
	if err != nil {
		return models.Board{}, err
	}
	values, err := self.values(ctx, actor, []*ent.Board{board})
	if err != nil {
		return models.Board{}, err
	}
	return values[0], nil
}

func (self *BoardService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Board], error) {
	filter := board_repo.ListFilter{Workspace: actor.Workspace}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Pattern = new("%" + utils.EscapeLike(query) + "%")
	}
	var err error
	if filter.Offset, err = input.Offset(); err != nil {
		return args.PageOf[models.Board]{}, err
	}
	filter.Limit = input.Size() + 1
	ids, err := self.repo.Board().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.Board]{}, err
	}
	boards, err := self.repo.Board().LoadAll(ctx, ids)
	if err != nil {
		return args.PageOf[models.Board]{}, err
	}
	values, err := self.values(ctx, actor, boards)
	if err != nil {
		return args.PageOf[models.Board]{}, err
	}
	return args.WrapOf(input.Page, values)
}

func (self *BoardService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Board, error) {
	id, err := self.resolve.Board(ctx, actor, input.ID)
	if err != nil {
		return models.Board{}, err
	}
	return self.value(ctx, nil, actor, id)
}

type checkedColumn struct {
	id    *uuid.UUID
	name  string
	color *string
}

func checkColumns(inputs []ColumnInput) ([]checkedColumn, error) {
	if len(inputs) == 0 {
		return nil, errdefs.BadRequest("A board needs at least one column")
	}
	columns := make([]checkedColumn, len(inputs))
	seen := map[string]bool{}
	for i, input := range inputs {
		name, err := utils.NonEmpty(input.Name, "column name")
		if err != nil {
			return nil, err
		}
		if seen[strings.ToLower(name)] {
			return nil, errdefs.BadRequest("Two columns are named %q", name)
		}
		seen[strings.ToLower(name)] = true
		columns[i] = checkedColumn{name: name}
		if input.Color != nil {
			color, err := utils.CheckColor(*input.Color)
			if err != nil {
				return nil, err
			}
			columns[i].color = &color
		}
		if input.ID == nil {
			continue
		}
		id, ok := utils.AsUUID(*input.ID)
		if !ok {
			return nil, errdefs.BadRequest("Column id %q isn't an id", *input.ID)
		}
		columns[i].id = &id
	}
	return columns, nil
}

func (self *BoardService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.Board, error) {
	var columns []checkedColumn
	if input.Columns != nil {
		var err error
		if columns, err = checkColumns(input.Columns); err != nil {
			return models.Board{}, err
		}
	}
	var value models.Board
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		id, action, err := self.createOrFind(ctx, tx, actor, input)
		if err != nil {
			return err
		}
		if action == "create" && columns == nil {
			for _, name := range defaultColumns {
				columns = append(columns, checkedColumn{name: name})
			}
		}
		if err := self.apply(ctx, tx, id, input, columns); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Board", action, value, nil, nil)
	})
	return value, err
}

func (self *BoardService) createOrFind(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, string, error) {
	if input.ID != nil {
		id, err := self.resolve.Board(ctx, actor, *input.ID)
		return id, "update", err
	}
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, "", err
	}
	id, err := self.repo.Board().Create(ctx, tx, board_repo.NewBoard{Workspace: actor.Workspace, Creator: actor.User, SlugID: utils.SlugID(), Name: name})
	return id, "create", err
}

func (self *BoardService) apply(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input *SaveInput, columns []checkedColumn) error {
	var changes []board_repo.Change
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		changes = append(changes, board_repo.SetName(name))
	}
	if input.Description.Set {
		changes = append(changes, board_repo.SetDescription(utils.Trimmed(input.Description.Ptr())))
	}
	if columns != nil {
		if err := self.setColumns(ctx, tx, id, columns); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return self.repo.Board().Update(ctx, tx, id, append(changes, board_repo.Touch(now))...)
}

// setColumns makes the board's columns these, keeping those named by id or by name.
func (self *BoardService) setColumns(ctx context.Context, tx repository.TxInterface, board uuid.UUID, columns []checkedColumn) error {
	existing, err := self.repo.Board().Columns(ctx, tx, board)
	if err != nil {
		return err
	}
	kept := make([]*uuid.UUID, len(columns))
	unclaimed := map[uuid.UUID]*ent.BoardColumn{}
	for _, c := range existing {
		unclaimed[c.ID] = c
	}
	for i, column := range columns {
		if column.id == nil {
			continue
		}
		if _, ok := unclaimed[*column.id]; !ok {
			return errdefs.BadRequest("The board has no column %s", *column.id)
		}
		kept[i] = column.id
		delete(unclaimed, *column.id)
	}
	for i, column := range columns {
		if kept[i] != nil {
			continue
		}
		for _, c := range existing {
			if _, ok := unclaimed[c.ID]; ok && strings.EqualFold(c.Name, column.name) {
				kept[i] = &c.ID
				delete(unclaimed, c.ID)
				break
			}
		}
	}
	removed := make([]uuid.UUID, 0, len(unclaimed))
	for id := range unclaimed {
		removed = append(removed, id)
	}
	if err := self.repo.Board().DeleteColumns(ctx, tx, removed); err != nil {
		return err
	}
	for i, column := range columns {
		position := float64(i + 1)
		if kept[i] == nil {
			if err := self.repo.Board().CreateColumn(ctx, tx, board, column.name, column.color, position); err != nil {
				return err
			}
			continue
		}
		if err := self.repo.Board().UpdateColumn(ctx, tx, *kept[i], column.name, column.color, position); err != nil {
			return err
		}
	}
	return nil
}

func (self *BoardService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedBoard, error) {
	id, err := self.resolve.Board(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedBoard{}, err
	}
	var value models.Board
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		if err := self.repo.Board().Delete(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Board", "remove", value, nil, nil)
	})
	if err != nil {
		return models.DeletedBoard{}, err
	}
	return models.DeletedBoard{Deleted: true, ID: id.String(), Name: value.Name}, nil
}

// column is a column of the board by id or name.
func (self *BoardService) column(ctx context.Context, tx repository.TxInterface, board uuid.UUID, value string) (*ent.BoardColumn, error) {
	columns, err := self.repo.Board().Columns(ctx, tx, board)
	if err != nil {
		return nil, err
	}
	value = strings.TrimSpace(value)
	id, isID := utils.AsUUID(value)
	names := make([]string, len(columns))
	for i, c := range columns {
		if isID && c.ID == id || strings.EqualFold(c.Name, value) {
			return c, nil
		}
		names[i] = c.Name
	}
	return nil, errdefs.BadRequest("The board has no column %q. %s", value, utils.Options(names))
}

func (self *BoardService) ListIssues(ctx context.Context, actor *auth.Actor, input *ListIssuesInput) (args.PageOf[models.BoardCard], error) {
	id, err := self.resolve.Board(ctx, actor, input.Board)
	if err != nil {
		return args.PageOf[models.BoardCard]{}, err
	}
	board, err := self.repo.Board().Load(ctx, nil, id)
	if err != nil {
		return args.PageOf[models.BoardCard]{}, err
	}
	visible, err := visibleTo(ctx, actor)
	if err != nil {
		return args.PageOf[models.BoardCard]{}, err
	}
	filter := board_repo.CardFilter{Board: id, Visible: visible}
	if input.Column != nil {
		column, err := self.column(ctx, nil, id, *input.Column)
		if err != nil {
			return args.PageOf[models.BoardCard]{}, err
		}
		filter.Column = &column.ID
	}
	if filter.Offset, err = input.Offset(); err != nil {
		return args.PageOf[models.BoardCard]{}, err
	}
	filter.Limit = input.Size() + 1
	cards, err := self.repo.Board().Cards(ctx, filter)
	if err != nil {
		return args.PageOf[models.BoardCard]{}, err
	}
	ids := make([]uuid.UUID, len(cards))
	for i, c := range cards {
		ids[i] = c.IssueID
	}
	issues, err := self.issues.Values(ctx, nil, ids, false)
	if err != nil {
		return args.PageOf[models.BoardCard]{}, err
	}
	byID := make(map[string]models.Issue, len(issues))
	for _, issue := range issues {
		byID[issue.ID] = issue
	}
	values := make([]models.BoardCard, 0, len(cards))
	for _, c := range cards {
		issue, ok := byID[models.ID(c.IssueID)]
		if !ok {
			continue
		}
		values = append(values, models.BoardCard{
			Board: models.BoardRefOf(board), Column: models.BoardColumnRefOf(c.Edges.Column), SortOrder: c.SortOrder, Issue: issue,
		})
	}
	return args.WrapOf(input.Page, values)
}

func (self *BoardService) Place(ctx context.Context, actor *auth.Actor, input *PlaceInput) (models.BoardCard, error) {
	boardID, err := self.resolve.Board(ctx, actor, input.Board)
	if err != nil {
		return models.BoardCard{}, err
	}
	issueID, err := self.resolve.Issue(ctx, actor, input.Issue)
	if err != nil {
		return models.BoardCard{}, err
	}
	var value models.BoardCard
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		column, err := self.column(ctx, tx, boardID, input.Column)
		if err != nil {
			return err
		}
		current, err := self.repo.Board().Card(ctx, tx, boardID, issueID)
		if err != nil {
			return err
		}
		sortOrder, err := self.sortOrder(ctx, tx, column.ID, current, input.SortOrder)
		if err != nil {
			return err
		}
		if err := self.repo.Board().Place(ctx, tx, boardID, column.ID, issueID, sortOrder); err != nil {
			return err
		}
		if value, err = self.card(ctx, tx, boardID, column, issueID, sortOrder); err != nil {
			return err
		}
		action := "update"
		if current == nil {
			action = "create"
		}
		return self.events.Emit(ctx, tx, actor, "BoardCard", action, value, nil, nil)
	})
	return value, err
}

func (self *BoardService) sortOrder(ctx context.Context, tx repository.TxInterface, column uuid.UUID, current *ent.BoardIssue, wanted *float64) (float64, error) {
	if wanted != nil {
		return *wanted, nil
	}
	if current != nil && current.ColumnID == column {
		return current.SortOrder, nil
	}
	last, err := self.repo.Board().LastSortOrder(ctx, tx, column)
	return last + 1, err
}

func (self *BoardService) card(ctx context.Context, tx repository.TxInterface, boardID uuid.UUID, column *ent.BoardColumn, issueID uuid.UUID, sortOrder float64) (models.BoardCard, error) {
	board, err := self.repo.Board().Load(ctx, tx, boardID)
	if err != nil {
		return models.BoardCard{}, err
	}
	issue, err := self.issues.Value(ctx, tx, issueID)
	if err != nil {
		return models.BoardCard{}, err
	}
	return models.BoardCard{Board: models.BoardRefOf(board), Column: models.BoardColumnRefOf(column), SortOrder: sortOrder, Issue: issue}, nil
}

func (self *BoardService) Remove(ctx context.Context, actor *auth.Actor, input *RemoveInput) (models.RemovedFromBoard, error) {
	boardID, err := self.resolve.Board(ctx, actor, input.Board)
	if err != nil {
		return models.RemovedFromBoard{}, err
	}
	issueID, err := self.resolve.Issue(ctx, actor, input.Issue)
	if err != nil {
		return models.RemovedFromBoard{}, err
	}
	var value models.RemovedFromBoard
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		current, err := self.repo.Board().Card(ctx, tx, boardID, issueID)
		if err != nil {
			return err
		}
		board, err := self.repo.Board().Load(ctx, tx, boardID)
		if err != nil {
			return err
		}
		issue, err := self.issues.Value(ctx, tx, issueID)
		if err != nil {
			return err
		}
		value = models.RemovedFromBoard{
			Board: models.BoardRefOf(board),
			Issue: models.IssueRef{ID: issue.ID, Identifier: issue.Identifier, Title: issue.Title, State: issue.State},
		}
		if current == nil {
			return nil
		}
		if _, err := self.repo.Board().Remove(ctx, tx, boardID, issueID); err != nil {
			return err
		}
		value.Removed = true
		card := models.BoardCard{Board: value.Board, Column: models.BoardColumnRefOf(current.Edges.Column), SortOrder: current.SortOrder, Issue: issue}
		return self.events.Emit(ctx, tx, actor, "BoardCard", "remove", card, nil, nil)
	})
	return value, err
}
