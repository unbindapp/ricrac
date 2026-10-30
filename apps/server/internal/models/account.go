package models

import (
	"time"

	"github.com/google/uuid"
)

type AppConfig struct {
	GoogleConfigured bool   `json:"googleConfigured"`
	EmailConfigured  bool   `json:"emailConfigured"`
	PublicURL        string `json:"publicUrl"`
	MCPURL           string `json:"mcpUrl"`
}

type WorkspaceSummary struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	URLKey  string    `json:"urlKey"`
	Role    string    `json:"role"`
	LogoURL *string   `json:"logoUrl"`
	Theme   string    `json:"theme" enum:"default,dank" doc:"The look of the web app for everyone in the workspace."`
	// Their own pick in this workspace, over theme; null follows it.
	OwnTheme *string `json:"ownTheme" enum:"default,dank"`
	// They may ask Claude here.
	Assistant bool `json:"assistant"`
	// They may turn Claude on here and pick who uses it.
	ManagesAssistant bool `json:"managesAssistant"`
}

type PendingInvitation struct {
	ID            uuid.UUID `json:"id"`
	WorkspaceName string    `json:"workspaceName"`
	Role          string    `json:"role"`
	Inviter       *string   `json:"inviter"`
}

type Me struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	AvatarURL   *string   `json:"avatarUrl"`
	GitHubLogin *string   `json:"githubLogin"`
	// Ricrac holds a GitHub token of theirs, to post their comments on GitHub as them.
	GitHubAuthorized bool                `json:"githubAuthorized"`
	Workspaces       []WorkspaceSummary  `json:"workspaces"`
	Invitations      []PendingInvitation `json:"invitations"`
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

type CreatedAPIKey struct {
	Key    APIKey `json:"key"`
	Secret string `json:"secret"`
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

type OAuthAppSecret struct {
	App          OAuthApp `json:"app"`
	ClientSecret string   `json:"clientSecret"`
}

type ConnectedApp struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Scopes        []string  `json:"scopes"`
	AllWorkspaces bool      `json:"allWorkspaces"`
	Workspaces    []string  `json:"workspaces"`
	CreatedAt     time.Time `json:"createdAt"`
	LastUsedAt    time.Time `json:"lastUsedAt"`
}

type OAuthClientInfo struct {
	ClientName   string   `json:"client_name"`
	RedirectHost string   `json:"redirect_host"`
	Scopes       []string `json:"scopes"`
	// Registered in a dashboard by someone, rather than by the client itself.
	Verified bool `json:"verified"`
}

type OAuthRedirect struct {
	Redirect string `json:"redirect"`
}

func UTCTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// NonNil is list, or an empty one for nil, so it answers [] rather than null.
func NonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
