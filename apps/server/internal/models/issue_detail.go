package models

import (
	"ricrac/ent"
	"ricrac/internal/common/utils"
)

// CommentThreads is an issue's comments, shown only when asked for.
type CommentThreads []CommentThread

func (c CommentThreads) IsZero() bool { return c == nil }

// HistoryEntries is an issue's history, shown only when asked for.
type HistoryEntries []HistoryEntry

func (h HistoryEntries) IsZero() bool { return h == nil }

// IssueChildOf needs the sub-issue's team, state and assignee loaded.
func IssueChildOf(i *ent.Issue) IssueChild {
	return IssueChild{
		ID: ID(i.ID), Identifier: Identifier(i.Edges.Team.Key, i.Number), Title: i.Title, State: *StateRefOf(i.Edges.State),
		Assignee: UserRefOf(i.Edges.Assignee), Priority: i.Priority,
	}
}

// IssueAttachmentOf needs the attachment's creator loaded.
func IssueAttachmentOf(a *ent.Attachment) IssueAttachment {
	return IssueAttachment{
		ID: ID(a.ID), Title: a.Title, Subtitle: a.Subtitle, URL: a.URL, SourceType: utils.OrEmpty(a.SourceType),
		Metadata: Object(a.Metadata), Creator: UserRefOf(a.Edges.Creator), CreatedAt: Time(a.CreatedAt),
	}
}

// HistoryEntryOf needs the entry's actor loaded.
func HistoryEntryOf(h *ent.IssueHistory) HistoryEntry {
	return HistoryEntry{Action: h.Action, Actor: UserRefOf(h.Edges.Actor), Changes: Object(h.Changes), CreatedAt: Time(h.CreatedAt)}
}

type ImportedIssues struct {
	DryRun         bool     `json:"dryRun" doc:"Nothing was imported, only checked."`
	Imported       int      `json:"imported" doc:"How many issues."`
	Comments       int      `json:"comments" doc:"How many comments."`
	Relations      int      `json:"relations" doc:"How many relations (relatedTo, blocks, blockedBy, duplicateOf)."`
	Identifiers    []string `json:"identifiers"`
	NextIdentifier string   `json:"nextIdentifier" doc:"The identifier the team's next issue gets."`
}
