package github_service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"time"

	"ricrac/ent"
	"ricrac/internal/integrations/github"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	github_repo "ricrac/internal/repositories/github"
)

// pushBackoff is the wait before the 2nd, 3rd, ... attempt; after the last the change is dropped.
var pushBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// GitHubJobs sends the changes queued for GitHub and imports a page of each repository's existing
// issues, saying whether there's more to do right away.
func (self *GitHubService) GitHubJobs(ctx context.Context) (bool, error) {
	pushed, err := self.pushChanges(ctx)
	if err != nil {
		return false, err
	}
	more, err := self.runBackfills(ctx)
	return more || pushed > 0, err
}

// pushChanges sends the queued changes that are due, answering how many it tried.
func (self *GitHubService) pushChanges(ctx context.Context) (int, error) {
	if self.app == nil {
		return 0, nil
	}
	entries, err := self.github().DueOutbox(ctx)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		pushErr := self.pushEntry(ctx, entry)
		var err error
		if pushErr == nil || entry.Attempts >= len(pushBackoff) {
			err = self.github().DeleteOutbox(ctx, entry.ID)
		} else {
			err = self.github().RetryOutbox(ctx, entry.ID, pushBackoff[entry.Attempts].Seconds())
		}
		if err != nil {
			return len(entries), err
		}
		_ = self.recordSync(ctx, entry.RepoSync, pushErr)
	}
	return len(entries), nil
}

func (self *GitHubService) pushEntry(ctx context.Context, entry github_repo.OutboxEntry) error {
	token, err := self.app.InstallationToken(ctx, entry.Installation)
	if err != nil {
		return err
	}
	switch entry.Kind {
	case "issue":
		return self.pushIssue(ctx, token, entry)
	case "link_back":
		return self.pushLinkBack(ctx, token, entry)
	case "comment_created":
		return self.pushNewComment(ctx, token, entry)
	case "comment_edited":
		return self.pushCommentEdit(ctx, token, entry)
	case "comment_deleted":
		return self.pushCommentDeletion(ctx, token, entry)
	}
	return nil
}

func (self *GitHubService) pushLinkBack(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.Issue == nil {
		return nil
	}
	identifier, workspaceKey, err := self.github().IssueAddress(ctx, *entry.Issue)
	if err != nil || identifier == "" {
		return err
	}
	link := models.IssueURL(self.cfg.PublicURL, workspaceKey, identifier)
	filled, err := self.fillRicracField(ctx, token, entry, link)
	if err != nil || filled {
		return err
	}
	_, err = self.app.Do(ctx, token, http.MethodPost, github.IssuePath(entry.FullName, entry.Number)+"/comments",
		M{"body": github.LinkBackBody(identifier, link)}, nil)
	return err
}

// fillRicracField puts the link in the organization's Ricrac issue field, false when it has none
// or the app can't use it.
func (self *GitHubService) fillRicracField(ctx context.Context, token string, entry github_repo.OutboxEntry, link string) (bool, error) {
	var fields []github.IssueField
	_, err := self.app.Do(ctx, token, http.MethodGet, github.IssueFieldsPath(entry.FullName), nil, &fields)
	if github.IsUnavailable(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	field := github.RicracField(fields)
	if field == nil {
		return false, nil
	}
	values := M{"issue_field_values": []M{{"field_id": field.ID, "value": link}}}
	_, err = self.app.Do(ctx, token, http.MethodPost, github.IssuePath(entry.FullName, entry.Number)+"/issue-field-values", values, nil)
	if github.IsUnavailable(err) {
		return false, nil
	}
	return err == nil, err
}

// bodyForApp is a comment's body as the app posts it as itself, with its author's name.
func bodyForApp(comment *ent.Comment) string {
	author := "Someone"
	if comment.Edges.User != nil {
		author = comment.Edges.User.Name
	}
	return github.RicracCommentBody(author, comment.Body)
}

// pushNewComment posts a comment as its author when they linked GitHub, else as the app. The
// issue's comments stay locked until it is linked, so its webhook doesn't import it back.
func (self *GitHubService) pushNewComment(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.Comment == nil || entry.Issue == nil {
		return nil
	}
	found, err := self.github().CommentWithAuthor(ctx, *entry.Comment)
	if err != nil || found == nil {
		return err
	}
	path := github.IssuePath(entry.FullName, entry.Number) + "/comments"
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.github().LockIssueComments(ctx, tx, *entry.Issue); err != nil {
			return err
		}
		posted, err := self.github().CommentPosted(ctx, tx, found.ID)
		if err != nil || posted {
			return err
		}
		var created github.Comment
		postedAs, err := self.asAuthor(ctx, found.UserID, func(userToken string) error {
			_, err := self.app.Do(ctx, userToken, http.MethodPost, path, M{"body": found.Body}, &created)
			return err
		})
		if err != nil {
			return err
		}
		if postedAs == nil {
			if _, err := self.app.Do(ctx, token, http.MethodPost, path, M{"body": bodyForApp(found)}, &created); err != nil {
				return err
			}
		}
		return self.github().LinkPostedComment(ctx, tx, found.ID, entry.Issue, created.ID, postedAs)
	})
}

func (self *GitHubService) pushCommentEdit(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.Comment == nil || entry.GitHubComment == nil {
		return nil
	}
	found, err := self.github().CommentWithAuthor(ctx, *entry.Comment)
	if err != nil || found == nil {
		return err
	}
	body := found.Body
	if entry.ActAs == nil {
		body = bodyForApp(found)
	}
	patch := func(token string) error {
		_, err := self.app.Do(ctx, token, http.MethodPatch, github.CommentPath(entry.FullName, *entry.GitHubComment), M{"body": body}, nil)
		return err
	}
	editedAs, err := self.asAuthor(ctx, entry.ActAs, patch)
	if err != nil || editedAs != nil {
		return err
	}
	return patch(token)
}

func (self *GitHubService) pushCommentDeletion(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.GitHubComment == nil {
		return nil
	}
	remove := func(token string) error {
		_, err := self.app.Do(ctx, token, http.MethodDelete, github.CommentPath(entry.FullName, *entry.GitHubComment), nil, nil)
		return err
	}
	deletedAs, err := self.asAuthor(ctx, entry.ActAs, remove)
	if err == nil && deletedAs == nil {
		err = remove(token)
	}
	if github.IsNotFound(err) {
		return nil
	}
	return err
}

// gitHubState is how a status type reads on GitHub: open, or closed as completed or not planned.
func gitHubState(kind string) (string, string) {
	switch kind {
	case "completed":
		return "closed", "completed"
	case "canceled":
		return "closed", "not_planned"
	}
	return "open", ""
}

// change reads a history change's from and to.
func change(changes M, field string) (any, any, bool) {
	value, ok := changes[field].(M)
	if !ok {
		return nil, nil, false
	}
	return value["from"], value["to"], true
}

func refField(ref any, key string) string {
	object, _ := ref.(M)
	text, _ := object[key].(string)
	return text
}

func names(list any) []string {
	items, _ := list.([]any)
	var all []string
	for _, item := range items {
		if text, ok := item.(string); ok {
			all = append(all, text)
		}
	}
	return all
}

// closesAsCanceled says whether the changes close the issue as canceled on GitHub.
func closesAsCanceled(changes M) bool {
	_, to, ok := change(changes, "state")
	return ok && refField(to, "type") == "canceled"
}

// issuePatch is the PATCH of a GitHub issue that brings over the changes, empty when none of
// them shows there. A canceled issue that duplicates one GitHub has (duplicateOf, its GitHub id)
// closes as a duplicate of it instead of as not planned.
func issuePatch(changes M, title, description string, duplicateOf *int64) M {
	patch := M{}
	if _, ok := changes["title"]; ok {
		patch["title"] = title
	}
	if _, ok := changes["description"]; ok {
		patch["body"] = description
	}
	if from, to, ok := change(changes, "state"); ok {
		before, beforeReason := gitHubState(refField(from, "type"))
		after, reason := gitHubState(refField(to, "type"))
		if before != after || beforeReason != reason {
			patch["state"] = after
			if reason != "" {
				patch["state_reason"] = reason
			}
			if reason == "not_planned" && duplicateOf != nil {
				patch["state_reason"] = "duplicate"
				patch["duplicate_issue_id"] = *duplicateOf
			}
		}
	}
	return patch
}

// labelChanges is the label names added and removed.
func labelChanges(from, to any) ([]string, []string) {
	before, after := names(from), names(to)
	added := slices.DeleteFunc(slices.Clone(after), func(name string) bool { return slices.Contains(before, name) })
	removed := slices.DeleteFunc(slices.Clone(before), func(name string) bool { return slices.Contains(after, name) })
	return added, removed
}

func (self *GitHubService) pushIssue(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	if entry.Issue == nil {
		return nil
	}
	issue, err := self.github().Issue(ctx, *entry.Issue)
	if err != nil || issue == nil {
		return err
	}
	var changes M
	_ = json.Unmarshal(entry.Changes, &changes)
	var duplicateOf *int64
	if closesAsCanceled(changes) {
		target, err := self.github().DuplicateTarget(ctx, *entry.Issue, entry.RepoSync)
		if err != nil {
			return err
		}
		if target != nil {
			duplicateOf = &target.GithubID
		}
	}
	path := github.IssuePath(entry.FullName, entry.Number)
	if patch := issuePatch(changes, issue.Title, issue.Description, duplicateOf); len(patch) > 0 {
		var updated github.Issue
		if _, err := self.app.Do(ctx, token, http.MethodPatch, path, patch, &updated); err != nil {
			return err
		}
		if err := self.github().TouchIssueLink(ctx, nil, *entry.Issue, updated.UpdatedAt); err != nil {
			return err
		}
	}
	if from, to, ok := change(changes, "labels"); ok {
		added, removed := labelChanges(from, to)
		if len(added) > 0 {
			if _, err := self.app.Do(ctx, token, http.MethodPost, path+"/labels", M{"labels": added}, nil); err != nil {
				return err
			}
		}
		for _, name := range removed {
			_, err := self.app.Do(ctx, token, http.MethodDelete, path+"/labels/"+url.PathEscape(name), nil, nil)
			if err != nil && !github.IsNotFound(err) {
				return err
			}
		}
	}
	if from, to, ok := change(changes, "assignee"); ok {
		for _, step := range []struct {
			method string
			user   any
		}{{http.MethodDelete, from}, {http.MethodPost, to}} {
			login, found, err := self.github().LoginOf(ctx, refField(step.user, "id"))
			if err != nil {
				return err
			}
			if !found || login == nil {
				continue
			}
			if _, err := self.app.Do(ctx, token, step.method, path+"/assignees", M{"assignees": []string{*login}}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
