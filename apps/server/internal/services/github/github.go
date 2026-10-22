// Package github_service is GitHub, as a GitHub App: admins install it on their GitHub account or
// organization and pick repositories. Pull requests, branches and commits that mention
// identifiers link to issues and move them (webhook.go); repositories bound to a team sync their
// issues and comments both ways (sync.go, push.go).
package github_service

import (
	"net/http"
	"time"

	"ricrac/config"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/integrations/github"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	attachment_service "ricrac/internal/services/attachment"
	comment_service "ricrac/internal/services/comment"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	resolve_service "ricrac/internal/services/resolve"
)

type M = map[string]any

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i GitHubServiceInterface -p github_service -s GitHubService -o github_service_iface.go
type GitHubService struct {
	cfg         config.Config
	keys        auth.Keys
	app         *github.App
	repo        repositories.RepositoriesInterface
	access      access_service.AccessServiceInterface
	resolve     resolve_service.ResolveServiceInterface
	events      events_service.EventsServiceInterface
	issues      issue_service.IssueServiceInterface
	comments    comment_service.CommentServiceInterface
	attachments attachment_service.AttachmentServiceInterface
	waker       events_service.Waker
}

// NewGitHubService acts as the GitHub App of cfg, none when cfg has none.
func NewGitHubService(cfg config.Config, keys auth.Keys, repo repositories.RepositoriesInterface, access access_service.AccessServiceInterface,
	resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, issues issue_service.IssueServiceInterface,
	comments comment_service.CommentServiceInterface, attachments attachment_service.AttachmentServiceInterface, waker events_service.Waker) *GitHubService {
	service := &GitHubService{
		cfg: cfg, keys: keys, repo: repo, access: access, resolve: resolve, events: events, issues: issues, comments: comments,
		attachments: attachments, waker: waker,
	}
	if cfg.GitHub != nil {
		service.app = github.NewApp(*cfg.GitHub, &http.Client{Timeout: 30 * time.Second})
	}
	return service
}

// SetApp changes the GitHub App the service acts as; nil turns GitHub off.
func (self *GitHubService) SetApp(app *github.App) {
	self.app = app
}

// gitHub is the app, or an error saying the server has none.
func (self *GitHubService) gitHub() (*github.App, error) {
	if self.app == nil {
		return nil, errdefs.BadRequest("This Ricrac server has no GitHub App; set GITHUB_APP_ID and the other GITHUB_ variables (see docs/github-app.md)")
	}
	return self.app, nil
}

func (self *GitHubService) webURL() string {
	if self.app == nil {
		return "https://github.com"
	}
	return self.app.WebURL
}

type NoInput struct{}

type PageInput struct {
	args.Page
}

type IDInput struct {
	ID string `json:"id"`
}

type DisconnectInput struct {
	Installation *string `json:"installation" doc:"The GitHub account (login) or installation id to forget; needed when GitHub is installed on several."`
}

type SaveRepoSyncInput struct {
	ID            *string               `json:"id" doc:"Only to change a sync: its id or the repository (owner/name). Omit to create one."`
	Repository    *string               `json:"repository" doc:"The repository as owner/name or GitHub id, among list_github_repositories. Required to create."`
	Team          *string               `json:"team" doc:"Optional: the workspace's team by default. Its key, name or id. The team whose issues the repository's are."`
	Project       args.Nullable[string] `json:"project" doc:"Project name, slug or id issues from GitHub join; null for none."`
	Backfill      *bool                 `json:"backfill" doc:"Import the repository's existing issues and comments: on create unless false; true imports again."`
	IncludeClosed *bool                 `json:"includeClosed" doc:"The import includes closed issues too (default false)."`
}
