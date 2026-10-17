package account_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/schema/pgarray"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
)

type CreateAPIKey struct {
	Label string `json:"label"`
	// read, write and/or admin; all of them when left out.
	Scopes []string `json:"scopes"`
	// The workspaces it reaches; every workspace of yours, including ones you join later, when
	// left out.
	WorkspaceIDs []uuid.UUID `json:"workspaceIds"`
}

// CheckScopes orders the scopes and adds read, which every other implies. Nil is all of them.
func CheckScopes(scopes []string) ([]string, error) {
	if scopes == nil {
		scopes = auth.AllScopes
	}
	if len(scopes) == 0 {
		return nil, errdefs.BadRequest("Pick at least one scope")
	}
	for _, scope := range scopes {
		if !slices.Contains(auth.AllScopes, scope) {
			return nil, errdefs.BadRequest("Unknown scope %q", scope)
		}
	}
	checked := []string{"read"}
	for _, scope := range auth.AllScopes[1:] {
		if slices.Contains(scopes, scope) {
			checked = append(checked, scope)
		}
	}
	return checked, nil
}

// reach is, for each list of workspaces (nil for all), the names of those that still exist in
// lower(name) order.
func (self *AccountService) reach(ctx context.Context, lists []*pgarray.UUID) ([][]string, error) {
	var ids []uuid.UUID
	for _, list := range lists {
		if list != nil {
			ids = append(ids, *list...)
		}
	}
	names := make([][]string, len(lists))
	if len(ids) == 0 {
		return names, nil
	}
	workspaces, err := self.repo.Account().WorkspaceNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i, list := range lists {
		if list == nil {
			continue
		}
		for _, w := range workspaces {
			if slices.Contains(*list, w.ID) {
				names[i] = append(names[i], w.Name)
			}
		}
	}
	return names, nil
}

func apiKeyOf(k *ent.APIKey, workspaces []string) models.APIKey {
	return models.APIKey{
		ID: k.ID, Label: k.Label, KeyHint: k.KeyHint, Scopes: models.NonNil([]string(k.Scopes)), AllWorkspaces: k.WorkspaceIds == nil,
		Workspaces: models.NonNil(workspaces), CreatedAt: k.CreatedAt.UTC(), LastUsedAt: models.UTCTimePtr(k.LastUsedAt),
	}
}

func (self *AccountService) apiKeys(ctx context.Context, keys []*ent.APIKey) ([]models.APIKey, error) {
	lists := make([]*pgarray.UUID, len(keys))
	for i, k := range keys {
		lists[i] = k.WorkspaceIds
	}
	names, err := self.reach(ctx, lists)
	if err != nil {
		return nil, err
	}
	values := make([]models.APIKey, len(keys))
	for i, k := range keys {
		values[i] = apiKeyOf(k, names[i])
	}
	return values, nil
}

func (self *AccountService) APIKeys(ctx context.Context, user uuid.UUID) ([]models.APIKey, error) {
	keys, err := self.repo.Account().APIKeys(ctx, user)
	if err != nil {
		return nil, err
	}
	return self.apiKeys(ctx, keys)
}

func (self *AccountService) CreateAPIKey(ctx context.Context, user uuid.UUID, input *CreateAPIKey) (models.CreatedAPIKey, error) {
	label := strings.TrimSpace(input.Label)
	if label == "" || len(label) > 80 {
		return models.CreatedAPIKey{}, errdefs.BadRequest("The label is 1 to 80 characters")
	}
	scopes, err := CheckScopes(input.Scopes)
	if err != nil {
		return models.CreatedAPIKey{}, err
	}
	if err := self.CheckWorkspaces(ctx, user, input.WorkspaceIDs); err != nil {
		return models.CreatedAPIKey{}, err
	}
	if slices.Contains(scopes, "admin") {
		admin, err := self.repo.Account().AdminIn(ctx, user, input.WorkspaceIDs)
		if err != nil {
			return models.CreatedAPIKey{}, err
		}
		if !admin {
			return models.CreatedAPIKey{}, errdefs.BadRequest("Only workspace admins can make keys with the admin scope")
		}
	}
	secret := auth.RandomSecret(auth.APIKeyPrefix)
	hint := secret[:len(auth.APIKeyPrefix)+4] + "…" + secret[len(secret)-4:]
	created, err := self.repo.Account().CreateAPIKey(ctx, user, input.WorkspaceIDs, label, auth.SecretHash(secret), hint, scopes)
	if err != nil {
		return models.CreatedAPIKey{}, err
	}
	keys, err := self.apiKeys(ctx, []*ent.APIKey{created})
	if err != nil {
		return models.CreatedAPIKey{}, err
	}
	return models.CreatedAPIKey{Key: keys[0], Secret: secret}, nil
}

func (self *AccountService) DeleteAPIKey(ctx context.Context, user, id uuid.UUID) error {
	return self.repo.Account().DeleteAPIKey(ctx, id, user)
}

func (self *AccountService) ConnectedApps(ctx context.Context, user uuid.UUID) ([]models.ConnectedApp, error) {
	grants, err := self.repo.Account().ConnectedApps(ctx, user)
	if err != nil {
		return nil, err
	}
	lists := make([]*pgarray.UUID, len(grants))
	for i, g := range grants {
		lists[i] = g.WorkspaceIds
	}
	names, err := self.reach(ctx, lists)
	if err != nil {
		return nil, err
	}
	apps := make([]models.ConnectedApp, len(grants))
	for i, g := range grants {
		apps[i] = models.ConnectedApp{
			ID: g.ID, Name: g.Edges.Client.Name, Scopes: models.NonNil([]string(g.Scopes)), AllWorkspaces: g.WorkspaceIds == nil,
			Workspaces: models.NonNil(names[i]), CreatedAt: g.CreatedAt.UTC(), LastUsedAt: g.LastUsedAt.UTC(),
		}
	}
	return apps, nil
}

func (self *AccountService) RevokeConnectedApp(ctx context.Context, user, id uuid.UUID) error {
	return self.repo.Account().RevokeGrant(ctx, id, user)
}
