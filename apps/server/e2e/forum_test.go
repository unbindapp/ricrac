package e2e

import (
	"strings"
	"testing"
)

// The forum: discussions, their threads and comments; and suggested edits on documents.

func TestDiscussionsThreadsAndTheirNotifications(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	carl := w.member("carl@acme.test", "Carl Clark", "member")
	started := call(t, w.Ann, "save_discussion", M{"title": "Q4 launch", "body": "Plan it with @bob"})
	eq(t, started.At("body").Str(), "Plan it with @bob")
	contains(t, started.At("url").Str(), "http://localhost:3000/acme/discussion/q4-launch-")
	eqList(t, started.At("subscribers").Strings("name"), []string{"Ann Smith", "Bob Jones"}, "starting and being mentioned subscribe")
	eqList(t, kinds(call(t, bob, "get_notifications", M{})), []string{"discussionMention"})

	thread := call(t, carl, "save_discussion_thread", M{"discussion": "Q4 launch", "kind": "bug", "title": "Button misaligned"})
	eq(t, thread.At("kind").Str(), "bug")
	eq(t, thread.At("discussion", "title").Str(), "Q4 launch")
	contains(t, thread.At("url").Str(), "?thread="+thread.At("id").Str())
	inbox := call(t, w.Ann, "get_notifications", M{})
	eqList(t, summaries(inbox), []string{"Carl Clark started a thread in Q4 launch: Button misaligned"})
	eq(t, inbox.At("nodes", 0, "discussion", "title").Str(), "Q4 launch")
	eq(t, inbox.At("nodes", 0, "discussionThread", "kind").Str(), "bug")
	contains(t, fails(t, carl, "save_discussion_thread", M{"discussion": "Q4 launch", "kind": "rant", "title": "x"}), "kind")
	patched := call(t, carl, "save_discussion_thread", M{"id": thread.At("id").V, "patch": []any{M{"op": "append", "text": "Off by 2px"}}})
	eq(t, patched.At("body").Str(), "Off by 2px")

	root := call(t, bob, "save_comment", M{"discussionThreadId": thread.At("id").V, "body": "Fixed in main"})
	eq(t, root.At("discussionThread", "title").Str(), "Button misaligned")
	contains(t, root.At("url").Str(), "?thread="+thread.At("id").Str()+"#comment-")
	call(t, carl, "save_comment", M{"parentId": root.At("id").V, "body": "Thanks"})
	eqList(t, kinds(call(t, bob, "get_notifications", M{})), []string{"discussionCommentReply", "discussionNewThread", "discussionMention"})
	eqList(t, kinds(call(t, w.Ann, "get_notifications", M{})), []string{"discussionNewComment", "discussionNewComment", "discussionNewThread"})
	call(t, w.Ann, "save_comment", M{"discussionId": "Q4 launch", "body": "Looking good"})

	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Align the button"})
	linked := call(t, w.Ann, "save_discussion_thread", M{"id": thread.At("id").V, "issue": "ENG-1", "resolved": true})
	eq(t, linked.At("issue", "identifier").Str(), "ENG-1")
	eq(t, linked.At("resolvingUser", "name").Str(), "Ann Smith")
	eq(t, linked.At("comments", 0, "replies", 0, "body").Str(), "Thanks")
	eq(t, linked.At("commentCount").Num(), 2)
	eqList(t, summaries(call(t, carl, "get_notifications", M{"type": "discussionThreadResolved"})), []string{"Ann Smith resolved Button misaligned in Q4 launch"})
	contains(t, fails(t, bob, "save_discussion_thread", M{"id": thread.At("id").V, "title": "Mine now"}), "author")
	unlinked := call(t, bob, "save_discussion_thread", M{"id": thread.At("id").V, "issue": nil, "resolved": false})
	check(t, unlinked.At("issue").IsNull() && unlinked.At("resolvedAt").IsNull(), unlinked.JSON())

	detail := call(t, bob, "get_discussion", M{"id": started.At("url").V})
	eq(t, detail.At("threadCount").Num(), 1)
	eq(t, detail.At("openThreadCount").Num(), 1)
	eq(t, detail.At("commentCount").Num(), 1)
	eq(t, detail.At("threads", 0, "commentCount").Num(), 2)
	eqList(t, detail.At("subscribers").Strings("name"), []string{"Ann Smith", "Bob Jones", "Carl Clark"})
	eq(t, call(t, bob, "list_comments", M{"discussionId": started.At("id").V}).At("nodes", 0, "body").Str(), "Looking good")
	contains(t, fails(t, bob, "save_discussion", M{"id": "Q4 launch", "title": "Bob's launch"}), "author")
	heard := call(t, bob, "get_notifications", M{"type": "discussionNewComment"}).At("nodes").Len()
	call(t, bob, "save_discussion", M{"id": "Q4 launch", "removeSubscribers": []any{"me"}})
	call(t, carl, "save_comment", M{"discussionId": "Q4 launch", "body": "One more"})
	eq(t, call(t, bob, "get_notifications", M{"type": "discussionNewComment"}).At("nodes").Len(), heard, "unsubscribed")

	call(t, w.Ann, "save_discussion", M{"title": "Offsite", "pinned": true})
	call(t, w.Ann, "save_discussion", M{"title": "Old idea", "closed": true})
	eqList(t, call(t, bob, "list_discussions", M{}).At("nodes").Strings("title"), []string{"Offsite", "Q4 launch"}, "pinned first, closed left out")
	eqList(t, call(t, bob, "list_discussions", M{"status": "closed"}).At("nodes").Strings("title"), []string{"Old idea"})
	eqList(t, call(t, bob, "search", M{"query": "offsite"}).At("nodes").Strings("type"), []string{"discussion"})

	call(t, w.Ann, "delete_discussion", M{"id": "Offsite"})
	fails(t, bob, "get_discussion", M{"id": "Offsite"})
	eqList(t, call(t, bob, "list_discussions", M{"status": "all", "onlyTrashed": true}).At("nodes").Strings("title"), []string{"Offsite"})
	contains(t, fails(t, bob, "restore_discussion", M{"id": "Offsite"}), "author")
	call(t, w.Ann, "restore_discussion", M{"id": "Offsite"})
	call(t, carl, "delete_discussion_thread", M{"id": thread.At("id").V})
	eq(t, call(t, bob, "get_discussion", M{"id": "Q4 launch"}).At("threadCount").Num(), 0)
}

func TestTeamDiscussionsStayWithTheTeam(t *testing.T) {
	w := world(t)
	guest := w.member("gus@acme.test", "Gus Guest", "guest")
	call(t, w.Ann, "save_discussion", M{"title": "Engineering only", "team": "ENG"})
	call(t, w.Ann, "save_discussion", M{"title": "Everyone"})
	eqList(t, call(t, guest, "list_discussions", M{}).At("nodes").Strings("title"), []string{"Everyone"})
	contains(t, fails(t, guest, "get_discussion", M{"id": "Engineering only"}), "No discussion")
	contains(t, fails(t, guest, "save_comment", M{"discussionId": "Engineering only", "body": "Hi"}), "No discussion")
	moved := call(t, w.Ann, "save_discussion", M{"id": "Engineering only", "team": nil})
	check(t, moved.At("team").IsNull())
	call(t, guest, "get_discussion", M{"id": "Engineering only"})
}

func TestDocumentSuggestionsAreAcceptedOrRejected(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_document", M{"title": "Plan", "team": "ENG", "content": "The quick brown fox.\n\nJumps over the lazy dog."})

	note := call(t, bob, "save_comment", M{"documentId": "Plan", "quotedText": "quick brown fox", "body": "Which fox?"})
	contains(t, note.At("url").Str(), "/acme/document/plan-")
	check(t, note.At("suggestedText").IsNull())
	suggested := call(t, bob, "save_comment", M{"documentId": "Plan", "quotedText": "lazy dog", "suggestedText": "sleepy cat", "body": "Cats"})
	eq(t, suggested.At("suggestionStatus").Str(), "pending")
	eqList(t, kinds(call(t, w.Ann, "get_notifications", M{})), []string{"documentSuggestion", "documentNewComment"})
	eqList(t, summaries(call(t, w.Ann, "get_notifications", M{"type": "documentSuggestion"})), []string{"Bob Jones suggested an edit to Plan"})

	contains(t, fails(t, bob, "save_comment", M{"documentId": "Plan", "quotedText": "he", "suggestedText": "she", "body": "x"}), "times")
	contains(t, fails(t, bob, "save_comment", M{"documentId": "Plan", "quotedText": "wolf", "suggestedText": "dog", "body": "x"}), "no passage")
	contains(t, fails(t, bob, "save_comment", M{"documentId": "Plan", "suggestedText": "dog", "body": "x"}), "quotedText")
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Crash"})
	contains(t, fails(t, bob, "save_comment", M{"issueId": "ENG-1", "quotedText": "Crash", "suggestedText": "Bug", "body": "x"}), "document")

	accepted := call(t, w.Ann, "accept_suggestion", M{"id": suggested.At("id").V})
	eq(t, accepted.At("suggestionStatus").Str(), "accepted")
	eq(t, accepted.At("resolvingUser", "name").Str(), "Ann Smith")
	document := call(t, w.Ann, "get_document", M{"id": "Plan"})
	eq(t, document.At("content").Str(), "The quick brown fox.\n\nJumps over the sleepy cat.")
	check(t, document.At("versionCount").Num() >= 1, document.JSON())
	eqList(t, summaries(call(t, bob, "get_notifications", M{})), []string{"Ann Smith accepted your suggestion on Plan"})
	contains(t, fails(t, w.Ann, "accept_suggestion", M{"id": suggested.At("id").V}), "already accepted")
	contains(t, fails(t, w.Ann, "accept_suggestion", M{"id": note.At("id").V}), "No suggestion")

	rejected := call(t, bob, "save_comment", M{"documentId": "Plan", "quotedText": "quick", "suggestedText": "slow", "body": "Calmer"})
	eq(t, call(t, w.Ann, "reject_suggestion", M{"id": rejected.At("id").V}).At("suggestionStatus").Str(), "rejected")
	stale := call(t, bob, "save_comment", M{"documentId": "Plan", "quotedText": "brown", "suggestedText": "red", "body": "Redder"})
	call(t, w.Ann, "save_document", M{"id": "Plan", "patch": []any{M{"op": "replace", "old_string": "brown", "new_string": "grey"}}})
	contains(t, fails(t, w.Ann, "accept_suggestion", M{"id": stale.At("id").V}), "no longer")
	eq(t, call(t, w.Ann, "get_document", M{"id": "Plan"}).At("content").Str(), "The quick grey fox.\n\nJumps over the sleepy cat.")

	threads := call(t, w.Ann, "list_comments", M{"documentId": "Plan"}).At("nodes")
	statuses := []string{}
	for _, thread := range threads.List() {
		statuses = append(statuses, strings.TrimSpace(thread.At("suggestionStatus").Str()))
	}
	eqList(t, statuses, []string{"", "accepted", "rejected", "pending"})
}
