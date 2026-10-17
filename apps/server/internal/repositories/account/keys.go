package account_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/apikey"
	"ricrac/ent/oauthclient"
	"ricrac/ent/oauthgrant"
	"ricrac/ent/oauthtoken"
	"ricrac/ent/schema/pgarray"
	"ricrac/ent/workspace"
	repository "ricrac/internal/repositories"
)

// WorkspaceNames is the names of the workspaces by id, in lower(name) order.
func (self *AccountRepository) WorkspaceNames(ctx context.Context, ids []uuid.UUID) ([]*ent.Workspace, error) {
	return self.base.DB.Workspace.Query().
		Where(workspace.IDIn(ids...)).
		Modify(func(s *entsql.Selector) { s.OrderExpr(entsql.Expr("lower(" + s.C(workspace.FieldName) + ")")) }).
		All(ctx)
}

// APIKeys is the user's keys, newest first.
func (self *AccountRepository) APIKeys(ctx context.Context, user uuid.UUID) ([]*ent.APIKey, error) {
	return self.base.DB.APIKey.Query().Where(apikey.UserID(user)).Order(ent.Desc(apikey.FieldCreatedAt)).All(ctx)
}

func (self *AccountRepository) CreateAPIKey(ctx context.Context, user uuid.UUID, workspaces []uuid.UUID, label, hash, hint string, scopes []string) (*ent.APIKey, error) {
	create := self.base.DB.APIKey.Create().SetUserID(user).SetLabel(label).SetKeyHash(hash).SetKeyHint(hint).SetScopes(pgarray.Text(scopes))
	if workspaces != nil {
		create.SetWorkspaceIds(pgarray.UUID(workspaces))
	}
	created, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	return self.base.DB.APIKey.Get(ctx, created.ID)
}

func (self *AccountRepository) DeleteAPIKey(ctx context.Context, id, user uuid.UUID) error {
	_, err := self.base.DB.APIKey.Delete().Where(apikey.ID(id), apikey.UserID(user)).Exec(ctx)
	return err
}

// OAuthApps is the workspace's apps, newest first.
func (self *AccountRepository) OAuthApps(ctx context.Context, workspaceID uuid.UUID) ([]*ent.OAuthClient, error) {
	return self.base.DB.OAuthClient.Query().
		Where(oauthclient.WorkspaceID(workspaceID)).
		Order(ent.Desc(oauthclient.FieldCreatedAt)).
		All(ctx)
}

// OAuthApp is the workspace's app, nil when there's none.
func (self *AccountRepository) OAuthApp(ctx context.Context, id string, workspaceID uuid.UUID) (*ent.OAuthClient, error) {
	found, err := self.base.DB.OAuthClient.Query().Where(oauthclient.ID(id), oauthclient.WorkspaceID(workspaceID)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// AppUsers is how many people authorized each of the clients.
func (self *AccountRepository) AppUsers(ctx context.Context, clients []string) (map[string]int64, error) {
	grants, err := self.base.DB.OAuthGrant.Query().Where(oauthgrant.ClientIDIn(clients...)).All(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]map[uuid.UUID]bool{}
	counts := map[string]int64{}
	for _, g := range grants {
		if seen[g.ClientID] == nil {
			seen[g.ClientID] = map[uuid.UUID]bool{}
		}
		if !seen[g.ClientID][g.UserID] {
			seen[g.ClientID][g.UserID] = true
			counts[g.ClientID]++
		}
	}
	return counts, nil
}

type SavedApp struct {
	Name         string
	Description  *string
	HomepageURL  *string
	RedirectURIs []string
}

func (self *AccountRepository) CreateOAuthApp(ctx context.Context, id, secretHash string, app SavedApp, owner, workspaceID uuid.UUID) error {
	return self.base.DB.OAuthClient.Create().SetID(id).SetSecretHash(secretHash).SetName(app.Name).
		SetNillableDescription(app.Description).SetNillableHomepageURL(app.HomepageURL).SetRedirectUris(pgarray.Text(app.RedirectURIs)).
		SetOwnerID(owner).SetWorkspaceID(workspaceID).Exec(ctx)
}

func (self *AccountRepository) UpdateOAuthApp(ctx context.Context, id string, app SavedApp) error {
	update := self.base.DB.OAuthClient.Update().Where(oauthclient.ID(id)).SetName(app.Name).SetRedirectUris(pgarray.Text(app.RedirectURIs))
	if app.Description != nil {
		update.SetDescription(*app.Description)
	} else {
		update.ClearDescription()
	}
	if app.HomepageURL != nil {
		update.SetHomepageURL(*app.HomepageURL)
	} else {
		update.ClearHomepageURL()
	}
	return update.Exec(ctx)
}

func (self *AccountRepository) SetClientSecret(ctx context.Context, id, secretHash string) error {
	return self.base.DB.OAuthClient.Update().Where(oauthclient.ID(id)).SetSecretHash(secretHash).Exec(ctx)
}

func (self *AccountRepository) DeleteOAuthClient(ctx context.Context, id string) error {
	_, err := self.base.DB.OAuthClient.Delete().Where(oauthclient.ID(id)).Exec(ctx)
	return err
}

// ConnectedApps is the user's grants that still have a live token, with their client, most
// recently used first.
func (self *AccountRepository) ConnectedApps(ctx context.Context, user uuid.UUID) ([]*ent.OAuthGrant, error) {
	return self.base.DB.OAuthGrant.Query().
		Where(oauthgrant.UserID(user), oauthgrant.HasOauthTokensWith(future(oauthtoken.FieldExpiresAt))).
		WithClient().
		Order(ent.Desc(oauthgrant.FieldLastUsedAt)).
		All(ctx)
}

func (self *AccountRepository) RevokeGrant(ctx context.Context, id, user uuid.UUID) error {
	_, err := self.base.DB.OAuthGrant.Delete().Where(oauthgrant.ID(id), oauthgrant.UserID(user)).Exec(ctx)
	return err
}

// future is a predicate on a time column: later than now().
func future(column string) func(*entsql.Selector) {
	return func(s *entsql.Selector) { s.Where(repository.Raw(s.C(column) + " > now()")) }
}
