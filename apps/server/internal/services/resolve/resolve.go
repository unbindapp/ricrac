// Package resolve_service turns what agents pass (ids, names, keys, identifiers like ENG-123,
// "me", links) into ids, checking the caller may see the thing. Errors list what would have
// matched.
package resolve_service

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/integrations/github"
	"ricrac/internal/repositories/repositories"
	resolve_repo "ricrac/internal/repositories/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ResolveServiceInterface -p resolve_service -s ResolveService -o resolve_service_iface.go
type ResolveService struct {
	repo repositories.RepositoriesInterface
}

func NewResolveService(repo repositories.RepositoriesInterface) *ResolveService {
	return &ResolveService{repo: repo}
}

func (self *ResolveService) lookups() resolve_repo.ResolveRepositoryInterface {
	return self.repo.Resolve()
}

// all resolves each value with one, without repeats.
func all(ctx context.Context, actor *auth.Actor, values []string, one func(context.Context, *auth.Actor, string) (uuid.UUID, error)) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	for _, value := range values {
		id, err := one(ctx, actor, value)
		if err != nil {
			return nil, err
		}
		ids = utils.AppendUnique(ids, id)
	}
	return ids, nil
}

// User is a member of the workspace by id, email, display name, name, the part of their email
// before the @, first name, or "me".
func (self *ResolveService) User(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "me") {
		return actor.User, nil
	}
	rows, err := self.lookups().Members(ctx, actor.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	needle := strings.ToLower(strings.TrimPrefix(value, "@"))
	id, isID := utils.AsUUID(value)
	tests := []func(resolve_repo.Member) bool{
		func(r resolve_repo.Member) bool { return isID && r.ID == id },
		func(r resolve_repo.Member) bool { return strings.ToLower(r.Email) == needle },
		func(r resolve_repo.Member) bool { return strings.ToLower(r.DisplayName) == needle },
		func(r resolve_repo.Member) bool { return strings.ToLower(r.Name) == needle },
		func(r resolve_repo.Member) bool {
			local, _, _ := strings.Cut(r.Email, "@")
			return strings.ToLower(local) == needle
		},
		func(r resolve_repo.Member) bool {
			first := strings.Fields(strings.ToLower(r.Name))
			return len(first) > 0 && first[0] == needle
		},
	}
	describe := func(rows []resolve_repo.Member) []string {
		names := make([]string, len(rows))
		for i, r := range rows {
			names[i] = fmt.Sprintf("%s <%s>", r.Name, r.Email)
		}
		return names
	}
	for _, test := range tests {
		var matches []resolve_repo.Member
		for _, r := range rows {
			if test(r) {
				matches = append(matches, r)
			}
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0].ID, nil
		default:
			return uuid.Nil, errdefs.BadRequest("%q matches more than one person; use their email. %s", value, utils.Options(describe(matches)))
		}
	}
	return uuid.Nil, errdefs.BadRequest("No member of this workspace matches %q. %s", value, utils.Options(describe(rows)))
}

func (self *ResolveService) Users(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	return all(ctx, actor, values, self.User)
}

// Team is a team the caller can see, by id, key or name, or by the id, key or name of a team that
// was merged into it or a key or name it had before.
func (self *ResolveService) Team(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	rows, err := self.lookups().Teams(ctx, visible)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	id, isID := utils.AsUUID(value)
	for _, match := range []func(resolve_repo.Keyed) bool{
		func(r resolve_repo.Keyed) bool { return isID && r.ID == id },
		func(r resolve_repo.Keyed) bool { return strings.EqualFold(r.Key, value) },
		func(r resolve_repo.Keyed) bool { return strings.EqualFold(r.Name, value) },
		func(r resolve_repo.Keyed) bool { return isID && slices.Contains(r.MergedIDs, id) },
		func(r resolve_repo.Keyed) bool {
			return slices.ContainsFunc(r.PreviousKeys, func(key string) bool { return strings.EqualFold(key, value) })
		},
		func(r resolve_repo.Keyed) bool {
			return slices.ContainsFunc(r.PreviousNames, func(name string) bool { return strings.EqualFold(name, value) })
		},
	} {
		for _, r := range rows {
			if match(r) {
				return r.ID, nil
			}
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = fmt.Sprintf("%s (%s)", r.Key, r.Name)
	}
	return uuid.Nil, errdefs.BadRequest("No team matches %q. %s", value, utils.Options(names))
}

// OptionalTeam resolves a team when one is named.
func (self *ResolveService) OptionalTeam(ctx context.Context, actor *auth.Actor, value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := self.Team(ctx, actor, *value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// DefaultTeam is the workspace's team, for arguments that name none.
func (self *ResolveService) DefaultTeam(ctx context.Context, actor *auth.Actor) (uuid.UUID, error) {
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	rows, err := self.lookups().Teams(ctx, visible)
	if err != nil {
		return uuid.Nil, err
	}
	switch len(rows) {
	case 1:
		return rows[0].ID, nil
	case 0:
		return uuid.Nil, errdefs.Forbidden("You aren't on the workspace's team; ask an admin to add you")
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.Key
	}
	slices.Sort(keys)
	return uuid.Nil, errdefs.BadRequest("Name the team: %s", strings.Join(keys, ", "))
}

// TeamOrDefault is the team named, or the workspace's team when value is nil or blank.
func (self *ResolveService) TeamOrDefault(ctx context.Context, actor *auth.Actor, value *string) (uuid.UUID, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return self.DefaultTeam(ctx, actor)
	}
	return self.Team(ctx, actor, *value)
}

// CurrentNames rewrites what a filter calls a team, status or label by an earlier name to what
// it is called now: the ids of those merged into others, and the earlier keys and names of teams.
func (self *ResolveService) CurrentNames(ctx context.Context, actor *auth.Actor, filter any) (any, error) {
	aliases, err := self.lookups().Aliases(ctx, actor.Workspace)
	if err != nil || len(aliases.IDs)+len(aliases.Keys)+len(aliases.Names) == 0 {
		return filter, err
	}
	return renamed(filter, "", aliases), nil
}

func renamed(value any, field string, aliases resolve_repo.Aliases) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, inner := range v {
			switch {
			case (field == "team" || field == "teams") && key == "key":
				out[key] = replaced(inner, func(s string) string { return aliases.Keys[strings.ToUpper(s)] })
			case (field == "team" || field == "teams") && key == "name":
				out[key] = replaced(inner, func(s string) string { return aliases.Names[strings.ToLower(s)] })
			default:
				out[key] = renamed(inner, key, aliases)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, inner := range v {
			out[i] = renamed(inner, field, aliases)
		}
		return out
	case string:
		if current, ok := aliases.IDs[strings.ToLower(v)]; ok {
			return current
		}
	}
	return value
}

// replaced is value with every string current names replaced by it.
func replaced(value any, current func(string) string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, inner := range v {
			out[key] = replaced(inner, current)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, inner := range v {
			out[i] = replaced(inner, current)
		}
		return out
	case string:
		if now := current(v); now != "" {
			return now
		}
	}
	return value
}

// FindTyped is the row with this id, name or type, or one the extra tests pick.
func FindTyped(rows []resolve_repo.Typed, value string, extra ...func(resolve_repo.Typed) bool) (uuid.UUID, bool) {
	id, isID := utils.AsUUID(value)
	tests := append([]func(resolve_repo.Typed) bool{
		func(r resolve_repo.Typed) bool { return isID && r.ID == id },
		func(r resolve_repo.Typed) bool { return strings.EqualFold(r.Name, value) },
		func(r resolve_repo.Typed) bool { return strings.EqualFold(r.Type, value) },
		func(r resolve_repo.Typed) bool { return isID && slices.Contains(r.MergedIDs, id) },
	}, extra...)
	for _, test := range tests {
		for _, r := range rows {
			if test(r) {
				return r.ID, true
			}
		}
	}
	return uuid.Nil, false
}

// TypedNames is each row as "Name (type)".
func TypedNames(rows []resolve_repo.Typed) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = fmt.Sprintf("%s (%s)", r.Name, r.Type)
	}
	return names
}

// State is a workflow state of the team, by id, name or type (the first of that type).
func (self *ResolveService) State(ctx context.Context, actor *auth.Actor, team uuid.UUID, value string) (uuid.UUID, error) {
	rows, err := self.lookups().States(ctx, team)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	id, found := FindTyped(rows, value,
		func(r resolve_repo.Typed) bool { return strings.EqualFold(value, "todo") && r.Type == "unstarted" },
		func(r resolve_repo.Typed) bool { return strings.EqualFold(value, "done") && r.Type == "completed" },
	)
	if found {
		return id, nil
	}
	return uuid.Nil, errdefs.BadRequest("The team has no status %q. %s", value, utils.Options(TypedNames(rows)))
}

// Issue is an issue the caller can see, by id, identifier (current or earlier) or link. Trashed
// issues are found too.
func (self *ResolveService) Issue(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	raw := strings.TrimSpace(value)
	candidate := raw
	if strings.Contains(raw, "://") {
		candidate = utils.LastSegment(raw)
		for _, segment := range strings.Split(raw, "/") {
			if _, _, ok := utils.ParseIdentifier(segment); ok {
				candidate = segment
				break
			}
		}
	}
	notFound := errdefs.NotFound("No issue %q; use an identifier like ENG-123, an id or a synced GitHub issue (owner/repo#123)", raw)
	var id uuid.UUID
	var found bool
	var err error
	if repo, number, ok := github.ParseIssue(raw); ok {
		id, found, err = self.lookups().IssueSyncedAs(ctx, actor.Workspace, repo, number)
	} else if uid, ok := utils.AsUUID(candidate); ok {
		id, found, err = self.lookups().IssueByID(ctx, actor.Workspace, uid)
	} else if key, number, ok := utils.ParseIdentifier(candidate); ok {
		id, found, err = self.lookups().IssueByNumber(ctx, actor.Workspace, key, number)
		if err == nil && !found {
			id, found, err = self.lookups().IssueByPreviousIdentifier(ctx, actor.Workspace, fmt.Sprintf("%s-%d", key, number))
		}
	}
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, notFound
	}
	visible, err := self.CanSeeIssue(ctx, actor, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, notFound
	}
	return id, nil
}

// CanSeeIssue says whether the issue is in a team the caller sees or shared with them.
func (self *ResolveService) CanSeeIssue(ctx context.Context, actor *auth.Actor, issue uuid.UUID) (bool, error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return false, err
	}
	return self.lookups().IssueVisible(ctx, issue, teams, actor.User, !actor.Visitor)
}

func (self *ResolveService) Issues(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	return all(ctx, actor, values, self.Issue)
}

// LabelMatches says whether value names the label: its id (or one merged into it), its name, or
// Group/Name.
func LabelMatches(id uuid.UUID, merged []uuid.UUID, name string, parent *string, value string) bool {
	if uid, ok := utils.AsUUID(value); ok && (uid == id || slices.Contains(merged, uid)) {
		return true
	}
	group, wanted := utils.SplitGroup(value)
	if !strings.EqualFold(name, wanted) {
		return false
	}
	return group == nil || parent != nil && strings.EqualFold(*parent, *group)
}

// Labels is issue labels by id or name, among the workspace's labels and the team's (the
// workspace's team when nil). A child of a group can be named Group/Child.
func (self *ResolveService) Labels(ctx context.Context, actor *auth.Actor, team *uuid.UUID, values []string) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if len(values) == 0 {
		return ids, nil
	}
	if team == nil {
		if fallback, err := self.DefaultTeam(ctx, actor); err == nil {
			team = &fallback
		}
	}
	rows, err := self.lookups().IssueLabels(ctx, actor.Workspace, team)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		var matches []resolve_repo.LabelRow
		for _, r := range rows {
			if !r.IsGroup && LabelMatches(r.ID, r.MergedIDs, r.Name, r.Parent, value) {
				matches = append(matches, r)
			}
		}
		var chosen resolve_repo.LabelRow
		switch len(matches) {
		case 0:
			var names []string
			for _, r := range rows {
				if r.IsGroup {
					continue
				}
				if r.Parent != nil {
					names = append(names, *r.Parent+"/"+r.Name)
				} else {
					names = append(names, r.Name)
				}
			}
			return nil, errdefs.BadRequest("No label %q. %s", value, utils.Options(names))
		case 1:
			chosen = matches[0]
		default:
			var teamLabels []resolve_repo.LabelRow
			for _, r := range matches {
				if r.TeamID != nil {
					teamLabels = append(teamLabels, r)
				}
			}
			if len(teamLabels) != 1 {
				_, name := utils.SplitGroup(value)
				return nil, errdefs.BadRequest("%q matches labels in several groups; name it as Group/%s", value, name)
			}
			chosen = teamLabels[0]
		}
		ids = utils.AppendUnique(ids, chosen.ID)
	}
	return ids, nil
}

// Project is a project by id, slug, link or name.
func (self *ResolveService) Project(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	rows, err := self.lookups().Projects(ctx, actor.Workspace, teams)
	if err != nil {
		return uuid.Nil, err
	}
	visible := func(id uuid.UUID) bool {
		return slices.ContainsFunc(rows, func(r resolve_repo.Named) bool { return r.ID == id })
	}
	if id, ok := utils.AsUUID(value); ok && visible(id) {
		return id, nil
	}
	bySlug, found, err := self.lookups().ProjectBySlug(ctx, actor.Workspace, utils.SlugTail(value))
	if err != nil {
		return uuid.Nil, err
	}
	if found && visible(bySlug) {
		return bySlug, nil
	}
	for _, r := range rows {
		if utils.MatchesName(r.Name, value) {
			return r.ID, nil
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return uuid.Nil, errdefs.BadRequest("No project %q. %s", value, utils.Options(names))
}

func (self *ResolveService) Projects(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	return all(ctx, actor, values, self.Project)
}

// Milestone is a milestone by id, or by name within the project.
func (self *ResolveService) Milestone(ctx context.Context, actor *auth.Actor, project *uuid.UUID, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if id, ok := utils.AsUUID(value); ok {
		projectOf, found, err := self.lookups().MilestoneProject(ctx, actor.Workspace, id)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			if _, err := self.Project(ctx, actor, projectOf.String()); err != nil {
				return uuid.Nil, err
			}
			return id, nil
		}
	}
	if project == nil {
		return uuid.Nil, errdefs.BadRequest("Name the project too, or pass the milestone's id")
	}
	rows, err := self.lookups().Milestones(ctx, *project)
	if err != nil {
		return uuid.Nil, err
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		if strings.EqualFold(r.Name, value) {
			return r.ID, nil
		}
		names[i] = r.Name
	}
	return uuid.Nil, errdefs.BadRequest("The project has no milestone %q. %s", value, utils.Options(names))
}

var relativeCycles = map[string]string{
	"current": "current", "active": "current",
	"next": "next", "upcoming": "next",
	"previous": "previous", "last": "previous",
}

// Cycle is a cycle of the team by id, number, name, or "current", "next" or "previous".
func (self *ResolveService) Cycle(ctx context.Context, actor *auth.Actor, team *uuid.UUID, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if id, ok := utils.AsUUID(value); ok {
		teamOf, found, err := self.lookups().CycleTeam(ctx, id)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			visible, err := actor.CanSeeTeam(ctx, teamOf)
			if err != nil {
				return uuid.Nil, err
			}
			if visible {
				return id, nil
			}
		}
		return uuid.Nil, errdefs.NotFound("No cycle %s", value)
	}
	if team == nil {
		fallback, err := self.DefaultTeam(ctx, actor)
		if err != nil {
			return uuid.Nil, err
		}
		team = &fallback
	}
	var id uuid.UUID
	var found bool
	var err error
	if which, ok := relativeCycles[strings.ToLower(value)]; ok {
		id, found, err = self.lookups().RelativeCycle(ctx, *team, which)
	} else if number, numErr := strconv.Atoi(strings.TrimSpace(strings.TrimLeft(value, "#Cc"))); numErr == nil {
		id, found, err = self.lookups().CycleByNumber(ctx, *team, number)
	} else {
		id, found, err = self.lookups().CycleByName(ctx, *team, value)
	}
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, errdefs.BadRequest("The team has no cycle %q; use its number, name, id, or current/next/previous", value)
	}
	return id, nil
}

// named is something with a workspace, a slug and a name or title, by id, slug, link or name.
func (self *ResolveService) named(ctx context.Context, actor *auth.Actor, table, nameColumn, extra, what, value string, args ...any) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	rows, err := self.lookups().Slugged(ctx, table, nameColumn, actor.Workspace, extra, args...)
	if err != nil {
		return uuid.Nil, err
	}
	id, isID := utils.AsUUID(value)
	slug := utils.SlugTail(value)
	for _, test := range []func(resolve_repo.Slugged) bool{
		func(r resolve_repo.Slugged) bool { return isID && r.ID == id },
		func(r resolve_repo.Slugged) bool { return r.Slug == slug },
		func(r resolve_repo.Slugged) bool { return utils.MatchesName(r.Name, value) },
	} {
		for _, r := range rows {
			if test(r) {
				return r.ID, nil
			}
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return uuid.Nil, errdefs.BadRequest("No %s %q. %s", what, value, utils.Options(names))
}

func (self *ResolveService) Initiative(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	return self.named(ctx, actor, "initiatives", "name", "AND trashed_at IS NULL", "initiative", value)
}

func (self *ResolveService) Initiatives(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	return all(ctx, actor, values, self.Initiative)
}

// Document is a document by id, slug, link or title. Documents of projects, teams and issues
// the caller can't see are left out.
func (self *ResolveService) Document(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	id, err := self.named(ctx, actor, "documents", "title", "AND trashed_at IS NULL", "document", value)
	if err != nil {
		return uuid.Nil, err
	}
	visible, err := self.CanSeeDocument(ctx, actor, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, errdefs.NotFound("No document %q", value)
	}
	return id, nil
}

func (self *ResolveService) CanSeeDocument(ctx context.Context, actor *auth.Actor, id uuid.UUID) (bool, error) {
	row, found, err := self.lookups().DocumentOwners(ctx, id)
	if err != nil || !found {
		return false, err
	}
	switch {
	case row.Team != nil:
		return actor.CanSeeTeam(ctx, *row.Team)
	case row.Project != nil:
		_, err := self.Project(ctx, actor, row.Project.String())
		return err == nil, nil
	case row.Issue != nil:
		return self.CanSeeIssue(ctx, actor, *row.Issue)
	case row.Cycle != nil:
		_, err := self.Cycle(ctx, actor, nil, row.Cycle.String())
		return err == nil, nil
	}
	return true, nil
}

// Discussion is a discussion by id, slug, link or title, among those of the workspace and of the
// teams the caller sees.
func (self *ResolveService) Discussion(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return self.named(ctx, actor, "discussions", "title", "AND trashed_at IS NULL AND (team_id IS NULL OR team_id = ANY($2))", "discussion", value, teams)
}

func (self *ResolveService) CanSeeDiscussion(ctx context.Context, actor *auth.Actor, id uuid.UUID) (bool, error) {
	team, _, found, err := self.repo.Discussion().Owner(ctx, actor.Workspace, id)
	if err != nil || !found {
		return false, err
	}
	if team == nil {
		return true, nil
	}
	return actor.CanSeeTeam(ctx, *team)
}

func (self *ResolveService) ProjectStatus(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	rows, err := self.lookups().ProjectStatuses(ctx, actor.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, found := FindTyped(rows, value); found {
		return id, nil
	}
	return uuid.Nil, errdefs.BadRequest("No project status %q. %s", value, utils.Options(TypedNames(rows)))
}

func (self *ResolveService) ProjectLabels(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if len(values) == 0 {
		return ids, nil
	}
	rows, err := self.lookups().ProjectLabels(ctx, actor.Workspace)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		index := slices.IndexFunc(rows, func(r resolve_repo.LabelRow) bool { return LabelMatches(r.ID, r.MergedIDs, r.Name, r.Parent, value) })
		if index < 0 {
			names := make([]string, len(rows))
			for i, r := range rows {
				names[i] = r.Name
			}
			return nil, errdefs.BadRequest("No project label %q. %s", value, utils.Options(names))
		}
		ids = utils.AppendUnique(ids, rows[index].ID)
	}
	return ids, nil
}

// PipelineVisible is SQL that is true when the pipeline in column has no teams or one the
// caller sees. The team ids are UUIDs, so inlining them is safe.
func (self *ResolveService) PipelineVisible(ctx context.Context, actor *auth.Actor, column string) (string, error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return "", err
	}
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.String()
	}
	return fmt.Sprintf(
		`AND (NOT EXISTS (SELECT 1 FROM release_pipeline_teams pt WHERE pt.pipeline_id = %s)
		  OR EXISTS (SELECT 1 FROM release_pipeline_teams pt WHERE pt.pipeline_id = %s
		             AND pt.team_id = ANY('{%s}'::uuid[])))`, column, column, strings.Join(ids, ",")), nil
}

func (self *ResolveService) Release(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	extra, err := self.PipelineVisible(ctx, actor, "releases.pipeline_id")
	if err != nil {
		return uuid.Nil, err
	}
	return self.named(ctx, actor, "releases", "name", extra, "release", value)
}

func (self *ResolveService) Releases(ctx context.Context, actor *auth.Actor, values []string) ([]uuid.UUID, error) {
	return all(ctx, actor, values, self.Release)
}

func (self *ResolveService) Pipeline(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	extra, err := self.PipelineVisible(ctx, actor, "release_pipelines.id")
	if err != nil {
		return uuid.Nil, err
	}
	return self.named(ctx, actor, "release_pipelines", "name", extra, "release pipeline", value)
}

func (self *ResolveService) Customer(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	id, found, err := self.lookups().CustomerByDomain(ctx, actor.Workspace, value)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		return id, nil
	}
	return self.named(ctx, actor, "customers", "name", "", "customer", value)
}

func (self *ResolveService) CustomView(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	return self.named(ctx, actor, "custom_views", "name", resolve_repo.CustomViewsVisibleTo(actor.User), "view", value)
}

// Template is a template by id or name, of the given type (issue, project or document).
func (self *ResolveService) Template(ctx context.Context, actor *auth.Actor, kind, value string) (uuid.UUID, error) {
	rows, err := self.lookups().Templates(ctx, actor.Workspace, kind)
	if err != nil {
		return uuid.Nil, err
	}
	visible, err := actor.TeamIDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	rows = slices.DeleteFunc(rows, func(r resolve_repo.TemplateRow) bool { return r.Team != nil && !slices.Contains(visible, *r.Team) })
	value = strings.TrimSpace(value)
	id, isID := utils.AsUUID(value)
	for _, r := range rows {
		if isID && r.ID == id {
			return r.ID, nil
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		if strings.EqualFold(r.Name, value) {
			return r.ID, nil
		}
		names[i] = r.Name
	}
	return uuid.Nil, errdefs.BadRequest("No %s template %q. %s", kind, value, utils.Options(names))
}

// ByID is a row of table in the caller's workspace by id, or a 404 naming what.
func (self *ResolveService) ByID(ctx context.Context, actor *auth.Actor, table, what, value string) (uuid.UUID, error) {
	notFound := errdefs.NotFound("No %s %q", what, strings.TrimSpace(value))
	id, ok := utils.AsUUID(utils.LastSegment(value))
	if !ok {
		return uuid.Nil, notFound
	}
	exists, err := self.lookups().InWorkspace(ctx, table, id, actor.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, notFound
	}
	return id, nil
}
