package ops

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
	"ricrac/github"
)

// GitHub, as a GitHub App: admins install it on their GitHub account or organization and pick
// repositories. Pull requests, branches and commits that mention identifiers link to issues and
// move them (github_webhook.go); repositories bound to a team sync their issues and comments both
// ways (github_sync.go, github_push.go).

func githubOps() []*Op {
	return []*Op{
		op("get_github_integration", app.Read, []string{"GET /github"},
			"Read the workspace's GitHub connection: whether this server has a GitHub App, the GitHub accounts it is "+
				"installed on (with the last webhook event), the repositories whose issues sync with a team, and your "+
				"linked GitHub login.",
			getGitHubIntegration).returns(ResultOf[GitHubIntegration]()),
		op("connect_github", app.AdminAccess, []string{"POST /github/connect"},
			"Start connecting GitHub: answers the link to install the GitHub App on a GitHub account or organization "+
				"(valid for an hour). Open it in a browser, pick the repositories, and GitHub sends you back to Ricrac's "+
				"integration settings. Admins only.",
			connectGitHub).returns(ResultOf[GitHubConnect]()),
		op("disconnect_github", app.AdminAccess, []string{"DELETE /github"},
			"Forget a GitHub installation: its repositories stop syncing and its events are ignored. Issues, comments "+
				"and links stay. Uninstall the app on GitHub (manageUrl) to remove its access there too. Admins only.",
			disconnectGitHub).returns(ResultOf[GitHubDisconnected]()),
		op("list_github_repositories", app.AdminAccess, []string{"GET /github/repositories"},
			"List the repositories the GitHub App was given, and which of them sync with a team. Admins only.",
			listGitHubRepositories).returns(ResultOf[PageOf[GitHubRepository]]()),
		op("list_github_repo_syncs", app.Read, []string{"GET /github/repo-syncs"},
			"List the repositories whose issues sync with a team, with how the import of their existing issues went "+
				"and the last error.",
			listGitHubRepoSyncs).returns(ResultOf[PageOf[GitHubRepoSync]]()),
		op("save_github_repo_sync", app.AdminAccess, []string{"POST /github/repo-syncs", "PATCH /github/repo-syncs/{id}"},
			"Sync a repository's issues with a team (omit id; repository and team are required) or change a sync (pass "+
				"id). Issues opened, edited, labeled, assigned, closed or commented on GitHub become Ricrac issues of the "+
				"team (and project, if set), and changes made in Ricrac to them go back to GitHub. Authors and assignees "+
				"map to people through their linked GitHub login. Creating a sync imports the repository's existing "+
				"issues unless backfill is false; backfill true imports them again. Admins only.",
			saveGitHubRepoSync).returns(ResultOf[GitHubRepoSync]()),
		op("delete_github_repo_sync", app.AdminAccess, []string{"DELETE /github/repo-syncs/{id}"},
			"Stop syncing a repository's issues. The issues stay in Ricrac, no longer linked. Admins only.",
			deleteGitHubRepoSync).returns(ResultOf[Deleted]()),
		accountOp("disconnect_github_account", app.Write, []string{"DELETE /me/github"},
			"Unlink your GitHub account: GitHub activity no longer shows as yours and GitHub issues assigned to you no "+
				"longer are in Ricrac. Link it in Ricrac's settings (Connect GitHub).",
			disconnectGitHubAccount).returns(ResultOf[GitHubAccount]()),
	}
}

// gitHub is the app, or an error saying the server has none.
func gitHub(state *app.State) (*github.App, error) {
	if state.GitHub == nil {
		return nil, app.BadRequest("This Ricrac server has no GitHub App; set GITHUB_APP_ID and the other GITHUB_ variables (see docs/github-app.md)")
	}
	return state.GitHub, nil
}

func gitHubWebURL(state *app.State) string {
	if state.GitHub == nil {
		return "https://github.com"
	}
	return state.GitHub.WebURL
}

/* Shapes */

type GitHubIntegration struct {
	Configured    bool                 `json:"configured" doc:"Whether this Ricrac server has a GitHub App set up."`
	InstallURL    *string              `json:"installUrl" doc:"The app's install page; connect_github gives the link that comes back to this workspace."`
	Installations []GitHubInstallation `json:"installations"`
	RepoSyncs     []GitHubRepoSync     `json:"repoSyncs"`
	GitHubLogin   *string              `json:"githubLogin" doc:"Your linked GitHub login."`
}

type GitHubInstallation struct {
	ID          int64    `json:"id" doc:"GitHub's installation id."`
	Account     string   `json:"account" doc:"The GitHub user or organization it is installed on."`
	AccountType string   `json:"accountType" enum:"User,Organization"`
	ManageURL   string   `json:"manageUrl" doc:"Where to pick its repositories or uninstall it on GitHub."`
	Installer   *UserRef `json:"installer"`
	SuspendedAt *string  `json:"suspendedAt"`
	LastEvent   *string  `json:"lastEvent" doc:"The last webhook event, like pull_request.opened."`
	LastEventAt *string  `json:"lastEventAt"`
	CreatedAt   string   `json:"createdAt"`
}

type GitHubRepositoryRef struct {
	ID       int64  `json:"id"`
	FullName string `json:"fullName"`
	URL      string `json:"url"`
}

type GitHubBackfill struct {
	Status          string `json:"status" enum:"pending,running,completed,failed"`
	IncludeClosed   bool   `json:"includeClosed"`
	IssuesCreated   int    `json:"issuesCreated"`
	IssuesUpdated   int    `json:"issuesUpdated"`
	CommentsCreated int    `json:"commentsCreated"`
	Skipped         int    `json:"skipped"`
}

type GitHubRepoSync struct {
	ID               string              `json:"id"`
	Repository       GitHubRepositoryRef `json:"repository"`
	InstallationID   int64               `json:"installationId"`
	Team             TeamRef             `json:"team"`
	Project          *ProjectRef         `json:"project"`
	Backfill         *GitHubBackfill     `json:"backfill" doc:"The import of the repository's existing issues; null when none was asked for."`
	LinkedIssueCount int                 `json:"linkedIssueCount"`
	LastError        *string             `json:"lastError" doc:"Why the last change from or to GitHub failed; null after one succeeds."`
	LastSyncedAt     *string             `json:"lastSyncedAt"`
	CreatedAt        string              `json:"createdAt"`
	UpdatedAt        string              `json:"updatedAt"`
}

type GitHubRepository struct {
	ID             int64   `json:"id"`
	FullName       string  `json:"fullName"`
	URL            string  `json:"url"`
	Private        bool    `json:"private"`
	HasIssues      bool    `json:"hasIssues"`
	DefaultBranch  string  `json:"defaultBranch"`
	InstallationID int64   `json:"installationId"`
	RepoSync       *string `json:"repoSync" doc:"The id of its sync with a team, if it has one."`
}

type GitHubConnect struct {
	InstallURL string `json:"installUrl"`
	ExpiresAt  string `json:"expiresAt"`
}

type GitHubDisconnected struct {
	Disconnected string `json:"disconnected" doc:"The GitHub account whose installation was forgotten."`
}

type GitHubAccount struct {
	GitHubLogin *string `json:"githubLogin"`
}

const installationJSON = `jsonb_build_object('id', g.id, 'account', g.account_login, 'accountType', g.account_type,
	'manageUrl', $2 || CASE WHEN g.account_type = 'Organization' THEN '/organizations/' || g.account_login ELSE '' END
		|| '/settings/installations/' || g.id,
	'installer', user_ref(g.installer_id), 'suspendedAt', g.suspended_at, 'lastEvent', g.last_event,
	'lastEventAt', g.last_event_at, 'createdAt', g.created_at)`

// repoSyncJSON is the JSON of repo sync r; $2 is GitHub's web URL.
const repoSyncJSON = `jsonb_build_object('id', r.id,
	'repository', jsonb_build_object('id', r.repo_id, 'fullName', r.full_name, 'url', $2 || '/' || r.full_name),
	'installationId', r.installation_id, 'team', team_ref(r.team_id), 'project', project_ref(r.project_id),
	'backfill', CASE WHEN r.backfill_status IS NOT NULL THEN jsonb_build_object('status', r.backfill_status,
		'includeClosed', r.backfill_closed, 'issuesCreated', r.issues_created, 'issuesUpdated', r.issues_updated,
		'commentsCreated', r.comments_created, 'skipped', r.skipped) END,
	'linkedIssueCount', (SELECT count(*) FROM github_issue_links l WHERE l.repo_sync_id = r.id),
	'lastError', r.last_error, 'lastSyncedAt', r.last_synced_at, 'createdAt', r.created_at, 'updatedAt', r.updated_at)`

func repoSyncValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, "SELECT "+repoSyncJSON+" FROM github_repo_syncs r WHERE r.id = $1", id, gitHubWebURL(c.State))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such repository sync")
	}
	return value, nil
}

/* Integration */

func getGitHubIntegration(c *app.Ctx, _ NoArgs) (any, error) {
	installations, err := app.All[any](c, c.DB(),
		"SELECT "+installationJSON+" FROM github_installations g WHERE g.workspace_id = $1 ORDER BY g.created_at",
		c.Workspace, gitHubWebURL(c.State))
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	syncs, err := app.All[any](c, c.DB(),
		"SELECT "+repoSyncJSON+" FROM github_repo_syncs r WHERE r.workspace_id = $1 AND r.team_id = ANY($3) ORDER BY lower(r.full_name)",
		c.Workspace, gitHubWebURL(c.State), teams)
	if err != nil {
		return nil, err
	}
	login, err := app.One[*string](c, c.DB(), "SELECT github_login FROM users WHERE id = $1", c.User)
	if err != nil {
		return nil, err
	}
	var installURL *string
	if c.State.GitHub != nil {
		installURL = new(c.State.GitHub.InstallURL(""))
	}
	return M{
		"configured": c.State.GitHub != nil, "installUrl": installURL, "installations": nonNil(installations),
		"repoSyncs": nonNil(syncs), "githubLogin": login,
	}, nil
}

const installStateTTL = time.Hour

func connectGitHub(c *app.Ctx, _ NoArgs) (any, error) {
	gh, err := gitHub(c.State)
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(installStateTTL)
	state := c.State.Keys.Sign(fmt.Sprintf("github-install:%d:%s:%s", expires.Unix(), c.Workspace, c.User))
	return M{"installUrl": gh.InstallURL(state), "expiresAt": expires.UTC().Format(time.RFC3339)}, nil
}

// ReadGitHubInstallState is the workspace and user connect_github signed the state for, false
// when it is forged; expired says it is too old to use (the workspace is still known).
func ReadGitHubInstallState(keys app.Keys, state string) (workspace, user uuid.UUID, expired, ok bool) {
	payload, ok := keys.Verify(state)
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

type DisconnectGitHubArgs struct {
	Installation *string `json:"installation" doc:"The GitHub account (login) or installation id to forget; needed when GitHub is installed on several."`
}

func disconnectGitHub(c *app.Ctx, args DisconnectGitHubArgs) (any, error) {
	type installation struct {
		ID      int64
		Account string
	}
	rows, err := app.Rows[installation](c, c.DB(),
		"SELECT id, account_login FROM github_installations WHERE workspace_id = $1 ORDER BY created_at", c.Workspace)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, app.NotFound("GitHub isn't connected to this workspace")
	}
	names := make([]string, len(rows))
	for i, row := range rows {
		names[i] = row.Account
	}
	var chosen *installation
	switch {
	case args.Installation != nil:
		wanted := strings.TrimSpace(*args.Installation)
		for i, row := range rows {
			if strings.EqualFold(row.Account, wanted) || strconv.FormatInt(row.ID, 10) == wanted {
				chosen = &rows[i]
			}
		}
		if chosen == nil {
			return nil, app.BadRequest("GitHub isn't installed on %q here. %s", wanted, options(names))
		}
	case len(rows) == 1:
		chosen = &rows[0]
	default:
		return nil, app.BadRequest("GitHub is installed on several accounts; pass installation. %s", options(names))
	}
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, "DELETE FROM github_installations WHERE id = $1", chosen.ID); err != nil {
			return err
		}
		return audit(c, tx, "github.disconnected", M{"account": chosen.Account, "installationId": chosen.ID})
	})
	if err != nil {
		return nil, err
	}
	return M{"disconnected": chosen.Account}, nil
}

func disconnectGitHubAccount(c *app.Caller, _ NoArgs) (any, error) {
	if err := app.Exec(c, c.DB(), "UPDATE users SET github_login = NULL WHERE id = $1", c.User); err != nil {
		return nil, err
	}
	return M{"githubLogin": nil}, nil
}

/* Repositories */

type installedRepository struct {
	github.Repository
	installation int64
}

// installedRepositories is every repository the workspace's installations were given.
func installedRepositories(c *app.Ctx) ([]installedRepository, error) {
	gh, err := gitHub(c.State)
	if err != nil {
		return nil, err
	}
	installations, err := app.All[int64](c, c.DB(),
		"SELECT id FROM github_installations WHERE workspace_id = $1 AND suspended_at IS NULL ORDER BY created_at", c.Workspace)
	if err != nil {
		return nil, err
	}
	if len(installations) == 0 {
		return nil, app.BadRequest("GitHub isn't connected to this workspace; connect it with connect_github first")
	}
	var all []installedRepository
	for _, installation := range installations {
		repositories, err := gh.InstallationRepositories(c, installation)
		if err != nil {
			return nil, app.Errorf(502, "GitHub didn't list the repositories: %s", err)
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

func listGitHubRepositories(c *app.Ctx, args Page) (any, error) {
	repositories, err := installedRepositories(c)
	if err != nil {
		return nil, err
	}
	type synced struct {
		Repo int64
		ID   string
	}
	syncs, err := app.Rows[synced](c, c.DB(), "SELECT repo_id, id::text FROM github_repo_syncs WHERE workspace_id = $1", c.Workspace)
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes := []any{}
	for _, r := range repositories[min(int(offset), len(repositories)):] {
		if len(nodes) > int(args.limit()) {
			break
		}
		var repoSync *string
		if i := slices.IndexFunc(syncs, func(s synced) bool { return s.Repo == r.ID }); i >= 0 {
			repoSync = &syncs[i].ID
		}
		nodes = append(nodes, M{
			"id": r.ID, "fullName": r.FullName, "url": r.HTMLURL, "private": r.Private, "hasIssues": r.HasIssues,
			"defaultBranch": r.DefaultBranch, "installationId": r.installation, "repoSync": repoSync,
		})
	}
	return args.wrap(nodes)
}

func listGitHubRepoSyncs(c *app.Ctx, args Page) (any, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		"SELECT "+repoSyncJSON+` FROM github_repo_syncs r WHERE r.workspace_id = $1 AND r.team_id = ANY($3)
		 ORDER BY lower(r.full_name), r.id LIMIT $4 OFFSET $5`,
		c.Workspace, gitHubWebURL(c.State), teams, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type SaveGitHubRepoSyncArgs struct {
	ID            *string          `json:"id" doc:"Only to change a sync: its id or the repository (owner/name). Omit to create one."`
	Repository    *string          `json:"repository" doc:"The repository as owner/name or GitHub id, among list_github_repositories. Required to create."`
	Team          *string          `json:"team" doc:"Team key, name or id whose issues the repository's are. Required to create."`
	Project       Nullable[string] `json:"project" doc:"Project name, slug or id issues from GitHub join; null for none."`
	Backfill      *bool            `json:"backfill" doc:"Import the repository's existing issues and comments: on create unless false; true imports again."`
	IncludeClosed *bool            `json:"includeClosed" doc:"The import includes closed issues too (default false)."`
}

// findRepoSync is a repo sync of the workspace by id or repository full name.
func findRepoSync(c *app.Ctx, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	id, found, err := app.Maybe[uuid.UUID](c, c.DB(),
		"SELECT id FROM github_repo_syncs WHERE workspace_id = $1 AND (id::text = $2 OR lower(full_name) = lower($2))", c.Workspace, value)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, app.NotFound("No repository sync %q; see list_github_repo_syncs", value)
	}
	return id, nil
}

func saveGitHubRepoSync(c *app.Ctx, args SaveGitHubRepoSyncArgs) (any, error) {
	var id uuid.UUID
	var team *uuid.UUID
	if args.Team != nil {
		resolved, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		team = &resolved
	}
	project, err := resolveOptional(args.Project, func(v string) (uuid.UUID, error) { return resolveProject(c, v) })
	if err != nil {
		return nil, err
	}
	backfill := args.Backfill != nil && *args.Backfill
	if args.ID != nil {
		if args.Repository != nil {
			return nil, app.BadRequest("A sync's repository can't change; delete it and sync the other repository")
		}
		if id, err = findRepoSync(c, *args.ID); err != nil {
			return nil, err
		}
		err := app.Exec(c, c.DB(),
			`UPDATE github_repo_syncs SET team_id = coalesce($2, team_id),
				project_id = CASE WHEN $3 THEN $4::uuid ELSE project_id END, updated_at = now()
			 WHERE id = $1`, id, team, args.Project.Set, project)
		if err != nil {
			return nil, err
		}
	} else {
		if args.Repository == nil || team == nil {
			return nil, app.BadRequest("Pass repository and team to sync a repository")
		}
		if id, err = createRepoSync(c, *args.Repository, *team, project); err != nil {
			return nil, err
		}
		backfill = args.Backfill == nil || *args.Backfill
	}
	if backfill {
		err := app.Exec(c, c.DB(),
			`UPDATE github_repo_syncs SET backfill_status = 'pending', backfill_page = 1, backfill_closed = coalesce($2, backfill_closed),
				issues_created = 0, issues_updated = 0, comments_created = 0, skipped = 0
			 WHERE id = $1`, id, args.IncludeClosed)
		if err != nil {
			return nil, err
		}
		c.State.QueueGitHub()
	}
	return repoSyncValue(c, c.DB(), id)
}

func createRepoSync(c *app.Ctx, reference string, team uuid.UUID, project *uuid.UUID) (uuid.UUID, error) {
	repository, installation, err := findInstalledRepository(c, reference)
	if err != nil {
		return uuid.Nil, err
	}
	taken, err := app.One[bool](c, c.DB(), "SELECT EXISTS (SELECT 1 FROM github_repo_syncs WHERE repo_id = $1)", repository.ID)
	if err != nil {
		return uuid.Nil, err
	}
	if taken {
		return uuid.Nil, app.Conflict("%s already syncs with a team; change that sync instead", repository.FullName)
	}
	return app.One[uuid.UUID](c, c.DB(),
		`INSERT INTO github_repo_syncs (workspace_id, installation_id, repo_id, full_name, team_id, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		c.Workspace, installation, repository.ID, repository.FullName, team, project)
}

// findInstalledRepository is a repository an installation of the workspace was given, by
// owner/name or id.
func findInstalledRepository(c *app.Ctx, value string) (github.Repository, int64, error) {
	repositories, err := installedRepositories(c)
	if err != nil {
		return github.Repository{}, 0, err
	}
	wanted := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(value), gitHubWebURL(c.State)+"/"), ".git")
	names := make([]string, len(repositories))
	for i, r := range repositories {
		names[i] = r.FullName
		if !strings.EqualFold(r.FullName, wanted) && strconv.FormatInt(r.ID, 10) != wanted {
			continue
		}
		if !r.HasIssues {
			return github.Repository{}, 0, app.BadRequest("%s has issues turned off on GitHub", r.FullName)
		}
		return r.Repository, r.installation, nil
	}
	return github.Repository{}, 0, app.BadRequest("The GitHub App can't see %q; give it the repository on GitHub. %s", wanted, options(names))
}

func deleteGitHubRepoSync(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findRepoSync(c, args.ID)
	if err != nil {
		return nil, err
	}
	if err := app.Exec(c, c.DB(), "DELETE FROM github_repo_syncs WHERE id = $1", id); err != nil {
		return nil, err
	}
	return M{"deleted": id}, nil
}
