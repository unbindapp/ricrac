// Package web serves the web app's build (apps/web/dist), copied to internal/web/dist before
// building the server.
package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

var assets, _ = fs.Sub(dist, "dist")

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https: blob:; " +
	"media-src 'self'; connect-src 'self'; font-src 'self'; frame-src 'none'; frame-ancestors 'none'; " +
	"base-uri 'none'; form-action 'self'"

// Serve serves the web app's files, and its index.html for every other path.
func Serve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if data, err := fs.ReadFile(assets, name); name != "" && err == nil {
		contentType := mime.TypeByExtension(path.Ext(name))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		// Vite puts content-hashed files under assets/; everything else may change between deploys.
		cache := "no-cache"
		if strings.HasPrefix(name, "assets/") {
			cache = "public, max-age=31536000, immutable"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", cache)
		_, _ = w.Write(data)
		return
	}
	if strings.HasPrefix(name, "assets/") {
		// Without it Cloudflare and browsers keep the 404 for hours, breaking pages that ask for it after a deploy.
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.Error(w, "The dashboard is not built. Run `bun run build` in apps/web.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Security-Policy", csp)
	_, _ = w.Write(index)
}
