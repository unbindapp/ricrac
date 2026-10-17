package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"slices"
	"strings"

	"ricrac/app"
	"ricrac/ops"
)

// core is the tools listed by default. Every other operation is reached through find_operations
// and call_operation, so a session doesn't start with ~140 tool definitions; /mcp?tools=all lists
// them all instead.
var core = []string{
	"list_workspaces",
	"get_workspace",
	"list_users",
	"get_user",
	"list_teams",
	"get_team",
	"list_issue_statuses",
	"list_issue_labels",
	"save_issue_label",
	"list_issues",
	"get_issue",
	"save_issue",
	"update_issues",
	"delete_issue",
	"get_issue_history",
	"list_comments",
	"save_comment",
	"delete_comment",
	"add_reaction",
	"create_attachment",
	"prepare_file_upload",
	"upload_file",
	"list_projects",
	"get_project",
	"save_project",
	"list_project_statuses",
	"list_milestones",
	"save_milestone",
	"get_status_updates",
	"save_status_update",
	"list_initiatives",
	"get_initiative",
	"save_initiative",
	"list_cycles",
	"get_cycle",
	"list_documents",
	"get_document",
	"save_document",
	"list_custom_views",
	"list_templates",
	"get_template",
	"get_notifications",
	"mark_notification",
	"search",
}

type findOperationsArgs struct {
	Query *string `json:"query" doc:"Words to look for in operation names and descriptions, e.g. \"webhook\", \"customer\", \"release note\", \"favorite\", \"archive project\". Leave out to list every operation by name."`
}

type callOperationArgs struct {
	Name      string `json:"name" doc:"The operation's name, from find_operations."`
	Arguments M      `json:"arguments" doc:"Its arguments, as described by find_operations."`
}

const findDescription = "Find operations beyond the tools listed here: webhooks, customers and customer needs, " +
	"releases, pipelines and release notes, favorites, document versions, templates and custom views (saving/deleting), " +
	"project and issue labels and statuses (saving/deleting), cycles (saving/archiving), teams and members (saving, " +
	"inviting, roles), archiving/restoring, sharing, the audit log, importing issues from another tracker, the GitHub " +
	"integration, and more. With a query, returns the matching operations " +
	"with their argument schemas; call them with call_operation."

const callDescription = "Run any Ricrac operation by name with its arguments (see find_operations). Returns " +
	"the operation's result."

const Instructions = `Ricrac is an issue tracker for agents: a workspace has teams; teams have issues (identifiers like ENG-123), workflow statuses (triage, backlog, unstarted, started, completed, canceled), labels, cycles and templates; projects (with milestones, status updates and documents) span teams and roll up into initiatives. There are also comments, attachments, custom views, favorites, an inbox of notifications, customers, releases and webhooks.

Referencing things: wherever a tool takes a person, team, status, label, project, cycle or milestone you may pass a name, key or identifier instead of an id. People also take an email or "me". Issues take their identifier (ENG-123), id or link. Errors list the valid options when a name doesn't match.

Writing: descriptions, comments, documents and updates are Markdown; write literal newlines, not escape sequences. Mention people with @displayName or @email: they are notified and subscribed. To change part of a long description or document, pass ` + "`patch`" + ` edits instead of resending it all.

Workspaces: a connection may reach several workspaces; list_workspaces lists them. When it reaches more than one, every tool takes a ` + "`workspace`" + ` argument (the URL key, like "acme") naming the one to act in. Identifiers like ENG-123 and names are only unique within a workspace; Ricrac links already name theirs.

Saving: save_* tools create when you omit ` + "`id`" + ` and update when you pass it; omitted fields stay as they are and null clears a field. List tools page with ` + "`limit` and `cursor`" + ` (pass pageInfo.endCursor). Start with get_workspace or list_teams to learn the teams and their keys, and list_issue_statuses for a team's workflow.

More operations: the tools listed cover everyday work. Everything else (webhooks, customers, releases, favorites, templates and views management, team and member administration, archiving, sharing, ...) is found with find_operations and run with call_operation.`

// protocolVersions are the MCP versions spoken, oldest first.
var protocolVersions = []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25"}

type callerKey struct{}

// rpcRequest is a JSON-RPC 2.0 request, or a notification when it has no id.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// mcpHandler is the MCP server at /mcp over streamable HTTP, statelessly: every POST carries
// one JSON-RPC message or a batch and is answered with JSON.
func mcpHandler(state *app.State) http.Handler {
	allowed := []string{"localhost", "127.0.0.1"}
	if _, host, ok := strings.Cut(state.Config.PublicURL, "://"); ok {
		allowed = append(allowed, strings.TrimSuffix(host, "/"))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, allowed) {
			http.Error(w, "Forbidden: Host header is not allowed", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := readBody(r)
		if err != nil {
			app.WriteError(w, err)
			return
		}
		body = bytes.TrimSpace(body)
		call := &mcpCall{caller: r.Context().Value(callerKey{}).(*app.Caller), all: r.URL.Query().Get("tools") == "all"}
		if len(body) > 0 && body[0] == '[' {
			var batch []json.RawMessage
			if err := json.Unmarshal(body, &batch); err != nil || len(batch) == 0 {
				app.WriteJSON(w, http.StatusBadRequest, parseError())
				return
			}
			responses := []rpcResponse{}
			for _, message := range batch {
				if response := call.handle(message); response != nil {
					responses = append(responses, *response)
				}
			}
			if len(responses) == 0 {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			app.WriteJSON(w, http.StatusOK, responses)
			return
		}
		response := call.handle(body)
		if response == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		status := http.StatusOK
		if response.Error != nil && response.Error.Code == -32700 {
			status = http.StatusBadRequest
		}
		app.WriteJSON(w, status, response)
	})
}

// hostAllowed checks the Host header against DNS rebinding: entries without a port allow any.
func hostAllowed(host string, allowed []string) bool {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.Trim(name, "[]")
	for _, entry := range allowed {
		if strings.EqualFold(entry, host) || strings.EqualFold(entry, name) {
			return true
		}
	}
	return false
}

func parseError() rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "Parse error"}}
}

type mcpCall struct {
	caller *app.Caller
	all    bool
}

// handle answers one message; nil for notifications and responses, which get no answer.
func (m *mcpCall) handle(message json.RawMessage) *rpcResponse {
	var request rpcRequest
	if err := json.Unmarshal(message, &request); err != nil {
		response := parseError()
		return &response
	}
	if len(request.ID) == 0 || string(request.ID) == "null" {
		return nil
	}
	response := &rpcResponse{JSONRPC: "2.0", ID: request.ID}
	if request.JSONRPC != "2.0" || request.Method == "" {
		response.Error = &rpcError{Code: -32600, Message: "Invalid request"}
		return response
	}
	result, err := m.dispatch(request.Method, request.Params)
	if err != nil {
		response.Error = err
		return response
	}
	response.Result = result
	return response
}

func (m *mcpCall) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		var init struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(params, &init)
		version := protocolVersions[len(protocolVersions)-1]
		if slices.Contains(protocolVersions, init.ProtocolVersion) {
			version = init.ProtocolVersion
		}
		return M{
			"protocolVersion": version,
			"capabilities":    M{"tools": M{}},
			"serverInfo":      M{"name": "ricrac", "version": Version},
			"instructions":    Instructions,
		}, nil
	case "ping":
		return M{}, nil
	case "tools/list":
		reach, err := reachOf(m.caller)
		if err != nil {
			return nil, &rpcError{Code: -32603, Message: app.AsError(err).Message}
		}
		tools := []any{}
		for _, op := range ops.All() {
			if reach.sees(op) && (m.all || slices.Contains(core, op.Name)) {
				tools = append(tools, tool(op, reach.several))
			}
		}
		if !m.all {
			tools = append(tools, metaTools()...)
		}
		return M{"tools": tools}, nil
	case "tools/call":
		var call struct {
			Name      string `json:"name"`
			Arguments M      `json:"arguments"`
		}
		if err := json.Unmarshal(params, &call); err != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid params: " + err.Error()}
		}
		return m.callTool(call.Name, call.Arguments)
	}
	return nil, &rpcError{Code: -32601, Message: "Method not found: " + method}
}

func (m *mcpCall) callTool(name string, args M) (any, *rpcError) {
	var value any
	var err error
	switch name {
	case "find_operations":
		reached, failed := reachOf(m.caller)
		if failed != nil {
			return nil, &rpcError{Code: -32603, Message: app.AsError(failed).Message}
		}
		var parsed findOperationsArgs
		if parsed, err = ops.Parse[findOperationsArgs](args); err == nil {
			value = findOperations(reached, parsed)
		}
	case "call_operation":
		var parsed callOperationArgs
		if parsed, err = ops.Parse[callOperationArgs](args); err == nil {
			value, err = callOperation(m.caller, parsed)
		}
	default:
		op := ops.Find(name)
		if op == nil {
			return nil, &rpcError{Code: -32602, Message: "Unknown tool " + name}
		}
		value, err = op.CallAs(m.caller, args, "")
	}
	if err != nil {
		return M{"content": []any{M{"type": "text", "text": app.AsError(err).Message}}, "isError": true}, nil
	}
	return M{"content": []any{M{"type": "text", "text": string(app.Marshal(value))}}, "isError": false}, nil
}

// reach is what the caller's token reaches, which decides the tools they see and their
// arguments.
type reach struct {
	scopes []app.Access
	// Admin in at least one of the workspaces.
	admin bool
	// More than one workspace, so every tool needs workspace.
	several bool
}

func reachOf(caller *app.Caller) (*reach, error) {
	reachable, err := caller.Reachable()
	if err != nil {
		return nil, err
	}
	admin := slices.ContainsFunc(reachable, func(w app.Reached) bool {
		role, ok := app.ParseRole(w.Role)
		return ok && role >= app.Admin
	})
	return &reach{scopes: caller.Scopes, admin: admin, several: len(reachable) > 1}, nil
}

// sees says whether the operation is offered: write tools need the write scope, admin tools an
// admin.
func (r *reach) sees(op *ops.Op) bool {
	return slices.Contains(r.scopes, op.Access) && (op.Access != app.AdminAccess || r.admin)
}

func findOperations(reach *reach, args findOperationsArgs) M {
	var candidates []*ops.Op
	for _, op := range ops.All() {
		if reach.sees(op) {
			candidates = append(candidates, op)
		}
	}
	query := ""
	if args.Query != nil {
		query = strings.ToLower(*args.Query)
	}
	if strings.TrimSpace(query) == "" {
		names := []any{}
		for _, op := range candidates {
			names = append(names, M{"name": op.Name, "summary": summary(op.Description)})
		}
		return M{"operations": names, "hint": "Search with a query to get argument schemas."}
	}
	var words []string
	for _, word := range strings.Fields(query) {
		words = append(words, strings.TrimRight(word, "s"))
	}
	type scored struct {
		score int
		op    *ops.Op
	}
	var matches []scored
	for _, op := range candidates {
		name := strings.ReplaceAll(op.Name, "_", " ")
		text := strings.ToLower(op.Description)
		score := 0
		for _, word := range words {
			if strings.Contains(name, word) {
				score += 3
			} else if strings.Contains(text, word) {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{score, op})
		}
	}
	slices.SortStableFunc(matches, func(a, b scored) int { return b.score - a.score })
	found := []any{}
	for _, match := range matches[:min(8, len(matches))] {
		found = append(found, M{"name": match.op.Name, "description": match.op.Description, "arguments": toolArguments(match.op, reach.several)})
	}
	return M{"operations": found}
}

func callOperation(caller *app.Caller, args callOperationArgs) (any, error) {
	op := ops.Find(strings.TrimSpace(args.Name))
	if op == nil {
		return nil, app.BadRequest("No operation %q; use find_operations", args.Name)
	}
	return op.CallAs(caller, args.Arguments, "")
}

// toolArguments is an operation's arguments, with workspace when the token reaches several.
func toolArguments(op *ops.Op, several bool) M {
	if several {
		return op.SchemaWithWorkspace(true)
	}
	return op.Schema
}

func tool(op *ops.Op, several bool) M {
	destructive := strings.HasPrefix(op.Name, "delete_") || strings.HasPrefix(op.Name, "remove_")
	return M{
		"name":        op.Name,
		"description": op.Description,
		"inputSchema": toolArguments(op, several),
		"annotations": M{
			"readOnlyHint":    op.ReadOnly(),
			"destructiveHint": destructive,
			"idempotentHint":  op.ReadOnly(),
			"openWorldHint":   false,
		},
	}
}

func metaTools() []any {
	return []any{
		M{
			"name":        "find_operations",
			"description": findDescription,
			"inputSchema": ops.SchemaOf[findOperationsArgs](),
			"annotations": M{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		M{
			"name":        "call_operation",
			"description": callDescription,
			"inputSchema": ops.SchemaOf[callOperationArgs](),
		},
	}
}

func withCaller(ctx context.Context, caller *app.Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, caller)
}
