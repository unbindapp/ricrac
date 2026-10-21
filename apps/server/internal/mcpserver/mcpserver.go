// Package mcpserver serves MCP at /mcp. It has no tools of its own beyond find_operations and
// call_operation: every tool is an API operation, called in process.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"slices"
	"strings"

	"ricrac/config"
	"ricrac/internal/api/meta"
	"ricrac/internal/api/middleware"
	"ricrac/internal/api/oapi"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
)

type M = map[string]any

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
	"list_discussions",
	"get_discussion",
	"save_discussion",
	"get_discussion_thread",
	"save_discussion_thread",
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
	"releases, pipelines and release notes, favorites, document versions and suggestions, deleting discussions and threads, " +
	"templates and custom views (saving/deleting), " +
	"project and issue labels and statuses (saving/deleting), cycles (saving/archiving), teams and members (saving, " +
	"inviting, roles), archiving/restoring, sharing, the audit log, importing issues from another tracker, the GitHub " +
	"integration, and more. With a query, returns the matching operations " +
	"with their argument schemas; call them with call_operation."

const callDescription = "Run any Ricrac operation by name with its arguments (see find_operations). Returns " +
	"the operation's result."

// protocolVersions are the MCP versions spoken, oldest first.
var protocolVersions = []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"}

// statelessVersion is the first version where every result says its resultType and clients name
// the version on each request (MCP-Protocol-Version, or _meta) instead of a session.
const statelessVersion = "2026-07-28"

const versionMetaKey = "io.modelcontextprotocol/protocolVersion"

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

// Server is the MCP server at /mcp over streamable HTTP, statelessly: every POST carries one
// JSON-RPC message or a batch and is answered with JSON. Its tools are the API's operations.
type Server struct {
	cfg    config.Config
	ops    *oapi.Registry
	access Reacher
}

// Reacher lists the workspaces a caller reaches.
type Reacher interface {
	Reachable(ctx context.Context, caller *auth.Caller) ([]auth.Reached, error)
}

func New(cfg config.Config, ops *oapi.Registry, access Reacher) *Server {
	return &Server{cfg: cfg, ops: ops, access: access}
}

// Handler serves MCP for callers RequireBearer let in.
func (s *Server) Handler() http.Handler {
	allowed := []string{"localhost", "127.0.0.1"}
	if _, host, ok := strings.Cut(s.cfg.PublicURL, "://"); ok {
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
		body, err := oapi.ReadBody(r)
		if err != nil {
			errdefs.Write(w, err)
			return
		}
		body = bytes.TrimSpace(body)
		call := &mcpCall{
			server:  s,
			ctx:     r.Context(),
			caller:  middleware.CallerFrom(r.Context()),
			all:     r.URL.Query().Get("tools") == "all",
			version: r.Header.Get("MCP-Protocol-Version"),
		}
		if len(body) > 0 && body[0] == '[' {
			var batch []json.RawMessage
			if err := json.Unmarshal(body, &batch); err != nil || len(batch) == 0 {
				errdefs.WriteJSON(w, http.StatusBadRequest, parseError())
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
			errdefs.WriteJSON(w, http.StatusOK, responses)
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
		errdefs.WriteJSON(w, status, response)
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
	server *Server
	ctx    context.Context
	caller *auth.Caller
	all    bool
	// The version the client named in the MCP-Protocol-Version header, if any.
	version string
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
	if object, ok := result.(M); ok && m.requestVersion(request.Method, request.Params) >= statelessVersion {
		object["resultType"] = "complete"
		if request.Method == "tools/list" {
			object["ttlMs"] = 0
			object["cacheScope"] = "private"
		}
	}
	response.Result = result
	return response
}

// requestVersion is the protocol version a request is made in: the one initialize negotiates,
// else the one named in its _meta or the request's header.
func (m *mcpCall) requestVersion(method string, params json.RawMessage) string {
	var named struct {
		ProtocolVersion string         `json:"protocolVersion"`
		Meta            map[string]any `json:"_meta"`
	}
	_ = json.Unmarshal(params, &named)
	if method == "initialize" {
		return negotiate(named.ProtocolVersion)
	}
	if method == "server/discover" {
		return statelessVersion
	}
	if version, ok := named.Meta[versionMetaKey].(string); ok {
		return version
	}
	return m.version
}

// negotiate is the requested version when it's spoken, else the newest one spoken.
func negotiate(requested string) string {
	if slices.Contains(protocolVersions, requested) {
		return requested
	}
	return protocolVersions[len(protocolVersions)-1]
}

func (m *mcpCall) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		var init struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(params, &init)
		return M{
			"protocolVersion": negotiate(init.ProtocolVersion),
			"capabilities":    M{"tools": M{}},
			"serverInfo":      M{"name": "ricrac", "version": meta.Version},
			"instructions":    meta.Instructions,
		}, nil
	case "ping":
		return M{}, nil
	case "server/discover":
		return M{
			"supportedVersions": protocolVersions,
			"capabilities":      M{"tools": M{}},
			"instructions":      meta.Instructions,
			"ttlMs":             0,
			"cacheScope":        "private",
			"_meta":             M{"io.modelcontextprotocol/serverInfo": M{"name": "ricrac", "version": meta.Version}},
		}, nil
	case "tools/list":
		reach, err := m.reach()
		if err != nil {
			return nil, &rpcError{Code: -32603, Message: errdefs.As(err).Message}
		}
		tools := []any{}
		for _, op := range m.server.ops.All() {
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

func (m *mcpCall) callTool(name string, arguments M) (any, *rpcError) {
	var value any
	var err error
	switch name {
	case "find_operations":
		reached, failed := m.reach()
		if failed != nil {
			return nil, &rpcError{Code: -32603, Message: errdefs.As(failed).Message}
		}
		var parsed findOperationsArgs
		if parsed, err = args.Parse[findOperationsArgs](arguments); err == nil {
			value = m.server.findOperations(reached, parsed)
		}
	case "call_operation":
		var parsed callOperationArgs
		if parsed, err = args.Parse[callOperationArgs](arguments); err == nil {
			value, err = m.callOperation(parsed)
		}
	default:
		op := m.server.ops.Find(name)
		if op == nil {
			return nil, &rpcError{Code: -32602, Message: "Unknown tool " + name}
		}
		value, err = m.server.ops.CallAs(m.ctx, op, m.caller, arguments, "")
	}
	if err != nil {
		return M{"content": []any{M{"type": "text", "text": errdefs.As(err).Message}}, "isError": true}, nil
	}
	return M{"content": []any{M{"type": "text", "text": string(errdefs.Marshal(errdefs.Sorted(value)))}}, "isError": false}, nil
}

// reach is what the caller's token reaches, which decides the tools they see and their
// arguments.
type reach struct {
	scopes []auth.Access
	// Admin in at least one of the workspaces.
	admin bool
	// More than one workspace, so every tool needs workspace.
	several bool
}

func (m *mcpCall) reach() (*reach, error) {
	reachable, err := m.server.access.Reachable(m.ctx, m.caller)
	if err != nil {
		return nil, err
	}
	admin := slices.ContainsFunc(reachable, func(w auth.Reached) bool {
		role, ok := auth.ParseRole(w.Role)
		return ok && role >= auth.Admin
	})
	return &reach{scopes: m.caller.Scopes, admin: admin, several: len(reachable) > 1}, nil
}

// sees says whether the operation is offered: write tools need the write scope, admin tools an
// admin.
func (r *reach) sees(op *oapi.Op) bool {
	return slices.Contains(r.scopes, op.Access) && (op.Access != auth.AdminAccess || r.admin)
}

func (s *Server) findOperations(reach *reach, args findOperationsArgs) M {
	var candidates []*oapi.Op
	for _, op := range s.ops.All() {
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
			names = append(names, M{"name": op.Name, "summary": meta.Summary(op.Description)})
		}
		return M{"operations": names, "hint": "Search with a query to get argument schemas."}
	}
	var words []string
	for _, word := range strings.Fields(query) {
		words = append(words, strings.TrimRight(word, "s"))
	}
	type scored struct {
		score int
		op    *oapi.Op
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

func (m *mcpCall) callOperation(args callOperationArgs) (any, error) {
	op := m.server.ops.Find(strings.TrimSpace(args.Name))
	if op == nil {
		return nil, errdefs.BadRequest("No operation %q; use find_operations", args.Name)
	}
	return m.server.ops.CallAs(m.ctx, op, m.caller, args.Arguments, "")
}

// toolArguments is an operation's arguments, with workspace when the token reaches several.
func toolArguments(op *oapi.Op, several bool) M {
	if several {
		return op.SchemaWithWorkspace(true)
	}
	return op.Schema
}

func tool(op *oapi.Op, several bool) M {
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
			"inputSchema": args.SchemaOf[findOperationsArgs](),
			"annotations": M{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		M{
			"name":        "call_operation",
			"description": callDescription,
			"inputSchema": args.SchemaOf[callOperationArgs](),
		},
	}
}
