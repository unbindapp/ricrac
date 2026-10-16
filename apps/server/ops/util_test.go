package ops

import (
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestDurations(t *testing.T) {
	cases := map[string]time.Duration{
		"-P1D":     -24 * time.Hour,
		"P2W":      14 * 24 * time.Hour,
		"-PT3H30M": -(3*time.Hour + 30*time.Minute),
	}
	for value, want := range cases {
		if got, ok := parseDuration(value); !ok || got != want {
			t.Errorf("%s: got %v, want %v", value, got, want)
		}
	}
	for _, value := range []string{"P", "1D"} {
		if _, ok := parseDuration(value); ok {
			t.Errorf("%s should not parse", value)
		}
	}
	for _, value := range []string{"2026-01-31", "2026-01-31T10:00:00+02:00"} {
		if _, err := parseDatetime(value); err != nil {
			t.Errorf("%s: %v", value, err)
		}
	}
	if _, err := parseDatetime("yesterday"); err == nil {
		t.Error("yesterday should not parse")
	}
}

func TestPatchesApplyInOrderOrNotAtAll(t *testing.T) {
	text := "# Title\n\nOne\n\nTwo\n"
	got, err := applyPatch(text, []TextEdit{
		{Op: "replace", OldString: "One", NewString: "1"},
		{Op: "insert_after", Anchor: "Two", Text: " and three"},
		{Op: "append", Text: "End\n"},
	})
	if err != nil || got != "# Title\n\n1\n\nTwo and three\nEnd\n" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := applyPatch(text, []TextEdit{{Op: "replace", OldString: "\n\n", NewString: ""}}); err == nil {
		t.Fatal("an ambiguous replacement should fail")
	}
	got, err = applyPatch(text, []TextEdit{{Op: "replace_range", From: "One", To: "Two", NewString: ""}})
	if err != nil || got != "# Title\n\nTwo\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestPages(t *testing.T) {
	page := Page{Limit: new(int64(2))}
	wrapped, err := page.wrap([]any{1, 2, 3})
	if err != nil || len(wrapped["nodes"].([]any)) != 2 {
		t.Fatalf("got %v, %v", wrapped, err)
	}
	cursor := wrapped["pageInfo"].(M)["endCursor"].(string)
	next := Page{Limit: new(int64(2)), Cursor: &cursor}
	if offset, _ := next.offset(); offset != 2 {
		t.Fatalf("offset %d", offset)
	}
	last, _ := next.wrap([]any{3})
	if last["pageInfo"].(M)["hasNextPage"] != false {
		t.Fatal("the last page has no next page")
	}
}

func TestMentions(t *testing.T) {
	if got := mentionHandles("Hey @ann, can @bob.smith look? cc @Carl."); !slices.Equal(got, []string{"ann", "bob.smith", "carl"}) {
		t.Errorf("got %v", got)
	}
	if got := mentionHandles("mail me at x@y.com"); len(got) != 0 {
		t.Errorf("got %v", got)
	}
	if got := mentionHandles("@ann@corp.com please"); !slices.Equal(got, []string{"ann@corp.com"}) {
		t.Errorf("got %v", got)
	}
}

func TestIdentifiersSlugsAndLinks(t *testing.T) {
	for value, want := range map[string]string{"eng-123": "ENG-123", "ENG2-7": "ENG2-7"} {
		key, number, ok := parseIdentifier(value)
		if !ok || key+"-"+strconv.Itoa(number) != want {
			t.Errorf("%s: %s %d %v", value, key, number, ok)
		}
	}
	for _, value := range []string{"ENG-", "123", "my-project-0a1b2c3d4e5f"} {
		if _, _, ok := parseIdentifier(value); ok {
			t.Errorf("%s should not parse", value)
		}
	}
	cases := map[string]string{
		"https://x.dev/acme/project/my-project-0a1b2c3d4e5f": "0a1b2c3d4e5f",
		"0a1b2c3d4e5f": "0a1b2c3d4e5f",
		"Launch":       "Launch",
	}
	for value, want := range cases {
		if got := slugTail(value); got != want {
			t.Errorf("%s: got %s", value, got)
		}
	}
	if got := lastSegment("https://x.dev/acme/view/abc/?q=1"); got != "abc" {
		t.Errorf("got %s", got)
	}
}

func TestEstimateScales(t *testing.T) {
	if got := allowedEstimates("fibonacci", false, false); !slices.Equal(got, []float64{1, 2, 3, 5, 8}) {
		t.Errorf("got %v", got)
	}
	if got := allowedEstimates("exponential", true, true); !slices.Equal(got, []float64{0, 1, 2, 4, 8, 16, 32, 64}) {
		t.Errorf("got %v", got)
	}
	if got := allowedEstimates("notUsed", true, true); !slices.Equal(got, []float64{0}) {
		t.Errorf("got %v", got)
	}
}

func TestUpdatedFromListsPreviousValues(t *testing.T) {
	got := changedFields(M{"title": "a", "priority": 1, "updatedAt": "x"}, M{"title": "b", "priority": 1, "updatedAt": "y"})
	if len(got) != 1 || got["title"] != "a" {
		t.Errorf("got %v", got)
	}
}

func TestEmojiAreShortcodesOrUnicode(t *testing.T) {
	for value, want := range map[string]string{":tada:": "tada", "+1": "+1", "👍": "👍"} {
		if got, err := normalizeEmoji(value); err != nil || got != want {
			t.Errorf("%s: got %s, %v", value, got, err)
		}
	}
	for _, value := range []string{"not an emoji", "::"} {
		if _, err := normalizeEmoji(value); err == nil {
			t.Errorf("%s should fail", value)
		}
	}
}

func TestSourcesFromHosts(t *testing.T) {
	base := "https://ricrac.example.com"
	cases := map[string]string{
		"https://github.com/acme/app/pull/1":           "github",
		"https://www.figma.com/file/x":                 "figma",
		"https://acme.slack.com/archives/C1/p1":        "slack",
		"https://gitlab.acme.dev/x/-/merge_requests/2": "gitlab",
		"https://ricrac.example.com/files/123":         "upload",
		"https://user@notgithub.com:8080/x":            "link",
	}
	for url, want := range cases {
		if got := sourceType(url, base); got != want {
			t.Errorf("%s: got %s", url, got)
		}
	}
}

func TestBase64Variants(t *testing.T) {
	for _, value := range []string{"aGk=", "aGk", "data:text/plain;base64,aGk="} {
		if got, err := decodeBase64(value); err != nil || string(got) != "hi" {
			t.Errorf("%s: got %q, %v", value, got, err)
		}
	}
	if _, err := decodeBase64("not base64!"); err == nil {
		t.Error("should fail")
	}
}

func TestTeamKeys(t *testing.T) {
	for name, want := range map[string]string{"Engineering": "ENG", "Mobile Platform Team": "MPT", "  ": "TEAM"} {
		if got := TeamKey(name); got != want {
			t.Errorf("%s: got %s", name, got)
		}
	}
}
