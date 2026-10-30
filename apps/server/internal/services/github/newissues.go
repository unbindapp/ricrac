package github_service

import (
	"context"
	"encoding/json"
	"net/http"

	"ricrac/internal/integrations/github"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	github_repo "ricrac/internal/repositories/github"
)

// pushNewIssue opens an issue created in Ricrac on GitHub, as its author when they linked GitHub,
// else as the app. The repository's new issues stay locked until it is linked, so its webhook
// doesn't import it back. What only someone with push access may set (labels, assignee, milestone,
// status) follows as a change the app makes.
func (self *GitHubService) pushNewIssue(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.Issue == nil {
		return nil
	}
	sync, err := self.github().SyncOf(ctx, entry.RepoSync)
	if err != nil || sync == nil {
		return err
	}
	queued := false
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.github().LockNewIssues(ctx, tx, sync.ID); err != nil {
			return err
		}
		linked, err := self.github().IssueLinked(ctx, tx, *entry.Issue)
		if err != nil || linked {
			return err
		}
		row, err := self.github().Issue(ctx, *entry.Issue)
		if err != nil || row == nil || row.TrashedAt != nil {
			return err
		}
		value, err := self.issues.Value(ctx, tx, *entry.Issue)
		if err != nil {
			return err
		}
		created, err := self.openIssue(ctx, token, entry, row.Title, row.Description, value.Creator)
		if err != nil {
			return err
		}
		if err := self.github().AddIssueLink(ctx, tx, *entry.Issue, sync.ID, created.ID, created.Number, created.NodeID, created.HTMLURL, created.UpdatedAt); err != nil {
			return err
		}
		if changes := newIssueChanges(value); len(changes) > 0 {
			encoded, err := json.Marshal(changes)
			if err != nil {
				return err
			}
			if _, err := self.repo.Events().QueueIssueForGitHub(ctx, tx, *entry.Issue, encoded); err != nil {
				return err
			}
			queued = true
		}
		if !sync.LinkBack {
			return nil
		}
		queued = true
		return self.github().QueueLinkBack(ctx, tx, *entry.Issue)
	})
	if err == nil && queued {
		self.waker.WakeGitHub()
	}
	return err
}

func (self *GitHubService) openIssue(ctx context.Context, token string, entry github_repo.OutboxEntry, title, description string, creator *models.UserRef) (github.Issue, error) {
	path := github.RepoPath(entry.FullName) + "/issues"
	var created github.Issue
	openedAs, err := self.asAuthor(ctx, entry.ActAs, func(userToken string) error {
		_, err := self.app.Do(ctx, userToken, http.MethodPost, path, M{"title": title, "body": description}, &created)
		return err
	})
	if err != nil || openedAs != nil {
		return created, err
	}
	author := "Someone"
	if creator != nil {
		author = creator.Name
	}
	body := M{"title": title, "body": github.RicracCommentBody(author, description)}
	_, err = self.app.Do(ctx, token, http.MethodPost, path, body, &created)
	return created, err
}

// newIssueChanges is a new issue's fields GitHub can hold, as changes from a blank open issue.
func newIssueChanges(value models.Issue) M {
	changes := M{}
	if kind := value.State.Type; kind == "completed" || kind == "canceled" {
		changes["state"] = M{"from": M{"type": "unstarted"}, "to": M{"type": kind}}
	}
	if len(value.Labels) > 0 {
		labels := make([]string, len(value.Labels))
		for i, label := range value.Labels {
			labels[i] = label.Name
		}
		changes["labels"] = M{"from": []string{}, "to": labels}
	}
	if value.Assignee != nil {
		changes["assignee"] = M{"from": nil, "to": M{"id": value.Assignee.ID}}
	}
	if value.ProjectMilestone != nil {
		changes["projectMilestone"] = M{"from": nil, "to": M{"id": value.ProjectMilestone.ID}}
	}
	if value.Priority > 0 {
		changes["priority"] = M{"from": 0, "to": value.Priority}
	}
	if value.Estimate != nil {
		changes["estimate"] = M{"from": nil, "to": *value.Estimate}
	}
	if value.DueDate != nil {
		changes["dueDate"] = M{"from": nil, "to": *value.DueDate}
	}
	if value.Cycle != nil {
		changes["cycle"] = M{"from": nil, "to": value.Cycle}
	}
	return changes
}
