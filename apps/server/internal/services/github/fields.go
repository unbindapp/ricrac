package github_service

import (
	"context"
	"net/http"
	"strconv"

	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/integrations/github"
	"ricrac/internal/models"
	github_repo "ricrac/internal/repositories/github"
	issue_service "ricrac/internal/services/issue"
)

// orgFields is the issue fields of the organization that owns the repository, none when it has
// none or the app can't read them.
func (self *GitHubService) orgFields(ctx context.Context, token, fullName string) ([]github.IssueField, error) {
	var fields []github.IssueField
	_, err := self.app.Do(ctx, token, http.MethodGet, github.IssueFieldsPath(fullName), nil, &fields)
	if github.IsUnavailable(err) {
		return nil, nil
	}
	return fields, err
}

// pushIssueFields brings the changed priority, estimate, due date and cycle over to the
// organization's issue fields holding them, when the team syncs fields.
func (self *GitHubService) pushIssueFields(ctx context.Context, token string, entry github_repo.OutboxEntry, changes M) error {
	var slots []github.FieldSlot
	for _, slot := range github.FieldSlots {
		if _, ok := changes[slot.Key]; ok {
			slots = append(slots, slot)
		}
	}
	if len(slots) == 0 {
		return nil
	}
	sync, err := self.github().SyncOf(ctx, entry.RepoSync)
	if err != nil || sync == nil {
		return err
	}
	if synced, err := self.github().TeamSyncsFields(ctx, sync.TeamID); err != nil || !synced {
		return err
	}
	fields, err := self.orgFields(ctx, token, entry.FullName)
	if err != nil || len(fields) == 0 {
		return err
	}
	value, err := self.issues.Value(ctx, nil, *entry.Issue)
	if err != nil {
		return err
	}
	path := github.IssueFieldValuesPath(entry.FullName, entry.Number)
	var values []M
	for _, slot := range slots {
		field := slot.FieldFor(fields)
		if field == nil {
			continue
		}
		wanted, ok := fieldValue(slot.Key, value, field)
		if !ok {
			continue
		}
		if wanted != nil {
			values = append(values, M{"field_id": field.ID, "value": wanted})
			continue
		}
		_, err := self.app.Do(ctx, token, http.MethodDelete, path+"/"+strconv.FormatInt(field.ID, 10), nil, nil)
		if err != nil && !github.IsNotFound(err) {
			return err
		}
	}
	if len(values) == 0 {
		return nil
	}
	_, err = self.app.Do(ctx, token, http.MethodPost, path, M{"issue_field_values": values}, nil)
	return err
}

// fieldValue is what the issue field holding key gets, nil to clear it; false when the field
// can't hold the issue's value.
func fieldValue(key string, issue models.Issue, field *github.IssueField) (any, bool) {
	switch key {
	case "priority":
		name := github.PriorityName(issue.Priority)
		if name == "" {
			return nil, true
		}
		return name, field.HasOption(name)
	case "estimate":
		if issue.Estimate == nil {
			return nil, true
		}
		return *issue.Estimate, true
	case "dueDate":
		if issue.DueDate == nil {
			return nil, true
		}
		return *issue.DueDate, true
	case "cycle":
		if issue.Cycle == nil {
			return nil, true
		}
		return github.CycleText(issue.Cycle.Number, issue.Cycle.Name), true
	}
	return nil, false
}

// fieldChange is what the GitHub issue's field values change on the Ricrac issue, nil when
// nothing or the team doesn't sync fields.
func (self *GitHubService) fieldChange(ctx context.Context, actor *auth.Actor, sync *ent.GithubRepoSync, row *ent.Issue, number int) (*issue_service.SaveInput, error) {
	if synced, err := self.github().TeamSyncsFields(ctx, sync.TeamID); err != nil || !synced {
		return nil, err
	}
	token, err := self.app.InstallationToken(ctx, sync.InstallationID)
	if err != nil {
		return nil, err
	}
	fields, err := self.orgFields(ctx, token, sync.FullName)
	if err != nil || len(fields) == 0 {
		return nil, err
	}
	var values []github.IssueFieldValue
	_, err = self.app.Do(ctx, token, http.MethodGet, github.IssueFieldValuesPath(sync.FullName, number)+"?per_page=100", nil, &values)
	if github.IsUnavailable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	valueOf := func(field *github.IssueField) string {
		for _, value := range values {
			if value.FieldID == field.ID {
				return value.Text()
			}
		}
		return ""
	}
	input := &issue_service.SaveInput{}
	changed := false
	for _, slot := range github.FieldSlots {
		field := slot.FieldFor(fields)
		if field == nil {
			continue
		}
		applied, err := self.applyField(ctx, actor, row, slot.Key, valueOf(field), input)
		if err != nil {
			return nil, err
		}
		changed = changed || applied
	}
	if !changed {
		return nil, nil
	}
	return input, nil
}

// applyField sets on input the change a field's text (empty when unset) makes to the issue,
// saying whether there is one. Text Ricrac can't read changes nothing.
func (self *GitHubService) applyField(ctx context.Context, actor *auth.Actor, row *ent.Issue, key, text string, input *issue_service.SaveInput) (bool, error) {
	switch key {
	case "priority":
		priority, ok := github.PriorityOf(text)
		if text != "" && !ok || priority == row.Priority {
			return false, nil
		}
		input.Priority = &priority
		return true, nil
	case "estimate":
		if text == "" {
			if row.Estimate == nil {
				return false, nil
			}
			input.Estimate = args.Nullable[float64]{Set: true}
			return true, nil
		}
		estimate, err := strconv.ParseFloat(text, 64)
		if err != nil || row.Estimate != nil && *row.Estimate == estimate {
			return false, nil
		}
		input.Estimate = args.Nullable[float64]{Set: true, Valid: true, Value: estimate}
		return true, nil
	case "dueDate":
		due := text[:min(len(text), len("2006-01-02"))]
		if due == derefText(models.DatePtr(row.DueDate)) {
			return false, nil
		}
		input.DueDate = optionalDate(nonEmpty(due))
		return true, nil
	case "cycle":
		if text == "" {
			if row.CycleID == nil {
				return false, nil
			}
			input.Cycle = args.Nullable[string]{Set: true}
			return true, nil
		}
		cycle, err := self.resolve.Cycle(ctx, actor, &row.TeamID, github.CycleReference(text))
		if err != nil || samePtr(row.CycleID, &cycle) {
			return false, nil
		}
		input.Cycle = some(cycle.String())
		return true, nil
	}
	return false, nil
}

func nonEmpty(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}
