package e2e

import (
	"testing"
)

// Comments, reactions, notifications, attachments, uploads and search.

func summaries(inbox J) []string {
	return inbox.At("nodes").Strings("summary")
}

func kinds(inbox J) []string {
	return inbox.At("nodes").Strings("type")
}

func TestCommentThreadsWithRepliesAndResolving(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Crash"})
	root := call(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "First"})
	eq(t, root.At("issue", "identifier").Str(), "ENG-1")
	eq(t, root.At("url").Str(), "http://localhost:3000/acme/issue/ENG-1#comment-"+root.At("id").Str())
	reply := call(t, bob, "save_comment", M{"parentId": root.At("id").V, "body": "A reply"})
	eq(t, reply.At("parentId").Str(), root.At("id").Str())
	nested := call(t, w.Ann, "save_comment", M{"parentId": reply.At("id").V, "body": "Reply to the reply"})
	eq(t, nested.At("parentId").Str(), root.At("id").Str(), "replies to replies join the thread")
	eq(t, nested.At("issue", "identifier").Str(), "ENG-1")
	call(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "Second", "quotedText": "Crash"})

	threads := call(t, w.Ann, "list_comments", M{"issueId": "ENG-1"})
	nodes := threads.At("nodes")
	eq(t, nodes.Len(), 2)
	eq(t, nodes.At(0, "body").Str(), "First")
	eqList(t, nodes.At(0, "replies").Strings("body"), []string{"A reply", "Reply to the reply"})
	eq(t, nodes.At(0, "replies", 0, "user", "name").Str(), "Bob Jones")
	eq(t, nodes.At(1, "quotedText").Str(), "Crash")
	firstPage := call(t, w.Ann, "list_comments", M{"issueId": "ENG-1", "limit": 1})
	eq(t, firstPage.At("pageInfo", "hasNextPage").Bool(), true)
	cursor := firstPage.At("pageInfo", "endCursor").V
	secondPage := call(t, w.Ann, "list_comments", M{"issueId": "ENG-1", "limit": 1, "cursor": cursor})
	eq(t, secondPage.At("nodes", 0, "body").Str(), "Second")

	resolved := call(t, bob, "resolve_comment", M{"id": root.At("id").V})
	check(t, resolved.At("resolvedAt").IsString())
	eq(t, resolved.At("resolvingUser", "name").Str(), "Bob Jones")
	notRoot := fails(t, bob, "resolve_comment", M{"id": reply.At("id").V})
	contains(t, notRoot, "first comment")
	reopened := call(t, w.Ann, "unresolve_comment", M{"id": root.At("id").V})
	check(t, reopened.At("resolvedAt").IsNull() && reopened.At("resolvingUser").IsNull())

	notAuthor := fails(t, bob, "save_comment", M{"id": root.At("id").V, "body": "Hijacked"})
	contains(t, notAuthor, "author")
	edited := call(t, w.Ann, "save_comment", M{"id": root.At("id").V, "body": "First, edited"})
	eq(t, edited.At("body").Str(), "First, edited")
	check(t, edited.At("editedAt").IsString())

	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-1", "includeComments": true})
	eq(t, issue.At("commentCount").Num(), 4)
	eq(t, issue.At("comments", 0, "body").Str(), "First, edited")
	eq(t, issue.At("comments", 0, "replies").Len(), 2)

	two := fails(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "projectId": "x", "body": "x"})
	contains(t, two, "only one")
	none := fails(t, w.Ann, "save_comment", M{"body": "x"})
	contains(t, none, "parentId")
	empty := fails(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "  "})
	contains(t, empty, "body")

	forbidden := fails(t, bob, "delete_comment", M{"id": root.At("id").V})
	contains(t, forbidden, "author")
	deleted := call(t, w.Ann, "delete_comment", M{"id": root.At("id").V})
	eq(t, deleted.At("deletedReplies").Num(), 2)
	issue = call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("commentCount").Num(), 1, "the thread's replies went with it")
}

func TestCommentsNotifySubscribersParticipantsAndMentioned(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	carl := w.member("carl@acme.test", "Carl Clark", "member")
	dave := w.member("dave@acme.test", "Dave Brown", "member")
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Crash"})
	root := call(t, bob, "save_comment", M{"issueId": "ENG-1", "body": "Looks bad"})
	call(t, carl, "save_comment", M{"parentId": root.At("id").V, "body": "Agreed, @dave can you check?"})

	inbox := call(t, w.Ann, "get_notifications", M{})
	eqList(t, summaries(inbox), []string{"Carl Clark commented on ENG-1", "Bob Jones commented on ENG-1"})
	eqList(t, kinds(inbox), []string{"issueNewComment", "issueNewComment"})
	eq(t, inbox.At("unreadCount").Num(), 2)
	eq(t, inbox.At("nodes", 0, "actor", "name").Str(), "Carl Clark")
	eq(t, inbox.At("nodes", 0, "issue", "identifier").Str(), "ENG-1")
	eq(t, inbox.At("nodes", 0, "comment", "body").Str(), "Agreed, @dave can you check?")
	bobInbox := call(t, bob, "get_notifications", M{})
	eqList(t, kinds(bobInbox), []string{"issueCommentReply"}, "bob wrote the thread; his own comment didn't notify him")
	daveInbox := call(t, dave, "get_notifications", M{})
	eqList(t, kinds(daveInbox), []string{"issueCommentMention"})
	eqList(t, summaries(daveInbox), []string{"Carl Clark mentioned you in a comment on ENG-1"})
	carlInbox := call(t, carl, "get_notifications", M{})
	eq(t, carlInbox.At("nodes").Len(), 0)
	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	subscribers := issue.At("subscribers").Strings("name")
	for _, name := range []string{"Ann Smith", "Bob Jones", "Carl Clark", "Dave Brown"} {
		found := false
		for _, s := range subscribers {
			found = found || s == name
		}
		check(t, found, name, " is subscribed: ", subscribers)
	}

	newest := inbox.At("nodes", 0, "id").Str()
	oldest := inbox.At("nodes", 1, "id").Str()
	read := call(t, w.Ann, "mark_notification", M{"id": newest, "action": "read"})
	check(t, read.At("nodes", 0, "readAt").IsString())
	inbox = call(t, w.Ann, "get_notifications", M{"unreadOnly": true})
	eq(t, inbox.At("unreadCount").Num(), 1)
	eq(t, inbox.At("nodes").Len(), 1)
	call(t, w.Ann, "mark_notification", M{"ids": []any{newest}, "action": "unread"})
	call(t, w.Ann, "mark_notification", M{"id": oldest, "action": "archive"})
	inbox = call(t, w.Ann, "get_notifications", M{})
	eq(t, inbox.At("nodes").Len(), 1)
	eq(t, inbox.At("unreadCount").Num(), 1)
	all := call(t, w.Ann, "get_notifications", M{"includeArchived": true})
	eq(t, all.At("nodes").Len(), 2)
	call(t, w.Ann, "mark_notification", M{"id": oldest, "action": "unarchive"})

	call(t, w.Ann, "mark_notification", M{"id": newest, "action": "read"})
	snoozed := call(t, w.Ann, "mark_notification", M{"id": newest, "action": "snooze", "until": "P1D"})
	check(t, snoozed.At("nodes", 0, "snoozedUntil").IsString())
	inbox = call(t, w.Ann, "get_notifications", M{})
	eq(t, inbox.At("nodes").Len(), 1, "snoozed notifications are hidden")
	withSnoozed := call(t, w.Ann, "get_notifications", M{"includeSnoozed": true})
	eq(t, withSnoozed.At("nodes").Len(), 2)
	w.exec("UPDATE notifications SET snoozed_until = now() - interval '1 minute' WHERE id = $1::uuid", newest)
	// needs jobs: what its automations do to snoozed notifications.
	w.exec("UPDATE notifications SET snoozed_until = NULL, read_at = NULL WHERE snoozed_until <= now()")
	inbox = call(t, w.Ann, "get_notifications", M{})
	eq(t, inbox.At("nodes", 0, "id").Str(), newest)
	check(t, inbox.At("nodes", 0, "readAt").IsNull() && inbox.At("nodes", 0, "snoozedUntil").IsNull(), "woke up unread")
	eq(t, inbox.At("unreadCount").Num(), 2)

	missing := fails(t, w.Ann, "mark_notification", M{"id": newest, "action": "snooze"})
	contains(t, missing, "until")
	bad := fails(t, w.Ann, "mark_notification", M{"id": newest, "action": "explode"})
	contains(t, bad, "unsnooze")
	notYours := fails(t, bob, "mark_notification", M{"id": newest, "action": "read"})
	contains(t, notYours, "No such notification")

	filtered := call(t, dave, "get_notifications", M{"type": "issueNewComment"})
	eq(t, filtered.At("nodes").Len(), 0)
	marked := call(t, w.Ann, "mark_all_notifications_read", M{"issue": "ENG-1"})
	eq(t, marked.At("marked").Num(), 2)
	inbox = call(t, w.Ann, "get_notifications", M{})
	eq(t, inbox.At("unreadCount").Num(), 0)
}

func TestReactionsAreIdempotent(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Crash"})
	first := call(t, w.Ann, "add_reaction", M{"issueId": "ENG-1", "emoji": ":+1:"})
	eq(t, first.At("emoji").Str(), "+1")
	again := call(t, w.Ann, "add_reaction", M{"issueId": "ENG-1", "emoji": "+1"})
	eq(t, again.At("id").Str(), first.At("id").Str())
	call(t, w.Ann, "add_reaction", M{"issueId": "ENG-1", "emoji": "🎉"})
	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("reactions").Len(), 2)

	comment := call(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "Hi"})
	onComment := call(t, w.Ann, "add_reaction", M{"commentId": comment.At("id").V, "emoji": "eyes"})
	threads := call(t, w.Ann, "list_comments", M{"issueId": "ENG-1"})
	eq(t, threads.At("nodes", 0, "reactions", 0, "emoji").Str(), "eyes")
	call(t, w.Ann, "remove_reaction", M{"commentId": comment.At("id").V, "emoji": ":eyes:"})
	gone := fails(t, w.Ann, "remove_reaction", M{"id": onComment.At("id").V})
	contains(t, gone, "no such reaction")
	call(t, w.Ann, "remove_reaction", M{"id": first.At("id").V})
	issue = call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("reactions", 0, "emoji").Str(), "🎉")

	both := fails(t, w.Ann, "add_reaction", M{"issueId": "ENG-1", "commentId": comment.At("id").V, "emoji": "+1"})
	contains(t, both, "exactly one")
	notEmoji := fails(t, w.Ann, "add_reaction", M{"issueId": "ENG-1", "emoji": "thumbs up"})
	contains(t, notEmoji, "emoji")
}

func TestAttachmentsAreUniquePerURL(t *testing.T) {
	w := world(t)
	pr := "https://github.com/acme/app/pull/7"
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Crash", "links": []any{M{"url": pr, "title": "PR"}}})
	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "links": []any{M{"url": pr, "title": "Fix crash PR"}}})
	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("attachments").Len(), 1)
	eq(t, issue.At("attachments", 0, "title").Str(), "Fix crash PR")
	eq(t, issue.At("attachments", 0, "sourceType").Str(), "github")

	same := call(t, w.Ann, "create_attachment", M{"issue": "ENG-1", "url": pr, "title": "Fix crash", "subtitle": "Open", "metadata": M{"state": "open"}})
	eq(t, same.At("id").Str(), issue.At("attachments", 0, "id").Str())
	eq(t, same.At("subtitle").Str(), "Open")
	eq(t, same.At("metadata", "state").Str(), "open")
	eq(t, same.At("issue", "identifier").Str(), "ENG-1")
	eq(t, same.At("creator", "name").Str(), "Ann Smith")
	design := call(t, w.Ann, "create_attachment", M{"issue": "ENG-1", "url": "https://www.figma.com/file/abc", "title": "Design"})
	eq(t, design.At("sourceType").Str(), "figma")
	listed := call(t, w.Ann, "list_attachments", M{"issue": "ENG-1"})
	eq(t, listed.At("nodes").Len(), 2)
	got := call(t, w.Ann, "get_attachment", M{"id": design.At("id").V})
	eq(t, got.At("title").Str(), "Design")
	call(t, w.Ann, "delete_attachment", M{"id": design.At("id").V})
	listed = call(t, w.Ann, "list_attachments", M{"issue": "ENG-1"})
	eq(t, listed.At("nodes").Len(), 1)

	notLink := fails(t, w.Ann, "create_attachment", M{"issue": "ENG-1", "url": "ftp://x", "title": "x"})
	contains(t, notLink, "http")
	badMetadata := fails(t, w.Ann, "create_attachment", M{"issue": "ENG-1", "url": "https://x.dev", "title": "x", "metadata": []any{1}})
	contains(t, badMetadata, "metadata")
}

func TestFilesUploadOnceAndDownloadForMembers(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Crash"})
	bytes := []byte("\x89PNG fake image")
	prepared := call(t, w.Ann, "prepare_file_upload", M{"name": "shot.png", "contentType": "image/png", "size": len(bytes)})
	uploadURL := prepared.At("uploadUrl").Str()
	assetURL := prepared.At("assetUrl").Str()
	contains(t, prepared.At("curl").Str(), uploadURL)
	eq(t, prepared.At("markdown").Str(), "![shot.png]("+assetURL+")")

	// needs the router (files): the PUT upload, single-use upload URLs and downloads with keys and
	// sessions. The upload is stored as the PUT stores it.
	w.exec(`UPDATE files SET data = $2, size = $3, upload_hash = NULL, upload_expires_at = NULL WHERE id = $1::uuid`,
		prepared.At("fileId").Str(), bytes, len(bytes))

	attachment := call(t, w.Ann, "create_attachment_from_upload", M{"issue": "ENG-1", "fileId": prepared.At("fileId").V})
	eq(t, attachment.At("url").Str(), assetURL)
	eq(t, attachment.At("sourceType").Str(), "upload")
	eq(t, attachment.At("title").Str(), "shot.png")

	inline := call(t, w.Ann, "upload_file", M{"name": "notes.html", "contentType": "text/html", "base64": "PGgxPmhpPC9oMT4="})
	stored := scalar[[]byte](w, "SELECT data FROM files WHERE id = $1::uuid", inline.At("fileId").Str())
	eq(t, string(stored), "<h1>hi</h1>")

	tooBig := fails(t, w.Ann, "prepare_file_upload", M{"name": "big.bin", "contentType": "application/octet-stream", "size": 2 << 20})
	contains(t, tooBig, "at most")
	badType := fails(t, w.Ann, "prepare_file_upload", M{"name": "x", "contentType": "png", "size": 1})
	contains(t, badType, "content type")
	pending := call(t, w.Ann, "prepare_file_upload", M{"name": "later.txt", "contentType": "text/plain", "size": 1})
	notYet := fails(t, w.Ann, "create_attachment_from_upload", M{"issue": "ENG-1", "fileId": pending.At("fileId").V})
	contains(t, notYet, "uploaded")
}

func TestSearchFindsWhatTheCallerCanSee(t *testing.T) {
	w := world(t)
	gus := w.member("gus@acme.test", "Gus Guest", "guest")
	design := w.createTeam("Design", "DES")
	w.exec("INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)", design, gus.User)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Login page crashes", "description": "The login button throws an error."})
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Signup flow", "description": "Mentions login once."})
	call(t, w.Ann, "save_issue", M{"team": "DES", "title": "Login mockups"})
	call(t, w.Ann, "save_comment", M{"issueId": "ENG-2", "body": "There is a deadlock in the scheduler"})
	project := scalar[string](w,
		`INSERT INTO projects (workspace_id, slug_id, name, status_id)
		 VALUES ($1, 'abcdef123456', 'Login Revamp!', (SELECT id FROM project_statuses WHERE workspace_id = $1 LIMIT 1))
		 RETURNING id::text`, w.Workspace)
	w.exec("INSERT INTO project_teams (project_id, team_id) SELECT $1::uuid, id FROM teams WHERE key = 'ENG'", project)
	w.exec(`INSERT INTO documents (workspace_id, slug_id, title, content, team_id)
		 SELECT $1, '0123456789ab', 'Runbook', 'How to debug login crashes in production.', id FROM teams WHERE key = 'ENG'`, w.Workspace)

	found := call(t, w.Ann, "search", M{"query": "login"})
	nodes := found.At("nodes").List()
	titles := found.At("nodes").Strings("title")
	position := func(title string) int {
		for i, t := range titles {
			if t == title {
				return i
			}
		}
		return -1
	}
	for _, title := range []string{"Login page crashes", "Signup flow", "Login mockups", "Login Revamp!", "Runbook"} {
		check(t, position(title) >= 0, title, " in ", titles)
	}
	check(t, position("Login page crashes") < position("Signup flow"), "title matches first: ", titles)
	check(t, position("Login Revamp!") < position("Runbook"), "title matches first: ", titles)
	byType := func(key, value string) J {
		for _, n := range nodes {
			if n.At(key).Str() == value {
				return n
			}
		}
		t.Fatalf("no result with %s %s in %s", key, value, found.JSON())
		return J{}
	}
	projectNode := byType("type", "project")
	eq(t, projectNode.At("url").Str(), "http://localhost:3000/acme/project/login-revamp-abcdef123456")
	eq(t, projectNode.At("status", "name").Str(), "Backlog")
	document := byType("type", "document")
	eq(t, document.At("url").Str(), "http://localhost:3000/acme/document/runbook-0123456789ab")
	contains(t, document.At("snippet").Str(), "**login**")
	issue := byType("identifier", "ENG-1")
	eq(t, issue.At("url").Str(), "http://localhost:3000/acme/issue/ENG-1")
	eq(t, issue.At("state", "name").Str(), "Backlog")

	byIdentifier := call(t, w.Ann, "search", M{"query": "eng-2"})
	eq(t, byIdentifier.At("nodes", 0, "identifier").Str(), "ENG-2")
	onlyIssues := call(t, w.Ann, "search", M{"query": "crash", "types": []any{"issue"}})
	eq(t, onlyIssues.At("nodes").Len(), 1, "stemmed words match: ", onlyIssues.JSON())
	inTeam := call(t, w.Ann, "search", M{"query": "login", "team": "DES"})
	eq(t, inTeam.At("nodes").Len(), 1)
	eq(t, inTeam.At("nodes", 0, "identifier").Str(), "DES-1")
	comments := call(t, w.Ann, "search", M{"query": "deadlock", "types": []any{"comments"}})
	eq(t, comments.At("nodes", 0, "type").Str(), "comment")
	eq(t, comments.At("nodes", 0, "identifier").Str(), "ENG-2")
	defaultTypes := call(t, w.Ann, "search", M{"query": "deadlock"})
	eq(t, defaultTypes.At("nodes").Len(), 0, "comments only when asked for")

	guest := call(t, gus, "search", M{"query": "login", "types": []any{"issue", "project", "document", "comment"}})
	eqList(t, guest.At("nodes").Strings("title"), []string{"Login mockups"})
	guestComments := call(t, gus, "search", M{"query": "deadlock", "types": []any{"comment"}})
	eq(t, guestComments.At("nodes").Len(), 0)

	bad := fails(t, w.Ann, "search", M{"query": "x", "types": []any{"widgets"}})
	contains(t, bad, "initiative")
}

func TestCommentsOnProjectsAndUpdatesNotifyTheMentioned(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	project := scalar[string](w,
		`INSERT INTO projects (workspace_id, slug_id, name, status_id)
		 VALUES ($1, 'abcdef123456', 'Launch', (SELECT id FROM project_statuses WHERE workspace_id = $1 LIMIT 1))
		 RETURNING id::text`, w.Workspace)
	w.exec("INSERT INTO project_teams (project_id, team_id) SELECT $1::uuid, id FROM teams WHERE key = 'ENG'", project)
	update := scalar[string](w,
		`INSERT INTO status_updates (workspace_id, project_id, user_id, body, health)
		 VALUES ($1, $2::uuid, $3, 'On track', 'onTrack') RETURNING id::text`, w.Workspace, project, bob.User)

	comment := call(t, w.Ann, "save_comment", M{"projectId": "Launch", "body": "@bob what's left?"})
	eq(t, comment.At("project", "name").Str(), "Launch")
	call(t, w.Ann, "save_comment", M{"statusUpdateId": update, "body": "Nice progress"})
	inbox := call(t, bob, "get_notifications", M{})
	eqList(t, kinds(inbox), []string{"projectUpdateNewComment", "projectCommentMention"})
	eq(t, summaries(inbox)[1], "Ann Smith mentioned you in a comment on Launch")
	eq(t, inbox.At("nodes", 1, "project", "name").Str(), "Launch")
	eq(t, inbox.At("nodes", 0, "statusUpdate", "id").Str(), update)
	threads := call(t, bob, "list_comments", M{"projectId": "Launch"})
	eq(t, threads.At("nodes", 0, "body").Str(), "@bob what's left?")
	reaction := call(t, bob, "add_reaction", M{"statusUpdateId": update, "emoji": "rocket"})
	eq(t, reaction.At("statusUpdate", "id").Str(), update)
}
