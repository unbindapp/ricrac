package favorite_repo

import (
	"context"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/customview"
	"ricrac/ent/cycle"
	"ricrac/ent/document"
	"ricrac/ent/initiative"
	"ricrac/ent/issue"
	"ricrac/ent/issuelabel"
	"ricrac/ent/project"
	"ricrac/ent/release"
	"ricrac/ent/team"
	"ricrac/ent/user"
	repository "ricrac/internal/repositories"
)

// Targets is what favorites point at, by id, with what their short forms show (issues with their
// team and state, cycles with their team, labels with their group).
type Targets struct {
	Issues      map[uuid.UUID]*ent.Issue
	Projects    map[uuid.UUID]*ent.Project
	Initiatives map[uuid.UUID]*ent.Initiative
	Cycles      map[uuid.UUID]*ent.Cycle
	Views       map[uuid.UUID]*ent.CustomView
	Documents   map[uuid.UUID]*ent.Document
	Labels      map[uuid.UUID]*ent.IssueLabel
	Teams       map[uuid.UUID]*ent.Team
	Users       map[uuid.UUID]*ent.User
	Releases    map[uuid.UUID]*ent.Release
}

func byID[T any](items []T, id func(T) uuid.UUID) map[uuid.UUID]T {
	found := make(map[uuid.UUID]T, len(items))
	for _, item := range items {
		found[id(item)] = item
	}
	return found
}

// Targets loads what the favorites point at.
func (self *FavoriteRepository) Targets(ctx context.Context, tx repository.TxInterface, favorites []*ent.Favorite) (Targets, error) {
	ids := map[string][]uuid.UUID{}
	for _, f := range favorites {
		ids[f.Type] = append(ids[f.Type], f.TargetID)
	}
	client := self.base.Client(tx)
	var targets Targets
	issues, err := client.Issue.Query().Where(issue.IDIn(ids["issue"]...)).WithTeam().WithState().All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Issues = byID(issues, func(i *ent.Issue) uuid.UUID { return i.ID })
	projects, err := client.Project.Query().Where(project.IDIn(ids["project"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Projects = byID(projects, func(p *ent.Project) uuid.UUID { return p.ID })
	initiatives, err := client.Initiative.Query().Where(initiative.IDIn(ids["initiative"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Initiatives = byID(initiatives, func(i *ent.Initiative) uuid.UUID { return i.ID })
	cycles, err := client.Cycle.Query().Where(cycle.IDIn(ids["cycle"]...)).WithTeam().All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Cycles = byID(cycles, func(c *ent.Cycle) uuid.UUID { return c.ID })
	views, err := client.CustomView.Query().Where(customview.IDIn(ids["custom_view"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Views = byID(views, func(v *ent.CustomView) uuid.UUID { return v.ID })
	documents, err := client.Document.Query().Where(document.IDIn(ids["document"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Documents = byID(documents, func(d *ent.Document) uuid.UUID { return d.ID })
	labels, err := client.IssueLabel.Query().Where(issuelabel.IDIn(ids["label"]...)).WithParent().All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Labels = byID(labels, func(l *ent.IssueLabel) uuid.UUID { return l.ID })
	teams, err := client.Team.Query().Where(team.IDIn(ids["team"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Teams = byID(teams, func(t *ent.Team) uuid.UUID { return t.ID })
	users, err := client.User.Query().Where(user.IDIn(ids["user"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Users = byID(users, func(u *ent.User) uuid.UUID { return u.ID })
	releases, err := client.Release.Query().Where(release.IDIn(ids["release"]...)).All(ctx)
	if err != nil {
		return targets, err
	}
	targets.Releases = byID(releases, func(r *ent.Release) uuid.UUID { return r.ID })
	return targets, nil
}
