package github_service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/integrations/github"
	repository "ricrac/internal/repositories"
	github_repo "ricrac/internal/repositories/github"
	issue_service "ricrac/internal/services/issue"
)

// runBackfills imports one page of existing issues of each repository waiting for it, saying
// whether more pages are left.
func (self *GitHubService) runBackfills(ctx context.Context) (bool, error) {
	if self.app == nil {
		return false, nil
	}
	ids, err := self.github().DueBackfills(ctx)
	if err != nil {
		return false, err
	}
	more := false
	for _, id := range ids {
		running, err := self.backfillPage(ctx, id)
		if err != nil {
			return more, err
		}
		more = more || running
	}
	return more, nil
}

type backfillCounts struct {
	created, updated, comments, skipped int
	failed                              error
}

func (self *GitHubService) backfillPage(ctx context.Context, id uuid.UUID) (bool, error) {
	sync, err := self.github().SyncOf(ctx, id)
	if err != nil || sync == nil {
		return false, err
	}
	which := "open"
	if sync.BackfillClosed {
		which = "all"
	}
	var issues []github.Issue
	token, err := self.app.InstallationToken(ctx, sync.InstallationID)
	next := 0
	if err == nil {
		path := fmt.Sprintf("%s/issues?state=%s&per_page=100&page=%d&sort=created&direction=asc", github.RepoPath(sync.FullName), which, sync.BackfillPage)
		next, err = self.app.Do(ctx, token, "GET", path, nil, &issues)
	}
	if err != nil {
		failErr := self.github().FailBackfill(ctx, id)
		return false, errors.Join(self.recordSync(ctx, id, err), failErr)
	}
	var counts backfillCounts
	for i := range issues {
		if err := self.backfillIssue(ctx, sync, token, &issues[i], &counts); err != nil {
			counts.skipped++
			counts.failed = err
		}
	}
	status := "running"
	if next == 0 {
		status = "completed"
	}
	err = self.github().AdvanceBackfill(ctx, id, github_repo.BackfillProgress{
		Status: status, NextPage: max(next, 1), Created: counts.created, Updated: counts.updated, Comments: counts.comments,
		Skipped: counts.skipped,
	})
	if err != nil {
		return false, err
	}
	if err := self.recordSync(ctx, id, counts.failed); err != nil {
		slog.Warn("some GitHub issues weren't imported", "repoSync", id, "err", err)
	}
	return next > 0, nil
}

func (self *GitHubService) backfillIssue(ctx context.Context, sync *ent.GithubRepoSync, token string, issue *github.Issue, counts *backfillCounts) error {
	if issue.IsPullRequest() {
		counts.skipped++
		return nil
	}
	link, err := self.github().IssueLink(ctx, sync.ID, issue.ID)
	if err != nil {
		return err
	}
	if link == nil {
		created, err := self.importIssue(ctx, sync, issue, false)
		if err != nil {
			return err
		}
		if created {
			counts.created++
		}
		if link, err = self.github().IssueLink(ctx, sync.ID, issue.ID); link == nil || err != nil {
			return err
		}
	} else {
		changed, err := self.reconcileIssue(ctx, sync, link, issue)
		if err != nil {
			return err
		}
		if !changed {
			counts.skipped++
			return nil
		}
		counts.updated++
	}
	for page := 1; page > 0 && issue.Comments > 0; {
		var comments []github.Comment
		path := fmt.Sprintf("%s/comments?per_page=100&page=%d", github.IssuePath(sync.FullName, issue.Number), page)
		if page, err = self.app.Do(ctx, token, "GET", path, nil, &comments); err != nil {
			return err
		}
		for i := range comments {
			created, err := self.importComment(ctx, sync, link, &comments[i])
			if err != nil {
				return err
			}
			if created {
				counts.comments++
			}
		}
	}
	return nil
}

// reconcileIssue brings a linked issue up to date with GitHub when GitHub changed it since.
func (self *GitHubService) reconcileIssue(ctx context.Context, sync *ent.GithubRepoSync, link *ent.GithubIssueLink, issue *github.Issue) (bool, error) {
	if !issue.UpdatedAt.After(link.GithubUpdatedAt) {
		return false, nil
	}
	actor := self.access.System(sync.WorkspaceID, uuid.Nil,
		auth.Origin{Label: "Synced from GitHub #" + strconv.Itoa(issue.Number), URL: issue.HTMLURL, FromGitHubIssue: true})
	row, err := self.issueRow(ctx, link.ID)
	if err != nil {
		return false, err
	}
	input := issue_service.SaveInput{}
	changed := false
	if title := clampTitle(issue.Title); title != row.Title {
		input.Title, changed = &title, true
	}
	if issue.Body != row.Description {
		input.Description, changed = &issue.Body, true
	}
	current, err := self.issues.StateType(ctx, nil, row.StateID)
	if err != nil {
		return false, err
	}
	if !matchesClosure(current, issue.Closure()) {
		status, err := self.statusFor(ctx, row.TeamID, issue.Closure())
		if err != nil {
			return false, err
		}
		input.State, changed = new(status.String()), true
	}
	for _, label := range issue.Labels {
		id, err := self.labelFor(ctx, actor, row.TeamID, label)
		if err != nil {
			return false, err
		}
		has, err := self.github().HasLabel(ctx, row.ID, id)
		if err != nil {
			return false, err
		}
		if !has {
			input.AddLabels, changed = append(input.AddLabels, id.String()), true
		}
	}
	milestone, err := self.issueMilestone(ctx, sync, issue)
	if err != nil {
		return false, err
	}
	if !samePtr(milestone, row.MilestoneID) {
		input.Milestone, changed = milestoneInput(milestone), true
	}
	assignee := self.assignable(ctx, self.memberByLogin(ctx, sync.WorkspaceID, issue.Assignee), row.TeamID)
	if assignee != uuid.Nil && !samePtr(row.AssigneeID, &assignee) {
		input.Assignee, changed = some(assignee.String()), true
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if changed {
			if err := self.issues.Update(ctx, tx, actor, link.ID, input); err != nil {
				return err
			}
		}
		return self.github().TouchIssueLink(ctx, tx, link.ID, issue.UpdatedAt)
	})
	return changed, err
}
