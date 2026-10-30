package events_repo

import (
	"context"
	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/githubreposync"
	"ricrac/ent/issuesubscriber"
	"ricrac/ent/user"
	"ricrac/ent/webhook"
	"ricrac/ent/workspacemember"
	repository "ricrac/internal/repositories"
)

// What happens around a change, written in its transaction: webhook deliveries, issue history,
// the audit log, notifications, subscriptions and changes queued for GitHub.

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i EventsRepositoryInterface -p events_repo -s EventsRepository -o events_repository_iface.go
type EventsRepository struct {
	base *repository.BaseRepository
}

func NewEventsRepository(db *ent.Client) *EventsRepository {
	return &EventsRepository{base: repository.NewBaseRepository(db)}
}

// Webhooks is the enabled webhooks of the workspace that listen to kind on the team, or on
// every team.
func (self *EventsRepository) Webhooks(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, kind string, team *uuid.UUID) ([]uuid.UUID, error) {
	return self.base.Client(tx).Webhook.Query().Where(webhook.WorkspaceID(workspace), webhook.Enabled(true), func(s *entsql.Selector) {
		s.Where(repository.Raw("? = ANY("+s.C(webhook.FieldResourceTypes)+")", kind))
		teamMatches := entsql.IsNull(s.C(webhook.FieldTeamID))
		if team != nil {
			teamMatches = entsql.Or(teamMatches, entsql.EQ(s.C(webhook.FieldTeamID), *team))
		}
		s.Where(teamMatches)
	}).IDs(ctx)
}

func (self *EventsRepository) QueueDelivery(ctx context.Context, tx repository.TxInterface, hook uuid.UUID, payload json.RawMessage) error {
	return self.base.Client(tx).WebhookDelivery.Create().SetWebhookID(hook).SetPayload(payload).Exec(ctx)
}

func (self *EventsRepository) AddHistory(ctx context.Context, tx repository.TxInterface, issue uuid.UUID, actor *uuid.UUID, action string, changes json.RawMessage) error {
	return self.base.Client(tx).IssueHistory.Create().SetIssueID(issue).SetNillableActorID(actor).SetAction(action).SetChanges(changes).Exec(ctx)
}

func (self *EventsRepository) AddAudit(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, actor *uuid.UUID, kind string, metadata json.RawMessage) error {
	return self.base.Client(tx).AuditEntry.Create().SetWorkspaceID(workspace).SetNillableActorID(actor).SetType(kind).SetMetadata(metadata).Exec(ctx)
}

// IsActiveMember says whether the user is an active member of the workspace.
func (self *EventsRepository) IsActiveMember(ctx context.Context, tx repository.TxInterface, workspace, user uuid.UUID) (bool, error) {
	return self.base.Client(tx).WorkspaceMember.Query().
		Where(workspacemember.WorkspaceID(workspace), workspacemember.UserID(user), workspacemember.Active(true)).Exist(ctx)
}

type Notification struct {
	Workspace, User                                             uuid.UUID
	Actor                                                       *uuid.UUID
	Kind, Summary                                               string
	Issue, Comment, Project, Initiative, Document, StatusUpdate *uuid.UUID
	Discussion, DiscussionThread                                *uuid.UUID
}

func (self *EventsRepository) AddNotification(ctx context.Context, tx repository.TxInterface, n Notification) error {
	return self.base.Client(tx).Notification.Create().
		SetWorkspaceID(n.Workspace).SetUserID(n.User).SetNillableActorID(n.Actor).SetType(n.Kind).
		SetNillableIssueID(n.Issue).SetNillableCommentID(n.Comment).SetNillableProjectID(n.Project).
		SetNillableInitiativeID(n.Initiative).SetNillableDocumentID(n.Document).SetNillableStatusUpdateID(n.StatusUpdate).
		SetNillableDiscussionID(n.Discussion).SetNillableDiscussionThreadID(n.DiscussionThread).
		SetSummary(n.Summary).Exec(ctx)
}

func (self *EventsRepository) IssueSubscribers(ctx context.Context, tx repository.TxInterface, issue uuid.UUID) ([]uuid.UUID, error) {
	rows, err := self.base.Client(tx).IssueSubscriber.Query().Where(issuesubscriber.IssueID(issue)).All(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		users[i] = row.UserID
	}
	return users, nil
}

func (self *EventsRepository) Subscribe(ctx context.Context, tx repository.TxInterface, issue uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	creates := make([]*ent.IssueSubscriberCreate, len(users))
	for i, u := range users {
		creates[i] = self.base.Client(tx).IssueSubscriber.Create().SetIssueID(issue).SetUserID(u)
	}
	return self.base.Client(tx).IssueSubscriber.CreateBulk(creates...).OnConflict().DoNothing().Exec(ctx)
}

// Mentioned is the active members whose display name, email or the part of their email before
// the @ is one of handles.
func (self *EventsRepository) Mentioned(ctx context.Context, workspace uuid.UUID, handles []string) ([]uuid.UUID, error) {
	return self.base.DB.User.Query().Modify(func(s *entsql.Selector) {
		members := entsql.Table(workspacemember.Table)
		s.Join(members).On(s.C(user.FieldID), members.C(workspacemember.FieldUserID)).
			Where(entsql.And(entsql.EQ(members.C(workspacemember.FieldWorkspaceID), workspace), entsql.EQ(members.C(workspacemember.FieldActive), true)))
		s.Where(repository.Raw("lower("+s.C(user.FieldDisplayName)+") = ANY(?) OR lower("+s.C(user.FieldEmail)+") = ANY(?) OR lower(split_part("+s.C(user.FieldEmail)+", '@', 1)) = ANY(?)",
			handles, handles, handles))
		s.Select(s.C(user.FieldID))
	}).IDs(ctx)
}

// QueueIssueForGitHub queues the issue's changes for the GitHub issue it is linked to, saying
// whether there was one.
func (self *EventsRepository) QueueIssueForGitHub(ctx context.Context, tx repository.TxInterface, issue uuid.UUID, changes json.RawMessage) (bool, error) {
	result, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, kind, changes)
		 SELECT repo_sync_id, github_number, issue_id, 'issue', $2 FROM github_issue_links WHERE issue_id = $1`, issue, changes)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// QueueMilestoneForGitHub queues a milestone's change or deletion for each GitHub milestone it is
// linked to but the one of except, saying whether anything was queued.
func (self *EventsRepository) QueueMilestoneForGitHub(ctx context.Context, tx repository.TxInterface, milestone uuid.UUID, kind string, except *uuid.UUID) (bool, error) {
	result, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO github_outbox (repo_sync_id, github_number, milestone_id, kind)
		 SELECT repo_sync_id, github_number, milestone_id, $2 FROM github_milestone_links
		 WHERE milestone_id = $1 AND repo_sync_id IS DISTINCT FROM $3`, milestone, kind, except)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// QueueNewIssueForGitHub queues a new issue for one of the team's synced repositories: the one
// named, else (when the team opens new issues on GitHub) the one synced with the issue's project,
// else the one with the most linked issues. It says whether there was one.
func (self *EventsRepository) QueueNewIssueForGitHub(ctx context.Context, tx repository.TxInterface, issue, team uuid.UUID, named *string, actAs *uuid.UUID) (bool, error) {
	result, err := self.base.Client(tx).ExecContext(ctx,
		`INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, kind, act_as)
		 SELECT s.id, 0, i.id, 'issue_created', $4 FROM github_repo_syncs s
		 JOIN issues i ON i.id = $1 JOIN teams t ON t.id = s.team_id
		 WHERE s.team_id = $2
		   AND ($3::text IS NULL AND t.github_open_issues
		     OR lower(s.full_name) = lower($3) OR lower(split_part(s.full_name, '/', 2)) = lower($3))
		 ORDER BY coalesce(s.project_id = i.project_id, false) DESC,
		   (SELECT count(*) FROM github_issue_links l WHERE l.repo_sync_id = s.id) DESC, s.created_at
		 LIMIT 1`, issue, team, named, actAs)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// TeamRepos is the names of the team's synced repositories.
func (self *EventsRepository) TeamRepos(ctx context.Context, tx repository.TxInterface, team uuid.UUID) ([]string, error) {
	return self.base.Client(tx).GithubRepoSync.Query().
		Where(githubreposync.TeamID(team)).
		Order(ent.Asc(githubreposync.FieldFullName)).Select(githubreposync.FieldFullName).Strings(ctx)
}

// QueueCommentForGitHub queues a new comment on a linked issue, or the edit or deletion of a
// comment the app posted (with its replies), saying whether anything was queued.
func (self *EventsRepository) QueueCommentForGitHub(ctx context.Context, tx repository.TxInterface, issue, comment uuid.UUID, kind string) (bool, error) {
	sql := `INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, comment_id, kind)
		SELECT repo_sync_id, github_number, issue_id, $2, $3 FROM github_issue_links WHERE issue_id = $1`
	if kind != "comment_created" {
		sql = `INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, comment_id, github_comment_id, kind, act_as)
			SELECT l.repo_sync_id, l.github_number, l.issue_id, cl.comment_id, cl.github_id, $3, cl.posted_as
			FROM github_comment_links cl JOIN github_issue_links l ON l.issue_id = cl.issue_id
			WHERE l.issue_id = $1 AND cl.posted AND cl.comment_id IN (SELECT id FROM comments WHERE id = $2 OR parent_id = $2)`
	}
	result, err := self.base.Client(tx).ExecContext(ctx, sql, issue, comment, kind)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
