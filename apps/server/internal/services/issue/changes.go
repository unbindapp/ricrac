package issue_service

import (
	"context"
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
	events_service "ricrac/internal/services/events"
)

// resolveOptional resolves a nullable reference: nil when null or a null word.
func resolveOptional(value args.Nullable[string], resolve func(string) (uuid.UUID, error)) (*uuid.UUID, error) {
	if !value.Valid || isNullWord(value.Value) {
		return nil, nil
	}
	id, err := resolve(value.Value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func samePtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func dateText(date *time.Time) *string {
	if date == nil {
		return nil
	}
	return new(date.Format(time.DateOnly))
}

// applyChanges applies the fields that work the same when creating and updating.
func (self *IssueService) applyChanges(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput, creating bool) error {
	row, err := self.lockIssue(ctx, tx, id)
	if err != nil {
		return err
	}
	changes := M{}
	team, err := self.repo.Issue().Team(ctx, tx, row.TeamID)
	if err != nil {
		return err
	}
	update := func(change issue_repo.Change) error { return self.repo.Issue().Update(ctx, tx, id, change) }
	if input.Priority != nil {
		priority, err := utils.CheckPriority(*input.Priority)
		if err != nil {
			return err
		}
		if priority != row.Priority {
			if err := update(issue_repo.SetPriority(priority)); err != nil {
				return err
			}
			changes["priority"] = M{"from": row.Priority, "to": priority}
		}
	}
	if input.Estimate.Set {
		var estimate *float64
		if input.Estimate.Valid {
			e, err := CheckEstimate(team, input.Estimate.Value)
			if err != nil {
				return err
			}
			estimate = &e
		}
		if !samePtr(estimate, row.Estimate) {
			if err := update(issue_repo.SetEstimate(estimate)); err != nil {
				return err
			}
			changes["estimate"] = M{"from": row.Estimate, "to": estimate}
		}
	}
	if input.Assignee.Set {
		if err := self.applyAssignee(ctx, tx, actor, row, input.Assignee, changes); err != nil {
			return err
		}
	}
	if creating {
		mentioned, err := self.events.Mentions(ctx, actor, row.Description)
		if err != nil {
			return err
		}
		if err := self.notifyMentions(ctx, tx, actor, id, mentioned); err != nil {
			return err
		}
	}
	if err := self.applyLabels(ctx, tx, actor, row, input, changes); err != nil {
		return err
	}
	project := row.ProjectID
	if input.Project.Set {
		if project, err = resolveOptional(input.Project, func(v string) (uuid.UUID, error) { return self.resolve.Project(ctx, actor, v) }); err != nil {
			return err
		}
		if !samePtr(project, row.ProjectID) {
			if err := update(issue_repo.SetProject(project)); err != nil {
				return err
			}
			if project != nil {
				if err := self.repo.Issue().AddProjectTeam(ctx, tx, *project, row.TeamID); err != nil {
					return err
				}
			}
			from, err := self.repo.Issue().Project(ctx, tx, row.ProjectID)
			if err != nil {
				return err
			}
			to, err := self.repo.Issue().Project(ctx, tx, project)
			if err != nil {
				return err
			}
			changes["project"] = M{"from": models.ProjectRefOf(from), "to": models.ProjectRefOf(to)}
		}
	}
	if input.Milestone.Set {
		if err := self.applyMilestone(ctx, tx, actor, row, project, input.Milestone, changes); err != nil {
			return err
		}
	}
	if input.Cycle.Set {
		if err := self.applyCycle(ctx, tx, actor, row, input.Cycle, changes); err != nil {
			return err
		}
	}
	if input.ParentID.Set {
		if err := self.applyParent(ctx, tx, actor, row, input.ParentID, changes); err != nil {
			return err
		}
	}
	if input.DueDate.Set {
		var due *time.Time
		if input.DueDate.Valid && !isNullWord(input.DueDate.Value) {
			date, err := utils.ParseDate(input.DueDate.Value)
			if err != nil {
				return err
			}
			due = &date
		}
		from, to := dateText(row.DueDate), dateText(due)
		if !samePtr(to, from) {
			if err := update(issue_repo.SetDueDate(due)); err != nil {
				return err
			}
			changes["dueDate"] = M{"from": from, "to": to}
		}
	}
	if input.SnoozedUntil.Set {
		var until *time.Time
		if input.SnoozedUntil.Valid {
			at, err := utils.ParseDatetime(input.SnoozedUntil.Value)
			if err != nil {
				return err
			}
			until = &at
		}
		if err := update(issue_repo.SetSnooze(until, actor.User)); err != nil {
			return err
		}
		changes["snoozedUntil"] = M{"from": row.SnoozedUntil, "to": until}
	}
	if input.SLABreachesAt.Set {
		var breaches *time.Time
		if input.SLABreachesAt.Valid {
			at, err := utils.ParseDatetime(input.SLABreachesAt.Value)
			if err != nil {
				return err
			}
			breaches = &at
		}
		slaType := "all"
		if input.SLAType != nil {
			switch *input.SLAType {
			case "all", "onlyBusinessDays":
				slaType = *input.SLAType
			default:
				return errdefs.BadRequest("slaType is \"all\" or \"onlyBusinessDays\", not %q", *input.SLAType)
			}
		}
		var startedAt *time.Time
		if row.SLAStartedAt == nil {
			now, err := self.repo.Now(ctx, tx)
			if err != nil {
				return err
			}
			startedAt = &now
		}
		if err := update(issue_repo.SetSLA(breaches, slaType, startedAt)); err != nil {
			return err
		}
		changes["slaBreachesAt"] = M{"from": row.SLABreachesAt, "to": breaches}
	} else if input.SLAType != nil {
		return errdefs.BadRequest("Set slaType together with slaBreachesAt")
	}
	if input.SortOrder != nil {
		if err := update(issue_repo.SetSortOrder(*input.SortOrder)); err != nil {
			return err
		}
	}
	if input.SubIssueSortOrder != nil {
		if err := update(issue_repo.SetSubIssueSortOrder(*input.SubIssueSortOrder)); err != nil {
			return err
		}
	}
	if err := self.applyRelations(ctx, tx, actor, id, input, changes); err != nil {
		return err
	}
	for _, link := range input.Links {
		if _, err := self.attachments.Upsert(ctx, tx, actor, id, link.URL, link.Title, nil, nil, nil); err != nil {
			return err
		}
	}
	if input.AddSubscribers != nil {
		users, err := self.resolve.Users(ctx, actor, input.AddSubscribers)
		if err != nil {
			return err
		}
		if err := self.events.Subscribe(ctx, tx, id, users); err != nil {
			return err
		}
	}
	if input.RemoveSubscribers != nil {
		users, err := self.resolve.Users(ctx, actor, input.RemoveSubscribers)
		if err != nil {
			return err
		}
		if err := self.repo.Issue().Unsubscribe(ctx, tx, id, users); err != nil {
			return err
		}
	}
	if err := self.applyReleases(ctx, tx, actor, id, input); err != nil {
		return err
	}
	if len(changes) > 0 && !creating {
		return self.events.History(ctx, tx, actor, id, "updated", changes)
	}
	return nil
}

func (self *IssueService) applyAssignee(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, value args.Nullable[string], changes M) error {
	id := row.ID
	assignee, err := resolveOptional(value, func(v string) (uuid.UUID, error) { return self.resolve.User(ctx, actor, v) })
	if err != nil {
		return err
	}
	if samePtr(assignee, row.AssigneeID) {
		return nil
	}
	if assignee != nil {
		ok, err := self.repo.Issue().CanBeAssigned(ctx, tx, *assignee, row.TeamID, id)
		if err != nil {
			return err
		}
		if !ok {
			return errdefs.BadRequest("That person can't see this team's issues; add them to the team or share the issue first")
		}
	}
	if err := self.repo.Issue().Update(ctx, tx, id, issue_repo.SetAssignee(assignee)); err != nil {
		return err
	}
	from, err := self.repo.Issue().User(ctx, tx, row.AssigneeID)
	if err != nil {
		return err
	}
	to, err := self.repo.Issue().User(ctx, tx, assignee)
	if err != nil {
		return err
	}
	changes["assignee"] = M{"from": models.UserRefOf(from), "to": models.UserRefOf(to)}
	identifier, err := self.repo.Issue().Identifier(ctx, tx, id)
	if err != nil {
		return err
	}
	if assignee != nil {
		if err := self.events.Subscribe(ctx, tx, id, []uuid.UUID{*assignee}); err != nil {
			return err
		}
		notice := events_service.Notice{Kind: "issueAssignedToYou", Summary: identifier + " was assigned to you", Issue: &id}
		if err := self.events.Notify(ctx, tx, actor, []uuid.UUID{*assignee}, notice); err != nil {
			return err
		}
	}
	if row.AssigneeID == nil {
		return nil
	}
	notice := events_service.Notice{Kind: "issueUnassignedFromYou", Summary: identifier + " was unassigned from you", Issue: &id}
	return self.events.Notify(ctx, tx, actor, []uuid.UUID{*row.AssigneeID}, notice)
}

func (self *IssueService) applyMilestone(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, project *uuid.UUID, value args.Nullable[string], changes M) error {
	milestone, err := resolveOptional(value, func(v string) (uuid.UUID, error) { return self.resolve.Milestone(ctx, actor, project, v) })
	if err != nil {
		return err
	}
	var milestoneProject *uuid.UUID
	found, err := self.repo.Issue().Milestone(ctx, tx, milestone)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if found != nil {
		milestoneProject = &found.ProjectID
	}
	if milestoneProject != nil && project != nil && *milestoneProject != *project {
		return errdefs.BadRequest("That milestone belongs to another project than the issue's")
	}
	if err := self.repo.Issue().Update(ctx, tx, row.ID, issue_repo.SetMilestone(milestone, milestoneProject)); err != nil {
		return err
	}
	if samePtr(milestone, row.MilestoneID) {
		return nil
	}
	from, err := self.repo.Issue().Milestone(ctx, tx, row.MilestoneID)
	if err != nil {
		return err
	}
	changes["projectMilestone"] = M{"from": models.MilestoneRefOf(from), "to": models.MilestoneRefOf(found)}
	return nil
}

func (self *IssueService) applyCycle(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, value args.Nullable[string], changes M) error {
	cycle, err := resolveOptional(value, func(v string) (uuid.UUID, error) { return self.resolve.Cycle(ctx, actor, &row.TeamID, v) })
	if err != nil {
		return err
	}
	to, err := self.repo.Issue().Cycle(ctx, tx, cycle)
	if err != nil {
		return err
	}
	if to != nil && to.TeamID != row.TeamID {
		return errdefs.BadRequest("That cycle belongs to another team")
	}
	if samePtr(cycle, row.CycleID) {
		return nil
	}
	if err := self.repo.Issue().Update(ctx, tx, row.ID, issue_repo.SetCycle(cycle)); err != nil {
		return err
	}
	from, err := self.repo.Issue().Cycle(ctx, tx, row.CycleID)
	if err != nil {
		return err
	}
	changes["cycle"] = M{"from": models.CycleRefOf(from), "to": models.CycleRefOf(to)}
	return nil
}

func (self *IssueService) applyParent(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, value args.Nullable[string], changes M) error {
	parent, err := resolveOptional(value, func(v string) (uuid.UUID, error) { return self.resolve.Issue(ctx, actor, v) })
	if err != nil {
		return err
	}
	if parent != nil {
		loops, err := self.repo.Issue().InOwnTree(ctx, tx, *parent, row.ID)
		if err != nil {
			return err
		}
		if loops {
			return errdefs.BadRequest("An issue can't be a sub-issue of itself or of its own sub-issues")
		}
	}
	if samePtr(parent, row.ParentID) {
		return nil
	}
	var order *float64
	if parent != nil {
		next, err := self.repo.Issue().NextSubIssueSortOrder(ctx, tx, *parent)
		if err != nil {
			return err
		}
		order = &next
	}
	if err := self.repo.Issue().Update(ctx, tx, row.ID, issue_repo.SetParent(parent, order)); err != nil {
		return err
	}
	from, err := self.repo.Issue().Ref(ctx, tx, row.ParentID)
	if err != nil {
		return err
	}
	to, err := self.repo.Issue().Ref(ctx, tx, parent)
	if err != nil {
		return err
	}
	changes["parent"] = M{"from": models.IssueRefOf(from), "to": models.IssueRefOf(to)}
	return nil
}

func (self *IssueService) applyLabels(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, row *ent.Issue, input SaveInput, changes M) error {
	if input.Labels != nil && (input.AddLabels != nil || input.RemoveLabels != nil) {
		return errdefs.BadRequest("Pass either labels or addLabels/removeLabels, not both")
	}
	if input.Labels == nil && input.AddLabels == nil && input.RemoveLabels == nil {
		return nil
	}
	current, err := self.repo.Issue().LabelIDs(ctx, tx, row.ID)
	if err != nil {
		return err
	}
	next := slices.Clone(current)
	if input.Labels != nil {
		if next, err = self.resolve.Labels(ctx, actor, &row.TeamID, input.Labels); err != nil {
			return err
		}
	}
	if input.AddLabels != nil {
		added, err := self.resolve.Labels(ctx, actor, &row.TeamID, input.AddLabels)
		if err != nil {
			return err
		}
		labels, err := self.repo.Issue().LabelsWithGroups(ctx, tx, added)
		if err != nil {
			return err
		}
		for _, label := range labels {
			if label.ParentID != nil && label.Edges.Parent != nil && label.Edges.Parent.GroupType == "singleSelect" {
				siblings, err := self.repo.Issue().Siblings(ctx, tx, *label.ParentID, label.ID)
				if err != nil {
					return err
				}
				next = slices.DeleteFunc(next, func(l uuid.UUID) bool { return slices.Contains(siblings, l) })
			}
			next = utils.AppendUnique(next, label.ID)
		}
	}
	if input.RemoveLabels != nil {
		removed, err := self.resolve.Labels(ctx, actor, &row.TeamID, input.RemoveLabels)
		if err != nil {
			return err
		}
		next = slices.DeleteFunc(next, func(l uuid.UUID) bool { return slices.Contains(removed, l) })
	}
	clashes, err := self.repo.Issue().SingleSelectClashes(ctx, tx, next)
	if err != nil {
		return err
	}
	if len(clashes) > 0 {
		return errdefs.BadRequest("An issue takes one label of the group %q", clashes[0])
	}
	sortedCurrent := slices.SortedFunc(slices.Values(current), compareUUID)
	sortedNext := slices.SortedFunc(slices.Values(next), compareUUID)
	if slices.Equal(sortedCurrent, sortedNext) {
		return nil
	}
	if next == nil {
		next = []uuid.UUID{}
	}
	if err := self.repo.Issue().SetLabels(ctx, tx, row.ID, next); err != nil {
		return err
	}
	from, err := self.repo.Issue().LabelNames(ctx, tx, current)
	if err != nil {
		return err
	}
	to, err := self.repo.Issue().LabelNames(ctx, tx, next)
	if err != nil {
		return err
	}
	changes["labels"] = M{"from": from, "to": to}
	return nil
}

func compareUUID(a, b uuid.UUID) int {
	return strings.Compare(a.String(), b.String())
}

type relationList struct {
	list    []string
	kind    string
	reverse bool
}

func (self *IssueService) applyRelations(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput, changes M) error {
	additions := []relationList{{input.Blocks, "blocks", false}, {input.BlockedBy, "blocks", true}, {input.RelatedTo, "related", false}}
	for _, add := range additions {
		for _, reference := range add.list {
			other, err := self.resolve.Issue(ctx, actor, reference)
			if err != nil {
				return err
			}
			if other == id {
				return errdefs.BadRequest("An issue can't be related to itself")
			}
			from, to := id, other
			if add.reverse {
				from, to = other, id
			}
			if add.kind == "related" {
				exists, err := self.repo.Issue().RelatedExists(ctx, tx, from, to)
				if err != nil {
					return err
				}
				if exists {
					continue
				}
			}
			if _, err := self.repo.Issue().Relate(ctx, tx, from, to, add.kind, actor.User); err != nil {
				return err
			}
			label := "relatedTo"
			if add.reverse {
				label = "blockedBy"
			} else if add.kind == "blocks" {
				label = "blocks"
			}
			otherIdentifier, err := self.repo.Issue().Identifier(ctx, tx, other)
			if err != nil {
				return err
			}
			key := "added_" + label
			list, _ := changes[key].([]any)
			changes[key] = append(list, otherIdentifier)
		}
	}
	removals := []relationList{{input.RemoveBlocks, "blocks", false}, {input.RemoveBlockedBy, "blocks", true}, {input.RemoveRelatedTo, "related", false}}
	for _, remove := range removals {
		for _, reference := range remove.list {
			other, err := self.resolve.Issue(ctx, actor, reference)
			if err != nil {
				return err
			}
			from, to := id, other
			if remove.reverse {
				from, to = other, id
			}
			if err := self.repo.Issue().Unrelate(ctx, tx, from, to, remove.kind); err != nil {
				return err
			}
		}
	}
	if !input.DuplicateOf.Set {
		return nil
	}
	if err := self.repo.Issue().RemoveDuplicateOf(ctx, tx, id); err != nil {
		return err
	}
	if !input.DuplicateOf.Valid || isNullWord(input.DuplicateOf.Value) {
		return nil
	}
	other, err := self.resolve.Issue(ctx, actor, input.DuplicateOf.Value)
	if err != nil {
		return err
	}
	if other == id {
		return errdefs.BadRequest("An issue can't duplicate itself")
	}
	if err := self.repo.Issue().MarkDuplicate(ctx, tx, id, other, actor.User); err != nil {
		return err
	}
	row, err := self.lockIssue(ctx, tx, id)
	if err != nil {
		return err
	}
	kind, err := self.StateType(ctx, tx, row.StateID)
	if err != nil {
		return err
	}
	if kind != "completed" && kind != "canceled" {
		canceled, err := self.FirstStateOfType(ctx, tx, row.TeamID, "canceled")
		if err != nil {
			return err
		}
		if canceled != nil {
			if err := self.SetState(ctx, tx, actor, id, *canceled, "marked_duplicate"); err != nil {
				return err
			}
		}
	}
	otherIdentifier, err := self.repo.Issue().Identifier(ctx, tx, other)
	if err != nil {
		return err
	}
	changes["duplicateOf"] = otherIdentifier
	return nil
}

func (self *IssueService) applyReleases(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input SaveInput) error {
	if input.SetReleases != nil && (input.AddReleases != nil || input.RemoveReleases != nil) {
		return errdefs.BadRequest("Pass either setReleases or addReleases/removeReleases, not both")
	}
	if input.SetReleases != nil {
		ids, err := self.resolve.Releases(ctx, actor, input.SetReleases)
		if err != nil {
			return err
		}
		if err := self.repo.Issue().ClearReleases(ctx, tx, id); err != nil {
			return err
		}
		if err := self.repo.Issue().AddReleases(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if input.AddReleases != nil {
		ids, err := self.resolve.Releases(ctx, actor, input.AddReleases)
		if err != nil {
			return err
		}
		if err := self.repo.Issue().AddReleases(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	if input.RemoveReleases != nil {
		ids, err := self.resolve.Releases(ctx, actor, input.RemoveReleases)
		if err != nil {
			return err
		}
		if err := self.repo.Issue().RemoveReleases(ctx, tx, id, ids); err != nil {
			return err
		}
	}
	return nil
}
