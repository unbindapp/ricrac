package models

import (
	"ricrac/ent"
	"ricrac/internal/common/utils"
)

// ProjectIssueStats is what project_issue_stats adds to projects and milestones.
type ProjectIssueStats struct {
	IssueCount          int     `json:"issueCount"`
	CompletedIssueCount int     `json:"completedIssueCount"`
	CanceledIssueCount  int     `json:"canceledIssueCount"`
	Progress            float64 `json:"progress" doc:"Completed issues / (all - canceled), from 0 to 1."`
}

type Project struct {
	ID                   string            `json:"id"`
	SlugID               string            `json:"slugId"`
	Name                 string            `json:"name"`
	Summary              *string           `json:"summary"`
	Description          Text              `json:"description,omitzero" doc:"Markdown; in lists only with includeDescription."`
	URL                  string            `json:"url"`
	Icon                 *string           `json:"icon"`
	Color                *string           `json:"color"`
	Status               ProjectStatusRef  `json:"status"`
	Priority             int               `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	PriorityLabel        string            `json:"priorityLabel"`
	Health               *string           `json:"health" enum:"onTrack,atRisk,offTrack"`
	Lead                 *UserRef          `json:"lead"`
	LeadTeam             *TeamRef          `json:"leadTeam"`
	Teams                []TeamRef         `json:"teams"`
	Labels               []ProjectLabelRef `json:"labels"`
	Initiatives          []InitiativeRef   `json:"initiatives"`
	StartDate            *string           `json:"startDate"`
	StartDateResolution  *string           `json:"startDateResolution" enum:"month,quarter,halfYear,year"`
	TargetDate           *string           `json:"targetDate"`
	TargetDateResolution *string           `json:"targetDateResolution" enum:"month,quarter,halfYear,year"`
	SortOrder            float64           `json:"sortOrder"`
	StartedAt            *string           `json:"startedAt"`
	CompletedAt          *string           `json:"completedAt"`
	CanceledAt           *string           `json:"canceledAt"`
	ArchivedAt           *string           `json:"archivedAt"`
	Trashed              bool              `json:"trashed"`
	TrashedAt            *string           `json:"trashedAt"`
	CreatedAt            string            `json:"createdAt"`
	UpdatedAt            string            `json:"updatedAt"`
	ProjectIssueStats
}

type ProjectDetail struct {
	Project
	Creator               *UserRef         `json:"creator"`
	Members               []UserRef        `json:"members"`
	Subscribers           []UserRef        `json:"subscribers"`
	Links                 []EntityLink     `json:"links"`
	Relations             ProjectRelations `json:"relations"`
	Documents             []DocumentRef    `json:"documents"`
	IssueCountByStateType map[string]int   `json:"issueCountByStateType" doc:"Issue counts by status type; types without issues are left out."`
	Estimate              ProjectEstimate  `json:"estimate"`
	Milestones            []Milestone      `json:"milestones"`
	LatestUpdate          *StatusUpdate    `json:"latestUpdate"`
}

// EntityLink is a link (resource) on a project or initiative.
type EntityLink struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Label     string   `json:"label"`
	Creator   *UserRef `json:"creator"`
	CreatedAt string   `json:"createdAt"`
}

type ProjectRelations struct {
	Blocks    []ProjectRef `json:"blocks"`
	BlockedBy []ProjectRef `json:"blockedBy"`
	Related   []ProjectRef `json:"related"`
}

type ProjectEstimate struct {
	Total     float64 `json:"total" doc:"The sum of the estimates of the project's issues, canceled ones left out."`
	Completed float64 `json:"completed"`
}

type ProjectStatus struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type" enum:"backlog,planned,started,paused,completed,canceled"`
	Color        string  `json:"color"`
	Description  *string `json:"description"`
	Position     float64 `json:"position"`
	ProjectCount int     `json:"projectCount"`
	ArchivedAt   *string `json:"archivedAt"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type DeletedProjectStatus struct {
	Deleted       bool   `json:"deleted"`
	ID            string `json:"id"`
	MovedProjects int    `json:"movedProjects"`
}

type ProjectLabel struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Color        string             `json:"color"`
	Description  *string            `json:"description"`
	IsGroup      bool               `json:"isGroup"`
	Parent       *ProjectLabelGroup `json:"parent"`
	ProjectCount int                `json:"projectCount"`
	Creator      *UserRef           `json:"creator"`
	RetiredAt    *string            `json:"retiredAt"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
}

type ProjectLabelGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeletedProjectLabel struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// ProjectOf needs the project's workspace, status, lead, lead team, teams (by key), labels (by
// name, with their parents) and initiatives not in the trash (by name) loaded.
func ProjectOf(p *ent.Project, publicURL string, stats ProjectIssueStats, withDescription bool) Project {
	e := p.Edges
	teams := make([]TeamRef, len(e.Teams))
	for i, t := range e.Teams {
		teams[i] = *TeamRefOf(t)
	}
	labels := make([]ProjectLabelRef, len(e.Labels))
	for i, l := range e.Labels {
		labels[i] = ProjectLabelRefOf(l)
	}
	initiatives := make([]InitiativeRef, len(e.Initiatives))
	for i, initiative := range e.Initiatives {
		initiatives[i] = *InitiativeRefOf(initiative)
	}
	project := Project{
		ID: ID(p.ID), SlugID: p.SlugID, Name: p.Name, Summary: p.Summary,
		URL:  utils.EntityURL(publicURL, e.Workspace.URLKey, "project", p.Name, p.SlugID),
		Icon: p.Icon, Color: p.Color, Status: *ProjectStatusRefOf(e.Status), Priority: p.Priority,
		Health: p.Health, Lead: UserRefOf(e.Lead), LeadTeam: TeamRefOf(e.LeadTeam), Teams: teams, Labels: labels,
		Initiatives: initiatives, StartDate: DatePtr(p.StartDate), StartDateResolution: p.StartDateResolution,
		TargetDate: DatePtr(p.TargetDate), TargetDateResolution: p.TargetDateResolution, SortOrder: p.SortOrder,
		StartedAt: TimePtr(p.StartedAt), CompletedAt: TimePtr(p.CompletedAt), CanceledAt: TimePtr(p.CanceledAt),
		ArchivedAt: TimePtr(p.ArchivedAt), Trashed: p.TrashedAt != nil, TrashedAt: TimePtr(p.TrashedAt),
		CreatedAt: Time(p.CreatedAt), UpdatedAt: Time(p.UpdatedAt), ProjectIssueStats: stats,
	}
	if p.Priority >= 0 && p.Priority < len(utils.PriorityNames) {
		project.PriorityLabel = utils.PriorityNames[p.Priority]
	}
	if withDescription {
		project.Description = TextOf(p.Content)
	}
	return project
}

// EntityLinkOf needs the link's creator loaded.
func EntityLinkOf(l *ent.EntityLink) EntityLink {
	return EntityLink{ID: ID(l.ID), URL: l.URL, Label: l.Label, Creator: UserRefOf(l.Edges.Creator), CreatedAt: Time(l.CreatedAt)}
}

func ProjectStatusOf(s *ent.ProjectStatus, projectCount int) ProjectStatus {
	return ProjectStatus{
		ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color, Description: s.Description, Position: s.Position,
		ProjectCount: projectCount, ArchivedAt: TimePtr(s.ArchivedAt), CreatedAt: Time(s.CreatedAt), UpdatedAt: Time(s.UpdatedAt),
	}
}

// ProjectLabelOf needs the label's parent and creator loaded.
func ProjectLabelOf(l *ent.ProjectLabel, projectCount int) ProjectLabel {
	label := ProjectLabel{
		ID: ID(l.ID), Name: l.Name, Color: l.Color, Description: l.Description, IsGroup: l.IsGroup,
		ProjectCount: projectCount, Creator: UserRefOf(l.Edges.Creator), RetiredAt: TimePtr(l.RetiredAt),
		CreatedAt: Time(l.CreatedAt), UpdatedAt: Time(l.UpdatedAt),
	}
	if l.Edges.Parent != nil {
		label.Parent = &ProjectLabelGroup{ID: ID(l.Edges.Parent.ID), Name: l.Edges.Parent.Name}
	}
	return label
}
