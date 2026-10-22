package email_service

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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
	Excerpt     string
	CreatedAt   time.Time
}

// noticeOf needs the notification's user, workspace, actor, comment, status update, issue with its
// team, project, initiative, document, discussion and discussion thread with its discussion loaded.
func noticeOf(n *ent.Notification, publicURL string) notice {
	w := n.Edges.Workspace
	found := notice{
		ID: n.ID, UserID: n.UserID, Email: n.Edges.User.Email, Kind: n.Type, Summary: n.Summary, ActorID: n.ActorID,
		WorkspaceID: w.ID, Workspace: w.Name, URLKey: w.URLKey, CreatedAt: n.CreatedAt,
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
	case e.DiscussionThread != nil:
		discussion := e.DiscussionThread.Edges.Discussion
		link := models.DiscussionThreadURL(models.DiscussionURL(publicURL, w.URLKey, discussion.Title, discussion.SlugID), models.ID(e.DiscussionThread.ID))
		found.Subject, found.Title, found.URL = &e.DiscussionThread.ID, &e.DiscussionThread.Title, &link
	case e.Discussion != nil:
		link := models.DiscussionURL(publicURL, w.URLKey, e.Discussion.Title, e.Discussion.SlugID)
		found.Subject, found.Title, found.URL = &e.Discussion.ID, &e.Discussion.Title, &link
	}
	switch e := n.Edges; {
	case e.Comment != nil:
		found.Excerpt = excerpt(e.Comment.Body)
	case e.StatusUpdate != nil:
		found.Excerpt = excerpt(e.StatusUpdate.Body)
	case e.DiscussionThread != nil && n.Type == "discussionNewThread":
		found.Excerpt = excerpt(e.DiscussionThread.Body)
	}
	return found
}

var (
	markdownImage  = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	markdownLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	markdownMarks  = regexp.MustCompile("(?m)^ {0,3}(#{1,6} +|> ?|```.*$)|\\*\\*|__|~~|`")
	repeatedBlanks = regexp.MustCompile(`\n{3,}`)
)

// excerpt is the start of Markdown as plain text.
func excerpt(markdown string) string {
	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	text = markdownImage.ReplaceAllStringFunc(text, func(image string) string {
		alt := markdownImage.FindStringSubmatch(image)[1]
		if alt == "" {
			alt = "image"
		}
		return "[" + alt + "]"
	})
	text = markdownLink.ReplaceAllString(text, "$1")
	text = markdownMarks.ReplaceAllString(text, "")
	text = repeatedBlanks.ReplaceAllString(strings.TrimSpace(text), "\n\n")
	runes := []rune(text)
	if len(runes) <= excerptLength {
		return text
	}
	return strings.TrimSpace(string(runes[:excerptLength])) + "…"
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
	subject := strings.TrimSpace(items[0].Identifier+" "+items[0].Title) + ": " + lineSentence(items[0].Lines[0])
	if count > 1 {
		subject += fmt.Sprintf(" (+%d more)", count-1)
	}
	content := mail.Content{
		Subject:   subject,
		Preheader: preheader(items[0].Lines[0]),
		Heading:   heading + ".",
		Items:     items,
		Footer: mail.Footer{
			InstanceURL: publicURL,
			ManageURL:   publicURL + "/settings/profile",
			ManageLabel: "Notification settings",
		},
	}
	if count > digestMaxItems {
		content.Button = mail.Button{Label: fmt.Sprintf("See all %d in your inbox", count), URL: inbox}
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
	"discussionNewThread":     "started a thread",
	"discussionMention":       "mentioned you",
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
		return mail.Line{Text: text, Excerpt: n.Excerpt}
	}
	line := mail.ActorLine(n.ActorID.String(), *n.ActorName, sentence)
	line.Excerpt = n.Excerpt
	return line
}

func lineSentence(line mail.Line) string {
	return strings.TrimSpace(line.ActorName + " " + line.Text)
}

func preheader(line mail.Line) string {
	if line.Excerpt == "" {
		return lineSentence(line)
	}
	return lineSentence(line) + ": " + line.Excerpt
}

func upperFirst(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
