package seed_repo

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
)

// What new workspaces and teams start with.

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i SeedRepositoryInterface -p seed_repo -s SeedRepository -o seed_repository_iface.go
type SeedRepository struct {
	base *repository.BaseRepository
}

func NewSeedRepository(db *ent.Client) *SeedRepository {
	return &SeedRepository{base: repository.NewBaseRepository(db)}
}

// WorkspaceDefaults adds project statuses, a few labels and customer statuses.
func (self *SeedRepository) WorkspaceDefaults(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID) error {
	client := self.base.Client(tx)
	statuses := [][3]string{
		{"Backlog", "backlog", "#bec2c8"},
		{"Planned", "planned", "#e2e2e2"},
		{"In Progress", "started", "#f2c94c"},
		{"Paused", "paused", "#95a2b3"},
		{"Completed", "completed", "#5e6ad2"},
		{"Canceled", "canceled", "#95a2b3"},
	}
	for position, s := range statuses {
		err := client.ProjectStatus.Create().SetWorkspaceID(workspace).SetName(s[0]).SetType(s[1]).SetColor(s[2]).
			SetPosition(float64(position)).Exec(ctx)
		if err != nil {
			return err
		}
	}
	for _, l := range [][2]string{{"Bug", "#eb5757"}, {"Feature", "#bb87fc"}, {"Improvement", "#4ea7fc"}} {
		if err := client.IssueLabel.Create().SetWorkspaceID(workspace).SetName(l[0]).SetColor(l[1]).Exec(ctx); err != nil {
			return err
		}
	}
	for position, s := range [][2]string{{"Active", "#4cb782"}, {"Prospect", "#f2c94c"}, {"Churned", "#95a2b3"}} {
		err := client.CustomerStatus.Create().SetWorkspaceID(workspace).SetName(s[0]).SetColor(s[1]).SetPosition(float64(position)).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// TeamDefaults is the workflow every team starts with. New issues go to Backlog; issues closed by
// the auto-close automation go to Canceled.
func (self *SeedRepository) TeamDefaults(ctx context.Context, tx repository.TxInterface, team uuid.UUID) error {
	client := self.base.Client(tx)
	states := [][3]string{
		{"Backlog", "backlog", "#bec2c8"},
		{"Todo", "unstarted", "#e2e2e2"},
		{"In Progress", "started", "#f2c94c"},
		{"In Review", "started", "#4cb782"},
		{"Done", "completed", "#5e6ad2"},
		{"Canceled", "canceled", "#95a2b3"},
		{"Duplicate", "canceled", "#95a2b3"},
	}
	ids := map[string]uuid.UUID{}
	for position, s := range states {
		created, err := client.WorkflowState.Create().SetTeamID(team).SetName(s[0]).SetType(s[1]).SetColor(s[2]).
			SetPosition(float64(position)).Save(ctx)
		if err != nil {
			return err
		}
		ids[s[0]] = created.ID
	}
	return client.Team.UpdateOneID(team).SetDefaultIssueStateID(ids["Backlog"]).SetAutoCloseStateID(ids["Canceled"]).Exec(ctx)
}

// EnsureTriageState adds the Triage status a team needs once triage is turned on.
func (self *SeedRepository) EnsureTriageState(ctx context.Context, tx repository.TxInterface, team uuid.UUID) (uuid.UUID, error) {
	client := self.base.Client(tx)
	existing, err := client.WorkflowState.Query().
		Where(workflowstate.TeamID(team), workflowstate.Type("triage"), workflowstate.ArchivedAtIsNil()).Limit(1).IDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if len(existing) > 0 {
		return existing[0], nil
	}
	states, err := client.WorkflowState.Query().Where(workflowstate.TeamID(team)).All(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	first := 0.0
	for i, s := range states {
		if i == 0 || s.Position < first {
			first = s.Position
		}
	}
	created, err := client.WorkflowState.Create().SetTeamID(team).SetName("Triage").SetType("triage").SetColor("#fc7840").
		SetPosition(first - 1).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// CreateTeam creates a team, keyed in upper case, with the creator as its first member and owner.
func (self *SeedRepository) CreateTeam(ctx context.Context, tx repository.TxInterface, workspace, creator uuid.UUID, name, key string) (uuid.UUID, error) {
	client := self.base.Client(tx)
	team, err := client.Team.Create().SetWorkspaceID(workspace).SetName(name).SetKey(strings.ToUpper(key)).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if err := self.TeamDefaults(ctx, tx, team.ID); err != nil {
		return uuid.Nil, err
	}
	return team.ID, client.TeamMember.Create().SetTeamID(team.ID).SetUserID(creator).SetOwner(true).Exec(ctx)
}
