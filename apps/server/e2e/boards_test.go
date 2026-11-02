package e2e

import "testing"

func cardTitles(page J) []string {
	var titles []string
	for i := 0; i < page.At("nodes").Len(); i++ {
		titles = append(titles, page.At("nodes", i, "column", "name").Str()+": "+page.At("nodes", i, "issue", "title").Str())
	}
	return titles
}

func TestBoardsHoldIssuesInColumnsOfTheirOwn(t *testing.T) {
	w := world(t)
	gus := w.member("gus@acme.test", "Gus Guest", "guest")
	board := call(t, w.Ann, "save_board", M{"name": "Ideas"})
	eqList(t, board.At("columns").Strings("name"), []string{"Ideas", "Backlog", "Doing", "Done"})
	eq(t, board.At("url").Str(), "http://localhost:3000/acme/board/ideas-"+board.At("slugId").Str())

	call(t, w.Ann, "save_board", M{"id": "Ideas", "columns": []M{{"name": "Ideas"}, {"name": "Design phase"}, {"name": "Design ready", "color": "#4cb782"}}})
	call(t, w.Ann, "save_issue", M{"title": "Dark mode", "githubRepository": "none"})
	call(t, w.Ann, "save_issue", M{"title": "Clock tab"})
	call(t, w.Ann, "save_issue", M{"title": "Widgets"})
	call(t, w.Ann, "place_on_board", M{"board": "Ideas", "issue": "ENG-1", "column": "ideas"})
	call(t, w.Ann, "place_on_board", M{"board": "Ideas", "issue": "ENG-2", "column": "Ideas"})
	placed := call(t, w.Ann, "place_on_board", M{"board": "Ideas", "issue": "ENG-3", "column": "Design phase"})
	eq(t, placed.At("issue", "identifier").Str(), "ENG-3")
	eqList(t, cardTitles(call(t, w.Ann, "list_board_issues", M{"board": "Ideas"})),
		[]string{"Ideas: Dark mode", "Ideas: Clock tab", "Design phase: Widgets"})

	call(t, w.Ann, "place_on_board", M{"board": "Ideas", "issue": "ENG-2", "column": "Ideas", "sortOrder": 0})
	call(t, w.Ann, "place_on_board", M{"board": "Ideas", "issue": "ENG-1", "column": "Design ready"})
	eqList(t, cardTitles(call(t, w.Ann, "list_board_issues", M{"board": "Ideas"})),
		[]string{"Ideas: Clock tab", "Design phase: Widgets", "Design ready: Dark mode"})
	eqList(t, cardTitles(call(t, w.Ann, "list_board_issues", M{"board": "Ideas", "column": "Design ready"})), []string{"Design ready: Dark mode"})
	contains(t, fails(t, w.Ann, "place_on_board", M{"board": "Ideas", "issue": "ENG-1", "column": "Shipped"}), "Design ready")
	eq(t, call(t, gus, "get_board", M{"id": "Ideas"}).At("columns", 0, "issueCount").Num(), 0, "guests count only what they see")

	design := call(t, w.Ann, "get_board", M{"id": "Ideas"}).At("columns", 1)
	eq(t, design.At("issueCount").Num(), 1)
	renamed := call(t, w.Ann, "save_board", M{"id": "Ideas", "columns": []M{{"id": design.At("id").V, "name": "Designing"}, {"name": "Ideas"}}})
	eqList(t, renamed.At("columns").Strings("name"), []string{"Designing", "Ideas"})
	eqList(t, cardTitles(call(t, w.Ann, "list_board_issues", M{"board": "Ideas"})),
		[]string{"Designing: Widgets", "Ideas: Clock tab"}, "the removed column's issues leave the board")
	contains(t, fails(t, w.Ann, "save_board", M{"id": "Ideas", "columns": []M{{"name": "A"}, {"name": "a"}}}), "Two columns")
	contains(t, fails(t, w.Ann, "save_board", M{"id": "Ideas", "columns": []M{}}), "at least one column")

	removed := call(t, w.Ann, "remove_from_board", M{"board": "Ideas", "issue": "ENG-2"})
	check(t, removed.At("removed").Bool())
	check(t, !call(t, w.Ann, "remove_from_board", M{"board": "Ideas", "issue": "ENG-2"}).At("removed").Bool())
	call(t, w.Ann, "delete_issue", M{"id": "ENG-3"})
	eq(t, call(t, w.Ann, "list_board_issues", M{"board": "Ideas"}).At("nodes").Len(), 0, "trashed issues leave the board's list")

	call(t, w.Ann, "save_board", M{"name": "Roadmap"})
	eqList(t, names(call(t, w.Ann, "list_boards", M{})), []string{"Ideas", "Roadmap"})
	call(t, w.Ann, "delete_board", M{"id": "Ideas"})
	eqList(t, names(call(t, w.Ann, "list_boards", M{})), []string{"Roadmap"})
	call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
}

func TestIssuesKeptOffGitHubOpenThereWhenAsked(t *testing.T) {
	g := withGitHub(t)
	g.syncRepo(t, "ENG")
	call(t, g.Ann, "save_team", M{"id": "ENG", "githubOpenIssues": true})
	call(t, g.Ann, "save_issue", M{"title": "Dark mode", "githubRepository": "none"})
	g.runJobs(t)
	eq(t, len(g.fake.opened()), 0)

	contains(t, fails(t, g.Ann, "save_issue", M{"id": "ENG-1", "githubRepository": "none"}), "only applies when creating")
	contains(t, fails(t, g.Ann, "save_issue", M{"id": "ENG-1", "githubRepository": "unbindapp/other"}), "unbindapp/unbind")
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "githubRepository": "unbind"})
	g.runJobs(t)
	opened := g.fake.opened()
	eq(t, len(opened), 1)
	eq(t, opened[0].Body["title"], any("Dark mode"))
	check(t, !call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("github").IsNull())
	contains(t, fails(t, g.Ann, "save_issue", M{"id": "ENG-1", "githubRepository": "unbind"}), "already on GitHub")
}
