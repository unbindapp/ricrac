package issue_repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/file"
	"ricrac/ent/issue"
	"ricrac/ent/team"
	repository "ricrac/internal/repositories"
)

// File is an uploaded file of the workspace.
func (self *IssueRepository) File(ctx context.Context, workspace, id uuid.UUID) (*ent.File, error) {
	return self.base.DB.File.Query().Where(file.ID(id), file.WorkspaceID(workspace)).Only(ctx)
}

// LockTeam is the team, locked until the transaction ends.
func (self *IssueRepository) LockTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Team, error) {
	return self.base.Client(tx).Team.Query().Where(team.ID(id)).ForUpdate().Only(ctx)
}

// TakenNumbers is which of the numbers the team's issues have, in order.
func (self *IssueRepository) TakenNumbers(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, numbers []int) ([]int, error) {
	return self.base.Client(tx).Issue.Query().Where(issue.TeamID(teamID), issue.NumberIn(numbers...)).
		Order(issue.ByNumber()).Select(issue.FieldNumber).Ints(ctx)
}

// RaiseIssueCount makes the team's count of issues at least count, and is the count.
func (self *IssueRepository) RaiseIssueCount(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, count int) (int, error) {
	return scanOne[int](ctx, self.base.Client(tx),
		"UPDATE teams SET issue_count = greatest(issue_count, $2) WHERE id = $1 RETURNING issue_count", teamID, count)
}

type ImportedIssue struct {
	Workspace, Team, State, Creator                uuid.UUID
	Number, Priority                               int
	Title, Description                             string
	Estimate                                       *float64
	Assignee, Project, Milestone, Cycle            *uuid.UUID
	DueDate                                        *time.Time
	SortOrder                                      float64
	StartedAt, CompletedAt, CanceledAt, ArchivedAt *time.Time
	CreatedAt, UpdatedAt                           time.Time
}

func (self *IssueRepository) InsertImported(ctx context.Context, tx repository.TxInterface, input ImportedIssue) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Issue.Create().
		SetWorkspaceID(input.Workspace).SetTeamID(input.Team).SetNumber(input.Number).SetTitle(input.Title).
		SetDescription(input.Description).SetStateID(input.State).SetPriority(input.Priority).SetNillableEstimate(input.Estimate).
		SetNillableAssigneeID(input.Assignee).SetCreatorID(input.Creator).SetNillableProjectID(input.Project).
		SetNillableMilestoneID(input.Milestone).SetNillableCycleID(input.Cycle).SetNillableDueDate(input.DueDate).
		SetSortOrder(input.SortOrder).SetNillableStartedAt(input.StartedAt).SetNillableCompletedAt(input.CompletedAt).
		SetNillableCanceledAt(input.CanceledAt).SetNillableArchivedAt(input.ArchivedAt).
		SetCreatedAt(input.CreatedAt).SetUpdatedAt(input.UpdatedAt).
		Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (self *IssueRepository) InsertComment(ctx context.Context, tx repository.TxInterface, workspace, id, author uuid.UUID, body string, at time.Time) error {
	return self.base.Client(tx).Comment.Create().SetWorkspaceID(workspace).SetIssueID(id).SetUserID(author).
		SetBody(body).SetCreatedAt(at).SetUpdatedAt(at).Exec(ctx)
}

// ImportedHistory records the issue's creation by its creator, at the time it was created.
func (self *IssueRepository) ImportedHistory(ctx context.Context, tx repository.TxInterface, id, creator uuid.UUID, changes json.RawMessage, at time.Time) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		"INSERT INTO issue_history (issue_id, actor_id, action, changes, created_at) VALUES ($1, $2, 'created', $3, $4)",
		id, creator, changes, at)
	return err
}

// RelateImported adds a relation unless it (or a related one the other way) exists, saying
// whether it did.
func (self *IssueRepository) RelateImported(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, kind string, creator uuid.UUID) (bool, error) {
	result, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id)
		 SELECT $1, $2, $3, $4 WHERE NOT ($3 = 'related' AND EXISTS (SELECT 1 FROM issue_relations
			WHERE type = 'related' AND issue_id = $2 AND related_issue_id = $1))
		 ON CONFLICT DO NOTHING`, from, to, kind, creator)
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added > 0, err
}
