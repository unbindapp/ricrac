package issue_service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	issue_repo "ricrac/internal/repositories/issue"
)

const maxImportIssues = 2000

var errDryRun = errors.New("dry run")

// Import imports issues from another tracker in one transaction, keeping their numbers, dates,
// people and comments.
func (self *IssueService) Import(ctx context.Context, actor *auth.Actor, input *ImportInput) (models.ImportedIssues, error) {
	if err := actor.RequireAdmin(); err != nil {
		return models.ImportedIssues{}, err
	}
	issues := input.Issues
	switch {
	case input.Issues != nil && input.FileID == nil:
	case input.Issues == nil && input.FileID != nil:
		var err error
		if issues, err = self.importFromFile(ctx, actor, *input.FileID); err != nil {
			return models.ImportedIssues{}, err
		}
	default:
		return models.ImportedIssues{}, errdefs.BadRequest("Pass either issues or fileId")
	}
	if len(issues) == 0 || len(issues) > maxImportIssues {
		return models.ImportedIssues{}, errdefs.BadRequest("Import 1 to %d issues at a time", maxImportIssues)
	}
	team, err := self.resolve.TeamOrDefault(ctx, actor, input.Team)
	if err != nil {
		return models.ImportedIssues{}, err
	}
	result := models.ImportedIssues{DryRun: input.DryRun}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.importInTx(ctx, tx, actor, team, issues, &result); err != nil {
			return err
		}
		if input.DryRun {
			return errDryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, errDryRun) {
		return models.ImportedIssues{}, err
	}
	return result, nil
}

func (self *IssueService) importInTx(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, issues []ImportIssue, result *models.ImportedIssues) error {
	locked, err := self.repo.Issue().LockTeam(ctx, tx, team)
	if err != nil {
		return err
	}
	key := strings.ToUpper(locked.Key)
	numbers, err := self.assignNumbers(ctx, tx, team, key, issues)
	if err != nil {
		return err
	}
	ids := map[int]uuid.UUID{}
	comments := 0
	for i, issue := range issues {
		id, err := self.insertImported(ctx, tx, actor, team, numbers[i], issue)
		if err != nil {
			return importedAt(key, numbers[i], err)
		}
		comments += len(issue.Comments)
		ids[numbers[i]] = id
	}
	relations := 0
	for i, issue := range issues {
		added, err := self.connectImported(ctx, tx, actor, key, ids, ids[numbers[i]], issue)
		if err != nil {
			return importedAt(key, numbers[i], err)
		}
		relations += added
	}
	count, err := self.repo.Issue().RaiseIssueCount(ctx, tx, team, slices.Max(numbers))
	if err != nil {
		return err
	}
	for _, number := range numbers {
		value, err := self.Value(ctx, tx, ids[number])
		if err != nil {
			return err
		}
		if err := self.events.Emit(ctx, tx, actor, "Issue", "create", value, nil, &team); err != nil {
			return err
		}
	}
	if err := self.events.Audit(ctx, tx, actor, "issues.imported", M{"team": key, "count": len(issues)}); err != nil {
		return err
	}
	identifiers := make([]string, len(numbers))
	for i, n := range numbers {
		identifiers[i] = fmt.Sprintf("%s-%d", key, n)
	}
	result.Imported = len(identifiers)
	result.Comments = comments
	result.Relations = relations
	result.Identifiers = identifiers
	result.NextIdentifier = fmt.Sprintf("%s-%d", key, count+1)
	return nil
}

func (self *IssueService) importFromFile(ctx context.Context, actor *auth.Actor, reference string) ([]ImportIssue, error) {
	notFound := errdefs.NotFound("No uploaded file %q", strings.TrimSpace(reference))
	id, ok := utils.AsUUID(reference)
	if !ok {
		return nil, notFound
	}
	found, err := self.repo.Issue().File(ctx, actor.Workspace, id)
	if ent.IsNotFound(err) {
		return nil, notFound
	}
	if err != nil {
		return nil, err
	}
	if found.Data == nil {
		return nil, errdefs.BadRequest("The file hasn't been uploaded yet; PUT it to its uploadUrl")
	}
	var parsed any
	if err := json.Unmarshal(*found.Data, &parsed); err != nil {
		return nil, errdefs.BadRequest("The file isn't JSON: %s", err)
	}
	if object, ok := parsed.(M); ok {
		if list, ok := object["issues"]; ok {
			parsed = list
		}
	}
	var issues []ImportIssue
	decoder := json.NewDecoder(bytes.NewReader(errdefs.Marshal(parsed)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&issues); err != nil {
		return nil, errdefs.BadRequest("The file's issues are invalid: %s", args.DescribeDecodeError(err))
	}
	return issues, nil
}

// assignNumbers is the numbers given, checked to be free, and the next ones after everything
// else for the rest.
func (self *IssueService) assignNumbers(ctx context.Context, tx repository.TxInterface, team uuid.UUID, key string, issues []ImportIssue) ([]int, error) {
	given := []int{}
	for _, issue := range issues {
		if issue.Number != nil {
			given = append(given, *issue.Number)
		}
	}
	for _, n := range given {
		if n < 1 {
			return nil, errdefs.BadRequest("%d isn't an issue number; numbers start at 1", n)
		}
	}
	seen := map[int]bool{}
	for _, n := range given {
		if seen[n] {
			return nil, errdefs.BadRequest("%s-%d appears twice in the import", key, n)
		}
		seen[n] = true
	}
	taken, err := self.repo.Issue().TakenNumbers(ctx, tx, team, given)
	if err != nil {
		return nil, err
	}
	if len(taken) > 0 {
		list := make([]string, min(len(taken), 20))
		for i := range list {
			list[i] = fmt.Sprintf("%s-%d", key, taken[i])
		}
		which := "These numbers are"
		if len(taken) == 1 {
			which = "This number is"
		}
		return nil, errdefs.BadRequest("%s taken in the team: %s", which, strings.Join(list, ", "))
	}
	stored, err := self.repo.Issue().Team(ctx, tx, team)
	if err != nil {
		return nil, err
	}
	next := stored.IssueCount
	for _, n := range given {
		next = max(next, n)
	}
	numbers := make([]int, len(issues))
	for i, issue := range issues {
		if issue.Number != nil {
			numbers[i] = *issue.Number
			continue
		}
		next++
		numbers[i] = next
	}
	return numbers, nil
}

// importedAt says which issue of the import an error is about.
func importedAt(key string, number int, err error) error {
	apiErr := errdefs.As(err)
	return errdefs.Errorf(apiErr.Status, "%s-%d: %s", key, number, apiErr.Message)
}

func optionalTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	at, err := utils.ParseDatetime(*value)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

func (self *IssueService) optionalUser(ctx context.Context, actor *auth.Actor, value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := self.resolve.User(ctx, actor, *value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (self *IssueService) optionalRef(value *string, resolve func(string) (uuid.UUID, error)) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := resolve(*value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (self *IssueService) insertImported(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, number int, issue ImportIssue) (uuid.UUID, error) {
	title, err := utils.NonEmpty(issue.Title, "title")
	if err != nil {
		return uuid.Nil, err
	}
	var state uuid.UUID
	if issue.State != nil {
		state, err = self.resolve.State(ctx, actor, team, *issue.State)
	} else {
		state, err = self.DefaultState(ctx, tx, team)
	}
	if err != nil {
		return uuid.Nil, err
	}
	kind, err := self.StateType(ctx, tx, state)
	if err != nil {
		return uuid.Nil, err
	}
	var estimate *float64
	if issue.Estimate != nil {
		settings, err := self.repo.Issue().Team(ctx, tx, team)
		if err != nil {
			return uuid.Nil, err
		}
		e, err := CheckEstimate(settings, *issue.Estimate)
		if err != nil {
			return uuid.Nil, err
		}
		estimate = &e
	}
	creator := actor.User
	if found, err := self.optionalUser(ctx, actor, issue.Creator); err != nil {
		return uuid.Nil, err
	} else if found != nil {
		creator = *found
	}
	assignee, err := self.optionalUser(ctx, actor, issue.Assignee)
	if err != nil {
		return uuid.Nil, err
	}
	project, err := self.optionalRef(issue.Project, func(v string) (uuid.UUID, error) { return self.resolve.Project(ctx, actor, v) })
	if err != nil {
		return uuid.Nil, err
	}
	milestone, err := self.optionalRef(issue.Milestone, func(v string) (uuid.UUID, error) { return self.resolve.Milestone(ctx, actor, project, v) })
	if err != nil {
		return uuid.Nil, err
	}
	cycle, err := self.optionalRef(issue.Cycle, func(v string) (uuid.UUID, error) { return self.resolve.Cycle(ctx, actor, &team, v) })
	if err != nil {
		return uuid.Nil, err
	}
	created := time.Now().UTC()
	if at, err := optionalTime(issue.CreatedAt); err != nil {
		return uuid.Nil, err
	} else if at != nil {
		created = *at
	}
	stamp := func(value *string, applies bool) (*time.Time, error) {
		at, err := optionalTime(value)
		if err != nil || at != nil || !applies {
			return at, err
		}
		return &created, nil
	}
	started, err := stamp(issue.StartedAt, kind == "started" || kind == "completed")
	if err != nil {
		return uuid.Nil, err
	}
	completed, err := stamp(issue.CompletedAt, kind == "completed")
	if err != nil {
		return uuid.Nil, err
	}
	canceled, err := stamp(issue.CanceledAt, kind == "canceled")
	if err != nil {
		return uuid.Nil, err
	}
	archived, err := optionalTime(issue.ArchivedAt)
	if err != nil {
		return uuid.Nil, err
	}
	commentTimes := make([]time.Time, len(issue.Comments))
	for i, comment := range issue.Comments {
		at, err := optionalTime(comment.CreatedAt)
		if err != nil {
			return uuid.Nil, err
		}
		commentTimes[i] = created
		if at != nil {
			commentTimes[i] = *at
		}
	}
	updated, err := optionalTime(issue.UpdatedAt)
	if err != nil {
		return uuid.Nil, err
	}
	if updated == nil {
		latest := created
		for _, at := range append([]*time.Time{started, completed, canceled, archived}, pointers(commentTimes)...) {
			if at != nil && at.After(latest) {
				latest = *at
			}
		}
		updated = &latest
	}
	var due *time.Time
	if issue.DueDate != nil {
		date, err := utils.ParseDate(*issue.DueDate)
		if err != nil {
			return uuid.Nil, err
		}
		due = &date
	}
	var sortOrder float64
	if issue.SortOrder != nil {
		sortOrder = *issue.SortOrder
	} else if sortOrder, err = self.repo.Issue().TopSortOrder(ctx, tx, team); err != nil {
		return uuid.Nil, err
	}
	priority := 0
	if issue.Priority != nil {
		priority = *issue.Priority
	}
	if priority, err = utils.CheckPriority(priority); err != nil {
		return uuid.Nil, err
	}
	issueProject := project
	if issueProject == nil && milestone != nil {
		found, err := self.repo.Issue().Milestone(ctx, tx, milestone)
		if err != nil && !ent.IsNotFound(err) {
			return uuid.Nil, err
		}
		if found != nil {
			issueProject = found.ProjectID
		}
	}
	id, err := self.repo.Issue().InsertImported(ctx, tx, issue_repo.ImportedIssue{
		Workspace: actor.Workspace, Team: team, Number: number, Title: title, Description: utils.OrEmpty(issue.Description),
		State: state, Priority: priority, Estimate: estimate, Assignee: assignee, Creator: creator, Project: issueProject,
		Milestone: milestone, Cycle: cycle, DueDate: due, SortOrder: sortOrder, StartedAt: started, CompletedAt: completed,
		CanceledAt: canceled, ArchivedAt: archived, CreatedAt: created, UpdatedAt: *updated,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if project != nil {
		if err := self.repo.Issue().AddProjectTeam(ctx, tx, *project, team); err != nil {
			return uuid.Nil, err
		}
	}
	if issue.Labels != nil {
		labels, err := self.resolve.Labels(ctx, actor, &team, issue.Labels)
		if err != nil {
			return uuid.Nil, err
		}
		clashes, err := self.repo.Issue().SingleSelectClashes(ctx, tx, labels)
		if err != nil {
			return uuid.Nil, err
		}
		if len(clashes) > 0 {
			return uuid.Nil, errdefs.BadRequest("An issue takes one label of the group %q", clashes[0])
		}
		if err := self.repo.Issue().AddLabels(ctx, tx, id, labels); err != nil {
			return uuid.Nil, err
		}
	}
	subscribers := []uuid.UUID{creator}
	if assignee != nil {
		subscribers = append(subscribers, *assignee)
	}
	others, err := self.resolve.Users(ctx, actor, issue.Subscribers)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.events.Subscribe(ctx, tx, id, append(subscribers, others...)); err != nil {
		return uuid.Nil, err
	}
	for _, link := range issue.Links {
		if _, err := self.attachments.Upsert(ctx, tx, actor, id, link.URL, link.Title, link.Subtitle, nil, nil); err != nil {
			return uuid.Nil, err
		}
	}
	for i, comment := range issue.Comments {
		author := actor.User
		if found, err := self.optionalUser(ctx, actor, comment.Author); err != nil {
			return uuid.Nil, err
		} else if found != nil {
			author = *found
		}
		body, err := utils.NonEmpty(comment.Body, "comment body")
		if err != nil {
			return uuid.Nil, err
		}
		if err := self.repo.Issue().InsertComment(ctx, tx, actor.Workspace, id, author, body, commentTimes[i]); err != nil {
			return uuid.Nil, err
		}
	}
	changes, err := json.Marshal(M{"importedBy": actor.User})
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.repo.Issue().ImportedHistory(ctx, tx, id, creator, changes, created); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func pointers(times []time.Time) []*time.Time {
	list := make([]*time.Time, len(times))
	for i := range times {
		list[i] = &times[i]
	}
	return list
}

// findImported is an issue of this import by its identifier, else one already in Ricrac.
func (self *IssueService) findImported(ctx context.Context, actor *auth.Actor, key string, ids map[int]uuid.UUID, value string) (uuid.UUID, error) {
	if k, number, ok := utils.ParseIdentifier(value); ok && k == key {
		if id, found := ids[number]; found {
			return id, nil
		}
	}
	return self.resolve.Issue(ctx, actor, value)
}

// connectImported adds the parent, relations and duplicates, once every issue of the import
// exists. Returns how many relations were added.
func (self *IssueService) connectImported(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, key string, ids map[int]uuid.UUID, id uuid.UUID, issue ImportIssue) (int, error) {
	if issue.Parent != nil {
		parent, err := self.findImported(ctx, actor, key, ids, *issue.Parent)
		if err != nil {
			return 0, err
		}
		loops, err := self.repo.Issue().InOwnTree(ctx, tx, parent, id)
		if err != nil {
			return 0, err
		}
		if loops {
			return 0, errdefs.BadRequest("An issue can't be a sub-issue of itself or of its own sub-issues")
		}
		order, err := self.repo.Issue().NextSubIssueSortOrder(ctx, tx, parent)
		if err != nil {
			return 0, err
		}
		if err := self.repo.Issue().Update(ctx, tx, id, issue_repo.SetParent(&parent, &order)); err != nil {
			return 0, err
		}
	}
	var duplicate []string
	if issue.DuplicateOf != nil {
		duplicate = []string{*issue.DuplicateOf}
	}
	lists := []relationList{
		{issue.RelatedTo, "related", false},
		{issue.Blocks, "blocks", false},
		{issue.BlockedBy, "blocks", true},
		{duplicate, "duplicate", false},
	}
	added := 0
	for _, list := range lists {
		for _, value := range list.list {
			other, err := self.findImported(ctx, actor, key, ids, value)
			if err != nil {
				return 0, err
			}
			if other == id {
				return 0, errdefs.BadRequest("An issue can't be related to itself")
			}
			from, to := id, other
			if list.reverse {
				from, to = other, id
			}
			created, err := self.repo.Issue().RelateImported(ctx, tx, from, to, list.kind, actor.User)
			if err != nil {
				return 0, err
			}
			if created {
				added++
			}
		}
	}
	return added, nil
}

type ImportInput struct {
	Team   *string       `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id. The team the issues go to."`
	Issues []ImportIssue `json:"issues" doc:"The issues, at most 2000. Pass these or fileId."`
	FileID *string       `json:"fileId" doc:"A JSON file from prepare_file_upload holding the issues as an array, or as {\"issues\": [...]}."`
	DryRun bool          `json:"dryRun,omitempty" doc:"Check everything and report what would be imported, without importing."`
}

type ImportIssue struct {
	Number      *int            `json:"number" doc:"The number to keep, e.g. 42 for ENG-42; must be free in the team. Without it the issue takes the next one."`
	Title       string          `json:"title"`
	Description *string         `json:"description" doc:"Markdown, kept as is. Mentions don't notify anyone."`
	State       *string         `json:"state" doc:"Status name, type or id; the team's default when left out."`
	Priority    *int            `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Estimate    *float64        `json:"estimate" doc:"Points on the team's scale."`
	Assignee    *string         `json:"assignee" doc:"Person (id, name, email); a member of the workspace."`
	Creator     *string         `json:"creator" doc:"Who created it (id, name, email); you when left out."`
	Labels      []string        `json:"labels" doc:"Label names or ids (Group/Child for a label in a group)."`
	Project     *string         `json:"project" doc:"Project name, slug or id."`
	Milestone   *string         `json:"milestone" doc:"Milestone name or id of the project."`
	Cycle       *string         `json:"cycle" doc:"Cycle number, name or id of the team."`
	Parent      *string         `json:"parent" doc:"The parent issue's identifier, within the import or already in Ricrac."`
	DueDate     *string         `json:"dueDate" doc:"YYYY-MM-DD."`
	SortOrder   *float64        `json:"sortOrder" doc:"Position on the team's board (lower is higher up)."`
	CreatedAt   *string         `json:"createdAt" doc:"ISO 8601; now when left out."`
	UpdatedAt   *string         `json:"updatedAt" doc:"ISO 8601; the latest of its other dates when left out."`
	StartedAt   *string         `json:"startedAt" doc:"ISO 8601; createdAt when left out and the status is started or completed."`
	CompletedAt *string         `json:"completedAt" doc:"ISO 8601; createdAt when left out and the status is completed."`
	CanceledAt  *string         `json:"canceledAt" doc:"ISO 8601; createdAt when left out and the status is canceled."`
	ArchivedAt  *string         `json:"archivedAt" doc:"ISO 8601; archives the issue."`
	Links       []ImportLink    `json:"links" doc:"Links to attach."`
	Subscribers []string        `json:"subscribers" doc:"People subscribed besides the creator and assignee."`
	RelatedTo   []string        `json:"relatedTo" doc:"Identifiers of related issues."`
	Blocks      []string        `json:"blocks" doc:"Identifiers of issues this one blocks."`
	BlockedBy   []string        `json:"blockedBy" doc:"Identifiers of issues blocking this one."`
	DuplicateOf *string         `json:"duplicateOf" doc:"Identifier of the issue this duplicates."`
	Comments    []ImportComment `json:"comments" doc:"Comments, oldest first."`
}

type ImportLink struct {
	URL      string  `json:"url"`
	Title    string  `json:"title"`
	Subtitle *string `json:"subtitle"`
}

type ImportComment struct {
	Body      string  `json:"body" doc:"Markdown."`
	Author    *string `json:"author" doc:"Person (id, name, email); you when left out."`
	CreatedAt *string `json:"createdAt" doc:"ISO 8601; the issue's createdAt when left out."`
}
