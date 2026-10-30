package models

// The assistant's answer streams as server-sent events: session first, then text and tool in
// the order they happen, and done or error last.

type AssistantSettings struct {
	Enabled bool     `json:"enabled"`
	UserIDs []string `json:"userIds" doc:"Who may ask Claude."`
}

type AssistantSession struct {
	SessionID string `json:"sessionId" doc:"Pass it with the next question to continue the conversation."`
}

type AssistantText struct {
	Text string `json:"text" doc:"The next piece of the answer's Markdown."`
}

type AssistantTool struct {
	Name string `json:"name" doc:"The operation the assistant runs, like list_issues."`
}

type AssistantDone struct {
	CostUSD float64 `json:"costUsd" doc:"What the question cost, as Claude Code estimates it."`
	Turns   int     `json:"turns"`
	Stopped *string `json:"stopped" doc:"Why it stopped early: turns, budget or error; null when it finished."`
}
