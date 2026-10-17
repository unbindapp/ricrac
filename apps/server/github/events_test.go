package github

import (
	"strings"
	"testing"
)

func TestValidSignature(t *testing.T) {
	body := []byte(`{"zen":"Keep it logically awesome."}`)
	signed := Sign("s3cret", body)
	cases := map[string]struct {
		secret string
		header string
		body   []byte
		want   bool
	}{
		"matching signature":   {secret: "s3cret", header: signed, body: body, want: true},
		"other secret":         {secret: "other", header: signed, body: body},
		"no secret":            {secret: "", header: Sign("", body), body: body},
		"tampered body":        {secret: "s3cret", header: signed, body: []byte(`{"zen":"x"}`)},
		"missing header":       {secret: "s3cret", header: "", body: body},
		"sha1 header":          {secret: "s3cret", header: "sha1=" + signed[len("sha256="):], body: body},
		"not hex":              {secret: "s3cret", header: "sha256=zz", body: body},
		"truncated signature":  {secret: "s3cret", header: signed[:len(signed)-2], body: body},
		"uppercase hex digest": {secret: "s3cret", header: "sha256=" + strings.ToUpper(signed[len("sha256="):]), body: body, want: true},
	}
	for name, tc := range cases {
		if got := ValidSignature(tc.secret, tc.header, tc.body); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

func TestPullRequestDisplayState(t *testing.T) {
	cases := map[string]struct {
		pr   PullRequest
		want string
	}{
		"open":            {pr: PullRequest{State: "open"}, want: StateOpen},
		"draft":           {pr: PullRequest{State: "open", Draft: true}, want: StateDraft},
		"merged":          {pr: PullRequest{State: "closed", Merged: true}, want: StateMerged},
		"closed unmerged": {pr: PullRequest{State: "closed"}, want: StateClosed},
		"closed draft":    {pr: PullRequest{State: "closed", Draft: true}, want: StateClosed},
	}
	for name, tc := range cases {
		if got := tc.pr.DisplayState(); got != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

func TestPushBranches(t *testing.T) {
	master := Repository{DefaultBranch: "master"}
	cases := map[string]struct {
		push      PushEvent
		branch    string
		isDefault bool
	}{
		"branch":                            {push: PushEvent{Ref: "refs/heads/bbedward/unb-1", Repository: master}, branch: "bbedward/unb-1"},
		"default branch":                    {push: PushEvent{Ref: "refs/heads/master", Repository: master}, branch: "master", isDefault: true},
		"tag":                               {push: PushEvent{Ref: "refs/tags/v1.0.0", Repository: master}},
		"tag named like the default branch": {push: PushEvent{Ref: "refs/tags/master", Repository: master}},
		"deleted branch":                    {push: PushEvent{Ref: "refs/heads/master", Deleted: true, Repository: master}},
		"unknown default branch":            {push: PushEvent{Ref: "refs/heads/master"}, branch: "master"},
	}
	for name, tc := range cases {
		branch, _ := tc.push.Branch()
		if branch != tc.branch || tc.push.OnDefaultBranch() != tc.isDefault {
			t.Errorf("%s: got %q (default %v)", name, branch, tc.push.OnDefaultBranch())
		}
	}
}

func TestPushCommitDisplay(t *testing.T) {
	long := strings.Repeat("a", 80)
	cases := map[string]struct {
		commit  PushCommit
		sha     string
		subject string
	}{
		"multi-line message":     {commit: PushCommit{ID: "7f3d2a9c1e4b5f60718293a4b5c6d7e8f9012345", Message: "Cap websocket frames\n\nFixes UNB-5"}, sha: "7f3d2a9", subject: "Cap websocket frames"},
		"crlf message":           {commit: PushCommit{ID: "abc", Message: "Cap frames\r\n\r\nbody"}, sha: "abc", subject: "Cap frames"},
		"long subject":           {commit: PushCommit{ID: "7f3d2a9c", Message: long + "\nbody"}, sha: "7f3d2a9", subject: long[:72]},
		"long multibyte subject": {commit: PushCommit{ID: "7f3d2a9c", Message: strings.Repeat("é", 80)}, sha: "7f3d2a9", subject: strings.Repeat("é", 72)},
	}
	for name, tc := range cases {
		if tc.commit.ShortSHA() != tc.sha || tc.commit.Subject() != tc.subject {
			t.Errorf("%s: got %q %q", name, tc.commit.ShortSHA(), tc.commit.Subject())
		}
	}
}

func TestPushCommitAuthorAndURL(t *testing.T) {
	repository := Repository{FullName: "unbindapp/unbind", HTMLURL: "https://github.com/unbindapp/unbind/"}
	commit := PushCommit{ID: "7f3d2a9c"}
	commit.Author.Name = "Mona Lisa Octocat"
	if commit.AuthorName() != "Mona Lisa Octocat" || commit.HTMLURL(repository) != "https://github.com/unbindapp/unbind/commit/7f3d2a9c" {
		t.Fatalf("got %q %q", commit.AuthorName(), commit.HTMLURL(repository))
	}
	commit.Author.Username = "bbedward"
	commit.URL = "https://github.com/unbindapp/unbind/commit/7f3d2a9cfull"
	if commit.AuthorName() != "bbedward" || commit.HTMLURL(repository) != commit.URL {
		t.Fatalf("got %q %q", commit.AuthorName(), commit.HTMLURL(repository))
	}
}

func TestNormalizeLogin(t *testing.T) {
	cases := map[string]bool{"octocat": true, " @Octo-Cat ": true, "a": true, "-octocat": false, "octocat-": false,
		"octo--cat": false, "octo_cat": false, "a234567890123456789012345678901234567890": false, "": false}
	for raw, want := range cases {
		if _, ok := NormalizeLogin(raw); ok != want {
			t.Errorf("%q: got %v", raw, ok)
		}
	}
	if login, _ := NormalizeLogin(" @Octo-Cat "); login != "octo-cat" {
		t.Errorf("got %q", login)
	}
}

func TestCommentBodies(t *testing.T) {
	if !FromRicrac(RicracCommentBody("Ann *Smith*", "Hi")) || FromRicrac("Hi") || FromRicrac(GitHubCommentBody("octocat", "Hi")) {
		t.Fatal("the app's own comments are recognized, nothing else")
	}
	if got := RicracCommentBody("Ann", "Hi"); got != "**Ann on Ricrac**\n\nHi" {
		t.Fatalf("got %q", got)
	}
}

func TestIssueClosure(t *testing.T) {
	cases := map[Closure]Issue{
		Open:             {State: "open"},
		ClosedCompleted:  {State: "closed", StateReason: "completed"},
		ClosedNotPlanned: {State: "closed", StateReason: "not_planned"},
	}
	for want, issue := range cases {
		if issue.Closure() != want {
			t.Errorf("%+v: got %v", issue, issue.Closure())
		}
	}
	if (Issue{State: "closed", StateReason: "duplicate"}).Closure() != ClosedNotPlanned {
		t.Error("duplicates are not planned")
	}
}
