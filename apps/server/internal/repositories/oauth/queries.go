package oauth_repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/schema/pgarray"
)

func (self *OAuthRepository) RegisterClient(ctx context.Context, id string, secretHash *string, name string, redirectURIs []string) error {
	return self.base.DB.OAuthClient.Create().SetID(id).SetNillableSecretHash(secretHash).SetName(name).
		SetRedirectUris(pgarray.Text(redirectURIs)).Exec(ctx)
}

// Client is the client, nil when there's none.
func (self *OAuthRepository) Client(ctx context.Context, id string) (*ent.OAuthClient, error) {
	found, err := self.base.DB.OAuthClient.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// SaveFetchedClient stores what a client ID metadata document says, noting it was just fetched.
func (self *OAuthRepository) SaveFetchedClient(ctx context.Context, id, name string, redirectURIs []string) error {
	_, err := self.base.DB.ExecContext(ctx,
		`INSERT INTO oauth_clients (id, name, redirect_uris, fetched_at) VALUES ($1, $2, $3, now())
		 ON CONFLICT (id) DO UPDATE SET name = excluded.name, redirect_uris = excluded.redirect_uris, fetched_at = now()`,
		id, name, redirectURIs)
	return err
}

type Code struct {
	ClientID      string
	UserID        uuid.UUID
	Workspaces    []uuid.UUID
	Scopes        []string
	RedirectURI   string
	CodeChallenge *string
	Resource      *string
	ExpiresAt     time.Time
}

// CreateCode stores an authorization code by its hash, valid for ttl seconds.
func (self *OAuthRepository) CreateCode(ctx context.Context, hash string, code Code, ttl int) error {
	_, err := self.base.DB.ExecContext(ctx,
		`INSERT INTO oauth_codes (code_hash, client_id, user_id, workspace_ids, scopes, redirect_uri, code_challenge, resource, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))`,
		hash, code.ClientID, code.UserID, code.Workspaces, code.Scopes, code.RedirectURI, code.CodeChallenge, code.Resource, float64(ttl))
	return err
}

// TakeCode deletes the code and returns it, nil when there's none.
func (self *OAuthRepository) TakeCode(ctx context.Context, hash string) (*Code, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		`DELETE FROM oauth_codes WHERE code_hash = $1
		 RETURNING client_id, user_id, workspace_ids, scopes, redirect_uri, code_challenge, resource, expires_at`, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var code Code
	var workspaces pgarray.UUID
	var scopes pgarray.Text
	err = rows.Scan(&code.ClientID, &code.UserID, &workspaces, &scopes, &code.RedirectURI, &code.CodeChallenge, &code.Resource, &code.ExpiresAt)
	if err != nil {
		return nil, err
	}
	code.Workspaces, code.Scopes = workspaces, scopes
	return &code, rows.Err()
}

func (self *OAuthRepository) CreateGrant(ctx context.Context, clientID string, user uuid.UUID, workspaces []uuid.UUID, scopes []string) (uuid.UUID, error) {
	create := self.base.DB.OAuthGrant.Create().SetClientID(clientID).SetUserID(user).SetScopes(pgarray.Text(scopes))
	if workspaces != nil {
		create.SetWorkspaceIds(pgarray.UUID(workspaces))
	}
	grant, err := create.Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return grant.ID, nil
}

// Grant is the grant, nil when there's none.
func (self *OAuthRepository) Grant(ctx context.Context, id uuid.UUID) (*ent.OAuthGrant, error) {
	found, err := self.base.DB.OAuthGrant.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

// CreateToken stores a token of the grant by its hash, valid for ttl seconds.
func (self *OAuthRepository) CreateToken(ctx context.Context, hash, kind string, grant uuid.UUID, ttl int) error {
	_, err := self.base.DB.ExecContext(ctx,
		"INSERT INTO oauth_tokens (token_hash, kind, grant_id, expires_at) VALUES ($1, $2, $3, now() + make_interval(secs => $4))",
		hash, kind, grant, float64(ttl))
	return err
}

type RefreshToken struct {
	Grant     uuid.UUID
	ExpiresAt time.Time
}

// TakeRefreshToken deletes the refresh token and returns it, nil when there's none.
func (self *OAuthRepository) TakeRefreshToken(ctx context.Context, hash string) (*RefreshToken, error) {
	rows, err := self.base.DB.QueryContext(ctx,
		"DELETE FROM oauth_tokens WHERE token_hash = $1 AND kind = 'refresh' RETURNING grant_id, expires_at", hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var token RefreshToken
	if err := rows.Scan(&token.Grant, &token.ExpiresAt); err != nil {
		return nil, err
	}
	return &token, rows.Err()
}

// RevokeGrantOf deletes the client's grant that the token belongs to.
func (self *OAuthRepository) RevokeGrantOf(ctx context.Context, hash, clientID string) error {
	_, err := self.base.DB.ExecContext(ctx,
		"DELETE FROM oauth_grants g USING oauth_tokens t WHERE t.token_hash = $1 AND t.grant_id = g.id AND g.client_id = $2",
		hash, clientID)
	return err
}

// Cleanup drops expired codes and tokens, grants without live tokens, and registrations never
// used.
func (self *OAuthRepository) Cleanup(ctx context.Context) error {
	for _, statement := range []string{
		"DELETE FROM oauth_codes WHERE expires_at < now()",
		"DELETE FROM oauth_tokens WHERE expires_at < now()",
		"DELETE FROM oauth_grants g WHERE NOT EXISTS (SELECT 1 FROM oauth_tokens t WHERE t.grant_id = g.id)",
		`DELETE FROM oauth_clients c WHERE c.owner_id IS NULL AND c.created_at < now() - interval '1 day'
		 AND NOT EXISTS (SELECT 1 FROM oauth_grants g WHERE g.client_id = c.id)`,
	} {
		if _, err := self.base.DB.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
