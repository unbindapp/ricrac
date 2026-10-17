package automation_repo

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/document"
	"ricrac/ent/email"
	"ricrac/ent/file"
	"ricrac/ent/initiative"
	"ricrac/ent/issue"
	"ricrac/ent/notification"
	"ricrac/ent/project"
	"ricrac/ent/team"
	"ricrac/ent/webhookdelivery"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
)

// What happens on its own every minute: auto-archive, auto-close, the trash, snoozes, due dates
// and cleanups.

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i AutomationRepositoryInterface -p automation_repo -s AutomationRepository -o automation_repository_iface.go
type AutomationRepository struct {
	base *repository.BaseRepository
}

func NewAutomationRepository(db *ent.Client) *AutomationRepository {
	return &AutomationRepository{base: repository.NewBaseRepository(db)}
}

func joinTeamAndState(s *entsql.Selector) (*entsql.SelectTable, *entsql.SelectTable) {
	teams := entsql.Table(team.Table)
	states := entsql.Table(workflowstate.Table)
	s.Join(teams).On(s.C(issue.FieldTeamID), teams.C(team.FieldID))
	s.Join(states).On(s.C(issue.FieldStateID), states.C(workflowstate.FieldID))
	return teams, states
}

// AutoArchive archives the closed issues of teams that archive them after some months, noting it
// in their history.
func (self *AutomationRepository) AutoArchive(ctx context.Context, tx repository.TxInterface, now time.Time) error {
	client := self.base.Client(tx)
	ids, err := client.Issue.Query().Where(issue.ArchivedAtIsNil(), issue.TrashedAtIsNil()).Modify(func(s *entsql.Selector) {
		teams, states := joinTeamAndState(s)
		s.Where(entsql.And(entsql.NotNull(teams.C(team.FieldAutoArchiveMonths)), entsql.In(states.C(workflowstate.FieldType), "completed", "canceled")))
		s.Where(repository.Raw("coalesce(" + s.C(issue.FieldCompletedAt) + ", " + s.C(issue.FieldCanceledAt) + ", " + s.C(issue.FieldUpdatedAt) +
			") < now() - make_interval(months => " + teams.C(team.FieldAutoArchiveMonths) + ")"))
		s.Select(s.C(issue.FieldID))
	}).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return err
	}
	if err := client.Issue.Update().Where(issue.IDIn(ids...)).SetArchivedAt(now).SetAutoArchivedAt(now).Exec(ctx); err != nil {
		return err
	}
	entries := make([]*ent.IssueHistoryCreate, len(ids))
	for i, id := range ids {
		entries[i] = client.IssueHistory.Create().SetIssueID(id).SetAction("auto_archived")
	}
	return client.IssueHistory.CreateBulk(entries...).Exec(ctx)
}

type Closable struct {
	Issue, From, To uuid.UUID
}

// Closable is the open issues of teams that close them after some months without a change, with
// the state they're in and the one they close to.
func (self *AutomationRepository) Closable(ctx context.Context, tx repository.TxInterface) ([]Closable, error) {
	var rows []struct {
		ID   uuid.UUID `json:"id"`
		From uuid.UUID `json:"from_state"`
		To   uuid.UUID `json:"to_state"`
	}
	err := self.base.Client(tx).Issue.Query().Where(issue.ArchivedAtIsNil(), issue.TrashedAtIsNil()).Modify(func(s *entsql.Selector) {
		teams, states := joinTeamAndState(s)
		s.Where(entsql.And(entsql.NotNull(teams.C(team.FieldAutoCloseMonths)), entsql.NotNull(teams.C(team.FieldAutoCloseStateID)),
			entsql.In(states.C(workflowstate.FieldType), "triage", "backlog", "unstarted")))
		s.Where(repository.Raw(s.C(issue.FieldUpdatedAt) + " < now() - make_interval(months => " + teams.C(team.FieldAutoCloseMonths) + ")"))
		s.Select(entsql.As(s.C(issue.FieldID), "id"), entsql.As(states.C(workflowstate.FieldID), "from_state"),
			entsql.As(teams.C(team.FieldAutoCloseStateID), "to_state"))
	}).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	closable := make([]Closable, len(rows))
	for i, row := range rows {
		closable[i] = Closable{Issue: row.ID, From: row.From, To: row.To}
	}
	return closable, nil
}

func (self *AutomationRepository) States(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]*ent.WorkflowState, error) {
	states, err := self.base.Client(tx).WorkflowState.Query().Where(workflowstate.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.WorkflowState, len(states))
	for _, s := range states {
		byID[s.ID] = s
	}
	return byID, nil
}

// AutoClose closes the issue to state, noting the change in its history.
func (self *AutomationRepository) AutoClose(ctx context.Context, tx repository.TxInterface, id, state uuid.UUID, changes json.RawMessage, now time.Time) error {
	client := self.base.Client(tx)
	err := client.Issue.UpdateOneID(id).SetStateID(state).SetCanceledAt(now).SetAutoClosedAt(now).SetUpdatedAt(now).Exec(ctx)
	if err != nil {
		return err
	}
	return client.IssueHistory.Create().SetIssueID(id).SetAction("auto_closed").SetChanges(changes).Exec(ctx)
}

// EmptyTrash deletes what has been in the trash for 30 days.
func (self *AutomationRepository) EmptyTrash(ctx context.Context, tx repository.TxInterface, now time.Time) error {
	client := self.base.Client(tx)
	cutoff := now.AddDate(0, 0, -30)
	if _, err := client.Issue.Delete().Where(issue.TrashedAtLT(cutoff)).Exec(ctx); err != nil {
		return err
	}
	if _, err := client.Project.Delete().Where(project.TrashedAtLT(cutoff)).Exec(ctx); err != nil {
		return err
	}
	if _, err := client.Initiative.Delete().Where(initiative.TrashedAtLT(cutoff)).Exec(ctx); err != nil {
		return err
	}
	_, err := client.Document.Delete().Where(document.TrashedAtLT(cutoff)).Exec(ctx)
	return err
}

// WakeSnoozed brings back snoozed notifications, unread, and issues whose snooze ended.
func (self *AutomationRepository) WakeSnoozed(ctx context.Context, tx repository.TxInterface, now time.Time) error {
	client := self.base.Client(tx)
	err := client.Notification.Update().Where(notification.SnoozedUntilLTE(now)).ClearSnoozedUntil().ClearReadAt().Exec(ctx)
	if err != nil {
		return err
	}
	return client.Issue.Update().Where(issue.SnoozedUntilLTE(now)).ClearSnoozedUntil().ClearSnoozedByID().Exec(ctx)
}

// NotifyDue tells assignees about their open issues due today or overdue, once a day.
func (self *AutomationRepository) NotifyDue(ctx context.Context, tx repository.TxInterface) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO notifications (workspace_id, user_id, type, issue_id, summary)
		 SELECT i.workspace_id, i.assignee_id, 'issueDue', i.id,
			issue_identifier(i.id) || CASE WHEN i.due_date < current_date THEN ' is overdue' ELSE ' is due today' END
		 FROM issues i JOIN workflow_states s ON s.id = i.state_id
		 WHERE i.assignee_id IS NOT NULL AND i.due_date <= current_date AND i.trashed_at IS NULL AND i.archived_at IS NULL
		 AND s.type NOT IN ('completed', 'canceled')
		 AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.issue_id = i.id AND n.user_id = i.assignee_id
			AND n.type = 'issueDue' AND n.created_at > now() - interval '1 day')`)
	return err
}

// CleanUp deletes uploads never made, webhook deliveries after 14 days and emails after 30.
func (self *AutomationRepository) CleanUp(ctx context.Context, tx repository.TxInterface, now time.Time) error {
	client := self.base.Client(tx)
	if _, err := client.File.Delete().Where(file.DataIsNil(), file.UploadExpiresAtLT(now)).Exec(ctx); err != nil {
		return err
	}
	if _, err := client.WebhookDelivery.Delete().Where(webhookdelivery.CreatedAtLT(now.AddDate(0, 0, -14))).Exec(ctx); err != nil {
		return err
	}
	_, err := client.Email.Delete().Where(email.CreatedAtLT(now.AddDate(0, 0, -30))).Exec(ctx)
	return err
}
