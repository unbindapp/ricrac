package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"ricrac/app"
	"ricrac/ops"
)

type M = map[string]any

// restCaller is the caller of a /v1 request (a bearer token, or the dashboard's session cookie)
// and the workspace named in x-ricrac-workspace, used when the arguments don't name one. It
// writes the error response when there's none.
func restCaller(state *app.State, w http.ResponseWriter, r *http.Request) (*app.Caller, string, bool) {
	workspace := r.Header.Get("X-Ricrac-Workspace")
	if caller := bearerCaller(state, r); caller != nil {
		return caller, workspace, true
	}
	user, ok := state.Keys.SessionUser(r)
	if r.Header.Get("Authorization") != "" || !ok {
		unauthorized(state, w, r)
		return nil, "", false
	}
	if r.Method != http.MethodGet && r.Header.Get("X-Ricrac") == "" {
		app.WriteError(w, app.Forbidden("Missing x-ricrac header"))
		return nil, "", false
	}
	return app.SessionCaller(r.Context(), state, user), workspace, true
}

func restRoutes(mux *http.ServeMux, state *app.State) {
	for _, op := range ops.All() {
		for _, route := range op.Routes {
			params := pathParams(route.Path)
			mux.HandleFunc(route.Method+" /v1"+route.Path, func(w http.ResponseWriter, r *http.Request) {
				values := map[string]string{}
				for _, name := range params {
					values[name] = r.PathValue(name)
				}
				callREST(state, w, r, op.Name, values, r.URL.RawQuery)
			})
		}
	}
	mux.HandleFunc("GET /v1", index(state))
	mux.HandleFunc("GET /v1/{$}", index(state))
	mux.HandleFunc("GET /v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		app.WriteJSON(w, http.StatusOK, OpenAPIDocument(state.Config.PublicURL))
	})
	mux.HandleFunc("GET /v1/ops", listOps)
	mux.HandleFunc("POST /v1/ops/{name}", func(w http.ResponseWriter, r *http.Request) {
		callREST(state, w, r, r.PathValue("name"), nil, "")
	})
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) {
		app.WriteError(w, app.NotFound("No such endpoint; see /v1/openapi.json"))
	})
}

func callREST(state *app.State, w http.ResponseWriter, r *http.Request, name string, path map[string]string, query string) {
	caller, workspace, ok := restCaller(state, w, r)
	if !ok {
		return
	}
	op := ops.Find(name)
	if op == nil {
		app.WriteError(w, app.NotFound("No operation %s", name))
		return
	}
	body, err := readBody(r)
	if err != nil {
		app.WriteError(w, err)
		return
	}
	args, err := arguments(op, path, query, body)
	if err != nil {
		app.WriteError(w, err)
		return
	}
	value, err := op.CallAs(caller, args, workspace)
	if err != nil {
		app.WriteError(w, err)
		return
	}
	app.WriteJSON(w, http.StatusOK, value)
}

func readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, app.Errorf(http.StatusRequestEntityTooLarge, "The body can have at most %d bytes", tooLarge.Limit)
	}
	if err != nil {
		return nil, app.BadRequest("Couldn't read the body")
	}
	return body, nil
}

// arguments merges the JSON body, then the query string, then the path parameters into one
// object.
func arguments(op *ops.Op, path map[string]string, query string, body []byte) (M, error) {
	args := M{}
	if len(bytes.TrimSpace(body)) > 0 {
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return nil, app.BadRequest("The body isn't valid JSON: %s", err)
		}
		object, ok := value.(M)
		if !ok {
			return nil, app.BadRequest("The body must be a JSON object")
		}
		args = object
	}
	properties, _ := op.Schema["properties"].(M)
	lists := map[string][]string{}
	var order []string
	for pair := range strings.SplitSeq(query, "&") {
		if pair == "" {
			continue
		}
		key, value, _ := strings.Cut(pair, "=")
		key = strings.TrimSuffix(decodeQuery(key), "[]")
		if _, seen := lists[key]; !seen {
			order = append(order, key)
		}
		lists[key] = append(lists[key], decodeQuery(value))
	}
	for _, key := range order {
		schema, _ := properties[key].(M)
		args[key] = coerce(lists[key], schema)
	}
	for key, value := range path {
		args[key] = value
	}
	return args, nil
}

func decodeQuery(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	if decoded, err := url.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}

// schemaTypes is the JSON types a schema allows, looking into anyOf and oneOf.
func schemaTypes(schema M) []string {
	var found []string
	switch kind := schema["type"].(type) {
	case string:
		found = append(found, kind)
	case []any:
		for _, k := range kind {
			if s, ok := k.(string); ok {
				found = append(found, s)
			}
		}
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		options, _ := schema[key].([]any)
		for _, option := range options {
			if o, ok := option.(M); ok {
				found = append(found, schemaTypes(o)...)
			}
		}
	}
	return found
}

// coerce turns query values, which are text, into what the argument's schema expects.
func coerce(values []string, schema M) any {
	kinds := schemaTypes(schema)
	has := func(kind string) bool { return slices.Contains(kinds, kind) }
	if has("array") {
		items, _ := schema["items"].(M)
		if len(values) == 1 {
			var list []any
			if json.Unmarshal([]byte(values[0]), &list) == nil && list != nil {
				return list
			}
		}
		parts := []any{}
		for _, value := range values {
			for part := range strings.SplitSeq(value, ",") {
				if part = strings.TrimSpace(part); part != "" {
					parts = append(parts, coerce([]string{part}, items))
				}
			}
		}
		return parts
	}
	value := ""
	if len(values) > 0 {
		value = values[len(values)-1]
	}
	if value == "null" && has("null") {
		return nil
	}
	if has("boolean") && (value == "true" || value == "false") {
		return value == "true"
	}
	if (has("integer") || has("number")) && !has("string") {
		var number float64
		if json.Unmarshal([]byte(value), &number) == nil {
			return number
		}
	}
	if has("object") {
		var object M
		if json.Unmarshal([]byte(value), &object) == nil && object != nil {
			return object
		}
	}
	if len(kinds) == 0 {
		var parsed any
		if json.Unmarshal([]byte(value), &parsed) == nil {
			return parsed
		}
	}
	return value
}

func index(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := state.Config.PublicURL
		app.WriteJSON(w, http.StatusOK, M{
			"name":           "Ricrac API",
			"openapi":        base + "/v1/openapi.json",
			"guide":          base + "/llms.txt",
			"mcp":            state.Config.MCPURL(),
			"authentication": "Authorization: Bearer <OAuth access token or API key>",
		})
	}
}

func routeList(op *ops.Op, format string) []string {
	routes := []string{}
	for _, route := range op.Routes {
		routes = append(routes, fmt.Sprintf(format, route.Method, route.Path))
	}
	return routes
}

func listOps(w http.ResponseWriter, r *http.Request) {
	list := []any{}
	for _, op := range ops.All() {
		list = append(list, M{
			"name":        op.Name,
			"description": op.Description,
			"access":      op.Access.String(),
			"routes":      routeList(op, "%s /v1%s"),
			"arguments":   op.SchemaWithWorkspace(false),
		})
	}
	app.WriteJSON(w, http.StatusOK, M{"operations": list})
}

func pathParams(path string) []string {
	var names []string
	for segment := range strings.SplitSeq(path, "/") {
		if name, ok := strings.CutPrefix(segment, "{"); ok && strings.HasSuffix(name, "}") {
			names = append(names, strings.TrimSuffix(name, "}"))
		}
	}
	return names
}

func summary(description string) string {
	first, _, _ := strings.Cut(description, ". ")
	return first
}

func OpenAPIDocument(base string) M {
	paths := map[string]M{}
	for _, op := range ops.All() {
		for n, route := range op.Routes {
			inPath := pathParams(route.Path)
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
				"summary":     summary(op.Description),
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
					if slices.Contains(schemaTypes(property), "object") {
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

// llmsTxt is a Markdown overview for agents reading the API without MCP.
func llmsTxt(state *app.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := state.Config.PublicURL
		var text strings.Builder
		fmt.Fprintf(&text, "# Ricrac\n\n%s\n\n## Connecting\n\n- MCP (streamable HTTP): %s/mcp, with OAuth (discovery at "+
			"%s/.well-known/oauth-authorization-server; dynamic registration and client ID metadata documents are supported).\n"+
			"- REST: %s/v1, with `Authorization: Bearer <token>`. Tokens are OAuth access tokens or API keys (rr_key_...) made in "+
			"the dashboard at %s. Scopes: %s.\n- A token reaches one, several or all of your workspaces. When it reaches several, name "+
			"the workspace in each call with the `workspace` argument (a URL key) or the `x-ricrac-workspace` header; "+
			"`GET %s/v1/workspaces` lists them.\n- OpenAPI: %s/v1/openapi.json. Every operation can also be called as "+
			"`POST %s/v1/ops/<name>` with its arguments as a JSON body.\n- Query strings take arrays as repeated or "+
			"comma-separated values and objects (like `filter`) as JSON.\n\n## Operations\n",
			Instructions, base, base, base, base, strings.Join(app.AllScopes, ", "), base, base, base)
		for _, op := range ops.All() {
			routes := routeList(op, "`%s /v1%s`")
			fmt.Fprintf(&text, "\n### %s\n\n%s (%s; %s)\n\n%s\n", op.Name, strings.Join(routes, ", "), op.Access.String(),
				"MCP tool of the same name", op.Description)
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(text.String()))
	}
}
