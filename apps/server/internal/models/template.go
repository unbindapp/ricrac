package models

import (
	"encoding/json"

	"ricrac/ent"
)

type Template struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	Type        string              `json:"type" enum:"issue,project,document"`
	Team        *TeamRef            `json:"team"`
	Creator     *UserRef            `json:"creator"`
	Recurrence  *TemplateRecurrence `json:"recurrence"`
	NextRunAt   *string             `json:"nextRunAt" doc:"When a recurring template next creates an issue."`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
}

type TemplateRecurrence struct {
	Every    int    `json:"every"`
	Unit     string `json:"unit" enum:"day,week,month"`
	StartsAt string `json:"startsAt"`
}

type TemplateDetail struct {
	Template
	Data M `json:"data" doc:"What the template fills in."`
}

type DeletedTemplate struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}

// TemplateOf needs the template's team and creator loaded.
func TemplateOf(t *ent.Template) Template {
	template := Template{
		ID: ID(t.ID), Name: t.Name, Description: t.Description, Type: t.Type, Team: TeamRefOf(t.Edges.Team),
		Creator: UserRefOf(t.Edges.Creator), NextRunAt: TimePtr(t.NextRunAt), CreatedAt: Time(t.CreatedAt), UpdatedAt: Time(t.UpdatedAt),
	}
	if len(t.Recurrence) > 0 {
		_ = json.Unmarshal(t.Recurrence, &template.Recurrence)
	}
	return template
}

// TemplateDetailOf needs the template's team and creator loaded.
func TemplateDetailOf(t *ent.Template) TemplateDetail {
	detail := TemplateDetail{Template: TemplateOf(t)}
	if len(t.Data) > 0 {
		_ = json.Unmarshal(t.Data, &detail.Data)
	}
	return detail
}
