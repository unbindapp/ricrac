package assistant_service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"ricrac/internal/models"
	assistant_repo "ricrac/internal/repositories/assistant"
)

const toolPrefix = "mcp__ricrac__"

// deniedTools keep the assistant from deleting, and from the operations beyond the everyday
// tools (members, settings, webhooks, ...).
var deniedTools = []string{toolPrefix + "delete_*", toolPrefix + "find_operations", toolPrefix + "call_operation"}

// Emit sends one server-sent event.
type Emit func(event string, data any) error

type streamLine struct {
	Type  string `json:"type"`
	Event *struct {
		Type         string `json:"type"`
		ContentBlock *struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content_block"`
		Delta *struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
	Subtype      string   `json:"subtype"`
	IsError      bool     `json:"is_error"`
	NumTurns     int      `json:"num_turns"`
	TotalCostUSD float64  `json:"total_cost_usd"`
	Result       string   `json:"result"`
	Errors       []string `json:"errors"`
}

// relay is what the CLI said, as it passes it on.
type relay struct {
	emit       Emit
	failed     bool
	wroteText  bool
	afterTool  bool
	called     bool
	result     *streamLine
	emitFailed error
}

func (r *relay) send(event string, data any) {
	if r.emitFailed != nil {
		return
	}
	r.emitFailed = r.emit(event, data)
}

func (r *relay) line(data []byte) {
	var line streamLine
	if json.Unmarshal(data, &line) != nil {
		return
	}
	switch line.Type {
	case "stream_event":
		r.called = true
		r.streamEvent(&line)
	case "result":
		r.result = &line
	}
}

func (r *relay) streamEvent(line *streamLine) {
	event := line.Event
	if event == nil {
		return
	}
	switch {
	case event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == "tool_use":
		r.send("tool", models.AssistantTool{Name: strings.TrimPrefix(event.ContentBlock.Name, toolPrefix)})
		r.afterTool = true
	case event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == "text":
		if r.wroteText && !r.afterTool {
			r.send("text", models.AssistantText{Text: "\n\n"})
		}
	case event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type == "text_delta" && event.Delta.Text != "":
		r.send("text", models.AssistantText{Text: event.Delta.Text})
		r.wroteText = true
		r.afterTool = false
	}
}

// Answer runs the CLI, streams its answer to emit and records what the question cost. A
// question that ends without the CLI's own count is recorded at its whole budget.
func (q *Question) Answer(ctx context.Context, emit Emit) {
	defer q.service.release(q.actor.User)
	ctx, cancel := context.WithTimeout(ctx, questionTimeout)
	defer cancel()
	r := &relay{emit: emit}
	r.send("session", models.AssistantSession{SessionID: q.session.String()})
	failure := q.run(ctx, cancel, r)
	run := assistant_repo.Run{Workspace: q.actor.Workspace, User: q.actor.User, Session: q.session}
	stopped := q.outcome(r, failure, &run)
	if failure == "" && !r.wroteText && len(r.result.Errors) > 0 {
		failure = "The assistant failed: " + r.result.Errors[0]
	}
	if err := q.service.repo.Assistant().Record(context.WithoutCancel(ctx), run); err != nil {
		slog.Error("recording an assistant run failed", "err", err)
	}
	if failure != "" {
		r.send("error", map[string]string{"error": failure})
		return
	}
	r.send("done", models.AssistantDone{CostUSD: run.CostUSD, Turns: run.Turns, Stopped: stopped})
}

func (q *Question) outcome(r *relay, failure string, run *assistant_repo.Run) *string {
	result := r.result
	if result == nil {
		run.Outcome = "error"
		if r.called {
			run.CostUSD = q.budget
		}
		return nil
	}
	run.CostUSD, run.Turns = result.TotalCostUSD, result.NumTurns
	var stopped *string
	switch {
	case result.Subtype == "error_max_turns":
		stopped = new("turns")
	case result.Subtype == "error_max_budget_usd":
		stopped = new("budget")
	case result.Subtype != "success" || result.IsError:
		stopped = new("error")
	}
	run.Outcome = "finished"
	if stopped != nil {
		run.Outcome = *stopped
	}
	if failure == "" && !r.wroteText && result.Result != "" {
		r.send("text", models.AssistantText{Text: result.Result})
	}
	return stopped
}

// run is the CLI's run, relayed line by line; it answers why it failed, empty when it didn't.
func (q *Question) run(ctx context.Context, cancel context.CancelFunc, r *relay) string {
	settings := q.service.cfg.Assistant
	home, work, err := workDirs()
	if err != nil {
		return "The assistant couldn't start: " + err.Error()
	}
	mcpConfig, err := q.writeMCPConfig()
	if err != nil {
		return "The assistant couldn't start: " + err.Error()
	}
	defer os.Remove(mcpConfig)
	cmd := exec.CommandContext(ctx, settings.Claude, q.args(mcpConfig)...)
	cmd.Dir = work
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, settings.Credential, "DISABLE_AUTOUPDATER=1"}
	cmd.Stdin = strings.NewReader(q.prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{buffer: &stderr, limit: 4096}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "The assistant couldn't start: " + err.Error()
	}
	if err := cmd.Start(); err != nil {
		return "The assistant couldn't start: " + err.Error()
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		r.line(scanner.Bytes())
		if r.emitFailed != nil {
			cancel()
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return "The assistant took too long and was stopped"
	case r.emitFailed != nil || ctx.Err() != nil:
		return "Stopped"
	case r.result != nil:
		return ""
	case waitErr != nil:
		slog.Warn("the assistant's CLI failed", "err", waitErr, "stderr", stderr.String())
		return "The assistant failed: " + lastLine(stderr.String(), waitErr.Error())
	}
	return "The assistant ended without an answer"
}

func (q *Question) args(mcpConfig string) []string {
	settings := q.service.cfg.Assistant
	args := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--model", settings.Model,
		"--max-turns", strconv.Itoa(settings.MaxTurns),
		"--max-budget-usd", strconv.FormatFloat(q.budget, 'f', 2, 64),
		"--tools", "",
		"--strict-mcp-config", "--mcp-config", mcpConfig,
		"--allowedTools", toolPrefix + "*",
		"--disallowedTools", strings.Join(deniedTools, ","),
		"--permission-mode", "dontAsk",
		"--system-prompt", q.system,
	}
	if q.resume {
		return append(args, "--resume", q.session.String())
	}
	return append(args, "--session-id", q.session.String())
}

// writeMCPConfig writes the file that points the CLI at Ricrac's MCP with a token for this
// question; the caller removes it.
func (q *Question) writeMCPConfig() (string, error) {
	token := q.service.keys.AssistantToken(q.actor.User, q.actor.Workspace, questionTimeout+time.Minute)
	config := map[string]any{"mcpServers": map[string]any{"ricrac": map[string]any{
		"type":    "http",
		"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp", q.service.cfg.Port),
		"headers": map[string]string{"Authorization": "Bearer " + token},
	}}}
	file, err := os.CreateTemp("", "ricrac-assistant-mcp-*.json")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(config); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func lastLine(text, otherwise string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return last
	}
	return otherwise
}

type limitedWriter struct {
	buffer *bytes.Buffer
	limit  int
}

func (w *limitedWriter) Write(data []byte) (int, error) {
	if room := w.limit - w.buffer.Len(); room > 0 {
		w.buffer.Write(data[:min(len(data), room)])
	}
	return len(data), nil
}
