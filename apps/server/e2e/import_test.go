package e2e

import (
	"encoding/base64"
	"strings"
	"testing"

	"ricrac/app"
)

func TestImportsIssuesWithTheirNumbersDatesPeopleAndComments(t *testing.T) {
	w := world(t)
	w.member("bob@acme.test", "Bob Jones", "member")
	result := call(t, w.Ann, "import_issues", M{"team": "ENG", "issues": []any{
		M{
			"number":      5,
			"title":       "Login fails",
			"description": "Steps for @bob\n\n1. Open  \n2. Crash\n",
			"state":       "Done",
			"priority":    2,
			"labels":      []any{"Bug"},
			"creator":     "bob@acme.test",
			"assignee":    "bob@acme.test",
			"createdAt":   "2025-02-09T08:00:01Z",
			"completedAt": "2025-02-10T09:00:00Z",
			"links":       []any{M{"url": "https://github.com/acme/app/pull/4", "title": "Fix login", "subtitle": "master"}},
			"comments": []any{
				M{"body": "Seen on staging", "author": "bob@acme.test", "createdAt": "2025-02-09T10:00:00Z"},
				M{"body": "Fixed", "createdAt": "2025-02-10T09:00:00Z"},
			},
		},
		M{"number": 7, "title": "Login is slow", "parent": "ENG-5", "relatedTo": []any{"ENG-5"}, "blockedBy": []any{"ENG-8"}},
		M{"title": "Numbered next", "state": "Todo"},
	}})
	eqList(t, result.At("identifiers").Strings(), []string{"ENG-5", "ENG-7", "ENG-8"})
	eq(t, result.At("comments").Num(), 2)
	eq(t, result.At("nextIdentifier").Str(), "ENG-9")

	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-5", "includeComments": true})
	eq(t, issue.At("description").Str(), "Steps for @bob\n\n1. Open  \n2. Crash\n", "kept as is")
	eq(t, issue.At("state", "name").Str(), "Done")
	eq(t, issue.At("creator", "name").Str(), "Bob Jones")
	eq(t, issue.At("assignee", "name").Str(), "Bob Jones")
	check(t, strings.HasPrefix(issue.At("createdAt").Str(), "2025-02-09T08:00:01"))
	check(t, strings.HasPrefix(issue.At("completedAt").Str(), "2025-02-10T09:00:00"))
	check(t, strings.HasPrefix(issue.At("startedAt").Str(), "2025-02-09T08:00:01"), "set from createdAt")
	eq(t, issue.At("labels", 0, "name").Str(), "Bug")
	eq(t, issue.At("attachments", 0, "subtitle").Str(), "master")
	comments := issue.At("comments")
	eq(t, comments.At(0, "user", "name").Str(), "Bob Jones")
	check(t, strings.HasPrefix(comments.At(0, "createdAt").Str(), "2025-02-09T10:00:00"))
	eq(t, comments.At(1, "user", "name").Str(), "Ann Smith")
	eq(t, issue.At("relations", "related", 0, "identifier").Str(), "ENG-7")
	eq(t, issue.At("children", 0, "identifier").Str(), "ENG-7")

	next := call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "After the import"})
	eq(t, next.At("identifier").Str(), "ENG-9")
	slow := call(t, w.Ann, "get_issue", M{"id": "ENG-7"})
	eq(t, slow.At("relations", "blockedBy", 0, "identifier").Str(), "ENG-8", "points at an issue later in the import")

	gap := call(t, w.Ann, "import_issues", M{"team": "ENG", "issues": []any{M{"number": 6, "title": "Filled gap"}}})
	eqList(t, gap.At("identifiers").Strings(), []string{"ENG-6"})
	eq(t, gap.At("nextIdentifier").Str(), "ENG-10", "filling a gap doesn't move the counter back")

	eq(t, scalar[int64](w, "SELECT count(*) FROM notifications"), 0, "imports notify nobody")
}

func TestRefusesTakenNumbersAndImportsNothingOnError(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Existing"})
	contains(t, fails(t, w.Ann, "import_issues", M{"team": "ENG", "issues": []any{M{"number": 1, "title": "x"}}}), "ENG-1")
	twice := fails(t, w.Ann, "import_issues", M{"team": "ENG", "issues": []any{M{"number": 3, "title": "a"}, M{"number": 3, "title": "b"}}})
	contains(t, twice, "ENG-3 appears twice")
	badLabel := fails(t, w.Ann, "import_issues", M{"team": "ENG", "issues": []any{
		M{"number": 2, "title": "ok"}, M{"number": 3, "title": "bad", "labels": []any{"Nope"}},
	}})
	check(t, strings.HasPrefix(badLabel, "ENG-3:"), "says which issue failed: ", badLabel)
	contains(t, fails(t, w.Ann, "get_issue", M{"id": "ENG-2"}), "No issue")

	dry := call(t, w.Ann, "import_issues", M{"team": "ENG", "dryRun": true, "issues": []any{M{"number": 2, "title": "Dry"}}})
	eqList(t, dry.At("identifiers").Strings(), []string{"ENG-2"})
	fails(t, w.Ann, "get_issue", M{"id": "ENG-2"})

	bob := w.member("bob@acme.test", "Bob Jones", "member")
	contains(t, fails(t, bob, "import_issues", M{"team": "ENG", "issues": []any{M{"title": "x"}}}), "admins")
}

func TestImportsIssuesFromAnUploadedFile(t *testing.T) {
	w := world(t)
	file := app.Marshal(M{"issues": []any{M{"number": 42, "title": "From a file", "description": "Long text"}}})
	upload := call(t, w.Ann, "upload_file", M{
		"name":        "issues.json",
		"contentType": "application/json",
		"base64":      base64.StdEncoding.EncodeToString(file),
	})
	result := call(t, w.Ann, "import_issues", M{"team": "ENG", "fileId": upload.At("fileId").V})
	eqList(t, result.At("identifiers").Strings(), []string{"ENG-42"})
	eq(t, call(t, w.Ann, "get_issue", M{"id": "ENG-42"}).At("description").Str(), "Long text")
	both := fails(t, w.Ann, "import_issues", M{"team": "ENG", "fileId": upload.At("fileId").V, "issues": []any{}})
	contains(t, both, "either issues or fileId")
}
