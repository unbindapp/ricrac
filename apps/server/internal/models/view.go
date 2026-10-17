package models

import (
	"regexp"
	"strings"

	"ricrac/ent"
)

type CustomView struct {
	ID          string   `json:"id"`
	SlugID      string   `json:"slugId"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Icon        *string  `json:"icon"`
	Color       *string  `json:"color"`
	Model       string   `json:"model" enum:"issue,project,initiative"`
	Filter      M        `json:"filter"`
	Display     M        `json:"display"`
	Shared      bool     `json:"shared"`
	Team        *TeamRef `json:"team"`
	Owner       *UserRef `json:"owner"`
	URL         string   `json:"url"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type DeletedCustomView struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}

var viewSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// ViewURL is the view's page in the web app: its name as a slug (60 characters at most) and its
// slug id.
func ViewURL(publicURL, workspaceKey, name, slugID string) string {
	slug := strings.Trim(viewSeparators.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return publicURL + "/" + workspaceKey + "/view/" + slugID
	}
	return publicURL + "/" + workspaceKey + "/view/" + slug + "-" + slugID
}

// CustomViewOf needs the view's workspace, team and owner loaded.
func CustomViewOf(v *ent.CustomView, publicURL string) CustomView {
	return CustomView{
		ID: ID(v.ID), SlugID: v.SlugID, Name: v.Name, Description: v.Description, Icon: v.Icon, Color: v.Color,
		Model: v.Model, Filter: Object(v.Filter), Display: Object(v.Display), Shared: v.Shared, Team: TeamRefOf(v.Edges.Team),
		Owner: UserRefOf(v.Edges.Owner), URL: ViewURL(publicURL, v.Edges.Workspace.URLKey, v.Name, v.SlugID),
		CreatedAt: Time(v.CreatedAt), UpdatedAt: Time(v.UpdatedAt),
	}
}
