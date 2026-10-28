// Package models is what operations answer. Every shape is the JSON the previous server built in
// SQL, field for field: its schema types the web client and is what MCP clients see. Timestamps
// and dates are ISO 8601 strings, formatted as Postgres formats them (see Time and Date).
package models

// M is a JSON object.
type M = map[string]any

type UserRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type TeamRef struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type StateRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type" enum:"triage,backlog,unstarted,started,completed,canceled"`
	Color string `json:"color"`
}

type IssueRef struct {
	ID         string   `json:"id"`
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	State      StateRef `json:"state"`
}

type LabelRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Color  string  `json:"color"`
	Parent *string `json:"parent" doc:"The name of its group."`
}

type ProjectRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type ProjectStatusRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type" enum:"backlog,planned,started,paused,completed,canceled"`
	Color string `json:"color"`
}

type ProjectLabelRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Color  string  `json:"color"`
	Parent *string `json:"parent"`
}

type MilestoneRef struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	TargetDate *string `json:"targetDate"`
}

type CycleRef struct {
	ID       string  `json:"id"`
	Number   int     `json:"number"`
	Name     *string `json:"name"`
	StartsAt string  `json:"startsAt"`
	EndsAt   string  `json:"endsAt"`
}

type InitiativeRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type DocumentRef struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	SlugID string `json:"slugId"`
}

type ReleaseRef struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Version *string `json:"version"`
	SlugID  string  `json:"slugId"`
}

type CustomerRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type CustomerStatusRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CustomerTierRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ReleasePipelineRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}

type ReleaseStageRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
}

// Deleted is what deleting something answers.
type Deleted struct {
	Deleted string `json:"deleted"`
}

// Ref is a thing named only by its id.
type Ref struct {
	ID string `json:"id"`
}

type Issue struct {
	ID                  string        `json:"id"`
	Identifier          string        `json:"identifier"`
	Number              int           `json:"number"`
	Title               string        `json:"title"`
	Description         Text          `json:"description,omitzero" doc:"Markdown; in lists only with includeDescription."`
	URL                 string        `json:"url"`
	BranchName          string        `json:"branchName"`
	Priority            int           `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	PriorityLabel       string        `json:"priorityLabel"`
	Estimate            *float64      `json:"estimate"`
	State               StateRef      `json:"state"`
	Team                TeamRef       `json:"team"`
	Assignee            *UserRef      `json:"assignee"`
	Creator             *UserRef      `json:"creator"`
	Labels              []LabelRef    `json:"labels"`
	Project             *ProjectRef   `json:"project"`
	ProjectMilestone    *MilestoneRef `json:"projectMilestone"`
	Cycle               *CycleRef     `json:"cycle"`
	Parent              *IssueRef     `json:"parent"`
	DueDate             *string       `json:"dueDate"`
	SortOrder           float64       `json:"sortOrder"`
	SubIssueSortOrder   *float64      `json:"subIssueSortOrder"`
	SubIssueCount       int           `json:"subIssueCount"`
	SLAType             *string       `json:"slaType"`
	SLAStartedAt        *string       `json:"slaStartedAt"`
	SLABreachesAt       *string       `json:"slaBreachesAt"`
	SnoozedUntil        *string       `json:"snoozedUntil"`
	StartedAt           *string       `json:"startedAt"`
	CompletedAt         *string       `json:"completedAt"`
	CanceledAt          *string       `json:"canceledAt"`
	StartedTriageAt     *string       `json:"startedTriageAt"`
	TriagedAt           *string       `json:"triagedAt"`
	AutoClosedAt        *string       `json:"autoClosedAt"`
	AutoArchivedAt      *string       `json:"autoArchivedAt"`
	ArchivedAt          *string       `json:"archivedAt"`
	Trashed             bool          `json:"trashed"`
	TrashedAt           *string       `json:"trashedAt"`
	PreviousIdentifiers []string      `json:"previousIdentifiers"`
	CreatedAt           string        `json:"createdAt"`
	UpdatedAt           string        `json:"updatedAt"`
}

type IssueDetail struct {
	Issue
	Children     []IssueChild      `json:"children"`
	Relations    IssueRelations    `json:"relations"`
	GitHub       *GitHubIssueRef   `json:"github" doc:"The GitHub issue it syncs with; null when none."`
	Attachments  []IssueAttachment `json:"attachments" doc:"GitHub pull requests are sourceType github, their metadata has repo, number, state, author, branch, base, summary (the body), additions, deletions, changed_files and merged_at."`
	Subscribers  []UserRef         `json:"subscribers"`
	Releases     []ReleaseRef      `json:"releases"`
	Documents    []DocumentRef     `json:"documents"`
	SharedWith   []UserRef         `json:"sharedWith"`
	Reactions    []IssueReaction   `json:"reactions"`
	CommentCount int               `json:"commentCount"`
	Comments     CommentThreads    `json:"comments,omitzero" doc:"With includeComments."`
	History      HistoryEntries    `json:"history,omitzero" doc:"With includeHistory."`
}

type IssueChild struct {
	ID         string   `json:"id"`
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	State      StateRef `json:"state"`
	Assignee   *UserRef `json:"assignee"`
	Priority   int      `json:"priority"`
}

type IssueRelations struct {
	Blocks      []IssueRef `json:"blocks"`
	BlockedBy   []IssueRef `json:"blockedBy"`
	Related     []IssueRef `json:"related"`
	DuplicateOf *IssueRef  `json:"duplicateOf"`
	Duplicates  []IssueRef `json:"duplicates"`
}

type GitHubIssueRef struct {
	Repository string `json:"repository" doc:"owner/name"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
}

type IssueAttachment struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Subtitle   *string  `json:"subtitle"`
	URL        string   `json:"url"`
	SourceType string   `json:"sourceType"`
	Metadata   M        `json:"metadata"`
	Creator    *UserRef `json:"creator"`
	CreatedAt  string   `json:"createdAt"`
}

type IssueReaction struct {
	Emoji string   `json:"emoji"`
	User  *UserRef `json:"user"`
}

type HistoryEntry struct {
	Action    string   `json:"action"`
	Actor     *UserRef `json:"actor"`
	Changes   M        `json:"changes" doc:"Changed fields, each {from, to}."`
	CreatedAt string   `json:"createdAt"`
}

type IssueHistory struct {
	Identifier string         `json:"identifier"`
	History    []HistoryEntry `json:"history"`
}

type UpdatedIssues struct {
	Updated []string `json:"updated" doc:"The identifiers of the changed issues."`
}

type IssueShare struct {
	Shared string `json:"shared"`
	With   string `json:"with"`
}

type IssueUnshare struct {
	Unshared string `json:"unshared"`
	With     string `json:"with"`
}

type Attachment struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	Subtitle   *string   `json:"subtitle"`
	SourceType string    `json:"sourceType"`
	Metadata   M         `json:"metadata"`
	IconURL    *string   `json:"iconUrl"`
	Creator    *UserRef  `json:"creator"`
	Issue      *IssueRef `json:"issue"`
	CreatedAt  string    `json:"createdAt"`
	UpdatedAt  string    `json:"updatedAt"`
}

type PreparedUpload struct {
	FileID    string `json:"fileId"`
	UploadURL string `json:"uploadUrl"`
	AssetURL  string `json:"assetUrl"`
	Markdown  string `json:"markdown"`
	ExpiresAt string `json:"expiresAt"`
	Curl      string `json:"curl"`
}

type UploadedFile struct {
	FileID      string `json:"fileId"`
	AssetURL    string `json:"assetUrl"`
	Markdown    string `json:"markdown"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

type CommentReaction struct {
	ID    string   `json:"id"`
	Emoji string   `json:"emoji"`
	User  *UserRef `json:"user"`
}

type Comment struct {
	ID               string            `json:"id"`
	Body             string            `json:"body"`
	User             *UserRef          `json:"user"`
	QuotedText       *string           `json:"quotedText"`
	SuggestedText    *string           `json:"suggestedText" doc:"On a document: the text suggested in place of quotedText."`
	SuggestionStatus *string           `json:"suggestionStatus" doc:"For a suggestion: pending, accepted or rejected."`
	Reactions        []CommentReaction `json:"reactions"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
	EditedAt         *string           `json:"editedAt"`
	DeletedAt        *string           `json:"deletedAt" doc:"When it was deleted; its body is then empty."`
	ResolvedAt       *string           `json:"resolvedAt"`
	ResolvingUser    *UserRef          `json:"resolvingUser"`
}

type CommentThread struct {
	Comment
	Replies []Comment `json:"replies"`
}

type CommentDetail struct {
	Comment
	ParentID         *string              `json:"parentId"`
	Issue            *IssueRef            `json:"issue"`
	Project          *ProjectRef          `json:"project"`
	Initiative       *InitiativeRef       `json:"initiative"`
	Document         *DocumentRef         `json:"document"`
	ProjectMilestone *MilestoneRef        `json:"projectMilestone"`
	StatusUpdate     *Ref                 `json:"statusUpdate"`
	Discussion       *DiscussionRef       `json:"discussion"`
	DiscussionThread *DiscussionThreadRef `json:"discussionThread"`
	URL              *string              `json:"url"`
}

type DeletedComment struct {
	Deleted        string `json:"deleted"`
	DeletedReplies int    `json:"deletedReplies"`
}

type Reaction struct {
	ID           string    `json:"id"`
	Emoji        string    `json:"emoji"`
	User         *UserRef  `json:"user"`
	Issue        *IssueRef `json:"issue"`
	Comment      *Ref      `json:"comment"`
	StatusUpdate *Ref      `json:"statusUpdate"`
	CreatedAt    string    `json:"createdAt"`
}

type RemovedReaction struct {
	Removed string `json:"removed"`
}
