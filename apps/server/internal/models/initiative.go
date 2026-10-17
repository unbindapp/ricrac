package models

import (
	"ricrac/ent"
	"ricrac/internal/common/utils"
)

type Initiative struct {
	ID                   string         `json:"id"`
	SlugID               string         `json:"slugId"`
	Name                 string         `json:"name"`
	Summary              *string        `json:"summary"`
	Description          Text           `json:"description,omitzero" doc:"Markdown; in lists only with includeDescription."`
	URL                  string         `json:"url"`
	Icon                 *string        `json:"icon"`
	Color                *string        `json:"color"`
	Status               string         `json:"status" enum:"planned,active,completed"`
	Health               *string        `json:"health" enum:"onTrack,atRisk,offTrack"`
	Owner                *UserRef       `json:"owner"`
	Creator              *UserRef       `json:"creator"`
	TargetDate           *string        `json:"targetDate"`
	TargetDateResolution *string        `json:"targetDateResolution" enum:"month,quarter,halfYear,year"`
	Parent               *InitiativeRef `json:"parent"`
	ProjectCount         int            `json:"projectCount"`
	Progress             float64        `json:"progress" doc:"The average progress of its projects, from 0 to 1."`
	SortOrder            float64        `json:"sortOrder"`
	StartedAt            *string        `json:"startedAt"`
	CompletedAt          *string        `json:"completedAt"`
	ArchivedAt           *string        `json:"archivedAt"`
	Trashed              bool           `json:"trashed"`
	TrashedAt            *string        `json:"trashedAt"`
	CreatedAt            string         `json:"createdAt"`
	UpdatedAt            string         `json:"updatedAt"`
}

type InitiativeDetail struct {
	Initiative
	Projects       []InitiativeProject `json:"projects"`
	SubInitiatives []SubInitiative     `json:"subInitiatives"`
	Links          []EntityLink        `json:"links"`
	Documents      []DocumentRef       `json:"documents"`
	LatestUpdate   *StatusUpdate       `json:"latestUpdate"`
}

type InitiativeProject struct {
	ProjectRef
	Status     ProjectStatusRef `json:"status"`
	Health     *string          `json:"health" enum:"onTrack,atRisk,offTrack"`
	Lead       *UserRef         `json:"lead"`
	TargetDate *string          `json:"targetDate"`
	Progress   float64          `json:"progress"`
}

type SubInitiative struct {
	InitiativeRef
	Status   string   `json:"status" enum:"planned,active,completed"`
	Health   *string  `json:"health" enum:"onTrack,atRisk,offTrack"`
	Owner    *UserRef `json:"owner"`
	Progress float64  `json:"progress"`
}

// InitiativeOf needs the initiative's workspace, owner, creator and parent loaded.
func InitiativeOf(i *ent.Initiative, publicURL string, projectCount int, progress float64, withDescription bool) Initiative {
	e := i.Edges
	initiative := Initiative{
		ID: ID(i.ID), SlugID: i.SlugID, Name: i.Name, Summary: i.Summary,
		URL:  utils.EntityURL(publicURL, e.Workspace.URLKey, "initiative", i.Name, i.SlugID),
		Icon: i.Icon, Color: i.Color, Status: i.Status, Health: i.Health, Owner: UserRefOf(e.Owner),
		Creator: UserRefOf(e.Creator), TargetDate: DatePtr(i.TargetDate), TargetDateResolution: i.TargetDateResolution,
		Parent: InitiativeRefOf(e.Parent), ProjectCount: projectCount, Progress: progress, SortOrder: i.SortOrder,
		StartedAt: TimePtr(i.StartedAt), CompletedAt: TimePtr(i.CompletedAt), ArchivedAt: TimePtr(i.ArchivedAt),
		Trashed: i.TrashedAt != nil, TrashedAt: TimePtr(i.TrashedAt), CreatedAt: Time(i.CreatedAt), UpdatedAt: Time(i.UpdatedAt),
	}
	if withDescription {
		initiative.Description = TextOf(i.Content)
	}
	return initiative
}

// InitiativeProjectOf needs the project's status and lead loaded.
func InitiativeProjectOf(p *ent.Project, progress float64) InitiativeProject {
	return InitiativeProject{
		ProjectRef: *ProjectRefOf(p), Status: *ProjectStatusRefOf(p.Edges.Status), Health: p.Health,
		Lead: UserRefOf(p.Edges.Lead), TargetDate: DatePtr(p.TargetDate), Progress: progress,
	}
}

// SubInitiativeOf needs the initiative's owner loaded.
func SubInitiativeOf(i *ent.Initiative, progress float64) SubInitiative {
	return SubInitiative{
		InitiativeRef: *InitiativeRefOf(i), Status: i.Status, Health: i.Health, Owner: UserRefOf(i.Edges.Owner), Progress: progress,
	}
}
