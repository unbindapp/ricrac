package github_service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/integrations/github"
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
	case "comment_created":
		return self.pushNewComment(ctx, token, entry)
	case "comment_edited":
		body, found, err := self.commentForGitHub(ctx, entry.Comment)
		if err != nil || !found || entry.GitHubComment == nil {
			return err
		}
		_, err = self.app.Do(ctx, token, http.MethodPatch, github.CommentPath(entry.FullName, *entry.GitHubComment), M{"body": body}, nil)
		return err
	case "comment_deleted":
		if entry.GitHubComment == nil {
			return nil
		}
		_, err := self.app.Do(ctx, token, http.MethodDelete, github.CommentPath(entry.FullName, *entry.GitHubComment), nil, nil)
		if github.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// commentForGitHub is a comment's body as the app posts it, with its author's name.
func (self *GitHubService) commentForGitHub(ctx context.Context, comment *uuid.UUID) (string, bool, error) {
	if comment == nil {
		return "", false, nil
	}
	found, err := self.github().CommentWithAuthor(ctx, *comment)
	if err != nil || found == nil {
		return "", false, err
	}
	author := "Someone"
	if found.Edges.User != nil {
		author = found.Edges.User.Name
	}
	return github.RicracCommentBody(author, found.Body), true, nil
}

func (self *GitHubService) pushNewComment(ctx context.Context, token string, entry github_repo.OutboxEntry) error {
	body, found, err := self.commentForGitHub(ctx, entry.Comment)
	if err != nil || !found {
		return err
	}
	posted, err := self.github().CommentPosted(ctx, *entry.Comment)
	if err != nil || posted {
		return err
	}
	var created github.Comment
	_, err = self.app.Do(ctx, token, http.MethodPost, github.IssuePath(entry.FullName, entry.Number)+"/comments", M{"body": body}, &created)
	if err != nil {
		return err
	}
	return self.github().LinkPostedComment(ctx, *entry.Comment, entry.Issue, created.ID)
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

// issuePatch is the PATCH of a GitHub issue that brings over the changes, empty when none of
// them shows there.
func issuePatch(changes M, title, description string) M {
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
	path := github.IssuePath(entry.FullName, entry.Number)
	if patch := issuePatch(changes, issue.Title, issue.Description); len(patch) > 0 {
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
