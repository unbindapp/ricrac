package models

import (
	"ricrac/ent"
)

type ReleasePipeline struct {
	ID           string        `json:"id"`
	SlugID       string        `json:"slugId"`
	Name         string        `json:"name"`
	Type         string        `json:"type" enum:"continuous,scheduled"`
	IsProduction bool          `json:"isProduction"`
	ReleaseCount int           `json:"releaseCount" doc:"Releases in it, archived ones left out."`
	ArchivedAt   *string       `json:"archivedAt"`
	CreatedAt    string        `json:"createdAt"`
	UpdatedAt    string        `json:"updatedAt"`
	Stages       ReleaseStages `json:"stages,omitzero" doc:"In order; in lists only with includeStages."`
	Teams        PipelineTeams `json:"teams,omitzero" doc:"In lists only with includeTeams."`
}

// ReleaseStages are shown only when asked for, even when there are none.
type ReleaseStages []ReleaseStage

func (s ReleaseStages) IsZero() bool { return s == nil }

// PipelineTeams are shown only when asked for, even when there are none.
type PipelineTeams []TeamRef

func (t PipelineTeams) IsZero() bool { return t == nil }

type ReleaseStage struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type" enum:"planned,started,completed,canceled"`
	Color    string  `json:"color"`
	Position float64 `json:"position"`
}

type ReleaseStageDetail struct {
	ReleaseStage
	Pipeline ReleasePipelineRef `json:"pipeline"`
}

type DeletedReleaseStage struct {
	ID            string `json:"id"`
	Deleted       bool   `json:"deleted"`
	ReleasesMoved int    `json:"releasesMoved" doc:"Releases moved to moveReleasesTo."`
}

type Release struct {
	ID          string             `json:"id"`
	SlugID      string             `json:"slugId"`
	Name        string             `json:"name"`
	Version     *string            `json:"version"`
	Description *string            `json:"description" doc:"Markdown."`
	CommitSHA   *string            `json:"commitSha"`
	Pipeline    ReleasePipelineRef `json:"pipeline"`
	Stage       *ReleaseStageRef   `json:"stage"`
	StartDate   *string            `json:"startDate"`
	TargetDate  *string            `json:"targetDate"`
	StartedAt   *string            `json:"startedAt"`
	CompletedAt *string            `json:"completedAt"`
	CanceledAt  *string            `json:"canceledAt"`
	IssueCount  int                `json:"issueCount"`
	Creator     *UserRef           `json:"creator"`
	ArchivedAt  *string            `json:"archivedAt"`
	CreatedAt   string             `json:"createdAt"`
	UpdatedAt   string             `json:"updatedAt"`
}

type ReleaseDetail struct {
	Release
	Issues       []ReleaseIssue       `json:"issues" doc:"The issues in it the caller can see."`
	ReleaseNotes []ReleaseNoteSummary `json:"releaseNotes" doc:"Newest first."`
}

type ReleaseIssue struct {
	IssueRef
	Assignee *UserRef `json:"assignee"`
}

type ReleaseNoteSummary struct {
	ID        string `json:"id"`
	SlugID    string `json:"slugId"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

type ReleaseNote struct {
	ID        string             `json:"id"`
	SlugID    string             `json:"slugId"`
	Title     string             `json:"title"`
	Content   Text               `json:"content,omitzero" doc:"Markdown; not in lists."`
	Pipeline  ReleasePipelineRef `json:"pipeline"`
	Releases  []ReleaseRef       `json:"releases"`
	Creator   *UserRef           `json:"creator"`
	CreatedAt string             `json:"createdAt"`
	UpdatedAt string             `json:"updatedAt"`
}

type DeletedReleaseNote struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func ReleaseStageOf(s *ent.ReleaseStage) ReleaseStage {
	return ReleaseStage{ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color, Position: s.Position}
}

// ReleasePipelineOf shows the stages and teams loaded on the pipeline, when asked for.
func ReleasePipelineOf(p *ent.ReleasePipeline, releaseCount int, withStages, withTeams bool) ReleasePipeline {
	pipeline := ReleasePipeline{
		ID: ID(p.ID), SlugID: p.SlugID, Name: p.Name, Type: p.Type, IsProduction: p.IsProduction,
		ReleaseCount: releaseCount, ArchivedAt: TimePtr(p.ArchivedAt), CreatedAt: Time(p.CreatedAt), UpdatedAt: Time(p.UpdatedAt),
	}
	if withStages {
		pipeline.Stages = ReleaseStages{}
		for _, s := range p.Edges.ReleaseStages {
			pipeline.Stages = append(pipeline.Stages, ReleaseStageOf(s))
		}
	}
	if withTeams {
		pipeline.Teams = PipelineTeams{}
		for _, t := range p.Edges.Teams {
			pipeline.Teams = append(pipeline.Teams, *TeamRefOf(t))
		}
	}
	return pipeline
}

// ReleaseOf needs the release's pipeline, stage and creator loaded.
func ReleaseOf(r *ent.Release, issueCount int) Release {
	return Release{
		ID: ID(r.ID), SlugID: r.SlugID, Name: r.Name, Version: r.Version, Description: r.Description, CommitSHA: r.CommitSha,
		Pipeline: *ReleasePipelineRefOf(r.Edges.Pipeline), Stage: ReleaseStageRefOf(r.Edges.Stage),
		StartDate: DatePtr(r.StartDate), TargetDate: DatePtr(r.TargetDate), StartedAt: TimePtr(r.StartedAt),
		CompletedAt: TimePtr(r.CompletedAt), CanceledAt: TimePtr(r.CanceledAt), IssueCount: issueCount,
		Creator: UserRefOf(r.Edges.Creator), ArchivedAt: TimePtr(r.ArchivedAt), CreatedAt: Time(r.CreatedAt), UpdatedAt: Time(r.UpdatedAt),
	}
}

// ReleaseIssueOf needs the issue's team, state and assignee loaded.
func ReleaseIssueOf(i *ent.Issue) ReleaseIssue {
	return ReleaseIssue{IssueRef: *IssueRefOf(i), Assignee: UserRefOf(i.Edges.Assignee)}
}

func ReleaseNoteSummaryOf(n *ent.ReleaseNote) ReleaseNoteSummary {
	return ReleaseNoteSummary{ID: ID(n.ID), SlugID: n.SlugID, Title: n.Title, CreatedAt: Time(n.CreatedAt)}
}

// ReleaseNoteOf needs the note's pipeline and creator loaded; releases are the ones it covers,
// oldest first.
func ReleaseNoteOf(n *ent.ReleaseNote, releases []*ent.Release, withContent bool) ReleaseNote {
	note := ReleaseNote{
		ID: ID(n.ID), SlugID: n.SlugID, Title: n.Title, Pipeline: *ReleasePipelineRefOf(n.Edges.Pipeline),
		Releases: []ReleaseRef{}, Creator: UserRefOf(n.Edges.Creator), CreatedAt: Time(n.CreatedAt), UpdatedAt: Time(n.UpdatedAt),
	}
	if withContent {
		note.Content = TextOf(n.Content)
	}
	for _, r := range releases {
		note.Releases = append(note.Releases, *ReleaseRefOf(r))
	}
	return note
}
