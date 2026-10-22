package github_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/integrations/github"
	repository "ricrac/internal/repositories"
	signin_service "ricrac/internal/services/signin"
)

const linkTTL = 10 * time.Minute

// Session is who the request is signed in as, if anyone.
type Session struct {
	User     uuid.UUID
	SignedIn bool
}

// settingsURL is the workspace's integration settings, with an error to show when there's one.
func settingsURL(urlKey, failure string) string {
	path := "/" + urlKey + "/settings/integrations"
	if failure != "" {
		path += "?github_error=" + url.QueryEscape(failure)
	}
	return path
}

func withGitHubError(returnTo, reason string) string {
	separator := "?"
	if strings.Contains(returnTo, "?") {
		separator = "&"
	}
	return returnTo + separator + "github_error=" + reason
}

// FinishInstall is where GitHub sends whoever installed the app (the Setup URL, and the Callback
// URL since the app asks for user authorization while installing), answering where to send them
// on. The installation is stored for the workspace Connect signed the state for, once the
// person's GitHub token shows they can reach it.
func (self *GitHubService) FinishInstall(ctx context.Context, session Session, params url.Values) string {
	workspace, user, expired, ok := self.ReadInstallState(params.Get("state"))
	if !ok || self.app == nil {
		return "/?github_error=expired"
	}
	urlKey, err := self.github().WorkspaceURLKey(ctx, workspace)
	if err != nil {
		return "/?github_error=expired"
	}
	if expired {
		return settingsURL(urlKey, "expired")
	}
	return settingsURL(urlKey, self.connectInstallation(ctx, session, workspace, user, params))
}

// connectInstallation stores the installation GitHub sent back, answering why it couldn't.
func (self *GitHubService) connectInstallation(ctx context.Context, session Session, workspace, user uuid.UUID, params url.Values) string {
	if !session.SignedIn || session.User != user {
		return "signed_out"
	}
	actor, err := self.access.Load(ctx, user, workspace, auth.EveryAccess)
	if err != nil || actor == nil || actor.RequireAdmin() != nil {
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
	token, err := self.app.ExchangeCode(ctx, params.Get("code"))
	if err != nil {
		slog.Warn("GitHub refused the installation's code", "err", err)
		return "not_authorized"
	}
	installations, err := self.app.UserInstallations(ctx, token.AccessToken)
	if err != nil {
		slog.Warn("listing the GitHub installations of the installer failed", "err", err)
		return "github_failed"
	}
	index := slices.IndexFunc(installations, func(i github.Installation) bool { return i.ID == installation })
	if index < 0 {
		return "not_your_installation"
	}
	if err := self.saveLogin(ctx, token, user); err != nil {
		slog.Warn("reading the installer's GitHub login failed", "err", err)
	}
	account := installations[index].Account
	saved, err := self.github().SaveInstallation(ctx, installation, workspace, account.Login, account.Type, user)
	if err == nil && !saved {
		return "connected_elsewhere"
	}
	if err != nil {
		slog.Error("saving the GitHub installation failed", "err", err)
		return "failed"
	}
	metadata, _ := json.Marshal(M{"account": account.Login, "installationId": installation})
	_ = self.repo.Events().AddAudit(ctx, nil, workspace, &user, "github.connected", metadata)
	return ""
}

// saveLogin links the GitHub account of a user token to user, taking it from anyone who had it,
// and keeps the token to post their comments as them.
func (self *GitHubService) saveLogin(ctx context.Context, token github.UserToken, user uuid.UUID) error {
	raw, err := self.app.UserLogin(ctx, token.AccessToken)
	if err != nil {
		return err
	}
	login, ok := github.NormalizeLogin(raw)
	if !ok {
		return fmt.Errorf("%q isn't a GitHub login", raw)
	}
	return self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.github().SetUserLogin(ctx, tx, user, login); err != nil {
			return err
		}
		return self.github().SaveUserToken(ctx, tx, user, self.seal(token))
	})
}

// StartLink is where to send a signed-in person to link their GitHub account.
func (self *GitHubService) StartLink(session Session, returnTo string) string {
	returnTo = signin_service.SafeReturnTo(returnTo)
	if !session.SignedIn {
		return withGitHubError(returnTo, "signed_out")
	}
	if self.app == nil {
		return withGitHubError(returnTo, "not_configured")
	}
	link := self.keys.Sign(fmt.Sprintf("github-link:%d:%s:%s", time.Now().Add(linkTTL).Unix(), session.User, returnTo))
	return self.app.AuthorizeURL(self.cfg.PublicURL+"/api/github/callback", link)
}

// FinishLink links the GitHub account GitHub sent back with code, answering where to send the
// person on.
func (self *GitHubService) FinishLink(ctx context.Context, session Session, params url.Values) string {
	user, returnTo, err := self.readLink(session, params.Get("state"))
	if err != nil {
		return withGitHubError(returnTo, "expired")
	}
	token, err := self.app.ExchangeCode(ctx, params.Get("code"))
	if err == nil {
		err = self.saveLogin(ctx, token, user)
	}
	if err != nil {
		slog.Warn("linking a GitHub account failed", "err", err)
		return withGitHubError(returnTo, "failed")
	}
	return returnTo
}

// readLink checks the state StartLink signed belongs to whoever is signed in.
func (self *GitHubService) readLink(session Session, state string) (uuid.UUID, string, error) {
	expired := errors.New("expired")
	payload, ok := self.keys.Verify(state)
	parts := strings.SplitN(payload, ":", 4)
	if !ok || self.app == nil || len(parts) != 4 || parts[0] != "github-link" {
		return uuid.Nil, "/", expired
	}
	returnTo := signin_service.SafeReturnTo(parts[3])
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expires < time.Now().Unix() {
		return uuid.Nil, returnTo, expired
	}
	user, err := uuid.Parse(parts[2])
	if err != nil || !session.SignedIn || session.User != user {
		return uuid.Nil, returnTo, expired
	}
	return user, returnTo, nil
}
