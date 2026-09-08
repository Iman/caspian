// SPDX-License-Identifier: AGPL-3.0-or-later
package panel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestApplicationReplacesHTMLWithOneFlutterUI(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":               {Data: []byte("<title>Caspian Flutter</title>")},
		"flutter_bootstrap.js":     {Data: []byte("bootstrap")},
		"canvaskit/canvaskit.wasm": {Data: []byte("wasm")},
	}
	calls := 0
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	})
	app := applicationWithAssets(api, assets)
	for _, path := range []string{"/", "/login", "/setup", "/help"} {
		r := httptest.NewRecorder()
		app.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 200 || !strings.Contains(r.Body.String(), "Caspian Flutter") {
			t.Fatalf("%s: %d %s", path, r.Code, r.Body.String())
		}
		if !strings.Contains(r.Header().Get("Content-Security-Policy"), "connect-src 'self'") {
			t.Fatal("missing same-origin policy")
		}
		if !strings.Contains(r.Header().Get("Content-Security-Policy"), "manifest-src 'self'") {
			t.Fatal("local application manifest is blocked by default-src")
		}
	}
	for _, path := range []string{"/api/v1/state", "/identifiers.json", "/status.json"} {
		r := httptest.NewRecorder()
		app.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 401 {
			t.Fatalf("API authentication bypass: %s %d", path, r.Code)
		}
	}
	if calls != 3 {
		t.Fatalf("API calls %d", calls)
	}
	for _, path := range []string{"/power", "/assets/panel.css", "/missing.js", "/../index.html"} {
		r := httptest.NewRecorder()
		app.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 404 {
			t.Fatalf("legacy or invalid route served: %s %d", path, r.Code)
		}
	}
	r := httptest.NewRecorder()
	app.ServeHTTP(r, httptest.NewRequest("POST", "/power", strings.NewReader("on=1")))
	if r.Code != 405 || calls != 3 {
		t.Fatal("legacy mutation reached backend")
	}
	r = httptest.NewRecorder()
	app.ServeHTTP(r, httptest.NewRequest("GET", "/canvaskit/canvaskit.wasm", nil))
	if r.Code != 200 || r.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("Wasm not served: %d %s", r.Code, r.Header().Get("Content-Type"))
	}
}
