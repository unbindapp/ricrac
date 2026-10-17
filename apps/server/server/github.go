package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
	"ricrac/github"
	"ricrac/ops"
)

// GitHub: the app's webhook, where GitHub sends people back after installing it, and linking a
// person's GitHub account.

func githubRoutes(mux *http.ServeMux, state *app.State) {
	mux.HandleFunc("POST /github/webhook", githubWebhook(state))
	mux.HandleFunc("GET /api/github/connect", githubConnect(state))
	// GitHub may send either round trip to either callback URL of the app.
	setup, callback := githubSetup(state), githubCallback(state)
	for _, path := range []string{"/api/github/setup", "/api/github/callback"} {
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("installation_id") || r.URL.Query().Has("setup_action") {
				setup(w, r)
				return
			}
			callback(w, r)
		})
	}
}

func githubWebhook(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if state.GitHub == nil {
			app.WriteError(w, app.NotFound("This server has no GitHub App"))
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			app.WriteError(w, app.BadRequest("Couldn't read the body"))
			return
		}
		if !github.ValidSignature(state.GitHub.WebhookSecret, r.Header.Get("X-Hub-Signature-256"), body) {
			app.WriteError(w, app.Errorf(http.StatusUnauthorized, "X-Hub-Signature-256 doesn't match the webhook secret"))
			return
		}
		if err := ops.HandleGitHubWebhook(r.Context(), state, r.Header.Get("X-GitHub-Event"), body); err != nil {
			app.WriteError(w, err)
			return
		}
		app.WriteJSON(w, http.StatusOK, M{"ok": true})
	}
}

// settingsURL is the workspace's integration settings, with an error to show when there's one.
func settingsURL(urlKey, failure string) string {
	path := "/" + urlKey + "/settings/integrations"
	if failure != "" {
		path += "?github_error=" + url.QueryEscape(failure)
	}
	return path
}

// githubSetup is where GitHub sends whoever installed the app (the Setup URL, and the Callback URL
// since the app asks for user authorization while installing). The installation is stored for
// the workspace connect_github signed the state for, once the person's GitHub token shows they can
// reach it.
func githubSetup(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		workspace, user, expired, ok := ops.ReadGitHubInstallState(state.Keys, params.Get("state"))
		if !ok || state.GitHub == nil {
			http.Redirect(w, r, "/?github_error=expired", http.StatusSeeOther)
			return
		}
		urlKey, err := app.One[string](r.Context(), state.DB, "SELECT url_key FROM workspaces WHERE id = $1", workspace)
		if err != nil {
			http.Redirect(w, r, "/?github_error=expired", http.StatusSeeOther)
			return
		}
		if expired {
			http.Redirect(w, r, settingsURL(urlKey, "expired"), http.StatusSeeOther)
			return
		}
		failure := connectInstallation(r, state, workspace, user, params)
		http.Redirect(w, r, settingsURL(urlKey, failure), http.StatusSeeOther)
	}
}

// connectInstallation stores the installation GitHub sent back, answering why it couldn't.
func connectInstallation(r *http.Request, state *app.State, workspace, user uuid.UUID, params url.Values) string {
	ctx := r.Context()
	if signedIn, ok := state.Keys.SessionUser(r); !ok || signedIn != user {
		return "signed_out"
	}
	c, err := app.LoadCtx(ctx, state, user, workspace, app.EveryAccess)
	if err != nil || c == nil || c.RequireAdmin() != nil {
		return "not_admin"
	}
	if params.Get("setup_action") == "request" {
		return "requested"
	}
	installation, err := strconv.ParseInt(params.Get("installation_id"), 10, 64)
	if err != nil {
		return "no_installation"
	}
	if params.Get("code") == "" {
		return "not_authorized"
	}
	token, err := state.GitHub.ExchangeCode(ctx, params.Get("code"))
	if err != nil {
		slog.Warn("GitHub refused the installation's code", "err", err)
		return "not_authorized"
	}
	installations, err := state.GitHub.UserInstallations(ctx, token)
	if err != nil {
		slog.Warn("listing the GitHub installations of the installer failed", "err", err)
		return "github_failed"
	}
	index := slices.IndexFunc(installations, func(i github.Installation) bool { return i.ID == installation })
	if index < 0 {
		return "not_your_installation"
	}
	if err := saveGitHubLogin(ctx, state, token, user); err != nil {
		slog.Warn("reading the installer's GitHub login failed", "err", err)
	}
	account := installations[index].Account
	saved, err := app.One[bool](ctx, state.DB,
		`INSERT INTO github_installations (id, workspace_id, account_login, account_type, installer_id) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET account_login = excluded.account_login, account_type = excluded.account_type,
			suspended_at = NULL, updated_at = now()
		 WHERE github_installations.workspace_id = excluded.workspace_id
		 RETURNING true`, installation, workspace, account.Login, account.Type, user)
	if app.NoRows(err) {
		return "connected_elsewhere"
	}
	if err != nil || !saved {
		slog.Error("saving the GitHub installation failed", "err", err)
		return "failed"
	}
	_ = app.Exec(ctx, state.DB, "INSERT INTO audit_entries (workspace_id, actor_id, type, metadata) VALUES ($1, $2, 'github.connected', $3)",
		workspace, user, M{"account": account.Login, "installationId": installation})
	return ""
}

// saveGitHubLogin links the GitHub account of a user token to user, taking it from anyone who had
// it.
func saveGitHubLogin(ctx context.Context, state *app.State, token string, user uuid.UUID) error {
	raw, err := state.GitHub.UserLogin(ctx, token)
	if err != nil {
		return err
	}
	login, ok := github.NormalizeLogin(raw)
	if !ok {
		return fmt.Errorf("%q isn't a GitHub login", raw)
	}
	return pgx.BeginFunc(ctx, state.DB, func(tx pgx.Tx) error {
		if err := app.Exec(ctx, tx, "UPDATE users SET github_login = NULL WHERE github_login = $1 AND id <> $2", login, user); err != nil {
			return err
		}
		return app.Exec(ctx, tx, "UPDATE users SET github_login = $2 WHERE id = $1", user, login)
	})
}

const githubLinkTTL = 10 * time.Minute

// githubConnect sends a signed-in person to GitHub to link their account.
func githubConnect(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
		user, ok := state.Keys.SessionUser(r)
		if !ok {
			http.Redirect(w, r, withGitHubError(returnTo, "signed_out"), http.StatusSeeOther)
			return
		}
		if state.GitHub == nil {
			http.Redirect(w, r, withGitHubError(returnTo, "not_configured"), http.StatusSeeOther)
			return
		}
		link := state.Keys.Sign(fmt.Sprintf("github-link:%d:%s:%s", time.Now().Add(githubLinkTTL).Unix(), user, returnTo))
		http.Redirect(w, r, state.GitHub.AuthorizeURL(state.Config.PublicURL+"/api/github/callback", link), http.StatusSeeOther)
	}
}

func withGitHubError(returnTo, reason string) string {
	separator := "?"
	if strings.Contains(returnTo, "?") {
		separator = "&"
	}
	return returnTo + separator + "github_error=" + reason
}

func githubCallback(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, returnTo, err := readGitHubLink(state, r)
		if err != nil {
			http.Redirect(w, r, withGitHubError(returnTo, "expired"), http.StatusSeeOther)
			return
		}
		token, err := state.GitHub.ExchangeCode(r.Context(), r.URL.Query().Get("code"))
		if err == nil {
			err = saveGitHubLogin(r.Context(), state, token, user)
		}
		if err != nil {
			slog.Warn("linking a GitHub account failed", "err", err)
			http.Redirect(w, r, withGitHubError(returnTo, "failed"), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
	}
}

// readGitHubLink checks the state githubConnect signed belongs to whoever is signed in.
func readGitHubLink(state *app.State, r *http.Request) (uuid.UUID, string, error) {
	expired := errors.New("expired")
	payload, ok := state.Keys.Verify(r.URL.Query().Get("state"))
	parts := strings.SplitN(payload, ":", 4)
	if !ok || state.GitHub == nil || len(parts) != 4 || parts[0] != "github-link" {
		return uuid.Nil, "/", expired
	}
	returnTo := safeReturnTo(parts[3])
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expires < time.Now().Unix() {
		return uuid.Nil, returnTo, expired
	}
	user, err := uuid.Parse(parts[2])
	if signedIn, ok := state.Keys.SessionUser(r); err != nil || !ok || signedIn != user {
		return uuid.Nil, returnTo, expired
	}
	return user, returnTo, nil
}
