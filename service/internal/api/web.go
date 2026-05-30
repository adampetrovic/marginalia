package api

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/adampetrovic/marginalia/service/internal/web"
)

// registerWebRoutes serves the embedded single-page app at the root with a
// history-API fallback: any path that doesn't map to a built asset returns
// index.html so client-side routing works on deep links and refreshes.
func (s *Server) registerWebRoutes(r chi.Router) {
	assets, err := web.FS()
	if err != nil {
		s.logger.Error("loading embedded web assets", "error", err)
		return
	}
	fileServer := http.FileServer(http.FS(assets))

	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name := strings.TrimPrefix(path.Clean(req.URL.Path), "/")
		if name == "" {
			serveIndex(w, assets)
			return
		}
		// Serve the asset if it exists; otherwise fall back to the SPA shell.
		if f, err := assets.Open(name); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, req)
			return
		}
		serveIndex(w, assets)
	}))
}

func serveIndex(w http.ResponseWriter, assets fs.FS) {
	f, err := assets.Open("index.html")
	if err != nil {
		http.Error(w, "web UI not built", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.Copy(w, f)
}
