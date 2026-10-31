package e2e

import (
	"errors"
	"net/http"
	"testing"
)

// Upvotes, closing, deleted messages and edit history in the forum.

func failsWith(t *testing.T, c *Actor, status int, name string, args M) string {
	t.Helper()
	_, err := tryCall(c, name, args)
	var failed *callError
	if !errors.As(err, &failed) {
		t.Fatalf("%s should have failed", name)
	}
	eq(t, failed.Status, status, name, failed.Message)
	return failed.Message
}

func TestDiscussionUpvotes(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	guest := w.member("gus@acme.test", "Gus Guest", "guest")
	lunch := call(t, w.Ann, "save_discussion", M{"title": "Lunch"})
	call(t, w.Ann, "save_discussion", M{"title": "Launch"})
	call(t, w.Ann, "save_discussion", M{"title": "Offsite"})
	eq(t, lunch.At("upvoteCount").Num(), 0)
	check(t, !lunch.At("upvoted").Bool(), lunch.JSON())

	voted := call(t, w.Ann, "vote_discussion", M{"id": "Lunch", "upvote": true})
	again := call(t, w.Ann, "vote_discussion", M{"id": "Lunch", "upvote": true})
	eq(t, voted.At("upvoteCount").Num(), 1)
	eq(t, again.At("upvoteCount").Num(), 1, "voting twice counts once")
	check(t, again.At("upvoted").Bool(), again.JSON())
	eq(t, again.At("lastActivityAt").Str(), lunch.At("lastActivityAt").Str(), "voting isn't activity")
	eq(t, call(t, bob, "vote_discussion", M{"id": "Lunch", "upvote": true}).At("upvoteCount").Num(), 2)
	call(t, bob, "vote_discussion", M{"id": "Launch", "upvote": true})
	launch := call(t, w.Ann, "get_discussion", M{"id": "Launch"})
	eq(t, launch.At("upvoteCount").Num(), 1)
	check(t, !launch.At("upvoted").Bool(), "upvoted is the caller's own", launch.JSON())
	eq(t, call(t, w.Ann, "get_notifications", M{}).At("nodes").Len(), 0, "voting notifies nobody")
	contains(t, failsWith(t, guest, http.StatusForbidden, "vote_discussion", M{"id": "Lunch", "upvote": true}), "Guests")

	listed := call(t, bob, "list_discussions", M{"sort": "top"}).At("nodes")
	eqList(t, listed.Strings("title"), []string{"Lunch", "Launch", "Offsite"})
	eq(t, listed.At(0, "upvoteCount").Num(), 2)
	check(t, listed.At(0, "upvoted").Bool() && !listed.At(2, "upvoted").Bool(), listed.JSON())

	call(t, w.Ann, "save_comment", M{"discussionId": "Launch", "body": "Soon"})
	eqList(t, call(t, bob, "list_discussions", M{}).At("nodes").Strings("title"), []string{"Launch", "Offsite", "Lunch"})
	eqList(t, call(t, bob, "list_discussions", M{"sort": "newest"}).At("nodes").Strings("title"), []string{"Offsite", "Launch", "Lunch"})
	eqList(t, call(t, bob, "list_discussions", M{"order": "asc"}).At("nodes").Strings("title"), []string{"Lunch", "Offsite", "Launch"})
	eqList(t, call(t, bob, "list_discussions", M{"sort": "newest", "order": "asc"}).At("nodes").Strings("title"), []string{"Lunch", "Launch", "Offsite"})
	eqList(t, call(t, bob, "list_discussions", M{"sort": "top", "order": "asc"}).At("nodes").Strings("title"), []string{"Offsite", "Launch", "Lunch"})

	takenBack := call(t, bob, "vote_discussion", M{"id": "Lunch", "upvote": false})
	eq(t, takenBack.At("upvoteCount").Num(), 1)
	check(t, !takenBack.At("upvoted").Bool(), takenBack.JSON())
	eq(t, call(t, bob, "vote_discussion", M{"id": "Lunch", "upvote": false}).At("upvoteCount").Num(), 1)
	eqList(t, call(t, bob, "list_discussions", M{"sort": "top"}).At("nodes").Strings("title"), []string{"Launch", "Lunch", "Offsite"}, "ties by recent activity")

	call(t, w.Ann, "save_discussion", M{"title": "Pinned", "pinned": true})
	eq(t, call(t, bob, "list_discussions", M{"sort": "top"}).At("nodes", 0, "title").Str(), "Pinned", "pinned first")
	eq(t, call(t, bob, "list_discussions", M{"sort": "top", "order": "asc"}).At("nodes", 0, "title").Str(), "Pinned", "pinned first either way")
	contains(t, fails(t, bob, "list_discussions", M{"sort": "best"}), "sort")
	contains(t, fails(t, bob, "list_discussions", M{"order": "up"}), "order")
}

func TestOnlyTheStarterClosesADiscussion(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, bob, "save_discussion", M{"title": "Idea"})
	refused := failsWith(t, w.Ann, http.StatusForbidden, "save_discussion", M{"id": "Idea", "closed": true})
	eq(t, refused, "Only the person who started a discussion can close or reopen it", "not even the owner")
	closed := call(t, bob, "save_discussion", M{"id": "Idea", "closed": true})
	check(t, closed.At("closedAt").IsString(), closed.JSON())
	failsWith(t, w.Ann, http.StatusForbidden, "save_discussion", M{"id": "Idea", "closed": false})
	check(t, call(t, bob, "save_discussion", M{"id": "Idea", "closed": false}).At("closedAt").IsNull())

	call(t, bob, "save_discussion", M{"title": "Orphan"})
	w.exec("UPDATE discussions SET creator_id = NULL WHERE title = 'Orphan'")
	failsWith(t, bob, http.StatusForbidden, "save_discussion", M{"id": "Orphan", "closed": true})
	check(t, call(t, w.Ann, "save_discussion", M{"id": "Orphan", "closed": true}).At("closedAt").IsString(), "admins once the starter is gone")
}

func TestDeletedForumMessagesStayAsTombstones(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_discussion", M{"title": "Plan"})
	root := call(t, bob, "save_comment", M{"discussionId": "Plan", "body": "First", "quotedText": "Plan"})
	reply := call(t, w.Ann, "save_comment", M{"parentId": root.At("id").V, "body": "Reply"})
	call(t, w.Ann, "add_reaction", M{"emoji": "+1", "commentId": root.At("id").V})
	call(t, bob, "save_comment", M{"id": root.At("id").V, "body": "First, edited"})

	deleted := call(t, bob, "delete_comment", M{"id": root.At("id").V})
	eq(t, deleted.At("deleted").Str(), root.At("id").Str())
	eq(t, deleted.At("deletedReplies").Num(), 0)
	eq(t, call(t, bob, "delete_comment", M{"id": root.At("id").V}).JSON(), deleted.JSON(), "deleting again answers the same")

	thread := call(t, bob, "list_comments", M{"discussionId": "Plan"}).At("nodes", 0)
	eq(t, thread.At("id").Str(), root.At("id").Str())
	eq(t, thread.At("body").Str(), "")
	check(t, thread.At("deletedAt").IsString() && thread.At("quotedText").IsNull() && thread.At("reactions").Len() == 0, thread.JSON())
	eq(t, thread.At("replies", 0, "id").Str(), reply.At("id").Str(), "replies stay")
	check(t, thread.At("replies", 0, "deletedAt").IsNull())
	eq(t, call(t, bob, "get_discussion", M{"id": "Plan"}).At("commentCount").Num(), 1)

	gone := "That message was deleted"
	eq(t, failsWith(t, bob, http.StatusBadRequest, "save_comment", M{"id": root.At("id").V, "body": "Back"}), gone)
	eq(t, failsWith(t, w.Ann, http.StatusBadRequest, "add_reaction", M{"emoji": "tada", "commentId": root.At("id").V}), gone)
	eq(t, failsWith(t, w.Ann, http.StatusBadRequest, "save_comment", M{"parentId": root.At("id").V, "body": "Hm"}), gone)
	eq(t, failsWith(t, bob, http.StatusBadRequest, "get_comment_history", M{"id": root.At("id").V}), gone)
	call(t, bob, "save_comment", M{"parentId": reply.At("id").V, "body": "The thread goes on"})

	started := call(t, w.Ann, "save_discussion_thread", M{"discussion": "Plan", "title": "Bug"})
	inThread := call(t, bob, "save_comment", M{"discussionThreadId": started.At("id").V, "body": "Repro"})
	call(t, bob, "delete_comment", M{"id": inThread.At("id").V})
	threadDetail := call(t, bob, "get_discussion_thread", M{"id": started.At("id").V})
	eq(t, threadDetail.At("commentCount").Num(), 0)
	check(t, threadDetail.At("comments", 0, "deletedAt").IsString(), threadDetail.JSON())
	eq(t, call(t, bob, "get_discussion", M{"id": "Plan"}).At("threads", 0, "commentCount").Num(), 0)

	call(t, w.Ann, "save_issue", M{"title": "Crash"})
	onIssue := call(t, bob, "save_comment", M{"issueId": "ENG-1", "body": "Seen it"})
	call(t, w.Ann, "save_comment", M{"parentId": onIssue.At("id").V, "body": "Me too"})
	eq(t, call(t, bob, "delete_comment", M{"id": onIssue.At("id").V}).At("deletedReplies").Num(), 1)
	eq(t, call(t, bob, "list_comments", M{"issueId": "ENG-1"}).At("nodes").Len(), 0, "comments on issues are deleted for good")
}

func TestForumMessagesKeepTheirEditHistory(t *testing.T) {
	w := world(t)
	started := call(t, w.Ann, "save_discussion", M{"title": "Plan", "body": "v1"})
	check(t, started.At("editedAt").IsNull(), started.JSON())

	comment := call(t, w.Ann, "save_comment", M{"discussionId": "Plan", "body": "one"})
	call(t, w.Ann, "save_comment", M{"id": comment.At("id").V, "body": "two"})
	edited := call(t, w.Ann, "save_comment", M{"id": comment.At("id").V, "body": "three"})
	versions := call(t, w.Ann, "get_comment_history", M{"id": comment.At("id").V}).At("versions")
	eqList(t, versions.Strings("body"), []string{"one", "two", "three"})
	eqList(t, []bool{versions.At(0, "current").Bool(), versions.At(1, "current").Bool(), versions.At(2, "current").Bool()}, []bool{false, false, true})
	eq(t, versions.At(0, "writtenAt").Str(), comment.At("createdAt").Str())
	eq(t, versions.At(2, "writtenAt").Str(), edited.At("editedAt").Str())

	call(t, w.Ann, "save_discussion", M{"id": "Plan", "body": "v2"})
	patched := call(t, w.Ann, "save_discussion", M{"id": "Plan", "patch": []any{M{"op": "append", "text": " more"}}})
	eq(t, patched.At("body").Str(), "v2 more")
	check(t, patched.At("editedAt").IsString(), patched.JSON())
	history := call(t, w.Ann, "get_discussion_history", M{"id": "Plan"}).At("versions")
	eqList(t, history.Strings("body"), []string{"v1", "v2 more"}, "quick edits are one version")
	eq(t, history.At(0, "writtenAt").Str(), started.At("createdAt").Str())
	eq(t, history.At(1, "writtenAt").Str(), patched.At("editedAt").Str())
	check(t, history.At(1, "current").Bool() && !history.At(0, "current").Bool(), history.JSON())

	retitled := call(t, w.Ann, "save_discussion", M{"id": "Plan", "title": "Plan B", "tags": []any{"q4"}})
	eq(t, retitled.At("editedAt").Str(), patched.At("editedAt").Str(), "only the body counts")
	eq(t, call(t, w.Ann, "get_discussion_history", M{"id": "Plan B"}).At("versions").Len(), 2)

	w.exec("UPDATE message_versions SET written_at = written_at - interval '1 hour'")
	w.exec("UPDATE discussions SET edited_at = edited_at - interval '11 minutes' WHERE title = 'Plan B'")
	session := call(t, w.Ann, "get_discussion", M{"id": "Plan B"}).At("editedAt").Str()
	call(t, w.Ann, "save_discussion", M{"id": "Plan B", "body": "v3"})
	later := call(t, w.Ann, "get_discussion_history", M{"id": "Plan B"}).At("versions")
	eqList(t, later.Strings("body"), []string{"v1", "v2 more", "v3"})
	eq(t, later.At(1, "writtenAt").Str(), session)
}
