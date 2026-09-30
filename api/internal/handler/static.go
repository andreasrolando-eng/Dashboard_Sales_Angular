package handler

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SPA serves the built Angular app from dir: existing files as-is, every other
// path falls back to index.html so client-side routes (/sales, /login, ...)
// work on reload. Unknown /api/* paths stay a JSON 404 instead of HTML.
//
// Caching: Angular's hashed bundles (main-ABC123.js) never change, so they are
// cached for a year; index.html is no-cache so a deploy is picked up at once.
func SPA(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint tidak ditemukan"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		clean := path.Clean("/" + r.URL.Path)
		if clean != "/" {
			if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); err == nil && !info.IsDir() {
				if strings.HasSuffix(clean, ".html") {
					w.Header().Set("Cache-Control", "no-cache")
				} else {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
