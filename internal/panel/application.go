// SPDX-License-Identifier: AGPL-3.0-or-later
package panel

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// applicationWithAssets serves one Flutter application and the authenticated
// backend. Legacy HTML and form routes are not reachable through this handler.
func applicationWithAssets(api http.Handler, assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") || r.URL.Path == "/identifiers.json" || r.URL.Path == "/status.json" {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		switch name {
		case "", "login", "setup", "help":
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		content, err := fs.ReadFile(assets, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:; manifest-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		contentType := mime.TypeByExtension(path.Ext(name))
		if path.Ext(name) == ".wasm" {
			contentType = "application/wasm"
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
	})
}
