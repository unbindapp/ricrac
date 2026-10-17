package models

import (
	"ricrac/ent"
)

type Team struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	Description   *string  `json:"description"`
	Icon          *string  `json:"icon"`
	Color         *string  `json:"color"`
	Private       bool     `json:"private"`
	Parent        *TeamRef `json:"parent"`
	URL           string   `json:"url"`
	IsMember      bool     `json:"isMember"`
	MemberCount   int      `json:"memberCount"`
	IssueCount    int      `json:"issueCount" doc:"Open issues: not completed, canceled, archived or trashed."`
	CyclesEnabled bool     `json:"cyclesEnabled"`
	TriageEnabled bool     `json:"triageEnabled"`
	Timezone      string   `json:"timezone"`
	ArchivedAt    *string  `json:"archivedAt"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

type TeamDetail struct {
	Team
	EstimateType                 string       `json:"estimateType" enum:"notUsed,exponential,fibonacci,linear,tShirt"`
	EstimateAllowZero            bool         `json:"estimateAllowZero"`
	EstimateExtended             bool         `json:"estimateExtended"`
	DefaultEstimate              *float64     `json:"defaultEstimate"`
	AllowedEstimates             []float64    `json:"allowedEstimates"`
	CycleDurationWeeks           int          `json:"cycleDurationWeeks"`
	CycleCooldownWeeks           int          `json:"cycleCooldownWeeks"`
	CycleStartDay                int          `json:"cycleStartDay" doc:"0 Sunday, 1 Monday ... 6 Saturday."`
	UpcomingCycleCount           int          `json:"upcomingCycleCount"`
	CycleAutoAddStarted          bool         `json:"cycleAutoAddStarted"`
	CycleAutoAddCompleted        bool         `json:"cycleAutoAddCompleted"`
	CycleRollover                bool         `json:"cycleRollover"`
	CurrentCycle                 *CycleRef    `json:"currentCycle"`
	TriageAction                 string       `json:"triageAction" enum:"none,notify,assign"`
	TriageResponsible            []UserRef    `json:"triageResponsible" doc:"Everyone responsible for triage, in rotation order."`
	TriageRotationWeeks          *int         `json:"triageRotationWeeks"`
	TriageRotationStart          *string      `json:"triageRotationStart"`
	RequirePriorityToLeaveTriage bool         `json:"requirePriorityToLeaveTriage"`
	Triage                       TeamTriage   `json:"triage"`
	AutoArchiveMonths            *int         `json:"autoArchiveMonths"`
	AutoCloseMonths              *int         `json:"autoCloseMonths"`
	AutoCloseState               *StateRef    `json:"autoCloseState"`
	AutoCloseParentIssues        bool         `json:"autoCloseParentIssues"`
	AutoCloseChildIssues         bool         `json:"autoCloseChildIssues"`
	DefaultIssueState            *StateRef    `json:"defaultIssueState"`
	GitHubPROpenedState          *StateRef    `json:"githubPrOpenedState" doc:"Where GitHub pull requests and branches move issues; null for the first started status."`
	GitHubPRMergedState          *StateRef    `json:"githubPrMergedState" doc:"Where merged pull requests that fix issues move them; null for the first completed status."`
	States                       []TeamStatus `json:"states" doc:"Its active statuses in board order."`
	Members                      []TeamMember `json:"members"`
	LabelCount                   int          `json:"labelCount"`
	SubTeams                     []TeamRef    `json:"subTeams"`
}

type TeamTriage struct {
	Enabled     bool      `json:"enabled"`
	Action      string    `json:"action" enum:"none,notify,assign"`
	Responsible []UserRef `json:"responsible" doc:"Who is responsible now: with a rotation, the person on duty."`
}

type TeamStatus struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type" enum:"triage,backlog,unstarted,started,completed,canceled"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
	Position    float64 `json:"position"`
}

type TeamMember struct {
	UserRef
	Owner bool `json:"owner"`
}

type DeletedTeam struct {
	Deleted string `json:"deleted"`
	Key     string `json:"key"`
}

type LeftTeam struct {
	Left TeamRef `json:"left"`
}

type TriageResponsibility struct {
	Team           TeamRef         `json:"team"`
	TriageEnabled  bool            `json:"triageEnabled"`
	Action         string          `json:"action" enum:"none,notify,assign"`
	Responsible    []UserRef       `json:"responsible" doc:"Who is responsible now: with a rotation, the person on duty."`
	Everyone       []UserRef       `json:"everyone" doc:"Everyone responsible for triage, in rotation order."`
	Rotation       *TriageRotation `json:"rotation"`
	UpcomingShifts []TriageShift   `json:"upcomingShifts"`
}

type TriageRotation struct {
	Weeks int     `json:"weeks"`
	Start *string `json:"start"`
}

type TriageShift struct {
	User     UserRef `json:"user"`
	StartsAt string  `json:"startsAt"`
	EndsAt   string  `json:"endsAt"`
}

// TeamStats is what a team's answer counts.
type TeamStats struct {
	IsMember    bool
	MemberCount int
	IssueCount  int
}

// TeamOf needs the team's workspace and parent loaded.
func TeamOf(t *ent.Team, publicURL string, stats TeamStats) Team {
	return Team{
		ID: ID(t.ID), Key: t.Key, Name: t.Name, Description: t.Description, Icon: t.Icon, Color: t.Color,
		Private: t.Private, Parent: TeamRefOf(t.Edges.Parent), URL: publicURL + "/" + t.Edges.Workspace.URLKey + "/team/" + t.Key,
		IsMember: stats.IsMember, MemberCount: stats.MemberCount, IssueCount: stats.IssueCount,
		CyclesEnabled: t.CyclesEnabled, TriageEnabled: t.TriageEnabled, Timezone: t.Timezone,
		ArchivedAt: TimePtr(t.ArchivedAt), CreatedAt: Time(t.CreatedAt), UpdatedAt: Time(t.UpdatedAt),
	}
}

// TeamDetailOf needs the team's workspace, parent, the states its settings name, its active
// states in board order, its members with their users by name, its visible sub-teams by name and
// its current cycle loaded.
func TeamDetailOf(t *ent.Team, publicURL string, stats TeamStats, labelCount int, allowed []float64, triageResponsible []*ent.User, action string, responsible []*ent.User) TeamDetail {
	detail := TeamDetail{
		Team:              TeamOf(t, publicURL, stats),
		EstimateType:      t.EstimateType,
		EstimateAllowZero: t.EstimateAllowZero, EstimateExtended: t.EstimateExtended, DefaultEstimate: t.DefaultEstimate,
		AllowedEstimates:   allowed,
		CycleDurationWeeks: t.CycleDurationWeeks, CycleCooldownWeeks: t.CycleCooldownWeeks, CycleStartDay: t.CycleStartDay,
		UpcomingCycleCount: t.UpcomingCycleCount, CycleAutoAddStarted: t.CycleAutoAddStarted,
		CycleAutoAddCompleted: t.CycleAutoAddCompleted, CycleRollover: t.CycleRollover,
		TriageAction: t.TriageAction, TriageResponsible: UserRefsOf(triageResponsible),
		TriageRotationWeeks: t.TriageRotationWeeks, TriageRotationStart: DatePtr(t.TriageRotationStart),
		RequirePriorityToLeaveTriage: t.RequirePriorityToLeaveTriage,
		Triage:                       TeamTriage{Enabled: t.TriageEnabled, Action: action, Responsible: UserRefsOf(responsible)},
		AutoArchiveMonths:            t.AutoArchiveMonths, AutoCloseMonths: t.AutoCloseMonths,
		AutoCloseState:        StateRefOf(t.Edges.AutoCloseState),
		AutoCloseParentIssues: t.AutoCloseParentIssues, AutoCloseChildIssues: t.AutoCloseChildIssues,
		DefaultIssueState:   StateRefOf(t.Edges.DefaultIssueState),
		GitHubPROpenedState: StateRefOf(t.Edges.GithubPrOpenedState),
		GitHubPRMergedState: StateRefOf(t.Edges.GithubPrMergedState),
		States:              make([]TeamStatus, len(t.Edges.WorkflowStates)),
		Members:             make([]TeamMember, len(t.Edges.TeamMembers)),
		LabelCount:          labelCount,
		SubTeams:            make([]TeamRef, len(t.Edges.Children)),
	}
	if len(t.Edges.Cycles) > 0 {
		detail.CurrentCycle = CycleRefOf(t.Edges.Cycles[0])
	}
	for i, s := range t.Edges.WorkflowStates {
		detail.States[i] = TeamStatus{ID: ID(s.ID), Name: s.Name, Type: s.Type, Color: s.Color, Description: s.Description, Position: s.Position}
	}
	for i, m := range t.Edges.TeamMembers {
		detail.Members[i] = TeamMember{UserRef: *UserRefOf(m.Edges.User), Owner: m.Owner}
	}
	for i, c := range t.Edges.Children {
		detail.SubTeams[i] = *TeamRefOf(c)
	}
	return detail
}

// UserRefsOf is the users' refs in their order, never nil.
func UserRefsOf(users []*ent.User) []UserRef {
	refs := make([]UserRef, len(users))
	for i, u := range users {
		refs[i] = *UserRefOf(u)
	}
	return refs
}
