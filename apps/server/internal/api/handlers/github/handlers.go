// Package github_handler is the GitHub operations, and on chi the app's webhook, where GitHub
// sends people back after installing it, and linking a person's GitHub account.
package github_handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	github_service "ricrac/internal/services/github"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	gh := srv.GitHubService
	oapi.Register(reg, oapi.Operation{
		Name: "get_github_integration", Access: auth.Read, Routes: []string{"GET /github"},
		Description: "Read the workspace's GitHub connection: whether this server has a GitHub App, the GitHub accounts it is " +
			"installed on (with the last webhook event), the repositories whose issues sync, and your " +
			"linked GitHub login.",
	}, gh.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "connect_github", Access: auth.AdminAccess, Routes: []string{"POST /github/connect"},
		Description: "Start connecting GitHub: answers the link to install the GitHub App on a GitHub account or organization " +
			"(valid for an hour). Open it in a browser, pick the repositories, and GitHub sends you back to Ricrac's " +
			"integration settings. Admins only.",
	}, gh.Connect)
	oapi.Register(reg, oapi.Operation{
		Name: "disconnect_github", Access: auth.AdminAccess, Routes: []string{"DELETE /github"},
		Description: "Forget a GitHub installation: its repositories stop syncing and its events are ignored. Issues, comments " +
			"and links stay. Uninstall the app on GitHub (manageUrl) to remove its access there too. Admins only.",
	}, gh.Disconnect)
	oapi.Register(reg, oapi.Operation{
		Name: "list_github_repositories", Access: auth.AdminAccess, Routes: []string{"GET /github/repositories"},
		Description: "List the repositories the GitHub App was given, and which of them sync. Admins only.",
	}, gh.ListRepositories)
	oapi.Register(reg, oapi.Operation{
		Name: "list_github_repo_syncs", Access: auth.Read, Routes: []string{"GET /github/repo-syncs"},
		Description: "List the repositories whose issues sync, with how the import of their existing issues went " +
			"and the last error.",
	}, gh.ListRepoSyncs)
	oapi.Register(reg, oapi.Operation{
		Name: "save_github_repo_sync", Access: auth.AdminAccess, Routes: []string{"POST /github/repo-syncs", "PATCH /github/repo-syncs/{id}"},
		Description: "Sync a repository's issues (omit id; repository is required, team is optional) or change a sync (pass " +
			"id). Issues opened, edited, labeled, assigned, closed or commented on GitHub become Ricrac issues " +
			"(of the project, if set), and changes made in Ricrac to them go back to GitHub. Authors and assignees " +
			"map to people through their linked GitHub login. Creating a sync imports the repository's existing " +
			"issues unless backfill is false; backfill true imports them again. Admins only.",
	}, gh.SaveRepoSync)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_github_repo_sync", Access: auth.AdminAccess, Routes: []string{"DELETE /github/repo-syncs/{id}"},
		Description: "Stop syncing a repository's issues. The issues stay in Ricrac, no longer linked. Admins only.",
	}, gh.DeleteRepoSync)
	oapi.RegisterAccount(reg, oapi.Operation{
		Name: "disconnect_github_account", Access: auth.Write, Routes: []string{"DELETE /me/github"},
		Description: "Unlink your GitHub account: GitHub activity no longer shows as yours and GitHub issues assigned to you no " +
			"longer are in Ricrac. Link it in Ricrac's settings (Connect GitHub).",
	}, gh.DisconnectAccount)
}

type handlers struct {
	keys   auth.Keys
	github *github_service.GitHubService
}

func Mount(r chi.Router, srv *server.Server) {
	h := &handlers{keys: srv.Keys, github: srv.GitHubService}
	r.Post("/github/webhook", h.webhook)
	r.Get("/api/github/connect", h.connect)
	// GitHub may send either round trip to either callback URL of the app.
	r.Get("/api/github/setup", h.callback)
	r.Get("/api/github/callback", h.callback)
}

func (h *handlers) session(r *http.Request) github_service.Session {
	user, signedIn := h.keys.SessionUser(r)
	return github_service.Session{User: user, SignedIn: signedIn}
}

func (h *handlers) webhook(w http.ResponseWriter, r *http.Request) {
	if !h.github.Configured() {
		errdefs.Write(w, errdefs.NotFound("This server has no GitHub App"))
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		errdefs.Write(w, errdefs.BadRequest("Couldn't read the body"))
		return
	}
	if err := h.github.HandleWebhook(r.Context(), r.Header.Get("X-GitHub-Event"), r.Header.Get("X-Hub-Signature-256"), body); err != nil {
		errdefs.Write(w, err)
		return
	}
	errdefs.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) connect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.github.StartLink(h.session(r), r.URL.Query().Get("return_to")), http.StatusSeeOther)
}

func (h *handlers) callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Has("installation_id") || query.Has("setup_action") {
		http.Redirect(w, r, h.github.FinishInstall(r.Context(), h.session(r), query), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, h.github.FinishLink(r.Context(), h.session(r), query), http.StatusSeeOther)
}
