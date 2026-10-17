package account_service

import (
	"context"
	"strings"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	account_repo "ricrac/internal/repositories/account"
)

type SaveOAuthApp struct {
	Name         string   `json:"name"`
	Description  *string  `json:"description"`
	HomepageURL  *string  `json:"homepageUrl"`
	RedirectURIs []string `json:"redirectUris"`
}

func checkApp(input *SaveOAuthApp) error {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return errdefs.BadRequest("The name is 1 to 80 characters")
	}
	if len(input.RedirectURIs) == 0 || len(input.RedirectURIs) > 10 {
		return errdefs.BadRequest("Add 1 to 10 callback URLs")
	}
	for _, uri := range input.RedirectURIs {
		if !auth.IsAllowedRedirect(strings.TrimSpace(uri)) {
			return errdefs.BadRequest("Callback URL not allowed: %s (https, http on localhost, or an app scheme)", uri)
		}
	}
	if url := trimmed(input.HomepageURL); url != nil && !strings.HasPrefix(*url, "https://") && !strings.HasPrefix(*url, "http://") {
		return errdefs.BadRequest("The homepage is an http(s) URL")
	}
	return nil
}

// trimmed is the text without surrounding spaces, nil when there's none.
func trimmed(text *string) *string {
	if text == nil || strings.TrimSpace(*text) == "" {
		return nil
	}
	t := strings.TrimSpace(*text)
	return &t
}

func savedApp(input *SaveOAuthApp) account_repo.SavedApp {
	uris := make([]string, len(input.RedirectURIs))
	for i, uri := range input.RedirectURIs {
		uris[i] = strings.TrimSpace(uri)
	}
	return account_repo.SavedApp{
		Name: strings.TrimSpace(input.Name), Description: trimmed(input.Description), HomepageURL: trimmed(input.HomepageURL), RedirectURIs: uris,
	}
}

func oauthAppOf(c *ent.OAuthClient, users int64) models.OAuthApp {
	return models.OAuthApp{
		ID: c.ID, Name: c.Name, Description: c.Description, HomepageURL: c.HomepageURL, RedirectURIs: models.NonNil([]string(c.RedirectUris)),
		CreatedAt: c.CreatedAt.UTC(), Users: users,
	}
}

func (self *AccountService) OAuthApps(ctx context.Context, actor *auth.Actor) ([]models.OAuthApp, error) {
	clients, err := self.repo.Account().OAuthApps(ctx, actor.Workspace)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(clients))
	for i, c := range clients {
		ids[i] = c.ID
	}
	users, err := self.repo.Account().AppUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	apps := make([]models.OAuthApp, len(clients))
	for i, c := range clients {
		apps[i] = oauthAppOf(c, users[c.ID])
	}
	return apps, nil
}

func (self *AccountService) loadApp(ctx context.Context, actor *auth.Actor, id string) (models.OAuthApp, error) {
	found, err := self.repo.Account().OAuthApp(ctx, id, actor.Workspace)
	if err != nil {
		return models.OAuthApp{}, err
	}
	if found == nil {
		return models.OAuthApp{}, errdefs.NotFound("No such app")
	}
	users, err := self.repo.Account().AppUsers(ctx, []string{id})
	if err != nil {
		return models.OAuthApp{}, err
	}
	return oauthAppOf(found, users[id]), nil
}

func (self *AccountService) withSecret(ctx context.Context, actor *auth.Actor, id, secret string) (models.OAuthAppSecret, error) {
	loaded, err := self.loadApp(ctx, actor, id)
	if err != nil {
		return models.OAuthAppSecret{}, err
	}
	return models.OAuthAppSecret{App: loaded, ClientSecret: secret}, nil
}

// appID is 32 lowercase letters and digits.
func appID() string {
	var id strings.Builder
	for _, ch := range auth.RandomSecret("") {
		if id.Len() < 32 && (ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			id.WriteRune(ch)
		}
	}
	return strings.ToLower(id.String())
}

func (self *AccountService) CreateOAuthApp(ctx context.Context, actor *auth.Actor, input *SaveOAuthApp) (models.OAuthAppSecret, error) {
	if err := actor.RequireMember(); err != nil {
		return models.OAuthAppSecret{}, err
	}
	if err := checkApp(input); err != nil {
		return models.OAuthAppSecret{}, err
	}
	id := appID()
	secret := auth.RandomSecret("rr_cs_")
	if err := self.repo.Account().CreateOAuthApp(ctx, id, auth.SecretHash(secret), savedApp(input), actor.User, actor.Workspace); err != nil {
		return models.OAuthAppSecret{}, err
	}
	return self.withSecret(ctx, actor, id, secret)
}

// CheckAppOwner lets the app's creator and workspace admins change it.
func (self *AccountService) CheckAppOwner(ctx context.Context, actor *auth.Actor, id string) error {
	found, err := self.repo.Account().OAuthApp(ctx, id, actor.Workspace)
	if err != nil {
		return err
	}
	if found == nil {
		return errdefs.NotFound("No such app")
	}
	if (found.OwnerID == nil || *found.OwnerID != actor.User) && actor.Role < auth.Admin {
		return errdefs.Forbidden("Only the app's creator or an admin can change it")
	}
	return nil
}

func (self *AccountService) UpdateOAuthApp(ctx context.Context, actor *auth.Actor, id string, input *SaveOAuthApp) (models.OAuthApp, error) {
	if err := self.CheckAppOwner(ctx, actor, id); err != nil {
		return models.OAuthApp{}, err
	}
	if err := checkApp(input); err != nil {
		return models.OAuthApp{}, err
	}
	if err := self.repo.Account().UpdateOAuthApp(ctx, id, savedApp(input)); err != nil {
		return models.OAuthApp{}, err
	}
	return self.loadApp(ctx, actor, id)
}

func (self *AccountService) RotateOAuthSecret(ctx context.Context, actor *auth.Actor, id string) (models.OAuthAppSecret, error) {
	if err := self.CheckAppOwner(ctx, actor, id); err != nil {
		return models.OAuthAppSecret{}, err
	}
	secret := auth.RandomSecret("rr_cs_")
	if err := self.repo.Account().SetClientSecret(ctx, id, auth.SecretHash(secret)); err != nil {
		return models.OAuthAppSecret{}, err
	}
	return self.withSecret(ctx, actor, id, secret)
}

func (self *AccountService) DeleteOAuthApp(ctx context.Context, actor *auth.Actor, id string) error {
	if err := self.CheckAppOwner(ctx, actor, id); err != nil {
		return err
	}
	return self.repo.Account().DeleteOAuthClient(ctx, id)
}
