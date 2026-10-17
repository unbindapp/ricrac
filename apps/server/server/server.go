// Package server is the HTTP side of Ricrac: the REST API, MCP, OAuth, sign-in with Google, the
// dashboard's API, files, the web app and the background jobs.
package server

import (
	"compress/gzip"
	"net/http"
	"strings"

	"ricrac/app"
)

const Version = "0.1.0"

const maxBody = 8 * 1024 * 1024

// Router serves everything; main serves it and the end-to-end tests call it.
func Router(state *app.State) http.Handler {
	return compress(securityHeaders(limitBody(cors(Mux(state)))))
}

// Mux routes requests without the wrappers Router adds.
func Mux(state *app.State) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", protectedResource(state))
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", protectedResource(state))
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", authorizationServer(state))
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/mcp", authorizationServer(state))
	mux.HandleFunc("POST /oauth/register", register(state))
	mux.HandleFunc("POST /oauth/token", token(state))
	mux.HandleFunc("POST /oauth/revoke", revoke(state))
	mux.HandleFunc("GET /llms.txt", llmsTxt(state))
	restRoutes(mux, state)
	mux.HandleFunc("GET /files/{id}", download(state))
	mux.HandleFunc("PUT /uploads/{token}", upload(state))
	mux.Handle("/mcp", requireBearer(state, mcpHandler(state)))
	githubRoutes(mux, state)
	accountRoutes(mux, state)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	return mux
}

// isPublic is what other origins call with bearer tokens, never cookies alone: MCP, OAuth, the
// REST API and files.
func isPublic(path string) bool {
	for _, prefix := range []string{"/.well-known/oauth-", "/oauth/register", "/oauth/token", "/oauth/revoke", "/v1/", "/files/", "/uploads/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return path == "/v1" || path == "/mcp" || path == "/llms.txt"
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Add("Vary", "origin, access-control-request-method, access-control-request-headers")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "*")
			h.Set("Access-Control-Allow-Headers", "*")
			w.WriteHeader(http.StatusOK)
			return
		}
		h.Set("Access-Control-Expose-Headers", "www-authenticate,mcp-session-id")
		next.ServeHTTP(w, r)
	})
}

// limitBody caps request bodies at 8 MB; uploads have their own limit.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/uploads/") {
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

const compressAbove = 1024

// compress gzips responses above 1 KB for clients that accept it, except event streams.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "accept-encoding")
		if !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		writer := &gzipWriter{ResponseWriter: w}
		defer writer.finish()
		next.ServeHTTP(writer, r)
	})
}

func acceptsGzip(header string) bool {
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(name), "gzip") && strings.ReplaceAll(params, " ", "") != "q=0" {
			return true
		}
	}
	return false
}

// gzipWriter holds back the first kilobyte to decide whether compressing is worth it.
type gzipWriter struct {
	http.ResponseWriter
	status  int
	buffer  []byte
	gz      *gzip.Writer
	decided bool
}

func (g *gzipWriter) WriteHeader(status int) {
	if g.status == 0 {
		g.status = status
	}
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	if g.decided {
		if g.gz != nil {
			return g.gz.Write(data)
		}
		return g.ResponseWriter.Write(data)
	}
	g.buffer = append(g.buffer, data...)
	if len(g.buffer) > compressAbove {
		g.decide(true)
	}
	return len(data), nil
}

func (g *gzipWriter) decide(large bool) {
	g.decided = true
	h := g.Header()
	compressible := large && h.Get("Content-Encoding") == "" &&
		!strings.HasPrefix(h.Get("Content-Type"), "text/event-stream") &&
		g.status != http.StatusNoContent && g.status != http.StatusNotModified
	if compressible {
		h.Del("Content-Length")
		h.Set("Content-Encoding", "gzip")
		g.gz = gzip.NewWriter(g.ResponseWriter)
	}
	g.ResponseWriter.WriteHeader(g.status)
	buffered := g.buffer
	g.buffer = nil
	if len(buffered) == 0 {
		return
	}
	if g.gz != nil {
		_, _ = g.gz.Write(buffered)
		return
	}
	_, _ = g.ResponseWriter.Write(buffered)
}

func (g *gzipWriter) finish() {
	if g.status == 0 {
		return
	}
	if !g.decided {
		g.decide(false)
	}
	if g.gz != nil {
		_ = g.gz.Close()
	}
}

func (g *gzipWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}
