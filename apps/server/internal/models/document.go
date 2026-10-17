package models

import (
	"ricrac/ent"
	"ricrac/internal/common/utils"
)

type Document struct {
	ID         string         `json:"id"`
	SlugID     string         `json:"slugId"`
	Title      string         `json:"title"`
	Content    Text           `json:"content,omitzero" doc:"Markdown; in lists only with includeContent."`
	URL        string         `json:"url"`
	Icon       *string        `json:"icon"`
	Color      *string        `json:"color"`
	Project    *ProjectRef    `json:"project"`
	Initiative *InitiativeRef `json:"initiative"`
	Team       *TeamRef       `json:"team"`
	Issue      *IssueRef      `json:"issue"`
	Cycle      *CycleRef      `json:"cycle"`
	Release    *ReleaseRef    `json:"release"`
	Creator    *UserRef       `json:"creator"`
	UpdatedBy  *UserRef       `json:"updatedBy"`
	Trashed    bool           `json:"trashed"`
	TrashedAt  *string        `json:"trashedAt"`
	CreatedAt  string         `json:"createdAt"`
	UpdatedAt  string         `json:"updatedAt"`
}

type DocumentDetail struct {
	Document
	VersionCount int `json:"versionCount"`
}

type DocumentVersionSummary struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Author    *UserRef `json:"author"`
	CreatedAt string   `json:"createdAt"`
}

type DocumentVersion struct {
	ID        int         `json:"id"`
	Document  DocumentRef `json:"document"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	Author    *UserRef    `json:"author"`
	CreatedAt string      `json:"createdAt"`
}

// DocumentURL is the document's page in the web app: its title's slug, if any, and its slug id.
func DocumentURL(publicURL, workspaceKey, title, slugID string) string {
	slug := slugID
	if words := utils.NameSlug(title); words != "" {
		slug = words + "-" + slugID
	}
	return publicURL + "/" + workspaceKey + "/document/" + slug
}

// DocumentOf needs the document's workspace, parents (an issue with its team and state),
// creator and last editor loaded.
func DocumentOf(d *ent.Document, publicURL string, withContent bool) Document {
	e := d.Edges
	document := Document{
		ID: ID(d.ID), SlugID: d.SlugID, Title: d.Title, URL: DocumentURL(publicURL, e.Workspace.URLKey, d.Title, d.SlugID),
		Icon: d.Icon, Color: d.Color, Project: ProjectRefOf(e.Project), Initiative: InitiativeRefOf(e.Initiative),
		Team: TeamRefOf(e.Team), Issue: IssueRefOf(e.Issue), Cycle: CycleRefOf(e.Cycle), Release: ReleaseRefOf(e.Release),
		Creator: UserRefOf(e.Creator), UpdatedBy: UserRefOf(e.UpdatedBy), Trashed: d.TrashedAt != nil,
		TrashedAt: TimePtr(d.TrashedAt), CreatedAt: Time(d.CreatedAt), UpdatedAt: Time(d.UpdatedAt),
	}
	if withContent {
		document.Content = TextOf(d.Content)
	}
	return document
}

// DocumentVersionSummaryOf needs the version's author loaded.
func DocumentVersionSummaryOf(v *ent.DocumentVersion) DocumentVersionSummary {
	return DocumentVersionSummary{ID: int(v.ID), Title: v.Title, Author: UserRefOf(v.Edges.Author), CreatedAt: Time(v.CreatedAt)}
}

// DocumentVersionOf needs the version's document and author loaded.
func DocumentVersionOf(v *ent.DocumentVersion) DocumentVersion {
	return DocumentVersion{
		ID: int(v.ID), Document: *DocumentRefOf(v.Edges.Document), Title: v.Title, Content: v.Content,
		Author: UserRefOf(v.Edges.Author), CreatedAt: Time(v.CreatedAt),
	}
}
