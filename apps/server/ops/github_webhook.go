package ops

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
	"ricrac/internal/integrations/github"
)

// GitHub's webhook events: the installation's lifecycle, pull requests, branches and commits that
// mention issues, and the issues and comments of synced repositories (github_sync.go).

const (
	sourcePullRequest = "github"
	sourceBranch      = "github_branch"
	sourceCommit      = "github_commit"
)

// hook is one webhook delivery for a workspace's installation.
type hook struct {
	ctx          context.Context
	state        *app.State
	workspace    uuid.UUID
	installation int64
}

// HandleGitHubWebhook applies a delivery whose signature the server checked. Events of
// installations no workspace has are ignored.
func HandleGitHubWebhook(ctx context.Context, state *app.State, event string, body []byte) error {
	var envelope github.Envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return app.BadRequest("The body isn't a GitHub event")
	}
	if envelope.Installation == nil {
		return nil
	}
	workspace, found, err := app.Maybe[uuid.UUID](ctx, state.DB,
		"UPDATE github_installations SET last_event = $2, last_event_at = now() WHERE id = $1 RETURNING workspace_id",
		envelope.Installation.ID, envelope.Kind(event))
	if err != nil || !found {
		return err
	}
	h := &hook{ctx, state, workspace, envelope.Installation.ID}
	if envelope.Repository != nil && envelope.Repository.FullName != "" {
		err := app.Exec(ctx, state.DB, "UPDATE github_repo_syncs SET full_name = $3 WHERE installation_id = $1 AND repo_id = $2 AND full_name <> $3",
			h.installation, envelope.Repository.ID, envelope.Repository.FullName)
		if err != nil {
			return err
		}
	}
	// What the app itself did on GitHub comes back as events; they were Ricrac's changes already.
	fromApp := state.GitHub != nil && strings.EqualFold(envelope.Sender.Login, state.GitHub.BotLogin())
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
		return decodeAnd(body, h.issues)
	case "issue_comment":
		if fromApp {
			return nil
		}
		return decodeAnd(body, h.issueComment)
	}
	return nil
}

func decodeAnd[T any](body []byte, handle func(*T) error) error {
	var event T
	if err := json.Unmarshal(body, &event); err != nil {
		return app.BadRequest("The body isn't a GitHub event")
	}
	return handle(&event)
}

func (h *hook) installationChanged(action string) error {
	switch action {
	case "deleted":
		return app.Exec(h.ctx, h.state.DB, "DELETE FROM github_installations WHERE id = $1", h.installation)
	case "suspend":
		return app.Exec(h.ctx, h.state.DB, "UPDATE github_installations SET suspended_at = now(), updated_at = now() WHERE id = $1", h.installation)
	case "unsuspend":
		return app.Exec(h.ctx, h.state.DB, "UPDATE github_installations SET suspended_at = NULL, updated_at = now() WHERE id = $1", h.installation)
	}
	return nil
}

func (h *hook) repositoriesChanged(event *github.InstallationRepositoriesEvent) error {
	removed := make([]int64, len(event.RepositoriesRemoved))
	for i, repository := range event.RepositoriesRemoved {
		removed[i] = repository.ID
	}
	return app.Exec(h.ctx, h.state.DB, "DELETE FROM github_repo_syncs WHERE installation_id = $1 AND repo_id = ANY($2)", h.installation, removed)
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
				"author": pr.User.Login, "head_sha": pr.Head.SHA},
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
	return h.link(github.References(pr.Head.Ref, pr.Title, pr.Body), func(ref github.Reference) linkPlan {
		if state == github.StateMerged && ref.Closes {
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
	failures := []error{h.link(github.References(branch), func(github.Reference) linkPlan { return plan })}
	for _, commit := range event.Commits {
		failures = append(failures, h.linkCommit(event, branch, commit))
	}
	return errors.Join(failures...)
}

// linkCommit links a commit to the issues its message mentions. One that says it fixes an issue
// only closes it on the default branch, where the fix ships.
func (h *hook) linkCommit(event *github.PushEvent, branch string, commit github.PushCommit) error {
	refs := github.References("", commit.Message)
	if commit.ID == "" || len(refs) == 0 {
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
	closes := event.OnDefaultBranch()
	return h.link(refs, func(ref github.Reference) linkPlan {
		if !closes || !ref.Closes {
			return plan
		}
		closing := plan
		closing.move = moveMerged
		closing.movedLabel = "Commit " + sha + " on " + branch + " closed the issue"
		return closing
	})
}

// link applies a plan to each issue the references name, each in its own transaction.
func (h *hook) link(refs []github.Reference, planFor func(github.Reference) linkPlan) error {
	type target struct {
		issue uuid.UUID
		ref   github.Reference
	}
	var targets []target
	for _, ref := range refs {
		ids, err := app.All[uuid.UUID](h.ctx, h.state.DB,
			`SELECT i.id FROM issues i JOIN teams t ON t.id = i.team_id
			 WHERE i.workspace_id = $1 AND i.trashed_at IS NULL
			 AND ((upper(t.key) = $2 AND i.number = $3) OR $4 = ANY(i.previous_identifiers))`,
			h.workspace, ref.Key, ref.Number, ref.Identifier())
		if err != nil {
			return err
		}
		for _, id := range ids {
			if i := slices.IndexFunc(targets, func(t target) bool { return t.issue == id }); i >= 0 {
				targets[i].ref.Closes = targets[i].ref.Closes || ref.Closes
				continue
			}
			targets = append(targets, target{id, ref})
		}
	}
	var failures []error
	for _, t := range targets {
		if err := h.linkIssue(t.issue, planFor(t.ref)); err != nil {
			slog.Warn("applying a GitHub event to an issue failed", "issue", t.issue, "err", err)
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (h *hook) linkIssue(issue uuid.UUID, plan linkPlan) error {
	c := app.SystemCtx(h.ctx, h.state, h.workspace, uuid.Nil, app.Origin{Label: plan.label, URL: plan.attachment.URL})
	return inTx(c, func(tx pgx.Tx) error {
		row, err := loadIssueRow(c, tx, issue)
		if err != nil {
			return err
		}
		if plan.skipWithPullRequestFrom != "" {
			skip, err := app.One[bool](c, tx,
				`SELECT EXISTS (SELECT 1 FROM attachments WHERE issue_id = $1 AND source_type = $2
				 AND metadata->>'repo' = $3 AND metadata->>'branch' = $4)`,
				issue, sourcePullRequest, plan.attachment.Metadata["repo"], plan.skipWithPullRequestFrom)
			if err != nil || skip {
				return err
			}
		}
		linked, err := syncGitHubAttachment(c, tx, row, plan)
		if err != nil {
			return err
		}
		var target *uuid.UUID
		if plan.move != stay {
			current, err := stateType(c, tx, row.StateID)
			if err != nil {
				return err
			}
			if slices.Contains(moveFrom[plan.move], current) {
				if target, err = pullRequestState(c, tx, row.TeamID, plan.move); err != nil {
					return err
				}
			}
		}
		if target == nil || *target == row.StateID {
			if !linked {
				return nil
			}
			return history(c, tx, issue, "github", M{})
		}
		if plan.movedLabel != "" {
			c.Origin.Label = plan.movedLabel
		}
		before, err := issueValue(c, tx, issue)
		if err != nil {
			return err
		}
		if err := setState(c, tx, issue, *target, "github"); err != nil {
			return err
		}
		after, err := issueValue(c, tx, issue)
		if err != nil {
			return err
		}
		return emit(c, tx, "Issue", "update", after, changedFields(before, after), &row.TeamID)
	})
}

// pullRequestState is where the team's pull requests move issues: the status the team chose, or
// its first started (opened) or completed (merged) status.
func pullRequestState(c *app.Ctx, tx pgx.Tx, team uuid.UUID, kind move) (*uuid.UUID, error) {
	column, fallback := "github_pr_opened_state_id", "started"
	if kind == moveMerged {
		column, fallback = "github_pr_merged_state_id", "completed"
	}
	id, found, err := app.Maybe[uuid.UUID](c, tx,
		`SELECT s.id FROM workflow_states s JOIN teams t ON t.id = s.team_id
		 WHERE s.team_id = $1 AND s.archived_at IS NULL AND (s.id = t.`+column+` OR s.type = $2)
		 ORDER BY coalesce(s.id = t.`+column+`, false) DESC, s.position LIMIT 1`, team, fallback)
	if err != nil || !found {
		return nil, err
	}
	return &id, nil
}

// syncGitHubAttachment adds or updates the issue's attachment for a pull request, branch or
// commit, saying whether it changed in a way worth a history entry.
func syncGitHubAttachment(c *app.Ctx, tx pgx.Tx, row issueRow, plan linkPlan) (bool, error) {
	wanted := plan.attachment
	urls := []string{wanted.URL}
	if plan.replaceURL != "" && plan.replaceURL != wanted.URL {
		urls = append(urls, plan.replaceURL)
	}
	type attachment struct {
		ID         uuid.UUID
		URL        string
		Title      string
		SourceType *string
		Metadata   M
	}
	found, err := app.Rows[attachment](c, tx,
		`SELECT id, url, title, source_type, metadata FROM attachments WHERE issue_id = $1 AND url = ANY($2)
		 ORDER BY array_position($2, url)`, row.ID, urls)
	if err != nil {
		return false, err
	}
	if len(found) == 0 {
		id, err := app.One[uuid.UUID](c, tx,
			`INSERT INTO attachments (workspace_id, issue_id, url, title, metadata, source_type)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, c.Workspace, row.ID, wanted.URL, wanted.Title, wanted.Metadata, wanted.SourceType)
		if err != nil {
			return false, err
		}
		return true, emitAttachment(c, tx, id, row.TeamID, "create")
	}
	existing := found[0]
	visible := existing.Title != wanted.Title || existing.URL != wanted.URL || orEmpty(existing.SourceType) != wanted.SourceType ||
		existing.Metadata["state"] != wanted.Metadata["state"]
	if !visible && string(app.Marshal(existing.Metadata)) == string(app.Marshal(wanted.Metadata)) {
		return false, nil
	}
	err = app.Exec(c, tx,
		"UPDATE attachments SET url = $2, title = $3, source_type = $4, metadata = $5, updated_at = now() WHERE id = $1",
		existing.ID, wanted.URL, wanted.Title, wanted.SourceType, wanted.Metadata)
	if err != nil {
		return false, err
	}
	return visible, emitAttachment(c, tx, existing.ID, row.TeamID, "update")
}

func emitAttachment(c *app.Ctx, tx pgx.Tx, id, team uuid.UUID, action string) error {
	value, err := attachmentValue(c, tx, id)
	if err != nil {
		return err
	}
	return emit(c, tx, "Attachment", action, value, nil, &team)
}
