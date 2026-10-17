package github_service

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/integrations/github"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	github_repo "ricrac/internal/repositories/github"
)

const installStateTTL = time.Hour

func (self *GitHubService) github() github_repo.GitHubRepositoryInterface {
	return self.repo.GitHub()
}

func (self *GitHubService) repoSyncValues(ctx context.Context, syncs []*ent.GithubRepoSync) ([]models.GitHubRepoSync, error) {
	ids := make([]uuid.UUID, len(syncs))
	for i, s := range syncs {
		ids[i] = s.ID
	}
	counts, err := self.github().LinkedIssueCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.GitHubRepoSync, len(syncs))
	for i, s := range syncs {
		values[i] = models.GitHubRepoSyncOf(s, counts[s.ID], self.webURL())
	}
	return values, nil
}

func (self *GitHubService) repoSyncValue(ctx context.Context, id uuid.UUID) (models.GitHubRepoSync, error) {
	found, err := self.github().RepoSync(ctx, id)
	if err != nil {
		return models.GitHubRepoSync{}, err
	}
	if found == nil {
		return models.GitHubRepoSync{}, errdefs.NotFound("No such repository sync")
	}
	values, err := self.repoSyncValues(ctx, []*ent.GithubRepoSync{found})
	if err != nil {
		return models.GitHubRepoSync{}, err
	}
	return values[0], nil
}

func (self *GitHubService) Get(ctx context.Context, actor *auth.Actor, _ *NoInput) (models.GitHubIntegration, error) {
	installations, err := self.github().Installations(ctx, actor.Workspace)
	if err != nil {
		return models.GitHubIntegration{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.GitHubIntegration{}, err
	}
	syncs, err := self.github().RepoSyncs(ctx, actor.Workspace, teams)
	if err != nil {
		return models.GitHubIntegration{}, err
	}
	repoSyncs, err := self.repoSyncValues(ctx, syncs)
	if err != nil {
		return models.GitHubIntegration{}, err
	}
	login, err := self.github().UserLogin(ctx, actor.User)
	if err != nil {
		return models.GitHubIntegration{}, err
	}
	integration := models.GitHubIntegration{
		Configured: self.app != nil, Installations: make([]models.GitHubInstallation, len(installations)),
		RepoSyncs: repoSyncs, GitHubLogin: login,
	}
	for i, g := range installations {
		integration.Installations[i] = models.GitHubInstallationOf(g, self.webURL())
	}
	if self.app != nil {
		integration.InstallURL = new(self.app.InstallURL(""))
	}
	return integration, nil
}

func (self *GitHubService) Connect(ctx context.Context, actor *auth.Actor, _ *NoInput) (models.GitHubConnect, error) {
	gh, err := self.gitHub()
	if err != nil {
		return models.GitHubConnect{}, err
	}
	expires := time.Now().Add(installStateTTL)
	state := self.keys.Sign(fmt.Sprintf("github-install:%d:%s:%s", expires.Unix(), actor.Workspace, actor.User))
	return models.GitHubConnect{InstallURL: gh.InstallURL(state), ExpiresAt: expires.UTC().Format(time.RFC3339)}, nil
}

// ReadInstallState is the workspace and user Connect signed the state for, false when it is
// forged; expired says it is too old to use (the workspace is still known).
func (self *GitHubService) ReadInstallState(state string) (workspace, user uuid.UUID, expired, ok bool) {
	payload, ok := self.keys.Verify(state)
	if !ok {
		return uuid.Nil, uuid.Nil, false, false
	}
	parts := strings.Split(payload, ":")
	if len(parts) != 4 || parts[0] != "github-install" {
		return uuid.Nil, uuid.Nil, false, false
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	workspace, errW := uuid.Parse(parts[2])
	user, errU := uuid.Parse(parts[3])
	if err != nil || errW != nil || errU != nil {
		return uuid.Nil, uuid.Nil, false, false
	}
	return workspace, user, expires < time.Now().Unix(), true
}

// chooseInstallation is the installation wanted names (login or id), or the only one.
func chooseInstallation(installations []*ent.GithubInstallation, wanted *string) (*ent.GithubInstallation, error) {
	names := make([]string, len(installations))
	for i, g := range installations {
		names[i] = g.AccountLogin
	}
	if wanted == nil {
		if len(installations) == 1 {
			return installations[0], nil
		}
		return nil, errdefs.BadRequest("GitHub is installed on several accounts; pass installation. %s", utils.Options(names))
	}
	name := strings.TrimSpace(*wanted)
	var chosen *ent.GithubInstallation
	for _, g := range installations {
		if strings.EqualFold(g.AccountLogin, name) || strconv.FormatInt(g.ID, 10) == name {
			chosen = g
		}
	}
	if chosen == nil {
		return nil, errdefs.BadRequest("GitHub isn't installed on %q here. %s", name, utils.Options(names))
	}
	return chosen, nil
}

func (self *GitHubService) Disconnect(ctx context.Context, actor *auth.Actor, input *DisconnectInput) (models.GitHubDisconnected, error) {
	installations, err := self.github().Installations(ctx, actor.Workspace)
	if err != nil {
		return models.GitHubDisconnected{}, err
	}
	if len(installations) == 0 {
		return models.GitHubDisconnected{}, errdefs.NotFound("GitHub isn't connected to this workspace")
	}
	chosen, err := chooseInstallation(installations, input.Installation)
	if err != nil {
		return models.GitHubDisconnected{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.github().DeleteInstallation(ctx, tx, chosen.ID); err != nil {
			return err
		}
		return self.events.Audit(ctx, tx, actor, "github.disconnected", M{"account": chosen.AccountLogin, "installationId": chosen.ID})
	})
	if err != nil {
		return models.GitHubDisconnected{}, err
	}
	return models.GitHubDisconnected{Disconnected: chosen.AccountLogin}, nil
}

func (self *GitHubService) DisconnectAccount(ctx context.Context, caller *auth.Caller, _ *NoInput) (models.GitHubAccount, error) {
	if err := self.github().ClearUserLogin(ctx, caller.User); err != nil {
		return models.GitHubAccount{}, err
	}
	return models.GitHubAccount{}, nil
}

type installedRepository struct {
	github.Repository
	installation int64
}

// installedRepositories is every repository the workspace's installations were given.
func (self *GitHubService) installedRepositories(ctx context.Context, actor *auth.Actor) ([]installedRepository, error) {
	gh, err := self.gitHub()
	if err != nil {
		return nil, err
	}
	installations, err := self.github().ActiveInstallations(ctx, actor.Workspace)
	if err != nil {
		return nil, err
	}
	if len(installations) == 0 {
		return nil, errdefs.BadRequest("GitHub isn't connected to this workspace; connect it with connect_github first")
	}
	var all []installedRepository
	for _, installation := range installations {
		repositories, err := gh.InstallationRepositories(ctx, installation)
		if err != nil {
			return nil, errdefs.Errorf(502, "GitHub didn't list the repositories: %s", err)
		}
		for _, repository := range repositories {
			all = append(all, installedRepository{repository, installation})
		}
	}
	slices.SortFunc(all, func(a, b installedRepository) int {
		return strings.Compare(strings.ToLower(a.FullName), strings.ToLower(b.FullName))
	})
	return all, nil
}

func (self *GitHubService) ListRepositories(ctx context.Context, actor *auth.Actor, input *PageInput) (args.PageOf[models.GitHubRepository], error) {
	repositories, err := self.installedRepositories(ctx, actor)
	if err != nil {
		return args.PageOf[models.GitHubRepository]{}, err
	}
	syncs, err := self.github().SyncedRepos(ctx, actor.Workspace)
	if err != nil {
		return args.PageOf[models.GitHubRepository]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.GitHubRepository]{}, err
	}
	nodes := []models.GitHubRepository{}
	for _, r := range repositories[min(int(offset), len(repositories)):] {
		if len(nodes) > int(input.Size()) {
			break
		}
		var repoSync *string
		if i := slices.IndexFunc(syncs, func(s *ent.GithubRepoSync) bool { return s.RepoID == r.ID }); i >= 0 {
			repoSync = models.IDPtr(&syncs[i].ID)
		}
		nodes = append(nodes, models.GitHubRepositoryOf(r.Repository, r.installation, repoSync))
	}
	return args.WrapOf(input.Page, nodes)
}

func (self *GitHubService) ListRepoSyncs(ctx context.Context, actor *auth.Actor, input *PageInput) (args.PageOf[models.GitHubRepoSync], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.GitHubRepoSync]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.GitHubRepoSync]{}, err
	}
	syncs, err := self.github().RepoSyncPage(ctx, actor.Workspace, teams, int(input.Size())+1, int(offset))
	if err != nil {
		return args.PageOf[models.GitHubRepoSync]{}, err
	}
	values, err := self.repoSyncValues(ctx, syncs)
	if err != nil {
		return args.PageOf[models.GitHubRepoSync]{}, err
	}
	return args.WrapOf(input.Page, values)
}

// findRepoSync is a repo sync of the workspace by id or repository full name.
func (self *GitHubService) findRepoSync(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	id, found, err := self.github().FindRepoSync(ctx, actor.Workspace, value)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, errdefs.NotFound("No repository sync %q; see list_github_repo_syncs", value)
	}
	return id, nil
}

func isNullWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "null", "none", "unassigned", "no project":
		return true
	}
	return false
}

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

func (self *GitHubService) SaveRepoSync(ctx context.Context, actor *auth.Actor, input *SaveRepoSyncInput) (models.GitHubRepoSync, error) {
	var team *uuid.UUID
	if input.Team != nil {
		resolved, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return models.GitHubRepoSync{}, err
		}
		team = &resolved
	}
	project, err := resolveOptional(input.Project, func(v string) (uuid.UUID, error) { return self.resolve.Project(ctx, actor, v) })
	if err != nil {
		return models.GitHubRepoSync{}, err
	}
	id, backfill, err := self.saveRepoSync(ctx, actor, input, team, project)
	if err != nil {
		return models.GitHubRepoSync{}, err
	}
	if backfill {
		if err := self.github().RequestBackfill(ctx, id, input.IncludeClosed); err != nil {
			return models.GitHubRepoSync{}, err
		}
		self.waker.WakeGitHub()
	}
	return self.repoSyncValue(ctx, id)
}

// saveRepoSync changes or creates the sync, saying whether to import the repository's issues.
func (self *GitHubService) saveRepoSync(ctx context.Context, actor *auth.Actor, input *SaveRepoSyncInput, team, project *uuid.UUID) (uuid.UUID, bool, error) {
	if input.ID == nil {
		if input.Repository == nil || team == nil {
			return uuid.Nil, false, errdefs.BadRequest("Pass repository and team to sync a repository")
		}
		id, err := self.createRepoSync(ctx, actor, *input.Repository, *team, project)
		return id, input.Backfill == nil || *input.Backfill, err
	}
	if input.Repository != nil {
		return uuid.Nil, false, errdefs.BadRequest("A sync's repository can't change; delete it and sync the other repository")
	}
	id, err := self.findRepoSync(ctx, actor, *input.ID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if err := self.github().ChangeRepoSync(ctx, id, team, input.Project.Set, project); err != nil {
		return uuid.Nil, false, err
	}
	return id, input.Backfill != nil && *input.Backfill, nil
}

func (self *GitHubService) createRepoSync(ctx context.Context, actor *auth.Actor, reference string, team uuid.UUID, project *uuid.UUID) (uuid.UUID, error) {
	repository, installation, err := self.findInstalledRepository(ctx, actor, reference)
	if err != nil {
		return uuid.Nil, err
	}
	taken, err := self.github().RepoTaken(ctx, repository.ID)
	if err != nil {
		return uuid.Nil, err
	}
	if taken {
		return uuid.Nil, errdefs.Conflict("%s already syncs with a team; change that sync instead", repository.FullName)
	}
	return self.github().CreateRepoSync(ctx, github_repo.NewRepoSync{
		Workspace: actor.Workspace, Installation: installation, RepoID: repository.ID, FullName: repository.FullName,
		Team: team, Project: project,
	})
}

// findInstalledRepository is a repository an installation of the workspace was given, by
// owner/name or id.
func (self *GitHubService) findInstalledRepository(ctx context.Context, actor *auth.Actor, value string) (github.Repository, int64, error) {
	repositories, err := self.installedRepositories(ctx, actor)
	if err != nil {
		return github.Repository{}, 0, err
	}
	wanted := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(value), self.webURL()+"/"), ".git")
	names := make([]string, len(repositories))
	for i, r := range repositories {
		names[i] = r.FullName
		if !strings.EqualFold(r.FullName, wanted) && strconv.FormatInt(r.ID, 10) != wanted {
			continue
		}
		if !r.HasIssues {
			return github.Repository{}, 0, errdefs.BadRequest("%s has issues turned off on GitHub", r.FullName)
		}
		return r.Repository, r.installation, nil
	}
	return github.Repository{}, 0, errdefs.BadRequest("The GitHub App can't see %q; give it the repository on GitHub. %s", wanted, utils.Options(names))
}

func (self *GitHubService) DeleteRepoSync(ctx context.Context, actor *auth.Actor, input *IDInput) (models.Deleted, error) {
	id, err := self.findRepoSync(ctx, actor, input.ID)
	if err != nil {
		return models.Deleted{}, err
	}
	if err := self.github().DeleteRepoSync(ctx, id); err != nil {
		return models.Deleted{}, err
	}
	return models.Deleted{Deleted: models.ID(id)}, nil
}
