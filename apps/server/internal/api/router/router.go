// Package router puts everything on one chi router: the operations through Huma, the API's
// documents, MCP and the rest of the HTTP surface.
package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	account_handler "ricrac/internal/api/handlers/account"
	attachments_handler "ricrac/internal/api/handlers/attachments"
	comments_handler "ricrac/internal/api/handlers/comments"
	files_handler "ricrac/internal/api/handlers/files"
	labels_handler "ricrac/internal/api/handlers/labels"
	notifications_handler "ricrac/internal/api/handlers/notifications"
	oauthserver_handler "ricrac/internal/api/handlers/oauthserver"
	search_handler "ricrac/internal/api/handlers/search"
	signin_handler "ricrac/internal/api/handlers/signin"
	states_handler "ricrac/internal/api/handlers/states"
	teams_handler "ricrac/internal/api/handlers/teams"
	workspace_handler "ricrac/internal/api/handlers/workspace"
	"ricrac/internal/api/meta"
	"ricrac/internal/api/middleware"
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/mcpserver"
	"ricrac/internal/web"
)

// Fallback serves what isn't ported to the new server yet.
type Fallback interface {
	Mux() http.Handler
	AddOps(reg *oapi.Registry)
	Order() []string
}

// HumaConfig answers errors as {"error": message} and writes JSON without escaping <, > and &.
func HumaConfig() huma.Config {
	huma.NewError = func(status int, message string, errs ...error) huma.StatusError {
		return &errdefs.Error{Status: status, Message: message}
	}
	format := huma.Format{
		Marshal: func(w io.Writer, v any) error {
			encoder := json.NewEncoder(w)
			encoder.SetEscapeHTML(false)
			return encoder.Encode(v)
		},
		Unmarshal: func(data []byte, v any) error {
			decoder := json.NewDecoder(bytes.NewReader(data))
			return decoder.Decode(v)
		},
	}
	return huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info:    &huma.Info{Title: "Ricrac", Version: meta.Version},
			Components: &huma.Components{
				Schemas: huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer),
			},
		},
		Formats:       map[string]huma.Format{"application/json": format, "json": format},
		DefaultFormat: "application/json",
	}
}

// New is the server's router and its operations.
func New(srv *server.Server, fallback Fallback) (http.Handler, *oapi.Registry) {
	r := chi.NewRouter()
	r.Use(middleware.Compress, middleware.SecurityHeaders, middleware.LimitBody, middleware.CORS, chimiddleware.GetHead)
	api := humachi.New(r, HumaConfig())
	reg := oapi.NewRegistry(api, srv.AccessService)
	srv.Ops.Set(reg)
	RegisterHandlers(srv, reg)
	if fallback != nil {
		fallback.AddOps(reg)
		reg.Order(fallback.Order())
	}
	authn := middleware.NewAuth(srv.Cfg, srv.Keys, srv.AccessService)
	reg.Mount(authn.RESTCaller)
	account_handler.RegisterHandlers(srv, api)
	oauthserver_handler.Mount(r, srv)
	signin_handler.Mount(r, srv)

	docs := meta.NewHandler(srv.Cfg, reg)
	r.Get("/v1", docs.Index())
	r.Get("/v1/", docs.Index())
	r.Get("/v1/openapi.json", docs.OpenAPI)
	r.Get("/v1/ops", docs.ListOps)
	r.Get("/llms.txt", docs.LLMsTxt())
	r.Post("/v1/ops/{name}", func(w http.ResponseWriter, req *http.Request) {
		if _, _, ok := authn.RESTCaller(w, req); !ok {
			return
		}
		errdefs.Write(w, errdefs.NotFound("No operation %s", chi.URLParam(req, "name")))
	})
	r.Handle("/mcp", authn.RequireBearer(mcpserver.New(srv.Cfg, reg, srv.AccessService).Handler()))
	files_handler.Mount(r, srv, authn)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	unmatched := unmatchedHandler(fallback)
	r.NotFound(unmatched)
	r.MethodNotAllowed(unmatched)
	return r, reg
}

// unmatchedHandler serves what no route took: what the fallback still serves, else /api's not
// found answer or the web app.
func unmatchedHandler(fallback Fallback) http.HandlerFunc {
	var rest http.Handler
	if fallback != nil {
		rest = fallback.Mux()
	}
	patterns, _ := rest.(interface {
		Handler(r *http.Request) (http.Handler, string)
	})
	return func(w http.ResponseWriter, r *http.Request) {
		pattern := ""
		if patterns != nil {
			_, pattern = patterns.Handler(r)
		}
		switch {
		case pattern == "/api/" || pattern == "" && strings.HasPrefix(r.URL.Path, "/api/"):
			account_handler.NotFound(w, r)
		case pattern == "":
			web.Serve(w, r)
		default:
			rest.ServeHTTP(w, r)
		}
	}
}

// RegisterHandlers puts every handler group's operations on the registry, in the order they
// are listed.
func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	workspace_handler.RegisterHandlers(srv, reg)
	teams_handler.RegisterHandlers(srv, reg)
	states_handler.RegisterHandlers(srv, reg)
	labels_handler.RegisterHandlers(srv, reg)
	comments_handler.RegisterHandlers(srv, reg)
	attachments_handler.RegisterHandlers(srv, reg)
	notifications_handler.RegisterHandlers(srv, reg)
	search_handler.RegisterHandlers(srv, reg)
}
