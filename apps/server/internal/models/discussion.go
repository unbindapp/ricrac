package models

import (
	"ricrac/ent"
	"ricrac/internal/common/utils"
)

type DiscussionRef struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	SlugID string `json:"slugId"`
}

type DiscussionThreadRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

type Discussion struct {
	ID              string   `json:"id"`
	SlugID          string   `json:"slugId"`
	Title           string   `json:"title"`
	Body            Text     `json:"body,omitzero" doc:"Markdown; in lists only with includeBody."`
	URL             string   `json:"url"`
	ThumbnailURL    *string  `json:"thumbnailUrl" doc:"The image the forum shows for it; null shows the first image in the body."`
	Tags            []string `json:"tags"`
	Team            *TeamRef `json:"team" doc:"The team it belongs to; only that team's members see it. Null for the whole workspace."`
	Creator         *UserRef `json:"creator"`
	PinnedAt        *string  `json:"pinnedAt"`
	ClosedAt        *string  `json:"closedAt"`
	Trashed         bool     `json:"trashed"`
	TrashedAt       *string  `json:"trashedAt"`
	ThreadCount     int      `json:"threadCount"`
	OpenThreadCount int      `json:"openThreadCount"`
	CommentCount    int      `json:"commentCount" doc:"Comments on the discussion itself, not in its threads."`
	LastActivityAt  string   `json:"lastActivityAt"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

type DiscussionDetail struct {
	Discussion
	Threads     []DiscussionThread `json:"threads" doc:"Oldest first."`
	Subscribers []UserRef          `json:"subscribers"`
}

type DiscussionTag struct {
	Name  string `json:"name"`
	Count int    `json:"count" doc:"How many discussions have it."`
}

type DiscussionThread struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind" doc:"bug, design, question, idea, task or general."`
	Title         string        `json:"title"`
	Body          string        `json:"body"`
	URL           string        `json:"url"`
	Discussion    DiscussionRef `json:"discussion"`
	Creator       *UserRef      `json:"creator"`
	Issue         *IssueRef     `json:"issue" doc:"The issue tracking it, if any."`
	ResolvedAt    *string       `json:"resolvedAt"`
	ResolvingUser *UserRef      `json:"resolvingUser"`
	CommentCount  int           `json:"commentCount"`
	CreatedAt     string        `json:"createdAt"`
	UpdatedAt     string        `json:"updatedAt"`
}

type DiscussionThreadDetail struct {
	DiscussionThread
	Comments []CommentThread `json:"comments" doc:"Oldest first, each with its replies."`
}

type DeletedDiscussionThread struct {
	Deleted string `json:"deleted"`
}

// DiscussionCounts is how many threads, open threads and comments a discussion has.
type DiscussionCounts struct {
	Threads, OpenThreads, Comments int
}

// DiscussionURL is the discussion's page in the web app: its title's slug, if any, and its slug
// id.
func DiscussionURL(publicURL, workspaceKey, title, slugID string) string {
	slug := slugID
	if words := utils.NameSlug(title); words != "" {
		slug = words + "-" + slugID
	}
	return publicURL + "/" + workspaceKey + "/discussion/" + slug
}

// DiscussionThreadURL opens the thread on its discussion's page.
func DiscussionThreadURL(discussionURL, thread string) string {
	return discussionURL + "?thread=" + thread
}

func DiscussionRefOf(d *ent.Discussion) *DiscussionRef {
	if d == nil {
		return nil
	}
	return &DiscussionRef{ID: ID(d.ID), Title: d.Title, SlugID: d.SlugID}
}

func DiscussionThreadRefOf(t *ent.DiscussionThread) *DiscussionThreadRef {
	if t == nil {
		return nil
	}
	return &DiscussionThreadRef{ID: ID(t.ID), Title: t.Title, Kind: t.Kind}
}

// DiscussionOf needs the discussion's workspace, team and creator loaded.
func DiscussionOf(d *ent.Discussion, publicURL string, counts DiscussionCounts, withBody bool) Discussion {
	e := d.Edges
	discussion := Discussion{
		ID: ID(d.ID), SlugID: d.SlugID, Title: d.Title, URL: DiscussionURL(publicURL, e.Workspace.URLKey, d.Title, d.SlugID),
		ThumbnailURL: d.ThumbnailURL, Tags: append([]string{}, d.Tags...), Team: TeamRefOf(e.Team), Creator: UserRefOf(e.Creator), PinnedAt: TimePtr(d.PinnedAt), ClosedAt: TimePtr(d.ClosedAt),
		Trashed: d.TrashedAt != nil, TrashedAt: TimePtr(d.TrashedAt), ThreadCount: counts.Threads,
		OpenThreadCount: counts.OpenThreads, CommentCount: counts.Comments, LastActivityAt: Time(d.LastActivityAt),
		CreatedAt: Time(d.CreatedAt), UpdatedAt: Time(d.UpdatedAt),
	}
	if withBody {
		discussion.Body = TextOf(d.Body)
	}
	return discussion
}

// DiscussionThreadOf needs the thread's discussion (with its workspace), creator, resolving user
// and issue (with its team and state) loaded.
func DiscussionThreadOf(t *ent.DiscussionThread, publicURL string, comments int) DiscussionThread {
	e := t.Edges
	discussion := e.Discussion
	return DiscussionThread{
		ID: ID(t.ID), Kind: t.Kind, Title: t.Title, Body: t.Body,
		URL:        DiscussionThreadURL(DiscussionURL(publicURL, discussion.Edges.Workspace.URLKey, discussion.Title, discussion.SlugID), ID(t.ID)),
		Discussion: *DiscussionRefOf(discussion), Creator: UserRefOf(e.Creator), Issue: IssueRefOf(e.Issue),
		ResolvedAt: TimePtr(t.ResolvedAt), ResolvingUser: UserRefOf(e.ResolvingUser), CommentCount: comments,
		CreatedAt: Time(t.CreatedAt), UpdatedAt: Time(t.UpdatedAt),
	}
}
