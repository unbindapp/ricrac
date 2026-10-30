package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const answered = `{"type":"system","subtype":"init"}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"mcp__ricrac__list_issues"}}}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Two issues "}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"are open."}}}
{"type":"result","subtype":"success","is_error":false,"num_turns":2,"total_cost_usd":0.25,"result":"Two issues are open."}`

// fakeClaude is a script in place of the claude CLI: it keeps its arguments and prompt in dir,
// prints output as the CLI's stream-json and exits with exit.
func fakeClaude(t *testing.T, output string, exit int) (path, dir string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "claude")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %s/args\ncat > %s/prompt\ncat <<'JSON'\n%s\nJSON\necho boom >&2\nexit %d\n",
		dir, dir, output, exit)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// assistantSettings calls the assistant's settings as the web app does: GET without a body,
// PATCH with one.
func assistantSettings(c *Actor, body M) (int, J) {
	method, reader := "GET", io.Reader(nil)
	if body != nil {
		method, reader = "PATCH", bytes.NewReader(marshal(body))
	}
	req := httptest.NewRequest(method, "/api/assistant/settings", reader)
	req.Header.Set("Cookie", c.w.session(c.User))
	req.Header.Set("X-Ricrac", "1")
	req.Header.Set("X-Ricrac-Workspace", c.Workspace.String())
	return send(c.w.Router, req)
}

// enableAssistant turns the assistant on in the workspace for people, as Ann.
func enableAssistant(t *testing.T, w *World, people ...*Actor) {
	t.Helper()
	ids := []string{}
	for _, person := range people {
		ids = append(ids, person.User.String())
	}
	if status, value := assistantSettings(w.Ann, M{"enabled": true, "userIds": ids}); status != http.StatusOK {
		t.Fatalf("enable the assistant: %d %s", status, value.JSON())
	}
}

type sseEvent struct {
	Name string
	Data J
}

// ask puts a question to the assistant as the web app does, answering the status and the events
// (or the error answer as one "failed" event).
func ask(c *Actor, body M) (int, []sseEvent) {
	req := httptest.NewRequest("POST", "/api/assistant", bytes.NewReader(marshal(body)))
	req.Header.Set("Cookie", c.w.session(c.User))
	req.Header.Set("X-Ricrac", "1")
	req.Header.Set("X-Ricrac-Workspace", c.Workspace.String())
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	c.w.Router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		var value any
		_ = json.Unmarshal(recorder.Body.Bytes(), &value)
		return recorder.Code, []sseEvent{{Name: "failed", Data: J{value}}}
	}
	var events []sseEvent
	for block := range strings.SplitSeq(strings.TrimSpace(recorder.Body.String()), "\n\n") {
		var event sseEvent
		for line := range strings.SplitSeq(block, "\n") {
			if name, ok := strings.CutPrefix(line, "event: "); ok {
				event.Name = name
			}
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				var value any
				_ = json.Unmarshal([]byte(data), &value)
				event.Data = J{value}
			}
		}
		events = append(events, event)
	}
	return recorder.Code, events
}

func eventNames(events []sseEvent) []string {
	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.Name
	}
	return names
}

func TestAssistantStreamsItsAnswerAndContinuesTheConversation(t *testing.T) {
	w := world(t)
	claude, dir := fakeClaude(t, answered, 0)
	w.B.ConfigureAssistant(claude, 1, 10)
	enableAssistant(t, w, w.Ann)
	call(t, w.Ann, "save_document", M{"title": "Assistant", "team": "ENG", "content": "Bugs get the bug label."})

	status, events := ask(w.Ann, M{"prompt": "What's open?", "page": "/acme/issues"})
	if status != http.StatusOK {
		t.Fatalf("ask: %d %s", status, events[0].Data.JSON())
	}
	if got := eventNames(events); !slices.Equal(got, []string{"session", "tool", "text", "text", "done"}) {
		t.Fatalf("events %v", got)
	}
	if events[1].Data.At("name").Str() != "list_issues" {
		t.Fatalf("tool %s", events[1].Data.JSON())
	}
	if text := events[2].Data.At("text").Str() + events[3].Data.At("text").Str(); text != "Two issues are open." {
		t.Fatalf("text %q", text)
	}
	done := events[4].Data
	if done.At("costUsd").Num() != 0.25 || done.At("turns").Num() != 2 || done.At("stopped").V != nil {
		t.Fatalf("done %s", done.JSON())
	}
	session := events[0].Data.At("sessionId").Str()
	args := strings.Split(readFile(t, filepath.Join(dir, "args")), "\n")
	for _, want := range [][]string{
		{"--session-id", session},
		{"--max-budget-usd", "1.00"},
		{"--permission-mode", "dontAsk"},
		{"--tools", ""},
		{"--disallowedTools", "mcp__ricrac__delete_*,mcp__ricrac__find_operations,mcp__ricrac__call_operation"},
	} {
		at := slices.Index(args, want[0])
		if at < 0 || args[at+1] != want[1] {
			t.Fatalf("the CLI should get %s %q: %v", want[0], want[1], args)
		}
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "args")), "Bugs get the bug label.") {
		t.Fatal("the workspace's Assistant document goes into the system prompt")
	}
	if prompt := readFile(t, filepath.Join(dir, "prompt")); prompt != "(I'm looking at /acme/issues)\n\nWhat's open?" {
		t.Fatalf("prompt %q", prompt)
	}

	status, events = ask(w.Ann, M{"prompt": "And closed?", "sessionId": session})
	if status != http.StatusOK || events[0].Data.At("sessionId").Str() != session {
		t.Fatalf("follow-up: %d %v", status, events)
	}
	args = strings.Split(readFile(t, filepath.Join(dir, "args")), "\n")
	if at := slices.Index(args, "--resume"); at < 0 || args[at+1] != session {
		t.Fatalf("a follow-up resumes the conversation: %v", args)
	}
	spent := scalar[float64](w, "SELECT sum(cost_usd) FROM assistant_runs WHERE session_id = $1", uuid.MustParse(session))
	if spent != 0.5 {
		t.Fatalf("both questions are recorded, spent %v", spent)
	}
}

func TestAssistantKeepsToItsDailyBudget(t *testing.T) {
	w := world(t)
	claude, dir := fakeClaude(t, answered, 0)
	w.B.ConfigureAssistant(claude, 1, 0.3)
	enableAssistant(t, w, w.Ann)

	if status, _ := ask(w.Ann, M{"prompt": "One"}); status != http.StatusOK {
		t.Fatalf("first question: %d", status)
	}
	if status, _ := ask(w.Ann, M{"prompt": "Two"}); status != http.StatusOK {
		t.Fatalf("second question: %d", status)
	}
	args := strings.Split(readFile(t, filepath.Join(dir, "args")), "\n")
	if at := slices.Index(args, "--max-budget-usd"); args[at+1] != "0.05" {
		t.Fatalf("a question gets only what's left of the day's budget: %v", args[at+1])
	}
	status, events := ask(w.Ann, M{"prompt": "Three"})
	if status != http.StatusTooManyRequests || !strings.Contains(events[0].Data.At("error").Str(), "$0.30") {
		t.Fatalf("the spent budget refuses questions: %d %s", status, events[0].Data.JSON())
	}
}

func TestAssistantIsOnlyForThePeopleAnAdminPicks(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	guest := w.member("gus@acme.test", "Gus Guest", "guest")
	if status, _ := ask(w.Ann, M{"prompt": "Hi"}); status != http.StatusNotFound {
		t.Fatalf("off without a credential: %d", status)
	}
	claude, _ := fakeClaude(t, answered, 0)
	w.B.ConfigureAssistant(claude, 1, 10)
	if on := assistantOn(w, w.Ann.User)["acme"]; on.assistant || !on.manages {
		t.Fatalf("off until Ann turns it on, and hers to turn on: %+v", on)
	}
	if status, _ := ask(w.Ann, M{"prompt": "Hi"}); status != http.StatusNotFound {
		t.Fatalf("off until it's turned on: %d", status)
	}
	if on := assistantOn(w, bob.User)["acme"]; on.manages {
		t.Fatal("Bob isn't in ASSISTANT_ADMINS")
	}
	if status, _ := assistantSettings(bob, M{"enabled": true}); status != http.StatusForbidden {
		t.Fatalf("Bob can't turn it on: %d", status)
	}
	carl := w.member("carl@acme.test", "Carl Admin", "admin")
	if status, _ := assistantSettings(carl, nil); status != http.StatusForbidden {
		t.Fatalf("a workspace admin outside ASSISTANT_ADMINS can't see the settings: %d", status)
	}
	if status, _ := assistantSettings(w.Ann, M{"enabled": true, "userIds": []string{guest.User.String()}}); status != http.StatusBadRequest {
		t.Fatalf("guests can't be picked: %d", status)
	}

	enableAssistant(t, w, w.Ann)
	if _, settings := assistantSettings(w.Ann, nil); !settings.At("enabled").Bool() || settings.At("userIds").Len() != 1 || settings.At("userIds", 0).Str() != w.Ann.User.String() {
		t.Fatalf("settings %s", settings.JSON())
	}
	if !assistantOn(w, w.Ann.User)["acme"].assistant || assistantOn(w, bob.User)["acme"].assistant {
		t.Fatal("Ann has it and Bob doesn't")
	}
	_, events := ask(w.Ann, M{"prompt": "Hi"})
	session := events[0].Data.At("sessionId").Str()
	if status, _ := ask(bob, M{"prompt": "Hi"}); status != http.StatusNotFound {
		t.Fatalf("Bob wasn't picked: %d", status)
	}
	if status, _ := ask(guest, M{"prompt": "Hi"}); status != http.StatusForbidden {
		t.Fatalf("guests can't ask: %d", status)
	}

	enableAssistant(t, w, w.Ann, bob)
	status, events := ask(bob, M{"prompt": "What did Ann ask?", "sessionId": session})
	if status != http.StatusBadRequest || !strings.Contains(events[0].Data.At("error").Str(), "start a new one") {
		t.Fatalf("someone else's conversation is refused: %d %s", status, events[0].Data.JSON())
	}

	if status, _ := assistantSettings(w.Ann, M{"enabled": false}); status != http.StatusOK {
		t.Fatalf("turn it off: %d", status)
	}
	if status, _ := ask(w.Ann, M{"prompt": "Hi"}); status != http.StatusNotFound {
		t.Fatalf("off again: %d", status)
	}
	if count := scalar[int](w, "SELECT count(*) FROM audit_entries WHERE type = 'assistant.updated'"); count != 3 {
		t.Fatalf("each change is audited: %d", count)
	}
}

// assistantOn is what /api/me says about the assistant in each of the user's workspaces, by URL key.
func assistantOn(w *World, user uuid.UUID) map[string]struct{ assistant, manages bool } {
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", w.session(user))
	_, me := send(w.Router, req)
	on := map[string]struct{ assistant, manages bool }{}
	for _, workspace := range me.At("workspaces").List() {
		on[workspace.At("urlKey").Str()] = struct{ assistant, manages bool }{workspace.At("assistant").Bool(), workspace.At("managesAssistant").Bool()}
	}
	return on
}

func TestAssistantReportsAFailedRun(t *testing.T) {
	w := world(t)
	claude, _ := fakeClaude(t, "", 1)
	w.B.ConfigureAssistant(claude, 1, 10)
	enableAssistant(t, w, w.Ann)

	status, events := ask(w.Ann, M{"prompt": "Hi"})
	if status != http.StatusOK || !slices.Equal(eventNames(events), []string{"session", "error"}) {
		t.Fatalf("events %d %v", status, eventNames(events))
	}
	if message := events[1].Data.At("error").Str(); message != "The assistant failed: boom" {
		t.Fatalf("error %q", message)
	}
	if spent := scalar[float64](w, "SELECT coalesce(sum(cost_usd), 0) FROM assistant_runs"); spent != 0 {
		t.Fatalf("a run that never reached Claude costs nothing, spent %v", spent)
	}
}

func TestAssistantTokenReachesOneWorkspaceWithoutAdmin(t *testing.T) {
	w := world(t)
	other := w.secondWorkspace("Other", "other", "OTH")
	token := w.B.AssistantToken(w.Ann.User, w.Workspace, time.Minute)
	opWith := func(token, name string, args M) (int, J) {
		req := httptest.NewRequest("POST", "/v1/ops/"+name, bytes.NewReader(marshal(args)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		return send(w.Router, req)
	}

	if status, value := opWith(token, "list_issues", M{}); status != http.StatusOK {
		t.Fatalf("the token reads its workspace: %d %s", status, value.JSON())
	}
	if status, value := opWith(token, "save_issue", M{"title": "Filed by the assistant"}); status != http.StatusOK {
		t.Fatalf("the token writes: %d %s", status, value.JSON())
	}
	if status, _ := opWith(token, "list_issues", M{"workspace": other.String()}); status == http.StatusOK {
		t.Fatal("the token reaches only its workspace")
	}
	if status, _ := opWith(token, "list_webhooks", M{}); status != http.StatusForbidden {
		t.Fatalf("the token has no admin scope: %d", status)
	}
	expired := w.B.AssistantToken(w.Ann.User, w.Workspace, -time.Minute)
	if status, _ := opWith(expired, "list_issues", M{}); status != http.StatusUnauthorized {
		t.Fatalf("an expired token is refused: %d", status)
	}
}
