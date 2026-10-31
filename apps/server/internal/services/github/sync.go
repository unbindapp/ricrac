package github_service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/integrations/github"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	comment_service "ricrac/internal/services/comment"
	issue_service "ricrac/internal/services/issue"
)

func (h *hook) syncedRepo(repository github.Repository) (*ent.GithubRepoSync, error) {
	return h.github().SyncedRepo(h.ctx, h.installation, repository.ID)
}

// recordSync notes on the repository sync whether the last change with GitHub worked.
func (self *GitHubService) recordSync(ctx context.Context, sync uuid.UUID, err error) error {
	var message *string
	if err != nil {
		slog.Warn("syncing with GitHub failed", "repoSync", sync, "err", err)
		message = new(err.Error())
	}
	if recordErr := self.github().RecordSync(ctx, sync, message); recordErr != nil {
		return errors.Join(err, recordErr)
	}
	return err
}

func (h *hook) issuesEvent(event *github.IssuesEvent) error {
	sync, err := h.syncedRepo(event.Repository)
	if sync == nil || err != nil {
		return err
	}
	link, err := h.github().IssueLink(h.ctx, sync.ID, event.Issue.ID)
	if err != nil {
		return err
	}
	switch event.Action {
	case "deleted", "transferred":
		return h.recordSync(h.ctx, sync.ID, h.unlink(link, event))
	case "opened", "edited", "labeled", "unlabeled", "assigned", "unassigned", "closed", "reopened", "milestoned", "demilestoned", "field_added", "field_removed":
	default:
		return nil
	}
	if link == nil {
		_, err := h.importIssue(h.ctx, sync, &event.Issue, true)
		return h.recordSync(h.ctx, sync.ID, err)
	}
	return h.recordSync(h.ctx, sync.ID, h.applyIssueEvent(h.ctx, sync, link, event))
}

// unlink forgets an issue deleted on GitHub, trashing it, or moved to another repository.
func (h *hook) unlink(link *ent.GithubIssueLink, event *github.IssuesEvent) error {
	if link == nil {
		return nil
	}
	if event.Action == "deleted" {
		actor := h.access.System(h.workspace, uuid.Nil,
			auth.Origin{Label: byLine("Deleted on GitHub", uuid.Nil, event.Sender.Login), URL: event.Issue.HTMLURL, FromGitHubIssue: true})
		if _, err := h.issues.Trash(h.ctx, actor, &issue_service.IDInput{ID: link.ID.String()}); err != nil {
			return err
		}
	}
	return h.github().DeleteIssueLink(h.ctx, link.ID)
}

// memberByLogin is the member of the workspace who linked this GitHub account, or uuid.Nil.
func (self *GitHubService) memberByLogin(ctx context.Context, workspace uuid.UUID, account *github.Account) uuid.UUID {
	if account == nil {
		return uuid.Nil
	}
	login, ok := github.NormalizeLogin(account.Login)
	if !ok {
		return uuid.Nil
	}
	id, _, _ := self.github().MemberByLogin(ctx, workspace, login)
	return id
}

// assignable is the member if they can see the team's issues, else uuid.Nil.
func (self *GitHubService) assignable(ctx context.Context, member, team uuid.UUID) uuid.UUID {
	if member == uuid.Nil {
		return uuid.Nil
	}
	ok, err := self.github().CanSeeTeam(ctx, team, member)
	if err != nil || !ok {
		return uuid.Nil
	}
	return member
}

// byLine says who did it on GitHub when they have no Ricrac account to show as the actor.
func byLine(label string, actor uuid.UUID, login string) string {
	if actor != uuid.Nil || login == "" {
		return label
	}
	return label + " by @" + login
}

func clampTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "Untitled GitHub issue"
	}
	if utf8.RuneCountInString(title) > 255 {
		return string([]rune(title)[:255])
	}
	return title
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func some(value string) args.Nullable[string] {
	return args.Nullable[string]{Set: true, Valid: true, Value: value}
}

func samePtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// errLinkedAlready is a GitHub issue found linked once the import waited its turn, like one Ricrac
// opened there.
var errLinkedAlready = errors.New("the GitHub issue is linked already")

// importIssue creates the Ricrac issue of a GitHub issue, linked to it; false when another
// delivery linked it first. With linkBack, GitHub gets the link when the sync wants it.
func (self *GitHubService) importIssue(ctx context.Context, sync *ent.GithubRepoSync, issue *github.Issue, linkBack bool) (bool, error) {
	member := self.memberByLogin(ctx, sync.WorkspaceID, &issue.User)
	label := byLine("Synced from GitHub #"+strconv.Itoa(issue.Number), member, issue.User.Login)
	actor := self.access.System(sync.WorkspaceID, member, auth.Origin{Label: label, URL: issue.HTMLURL, FromGitHubIssue: true})
	status, err := self.statusFor(ctx, sync.TeamID, issue.Closure())
	if err != nil {
		return false, err
	}
	input := issue_service.SaveInput{
		Team: new(sync.TeamID.String()), Title: new(clampTitle(issue.Title)), Description: &issue.Body, State: new(status.String()),
		Labels: []string{},
	}
	for _, label := range issue.Labels {
		id, err := self.labelFor(ctx, actor, sync.TeamID, label)
		if err != nil {
			return false, err
		}
		input.Labels = append(input.Labels, id.String())
	}
	if assignee := self.assignable(ctx, self.memberByLogin(ctx, sync.WorkspaceID, issue.Assignee), sync.TeamID); assignee != uuid.Nil {
		input.Assignee = some(assignee.String())
	}
	if sync.ProjectID != nil {
		input.Project = some(sync.ProjectID.String())
	}
	milestone, err := self.issueMilestone(ctx, sync, issue)
	if err != nil {
		return false, err
	}
	if milestone != nil {
		input.Milestone = some(milestone.String())
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.github().LockNewIssues(ctx, tx, sync.ID); err != nil {
			return err
		}
		if link, err := self.github().IssueLink(ctx, sync.ID, issue.ID); err != nil || link != nil {
			return errors.Join(err, errLinkedAlready)
		}
		id, err := self.issues.Create(ctx, tx, actor, input)
		if err != nil {
			return err
		}
		if err := self.github().AddIssueLink(ctx, tx, id, sync.ID, issue.ID, issue.Number, issue.NodeID, issue.HTMLURL, issue.UpdatedAt); err != nil {
			return err
		}
		if !linkBack || !sync.LinkBack {
			return nil
		}
		return self.github().QueueLinkBack(ctx, tx, id)
	})
	if isUniqueViolation(err) || errors.Is(err, errLinkedAlready) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if linkBack && sync.LinkBack {
		self.waker.WakeGitHub()
	}
	return true, nil
}

// statusFor is the status of the team a GitHub issue in this state goes to: open issues go to
// triage when the team uses it, else the default status; duplicates to a canceled status named
// Duplicate when the team has one.
func (self *GitHubService) statusFor(ctx context.Context, team uuid.UUID, closure github.Closure) (uuid.UUID, error) {
	var kinds []string
	switch closure {
	case github.ClosedCompleted:
		kinds = []string{"completed"}
	case github.ClosedDuplicate:
		duplicate, err := self.github().DuplicateState(ctx, team)
		if err != nil {
			return uuid.Nil, err
		}
		if duplicate != nil {
			return *duplicate, nil
		}
		kinds = []string{"canceled", "completed"}
	case github.ClosedNotPlanned:
		kinds = []string{"canceled", "completed"}
	default:
		triage, err := self.github().TriageEnabled(ctx, team)
		if err != nil {
			return uuid.Nil, err
		}
		if !triage {
			return self.issues.DefaultState(ctx, nil, team)
		}
		kinds = []string{"triage"}
	}
	for _, kind := range kinds {
		id, err := self.issues.FirstStateOfType(ctx, nil, team, kind)
		if err != nil {
			return uuid.Nil, err
		}
		if id != nil {
			return *id, nil
		}
	}
	return self.issues.DefaultState(ctx, nil, team)
}

func matchesClosure(kind string, closure github.Closure) bool {
	switch closure {
	case github.ClosedCompleted:
		return kind == "completed"
	case github.ClosedNotPlanned, github.ClosedDuplicate:
		return kind == "canceled"
	}
	return kind != "completed" && kind != "canceled"
}

// labelFor is the team's or workspace's label named like a GitHub label, created when there's
// none.
func (self *GitHubService) labelFor(ctx context.Context, actor *auth.Actor, team uuid.UUID, wanted github.Label) (uuid.UUID, error) {
	name := strings.TrimSpace(wanted.Name)
	if id, found, err := self.github().LabelNamed(ctx, actor.Workspace, team, name); found || err != nil {
		return id, err
	}
	var id uuid.UUID
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var err error
		if id, err = self.github().CreateLabel(ctx, tx, actor.Workspace, name, github.RicracColor(wanted.Color)); err != nil {
			return err
		}
		value, err := self.labelValue(ctx, tx, id)
		if err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "IssueLabel", "create", value, nil, nil)
	})
	return id, err
}

func (self *GitHubService) labelValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.IssueLabel, error) {
	labels, err := self.repo.Label().Load(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.IssueLabel{}, err
	}
	if len(labels) == 0 {
		return models.IssueLabel{}, errdefs.NotFound("No such label")
	}
	counts, err := self.repo.Label().IssueCounts(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.IssueLabel{}, err
	}
	return models.IssueLabelOf(labels[0], counts[id]), nil
}

// issueRow is the issue a link points to.
func (self *GitHubService) issueRow(ctx context.Context, id uuid.UUID) (*ent.Issue, error) {
	row, err := self.repo.Issue().Get(ctx, nil, id)
	if ent.IsNotFound(err) {
		return nil, errdefs.NotFound("No such issue")
	}
	return row, err
}

func (self *GitHubService) applyIssueEvent(ctx context.Context, sync *ent.GithubRepoSync, link *ent.GithubIssueLink, event *github.IssuesEvent) error {
	issue := &event.Issue
	switch event.Action {
	case "edited", "closed", "reopened":
		if issue.UpdatedAt.Before(link.GithubUpdatedAt) {
			return nil
		}
	}
	member := self.memberByLogin(ctx, sync.WorkspaceID, &event.Sender)
	actor := self.access.System(sync.WorkspaceID, member, auth.Origin{URL: issue.HTMLURL, FromGitHubIssue: true})
	row, err := self.issueRow(ctx, link.ID)
	if err != nil {
		return err
	}
	input, label, err := self.issueChange(ctx, actor, sync, row, event)
	if err != nil || input == nil {
		return err
	}
	actor.Origin.Label = byLine(label, member, event.Sender.Login)
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.issues.Update(ctx, tx, actor, link.ID, *input); err != nil {
			return err
		}
		return self.github().TouchIssueLink(ctx, tx, link.ID, issue.UpdatedAt)
	})
}

// issueChange is what an issues event changes on the Ricrac issue, nil when nothing.
func (self *GitHubService) issueChange(ctx context.Context, actor *auth.Actor, sync *ent.GithubRepoSync, row *ent.Issue, event *github.IssuesEvent) (*issue_service.SaveInput, string, error) {
	issue := &event.Issue
	input := &issue_service.SaveInput{}
	switch event.Action {
	case "edited":
		title := clampTitle(issue.Title)
		if title == row.Title && issue.Body == row.Description {
			return nil, "", nil
		}
		if title != row.Title {
			input.Title = &title
		}
		if issue.Body != row.Description {
			input.Description = &issue.Body
		}
		return input, "Edited on GitHub", nil
	case "labeled":
		if event.Label == nil {
			return nil, "", nil
		}
		id, err := self.labelFor(ctx, actor, row.TeamID, *event.Label)
		if err != nil {
			return nil, "", err
		}
		has, err := self.github().HasLabel(ctx, row.ID, id)
		if err != nil || has {
			return nil, "", err
		}
		input.AddLabels = []string{id.String()}
		return input, "Labeled on GitHub", nil
	case "unlabeled":
		if event.Label == nil {
			return nil, "", nil
		}
		ids, err := self.github().LabelsNamedOn(ctx, row.ID, event.Label.Name)
		if err != nil || len(ids) == 0 {
			return nil, "", err
		}
		for _, id := range ids {
			input.RemoveLabels = append(input.RemoveLabels, id.String())
		}
		return input, "Unlabeled on GitHub", nil
	case "assigned":
		assignee := self.assignable(ctx, self.memberByLogin(ctx, actor.Workspace, event.Assignee), row.TeamID)
		if assignee == uuid.Nil || samePtr(row.AssigneeID, &assignee) {
			return nil, "", nil
		}
		input.Assignee = some(assignee.String())
		return input, "Assigned on GitHub", nil
	case "unassigned":
		if row.AssigneeID == nil || event.Assignee == nil {
			return nil, "", nil
		}
		login, err := self.github().UserLogin(ctx, *row.AssigneeID)
		if err != nil || login == nil || !strings.EqualFold(*login, event.Assignee.Login) {
			return nil, "", err
		}
		input.Assignee = args.Nullable[string]{Set: true}
		return input, "Unassigned on GitHub", nil
	case "closed", "reopened":
		closure := issue.Closure()
		current, err := self.issues.StateType(ctx, nil, row.StateID)
		if err != nil || matchesClosure(current, closure) {
			return nil, "", err
		}
		status, err := self.statusFor(ctx, row.TeamID, closure)
		if err != nil {
			return nil, "", err
		}
		input.State = new(status.String())
		if closure == github.Open {
			return input, "Reopened on GitHub", nil
		}
		return input, "Closed on GitHub", nil
	case "field_added", "field_removed":
		input, err := self.fieldChange(ctx, actor, sync, row, issue.Number)
		return input, "Fields changed on GitHub", err
	case "milestoned", "demilestoned":
		milestone, err := self.issueMilestone(ctx, sync, issue)
		if err != nil || samePtr(milestone, row.MilestoneID) {
			return nil, "", err
		}
		input.Milestone = milestoneInput(milestone)
		if milestone == nil {
			return input, "Milestone removed on GitHub", nil
		}
		return input, "Milestone set on GitHub", nil
	}
	return nil, "", nil
}

/* Comments */

func (h *hook) issueComment(event *github.IssueCommentEvent) error {
	if event.Issue.IsPullRequest() {
		return nil
	}
	switch event.Action {
	case "created", "edited", "deleted":
	default:
		return nil
	}
	sync, err := h.syncedRepo(event.Repository)
	if sync == nil || err != nil {
		return err
	}
	link, err := h.github().IssueLink(h.ctx, sync.ID, event.Issue.ID)
	if err != nil {
		return err
	}
	if link == nil && event.Action == "deleted" {
		return nil
	}
	if link == nil {
		if _, err := h.importIssue(h.ctx, sync, &event.Issue, true); err != nil {
			return h.recordSync(h.ctx, sync.ID, err)
		}
		if link, err = h.github().IssueLink(h.ctx, sync.ID, event.Issue.ID); link == nil || err != nil {
			return h.recordSync(h.ctx, sync.ID, err)
		}
	}
	switch event.Action {
	case "deleted":
		err = h.deleteComment(event.Comment.ID)
	case "edited":
		err = h.editComment(sync, link, &event.Comment)
	default:
		_, err = h.importComment(h.ctx, sync, link, &event.Comment)
	}
	return h.recordSync(h.ctx, sync.ID, err)
}

// githubAuthor is the login of a GitHub comment's author when they have no Ricrac account.
func githubAuthor(member uuid.UUID, comment *github.Comment) *string {
	if member != uuid.Nil || comment.User.Login == "" {
		return nil
	}
	return &comment.User.Login
}

// importComment adds a GitHub comment to the linked issue, unless the app posted it or it is
// there already.
func (self *GitHubService) importComment(ctx context.Context, sync *ent.GithubRepoSync, link *ent.GithubIssueLink, comment *github.Comment) (bool, error) {
	if github.FromRicrac(comment.Body) || strings.TrimSpace(comment.Body) == "" {
		return false, nil
	}
	member := self.memberByLogin(ctx, sync.WorkspaceID, &comment.User)
	actor := self.access.System(sync.WorkspaceID, member, auth.Origin{Label: "Commented on GitHub", URL: comment.HTMLURL, FromGitHubIssue: true})
	imported := false
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.github().LockIssueComments(ctx, tx, link.ID); err != nil {
			return err
		}
		known, err := self.github().CommentLinked(ctx, tx, comment.ID)
		if err != nil || known {
			return err
		}
		imported = true
		id, _, err := self.comments.Create(ctx, tx, actor, comment_service.Target{Column: "issue_id", ID: link.ID}, nil, comment.Body, nil)
		if err != nil {
			return err
		}
		if err := self.github().StampImportedComment(ctx, tx, id, comment.CreatedAt, githubAuthor(member, comment)); err != nil {
			return err
		}
		return self.github().AddCommentLink(ctx, tx, id, link.ID, comment.ID)
	})
	if isUniqueViolation(err) {
		return false, nil
	}
	return imported && err == nil, err
}

func (h *hook) editComment(sync *ent.GithubRepoSync, link *ent.GithubIssueLink, comment *github.Comment) error {
	if github.FromRicrac(comment.Body) || strings.TrimSpace(comment.Body) == "" {
		return nil
	}
	id, found, err := h.github().LinkedComment(h.ctx, comment.ID)
	if err != nil {
		return err
	}
	if !found {
		_, err := h.importComment(h.ctx, sync, link, comment)
		return err
	}
	member := h.memberByLogin(h.ctx, sync.WorkspaceID, &comment.User)
	actor := h.access.System(sync.WorkspaceID, member, auth.Origin{Label: "Edited a comment on GitHub", URL: comment.HTMLURL, FromGitHubIssue: true})
	row, err := h.comments.Load(h.ctx, actor, id.String())
	if err != nil {
		return err
	}
	if comment.Body == row.Body {
		return nil
	}
	return h.repo.WithTx(h.ctx, func(tx repository.TxInterface) error {
		_, err := h.comments.Change(h.ctx, tx, actor, row, comment.Body)
		return err
	})
}

func (h *hook) deleteComment(githubID int64) error {
	id, found, err := h.github().LinkedComment(h.ctx, githubID)
	if err != nil || !found {
		return err
	}
	actor := h.access.System(h.workspace, uuid.Nil, auth.Origin{Label: "Deleted a comment on GitHub", FromGitHubIssue: true})
	row, err := h.comments.Load(h.ctx, actor, id.String())
	if err != nil {
		return err
	}
	return h.repo.WithTx(h.ctx, func(tx repository.TxInterface) error {
		_, err := h.comments.Remove(h.ctx, tx, actor, row)
		return err
	})
}
