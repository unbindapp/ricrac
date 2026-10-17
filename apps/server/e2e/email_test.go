package e2e

import (
	"context"
	"errors"
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

	w.exec("UPDATE notifications SET created_at = created_at - interval '3 minutes'")
	w.exec("UPDATE notifications SET read_at = now() WHERE issue_id = (SELECT id FROM issues WHERE number = 2)")
	sendEmails(t, w)
	toBob := box.to("bob@acme.test")
	eq(t, len(toBob), 1)
	email := toBob[0]
	eq(t, email.Subject, "2 unread notifications")
	contains(t, email.Text,
		"You have 2 unread notifications in Acme.",
		"ENG-1 Fix login",
		"Ann Smith assigned the issue to you",
		"Ann Smith moved the issue to In Progress",
		"http://localhost:3000/acme/issue/ENG-1",
		"http://localhost:3000/acme/inbox",
	)
	check(t, !strings.Contains(email.Text, "Billing"), "read notifications aren't emailed")
	eq(t, len(box.to("cat@acme.test")), 0, "Cat turned emails off")

	sendEmails(t, w)
	eq(t, len(box.sent), 1, "each notification is emailed once")
}

func TestFailedEmailsAreRetried(t *testing.T) {
	w := world(t)
	box := &outbox{fail: true}
	w.B.SetMailer(box)
	w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "Fix login", "assignee": "bob"})
	w.exec("UPDATE notifications SET created_at = created_at - interval '3 minutes'")
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
	contains(t, sent[0].Text, "as an admin", "Sign in with Google as new@example.com", "http://localhost:3000")
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
