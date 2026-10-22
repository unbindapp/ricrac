package mail

import (
	"strings"
	"testing"
)

func digestContent() Content {
	commented := ActorLine("7d4e2f60-1c2b-4a8e-b0a1-9e5c3d2f4a77", "Ada Lovelace", "commented")
	commented.Excerpt = "Fixed in main.\nShipping today."
	return Content{
		Subject:   "3 unread notifications",
		Preheader: "Ann Smith assigned the issue to you",
		Heading:   "You have 3 unread notifications in Acme.",
		Items: []Item{
			{
				Identifier: "ENG-12",
				Title:      "Fix login",
				URL:        "https://ricrac.example.com/acme/issue/ENG-12",
				Lines: []Line{
					ActorLine("0b9f6c1e-5d1a-4c55-9a43-3f7f1a2b8c11", "Ann Smith", "assigned the issue to you"),
					ActorLine("7d4e2f60-1c2b-4a8e-b0a1-9e5c3d2f4a77", "Ada Lovelace", "moved the issue to Done"),
					commented,
				},
			},
			{
				Identifier: "ENG-7",
				Title:      "Billing page",
				URL:        "https://ricrac.example.com/acme/issue/ENG-7",
				Lines:      []Line{{Text: "Due today"}},
			},
		},
		Button: Button{Label: "Open your inbox", URL: "https://ricrac.example.com/acme/inbox"},
		Footer: Footer{
			InstanceURL: "https://ricrac.example.com",
			ManageURL:   "https://ricrac.example.com/settings/notifications",
			ManageLabel: "Notification settings",
		},
	}
}

func mustContain(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("missing %q", part)
		}
	}
}

func mustNotContain(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if strings.Contains(text, part) {
			t.Errorf("unexpected %q", part)
		}
	}
}

func TestRenderDigest(t *testing.T) {
	msg, err := Render("grace@example.com", digestContent())
	if err != nil {
		t.Fatal(err)
	}
	if msg.To != "grace@example.com" || msg.Subject != "3 unread notifications" {
		t.Fatalf("got %q %q", msg.To, msg.Subject)
	}
	want := `You have 3 unread notifications in Acme.

ENG-12 Fix login
    Ann Smith assigned the issue to you
    Ada Lovelace moved the issue to Done
    Ada Lovelace commented
    > Fixed in main.
    > Shipping today.
    https://ricrac.example.com/acme/issue/ENG-12

ENG-7 Billing page
    Due today
    https://ricrac.example.com/acme/issue/ENG-7

Open your inbox
https://ricrac.example.com/acme/inbox

--
Ricrac https://ricrac.example.com
Notification settings: https://ricrac.example.com/settings/notifications
`
	if msg.Text != want {
		t.Fatalf("text:\n%s", msg.Text)
	}
	mustContain(t, msg.HTML,
		`@media (prefers-color-scheme: dark)`,
		`<a href="https://ricrac.example.com/acme/issue/ENG-12"`,
		`>ENG-12</span>`,
		`>AS</td>`,
		`>AL</td>`,
		`Ada Lovelace</span> moved the issue to Done`,
		`>Due today</td>`,
		"white-space:pre-line;\">Fixed in main.\nShipping today.</div>",
		`>Open your inbox</a>`,
		`>Notification settings</a>`,
		`src="cid:logo.png"`,
	)
	mustNotContain(t, msg.HTML, "Or open this link")
	if len(msg.Inline) != 1 || msg.Inline[0].Name != "logo.png" {
		t.Fatal("the logo is inlined")
	}
}

func TestRenderInvitation(t *testing.T) {
	msg, err := Render("new@example.com", Content{
		Subject: "Ada Lovelace invited you to Acme on Ricrac",
		Heading: "Join Acme on Ricrac",
		Intro:   []string{"Ada Lovelace invited you to the Acme workspace as a member."},
		Button:  Button{Label: "Accept invitation", URL: "https://ricrac.example.com/"},
		Note:    "This invitation expires in 14 days. If you were not expecting it, you can ignore this email.",
	})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, msg.HTML,
		">Join Acme on Ricrac</td>",
		`mso-hide:all;">Ada Lovelace invited you to the Acme workspace as a member.&zwnj;`,
		`Or open this link: <a href="https://ricrac.example.com/"`,
	)
	mustNotContain(t, msg.HTML, `class="rr-rule"`, "Notification settings")
	if !strings.HasSuffix(msg.Text, "--\nRicrac\n") {
		t.Fatalf("text ends with %q", msg.Text[len(msg.Text)-20:])
	}
}

func TestRenderEscapesUserInput(t *testing.T) {
	content := digestContent()
	content.Heading = "Join <b>Acme</b>"
	content.Items[0].Title = "<script>alert(1)</script>"
	content.Items[0].URL = "javascript:alert(1)"
	content.Items[0].Lines[0].ActorName = `<a href="https://evil.example">Mallory</a>`
	content.Items[0].Lines[2].Excerpt = `<img src=x onerror=alert(1)>`
	msg, err := Render("grace@example.com", content)
	if err != nil {
		t.Fatal(err)
	}
	mustNotContain(t, msg.HTML, "<b>Acme", "<script>", "javascript:alert", `<a href="https://evil.example">`, "<img src=x")
	mustContain(t, msg.HTML, "Join &lt;b&gt;Acme&lt;/b&gt;", `href="#ZgotmplZ"`)
}

func TestAvatarMatchesTheWebApp(t *testing.T) {
	for id, color := range map[string]string{
		"0b9f6c1e-5d1a-4c55-9a43-3f7f1a2b8c11": "#f2994a",
		"7d4e2f60-1c2b-4a8e-b0a1-9e5c3d2f4a77": "#26b5ce",
		"":                                     "#95a2b3",
		"a":                                    "#5e6ad2",
	} {
		if got := AvatarColor(id); got != color {
			t.Errorf("%q: got %s, want %s", id, got, color)
		}
	}
	for name, initials := range map[string]string{"Ada Lovelace": "AL", "ngyekta": "NG", "  grace  brewster  hopper ": "GH", "": "?", "émile zola": "ÉZ"} {
		if got := Initials(name); got != initials {
			t.Errorf("%q: got %s, want %s", name, got, initials)
		}
	}
}

func TestParseConfig(t *testing.T) {
	config, err := ParseConfig("smtps://resend:re_123@smtp.resend.com", "Ricrac <notifications@ricrac.dev>")
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "smtp.resend.com" || config.Port != 465 || !config.ImplicitTLS || config.Username != "resend" ||
		config.Password != "re_123" || config.From.Address != "notifications@ricrac.dev" || config.From.Name != "Ricrac" {
		t.Fatalf("got %+v", config)
	}
	starttls, err := ParseConfig("smtp://u:p@mail.example.com:2525", "a@b.c")
	if err != nil || starttls.Port != 2525 || starttls.ImplicitTLS || starttls.Insecure {
		t.Fatalf("got %+v, %v", starttls, err)
	}
	off, err := ParseConfig("", "")
	if err != nil || off != nil {
		t.Fatal("no settings turn email off")
	}
	for _, bad := range [][2]string{{"smtps://x", ""}, {"http://x", "a@b.c"}, {"smtp://h", "not an address"}} {
		if _, err := ParseConfig(bad[0], bad[1]); err == nil {
			t.Errorf("%v should fail", bad)
		}
	}
}
