package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
	"ricrac/ops"
)

type signedInHandler func(w http.ResponseWriter, r *http.Request, user uuid.UUID) error

type memberHandler func(w http.ResponseWriter, r *http.Request, c *app.Ctx) error

// accountRoutes is the JSON API behind the dashboard (/api). Writes need the x-ricrac header,
// which cross-site forms can't send.
func accountRoutes(mux *http.ServeMux, state *app.State) {
	plain := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, requireAppHeader(handler))
	}
	signedIn := func(pattern string, handler signedInHandler) {
		plain(pattern, func(w http.ResponseWriter, r *http.Request) {
			user, err := signedInUser(state, r)
			if err == nil {
				err = handler(w, r, user)
			}
			if err != nil {
				app.WriteError(w, err)
			}
		})
	}
	member := func(pattern string, handler memberHandler) {
		signedIn(pattern, func(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
			c, err := memberCtx(state, r, user)
			if err != nil {
				return err
			}
			return handler(w, r, c)
		})
	}
	a := &account{state}
	plain("GET /api/config", a.config)
	plain("GET /api/me", a.me)
	signedIn("PATCH /api/me", a.updateMe)
	plain("GET /api/auth/sign-in", signIn(state))
	plain("GET /api/auth/callback/google", googleCallback(state))
	plain("POST /api/auth/sign-out", signOut(state))
	signedIn("POST /api/workspaces", a.createWorkspace)
	signedIn("DELETE /api/workspaces/{id}", a.deleteWorkspace)
	signedIn("POST /api/workspaces/{id}/leave", a.leaveWorkspace)
	signedIn("POST /api/invitations/{id}/accept", a.acceptInvitation)
	signedIn("DELETE /api/invitations/{id}", a.declineInvitation)
	signedIn("GET /api/api-keys", a.listAPIKeys)
	signedIn("POST /api/api-keys", a.createAPIKey)
	signedIn("DELETE /api/api-keys/{id}", a.deleteAPIKey)
	member("GET /api/oauth-apps", a.listOAuthApps)
	member("POST /api/oauth-apps", a.createOAuthApp)
	member("PATCH /api/oauth-apps/{id}", a.updateOAuthApp)
	member("DELETE /api/oauth-apps/{id}", a.deleteOAuthApp)
	member("POST /api/oauth-apps/{id}/secret", a.rotateOAuthSecret)
	signedIn("GET /api/connected-apps", a.connectedApps)
	signedIn("DELETE /api/connected-apps/{id}", a.revokeConnectedApp)
	plain("GET /api/oauth/client", a.oauthClient)
	signedIn("POST /api/oauth/decide", a.oauthDecide)
	plain("/api/", func(w http.ResponseWriter, r *http.Request) { app.WriteError(w, app.NotFound("Not found")) })
}

func requireAppHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exempt := strings.HasPrefix(r.URL.Path, "/api/auth/")
		if r.Method != http.MethodGet && !exempt && r.Header.Get("X-Ricrac") == "" {
			app.WriteError(w, app.Forbidden("Missing x-ricrac header"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func signedInUser(state *app.State, r *http.Request) (uuid.UUID, error) {
	user, ok := state.Keys.SessionUser(r)
	if !ok {
		return uuid.Nil, app.Unauthorized()
	}
	exists, err := app.One[bool](r.Context(), state.DB, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", user)
	if err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, app.Unauthorized()
	}
	return user, nil
}

// memberCtx is the signed-in user in the workspace of x-ricrac-workspace (id or URL key), or
// their only one.
func memberCtx(state *app.State, r *http.Request, user uuid.UUID) (*app.Ctx, error) {
	var named *string
	if value := strings.TrimSpace(r.Header.Get("X-Ricrac-Workspace")); value != "" {
		named = &value
	}
	ids, err := app.All[uuid.UUID](r.Context(), state.DB,
		`SELECT w.id FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
		 WHERE m.user_id = $1 AND m.active AND ($2::text IS NULL OR w.id::text = $2 OR w.url_key = lower($2))`, user, named)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, app.Forbidden("No such workspace")
	}
	if len(ids) > 1 {
		return nil, app.BadRequest("Name the workspace in the x-ricrac-workspace header")
	}
	c, err := app.LoadCtx(r.Context(), state, user, ids[0], app.EveryAccess)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, app.Forbidden("Not a member")
	}
	return c, nil
}

func decodeJSON(r *http.Request, into any) error {
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		return app.BadRequest("The body isn't valid JSON: %s", err)
	}
	return nil
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, app.BadRequest("Invalid id %q", r.PathValue("id"))
	}
	return id, nil
}

func ok(w http.ResponseWriter) error {
	app.WriteJSON(w, http.StatusOK, M{"ok": true})
	return nil
}

type account struct {
	state *app.State
}

type AppConfig struct {
	GoogleConfigured bool   `json:"googleConfigured"`
	PublicURL        string `json:"publicUrl"`
	MCPURL           string `json:"mcpUrl"`
}

func (a *account) config(w http.ResponseWriter, r *http.Request) {
	app.WriteJSON(w, http.StatusOK, AppConfig{
		GoogleConfigured: a.state.Config.GoogleConfigured(),
		PublicURL:        a.state.Config.PublicURL,
		MCPURL:           a.state.Config.MCPURL(),
	})
}

type WorkspaceSummary struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	URLKey string    `json:"urlKey"`
	Role   string    `json:"role"`
}

type PendingInvitation struct {
	ID            uuid.UUID `json:"id"`
	WorkspaceName string    `json:"workspaceName"`
	Role          string    `json:"role"`
	Inviter       *string   `json:"inviter"`
}

type Me struct {
	ID          uuid.UUID           `json:"id"`
	Email       string              `json:"email"`
	Name        string              `json:"name"`
	DisplayName string              `json:"displayName"`
	AvatarURL   *string             `json:"avatarUrl"`
	GitHubLogin *string             `json:"githubLogin"`
	Workspaces  []WorkspaceSummary  `json:"workspaces"`
	Invitations []PendingInvitation `json:"invitations"`
}

// me is the signed-in user, or null for visitors.
func (a *account) me(w http.ResponseWriter, r *http.Request) {
	me, err := a.loadMe(r)
	if err != nil {
		app.WriteError(w, err)
		return
	}
	app.WriteJSON(w, http.StatusOK, me)
}

func (a *account) loadMe(r *http.Request) (*Me, error) {
	ctx := r.Context()
	id, signedIn := a.state.Keys.SessionUser(r)
	if !signedIn {
		return nil, nil
	}
	user, found, err := app.Row[struct {
		Email, Name, DisplayName string
		AvatarURL, GitHubLogin   *string
	}](ctx, a.state.DB, "SELECT email, name, display_name, avatar_url, github_login FROM users WHERE id = $1", id)
	if err != nil || !found {
		return nil, err
	}
	workspaces, err := app.Rows[WorkspaceSummary](ctx, a.state.DB,
		`SELECT w.id, w.name, w.url_key, m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
		 WHERE m.user_id = $1 AND m.active ORDER BY w.name`, id)
	if err != nil {
		return nil, err
	}
	invitations, err := app.Rows[PendingInvitation](ctx, a.state.DB,
		`SELECT i.id, w.name AS workspace_name, i.role, u.name AS inviter FROM invitations i
		 JOIN workspaces w ON w.id = i.workspace_id LEFT JOIN users u ON u.id = i.inviter_id
		 WHERE i.email = $1 AND i.expires_at > now()
		 AND NOT EXISTS (SELECT 1 FROM workspace_members m WHERE m.workspace_id = i.workspace_id AND m.user_id = $2 AND m.active)`,
		user.Email, id)
	if err != nil {
		return nil, err
	}
	return &Me{
		ID: id, Email: user.Email, Name: user.Name, DisplayName: user.DisplayName, AvatarURL: user.AvatarURL, GitHubLogin: user.GitHubLogin,
		Workspaces: nonNil(workspaces), Invitations: nonNil(invitations),
	}, nil
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

type UpdateMe struct {
	Name        *string `json:"name"`
	DisplayName *string `json:"displayName"`
}

func validHandle(handle string) bool {
	return handle != "" && len(handle) <= 40 && strings.IndexFunc(handle, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '.' && c != '_' && c != '-'
	}) < 0
}

func (a *account) updateMe(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body UpdateMe
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	ctx := r.Context()
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len(name) > 100 {
			return app.BadRequest("Name must be 1 to 100 characters")
		}
		if err := app.Exec(ctx, a.state.DB, "UPDATE users SET name = $2 WHERE id = $1", user, name); err != nil {
			return err
		}
	}
	if body.DisplayName != nil {
		handle := strings.TrimSpace(*body.DisplayName)
		if !validHandle(handle) {
			return app.BadRequest("Display names are 1 to 40 letters, digits, dots, dashes or underscores")
		}
		if err := app.Exec(ctx, a.state.DB, "UPDATE users SET display_name = $2 WHERE id = $1", user, handle); err != nil {
			return err
		}
	}
	return ok(w)
}

type CreateWorkspace struct {
	Name string `json:"name"`
	// Lowercase letters, digits and dashes, used in links.
	URLKey string `json:"urlKey"`
	// The first team; named after the workspace when left out.
	TeamName *string `json:"teamName"`
	TeamKey  *string `json:"teamKey"`
}

func (a *account) createWorkspace(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body CreateWorkspace
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 80 {
		return app.BadRequest("The name is 1 to 80 characters")
	}
	urlKey, err := ops.CheckURLKey(body.URLKey)
	if err != nil {
		return err
	}
	teamName := name
	if body.TeamName != nil && strings.TrimSpace(*body.TeamName) != "" {
		teamName = strings.TrimSpace(*body.TeamName)
	}
	teamKey := ops.TeamKey(teamName)
	if body.TeamKey != nil {
		teamKey = *body.TeamKey
	}
	if teamKey, err = ops.CheckTeamKey(teamKey); err != nil {
		return err
	}
	ctx := r.Context()
	var summary WorkspaceSummary
	err = pgx.BeginFunc(ctx, a.state.DB, func(tx pgx.Tx) error {
		taken, err := app.One[bool](ctx, tx, "SELECT EXISTS (SELECT 1 FROM workspaces WHERE url_key = $1)", urlKey)
		if err != nil {
			return err
		}
		if taken {
			return app.Conflict("The URL key %q is taken", urlKey)
		}
		id, err := app.One[uuid.UUID](ctx, tx, "INSERT INTO workspaces (name, url_key) VALUES ($1, $2) RETURNING id", name, urlKey)
		if err != nil {
			return err
		}
		if err := app.Exec(ctx, tx, "INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')", id, user); err != nil {
			return err
		}
		if err := ops.WorkspaceDefaults(ctx, tx, id); err != nil {
			return err
		}
		if _, err := ops.CreateTeam(ctx, tx, id, user, teamName, teamKey); err != nil {
			return err
		}
		summary = WorkspaceSummary{ID: id, Name: name, URLKey: urlKey, Role: "owner"}
		return nil
	})
	if err != nil {
		return err
	}
	app.WriteJSON(w, http.StatusOK, summary)
	return nil
}

type DeleteWorkspace struct {
	ConfirmURLKey string `json:"confirmUrlKey"`
}

// deleteWorkspace deletes the workspace and everything in it. Owners only, confirmed by repeating
// its URL key.
func (a *account) deleteWorkspace(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var body DeleteWorkspace
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	ctx := r.Context()
	c, err := app.LoadCtx(ctx, a.state, user, id, nil)
	if err != nil {
		return err
	}
	if c == nil {
		return app.NotFound("No such workspace")
	}
	if c.Role != app.Owner {
		return app.Forbidden("Only owners can delete a workspace")
	}
	err = pgx.BeginFunc(ctx, a.state.DB, func(tx pgx.Tx) error {
		deleted, err := tx.Exec(ctx, "DELETE FROM workspaces WHERE id = $1 AND url_key = $2", id, strings.ToLower(strings.TrimSpace(body.ConfirmURLKey)))
		if err != nil {
			return err
		}
		if deleted.RowsAffected() != 1 {
			return app.BadRequest("Type the workspace's URL key to confirm")
		}
		return dropWorkspaceAccess(ctx, tx, id, nil)
	})
	if err != nil {
		return err
	}
	return ok(w)
}

// leaveWorkspace leaves the workspace. The last owner has to hand ownership over or delete it
// instead.
func (a *account) leaveWorkspace(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	err = pgx.BeginFunc(ctx, a.state.DB, func(tx pgx.Tx) error {
		owners, err := app.All[uuid.UUID](ctx, tx,
			"SELECT user_id FROM workspace_members WHERE workspace_id = $1 AND role = 'owner' AND active FOR UPDATE", id)
		if err != nil {
			return err
		}
		if len(owners) == 1 && owners[0] == user {
			return app.BadRequest("You're the only owner: make someone else an owner first, or delete the workspace")
		}
		if err := app.Exec(ctx, tx, "UPDATE workspace_members SET active = false WHERE workspace_id = $1 AND user_id = $2", id, user); err != nil {
			return err
		}
		err = app.Exec(ctx, tx,
			"DELETE FROM team_members m USING teams t WHERE t.id = m.team_id AND t.workspace_id = $1 AND m.user_id = $2", id, user)
		if err != nil {
			return err
		}
		return dropWorkspaceAccess(ctx, tx, id, &user)
	})
	if err != nil {
		return err
	}
	return ok(w)
}

// dropWorkspaceAccess takes the workspace out of the keys and OAuth grants that list it (of one
// user, or everyone's), deleting those left without a workspace. Those for all workspaces stop
// reaching it on their own.
func dropWorkspaceAccess(ctx context.Context, tx pgx.Tx, workspace uuid.UUID, user *uuid.UUID) error {
	for _, table := range []string{"api_keys", "oauth_grants", "oauth_codes"} {
		err := app.Exec(ctx, tx, fmt.Sprintf(
			"DELETE FROM %s WHERE workspace_ids = ARRAY[$1]::uuid[] AND ($2::uuid IS NULL OR user_id = $2)", table), workspace, user)
		if err != nil {
			return err
		}
		err = app.Exec(ctx, tx, fmt.Sprintf(
			`UPDATE %s SET workspace_ids = array_remove(workspace_ids, $1)
			 WHERE $1 = ANY(workspace_ids) AND ($2::uuid IS NULL OR user_id = $2)`, table), workspace, user)
		if err != nil {
			return err
		}
	}
	return nil
}

// accessColumns says which workspaces the rows of alias (keys or grants) reach.
func accessColumns(alias string) string {
	return fmt.Sprintf(`%[1]s.workspace_ids IS NULL AS all_workspaces,
		ARRAY(SELECT w.name FROM workspaces w WHERE w.id = ANY(%[1]s.workspace_ids) ORDER BY lower(w.name)) AS workspaces`, alias)
}

func (a *account) acceptInvitation(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	var summary WorkspaceSummary
	err = pgx.BeginFunc(ctx, a.state.DB, func(tx pgx.Tx) error {
		invitation, found, err := app.Row[struct {
			Workspace uuid.UUID
			Role      string
			Teams     []uuid.UUID
		}](ctx, tx,
			`DELETE FROM invitations i USING users u WHERE i.id = $1 AND u.id = $2 AND i.email = u.email AND i.expires_at > now()
			 RETURNING i.workspace_id, i.role, i.team_ids`, id, user)
		if err != nil {
			return err
		}
		if !found {
			return app.NotFound("No such invitation, or it expired")
		}
		err = app.Exec(ctx, tx,
			`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
			 ON CONFLICT (workspace_id, user_id) DO UPDATE SET active = true, role = excluded.role`,
			invitation.Workspace, user, invitation.Role)
		if err != nil {
			return err
		}
		err = app.Exec(ctx, tx,
			"INSERT INTO team_members (team_id, user_id) SELECT unnest($1::uuid[]), $2 ON CONFLICT DO NOTHING", invitation.Teams, user)
		if err != nil {
			return err
		}
		summary, _, err = app.Row[WorkspaceSummary](ctx, tx,
			`SELECT w.id, w.name, w.url_key, m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
			 WHERE w.id = $1 AND m.user_id = $2`, invitation.Workspace, user)
		return err
	})
	if err != nil {
		return err
	}
	app.WriteJSON(w, http.StatusOK, summary)
	return nil
}

func (a *account) declineInvitation(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	err = app.Exec(r.Context(), a.state.DB,
		"DELETE FROM invitations i USING users u WHERE i.id = $1 AND u.id = $2 AND i.email = u.email", id, user)
	if err != nil {
		return err
	}
	return ok(w)
}

type APIKey struct {
	ID      uuid.UUID `json:"id"`
	Label   string    `json:"label"`
	KeyHint string    `json:"keyHint"`
	Scopes  []string  `json:"scopes"`
	// Every workspace of its owner, including ones they join later.
	AllWorkspaces bool `json:"allWorkspaces"`
	// The names of the workspaces it reaches, when not all.
	Workspaces []string   `json:"workspaces"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

func (k *APIKey) utc() {
	k.CreatedAt = k.CreatedAt.UTC()
	if k.LastUsedAt != nil {
		t := k.LastUsedAt.UTC()
		k.LastUsedAt = &t
	}
	k.Scopes = nonNil(k.Scopes)
	k.Workspaces = nonNil(k.Workspaces)
}

func keyColumns() string {
	return "k.id, k.label, k.key_hint, k.scopes, " + accessColumns("k") + ", k.created_at, k.last_used_at"
}

func (a *account) listAPIKeys(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	keys, err := app.Rows[APIKey](r.Context(), a.state.DB,
		"SELECT "+keyColumns()+" FROM api_keys k WHERE k.user_id = $1 ORDER BY k.created_at DESC", user)
	if err != nil {
		return err
	}
	for i := range keys {
		keys[i].utc()
	}
	app.WriteJSON(w, http.StatusOK, nonNil(keys))
	return nil
}

type CreateAPIKey struct {
	Label string `json:"label"`
	// read, write and/or admin; all of them when left out.
	Scopes []string `json:"scopes"`
	// The workspaces it reaches; every workspace of yours, including ones you join later, when
	// left out.
	WorkspaceIDs []uuid.UUID `json:"workspaceIds"`
}

type CreatedAPIKey struct {
	Key APIKey `json:"key"`
	// Shown once.
	Secret string `json:"secret"`
}

// checkScopes orders the scopes and adds read, which every other implies. Nil is all of them.
func checkScopes(scopes []string) ([]string, error) {
	if scopes == nil {
		scopes = app.AllScopes
	}
	if len(scopes) == 0 {
		return nil, app.BadRequest("Pick at least one scope")
	}
	for _, scope := range scopes {
		if !slices.Contains(app.AllScopes, scope) {
			return nil, app.BadRequest("Unknown scope %q", scope)
		}
	}
	checked := []string{"read"}
	for _, scope := range app.AllScopes[1:] {
		if slices.Contains(scopes, scope) {
			checked = append(checked, scope)
		}
	}
	return checked, nil
}

func (a *account) createAPIKey(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body CreateAPIKey
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	label := strings.TrimSpace(body.Label)
	if label == "" || len(label) > 80 {
		return app.BadRequest("The label is 1 to 80 characters")
	}
	scopes, err := checkScopes(body.Scopes)
	if err != nil {
		return err
	}
	ctx := r.Context()
	if err := checkWorkspaces(ctx, a.state, user, body.WorkspaceIDs); err != nil {
		return err
	}
	if slices.Contains(scopes, "admin") {
		admin, err := app.One[bool](ctx, a.state.DB,
			`SELECT EXISTS (SELECT 1 FROM workspace_members WHERE user_id = $1 AND active AND role IN ('owner', 'admin')
			 AND ($2::uuid[] IS NULL OR workspace_id = ANY($2)))`, user, body.WorkspaceIDs)
		if err != nil {
			return err
		}
		if !admin {
			return app.BadRequest("Only workspace admins can make keys with the admin scope")
		}
	}
	secret := app.RandomSecret(APIKeyPrefix)
	hint := secret[:len(APIKeyPrefix)+4] + "…" + secret[len(secret)-4:]
	key, _, err := app.Row[APIKey](ctx, a.state.DB,
		`WITH k AS (INSERT INTO api_keys (workspace_ids, user_id, label, key_hash, key_hint, scopes)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING *) SELECT `+keyColumns()+` FROM k`,
		body.WorkspaceIDs, user, label, app.SecretHash(secret), hint, scopes)
	if err != nil {
		return err
	}
	key.utc()
	app.WriteJSON(w, http.StatusOK, CreatedAPIKey{Key: key, Secret: secret})
	return nil
}

func (a *account) deleteAPIKey(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := app.Exec(r.Context(), a.state.DB, "DELETE FROM api_keys WHERE id = $1 AND user_id = $2", id, user); err != nil {
		return err
	}
	return ok(w)
}

type OAuthApp struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	HomepageURL  *string   `json:"homepageUrl"`
	RedirectURIs []string  `json:"redirectUris"`
	CreatedAt    time.Time `json:"createdAt"`
	// People who authorized it.
	Users int64 `json:"users"`
}

type SaveOAuthApp struct {
	Name         string   `json:"name"`
	Description  *string  `json:"description"`
	HomepageURL  *string  `json:"homepageUrl"`
	RedirectURIs []string `json:"redirectUris"`
}

type OAuthAppSecret struct {
	App OAuthApp `json:"app"`
	// Shown once.
	ClientSecret string `json:"clientSecret"`
}

const appColumns = `c.id, c.name, c.description, c.homepage_url, c.redirect_uris, c.created_at,
	(SELECT count(DISTINCT g.user_id) FROM oauth_grants g WHERE g.client_id = c.id) AS users`

func checkApp(body *SaveOAuthApp) error {
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 80 {
		return app.BadRequest("The name is 1 to 80 characters")
	}
	if len(body.RedirectURIs) == 0 || len(body.RedirectURIs) > 10 {
		return app.BadRequest("Add 1 to 10 callback URLs")
	}
	for _, uri := range body.RedirectURIs {
		if !IsAllowedRedirect(strings.TrimSpace(uri)) {
			return app.BadRequest("Callback URL not allowed: %s (https, http on localhost, or an app scheme)", uri)
		}
	}
	if url := trimmed(body.HomepageURL); url != nil && !strings.HasPrefix(*url, "https://") && !strings.HasPrefix(*url, "http://") {
		return app.BadRequest("The homepage is an http(s) URL")
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

func trimmedAll(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = strings.TrimSpace(s)
	}
	return out
}

func (a *account) listOAuthApps(w http.ResponseWriter, r *http.Request, c *app.Ctx) error {
	apps, err := app.Rows[OAuthApp](c, a.state.DB,
		"SELECT "+appColumns+" FROM oauth_clients c WHERE c.workspace_id = $1 ORDER BY c.created_at DESC", c.Workspace)
	if err != nil {
		return err
	}
	for i := range apps {
		apps[i].CreatedAt = apps[i].CreatedAt.UTC()
	}
	app.WriteJSON(w, http.StatusOK, nonNil(apps))
	return nil
}

func (a *account) createOAuthApp(w http.ResponseWriter, r *http.Request, c *app.Ctx) error {
	if err := c.RequireMember(); err != nil {
		return err
	}
	var body SaveOAuthApp
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if err := checkApp(&body); err != nil {
		return err
	}
	var id strings.Builder
	for _, ch := range app.RandomSecret("") {
		if id.Len() < 32 && (ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			id.WriteRune(ch)
		}
	}
	appID := strings.ToLower(id.String())
	secret := app.RandomSecret("rr_cs_")
	err := app.Exec(c, a.state.DB,
		`INSERT INTO oauth_clients (id, secret_hash, name, description, homepage_url, redirect_uris, owner_id, workspace_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		appID, app.SecretHash(secret), strings.TrimSpace(body.Name), trimmed(body.Description), trimmed(body.HomepageURL),
		trimmedAll(body.RedirectURIs), c.User, c.Workspace)
	if err != nil {
		return err
	}
	return a.writeAppSecret(w, c, appID, secret)
}

func (a *account) loadApp(c *app.Ctx, id string) (OAuthApp, error) {
	found, ok, err := app.Row[OAuthApp](c, a.state.DB,
		"SELECT "+appColumns+" FROM oauth_clients c WHERE c.id = $1 AND c.workspace_id = $2", id, c.Workspace)
	if err != nil {
		return found, err
	}
	if !ok {
		return found, app.NotFound("No such app")
	}
	found.CreatedAt = found.CreatedAt.UTC()
	return found, nil
}

func (a *account) writeAppSecret(w http.ResponseWriter, c *app.Ctx, id, secret string) error {
	loaded, err := a.loadApp(c, id)
	if err != nil {
		return err
	}
	app.WriteJSON(w, http.StatusOK, OAuthAppSecret{App: loaded, ClientSecret: secret})
	return nil
}

// checkAppOwner lets the app's creator and workspace admins change it.
func (a *account) checkAppOwner(c *app.Ctx, id string) error {
	owner, found, err := app.Maybe[*uuid.UUID](c, a.state.DB,
		"SELECT owner_id FROM oauth_clients WHERE id = $1 AND workspace_id = $2", id, c.Workspace)
	if err != nil {
		return err
	}
	if !found {
		return app.NotFound("No such app")
	}
	if (owner == nil || *owner != c.User) && c.Role < app.Admin {
		return app.Forbidden("Only the app's creator or an admin can change it")
	}
	return nil
}

func (a *account) updateOAuthApp(w http.ResponseWriter, r *http.Request, c *app.Ctx) error {
	id := r.PathValue("id")
	if err := a.checkAppOwner(c, id); err != nil {
		return err
	}
	var body SaveOAuthApp
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if err := checkApp(&body); err != nil {
		return err
	}
	err := app.Exec(c, a.state.DB,
		"UPDATE oauth_clients SET name = $2, description = $3, homepage_url = $4, redirect_uris = $5 WHERE id = $1",
		id, strings.TrimSpace(body.Name), trimmed(body.Description), trimmed(body.HomepageURL), trimmedAll(body.RedirectURIs))
	if err != nil {
		return err
	}
	loaded, err := a.loadApp(c, id)
	if err != nil {
		return err
	}
	app.WriteJSON(w, http.StatusOK, loaded)
	return nil
}

func (a *account) rotateOAuthSecret(w http.ResponseWriter, r *http.Request, c *app.Ctx) error {
	id := r.PathValue("id")
	if err := a.checkAppOwner(c, id); err != nil {
		return err
	}
	secret := app.RandomSecret("rr_cs_")
	if err := app.Exec(c, a.state.DB, "UPDATE oauth_clients SET secret_hash = $2 WHERE id = $1", id, app.SecretHash(secret)); err != nil {
		return err
	}
	return a.writeAppSecret(w, c, id, secret)
}

func (a *account) deleteOAuthApp(w http.ResponseWriter, r *http.Request, c *app.Ctx) error {
	id := r.PathValue("id")
	if err := a.checkAppOwner(c, id); err != nil {
		return err
	}
	if err := app.Exec(c, a.state.DB, "DELETE FROM oauth_clients WHERE id = $1", id); err != nil {
		return err
	}
	return ok(w)
}

type ConnectedApp struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Scopes []string  `json:"scopes"`
	// Every workspace of yours, including ones you join later.
	AllWorkspaces bool `json:"allWorkspaces"`
	// The names of the workspaces it reaches, when not all.
	Workspaces []string  `json:"workspaces"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}

func (a *account) connectedApps(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	apps, err := app.Rows[ConnectedApp](r.Context(), a.state.DB,
		`SELECT g.id, c.name, g.scopes, `+accessColumns("g")+`, g.created_at, g.last_used_at FROM oauth_grants g
		 JOIN oauth_clients c ON c.id = g.client_id
		 WHERE g.user_id = $1 AND EXISTS (SELECT 1 FROM oauth_tokens t WHERE t.grant_id = g.id AND t.expires_at > now())
		 ORDER BY g.last_used_at DESC`, user)
	if err != nil {
		return err
	}
	for i := range apps {
		apps[i].CreatedAt = apps[i].CreatedAt.UTC()
		apps[i].LastUsedAt = apps[i].LastUsedAt.UTC()
		apps[i].Scopes = nonNil(apps[i].Scopes)
		apps[i].Workspaces = nonNil(apps[i].Workspaces)
	}
	app.WriteJSON(w, http.StatusOK, nonNil(apps))
	return nil
}

func (a *account) revokeConnectedApp(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := app.Exec(r.Context(), a.state.DB, "DELETE FROM oauth_grants WHERE id = $1 AND user_id = $2", id, user); err != nil {
		return err
	}
	return ok(w)
}

func (a *account) oauthClient(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	optional := func(key string) *string {
		if !query.Has(key) {
			return nil
		}
		value := query.Get(key)
		return &value
	}
	for _, key := range []string{"client_id", "redirect_uri", "response_type"} {
		if !query.Has(key) {
			app.WriteError(w, app.BadRequest("Missing %s", key))
			return
		}
	}
	req := &OAuthRequest{
		ClientID:            query.Get("client_id"),
		RedirectURI:         query.Get("redirect_uri"),
		ResponseType:        query.Get("response_type"),
		CodeChallenge:       optional("code_challenge"),
		CodeChallengeMethod: optional("code_challenge_method"),
		State:               optional("state"),
		Scope:               optional("scope"),
		Resource:            optional("resource"),
	}
	info, err := checkRequest(r.Context(), a.state, req)
	if err != nil {
		app.WriteError(w, err)
		return
	}
	app.WriteJSON(w, http.StatusOK, info)
}

type OAuthDecision struct {
	Request OAuthRequest `json:"request"`
	// The workspaces to grant; all of the user's, including ones they join later, when left out.
	WorkspaceIDs []uuid.UUID `json:"workspaceIds"`
	Allow        bool        `json:"allow"`
}

type OAuthRedirect struct {
	Redirect string `json:"redirect"`
}

func (a *account) oauthDecide(w http.ResponseWriter, r *http.Request, user uuid.UUID) error {
	var body OAuthDecision
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	redirect, err := Decide(r.Context(), a.state, user, body.WorkspaceIDs, &body.Request, body.Allow)
	if err != nil {
		return err
	}
	app.WriteJSON(w, http.StatusOK, OAuthRedirect{Redirect: redirect})
	return nil
}
