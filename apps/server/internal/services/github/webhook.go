package github_service

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/integrations/github"
	repository "ricrac/internal/repositories"
	github_repo "ricrac/internal/repositories/github"
	events_service "ricrac/internal/services/events"
)

const (
	sourcePullRequest = "github"
	sourceBranch      = "github_branch"
	sourceCommit      = "github_commit"
)

// hook is one webhook delivery for a workspace's installation.
type hook struct {
	*GitHubService
	ctx          context.Context
	workspace    uuid.UUID
	installation int64
}

// Configured says whether the server has a GitHub App.
func (self *GitHubService) Configured() bool {
	return self.app != nil
}

// HandleWebhook applies a delivery signed with the app's webhook secret. Events of installations
// no workspace has are ignored.
func (self *GitHubService) HandleWebhook(ctx context.Context, event, signature string, body []byte) error {
	if !github.ValidSignature(self.app.WebhookSecret, signature, body) {
		return errdefs.Errorf(http.StatusUnauthorized, "X-Hub-Signature-256 doesn't match the webhook secret")
	}
	var envelope github.Envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return errdefs.BadRequest("The body isn't a GitHub event")
	}
	if envelope.Installation == nil {
		return nil
	}
	workspace, found, err := self.github().RecordEvent(ctx, envelope.Installation.ID, envelope.Kind(event))
	if err != nil || !found {
		return err
	}
	h := &hook{self, ctx, workspace, envelope.Installation.ID}
	if envelope.Repository != nil && envelope.Repository.FullName != "" {
		if err := self.github().RenameRepo(ctx, h.installation, envelope.Repository.ID, envelope.Repository.FullName); err != nil {
			return err
		}
	}
	// What the app itself did on GitHub comes back as events; they were Ricrac's changes already.
	fromApp := strings.EqualFold(envelope.Sender.Login, self.app.BotLogin())
	switch event {
	case "installation":
		return h.installationChanged(envelope.Action)
	case "installation_repositories":
		return decodeAnd(body, h.repositoriesChanged)
	case "pull_request":
		return decodeAnd(body, h.pullRequest)
	case "push":
		return decodeAnd(body, h.push)
	case "issues":
		if fromApp {
			return nil
		}
		return decodeAnd(body, h.issuesEvent)
	case "issue_comment":
		if fromApp {
			return nil
		}
		return decodeAnd(body, h.issueComment)
	case "milestone":
		if fromApp {
			return nil
		}
		return decodeAnd(body, h.milestoneEvent)
	}
	return nil
}

func decodeAnd[T any](body []byte, handle func(*T) error) error {
	var event T
	if err := json.Unmarshal(body, &event); err != nil {
		return errdefs.BadRequest("The body isn't a GitHub event")
	}
	return handle(&event)
}

func (h *hook) installationChanged(action string) error {
	switch action {
	case "deleted":
		return h.github().DeleteInstallation(h.ctx, nil, h.installation)
	case "suspend":
		return h.github().SetSuspended(h.ctx, h.installation, true)
	case "unsuspend":
		return h.github().SetSuspended(h.ctx, h.installation, false)
	}
	return nil
}

func (h *hook) repositoriesChanged(event *github.InstallationRepositoriesEvent) error {
	removed := make([]int64, len(event.RepositoriesRemoved))
	for i, repository := range event.RepositoriesRemoved {
		removed[i] = repository.ID
	}
	return h.github().UnsyncRepos(h.ctx, h.installation, removed)
}

/* Linking pull requests, branches and commits */

type move int

const (
	stay move = iota
	moveOpened
	moveMerged
)

// The statuses a move takes issues from: an opened pull request only moves issues nobody started.
var moveFrom = map[move][]string{
	moveOpened: {"triage", "backlog", "unstarted"},
	moveMerged: {"triage", "backlog", "unstarted", "started"},
}

// gitHubAttachment is the attachment a pull request, branch or commit becomes on an issue.
type gitHubAttachment struct {
	URL, Title, SourceType string
	Metadata               M
}

type linkPlan struct {
	label      string
	movedLabel string
	attachment gitHubAttachment
	// An attachment with this URL (the branch a pull request comes from) becomes this one.
	replaceURL string
	move       move
	// A branch isn't linked to issues that have a pull request from it.
	skipWithPullRequestFrom string
}

func (h *hook) pullRequest(event *github.PullRequestEvent) error {
	pr := event.PullRequest
	state := pr.DisplayState()
	number := strconv.Itoa(pr.Number)
	plan := linkPlan{
		attachment: gitHubAttachment{
			URL: pr.HTMLURL, Title: "#" + number + " " + pr.Title, SourceType: sourcePullRequest,
			Metadata: M{"repo": event.Repository.FullName, "number": pr.Number, "state": state, "branch": pr.Head.Ref,
				"author": pr.User.Login, "head_sha": pr.Head.SHA, "base": pr.Base.Ref, "summary": pr.Summary(),
				"additions": pr.Additions, "deletions": pr.Deletions, "changed_files": pr.ChangedFiles, "merged_at": pr.MergedAt},
		},
		replaceURL: github.BranchURL(event.Repository, pr.Head.Ref),
	}
	opens := state == github.StateOpen
	switch event.Action {
	case "opened":
		plan.label = "Pull request #" + number + " opened"
		if state == github.StateDraft {
			plan.label = "Draft pull request #" + number + " opened"
		}
	case "reopened":
		plan.label = "Pull request #" + number + " reopened"
	case "ready_for_review":
		plan.label = "Pull request #" + number + " ready for review"
	case "edited", "synchronize":
		plan.label = "Pull request #" + number + " updated"
		opens = false
	case "closed":
		plan.label = "Pull request #" + number + " closed"
		if state == github.StateMerged {
			plan.label = "Pull request #" + number + " merged"
		}
	default:
		return nil
	}
	if opens {
		plan.move = moveOpened
	}
	found := mentions{
		refs:          github.References(pr.Head.Ref, pr.Title, pr.Body),
		numbers:       github.IssueNumbers(pr.Title, pr.Body),
		repo:          event.Repository.FullName,
		closesNumbers: pr.Base.Ref == event.Repository.DefaultBranch,
	}
	return h.link(found, func(closes bool) linkPlan {
		if state == github.StateMerged && closes {
			merged := plan
			merged.move = moveMerged
			return merged
		}
		return plan
	})
}

func (h *hook) push(event *github.PushEvent) error {
	branch, ok := event.Branch()
	if !ok {
		return nil
	}
	plan := linkPlan{
		label: "Branch " + branch + " pushed",
		attachment: gitHubAttachment{
			URL: github.BranchURL(event.Repository, branch), Title: branch, SourceType: sourceBranch,
			Metadata: M{"repo": event.Repository.FullName, "branch": branch, "author": event.Sender.Login, "head_sha": event.After},
		},
		move:                    moveOpened,
		skipWithPullRequestFrom: branch,
	}
	failures := []error{h.link(mentions{refs: github.References(branch)}, func(bool) linkPlan { return plan })}
	for _, commit := range event.Commits {
		failures = append(failures, h.linkCommit(event, branch, commit))
	}
	return errors.Join(failures...)
}

// linkCommit links a commit to the issues its message mentions. One that says it fixes an issue
// only closes it on the default branch, where the fix ships.
func (h *hook) linkCommit(event *github.PushEvent, branch string, commit github.PushCommit) error {
	closes := event.OnDefaultBranch()
	found := mentions{
		refs:          github.References("", commit.Message),
		numbers:       github.IssueNumbers(commit.Message),
		repo:          event.Repository.FullName,
		closesNumbers: closes,
	}
	if commit.ID == "" || len(found.refs)+len(found.numbers) == 0 {
		return nil
	}
	sha := commit.ShortSHA()
	plan := linkPlan{
		label: "Commit " + sha + " pushed to " + branch,
		attachment: gitHubAttachment{
			URL: commit.HTMLURL(event.Repository), Title: sha + " " + commit.Subject(), SourceType: sourceCommit,
			Metadata: M{"repo": event.Repository.FullName, "sha": commit.ID, "branch": branch, "author": commit.AuthorName()},
		},
		move: moveOpened,
	}
	return h.link(found, func(closesIssue bool) linkPlan {
		if !closes || !closesIssue {
			return plan
		}
		closing := plan
		closing.move = moveMerged
		closing.movedLabel = "Commit " + sha + " on " + branch + " closed the issue"
		return closing
	})
}

// mentions is what a pull request, branch or commit names: Ricrac identifiers, and GitHub issues
// synced with Ricrac (#123 is one of repo). A GitHub issue only moves when a closing phrase names
// it, and only closes when the change reaches the default branch, as on GitHub.
type mentions struct {
	refs          []github.Reference
	numbers       []github.IssueNumber
	repo          string
	closesNumbers bool
}

// link applies a plan to each issue mentioned, each in its own transaction.
func (h *hook) link(found mentions, planFor func(closes bool) linkPlan) error {
	type target struct {
		issue         uuid.UUID
		closes, moves bool
	}
	var targets []target
	add := func(ids []uuid.UUID, closes, moves bool) {
		for _, id := range ids {
			if i := slices.IndexFunc(targets, func(t target) bool { return t.issue == id }); i >= 0 {
				targets[i].closes = targets[i].closes || closes
				targets[i].moves = targets[i].moves || moves
				continue
			}
			targets = append(targets, target{id, closes, moves})
		}
	}
	for _, ref := range found.refs {
		ids, err := h.github().IssuesNamed(h.ctx, h.workspace, ref.Key, ref.Number, ref.Identifier())
		if err != nil {
			return err
		}
		add(ids, ref.Closes, true)
	}
	for _, number := range found.numbers {
		ids, err := h.github().IssuesSyncedAs(h.ctx, h.workspace, cmp.Or(number.Repo, found.repo), number.Number)
		if err != nil {
			return err
		}
		add(ids, number.Closes && found.closesNumbers, number.Closes)
	}
	var failures []error
	for _, t := range targets {
		plan := planFor(t.closes)
		if !t.moves {
			plan.move = stay
		}
		if err := h.linkIssue(t.issue, plan); err != nil {
			slog.Warn("applying a GitHub event to an issue failed", "issue", t.issue, "err", err)
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// lockIssue is the issue, locked until tx ends.
func (self *GitHubService) lockIssue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Issue, error) {
	row, err := self.repo.Issue().GetForUpdate(ctx, tx, id)
	if ent.IsNotFound(err) {
		return nil, errdefs.NotFound("No such issue")
	}
	return row, err
}

func (h *hook) linkIssue(issue uuid.UUID, plan linkPlan) error {
	ctx := h.ctx
	actor := h.access.System(h.workspace, uuid.Nil, auth.Origin{Label: plan.label, URL: plan.attachment.URL})
	return h.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		row, err := h.lockIssue(ctx, tx, issue)
		if err != nil {
			return err
		}
		if plan.skipWithPullRequestFrom != "" {
			repo, _ := plan.attachment.Metadata["repo"].(string)
			skip, err := h.github().HasPullRequestFrom(ctx, tx, issue, sourcePullRequest, repo, plan.skipWithPullRequestFrom)
			if err != nil || skip {
				return err
			}
		}
		linked, err := h.syncAttachment(ctx, tx, actor, row, plan)
		if err != nil {
			return err
		}
		target, err := h.moveTarget(ctx, tx, row, plan.move)
		if err != nil {
			return err
		}
		if target == nil || *target == row.StateID {
			if !linked {
				return nil
			}
			return h.events.History(ctx, tx, actor, issue, "github", M{})
		}
		if plan.movedLabel != "" {
			actor.Origin.Label = plan.movedLabel
		}
		before, err := h.issues.Value(ctx, tx, issue)
		if err != nil {
			return err
		}
		if err := h.issues.SetState(ctx, tx, actor, issue, *target, "github"); err != nil {
			return err
		}
		after, err := h.issues.Value(ctx, tx, issue)
		if err != nil {
			return err
		}
		return h.events.Emit(ctx, tx, actor, "Issue", "update", after, events_service.ChangedFields(before, after), &row.TeamID)
	})
}

// moveTarget is the status a move takes the issue to, nil when it stays.
func (self *GitHubService) moveTarget(ctx context.Context, tx repository.TxInterface, row *ent.Issue, kind move) (*uuid.UUID, error) {
	if kind == stay {
		return nil, nil
	}
	current, err := self.issues.StateType(ctx, tx, row.StateID)
	if err != nil || !slices.Contains(moveFrom[kind], current) {
		return nil, err
	}
	return self.pullRequestState(ctx, tx, row.TeamID, kind)
}

// pullRequestState is where the team's pull requests move issues: the status the team chose, or
// its first started (opened) or completed (merged) status.
func (self *GitHubService) pullRequestState(ctx context.Context, tx repository.TxInterface, team uuid.UUID, kind move) (*uuid.UUID, error) {
	if kind == moveMerged {
		return self.github().PullRequestState(ctx, tx, team, true, "completed")
	}
	return self.github().PullRequestState(ctx, tx, team, false, "started")
}

// syncAttachment adds or updates the issue's attachment for a pull request, branch or commit,
// saying whether it changed in a way worth a history entry.
func (self *GitHubService) syncAttachment(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, plan linkPlan) (bool, error) {
	wanted := plan.attachment
	urls := []string{wanted.URL}
	if plan.replaceURL != "" && plan.replaceURL != wanted.URL {
		urls = append(urls, plan.replaceURL)
	}
	found, err := self.github().AttachmentsAt(ctx, tx, row.ID, urls)
	if err != nil {
		return false, err
	}
	metadata, err := json.Marshal(wanted.Metadata)
	if err != nil {
		return false, err
	}
	attachment := github_repo.NewAttachment{
		Workspace: actor.Workspace, Issue: row.ID, URL: wanted.URL, Title: wanted.Title, Metadata: metadata, SourceType: wanted.SourceType,
	}
	if len(found) == 0 {
		id, err := self.github().AddAttachment(ctx, tx, attachment)
		if err != nil {
			return false, err
		}
		return true, self.emitAttachment(ctx, tx, actor, id, row.TeamID, "create")
	}
	existing := found[0]
	var existingMetadata M
	_ = json.Unmarshal(existing.Metadata, &existingMetadata)
	visible := existing.Title != wanted.Title || existing.URL != wanted.URL || utils.OrEmpty(existing.SourceType) != wanted.SourceType ||
		existingMetadata["state"] != wanted.Metadata["state"]
	if !visible && string(errdefs.Marshal(existingMetadata)) == string(errdefs.Marshal(wanted.Metadata)) {
		return false, nil
	}
	if err := self.github().ChangeAttachment(ctx, tx, existing.ID, attachment); err != nil {
		return false, err
	}
	return visible, self.emitAttachment(ctx, tx, actor, existing.ID, row.TeamID, "update")
}

func (self *GitHubService) emitAttachment(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id, team uuid.UUID, action string) error {
	value, err := self.attachments.Value(ctx, tx, id)
	if err != nil {
		return err
	}
	return self.events.Emit(ctx, tx, actor, "Attachment", action, value, nil, &team)
}
