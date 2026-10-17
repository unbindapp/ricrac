package models

import (
	"encoding/json"
	"strconv"
	"time"

	"ricrac/ent"
)

type Cycle struct {
	ID          string          `json:"id"`
	Number      int             `json:"number"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	URL         string          `json:"url"`
	StartsAt    string          `json:"startsAt"`
	EndsAt      string          `json:"endsAt"`
	CompletedAt *string         `json:"completedAt"`
	ArchivedAt  *string         `json:"archivedAt"`
	IsActive    bool            `json:"isActive"`
	IsFuture    bool            `json:"isFuture"`
	IsPast      bool            `json:"isPast"`
	Team        TeamRef         `json:"team"`
	Progress    float64         `json:"progress" doc:"The share of its issues completed, 0 to 1."`
	Scope       CycleScope      `json:"scope"`
	History     []CycleSnapshot `json:"history" doc:"The counts of each day the cycle ran."`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

type CycleScope struct {
	Issues            int     `json:"issues"`
	Completed         int     `json:"completed"`
	Started           int     `json:"started"`
	Unstarted         int     `json:"unstarted"`
	Canceled          int     `json:"canceled"`
	Estimate          float64 `json:"estimate"`
	CompletedEstimate float64 `json:"completedEstimate"`
	StartedEstimate   float64 `json:"startedEstimate"`
}

type CycleSnapshot struct {
	Date              string  `json:"date"`
	Scope             int     `json:"scope"`
	Completed         int     `json:"completed"`
	Started           int     `json:"started"`
	ScopeEstimate     float64 `json:"scopeEstimate"`
	CompletedEstimate float64 `json:"completedEstimate"`
}

type CycleDetail struct {
	Cycle
	Issues CycleIssues `json:"issues,omitzero" doc:"Unless includeIssues is false."`
}

// CycleIssues is shown only when asked for, even empty.
type CycleIssues []CycleIssue

func (x CycleIssues) IsZero() bool { return x == nil }

type CycleIssue struct {
	ID         string   `json:"id"`
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	State      StateRef `json:"state"`
	Assignee   *UserRef `json:"assignee"`
	Estimate   *float64 `json:"estimate"`
	Priority   int      `json:"priority"`
}

// CycleURL is the cycle's page in the web app.
func CycleURL(publicURL, workspaceKey, teamKey string, number int) string {
	return publicURL + "/" + workspaceKey + "/team/" + teamKey + "/cycle/" + strconv.Itoa(number)
}

// CycleProgress is the share of completed issues rounded to 4 decimals, as Postgres' round does.
func CycleProgress(completed, issues int) float64 {
	if issues == 0 {
		return 0
	}
	return float64((completed*20000+issues)/(2*issues)) / 10000
}

// CycleOf needs the cycle's team and its workspace loaded; now is the time it is read at.
func CycleOf(c *ent.Cycle, publicURL string, scope CycleScope, now time.Time) Cycle {
	team := c.Edges.Team
	history := []CycleSnapshot{}
	if len(c.History) > 0 {
		_ = json.Unmarshal(c.History, &history)
	}
	return Cycle{
		ID: ID(c.ID), Number: c.Number, Name: c.Name, Description: c.Description,
		URL:      CycleURL(publicURL, team.Edges.Workspace.URLKey, team.Key, c.Number),
		StartsAt: Time(c.StartsAt), EndsAt: Time(c.EndsAt), CompletedAt: TimePtr(c.CompletedAt), ArchivedAt: TimePtr(c.ArchivedAt),
		IsActive: !c.StartsAt.After(now) && c.EndsAt.After(now), IsFuture: c.StartsAt.After(now), IsPast: !c.EndsAt.After(now),
		Team: *TeamRefOf(team), Progress: CycleProgress(scope.Completed, scope.Issues), Scope: scope, History: history,
		CreatedAt: Time(c.CreatedAt), UpdatedAt: Time(c.UpdatedAt),
	}
}

// CycleIssueOf needs the issue's team, state and assignee loaded.
func CycleIssueOf(i *ent.Issue) CycleIssue {
	return CycleIssue{
		ID: ID(i.ID), Identifier: Identifier(i.Edges.Team.Key, i.Number), Title: i.Title, State: *StateRefOf(i.Edges.State),
		Assignee: UserRefOf(i.Edges.Assignee), Estimate: i.Estimate, Priority: i.Priority,
	}
}
