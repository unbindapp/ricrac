package email_service

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDigestGathersNotificationsPerSubject(t *testing.T) {
	user, workspace, issue, actor := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ann, identifier, title := "Ann Smith", "ENG-1", "Fix login"
	link := "http://localhost:3000/acme/issue/ENG-1"
	n := func(kind, summary string) notice {
		return notice{
			ID: uuid.New(), UserID: user, Email: "bob@acme.test", Kind: kind, Summary: summary, ActorID: &actor, ActorName: &ann,
			WorkspaceID: workspace, Workspace: "Acme", URLKey: "acme", Subject: &issue, Identifier: &identifier, Title: &title, URL: &link,
		}
	}
	content, to := digest([]notice{
		n("issueAssignedToYou", "ENG-1 was assigned to you"),
		n("issueStatusChanged", "ENG-1 moved to In Progress"),
	}, "http://localhost:3000")
	assert.Equal(t, "bob@acme.test", to)
	assert.Equal(t, "ENG-1 Fix login: Ann Smith assigned the issue to you (+1 more)", content.Subject)
	assert.Equal(t, "You have 2 unread notifications in Acme.", content.Heading)
	assert.Empty(t, content.Button.URL)
	assert.Len(t, content.Items, 1)
	assert.Equal(t, "ENG-1", content.Items[0].Identifier)
	assert.Equal(t, []string{"assigned the issue to you", "moved the issue to In Progress"},
		[]string{content.Items[0].Lines[0].Text, content.Items[0].Lines[1].Text})
}

func TestDigestOfOneNotificationNamesItInTheSubject(t *testing.T) {
	identifier, title := "ENG-2", "Ship it"
	content, _ := digest([]notice{{
		ID: uuid.New(), Kind: "issueDue", Summary: "ENG-2 is due today", WorkspaceID: uuid.New(), Workspace: "Acme", URLKey: "acme",
		Identifier: &identifier, Title: &title,
	}}, "http://localhost:3000")
	assert.Equal(t, "ENG-2 Ship it: Is due today", content.Subject)
	assert.Equal(t, "You have an unread notification in Acme.", content.Heading)
}

func TestDigestOfManyWorkspacesLinksTheInstance(t *testing.T) {
	var batch []notice
	for range digestMaxItems + 3 {
		batch = append(batch, notice{ID: uuid.New(), Kind: "issueDue", Summary: "due", WorkspaceID: uuid.New(), Workspace: "W"})
	}
	content, _ := digest(batch, "http://localhost:3000")
	assert.Equal(t, "You have 23 unread notifications.", content.Heading)
	assert.Equal(t, "http://localhost:3000", content.Button.URL)
	assert.Equal(t, "See all 23 in your inbox", content.Button.Label)
	assert.Len(t, content.Items, digestMaxItems)
}

func TestExcerptIsTheStartOfTheMarkdownAsText(t *testing.T) {
	assert.Equal(t, "Looks good, see the doc and [shot.png]\n\nShip it\n[image]",
		excerpt("## Looks **good**, see [the doc](http://x/doc) and ![shot.png](http://x/a.png)\r\n\n\n\n> Ship `it`\n![](http://x/b.png)"))
	long := excerpt(strings.Repeat("word ", 200))
	assert.Len(t, []rune(long), excerptLength)
	assert.True(t, strings.HasSuffix(long, "word…"))
}
