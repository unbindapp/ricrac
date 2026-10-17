package ops

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"ricrac/app"
)

// What new workspaces and teams start with.

// WorkspaceDefaults adds project statuses, a few labels and customer statuses.
func WorkspaceDefaults(ctx context.Context, tx app.Querier, workspace uuid.UUID) error {
	statuses := [][3]string{
		{"Backlog", "backlog", "#bec2c8"},
		{"Planned", "planned", "#e2e2e2"},
		{"In Progress", "started", "#f2c94c"},
		{"Paused", "paused", "#95a2b3"},
		{"Completed", "completed", "#5e6ad2"},
		{"Canceled", "canceled", "#95a2b3"},
	}
	for position, s := range statuses {
		if err := app.Exec(ctx, tx,
			"INSERT INTO project_statuses (workspace_id, name, type, color, position) VALUES ($1, $2, $3, $4, $5)",
			workspace, s[0], s[1], s[2], float64(position)); err != nil {
			return err
		}
	}
	for _, l := range [][2]string{{"Bug", "#eb5757"}, {"Feature", "#bb87fc"}, {"Improvement", "#4ea7fc"}} {
		if err := app.Exec(ctx, tx, "INSERT INTO issue_labels (workspace_id, name, color) VALUES ($1, $2, $3)",
			workspace, l[0], l[1]); err != nil {
			return err
		}
	}
	for position, s := range [][2]string{{"Active", "#4cb782"}, {"Prospect", "#f2c94c"}, {"Churned", "#95a2b3"}} {
		if err := app.Exec(ctx, tx,
			"INSERT INTO customer_statuses (workspace_id, name, color, position) VALUES ($1, $2, $3, $4)",
			workspace, s[0], s[1], float64(position)); err != nil {
			return err
		}
	}
	return nil
}

// TeamDefaults is the workflow every team starts with. New issues go to Backlog; issues closed by
// the auto-close automation go to Canceled.
func TeamDefaults(ctx context.Context, tx app.Querier, team uuid.UUID) error {
	states := [][3]string{
		{"Backlog", "backlog", "#bec2c8"},
		{"Todo", "unstarted", "#e2e2e2"},
		{"In Progress", "started", "#f2c94c"},
		{"In Review", "started", "#4cb782"},
		{"Done", "completed", "#5e6ad2"},
		{"Canceled", "canceled", "#95a2b3"},
		{"Duplicate", "canceled", "#95a2b3"},
	}
	for position, s := range states {
		if err := app.Exec(ctx, tx,
			"INSERT INTO workflow_states (team_id, name, type, color, position) VALUES ($1, $2, $3, $4, $5)",
			team, s[0], s[1], s[2], float64(position)); err != nil {
			return err
		}
	}
	return app.Exec(ctx, tx,
		`UPDATE teams SET
			default_issue_state_id = (SELECT id FROM workflow_states WHERE team_id = $1 AND name = 'Backlog'),
			auto_close_state_id = (SELECT id FROM workflow_states WHERE team_id = $1 AND name = 'Canceled')
		 WHERE id = $1`, team)
}

// EnsureTriageState adds the Triage status a team needs once triage is turned on.
func EnsureTriageState(ctx context.Context, tx app.Querier, team uuid.UUID) (uuid.UUID, error) {
	existing, found, err := app.Maybe[uuid.UUID](ctx, tx,
		"SELECT id FROM workflow_states WHERE team_id = $1 AND type = 'triage' AND archived_at IS NULL LIMIT 1", team)
	if err != nil || found {
		return existing, err
	}
	return app.One[uuid.UUID](ctx, tx,
		`INSERT INTO workflow_states (team_id, name, type, color, position)
		 VALUES ($1, 'Triage', 'triage', '#fc7840', (SELECT coalesce(min(position), 0) - 1 FROM workflow_states WHERE team_id = $1))
		 RETURNING id`, team)
}

// TeamKey is a team key from a name: its initials, or its first letters, up to five, uppercase.
func TeamKey(name string) string {
	var words []string
	for _, w := range strings.Fields(name) {
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			words = append(words, w)
		}
	}
	isASCIIAlnum := func(r rune) bool { return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) }
	var key []rune
	if len(words) > 1 {
		for _, w := range words {
			if i := strings.IndexFunc(w, isASCIIAlnum); i >= 0 && len(key) < 5 {
				key = append(key, rune(w[i]))
			}
		}
	} else {
		for _, r := range name {
			if isASCIIAlnum(r) && len(key) < 3 {
				key = append(key, r)
			}
		}
	}
	if len(key) == 0 {
		return "TEAM"
	}
	return strings.ToUpper(string(key))
}

// CreateTeam creates a team with the default workflow and the creator as its first member.
func CreateTeam(ctx context.Context, tx app.Querier, workspace, creator uuid.UUID, name, key string) (uuid.UUID, error) {
	team, err := app.One[uuid.UUID](ctx, tx,
		"INSERT INTO teams (workspace_id, name, key) VALUES ($1, $2, $3) RETURNING id", workspace, name, strings.ToUpper(key))
	if err != nil {
		return uuid.Nil, err
	}
	if err := TeamDefaults(ctx, tx, team); err != nil {
		return uuid.Nil, err
	}
	return team, app.Exec(ctx, tx, "INSERT INTO team_members (team_id, user_id, owner) VALUES ($1, $2, true)", team, creator)
}

// CheckURLKey checks a workspace URL key: 2 to 32 lowercase letters, digits and dashes, not a
// path the server uses.
func CheckURLKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	valid := len(key) >= 2 && len(key) <= 32 && !strings.HasPrefix(key, "-") && !strings.HasSuffix(key, "-")
	for _, b := range []byte(key) {
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-') {
			valid = false
		}
	}
	if !valid {
		return "", app.BadRequest("The URL key is 2 to 32 lowercase letters, digits and dashes")
	}
	reserved := []string{"api", "v1", "mcp", "oauth", "assets", "files", "settings", "login", "admin", "docs", "llms.txt",
		"uploads", "privacy", "terms"}
	if slices.Contains(reserved, key) {
		return "", app.BadRequest("%q is reserved", key)
	}
	return key, nil
}

// CheckTeamKey checks a team key: 1 to 7 letters and digits, starting with a letter.
func CheckTeamKey(key string) (string, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	valid := len(key) >= 1 && len(key) <= 7 && key[0] >= 'A' && key[0] <= 'Z'
	for _, b := range []byte(key) {
		if !(b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			valid = false
		}
	}
	if !valid {
		return "", app.BadRequest("A team key is 1 to 7 letters and digits, starting with a letter, like ENG")
	}
	return key, nil
}
