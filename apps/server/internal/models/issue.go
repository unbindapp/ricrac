package models

import (
	"regexp"
	"strings"

	"ricrac/ent"
	"ricrac/internal/common/utils"
)

var branchSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// BranchName is the git branch named after an issue: eng-12-fix-the-login.
func BranchName(teamKey string, number int, title string) string {
	slug := strings.Trim(branchSeparators.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return strings.ToLower(Identifier(teamKey, number)) + "-" + slug
}

// IssueURL is the issue's page in the web app.
func IssueURL(publicURL, workspaceKey, identifier string) string {
	return publicURL + "/" + workspaceKey + "/issue/" + identifier
}

// IssueOf needs the issue's workspace, team, state, assignee, creator, labels (by name, with
// their parents), project, milestone, cycle and parent (with its team and state) loaded.
func IssueOf(i *ent.Issue, publicURL string, subIssueCount int, withDescription bool) Issue {
	e := i.Edges
	identifier := Identifier(e.Team.Key, i.Number)
	labels := make([]LabelRef, len(e.Labels))
	for n, l := range e.Labels {
		labels[n] = LabelRefOf(l)
	}
	issue := Issue{
		ID: ID(i.ID), Identifier: identifier, Number: i.Number, Title: i.Title,
		URL:        IssueURL(publicURL, e.Workspace.URLKey, identifier),
		BranchName: BranchName(e.Team.Key, i.Number, i.Title),
		Priority:   i.Priority, Estimate: i.Estimate, State: *StateRefOf(e.State), Team: *TeamRefOf(e.Team),
		Assignee: UserRefOf(e.Assignee), Creator: UserRefOf(e.Creator), Labels: labels,
		Project: ProjectRefOf(e.Project), ProjectMilestone: MilestoneRefOf(e.Milestone), Cycle: CycleRefOf(e.Cycle),
		Parent: IssueRefOf(e.Parent), DueDate: DatePtr(i.DueDate), SortOrder: i.SortOrder,
		SubIssueSortOrder: i.SubIssueSortOrder, SubIssueCount: subIssueCount, SLAType: i.SLAType,
		SLAStartedAt: TimePtr(i.SLAStartedAt), SLABreachesAt: TimePtr(i.SLABreachesAt), SnoozedUntil: TimePtr(i.SnoozedUntil),
		StartedAt: TimePtr(i.StartedAt), CompletedAt: TimePtr(i.CompletedAt), CanceledAt: TimePtr(i.CanceledAt),
		StartedTriageAt: TimePtr(i.StartedTriageAt), TriagedAt: TimePtr(i.TriagedAt), AutoClosedAt: TimePtr(i.AutoClosedAt),
		AutoArchivedAt: TimePtr(i.AutoArchivedAt), ArchivedAt: TimePtr(i.ArchivedAt), Trashed: i.TrashedAt != nil,
		TrashedAt: TimePtr(i.TrashedAt), PreviousIdentifiers: previousIdentifiers(i), CreatedAt: Time(i.CreatedAt),
		UpdatedAt: Time(i.UpdatedAt),
	}
	if i.Priority >= 0 && i.Priority < len(utils.PriorityNames) {
		issue.PriorityLabel = utils.PriorityNames[i.Priority]
	}
	if withDescription {
		issue.Description = TextOf(i.Description)
	}
	return issue
}

func previousIdentifiers(i *ent.Issue) []string {
	if i.PreviousIdentifiers == nil {
		return []string{}
	}
	return []string(i.PreviousIdentifiers)
}
