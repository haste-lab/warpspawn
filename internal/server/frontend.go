package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:frontend_dist
var frontendFS embed.FS

// frontendHandler serves the embedded Svelte frontend.
// API routes are handled separately and take precedence.
func frontendHandler() http.Handler {
	// Strip the "frontend_dist" prefix to serve from root
	sub, err := fs.Sub(frontendFS, "frontend_dist")
	if err != nil {
		// Fallback: serve nothing (dev mode without built frontend)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Warpspawn</title></head>
<body style="background:#0a0e17;color:#e8edf5;font-family:sans-serif;padding:40px;">
<h1>Warpspawn</h1>
<p>Frontend not built. Run: cd frontend && npm run build</p>
</body></html>`))
		})
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For SPA routing: if the path doesn't match a file, serve index.html
		path := r.URL.Path
		if path == "/" || path == "/index.html" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the file exists
		if _, err := fs.Stat(sub, strings.TrimPrefix(path, "/")); err != nil {
			// File doesn't exist — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
