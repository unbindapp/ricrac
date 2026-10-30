package issue_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/issue"
	"ricrac/ent/issuelabel"
	"ricrac/ent/issuelabellink"
	"ricrac/ent/issuerelease"
	"ricrac/ent/issueshare"
	"ricrac/ent/issuesubscriber"
	"ricrac/ent/projectmilestone"
	"ricrac/ent/projectteam"
	"ricrac/ent/user"
	"ricrac/ent/workflowstate"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
)

// NextNumber counts a new issue in the team and is its number.
func (self *IssueRepository) NextNumber(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID) (int, error) {
	return scanOne[int](ctx, self.base.Client(tx), "UPDATE teams SET issue_count = issue_count + 1 WHERE id = $1 RETURNING issue_count", teamID)
}

// TopSortOrder puts a new issue above the team's others on the board.
func (self *IssueRepository) TopSortOrder(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID) (float64, error) {
	return scanOne[float64](ctx, self.base.Client(tx), "SELECT coalesce(min(sort_order), 0) - 1000 FROM issues WHERE team_id = $1", teamID)
}

type NewIssue struct {
	Workspace   uuid.UUID
	Team        uuid.UUID
	Number      int
	Title       string
	Description string
	State       uuid.UUID
	Creator     *uuid.UUID
	SortOrder   float64
	Estimate    *float64
}

func (self *IssueRepository) Insert(ctx context.Context, tx repository.TxInterface, input NewIssue) (uuid.UUID, error) {
	created, err := self.base.Client(tx).Issue.Create().
		SetWorkspaceID(input.Workspace).SetTeamID(input.Team).SetNumber(input.Number).SetTitle(input.Title).
		SetDescription(input.Description).SetStateID(input.State).SetPriority(0).SetNillableCreatorID(input.Creator).
		SetSortOrder(input.SortOrder).SetNillableEstimate(input.Estimate).
		Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// StampCreated sets the times a new issue in a state of this type starts with.
func (self *IssueRepository) StampCreated(ctx context.Context, tx repository.TxInterface, id uuid.UUID, kind string, now time.Time) error {
	update := self.base.Client(tx).Issue.UpdateOneID(id)
	stamp := func(applies bool, set func(time.Time) *ent.IssueUpdateOne, clear func() *ent.IssueUpdateOne) {
		if applies {
			set(now)
			return
		}
		clear()
	}
	stamp(kind == "started" || kind == "completed", update.SetStartedAt, update.ClearStartedAt)
	stamp(kind == "completed", update.SetCompletedAt, update.ClearCompletedAt)
	stamp(kind == "canceled", update.SetCanceledAt, update.ClearCanceledAt)
	stamp(kind == "triage", update.SetStartedTriageAt, update.ClearStartedTriageAt)
	return update.Exec(ctx)
}

func (self *IssueRepository) Template(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Template, error) {
	return self.base.Client(tx).Template.Get(ctx, id)
}

// AssignIfUnassigned gives the issue an assignee unless it has one.
func (self *IssueRepository) AssignIfUnassigned(ctx context.Context, tx repository.TxInterface, id, assignee uuid.UUID) error {
	return self.base.Client(tx).Issue.Update().Where(issue.ID(id), issue.AssigneeIDIsNil()).SetAssigneeID(assignee).Exec(ctx)
}

// Change is a change of some of an issue's columns.
type Change func(*ent.IssueUpdateOne)

func SetTitle(title string) Change { return func(u *ent.IssueUpdateOne) { u.SetTitle(title) } }

func SetDescription(description string) Change {
	return func(u *ent.IssueUpdateOne) { u.SetDescription(description) }
}

func SetPriority(priority int) Change { return func(u *ent.IssueUpdateOne) { u.SetPriority(priority) } }

func SetEstimate(estimate *float64) Change {
	return func(u *ent.IssueUpdateOne) {
		if estimate == nil {
			u.ClearEstimate()
			return
		}
		u.SetEstimate(*estimate)
	}
}

func SetAssignee(assignee *uuid.UUID) Change {
	return func(u *ent.IssueUpdateOne) {
		if assignee == nil {
			u.ClearAssigneeID()
			return
		}
		u.SetAssigneeID(*assignee)
	}
}

// SetProject moves the issue to another project (or none), out of its milestone when asked.
func SetProject(project *uuid.UUID, leaveMilestone bool) Change {
	return func(u *ent.IssueUpdateOne) {
		if leaveMilestone {
			u.ClearMilestoneID()
		}
		if project == nil {
			u.ClearProjectID()
			return
		}
		u.SetProjectID(*project)
	}
}

// SetMilestone sets the milestone, and its project when given.
func SetMilestone(milestone, project *uuid.UUID) Change {
	return func(u *ent.IssueUpdateOne) {
		if project != nil {
			u.SetProjectID(*project)
		}
		if milestone == nil {
			u.ClearMilestoneID()
			return
		}
		u.SetMilestoneID(*milestone)
	}
}

func SetCycle(cycle *uuid.UUID) Change {
	return func(u *ent.IssueUpdateOne) {
		if cycle == nil {
			u.ClearCycleID()
			return
		}
		u.SetCycleID(*cycle)
	}
}

func SetParent(parent *uuid.UUID, subIssueSortOrder *float64) Change {
	return func(u *ent.IssueUpdateOne) {
		if parent == nil {
			u.ClearParentID().ClearSubIssueSortOrder()
			return
		}
		u.SetParentID(*parent).SetNillableSubIssueSortOrder(subIssueSortOrder)
	}
}

func SetDueDate(due *time.Time) Change {
	return func(u *ent.IssueUpdateOne) {
		if due == nil {
			u.ClearDueDate()
			return
		}
		u.SetDueDate(*due)
	}
}

// SetSnooze snoozes the issue until a time, by someone, or wakes it.
func SetSnooze(until *time.Time, by uuid.UUID) Change {
	return func(u *ent.IssueUpdateOne) {
		if until == nil {
			u.ClearSnoozedUntil().ClearSnoozedByID()
			return
		}
		u.SetSnoozedUntil(*until).SetSnoozedByID(by)
	}
}

// SetSLA sets when the SLA breaches, starting it if it hadn't, or removes it.
func SetSLA(breaches *time.Time, kind string, startedAt *time.Time) Change {
	return func(u *ent.IssueUpdateOne) {
		if breaches == nil {
			u.ClearSLABreachesAt().ClearSLAType().ClearSLAStartedAt()
			return
		}
		u.SetSLABreachesAt(*breaches).SetSLAType(kind).SetNillableSLAStartedAt(startedAt)
	}
}

func SetSortOrder(order float64) Change { return func(u *ent.IssueUpdateOne) { u.SetSortOrder(order) } }

func SetSubIssueSortOrder(order float64) Change {
	return func(u *ent.IssueUpdateOne) { u.SetSubIssueSortOrder(order) }
}

func Touch(now time.Time) Change { return func(u *ent.IssueUpdateOne) { u.SetUpdatedAt(now) } }

// Update applies changes to the issue.
func (self *IssueRepository) Update(ctx context.Context, tx repository.TxInterface, id uuid.UUID, changes ...Change) error {
	update := self.base.Client(tx).Issue.UpdateOneID(id)
	for _, change := range changes {
		change(update)
	}
	return update.Exec(ctx)
}

// NextSubIssueSortOrder puts a new sub-issue below the parent's others.
func (self *IssueRepository) NextSubIssueSortOrder(ctx context.Context, tx repository.TxInterface, parent uuid.UUID) (float64, error) {
	return scanOne[float64](ctx, self.base.Client(tx),
		"SELECT coalesce(max(sub_issue_sort_order), 0) + 1000 FROM issues WHERE parent_id = $1", parent)
}

// InOwnTree says whether issue is parent or one of its ancestors.
func (self *IssueRepository) InOwnTree(ctx context.Context, tx repository.TxInterface, parent, issue uuid.UUID) (bool, error) {
	return scanOne[bool](ctx, self.base.Client(tx),
		`WITH RECURSIVE up AS (SELECT id, parent_id FROM issues WHERE id = $1
		 UNION SELECT i.id, i.parent_id FROM issues i JOIN up ON i.id = up.parent_id)
		 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, parent, issue)
}

// CanBeAssigned says whether the user sees the team's issues, or this one shared with them.
func (self *IssueRepository) CanBeAssigned(ctx context.Context, tx repository.TxInterface, user, teamID, id uuid.UUID) (bool, error) {
	return scanOne[bool](ctx, self.base.Client(tx),
		`SELECT EXISTS (SELECT 1 FROM teams t JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = $2
			WHERE t.id = $1 AND ((NOT t.private AND m.role <> 'guest')
				OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2)))
		 OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = $3 AND s.user_id = $2)`, teamID, user, id)
}

func (self *IssueRepository) User(ctx context.Context, tx repository.TxInterface, id *uuid.UUID) (*ent.User, error) {
	if id == nil {
		return nil, nil
	}
	return self.base.Client(tx).User.Query().Where(user.ID(*id)).Only(ctx)
}

func (self *IssueRepository) Project(ctx context.Context, tx repository.TxInterface, id *uuid.UUID) (*ent.Project, error) {
	if id == nil {
		return nil, nil
	}
	return self.base.Client(tx).Project.Get(ctx, *id)
}

func (self *IssueRepository) Milestone(ctx context.Context, tx repository.TxInterface, id *uuid.UUID) (*ent.ProjectMilestone, error) {
	if id == nil {
		return nil, nil
	}
	return self.base.Client(tx).ProjectMilestone.Query().Where(projectmilestone.ID(*id)).Only(ctx)
}

func (self *IssueRepository) Cycle(ctx context.Context, tx repository.TxInterface, id *uuid.UUID) (*ent.Cycle, error) {
	if id == nil {
		return nil, nil
	}
	return self.base.Client(tx).Cycle.Query().Where(cycle.ID(*id)).Only(ctx)
}

// Ref is the issue with its team and state, for its short form; nil for none.
func (self *IssueRepository) Ref(ctx context.Context, tx repository.TxInterface, id *uuid.UUID) (*ent.Issue, error) {
	if id == nil {
		return nil, nil
	}
	return self.base.Client(tx).Issue.Query().Where(issue.ID(*id)).WithTeam().WithState().Only(ctx)
}

// AddProjectTeam makes the team one of the project's.
func (self *IssueRepository) AddProjectTeam(ctx context.Context, tx repository.TxInterface, project, teamID uuid.UUID) error {
	return self.base.Client(tx).ProjectTeam.Create().SetProjectID(project).SetTeamID(teamID).
		OnConflictColumns(projectteam.FieldProjectID, projectteam.FieldTeamID).DoNothing().Exec(ctx)
}

// LabelIDs is the labels on the issue.
func (self *IssueRepository) LabelIDs(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := self.base.Client(tx).IssueLabelLink.Query().Where(issuelabellink.IssueID(id)).
		Select(issuelabellink.FieldLabelID).Scan(ctx, &ids)
	return ids, err
}

// LabelsWithGroups is the labels with their groups.
func (self *IssueRepository) LabelsWithGroups(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.IssueLabel, error) {
	return self.base.Client(tx).IssueLabel.Query().Where(issuelabel.IDIn(ids...)).WithParent().All(ctx)
}

// Siblings is the other labels of the group.
func (self *IssueRepository) Siblings(ctx context.Context, tx repository.TxInterface, group, label uuid.UUID) ([]uuid.UUID, error) {
	return self.base.Client(tx).IssueLabel.Query().Where(issuelabel.ParentID(group), issuelabel.IDNEQ(label)).IDs(ctx)
}

// SingleSelectClashes is the single-select groups more than one of the labels belong to.
func (self *IssueRepository) SingleSelectClashes(ctx context.Context, tx repository.TxInterface, labels []uuid.UUID) ([]string, error) {
	return scanAll[string](ctx, self.base.Client(tx),
		`SELECT p.name FROM issue_labels l JOIN issue_labels p ON p.id = l.parent_id
		 WHERE l.id = ANY($1) AND p.group_type = 'singleSelect' GROUP BY p.name HAVING count(*) > 1`, labels)
}

// SetLabels makes the labels the issue's.
func (self *IssueRepository) SetLabels(ctx context.Context, tx repository.TxInterface, id uuid.UUID, labels []uuid.UUID) error {
	client := self.base.Client(tx)
	if _, err := client.ExecContext(ctx, "DELETE FROM issue_label_links WHERE issue_id = $1 AND NOT (label_id = ANY($2))", id, labels); err != nil {
		return err
	}
	return self.AddLabels(ctx, tx, id, labels)
}

func (self *IssueRepository) AddLabels(ctx context.Context, tx repository.TxInterface, id uuid.UUID, labels []uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		"INSERT INTO issue_label_links (issue_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, labels)
	return err
}

// LabelNames is the labels' names, sorted.
func (self *IssueRepository) LabelNames(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]string, error) {
	names, err := self.base.Client(tx).IssueLabel.Query().Where(issuelabel.IDIn(ids...)).
		Order(issuelabel.ByName()).Select(issuelabel.FieldName).Strings(ctx)
	if names == nil {
		names = []string{}
	}
	return names, err
}

// RelatedExists says whether the issues are related, either way.
func (self *IssueRepository) RelatedExists(ctx context.Context, tx repository.TxInterface, a, b uuid.UUID) (bool, error) {
	return scanOne[bool](ctx, self.base.Client(tx),
		`SELECT EXISTS (SELECT 1 FROM issue_relations WHERE type = 'related'
			AND ((issue_id = $1 AND related_issue_id = $2) OR (issue_id = $2 AND related_issue_id = $1)))`, a, b)
}

// Relate adds a relation from one issue to another unless it exists, saying whether it did.
func (self *IssueRepository) Relate(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, kind string, creator uuid.UUID) (bool, error) {
	result, err := self.base.Client(tx).ExecContext(ctx,
		"INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING",
		from, to, kind, creator)
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added > 0, err
}

// Unrelate removes a relation; related ones either way.
func (self *IssueRepository) Unrelate(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, kind string) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`DELETE FROM issue_relations WHERE type = $3 AND ((issue_id = $1 AND related_issue_id = $2)
			OR ($4 AND issue_id = $2 AND related_issue_id = $1))`, from, to, kind, kind == "related")
	return err
}

func (self *IssueRepository) RemoveDuplicateOf(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx, "DELETE FROM issue_relations WHERE issue_id = $1 AND type = 'duplicate'", id)
	return err
}

func (self *IssueRepository) MarkDuplicate(ctx context.Context, tx repository.TxInterface, id, of, creator uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		"INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id) VALUES ($1, $2, 'duplicate', $3)", id, of, creator)
	return err
}

func (self *IssueRepository) Unsubscribe(ctx context.Context, tx repository.TxInterface, id uuid.UUID, users []uuid.UUID) error {
	_, err := self.base.Client(tx).IssueSubscriber.Delete().
		Where(issuesubscriber.IssueID(id), issuesubscriber.UserIDIn(users...)).Exec(ctx)
	return err
}

func (self *IssueRepository) ClearReleases(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	_, err := self.base.Client(tx).IssueRelease.Delete().Where(issuerelease.IssueID(id)).Exec(ctx)
	return err
}

func (self *IssueRepository) AddReleases(ctx context.Context, tx repository.TxInterface, id uuid.UUID, releases []uuid.UUID) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		"INSERT INTO issue_releases (issue_id, release_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, releases)
	return err
}

func (self *IssueRepository) RemoveReleases(ctx context.Context, tx repository.TxInterface, id uuid.UUID, releases []uuid.UUID) error {
	_, err := self.base.Client(tx).IssueRelease.Delete().
		Where(issuerelease.IssueID(id), issuerelease.ReleaseIDIn(releases...)).Exec(ctx)
	return err
}

// SameState is the team's unarchived state named like this one.
func (self *IssueRepository) SameState(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, name string) (*uuid.UUID, error) {
	ids, err := self.base.Client(tx).WorkflowState.Query().
		Where(workflowstate.TeamID(teamID), workflowstate.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(workflowstate.FieldName)+") = lower(?)", name))
		}).
		Limit(1).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &ids[0], nil
}

// MoveTeam gives the issue its number in another team and a state there, out of its cycle,
// keeping its identifier among the previous ones, and takes off the labels of other teams.
func (self *IssueRepository) MoveTeam(ctx context.Context, tx repository.TxInterface, id, teamID uuid.UUID, number int, state uuid.UUID, previous string) error {
	client := self.base.Client(tx)
	if _, err := client.ExecContext(ctx,
		`UPDATE issues SET team_id = $2, number = $3, state_id = $4, cycle_id = NULL,
			previous_identifiers = array_append(previous_identifiers, $5), updated_at = now()
		 WHERE id = $1`, id, teamID, number, state, previous); err != nil {
		return err
	}
	_, err := client.ExecContext(ctx,
		`DELETE FROM issue_label_links l USING issue_labels lb
		 WHERE l.issue_id = $1 AND lb.id = l.label_id AND lb.team_id IS NOT NULL AND lb.team_id <> $2`, id, teamID)
	return err
}

// TeamChildren is the sub-issues not in the trash still in the team.
func (self *IssueRepository) TeamChildren(ctx context.Context, tx repository.TxInterface, parent, teamID uuid.UUID) ([]uuid.UUID, error) {
	return self.base.Client(tx).Issue.Query().
		Where(issue.ParentID(parent), issue.TeamID(teamID), issue.TrashedAtIsNil()).IDs(ctx)
}

// Mark runs a trash, restore, archive or unarchive update, answering the issues it changed.
func (self *IssueRepository) Mark(ctx context.Context, tx repository.TxInterface, sql string, id uuid.UUID) ([]uuid.UUID, error) {
	return scanAll[uuid.UUID](ctx, self.base.Client(tx), sql, id)
}

// Share lets the user see the issue.
func (self *IssueRepository) Share(ctx context.Context, tx repository.TxInterface, id, with, sharer uuid.UUID) error {
	return self.base.Client(tx).IssueShare.Create().SetIssueID(id).SetUserID(with).SetSharerID(sharer).
		OnConflictColumns(issueshare.FieldIssueID, issueshare.FieldUserID).DoNothing().Exec(ctx)
}

func (self *IssueRepository) Unshare(ctx context.Context, tx repository.TxInterface, id, with uuid.UUID) error {
	_, err := self.base.Client(tx).IssueShare.Delete().Where(issueshare.IssueID(id), issueshare.UserID(with)).Exec(ctx)
	return err
}

// PendingGitHub is where an issue queued to open on GitHub goes and the login it would be opened
// as, nil when it isn't queued.
func (self *IssueRepository) PendingGitHub(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*models.GitHubPreview, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT s.full_name, u.github_login FROM github_outbox o
		 JOIN github_repo_syncs s ON s.id = o.repo_sync_id
		 LEFT JOIN users u ON u.id = o.act_as AND EXISTS (SELECT 1 FROM github_user_tokens t WHERE t.user_id = u.id)
		 WHERE o.issue_id = $1 AND o.kind = 'issue_created' LIMIT 1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var preview models.GitHubPreview
	if err := rows.Scan(&preview.Repository, &preview.OpenedAs); err != nil {
		return nil, err
	}
	return &preview, rows.Close()
}
