package email_service

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/common/utils"
	"ricrac/internal/infrastructure/mail"
	"ricrac/internal/models"
)

// notice is a notification as a digest shows it.
type notice struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Email       string
	Kind        string
	Summary     string
	ActorID     *uuid.UUID
	ActorName   *string
	WorkspaceID uuid.UUID
	Workspace   string
	URLKey      string
	Subject     *uuid.UUID
	Identifier  *string
	Title       *string
	URL         *string
}

// noticeOf needs the notification's user, workspace, actor, issue with its team, project,
// initiative and document loaded.
func noticeOf(n *ent.Notification, publicURL string) notice {
	w := n.Edges.Workspace
	found := notice{
		ID: n.ID, UserID: n.UserID, Email: n.Edges.User.Email, Kind: n.Type, Summary: n.Summary, ActorID: n.ActorID,
		WorkspaceID: w.ID, Workspace: w.Name, URLKey: w.URLKey,
	}
	if a := n.Edges.Actor; a != nil {
		found.ActorName = &a.Name
	}
	base := publicURL + "/" + w.URLKey
	slugged := func(kind, name, slugID string) *string {
		link := base + "/" + kind + "/" + strings.Join(nonEmpty(utils.NameSlug(name), slugID), "-")
		return &link
	}
	switch e := n.Edges; {
	case e.Issue != nil:
		identifier := models.Identifier(e.Issue.Edges.Team.Key, e.Issue.Number)
		link := base + "/issue/" + identifier
		found.Subject, found.Identifier, found.Title, found.URL = &e.Issue.ID, &identifier, &e.Issue.Title, &link
	case e.Project != nil:
		found.Subject, found.Title, found.URL = &e.Project.ID, &e.Project.Name, slugged("project", e.Project.Name, e.Project.SlugID)
	case e.Initiative != nil:
		found.Subject, found.Title, found.URL = &e.Initiative.ID, &e.Initiative.Name, slugged("initiative", e.Initiative.Name, e.Initiative.SlugID)
	case e.Document != nil:
		found.Subject, found.Title, found.URL = &e.Document.ID, &e.Document.Title, slugged("document", e.Document.Title, e.Document.SlugID)
	}
	return found
}

func nonEmpty(parts ...string) []string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return kept
}

// digest is the email gathering a person's notifications, and its recipient.
func digest(batch []notice, publicURL string) (mail.Content, string) {
	count := len(batch)
	var items []mail.Item
	index := map[string]int{}
	for _, n := range batch[:min(count, digestMaxItems)] {
		key := n.ID.String()
		if n.Subject != nil {
			key = n.Subject.String()
		}
		position, seen := index[key]
		if !seen {
			position = len(items)
			index[key] = position
			items = append(items, digestItem(n))
		}
		items[position].Lines = append(items[position].Lines, digestLine(n))
	}
	heading := fmt.Sprintf("You have %d unread notifications", count)
	if count == 1 {
		heading = "You have an unread notification"
	}
	sameWorkspace := true
	for _, n := range batch {
		sameWorkspace = sameWorkspace && n.WorkspaceID == batch[0].WorkspaceID
	}
	inbox := publicURL
	if sameWorkspace {
		heading += " in " + batch[0].Workspace
		inbox = publicURL + "/" + batch[0].URLKey + "/inbox"
	}
	subject := fmt.Sprintf("%d unread notifications", count)
	if count == 1 {
		subject = strings.TrimSpace(items[0].Identifier+" "+items[0].Title) + ": " + lineSentence(items[0].Lines[0])
	}
	content := mail.Content{
		Subject:   subject,
		Preheader: lineSentence(items[0].Lines[0]),
		Heading:   heading + ".",
		Items:     items,
		Button:    mail.Button{Label: "Open your inbox", URL: inbox},
		Footer: mail.Footer{
			InstanceURL: publicURL,
			ManageURL:   publicURL + "/settings/profile",
			ManageLabel: "Notification settings",
		},
	}
	if count > digestMaxItems {
		content.Note = fmt.Sprintf("And %d more in your inbox.", count-digestMaxItems)
	}
	return content, batch[0].Email
}

func digestItem(n notice) mail.Item {
	item := mail.Item{Title: n.Summary, URL: utils.OrEmpty(n.URL)}
	if n.Title != nil {
		item.Title = *n.Title
	}
	if n.Identifier != nil {
		item.Identifier = *n.Identifier
	}
	return item
}

// actorSentences say what the actor of a notification did, by its kind.
var actorSentences = map[string]string{
	"issueAssignedToYou":      "assigned the issue to you",
	"issueUnassignedFromYou":  "unassigned you from the issue",
	"issueMention":            "mentioned you",
	"issueShared":             "shared the issue with you",
	"projectMention":          "mentioned you in the project",
	"projectAssigned":         "made you the project lead",
	"documentMention":         "mentioned you in the document",
	"projectUpdateCreated":    "posted an update",
	"projectUpdateMention":    "mentioned you in an update",
	"initiativeUpdateCreated": "posted an update",
}

func digestLine(n notice) mail.Line {
	sentence, known := actorSentences[n.Kind]
	switch {
	case strings.HasSuffix(n.Kind, "CommentMention"):
		sentence, known = "mentioned you in a comment", true
	case strings.HasSuffix(n.Kind, "CommentReply"):
		sentence, known = "replied to a thread you're in", true
	case strings.HasSuffix(n.Kind, "NewComment"):
		sentence, known = "commented", true
	case n.Kind == "issueStatusChanged":
		if _, state, found := strings.Cut(n.Summary, " moved to "); found {
			sentence, known = "moved the issue to "+state, true
		}
	}
	if !known || n.ActorID == nil || n.ActorName == nil {
		text := n.Summary
		if n.Identifier != nil {
			text = upperFirst(strings.TrimSpace(strings.TrimPrefix(text, *n.Identifier)))
		}
		return mail.Line{Text: text}
	}
	return mail.ActorLine(n.ActorID.String(), *n.ActorName, sentence)
}

func lineSentence(line mail.Line) string {
	return strings.TrimSpace(line.ActorName + " " + line.Text)
}

func upperFirst(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
