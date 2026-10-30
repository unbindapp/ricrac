package config

import (
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// Assistant is the web app's assistant: the Claude Code CLI, run with Ricrac's MCP tools.
type Assistant struct {
	Claude string
	// The credential handed to the CLI: ANTHROPIC_API_KEY=... or CLAUDE_CODE_OAUTH_TOKEN=...
	Credential        string
	Model             string
	MaxTurns          int
	QuestionBudgetUSD float64
	DailyBudgetUSD    float64
	// Who may turn the assistant on and pick its users, in workspaces they administer; lowercase.
	Admins []string
}

// credentialVars are the CLI's credentials, in the order it prefers them.
var credentialVars = []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"}

// ParseAssistant is the assistant, nil when no credential is set.
func ParseAssistant() (*Assistant, error) {
	var credential string
	for _, name := range credentialVars {
		if value := env(name); value != "" {
			credential = name + "=" + value
			break
		}
	}
	if credential == "" {
		return nil, nil
	}
	claude, err := exec.LookPath(fallback(env("ASSISTANT_CLAUDE_PATH"), "claude"))
	if err != nil {
		return nil, errors.New("the claude CLI isn't installed: " + err.Error())
	}
	maxTurns, err := number("ASSISTANT_MAX_TURNS", 30)
	if err != nil {
		return nil, err
	}
	question, err := dollars("ASSISTANT_QUESTION_BUDGET_USD", 1)
	if err != nil {
		return nil, err
	}
	daily, err := dollars("ASSISTANT_DAILY_BUDGET_USD", 10)
	if err != nil {
		return nil, err
	}
	return &Assistant{
		Claude:            claude,
		Credential:        credential,
		Model:             fallback(env("ASSISTANT_MODEL"), "sonnet"),
		MaxTurns:          maxTurns,
		QuestionBudgetUSD: question,
		DailyBudgetUSD:    daily,
		Admins:            list(env("ASSISTANT_ADMINS")),
	}, nil
}

// Manages says whether the person with email may turn the assistant on and pick its users.
func (a *Assistant) Manages(email string) bool {
	return a != nil && slices.Contains(a.Admins, strings.ToLower(email))
}

func list(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.ToLower(strings.TrimSpace(item)); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func fallback(value, otherwise string) string {
	if value == "" {
		return otherwise
	}
	return value
}

func dollars(name string, otherwise float64) (float64, error) {
	value := env(name)
	if value == "" {
		return otherwise, nil
	}
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil || amount <= 0 {
		return 0, errors.New(name + " must be a positive amount of dollars")
	}
	return amount, nil
}
