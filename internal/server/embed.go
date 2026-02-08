package server

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed dist/*
var distFS embed.FS

// SetupStaticHandler adds a catch-all handler that serves the embedded
// React frontend. API routes are registered first and take priority.
func (s *Server) SetupStaticHandler() {
	// Check for a local dev override: if a "dist" directory exists next to
	// the binary, serve from disk instead of the embedded FS. This is useful
	// during frontend development.
	if dir, err := execRelativeDist(); err == nil {
		s.mux.Handle("/", http.FileServer(http.Dir(dir)))
		return
	}

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("embedded dist/ not found: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try serving the requested file. If it doesn't exist, serve
		// index.html for client-side routing.
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		if _, err := fs.Stat(sub, path[1:]); err != nil {
			// File not found — serve index.html for SPA routing.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func execRelativeDist() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(exe), "dist")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", os.ErrNotExist
	}
	return dir, nil
}
