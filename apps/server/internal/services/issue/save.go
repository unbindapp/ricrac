package issue_service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	issue_repo "ricrac/internal/repositories/issue"
	events_service "ricrac/internal/services/events"
)

// errPreviewed rolls back a save made only to preview it.
var errPreviewed = errors.New("previewed")

func (self *IssueService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.IssueDetail, error) {
	var id uuid.UUID
	var preview models.IssueDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var err error
		if id, err = self.save(ctx, tx, actor, input); err != nil {
			return err
		}
		if input.Preview == nil || !*input.Preview {
			return nil
		}
		if preview, err = self.preview(ctx, tx, actor.Fresh(), id); err != nil {
			return err
		}
		return errPreviewed
	})
	if errors.Is(err, errPreviewed) {
		return preview, nil
	}
	if err != nil {
		return models.IssueDetail{}, err
	}
	return self.Detail(ctx, actor.Fresh(), id, false, false)
}

func (self *IssueService) save(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	if input.ID == nil {
		return self.Create(ctx, tx, actor, *input)
	}
	if input.GitHubRepository != nil {
		return uuid.Nil, errdefs.BadRequest("githubRepository only applies when creating an issue")
	}
	id, err := self.resolve.Issue(ctx, actor, *input.ID)
	if err != nil {
		return uuid.Nil, err
	}
	return id, self.Update(ctx, tx, actor, id, *input)
}

// preview is the issue as tx saved it, with where it would open on GitHub.
func (self *IssueService) preview(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.IssueDetail, error) {
	detail, err := self.detailIn(ctx, tx, actor, id, false, false)
	if err != nil {
		return models.IssueDetail{}, err
	}
	github, err := self.repo.Issue().PendingGitHub(ctx, tx, id)
	if err != nil {
		return models.IssueDetail{}, err
	}
	detail.Preview = &models.IssuePreview{GitHub: github}
	return detail, nil
}

func (self *IssueService) UpdateMany(ctx context.Context, actor *auth.Actor, input *UpdateManyInput) (models.UpdatedIssues, error) {
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		return models.UpdatedIssues{}, errdefs.BadRequest("Pass 1 to 100 issue ids")
	}
	if input.Changes == nil {
		return models.UpdatedIssues{}, errdefs.BadRequest("Invalid arguments: missing field `changes`")
	}
	changes, err := args.Parse[SaveInput](input.Changes)
	if err != nil {
		return models.UpdatedIssues{}, errdefs.BadRequest("Invalid changes: %s", strings.TrimPrefix(errdefs.As(err).Message, "Invalid arguments: "))
	}
	if changes.ID != nil || changes.Template != nil {
		return models.UpdatedIssues{}, errdefs.BadRequest("changes can't have an id or a template")
	}
	updated := []string{}
	for _, reference := range input.IDs {
		id, err := self.resolve.Issue(ctx, actor, reference)
		if err != nil {
			return models.UpdatedIssues{}, err
		}
		var identifier string
		err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
			if err := self.Update(ctx, tx, actor, id, changes); err != nil {
				return err
			}
			identifier, err = self.repo.Issue().Identifier(ctx, tx, id)
			return err
		})
		if err != nil {
			return models.UpdatedIssues{}, err
		}
		updated = append(updated, identifier)
	}
	return models.UpdatedIssues{Updated: updated}, nil
}

// issueTemplate is what a template fills in on a new issue.
type issueTemplate struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Team        *string  `json:"team"`
	State       *string  `json:"state"`
	Priority    *int     `json:"priority"`
	Estimate    *float64 `json:"estimate"`
	Assignee    *string  `json:"assignee"`
	Labels      []string `json:"labels"`
	Project     *string  `json:"project"`
	Cycle       *string  `json:"cycle"`
	DueDate     *string  `json:"dueDate"`
	SubIssues   []M      `json:"subIssues"`
}

func some[T any](value T) args.Nullable[T] {
	return args.Nullable[T]{Set: true, Valid: true, Value: value}
}

func orNullable[T any](current args.Nullable[T], fallback *T) args.Nullable[T] {
	if current.Set || fallback == nil {
		return current
	}
	return some(*fallback)
}

func firstOf[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// fromTemplate fills in what the template sets and the arguments don't, and is its sub-issues.
func (self *IssueService) fromTemplate(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) ([]M, error) {
	templateID, err := self.resolve.Template(ctx, actor, "issue", *input.Template)
	if err != nil {
		return nil, err
	}
	stored, err := self.repo.Issue().Template(ctx, tx, templateID)
	if err != nil {
		return nil, err
	}
	var t issueTemplate
	_ = json.Unmarshal(stored.Data, &t)
	var templateTeam *string
	if stored.TeamID != nil {
		templateTeam = new(stored.TeamID.String())
	}
	input.Template = nil
	input.Team = firstOf(input.Team, t.Team, templateTeam)
	input.Title = firstOf(input.Title, t.Title)
	if utils.OrEmpty(input.Description) == "" {
		input.Description = t.Description
	}
	input.State = firstOf(input.State, t.State)
	input.Priority = firstOf(input.Priority, t.Priority)
	input.Estimate = orNullable(input.Estimate, t.Estimate)
	input.Assignee = orNullable(input.Assignee, t.Assignee)
	if t.Labels != nil {
		input.AddLabels = append(input.AddLabels, t.Labels...)
	}
	input.Project = orNullable(input.Project, t.Project)
	input.Cycle = orNullable(input.Cycle, t.Cycle)
	input.DueDate = orNullable(input.DueDate, t.DueDate)
	return t.SubIssues, nil
}

// Create creates an issue in tx, with what follows: subscriptions, history, triage, the
// template's sub-issues and the webhook.
func (self *IssueService) Create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input SaveInput) (uuid.UUID, error) {
	var subIssues []M
	if input.Template != nil {
		var err error
		if subIssues, err = self.fromTemplate(ctx, tx, actor, &input); err != nil {
			return uuid.Nil, err
		}
	}
	var parent *uuid.UUID
	if input.ParentID.Valid {
		id, err := self.resolve.Issue(ctx, actor, input.ParentID.Value)
		if err != nil {
			return uuid.Nil, err
		}
		parent = &id
	}
	team, err := self.newIssueTeam(ctx, tx, actor, input.Team, parent)
	if err != nil {
		return uuid.Nil, err
	}
	title, err := utils.NonEmpty(utils.OrEmpty(input.Title), "title")
	if err != nil {
		return uuid.Nil, err
	}
	settings, err := self.repo.Issue().Team(ctx, tx, team)
	if err != nil {
		return uuid.Nil, err
	}
	var state uuid.UUID
	if input.State != nil {
		state, err = self.resolve.State(ctx, actor, team, *input.State)
	} else {
		state, err = self.DefaultState(ctx, tx, team)
	}
	if err != nil {
		return uuid.Nil, err
	}
	number, err := self.repo.Issue().NextNumber(ctx, tx, team)
	if err != nil {
		return uuid.Nil, err
	}
	var sortOrder float64
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	} else if sortOrder, err = self.repo.Issue().TopSortOrder(ctx, tx, team); err != nil {
		return uuid.Nil, err
	}
	var estimate *float64
	switch {
	case input.Estimate.Valid:
		e, err := CheckEstimate(settings, input.Estimate.Value)
		if err != nil {
			return uuid.Nil, err
		}
		estimate = &e
	case !input.Estimate.Set && settings.EstimateType != "notUsed":
		estimate = settings.DefaultEstimate
	}
	input.Estimate = args.Nullable[float64]{}
	id, err := self.repo.Issue().Insert(ctx, tx, issue_repo.NewIssue{
		Workspace: actor.Workspace, Team: team, Number: number, Title: title, Description: utils.OrEmpty(input.Description),
		State: state, Creator: actor.ActorID(), SortOrder: sortOrder, Estimate: estimate,
	})
	if err != nil {
		return uuid.Nil, err
	}
	input.Description = nil
	newType, err := self.StateType(ctx, tx, state)
	if err != nil {
		return uuid.Nil, err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.repo.Issue().StampCreated(ctx, tx, id, newType, now); err != nil {
		return uuid.Nil, err
	}
	if err := self.events.Subscribe(ctx, tx, id, []uuid.UUID{actor.User}); err != nil {
		return uuid.Nil, err
	}
	if err := self.events.History(ctx, tx, actor, id, "created", M{}); err != nil {
		return uuid.Nil, err
	}
	input.State, input.Title, input.Team = nil, nil, nil
	if err := self.applyChanges(ctx, tx, actor, id, input, true); err != nil {
		return uuid.Nil, err
	}
	if err := self.events.QueueNewIssueForGitHub(ctx, tx, actor, id, team, input.GitHubRepository); err != nil {
		return uuid.Nil, err
	}
	if newType == "triage" && settings.TriageEnabled {
		if err := self.routeToTriage(ctx, tx, actor, id, team); err != nil {
			return uuid.Nil, err
		}
	}
	for n, sub := range subIssues {
		subInput, err := args.Parse[SaveInput](sub)
		if err != nil {
			return uuid.Nil, errdefs.BadRequest("The template's sub-issue %d is invalid: %s", n+1, strings.TrimPrefix(errdefs.As(err).Message, "Invalid arguments: "))
		}
		subInput.ID, subInput.Template = nil, nil
		// The parent isn't committed yet, so resolving it by reference would miss it.
		subInput.ParentID = args.Nullable[string]{}
		if subInput.Team == nil {
			subInput.Team = new(team.String())
		}
		child, err := self.Create(ctx, tx, actor, subInput)
		if err != nil {
			return uuid.Nil, err
		}
		if err := self.repo.Issue().Update(ctx, tx, child, issue_repo.SetParent(&id, new(float64(n+1)*1000))); err != nil {
			return uuid.Nil, err
		}
	}
	value, err := self.Value(ctx, tx, id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, self.events.Emit(ctx, tx, actor, "Issue", "create", value, nil, &team)
}

// newIssueTeam is the team named, else the parent's, else the workspace's.
func (self *IssueService) newIssueTeam(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, named *string, parent *uuid.UUID) (uuid.UUID, error) {
	if named != nil || parent == nil {
		return self.resolve.TeamOrDefault(ctx, actor, named)
	}
	row, err := self.repo.Issue().Get(ctx, tx, *parent)
	if err != nil {
		return uuid.Nil, err
	}
	return row.TeamID, nil
}

func (self *IssueService) routeToTriage(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue, team uuid.UUID) error {
	action, people, err := self.TriageResponsible(ctx, tx, team)
	if err != nil || len(people) == 0 || action == "none" {
		return err
	}
	identifier, err := self.repo.Issue().Identifier(ctx, tx, issue)
	if err != nil {
		return err
	}
	if action == "assign" {
		assignee := people[0]
		if err := self.repo.Issue().AssignIfUnassigned(ctx, tx, issue, assignee); err != nil {
			return err
		}
		if err := self.events.Subscribe(ctx, tx, issue, []uuid.UUID{assignee}); err != nil {
			return err
		}
	}
	return self.events.Notify(ctx, tx, actor, people, events_service.Notice{Kind: "triageIssue", Summary: identifier + " needs triage", Issue: &issue})
}

// Update changes an issue in tx, with what follows: history, notifications, automations and
// the webhook.
func (self *IssueService) Update(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput) error {
	if input.Template != nil {
		return errdefs.BadRequest("Templates only apply when creating an issue")
	}
	before, err := self.Value(ctx, tx, id)
	if err != nil {
		return err
	}
	row, err := self.lockIssue(ctx, tx, id)
	if err != nil {
		return err
	}
	if input.Team != nil {
		team, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return err
		}
		if team != row.TeamID {
			if err := self.moveTeam(ctx, tx, actor, row, team); err != nil {
				return err
			}
		}
		input.Team = nil
	}
	if row, err = self.lockIssue(ctx, tx, id); err != nil {
		return err
	}
	changes := M{}
	if input.Title != nil {
		title, err := utils.NonEmpty(*input.Title, "title")
		if err != nil {
			return err
		}
		if title != row.Title {
			if err := self.repo.Issue().Update(ctx, tx, id, issue_repo.SetTitle(title)); err != nil {
				return err
			}
			changes["title"] = M{"from": row.Title, "to": title}
		}
		input.Title = nil
	}
	description, err := args.NewContent(row.Description, input.Description, input.Patch)
	if err != nil {
		return err
	}
	input.Description, input.Patch = nil, nil
	if description != nil && *description != row.Description {
		if err := self.repo.Issue().Update(ctx, tx, id, issue_repo.SetDescription(*description)); err != nil {
			return err
		}
		changes["description"] = M{"changed": true}
		previous, err := self.events.Mentions(ctx, actor, row.Description)
		if err != nil {
			return err
		}
		current, err := self.events.Mentions(ctx, actor, *description)
		if err != nil {
			return err
		}
		mentioned := slices.DeleteFunc(current, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
		if err := self.notifyMentions(ctx, tx, actor, id, mentioned); err != nil {
			return err
		}
	}
	if input.State != nil {
		state, err := self.resolve.State(ctx, actor, row.TeamID, *input.State)
		if err != nil {
			return err
		}
		if state != row.StateID {
			if err := self.SetState(ctx, tx, actor, id, state, "updated"); err != nil {
				return err
			}
		}
		input.State = nil
	}
	if len(changes) > 0 {
		if err := self.events.History(ctx, tx, actor, id, "updated", changes); err != nil {
			return err
		}
	}
	if err := self.applyChanges(ctx, tx, actor, id, input, false); err != nil {
		return err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	if err := self.repo.Issue().Update(ctx, tx, id, issue_repo.Touch(now)); err != nil {
		return err
	}
	after, err := self.Value(ctx, tx, id)
	if err != nil {
		return err
	}
	updatedFrom := events_service.ChangedFields(before, after)
	if len(updatedFrom) == 0 {
		return nil
	}
	return self.events.Emit(ctx, tx, actor, "Issue", "update", after, updatedFrom, &row.TeamID)
}

// lockIssue is the issue, locked until the transaction ends.
func (self *IssueService) lockIssue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Issue, error) {
	row, err := self.repo.Issue().GetForUpdate(ctx, tx, id)
	if ent.IsNotFound(err) {
		return nil, errdefs.NotFound("No such issue")
	}
	return row, err
}

func (self *IssueService) notifyMentions(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, issue uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	if err := self.events.Subscribe(ctx, tx, issue, users); err != nil {
		return err
	}
	identifier, err := self.repo.Issue().Identifier(ctx, tx, issue)
	if err != nil {
		return err
	}
	return self.events.Notify(ctx, tx, actor, users, events_service.Notice{Kind: "issueMention", Summary: "You were mentioned in " + identifier, Issue: &issue})
}

// moveTeam moves the issue to another team: a new number (the old identifier keeps resolving), a
// status of the same name or type, and only labels the new team has. Its sub-issues come along.
func (self *IssueService) moveTeam(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, team uuid.UUID) error {
	oldIdentifier, err := self.repo.Issue().Identifier(ctx, tx, row.ID)
	if err != nil {
		return err
	}
	number, err := self.repo.Issue().NextNumber(ctx, tx, team)
	if err != nil {
		return err
	}
	old, err := self.repo.Issue().State(ctx, tx, row.StateID)
	if err != nil {
		return err
	}
	state, err := self.repo.Issue().SameState(ctx, tx, team, old.Name)
	if err != nil {
		return err
	}
	if state == nil {
		if state, err = self.FirstStateOfType(ctx, tx, team, old.Type); err != nil {
			return err
		}
	}
	if state == nil {
		fallback, err := self.DefaultState(ctx, tx, team)
		if err != nil {
			return err
		}
		state = &fallback
	}
	if err := self.repo.Issue().MoveTeam(ctx, tx, row.ID, team, number, *state, oldIdentifier); err != nil {
		return err
	}
	newIdentifier, err := self.repo.Issue().Identifier(ctx, tx, row.ID)
	if err != nil {
		return err
	}
	from, err := self.repo.Issue().Team(ctx, tx, row.TeamID)
	if err != nil {
		return err
	}
	to, err := self.repo.Issue().Team(ctx, tx, team)
	if err != nil {
		return err
	}
	if err := self.events.History(ctx, tx, actor, row.ID, "moved", M{
		"team":       M{"from": models.TeamRefOf(from), "to": models.TeamRefOf(to)},
		"identifier": M{"from": oldIdentifier, "to": newIdentifier},
	}); err != nil {
		return err
	}
	children, err := self.repo.Issue().TeamChildren(ctx, tx, row.ID, row.TeamID)
	if err != nil {
		return err
	}
	for _, child := range children {
		childRow, err := self.lockIssue(ctx, tx, child)
		if err != nil {
			return err
		}
		if err := self.moveTeam(ctx, tx, actor, childRow, team); err != nil {
			return err
		}
	}
	return nil
}
