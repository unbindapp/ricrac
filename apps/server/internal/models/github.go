package models

import (
	"strconv"

	"ricrac/ent"
	"ricrac/internal/integrations/github"
)

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
	LinkBack         bool                `json:"linkBack" doc:"Issues opened on GitHub get a comment linking their Ricrac issue."`
	SyncFields       bool                `json:"syncFields" doc:"Priority, estimate, due date and cycle sync with the organization's issue fields."`
	CreateIssues     bool                `json:"createIssues" doc:"Issues created in Ricrac open on GitHub in this repository too."`
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

// GitHubInstallationOf needs the installation's installer loaded; webURL is GitHub's.
func GitHubInstallationOf(g *ent.GithubInstallation, webURL string) GitHubInstallation {
	manage := webURL
	if g.AccountType == "Organization" {
		manage += "/organizations/" + g.AccountLogin
	}
	manage += "/settings/installations/" + strconv.FormatInt(g.ID, 10)
	return GitHubInstallation{
		ID: g.ID, Account: g.AccountLogin, AccountType: g.AccountType, ManageURL: manage, Installer: UserRefOf(g.Edges.Installer),
		SuspendedAt: TimePtr(g.SuspendedAt), LastEvent: g.LastEvent, LastEventAt: TimePtr(g.LastEventAt), CreatedAt: Time(g.CreatedAt),
	}
}

// GitHubRepoSyncOf needs the sync's team and project loaded; webURL is GitHub's.
func GitHubRepoSyncOf(r *ent.GithubRepoSync, linkedIssues int, webURL string) GitHubRepoSync {
	sync := GitHubRepoSync{
		ID:               ID(r.ID),
		Repository:       GitHubRepositoryRef{ID: r.RepoID, FullName: r.FullName, URL: webURL + "/" + r.FullName},
		InstallationID:   r.InstallationID,
		Team:             *TeamRefOf(r.Edges.Team),
		Project:          ProjectRefOf(r.Edges.Project),
		LinkBack:         r.LinkBack,
		CreateIssues:     r.CreateIssues,
		SyncFields:       r.SyncFields,
		LinkedIssueCount: linkedIssues,
		LastError:        r.LastError,
		LastSyncedAt:     TimePtr(r.LastSyncedAt),
		CreatedAt:        Time(r.CreatedAt),
		UpdatedAt:        Time(r.UpdatedAt),
	}
	if r.BackfillStatus != nil {
		sync.Backfill = &GitHubBackfill{
			Status: *r.BackfillStatus, IncludeClosed: r.BackfillClosed, IssuesCreated: r.IssuesCreated, IssuesUpdated: r.IssuesUpdated,
			CommentsCreated: r.CommentsCreated, Skipped: r.Skipped,
		}
	}
	return sync
}

// GitHubRepositoryOf is a repository an installation was given, with the id of its sync.
func GitHubRepositoryOf(r github.Repository, installation int64, repoSync *string) GitHubRepository {
	return GitHubRepository{
		ID: r.ID, FullName: r.FullName, URL: r.HTMLURL, Private: r.Private, HasIssues: r.HasIssues,
		DefaultBranch: r.DefaultBranch, InstallationID: installation, RepoSync: repoSync,
	}
}
