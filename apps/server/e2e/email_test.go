package e2e

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// outbox records the emails it is asked to send, failing while fail is set.
type outbox struct {
	mu   sync.Mutex
	sent []mailMessage
	fail bool
}

func (o *outbox) Send(_ context.Context, message mailMessage) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.fail {
		return errors.New("smtp is down")
	}
	o.sent = append(o.sent, message)
	return nil
}

func (o *outbox) to(address string) []mailMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	var found []mailMessage
	for _, m := range o.sent {
		if m.To == address {
			found = append(found, m)
		}
	}
	return found
}

func sendEmails(t *testing.T, w *World) {
	t.Helper()
	if err := w.B.SendEmails(context.Background()); err != nil {
		t.Fatalf("send emails: %v", err)
	}
}

func TestUnreadNotificationsAreEmailedAsADigest(t *testing.T) {
	w := world(t)
	box := &outbox{}
	w.B.SetMailer(box)
	w.member("bob@acme.test", "Bob Jones", "member")
	cat := w.member("cat@acme.test", "Cat Lee", "member")
	w.exec("UPDATE users SET email_notifications = false WHERE id = $1", cat.User)
	call(t, w.Ann, "save_issue", M{"title": "Fix login", "assignee": "bob", "description": "cc @cat"})
	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "state": "In Progress"})
	call(t, w.Ann, "save_issue", M{"title": "Billing", "assignee": "bob"})

	sendEmails(t, w)
	eq(t, len(box.sent), 0, "notifications wait a moment in case they're read")

	w.exec("UPDATE notifications SET created_at = created_at - interval '6 minutes'")
	w.exec("UPDATE notifications SET read_at = now() WHERE issue_id = (SELECT id FROM issues WHERE number = 2)")
	sendEmails(t, w)
	toBob := box.to("bob@acme.test")
	eq(t, len(toBob), 1)
	email := toBob[0]
	eq(t, email.Subject, "ENG-1 Fix login: Ann Smith assigned the issue to you (+1 more)")
	contains(t, email.Text,
		"You have 2 unread notifications in Acme.",
		"ENG-1 Fix login",
		"Ann Smith assigned the issue to you",
		"Ann Smith moved the issue to In Progress",
		"http://localhost:3000/acme/issue/ENG-1",
	)
	check(t, !strings.Contains(email.Text, "Billing"), "read notifications aren't emailed")
	eq(t, len(box.to("cat@acme.test")), 0, "Cat turned emails off")

	sendEmails(t, w)
	eq(t, len(box.sent), 1, "each notification is emailed once")
}

func TestDigestsComeAtMostHourlyAndQuoteComments(t *testing.T) {
	w := world(t)
	box := &outbox{}
	w.B.SetMailer(box)
	w.member("bob@acme.test", "Bob Jones", "member")
	age := func() { w.exec("UPDATE notifications SET created_at = created_at - interval '6 minutes'") }
	call(t, w.Ann, "save_issue", M{"title": "Fix login", "assignee": "bob"})
	age()
	sendEmails(t, w)
	eq(t, len(box.to("bob@acme.test")), 1)

	call(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "Looks **good**, see [the doc](http://x.test/doc)"})
	call(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "Also the logout button"})
	age()
	sendEmails(t, w)
	eq(t, len(box.to("bob@acme.test")), 1, "comments wait for the hour since the last digest")

	w.exec("UPDATE notifications SET emailed_at = emailed_at - interval '1 hour'")
	sendEmails(t, w)
	toBob := box.to("bob@acme.test")
	eq(t, len(toBob), 2)
	eq(t, toBob[1].Subject, "ENG-1 Fix login: Ann Smith commented (+1 more)")
	contains(t, toBob[1].Text,
		"Ann Smith commented\n    > Looks good, see the doc\n",
		"Ann Smith commented\n    > Also the logout button\n",
	)
	check(t, !strings.Contains(toBob[1].Text, "inbox"), "the digest carries the comments, not a link to the inbox")
}

func TestDigestsLinkDiscussionThreads(t *testing.T) {
	w := world(t)
	box := &outbox{}
	w.B.SetMailer(box)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_discussion", M{"title": "Q4 launch", "body": "Plan it"})
	thread := call(t, bob, "save_discussion_thread", M{"discussion": "Q4 launch", "kind": "bug", "title": "Button misaligned", "body": "Off by 2px"})
	w.exec("UPDATE notifications SET created_at = created_at - interval '6 minutes'")
	sendEmails(t, w)
	toAnn := box.to("ann@acme.test")
	eq(t, len(toAnn), 1)
	eq(t, toAnn[0].Subject, "Button misaligned: Bob Jones started a thread")
	contains(t, toAnn[0].Text, "    > Off by 2px\n", thread.At("url").Str())
}

func TestFailedEmailsAreRetried(t *testing.T) {
	w := world(t)
	box := &outbox{fail: true}
	w.B.SetMailer(box)
	w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "Fix login", "assignee": "bob"})
	w.exec("UPDATE notifications SET created_at = created_at - interval '6 minutes'")
	sendEmails(t, w)
	eq(t, len(box.sent), 0)
	eqList(t, w.strings("SELECT error FROM emails"), []string{"smtp is down"})

	box.fail = false
	w.exec("UPDATE emails SET send_after = now()")
	sendEmails(t, w)
	eq(t, len(box.to("bob@acme.test")), 1)
	eq(t, box.sent[0].Subject, "ENG-1 Fix login: Ann Smith assigned the issue to you")
}

func TestInvitationsAreEmailed(t *testing.T) {
	w := world(t)
	box := &outbox{}
	w.B.SetMailer(box)
	call(t, w.Ann, "invite_user", M{"email": "New@Example.com", "role": "admin"})
	sendEmails(t, w)
	sent := box.to("new@example.com")
	eq(t, len(sent), 1)
	eq(t, sent[0].Subject, "Ann Smith invited you to Acme on Ricrac")
	contains(t, sent[0].Text, "as an admin", "Sign in as new@example.com", "http://localhost:3000")
}

func TestPeopleTurnEmailNotificationsOff(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	eq(t, call(t, bob, "get_user", M{"id": "me"}).At("emailNotifications").Bool(), true)
	off := call(t, bob, "update_user", M{"id": "me", "emailNotifications": false})
	eq(t, off.At("emailNotifications").Bool(), false)
	check(t, call(t, w.Ann, "get_user", M{"id": "bob"}).At("emailNotifications").IsNull(), "only you see your setting")
	contains(t, fails(t, w.Ann, "update_user", M{"id": "bob", "emailNotifications": true}), "your own")
}

func TestAnyEmailSignsInWithALinkThatWorksOnce(t *testing.T) {
	w := world(t)
	box := &outbox{}
	w.B.SetMailer(box)
	call(t, w.Ann, "invite_user", M{"email": "new@example.com", "role": "member"})
	post := func(path string, body M) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(string(marshal(body))))
		req.Header.Set("X-Ricrac", "1")
		recorder := httptest.NewRecorder()
		w.Router.ServeHTTP(recorder, req)
		return recorder
	}
	emailedLink := func(to string) url.Values {
		t.Helper()
		sent := box.to(to)
		check(t, len(sent) > 0, "a link went to ", to)
		for line := range strings.Lines(sent[len(sent)-1].Text) {
			if link, ok := strings.CutPrefix(strings.TrimSpace(line), publicURL+"/sign-in/email?"); ok {
				query, _ := url.ParseQuery(link)
				return query
			}
		}
		t.Fatalf("no link in %s", sent[len(sent)-1].Text)
		return nil
	}
	finish := func(link url.Values) *httptest.ResponseRecorder {
		return post("/api/auth/email/callback", M{"email": link.Get("email"), "token": link.Get("token")})
	}
	me := func(signedIn *httptest.ResponseRecorder) J {
		req := httptest.NewRequest("GET", "/api/me", nil)
		for _, cookie := range signedIn.Result().Cookies() {
			req.AddCookie(cookie)
		}
		_, found := send(w.Router, req)
		return found
	}

	req := httptest.NewRequest("POST", "/api/auth/email", strings.NewReader(`{"email":"new@example.com"}`))
	status, _ := send(w.Router, req)
	eq(t, status, http.StatusForbidden, "writes need the x-ricrac header")
	eq(t, post("/api/auth/email", M{"email": "not an email"}).Code, http.StatusBadRequest)

	eq(t, post("/api/auth/email", M{"email": " New@Example.com ", "returnTo": "/acme"}).Code, http.StatusOK)
	sent := box.to("new@example.com")
	eq(t, len(sent), 1)
	eq(t, sent[0].Subject, "Sign in to Ricrac")
	link := emailedLink("new@example.com")
	eq(t, link.Get("email"), "new@example.com")

	forged := url.Values{"email": {"new@example.com"}, "token": {"forged"}}
	eq(t, finish(forged).Code, http.StatusBadRequest)
	elsewhere := url.Values{"email": {"ann@acme.test"}, "token": {link.Get("token")}}
	eq(t, finish(elsewhere).Code, http.StatusBadRequest, "a link only signs in as its own email")

	signedIn := finish(link)
	eq(t, signedIn.Code, http.StatusOK, signedIn.Body.String())
	contains(t, signedIn.Body.String(), `"returnTo":"/acme"`)
	newcomer := me(signedIn)
	eq(t, newcomer.At("email").Str(), "new@example.com")
	eq(t, newcomer.At("displayName").Str(), "new")
	eq(t, newcomer.At("invitations", 0, "workspaceName").Str(), "Acme")
	again := finish(link)
	eq(t, again.Code, http.StatusBadRequest)
	contains(t, again.Body.String(), "expired or was already used")

	eq(t, post("/api/auth/email", M{"email": "ann@acme.test", "returnTo": "https://evil.com"}).Code, http.StatusOK)
	ann := finish(emailedLink("ann@acme.test"))
	contains(t, ann.Body.String(), `"returnTo":"/"`)
	eq(t, me(ann).At("id").Str(), w.Ann.User.String(), "an existing account signs in by its email")

	eq(t, post("/api/auth/email", M{"email": "new@example.com"}).Code, http.StatusOK)
	w.exec("UPDATE sign_in_links SET expires_at = now() - interval '1 second'")
	eq(t, finish(emailedLink("new@example.com")).Code, http.StatusBadRequest, "expired links don't sign in")
	for range 3 {
		eq(t, post("/api/auth/email", M{"email": "new@example.com"}).Code, http.StatusOK)
	}
	tooMany := post("/api/auth/email", M{"email": "new@example.com"})
	eq(t, tooMany.Code, http.StatusTooManyRequests)
	eq(t, len(box.to("new@example.com")), 5)

	box.fail = true
	eq(t, post("/api/auth/email", M{"email": "cat@example.com"}).Code, http.StatusBadGateway)
}

func TestSigningInByEmailNeedsEmail(t *testing.T) {
	w := world(t)
	req := httptest.NewRequest("POST", "/api/auth/email", strings.NewReader(`{"email":"new@example.com"}`))
	req.Header.Set("X-Ricrac", "1")
	status, answer := send(w.Router, req)
	eq(t, status, http.StatusBadRequest)
	contains(t, answer.At("error").Str(), "isn't set up")
}
