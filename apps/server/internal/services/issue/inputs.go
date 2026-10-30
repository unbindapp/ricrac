package issue_service

import "ricrac/internal/common/args"

type ListInput struct {
	Team               *string `json:"team" doc:"Optional team key, name or id."`
	IncludeSubTeams    bool    `json:"includeSubTeams,omitempty" doc:"With team: also issues of its sub-teams."`
	Assignee           *string `json:"assignee" doc:"Person (id, name, email or \"me\"), or \"null\" for unassigned issues."`
	Creator            *string `json:"creator" doc:"Person who created the issues (id, name, email or \"me\")."`
	Subscriber         *string `json:"subscriber" doc:"Person subscribed to the issues (id, name, email or \"me\")."`
	State              *string `json:"state" doc:"Status name, type (triage, backlog, unstarted, started, completed, canceled) or id."`
	Label              *string `json:"label" doc:"Label name or id."`
	Project            *string `json:"project" doc:"Project name, slug or id, or \"null\" for issues in no project."`
	Milestone          *string `json:"milestone" doc:"Milestone name or id."`
	Cycle              *string `json:"cycle" doc:"Cycle number, name or id, or \"current\", \"next\", \"previous\"."`
	Release            *string `json:"release" doc:"Release name, slug or id."`
	ParentID           *string `json:"parentId" doc:"Sub-issues of this issue (identifier or id)."`
	Priority           *int    `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Query              *string `json:"query" doc:"Words to find in the title, description or comments."`
	CreatedAt          *string `json:"createdAt" doc:"Created after: ISO 8601 date/time or a duration like -P1D."`
	UpdatedAt          *string `json:"updatedAt" doc:"Updated after: ISO 8601 date/time or a duration like -P1W."`
	DueBefore          *string `json:"dueBefore" doc:"Due on or before this date (or duration from now, e.g. P1W)."`
	CustomView         *string `json:"customView" doc:"A saved custom view (name, slug or id) whose filter applies too."`
	Filter             any     `json:"filter" doc:"Any further conditions. See the filter help in the description of this field: Fields: id, number, title, description, priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt, triagedAt, snoozedUntil, slaBreachesAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, projectMilestone {id,name}, cycle {id,number,name,isActive}, parent {id,identifier}, subscribers {some: user}, hasBlockedByRelations, hasBlockingRelations, hasRelatedRelations, hasDuplicateRelations, children {some: filter}, searchableContent {contains}, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, contains, notContains, startsWith, endsWith, null. Example: {\"or\": [{\"priority\": {\"lte\": 2, \"neq\": 0}}, {\"labels\": {\"some\": {\"name\": {\"eq\": \"Bug\"}}}}]}"`
	IncludeArchived    bool    `json:"includeArchived,omitempty"`
	IncludeTrashed     bool    `json:"includeTrashed,omitempty"`
	IncludeDescription bool    `json:"includeDescription,omitempty" doc:"Include each issue's description."`
	OrderBy            *string `json:"orderBy" doc:"updatedAt (default), createdAt, priority, dueDate, manual (the team's board order) or identifier."`
	args.Page
}

type GetInput struct {
	ID              string `json:"id" doc:"Identifier (ENG-123), id or link."`
	IncludeComments bool   `json:"includeComments,omitempty" doc:"Also return the comment threads."`
	IncludeHistory  bool   `json:"includeHistory,omitempty" doc:"Also return the change history."`
}

type LinkInput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type SaveInput struct {
	ID                *string                `json:"id" doc:"Only to update an issue: its identifier (ENG-123), id or link. Omit to create one."`
	Team              *string                `json:"team" doc:"Optional: the workspace's team by default (or the parent's, or the template's). Its key, name or id."`
	Title             *string                `json:"title"`
	Description       *string                `json:"description" doc:"Markdown. Mention people with @displayName, @email or their linked GitHub login; they are notified and subscribed."`
	Patch             []args.TextEdit        `json:"patch" doc:"Edits to the current description instead of the whole text, applied in order, all or none."`
	State             *string                `json:"state" doc:"Status name, type (backlog, unstarted, started, completed, canceled, triage) or id."`
	Priority          *int                   `json:"priority" doc:"1 urgent, 2 high, 3 medium, 4 low; 0 means no priority (not the highest)."`
	Estimate          args.Nullable[float64] `json:"estimate" doc:"Estimate points on the team's scale; null clears it. T-shirt sizes are XS=1, S=2, M=3, L=5, XL=8, XXL=13, XXXL=21."`
	Assignee          args.Nullable[string]  `json:"assignee" doc:"Person (id, name, email or \"me\"); null unassigns."`
	Labels            []string               `json:"labels" doc:"Replace all labels with these (names or ids; Group/Child for a label in a group)."`
	AddLabels         []string               `json:"addLabels"`
	RemoveLabels      []string               `json:"removeLabels"`
	Project           args.Nullable[string]  `json:"project" doc:"Project name, slug or id; null removes the issue from its project."`
	Milestone         args.Nullable[string]  `json:"milestone" doc:"Milestone name or id: the project's, or one of the workspace's own; null clears it."`
	Cycle             args.Nullable[string]  `json:"cycle" doc:"Cycle number, name or id, or \"current\" / \"next\"; null removes it from its cycle."`
	ParentID          args.Nullable[string]  `json:"parentId" doc:"Parent issue (identifier or id), making this a sub-issue; null detaches it."`
	DueDate           args.Nullable[string]  `json:"dueDate" doc:"YYYY-MM-DD; null clears it."`
	Blocks            []string               `json:"blocks" doc:"Issues this one blocks (added to existing relations)."`
	BlockedBy         []string               `json:"blockedBy" doc:"Issues blocking this one (added to existing relations)."`
	RelatedTo         []string               `json:"relatedTo" doc:"Issues related to this one (added)."`
	RemoveBlocks      []string               `json:"removeBlocks"`
	RemoveBlockedBy   []string               `json:"removeBlockedBy"`
	RemoveRelatedTo   []string               `json:"removeRelatedTo"`
	DuplicateOf       args.Nullable[string]  `json:"duplicateOf" doc:"The issue this duplicates; null removes the duplicate relation. Marking a duplicate moves it to a canceled status."`
	Links             []LinkInput            `json:"links" doc:"Links to attach ({url, title}); a link already on the issue is updated."`
	AddSubscribers    []string               `json:"addSubscribers"`
	RemoveSubscribers []string               `json:"removeSubscribers"`
	AddReleases       []string               `json:"addReleases" doc:"Release names, slugs or ids to add."`
	RemoveReleases    []string               `json:"removeReleases"`
	SetReleases       []string               `json:"setReleases" doc:"Replace all releases with these."`
	SnoozedUntil      args.Nullable[string]  `json:"snoozedUntil" doc:"Hide the issue from triage / inbox until this time (ISO 8601 or duration like P1W); null wakes it."`
	SLABreachesAt     args.Nullable[string]  `json:"slaBreachesAt" doc:"When the SLA breaches (ISO 8601); null removes the SLA."`
	SLAType           *string                `json:"slaType" doc:"\"all\" or \"onlyBusinessDays\"."`
	SortOrder         *float64               `json:"sortOrder" doc:"Position on the team's board (lower is higher up)."`
	SubIssueSortOrder *float64               `json:"subIssueSortOrder" doc:"Position among its parent's sub-issues."`
	Template          *string                `json:"template" doc:"Issue template name or id, applied on create: it fills in fields and sub-issues the arguments don't set."`
	Preview           *bool                  `json:"preview" doc:"true answers the issue as it would be saved, and where it would open on GitHub, without saving anything: to show it to the user before they confirm."`
	GitHubRepository  *string                `json:"githubRepository" doc:"Only on create: the synced repository (owner/name, or name) to open the issue in on GitHub too, or \"none\" to keep it off GitHub. Omitted, it opens on GitHub only when the team's githubOpenIssues is on, in the repository synced with the issue's project, else the one with the most synced issues."`
}

type UpdateManyInput struct {
	IDs     []string `json:"ids" doc:"Identifiers (ENG-123) or ids of the issues to change, at most 100."`
	Changes args.M   `json:"changes" doc:"The change: an object with save_issue's fields (except id and template), e.g. {\"state\": \"Done\"}, {\"assignee\": \"me\", \"addLabels\": [\"Bug\"]} or {\"cycle\": \"current\"}."`
}

type ShareInput struct {
	ID   string `json:"id" doc:"Identifier (ENG-123) or id of the issue."`
	User string `json:"user" doc:"Workspace member to share with: id, name, email or \"me\"."`
}

// IDInput names one issue.
type IDInput struct {
	ID string `json:"id"`
}
