package github_service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/integrations/github"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	github_repo "ricrac/internal/repositories/github"
	issue_service "ricrac/internal/services/issue"
	milestone_service "ricrac/internal/services/milestone"
)

// milestoneSyncsPerRun is how many repositories' milestones a run of the jobs reads from GitHub.
const milestoneSyncsPerRun = 5

func (self *GitHubService) milestoneActor(sync *ent.GithubRepoSync, label, url string) *auth.Actor {
	return self.access.System(sync.WorkspaceID, uuid.Nil, auth.Origin{Label: label, URL: url, FromGitHubIssue: true})
}

// milestoneFor is the Ricrac milestone of a GitHub milestone of the sync's repository, with
// GitHub's changes since applied: the one linked to it, else the workspace's own of the same
// name, else a new one. It also says whether the link is new.
func (self *GitHubService) milestoneFor(ctx context.Context, sync *ent.GithubRepoSync, wanted *github.Milestone) (uuid.UUID, bool, error) {
	link, err := self.github().MilestoneLink(ctx, sync.ID, wanted.ID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if link != nil {
		return link.MilestoneID, false, self.applyMilestone(ctx, sync, link, wanted)
	}
	id, err := self.namedMilestone(ctx, sync, wanted)
	if err != nil {
		return uuid.Nil, false, err
	}
	if err := self.forgetReplacedMilestone(ctx, id, sync.ID); err != nil {
		return uuid.Nil, false, err
	}
	err = self.github().AddMilestoneLink(ctx, id, sync.ID, wanted.ID, wanted.Number, wanted.HTMLURL, wanted.UpdatedAt)
	if isUniqueViolation(err) {
		link, err := self.github().MilestoneLink(ctx, sync.ID, wanted.ID)
		if err != nil || link == nil {
			return uuid.Nil, false, err
		}
		return link.MilestoneID, false, nil
	}
	return id, true, err
}

// forgetReplacedMilestone drops the milestone's link to a GitHub milestone of the repository that
// another of the same name replaced.
func (self *GitHubService) forgetReplacedMilestone(ctx context.Context, milestone, repoSync uuid.UUID) error {
	replaced, err := self.github().MilestoneLinkIn(ctx, milestone, repoSync)
	if err != nil || replaced == nil {
		return err
	}
	return self.github().DeleteMilestoneLink(ctx, replaced.ID)
}

// namedMilestone is the workspace's own milestone named like the GitHub one, created from it when
// there's none.
func (self *GitHubService) namedMilestone(ctx context.Context, sync *ent.GithubRepoSync, wanted *github.Milestone) (uuid.UUID, error) {
	name := clampTitle(wanted.Title)
	found, err := self.repo.Milestone().WorkspaceNamed(ctx, nil, sync.WorkspaceID, name)
	if err != nil {
		return uuid.Nil, err
	}
	if found != nil {
		return found.ID, nil
	}
	input := &milestone_service.SaveInput{
		Name: &name, Description: optionalText(wanted.Description), TargetDate: optionalDate(wanted.DueDate()),
		Closed: new(wanted.Closed()),
	}
	created, err := self.milestones.Save(ctx, self.milestoneActor(sync, "Synced from GitHub", wanted.HTMLURL), input)
	if err != nil && errdefs.As(err).Status == http.StatusConflict {
		found, err := self.repo.Milestone().WorkspaceNamed(ctx, nil, sync.WorkspaceID, name)
		if err != nil || found == nil {
			return uuid.Nil, errors.Join(err, errdefs.Conflict("The milestone %q was taken and gone again", name))
		}
		return found.ID, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(created.ID)
}

func optionalText(text string) args.Nullable[string] {
	if strings.TrimSpace(text) == "" {
		return args.Nullable[string]{Set: true}
	}
	return some(text)
}

func optionalDate(date *string) args.Nullable[string] {
	if date == nil {
		return args.Nullable[string]{Set: true}
	}
	return some(*date)
}

// applyMilestone brings GitHub's changes to a linked milestone over to Ricrac, and from there to
// the other repositories it is linked in.
func (self *GitHubService) applyMilestone(ctx context.Context, sync *ent.GithubRepoSync, link *ent.GithubMilestoneLink, wanted *github.Milestone) error {
	if !wanted.UpdatedAt.After(link.GithubUpdatedAt) {
		return nil
	}
	row, err := self.repo.Milestone().Get(ctx, nil, link.MilestoneID)
	if err != nil {
		return err
	}
	input := milestoneChange(row, wanted)
	if input != nil {
		input.ID = new(row.ID.String())
		if _, err := self.milestones.Save(ctx, self.milestoneActor(sync, "Edited on GitHub", wanted.HTMLURL), input); err != nil {
			return err
		}
		queued, err := self.repo.Events().QueueMilestoneForGitHub(ctx, nil, row.ID, "milestone", &sync.ID)
		if err != nil {
			return err
		}
		if queued {
			self.waker.WakeGitHub()
		}
	}
	return self.github().TouchMilestoneLink(ctx, link.ID, wanted.UpdatedAt)
}

// milestoneChange is what differs between a GitHub milestone and its Ricrac one, nil when nothing.
func milestoneChange(row *ent.ProjectMilestone, wanted *github.Milestone) *milestone_service.SaveInput {
	input := &milestone_service.SaveInput{}
	changed := false
	if name := clampTitle(wanted.Title); name != row.Name {
		input.Name, changed = &name, true
	}
	if description := strings.TrimSpace(wanted.Description); description != strings.TrimSpace(derefText(row.Description)) {
		input.Description, changed = optionalText(wanted.Description), true
	}
	if due := wanted.DueDate(); !sameText(due, models.DatePtr(row.TargetDate)) {
		input.TargetDate, changed = optionalDate(due), true
	}
	if closed := wanted.Closed(); closed != (row.ClosedAt != nil) {
		input.Closed, changed = &closed, true
	}
	if !changed {
		return nil
	}
	return input
}

func derefText(text *string) string {
	if text == nil {
		return ""
	}
	return *text
}

func sameText(a, b *string) bool {
	return derefText(a) == derefText(b)
}

// unlinkMilestone forgets a GitHub milestone deleted there. The workspace's own milestone goes
// too when no other repository has it.
func (self *GitHubService) unlinkMilestone(ctx context.Context, sync *ent.GithubRepoSync, link *ent.GithubMilestoneLink) error {
	others, err := self.github().OtherMilestoneLinks(ctx, link.MilestoneID, link.ID)
	if err != nil {
		return err
	}
	if err := self.github().DeleteMilestoneLink(ctx, link.ID); err != nil {
		return err
	}
	row, err := self.repo.Milestone().Get(ctx, nil, link.MilestoneID)
	if ent.IsNotFound(err) || others || row != nil && row.ProjectID != nil {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = self.milestones.Delete(ctx, self.milestoneActor(sync, "Deleted on GitHub", link.GithubURL),
		&milestone_service.MilestoneInput{ID: link.MilestoneID.String()})
	return err
}

func (h *hook) milestoneEvent(event *github.MilestoneEvent) error {
	sync, err := h.syncedRepo(event.Repository)
	if sync == nil || err != nil {
		return err
	}
	if event.Action != "deleted" {
		_, _, err := h.milestoneFor(h.ctx, sync, &event.Milestone)
		return h.recordSync(h.ctx, sync.ID, err)
	}
	link, err := h.github().MilestoneLink(h.ctx, sync.ID, event.Milestone.ID)
	if link == nil || err != nil {
		return err
	}
	return h.recordSync(h.ctx, sync.ID, h.unlinkMilestone(h.ctx, sync, link))
}

// issueMilestone is the Ricrac milestone of a GitHub issue's milestone, nil when it has none.
func (self *GitHubService) issueMilestone(ctx context.Context, sync *ent.GithubRepoSync, issue *github.Issue) (*uuid.UUID, error) {
	if issue.Milestone == nil {
		return nil, nil
	}
	id, _, err := self.milestoneFor(ctx, sync, issue.Milestone)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func milestoneInput(id *uuid.UUID) args.Nullable[string] {
	if id == nil {
		return args.Nullable[string]{Set: true}
	}
	return some(id.String())
}

/* Reading milestones from GitHub, which sends no milestone events unless the app asks for them */

// syncMilestones reads the milestones of the repositories due for it, saying whether more are due.
func (self *GitHubService) syncMilestones(ctx context.Context) (bool, error) {
	if self.app == nil {
		return false, nil
	}
	syncs, err := self.github().MilestoneSyncsDue(ctx, milestoneSyncsPerRun+1)
	if err != nil {
		return false, err
	}
	for i, sync := range syncs {
		if i == milestoneSyncsPerRun {
			return true, nil
		}
		if err := self.syncRepoMilestones(ctx, sync); err != nil && !github.IsUnavailable(err) {
			_ = self.recordSync(ctx, sync.ID, err)
		}
		if err := self.github().MarkMilestonesSynced(ctx, sync.ID); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (self *GitHubService) syncRepoMilestones(ctx context.Context, sync *ent.GithubRepoSync) error {
	token, err := self.app.InstallationToken(ctx, sync.InstallationID)
	if err != nil {
		return err
	}
	milestones, err := self.listMilestones(ctx, token, sync.FullName)
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	var failures []error
	for i := range milestones {
		seen[milestones[i].ID] = true
		id, linked, err := self.milestoneFor(ctx, sync, &milestones[i])
		if err == nil && (linked || sync.MilestonesSyncedAt == nil) {
			err = self.milestoneIssues(ctx, sync, token, &milestones[i], id)
		}
		failures = append(failures, err)
	}
	links, err := self.github().MilestoneLinks(ctx, sync.ID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if !seen[link.GithubID] {
			failures = append(failures, self.unlinkMilestone(ctx, sync, link))
		}
	}
	return errors.Join(failures...)
}

// listMilestones is the repository's milestones, open and closed, at most 1000.
func (self *GitHubService) listMilestones(ctx context.Context, token, fullName string) ([]github.Milestone, error) {
	var all []github.Milestone
	for page := 1; page > 0 && page <= 10; {
		var milestones []github.Milestone
		next, err := self.app.Do(ctx, token, http.MethodGet, fmt.Sprintf("%s?state=all&per_page=100&page=%d", github.MilestonesPath(fullName), page), nil, &milestones)
		if err != nil {
			return nil, err
		}
		all = append(all, milestones...)
		page = next
	}
	return all, nil
}

// milestoneIssues puts the linked issues GitHub has in a milestone in its Ricrac milestone.
func (self *GitHubService) milestoneIssues(ctx context.Context, sync *ent.GithubRepoSync, token string, milestone *github.Milestone, id uuid.UUID) error {
	actor := self.milestoneActor(sync, "Milestone set on GitHub", milestone.HTMLURL)
	for page := 1; page > 0 && page <= 10; {
		var issues []github.Issue
		path := fmt.Sprintf("%s/issues?milestone=%d&state=all&per_page=100&page=%d", github.RepoPath(sync.FullName), milestone.Number, page)
		next, err := self.app.Do(ctx, token, http.MethodGet, path, nil, &issues)
		if err != nil {
			return err
		}
		for _, issue := range issues {
			if err := self.setIssueMilestone(ctx, actor, sync, issue.ID, id); err != nil {
				return err
			}
		}
		page = next
	}
	return nil
}

func (self *GitHubService) setIssueMilestone(ctx context.Context, actor *auth.Actor, sync *ent.GithubRepoSync, githubID int64, milestone uuid.UUID) error {
	link, err := self.github().IssueLink(ctx, sync.ID, githubID)
	if link == nil || err != nil {
		return err
	}
	row, err := self.issueRow(ctx, link.ID)
	if err != nil || samePtr(row.MilestoneID, &milestone) {
		return err
	}
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		return self.issues.Update(ctx, tx, actor, link.ID, issue_service.SaveInput{Milestone: some(milestone.String())})
	})
}

/* Sending milestones to GitHub */

// gitHubMilestone is the number of the Ricrac milestone's GitHub milestone in the entry's
// repository: the one linked, else the one of the same name, else a new one.
func (self *GitHubService) gitHubMilestone(ctx context.Context, token string, entry github_repo.OutboxEntry, milestone uuid.UUID) (*int, error) {
	link, err := self.github().MilestoneLinkIn(ctx, milestone, entry.RepoSync)
	if err != nil {
		return nil, err
	}
	if link != nil {
		return &link.GithubNumber, nil
	}
	row, err := self.repo.Milestone().Get(ctx, nil, milestone)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	existing, err := self.listMilestones(ctx, token, entry.FullName)
	if err != nil {
		return nil, err
	}
	var target *github.Milestone
	for i := range existing {
		if strings.EqualFold(strings.TrimSpace(existing[i].Title), row.Name) {
			target = &existing[i]
		}
	}
	if target != nil {
		taken, err := self.github().MilestoneLink(ctx, entry.RepoSync, target.ID)
		if err != nil {
			return nil, err
		}
		if taken != nil {
			return nil, errdefs.Conflict("%s has a milestone %q for another Ricrac milestone", entry.FullName, row.Name)
		}
	} else {
		target = &github.Milestone{}
		if _, err := self.app.Do(ctx, token, http.MethodPost, github.MilestonesPath(entry.FullName), milestoneBody(row), target); err != nil {
			return nil, err
		}
	}
	err = self.github().AddMilestoneLink(ctx, milestone, entry.RepoSync, target.ID, target.Number, target.HTMLURL, target.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &target.Number, nil
}

func milestoneBody(row *ent.ProjectMilestone) M {
	state := "open"
	if row.ClosedAt != nil {
		state = "closed"
	}
	return M{"title": row.Name, "description": derefText(row.Description), "due_on": github.DueOn(models.DatePtr(row.TargetDate)), "state": state}
}

func (self *GitHubService) pushMilestone(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.Milestone == nil {
		return nil
	}
	link, err := self.github().MilestoneLinkIn(ctx, *entry.Milestone, entry.RepoSync)
	if err != nil || link == nil {
		return err
	}
	row, err := self.repo.Milestone().Get(ctx, nil, *entry.Milestone)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var updated github.Milestone
	_, err = self.app.Do(ctx, token, http.MethodPatch, github.MilestonePath(entry.FullName, link.GithubNumber), milestoneBody(row), &updated)
	if github.IsNotFound(err) {
		return self.github().DeleteMilestoneLink(ctx, link.ID)
	}
	if err != nil {
		return err
	}
	return self.github().TouchMilestoneLink(ctx, link.ID, updated.UpdatedAt)
}

func (self *GitHubService) pushMilestoneDeletion(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	_, err := self.app.Do(ctx, token, http.MethodDelete, github.MilestonePath(entry.FullName, entry.Number), nil, nil)
	if github.IsNotFound(err) {
		return nil
	}
	return err
}
