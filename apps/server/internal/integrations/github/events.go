package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// Webhook payloads, with the fields Ricrac reads.

const (
	StateOpen   = "open"
	StateDraft  = "draft"
	StateMerged = "merged"
	StateClosed = "closed"
)

// ValidSignature checks the X-Hub-Signature-256 header (sha256=<hex HMAC of the body>) in
// constant time.
func ValidSignature(secret, header string, body []byte) bool {
	given, ok := strings.CutPrefix(header, "sha256=")
	if !ok || secret == "" {
		return false
	}
	decoded, err := hex.DecodeString(given)
	if err != nil {
		return false
	}
	return hmac.Equal(decoded, mac(secret, body))
}

// Sign is the X-Hub-Signature-256 header GitHub sends with body.
func Sign(secret string, body []byte) string {
	return "sha256=" + hex.EncodeToString(mac(secret, body))
}

func mac(secret string, body []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return h.Sum(nil)
}

type Account struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	HasIssues     bool   `json:"has_issues"`
}

type Installation struct {
	ID      int64   `json:"id"`
	Account Account `json:"account"`
}

// Envelope is what every event shares.
type Envelope struct {
	Action       string        `json:"action"`
	Installation *Installation `json:"installation"`
	Repository   *Repository   `json:"repository"`
	Sender       Account       `json:"sender"`
}

// Kind is the event and its action, like pull_request.opened.
func (e Envelope) Kind(event string) string {
	if e.Action == "" {
		return event
	}
	return event + "." + e.Action
}

type InstallationRepositoriesEvent struct {
	RepositoriesRemoved []Repository `json:"repositories_removed"`
}

type PullRequest struct {
	Number  int     `json:"number"`
	Title   string  `json:"title"`
	Body    string  `json:"body"`
	HTMLURL string  `json:"html_url"`
	State   string  `json:"state"`
	Draft   bool    `json:"draft"`
	Merged  bool    `json:"merged"`
	User    Account `json:"user"`
	Head    struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Additions    int        `json:"additions"`
	Deletions    int        `json:"deletions"`
	ChangedFiles int        `json:"changed_files"`
	MergedAt     *time.Time `json:"merged_at"`
}

const maxSummaryLength = 4000

// Summary is the body cut to 4000 characters, enough to know what it does without GitHub.
func (p PullRequest) Summary() string {
	runes := []rune(strings.TrimSpace(p.Body))
	if len(runes) <= maxSummaryLength {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:maxSummaryLength])) + "…"
}

// DisplayState is open, draft, merged or closed.
func (p PullRequest) DisplayState() string {
	switch {
	case p.Merged:
		return StateMerged
	case p.State == "closed":
		return StateClosed
	case p.Draft:
		return StateDraft
	}
	return StateOpen
}

type PullRequestEvent struct {
	Action      string      `json:"action"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
}

type PushEvent struct {
	Ref        string       `json:"ref"`
	After      string       `json:"after"`
	Deleted    bool         `json:"deleted"`
	Repository Repository   `json:"repository"`
	Sender     Account      `json:"sender"`
	Commits    []PushCommit `json:"commits"`
}

// Branch is the branch pushed to; false for tags and deleted branches.
func (p PushEvent) Branch() (string, bool) {
	branch, ok := strings.CutPrefix(p.Ref, "refs/heads/")
	if !ok || branch == "" || p.Deleted {
		return "", false
	}
	return branch, true
}

func (p PushEvent) OnDefaultBranch() bool {
	branch, ok := p.Branch()
	return ok && p.Repository.DefaultBranch != "" && branch == p.Repository.DefaultBranch
}

const maxSubjectLength = 72

type PushCommit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Author  struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
}

func (c PushCommit) ShortSHA() string {
	return c.ID[:min(len(c.ID), 7)]
}

// Subject is the message's first line, at most 72 characters.
func (c PushCommit) Subject() string {
	subject, _, _ := strings.Cut(c.Message, "\n")
	runes := []rune(strings.TrimSpace(subject))
	return strings.TrimSpace(string(runes[:min(len(runes), maxSubjectLength)]))
}

func (c PushCommit) AuthorName() string {
	if c.Author.Username != "" {
		return c.Author.Username
	}
	return c.Author.Name
}

func (c PushCommit) HTMLURL(repository Repository) string {
	if c.URL != "" {
		return c.URL
	}
	return strings.TrimRight(repository.HTMLURL, "/") + "/commit/" + c.ID
}

func BranchURL(repository Repository, branch string) string {
	return strings.TrimRight(repository.HTMLURL, "/") + "/tree/" + branch
}

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Issue struct {
	ID          int64           `json:"id"`
	NodeID      string          `json:"node_id"`
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	State       string          `json:"state"`
	StateReason string          `json:"state_reason"`
	HTMLURL     string          `json:"html_url"`
	User        Account         `json:"user"`
	Labels      []Label         `json:"labels"`
	Assignee    *Account        `json:"assignee"`
	Comments    int             `json:"comments"`
	Milestone   *Milestone      `json:"milestone"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	PullRequest json.RawMessage `json:"pull_request,omitempty"`
}

func (i Issue) IsPullRequest() bool {
	return len(i.PullRequest) > 0 && string(i.PullRequest) != "null"
}

// Closure is whether an issue is open, or closed as completed, not planned or a duplicate.
type Closure int

const (
	Open Closure = iota
	ClosedCompleted
	ClosedNotPlanned
	ClosedDuplicate
)

func (i Issue) Closure() Closure {
	if i.State != StateClosed {
		return Open
	}
	switch i.StateReason {
	case "not_planned":
		return ClosedNotPlanned
	case "duplicate":
		return ClosedDuplicate
	}
	return ClosedCompleted
}

type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	User      Account   `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IssuesEvent struct {
	Action     string     `json:"action"`
	Issue      Issue      `json:"issue"`
	Label      *Label     `json:"label"`
	Assignee   *Account   `json:"assignee"`
	Repository Repository `json:"repository"`
	Sender     Account    `json:"sender"`
}

type Milestone struct {
	ID          int64      `json:"id"`
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	State       string     `json:"state"`
	DueOn       *time.Time `json:"due_on"`
	HTMLURL     string     `json:"html_url"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// DueDate is the due date as YYYY-MM-DD, nil when it has none.
func (m Milestone) DueDate() *string {
	if m.DueOn == nil {
		return nil
	}
	date := m.DueOn.UTC().Format(time.DateOnly)
	return &date
}

func (m Milestone) Closed() bool {
	return m.State == StateClosed
}

// DueOn is a YYYY-MM-DD date as a milestone's due_on, nil for none.
func DueOn(date *string) any {
	if date == nil {
		return nil
	}
	// Noon UTC keeps the calendar day in any timezone within 11 hours of UTC.
	return *date + "T12:00:00Z"
}

type MilestoneEvent struct {
	Action     string     `json:"action"`
	Milestone  Milestone  `json:"milestone"`
	Repository Repository `json:"repository"`
	Sender     Account    `json:"sender"`
}

type IssueCommentEvent struct {
	Action     string     `json:"action"`
	Issue      Issue      `json:"issue"`
	Comment    Comment    `json:"comment"`
	Repository Repository `json:"repository"`
	Sender     Account    `json:"sender"`
}

var loginPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9]|-[a-z0-9]){0,38}$`)

// NormalizeLogin is a GitHub login in lowercase, false when it can't be one.
func NormalizeLogin(raw string) (string, bool) {
	login := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "@"))
	return login, loginPattern.MatchString(login)
}

var fromRicrac = regexp.MustCompile(`^(\*\*[^*\n]+ on Ricrac\*\*\n|Tracked in Ricrac as \[)`)

// RicracCommentBody is how a Ricrac comment reads on GitHub, where the app posts it.
func RicracCommentBody(author, body string) string {
	return "**" + strings.ReplaceAll(author, "*", "") + " on Ricrac**\n\n" + body
}

// FromRicrac says whether a GitHub comment is one the app posted for Ricrac.
func FromRicrac(body string) bool {
	return fromRicrac.MatchString(body)
}

// IssueField is a field an organization's issues have.
type IssueField struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Options  []struct {
		Name string `json:"name"`
	} `json:"options"`
}

// HasOption says whether a select field offers an option of this name.
func (f IssueField) HasOption(name string) bool {
	for _, option := range f.Options {
		if strings.EqualFold(option.Name, name) {
			return true
		}
	}
	return false
}

// RicracField is the text field named Ricrac, which holds the link to an issue's Ricrac issue.
func RicracField(fields []IssueField) *IssueField {
	for i, field := range fields {
		if field.DataType == "text" && strings.EqualFold(strings.TrimSpace(field.Name), "Ricrac") {
			return &fields[i]
		}
	}
	return nil
}

// LinkBackBody is the comment the app leaves on a GitHub issue it imported.
func LinkBackBody(identifier, url string) string {
	return "Tracked in Ricrac as [" + identifier + "](" + url + ")."
}

var hexColor = regexp.MustCompile(`^[0-9a-f]{6}$`)

// RicracColor is a GitHub label color (d73a4a) as Ricrac stores it (#d73a4a).
func RicracColor(color string) string {
	color = strings.ToLower(strings.TrimPrefix(color, "#"))
	if !hexColor.MatchString(color) {
		color = "ededed"
	}
	return "#" + color
}
