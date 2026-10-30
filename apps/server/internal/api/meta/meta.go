// Package meta serves what describes the API: its index, the OpenAPI document, the list of
// operations and llms.txt.
package meta

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"ricrac/config"
	"ricrac/internal/api/oapi"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
)

type M = map[string]any

const Version = "0.1.0"

const Instructions = `Ricrac is an issue tracker for agents: a workspace has issues (identifiers like ENG-123, ENG being the workspace's issue prefix), workflow statuses (triage, backlog, unstarted, started, completed, canceled), labels, cycles and templates; projects (with milestones, status updates and documents) roll up into initiatives. There are also comments (on documents they can annotate a passage or suggest an edit), a forum of discussions with smaller threads (bugs, design checks, questions), attachments, custom views, favorites, an inbox of notifications, customers, releases and webhooks.

Referencing things: wherever a tool takes a person, status, label, project, cycle or milestone you may pass a name, key or identifier instead of an id. People also take an email or "me". Issues take their identifier (ENG-123, or one they had before), id or link. Errors list the valid options when a name doesn't match.

Teams: a workspace has one team, which holds its issue prefix, statuses, labels, cycles and settings (get_team, save_team). Arguments named team are optional and default to it; keys, names and ids of teams from before still work.

Writing: descriptions, comments, documents and updates are Markdown; write literal newlines, not escape sequences. Mention people with @displayName, @email or the GitHub login they linked: they are notified and subscribed, and on GitHub the mention names their GitHub login. To change part of a long description or document, pass ` + "`patch`" + ` edits instead of resending it all. Show screenshots and recordings by uploading them with prepare_file_upload and embedding the returned markdown: ` + "`![shot.png](assetUrl)`" + ` shows an image and ` + "`![demo.mp4](assetUrl)`" + ` plays a video (mp4, webm, mov, m4v, ogv).

Workspaces: a connection may reach several workspaces; list_workspaces lists them. When it reaches more than one, every tool takes a ` + "`workspace`" + ` argument (the URL key, like "acme") naming the one to act in. Identifiers like ENG-123 and names are only unique within a workspace; Ricrac links already name theirs.

Saving: save_* tools create when you omit ` + "`id`" + ` and update when you pass it; omitted fields stay as they are and null clears a field. List tools page with ` + "`limit` and `cursor`" + ` (pass pageInfo.endCursor). Start with get_workspace to learn the workspace and its issue prefix, and list_issue_statuses for its workflow.

More operations: the tools listed cover everyday work. Everything else (webhooks, customers, releases, favorites, templates and views management, workspace settings and members, archiving, sharing, ...) is found with find_operations and run with call_operation.`

type Handler struct {
	cfg config.Config
	ops *oapi.Registry
}

func NewHandler(cfg config.Config, ops *oapi.Registry) *Handler {
	return &Handler{cfg: cfg, ops: ops}
}

// OpenAPI serves OpenAPIDocument.
func (h *Handler) OpenAPI(w http.ResponseWriter, r *http.Request) {
	errdefs.WriteJSON(w, http.StatusOK, h.OpenAPIDocument(h.cfg.PublicURL))
}

// Index points agents at the API's documents.
func (h *Handler) Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := h.cfg.PublicURL
		errdefs.WriteJSON(w, http.StatusOK, M{
			"name":           "Ricrac API",
			"openapi":        base + "/v1/openapi.json",
			"guide":          base + "/llms.txt",
			"mcp":            h.cfg.MCPURL(),
			"authentication": "Authorization: Bearer <OAuth access token or API key>",
		})
	}
}

func routeList(op *oapi.Op, format string) []string {
	routes := []string{}
	for _, route := range op.Routes {
		routes = append(routes, fmt.Sprintf(format, route.Method, route.Path))
	}
	return routes
}

// ListOps lists every operation with its routes and arguments.
func (h *Handler) ListOps(w http.ResponseWriter, r *http.Request) {
	list := []any{}
	for _, op := range h.ops.All() {
		list = append(list, M{
			"name":        op.Name,
			"description": op.Description,
			"access":      op.Access.String(),
			"routes":      routeList(op, "%s /v1%s"),
			"arguments":   op.SchemaWithWorkspace(false),
		})
	}
	errdefs.WriteJSON(w, http.StatusOK, M{"operations": list})
}

// Summary is the first sentence of a description.
func Summary(description string) string {
	first, _, _ := strings.Cut(description, ". ")
	return first
}

// OpenAPIDocument describes the REST routes.
func (h *Handler) OpenAPIDocument(base string) M {
	paths := map[string]M{}
	for _, op := range h.ops.All() {
		for n, route := range op.Routes {
			inPath := oapi.PathParams(route.Path)
			schema := op.SchemaWithWorkspace(false)
			properties, _ := schema["properties"].(M)
			var required []string
			list, _ := schema["required"].([]any)
			for _, r := range list {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
			parameters := []any{}
			for _, p := range inPath {
				parameters = append(parameters, M{"name": p, "in": "path", "required": true, "schema": M{"type": "string"}})
			}
			operationID := op.Name
			if n > 0 {
				operationID = op.Name + "_" + strings.ToLower(route.Method)
			}
			operation := M{
				"operationId": operationID,
				"summary":     Summary(op.Description),
				"description": fmt.Sprintf("%s\n\nAlso callable as POST /v1/ops/%s and as the MCP tool %s.", op.Description, op.Name, op.Name),
				"x-access":    op.Access.String(),
				"responses": M{
					"200": M{"description": "The result", "content": M{"application/json": M{"schema": M{"type": "object"}}}},
					"400": M{"description": "Invalid arguments; the error says why"},
					"401": M{"description": "Missing or invalid token"},
					"403": M{"description": "Not allowed"},
					"404": M{"description": "Not found"},
				},
			}
			rest := M{}
			for name, property := range properties {
				if !slices.Contains(inPath, name) {
					rest[name] = property
				}
			}
			if route.Method == "GET" || route.Method == "DELETE" {
				names := make([]string, 0, len(rest))
				for name := range rest {
					names = append(names, name)
				}
				slices.Sort(names)
				for _, name := range names {
					property, _ := rest[name].(M)
					parameter := M{
						"name":        name,
						"in":          "query",
						"required":    slices.Contains(required, name),
						"description": property["description"],
					}
					if slices.Contains(oapi.SchemaTypes(property), "object") {
						parameter["content"] = M{"application/json": M{"schema": property}}
					} else {
						parameter["schema"] = property
					}
					parameters = append(parameters, parameter)
				}
			} else if len(rest) > 0 {
				body := M{}
				for key, value := range schema {
					body[key] = value
				}
				body["properties"] = rest
				if list, ok := body["required"].([]any); ok {
					kept := []any{}
					for _, r := range list {
						if s, _ := r.(string); !slices.Contains(inPath, s) {
							kept = append(kept, r)
						}
					}
					body["required"] = kept
				}
				operation["requestBody"] = M{"required": false, "content": M{"application/json": M{"schema": body}}}
			}
			operation["parameters"] = parameters
			key := "/v1" + route.Path
			if paths[key] == nil {
				paths[key] = M{}
			}
			paths[key][strings.ToLower(route.Method)] = operation
		}
	}
	return M{
		"openapi": "3.1.0",
		"info": M{
			"title":       "Ricrac",
			"version":     Version,
			"description": Instructions,
		},
		"servers":  []any{M{"url": base}},
		"security": []any{M{"bearer": []any{}}},
		"components": M{
			"securitySchemes": M{
				"bearer": M{"type": "http", "scheme": "bearer", "description": "An OAuth access token or an API key (rr_key_...)"},
				"oauth": M{
					"type": "oauth2",
					"flows": M{"authorizationCode": M{
						"authorizationUrl": base + "/oauth/authorize",
						"tokenUrl":         base + "/oauth/token",
						"refreshUrl":       base + "/oauth/token",
						"scopes":           M{"read": "Read everything", "write": "Create and change things", "admin": "Workspace administration"},
					}},
				},
			},
		},
		"paths": paths,
	}
}

// LLMsTxt is a Markdown overview for agents reading the API without MCP.
func (h *Handler) LLMsTxt() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := h.cfg.PublicURL
		var text strings.Builder
		fmt.Fprintf(&text, "# Ricrac\n\n%s\n\n## Connecting\n\n- MCP (streamable HTTP): %s/mcp, with OAuth (discovery at "+
			"%s/.well-known/oauth-authorization-server; dynamic registration and client ID metadata documents are supported).\n"+
			"- REST: %s/v1, with `Authorization: Bearer <token>`. Tokens are OAuth access tokens or API keys (rr_key_...) made in "+
			"the dashboard at %s. Scopes: %s.\n- A token reaches one, several or all of your workspaces. When it reaches several, name "+
			"the workspace in each call with the `workspace` argument (a URL key) or the `x-ricrac-workspace` header; "+
			"`GET %s/v1/workspaces` lists them.\n- OpenAPI: %s/v1/openapi.json. Every operation can also be called as "+
			"`POST %s/v1/ops/<name>` with its arguments as a JSON body.\n- Query strings take arrays as repeated or "+
			"comma-separated values and objects (like `filter`) as JSON.\n\n## Operations\n",
			Instructions, base, base, base, base, strings.Join(auth.AllScopes, ", "), base, base, base)
		for _, op := range h.ops.All() {
			routes := routeList(op, "`%s /v1%s`")
			fmt.Fprintf(&text, "\n### %s\n\n%s (%s; %s)\n\n%s\n", op.Name, strings.Join(routes, ", "), op.Access.String(),
				"MCP tool of the same name", op.Description)
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(text.String()))
	}
}
