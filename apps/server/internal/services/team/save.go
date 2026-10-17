package team_service

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/schema/pgarray"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	seed_service "ricrac/internal/services/seed"
)

type SaveInput struct {
	ID                           *string                `json:"id" doc:"Only to change a team: its key, name or id. Omit to create one."`
	Name                         *string                `json:"name" doc:"Required to create."`
	Key                          *string                `json:"key" doc:"1 to 7 letters and digits starting with a letter, like ENG. Defaults to the name's initials."`
	Description                  args.Nullable[string]  `json:"description" doc:"Null clears it."`
	Icon                         args.Nullable[string]  `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color                        args.Nullable[string]  `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Private                      *bool                  `json:"private" doc:"Private teams and their issues are only visible to their members."`
	Parent                       args.Nullable[string]  `json:"parent" doc:"Parent team (key, name or id), making this a sub-team; null makes it a top-level team."`
	Timezone                     *string                `json:"timezone" doc:"IANA timezone like Europe/Berlin, used for cycles."`
	EstimateType                 *string                `json:"estimateType" doc:"notUsed, exponential, fibonacci, linear or tShirt."`
	EstimateAllowZero            *bool                  `json:"estimateAllowZero" doc:"Allow an estimate of 0."`
	EstimateExtended             *bool                  `json:"estimateExtended" doc:"Allow the larger estimates of the scale."`
	DefaultEstimate              args.Nullable[float64] `json:"defaultEstimate" doc:"The estimate new issues get; null for none."`
	CyclesEnabled                *bool                  `json:"cyclesEnabled"`
	CycleDurationWeeks           *int                   `json:"cycleDurationWeeks" doc:"1 to 8."`
	CycleCooldownWeeks           *int                   `json:"cycleCooldownWeeks" doc:"Weeks off between cycles, 0 to 4."`
	CycleStartDay                *int                   `json:"cycleStartDay" doc:"Day cycles start: 0 Sunday, 1 Monday ... 6 Saturday."`
	UpcomingCycleCount           *int                   `json:"upcomingCycleCount" doc:"How many upcoming cycles to create ahead, 1 to 8."`
	CycleAutoAddStarted          *bool                  `json:"cycleAutoAddStarted" doc:"Put issues into the current cycle when they're started."`
	CycleAutoAddCompleted        *bool                  `json:"cycleAutoAddCompleted" doc:"Put issues into the current cycle when they're completed."`
	CycleRollover                *bool                  `json:"cycleRollover" doc:"Move unfinished issues to the next cycle when one ends."`
	TriageEnabled                *bool                  `json:"triageEnabled" doc:"Route issues from other teams and integrations to a Triage status first. Turning it off moves issues in triage to the default status."`
	TriageAction                 *string                `json:"triageAction" doc:"What happens to new triage issues: none, notify (the responsible people) or assign (to the person on duty)."`
	TriageResponsible            []string               `json:"triageResponsible" doc:"People responsible for triage (replaces the list): id, name, email or \"me\"."`
	TriageRotationWeeks          args.Nullable[int]     `json:"triageRotationWeeks" doc:"With a number of weeks, the responsible people take turns, in order, for that many weeks each; null for no rotation."`
	TriageRotationStart          args.Nullable[string]  `json:"triageRotationStart" doc:"YYYY-MM-DD the rotation starts (the first person's first shift); defaults to today."`
	RequirePriorityToLeaveTriage *bool                  `json:"requirePriorityToLeaveTriage" doc:"Issues need a priority before they can leave triage."`
	AutoArchiveMonths            args.Nullable[int]     `json:"autoArchiveMonths" doc:"Archive closed issues after 1, 3, 6, 9 or 12 months; null turns it off."`
	AutoCloseMonths              args.Nullable[int]     `json:"autoCloseMonths" doc:"Close issues not updated for 1, 3, 6, 9 or 12 months; null turns it off."`
	AutoCloseState               args.Nullable[string]  `json:"autoCloseState" doc:"The canceled-type status auto-closed issues go to (name or id); null for the first canceled status."`
	AutoCloseParentIssues        *bool                  `json:"autoCloseParentIssues" doc:"Complete a parent issue when all its sub-issues are done."`
	AutoCloseChildIssues         *bool                  `json:"autoCloseChildIssues" doc:"Close the sub-issues when their parent is closed."`
	DefaultIssueState            *string                `json:"defaultIssueState" doc:"The status new issues start in (a backlog or unstarted status: name or id)."`
	GitHubPROpenedState          args.Nullable[string]  `json:"githubPrOpenedState" doc:"The status issues move to when a GitHub pull request or branch mentioning them opens (name or id); null for the first started status."`
	GitHubPRMergedState          args.Nullable[string]  `json:"githubPrMergedState" doc:"The status issues move to when a pull request that fixes them merges (name or id); null for the first completed status."`
	AddMembers                   []string               `json:"addMembers" doc:"People to add to the team: id, name, email or \"me\"."`
	RemoveMembers                []string               `json:"removeMembers" doc:"People to take off the team."`
}

func (self *TeamService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.TeamDetail, error) {
	if err := actor.RequireMember(); err != nil {
		return models.TeamDetail{}, err
	}
	var value models.TeamDetail
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var team uuid.UUID
		var before *models.TeamDetail
		var err error
		if input.ID != nil {
			if team, err = self.resolve.Team(ctx, actor, *input.ID); err != nil {
				return err
			}
			if err := self.RequireCanEdit(ctx, tx, actor, team); err != nil {
				return err
			}
			if err := self.repo.Team().Lock(ctx, tx, team); err != nil {
				return err
			}
			current, err := self.Value(ctx, tx, actor, team)
			if err != nil {
				return err
			}
			before = &current
		} else if team, err = self.create(ctx, tx, actor, input); err != nil {
			return err
		}
		if err := self.apply(ctx, tx, actor, team, input, before == nil); err != nil {
			return err
		}
		if value, err = self.Value(ctx, tx, actor, team); err != nil {
			return err
		}
		if before == nil {
			return self.events.Emit(ctx, tx, actor, "Team", "create", value, nil, &team)
		}
		updatedFrom := events_service.ChangedFields(*before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.Team().Update(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetUpdatedAt(now) }); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "Team", "update", value, updatedFrom, &team)
	})
	if err != nil {
		return models.TeamDetail{}, err
	}
	return value, nil
}

func (self *TeamService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	key := seed_service.TeamKey(name)
	if input.Key != nil {
		key = *input.Key
	}
	if key, err = seed_service.CheckTeamKey(key); err != nil {
		return uuid.Nil, err
	}
	if err := self.checkNameFree(ctx, tx, actor, nil, name); err != nil {
		return uuid.Nil, err
	}
	if err := self.checkKeyFree(ctx, tx, actor, nil, key); err != nil {
		return uuid.Nil, err
	}
	return self.seed.CreateTeam(ctx, tx, actor.Workspace, actor.User, name, key)
}

func (self *TeamService) checkKeyFree(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team *uuid.UUID, key string) error {
	taken, found, err := self.repo.Team().KeyTakenBy(ctx, tx, actor.Workspace, key, team)
	if err != nil {
		return err
	}
	if found {
		return errdefs.Conflict("The key %s is taken by the team %s; choose another key", key, taken)
	}
	return nil
}

func (self *TeamService) checkNameFree(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team *uuid.UUID, name string) error {
	taken, err := self.repo.Team().NameTaken(ctx, tx, actor.Workspace, name, team)
	if err != nil {
		return err
	}
	if taken {
		return errdefs.BadRequest("There already is a team named %q", name)
	}
	return nil
}

func (self *TeamService) set(ctx context.Context, tx repository.TxInterface, team uuid.UUID, change func(*ent.TeamUpdateOne)) error {
	return self.repo.Team().Update(ctx, tx, team, change)
}

func checkMonths(value *int, what string) (*int, error) {
	if value != nil && !slices.Contains([]int{1, 3, 6, 9, 12}, *value) {
		return nil, errdefs.BadRequest("%s is 1, 3, 6, 9 or 12 (or null for off)", what)
	}
	return value, nil
}

func (self *TeamService) apply(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, input *SaveInput, creating bool) error {
	if !creating {
		if err := self.rename(ctx, tx, actor, team, input); err != nil {
			return err
		}
	}
	if input.Description.Set {
		description := utils.Trimmed(input.Description.Ptr())
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(description, u.SetDescription, u.ClearDescription) }); err != nil {
			return err
		}
	}
	if input.Icon.Set {
		icon := utils.Trimmed(input.Icon.Ptr())
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(icon, u.SetIcon, u.ClearIcon) }); err != nil {
			return err
		}
	}
	if input.Color.Set {
		var color *string
		if text := utils.Trimmed(input.Color.Ptr()); text != nil {
			checked, err := utils.CheckColor(*text)
			if err != nil {
				return err
			}
			color = &checked
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(color, u.SetColor, u.ClearColor) }); err != nil {
			return err
		}
	}
	if input.Private != nil {
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetPrivate(*input.Private) }); err != nil {
			return err
		}
	}
	if input.Parent.Set {
		if err := self.setParent(ctx, tx, actor, team, input.Parent.Ptr()); err != nil {
			return err
		}
	}
	if input.Timezone != nil {
		timezone := strings.TrimSpace(*input.Timezone)
		if err := self.workspaces.CheckTimezone(ctx, tx, timezone); err != nil {
			return err
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetTimezone(timezone) }); err != nil {
			return err
		}
	}
	if input.EstimateType != nil {
		if !slices.Contains([]string{"notUsed", "exponential", "fibonacci", "linear", "tShirt"}, *input.EstimateType) {
			return errdefs.BadRequest("estimateType is notUsed, exponential, fibonacci, linear or tShirt")
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetEstimateType(*input.EstimateType) }); err != nil {
			return err
		}
	}
	for _, flag := range []struct {
		value *bool
		set   func(*ent.TeamUpdateOne, bool) *ent.TeamUpdateOne
	}{
		{input.EstimateAllowZero, (*ent.TeamUpdateOne).SetEstimateAllowZero},
		{input.EstimateExtended, (*ent.TeamUpdateOne).SetEstimateExtended},
		{input.CyclesEnabled, (*ent.TeamUpdateOne).SetCyclesEnabled},
		{input.CycleAutoAddStarted, (*ent.TeamUpdateOne).SetCycleAutoAddStarted},
		{input.CycleAutoAddCompleted, (*ent.TeamUpdateOne).SetCycleAutoAddCompleted},
		{input.CycleRollover, (*ent.TeamUpdateOne).SetCycleRollover},
		{input.RequirePriorityToLeaveTriage, (*ent.TeamUpdateOne).SetRequirePriorityToLeaveTriage},
		{input.AutoCloseParentIssues, (*ent.TeamUpdateOne).SetAutoCloseParentIssues},
		{input.AutoCloseChildIssues, (*ent.TeamUpdateOne).SetAutoCloseChildIssues},
	} {
		if flag.value == nil {
			continue
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { flag.set(u, *flag.value) }); err != nil {
			return err
		}
	}
	if input.DefaultEstimate.Set {
		estimate := input.DefaultEstimate.Ptr()
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(estimate, u.SetDefaultEstimate, u.ClearDefaultEstimate) }); err != nil {
			return err
		}
	}
	for _, number := range []struct {
		value     *int
		low, high int
		what      string
		set       func(*ent.TeamUpdateOne, int) *ent.TeamUpdateOne
	}{
		{input.CycleDurationWeeks, 1, 8, "cycleDurationWeeks", (*ent.TeamUpdateOne).SetCycleDurationWeeks},
		{input.CycleCooldownWeeks, 0, 4, "cycleCooldownWeeks", (*ent.TeamUpdateOne).SetCycleCooldownWeeks},
		{input.CycleStartDay, 0, 6, "cycleStartDay", (*ent.TeamUpdateOne).SetCycleStartDay},
		{input.UpcomingCycleCount, 1, 8, "upcomingCycleCount", (*ent.TeamUpdateOne).SetUpcomingCycleCount},
	} {
		if number.value == nil {
			continue
		}
		value, err := inRange(*number.value, number.low, number.high, number.what)
		if err != nil {
			return err
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { number.set(u, value) }); err != nil {
			return err
		}
	}
	for _, months := range []struct {
		value args.Nullable[int]
		what  string
		set   func(*ent.TeamUpdateOne, int) *ent.TeamUpdateOne
		clear func(*ent.TeamUpdateOne) *ent.TeamUpdateOne
	}{
		{input.AutoArchiveMonths, "autoArchiveMonths", (*ent.TeamUpdateOne).SetAutoArchiveMonths, (*ent.TeamUpdateOne).ClearAutoArchiveMonths},
		{input.AutoCloseMonths, "autoCloseMonths", (*ent.TeamUpdateOne).SetAutoCloseMonths, (*ent.TeamUpdateOne).ClearAutoCloseMonths},
	} {
		if !months.value.Set {
			continue
		}
		value, err := checkMonths(months.value.Ptr(), months.what)
		if err != nil {
			return err
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) {
			setNillable(value, func(v int) *ent.TeamUpdateOne { return months.set(u, v) }, func() *ent.TeamUpdateOne { return months.clear(u) })
		}); err != nil {
			return err
		}
	}
	if input.AutoCloseState.Set {
		if err := self.setAutoCloseState(ctx, tx, team, input.AutoCloseState.Ptr()); err != nil {
			return err
		}
	}
	if input.DefaultIssueState != nil {
		if err := self.setDefaultState(ctx, tx, team, *input.DefaultIssueState); err != nil {
			return err
		}
	}
	for _, setting := range []struct {
		value args.Nullable[string]
		set   func(*ent.TeamUpdateOne, uuid.UUID) *ent.TeamUpdateOne
		clear func(*ent.TeamUpdateOne) *ent.TeamUpdateOne
	}{
		{input.GitHubPROpenedState, (*ent.TeamUpdateOne).SetGithubPrOpenedStateID, (*ent.TeamUpdateOne).ClearGithubPrOpenedStateID},
		{input.GitHubPRMergedState, (*ent.TeamUpdateOne).SetGithubPrMergedStateID, (*ent.TeamUpdateOne).ClearGithubPrMergedStateID},
	} {
		if !setting.value.Set {
			continue
		}
		var state *uuid.UUID
		if setting.value.Valid {
			id, err := self.FindState(ctx, tx, team, setting.value.Value)
			if err != nil {
				return err
			}
			state = &id
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) {
			setNillable(state, func(v uuid.UUID) *ent.TeamUpdateOne { return setting.set(u, v) }, func() *ent.TeamUpdateOne { return setting.clear(u) })
		}); err != nil {
			return err
		}
	}
	if err := self.members(ctx, tx, actor, team, input); err != nil {
		return err
	}
	if err := self.triage(ctx, tx, actor, team, input); err != nil {
		return err
	}
	return self.check(ctx, tx, team)
}

// setNillable sets the value, or clears it when it is nil.
func setNillable[T any](value *T, set func(T) *ent.TeamUpdateOne, clear func() *ent.TeamUpdateOne) {
	if value == nil {
		clear()
		return
	}
	set(*value)
}

func (self *TeamService) rename(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, input *SaveInput) error {
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := self.checkNameFree(ctx, tx, actor, &team, name); err != nil {
			return err
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetName(name) }); err != nil {
			return err
		}
	}
	if input.Key == nil {
		return nil
	}
	key, err := seed_service.CheckTeamKey(*input.Key)
	if err != nil {
		return err
	}
	return self.changeKey(ctx, tx, actor, team, key)
}

// changeKey gives the team a new key. Every issue keeps its old identifier in
// previous_identifiers.
func (self *TeamService) changeKey(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, key string) error {
	current, err := self.repo.Team().Get(ctx, tx, team)
	if err != nil {
		return err
	}
	if current.Key == key {
		return nil
	}
	if err := self.checkKeyFree(ctx, tx, actor, &team, key); err != nil {
		return err
	}
	if err := self.repo.Team().RenameIssues(ctx, tx, team, current.Key); err != nil {
		return err
	}
	return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetKey(key) })
}

func (self *TeamService) setParent(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, reference *string) error {
	if reference == nil {
		return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.ClearParentID() })
	}
	parent, err := self.resolve.Team(ctx, actor, *reference)
	if err != nil {
		return err
	}
	loops, err := self.repo.Team().InOwnTree(ctx, tx, parent, team)
	if err != nil {
		return err
	}
	if loops {
		return errdefs.BadRequest("A team can't be a sub-team of itself or of its own sub-teams")
	}
	return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetParentID(parent) })
}

func (self *TeamService) setAutoCloseState(ctx context.Context, tx repository.TxInterface, team uuid.UUID, reference *string) error {
	var state *uuid.UUID
	if reference != nil {
		id, err := self.FindState(ctx, tx, team, *reference)
		if err != nil {
			return err
		}
		state = &id
	} else {
		var err error
		if state, err = self.issues.FirstStateOfType(ctx, tx, team, "canceled"); err != nil {
			return err
		}
	}
	if state != nil {
		kind, err := self.issues.StateType(ctx, tx, *state)
		if err != nil {
			return err
		}
		if kind != "canceled" {
			return errdefs.BadRequest("autoCloseState must be a canceled-type status")
		}
	}
	return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(state, u.SetAutoCloseStateID, u.ClearAutoCloseStateID) })
}

func (self *TeamService) setDefaultState(ctx context.Context, tx repository.TxInterface, team uuid.UUID, reference string) error {
	state, err := self.FindState(ctx, tx, team, reference)
	if err != nil {
		return err
	}
	kind, err := self.issues.StateType(ctx, tx, state)
	if err != nil {
		return err
	}
	if kind != "backlog" && kind != "unstarted" {
		return errdefs.BadRequest("defaultIssueState must be a backlog or unstarted status, not %s", kind)
	}
	return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetDefaultIssueStateID(state) })
}

// checkCanSee checks that users can see the team: its members, and for a public team every
// member who isn't a guest.
func (self *TeamService) checkCanSee(ctx context.Context, tx repository.TxInterface, team uuid.UUID, users []uuid.UUID) error {
	blind, err := self.repo.Team().Blind(ctx, tx, team, users)
	if err != nil {
		return err
	}
	if len(blind) > 0 {
		return errdefs.BadRequest("%s can't see this team; add them to it first", strings.Join(blind, ", "))
	}
	return nil
}

func (self *TeamService) members(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, input *SaveInput) error {
	if input.AddMembers != nil {
		users, err := self.resolve.Users(ctx, actor, input.AddMembers)
		if err != nil {
			return err
		}
		if err := self.repo.Team().AddMembers(ctx, tx, team, users); err != nil {
			return err
		}
	}
	if input.RemoveMembers == nil {
		return nil
	}
	users, err := self.resolve.Users(ctx, actor, input.RemoveMembers)
	if err != nil {
		return err
	}
	if err := self.repo.Team().RemoveMembers(ctx, tx, team, users); err != nil {
		return err
	}
	return self.repo.Team().DropFromTriage(ctx, tx, team, users)
}

func (self *TeamService) triage(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID, input *SaveInput) error {
	if input.TriageAction != nil {
		if !slices.Contains([]string{"none", "notify", "assign"}, *input.TriageAction) {
			return errdefs.BadRequest("triageAction is none, notify or assign")
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetTriageAction(*input.TriageAction) }); err != nil {
			return err
		}
	}
	if input.TriageResponsible != nil {
		users, err := self.resolve.Users(ctx, actor, input.TriageResponsible)
		if err != nil {
			return err
		}
		if err := self.checkCanSee(ctx, tx, team, users); err != nil {
			return err
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetTriageResponsibleIds(pgarray.UUID(users)) }); err != nil {
			return err
		}
	}
	if input.TriageRotationWeeks.Set {
		var weeks *int
		if input.TriageRotationWeeks.Valid {
			value, err := inRange(input.TriageRotationWeeks.Value, 1, 52, "triageRotationWeeks")
			if err != nil {
				return err
			}
			weeks = &value
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(weeks, u.SetTriageRotationWeeks, u.ClearTriageRotationWeeks) }); err != nil {
			return err
		}
	}
	if input.TriageRotationStart.Set {
		var start *time.Time
		if input.TriageRotationStart.Valid {
			date, err := utils.ParseDate(input.TriageRotationStart.Value)
			if err != nil {
				return err
			}
			start = &date
		}
		if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { setNillable(start, u.SetTriageRotationStart, u.ClearTriageRotationStart) }); err != nil {
			return err
		}
	}
	if err := self.startRotation(ctx, tx, team); err != nil {
		return err
	}
	if input.TriageEnabled == nil {
		return nil
	}
	if *input.TriageEnabled {
		return self.enableTriage(ctx, tx, team)
	}
	return self.disableTriage(ctx, tx, actor, team)
}

// startRotation starts a rotation without a start today.
func (self *TeamService) startRotation(ctx context.Context, tx repository.TxInterface, team uuid.UUID) error {
	current, err := self.repo.Team().Get(ctx, tx, team)
	if err != nil {
		return err
	}
	if current.TriageRotationWeeks == nil || current.TriageRotationStart != nil {
		return nil
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	today := now.UTC().Truncate(24 * time.Hour)
	return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetTriageRotationStart(today) })
}

func (self *TeamService) enableTriage(ctx context.Context, tx repository.TxInterface, team uuid.UUID) error {
	archived, found, err := self.repo.Team().ArchivedTriageState(ctx, tx, team)
	if err != nil {
		return err
	}
	if found {
		if err := self.repo.Team().Unarchive(ctx, tx, archived); err != nil {
			return err
		}
	}
	if _, err := self.seed.EnsureTriageState(ctx, tx, team); err != nil {
		return err
	}
	return self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetTriageEnabled(true) })
}

// disableTriage turns triage off and moves the issues waiting in it to the default status.
func (self *TeamService) disableTriage(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, team uuid.UUID) error {
	if err := self.set(ctx, tx, team, func(u *ent.TeamUpdateOne) { u.SetTriageEnabled(false) }); err != nil {
		return err
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	triage, err := self.repo.Team().ArchiveTriageStates(ctx, tx, team, now)
	if err != nil {
		return err
	}
	waiting, err := self.repo.Team().IssuesInStates(ctx, tx, triage)
	if err != nil || len(waiting) == 0 {
		return err
	}
	target, err := self.issues.DefaultState(ctx, tx, team)
	if err != nil {
		return err
	}
	for _, issue := range waiting {
		if err := self.issues.SetState(ctx, tx, actor, issue, target, "updated"); err != nil {
			return err
		}
	}
	return nil
}

const privateNeedsMember = "A private team needs at least one member"

// check checks what must hold after any change: private teams keep a member, the default
// estimate is on the scale.
func (self *TeamService) check(ctx context.Context, tx repository.TxInterface, team uuid.UUID) error {
	t, err := self.repo.Team().Get(ctx, tx, team)
	if err != nil {
		return err
	}
	members, err := self.repo.Team().MemberCounts(ctx, tx, []uuid.UUID{team})
	if err != nil {
		return err
	}
	if t.Private && members[team] == 0 {
		return errdefs.BadRequest(privateNeedsMember)
	}
	if t.DefaultEstimate == nil || t.EstimateType == "notUsed" {
		return nil
	}
	allowed := issue_service.AllowedEstimates(t.EstimateType, t.EstimateAllowZero, t.EstimateExtended)
	if slices.Contains(allowed, *t.DefaultEstimate) {
		return nil
	}
	list := make([]string, len(allowed))
	for i, v := range allowed {
		list[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return errdefs.BadRequest("defaultEstimate %s isn't on the %s scale; use one of %s",
		strconv.FormatFloat(*t.DefaultEstimate, 'f', -1, 64), t.EstimateType, strings.Join(list, ", "))
}
