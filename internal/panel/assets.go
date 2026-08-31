// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

// Everything the browser loads is compiled into the binary.
//
// Design section 5.7 gives two reasons and the second is the stronger one. The
// privacy reason is that a remote asset tells a third party the address of
// everyone who opens the panel. The availability reason is that the panel has
// to load when the tunnel is down, which is exactly when somebody needs it: a
// panel that waits on a font from a CDN while the box has no working internet
// is a blank screen at the worst possible moment.
//
// So there is no CDN, no web font, no remote script, no remote stylesheet and
// no favicon fetch. TestNoAssetReferencesAnExternalURL scans these files and
// every rendered page for an absolute URL, and the Content-Security-Policy in
// setSecurityHeaders makes the browser refuse one that got past the test.

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/panel.css assets/panel.js assets/favicon.svg
var assetFS embed.FS

// pageNames are the pages the panel serves. Each is parsed together with the
// shell in base.html, as its own template set, so that two pages may both
// define a block called "main" without one silently winning.
//
// A page missing from this list parses nowhere and its handler answers 500,
// which is what happened when help was added to the routes and not to here.
// TestEveryAssetTheHTMLNamesIsServed catches it, which is why that test walks
// the rendered links rather than the list.
var pageNames = []string{"index", "login", "setup", "problem", "help"}

var pages = func() map[string]*template.Template {
	out := make(map[string]*template.Template, len(pageNames))
	for _, name := range pageNames {
		out[name] = template.Must(template.New("base.html").
			ParseFS(templateFS, "templates/base.html", "templates/"+name+".html"))
	}
	return out
}()

// render writes one page.
//
// It renders into a buffer first. A template that fails halfway through would
// otherwise leave a truncated page behind a 200, which looks to the user like
// the panel is broken in some way they cannot describe. With the buffer, a
// failed render is a clean error page instead.
func (p *Panel) render(w http.ResponseWriter, status int, name string, data any) {
	t, ok := pages[name]
	if !ok {
		p.log.Error("no such page template", "page", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base.html", data); err != nil {
		// The error is logged, not shown. A template error message can quote
		// the data it was rendering, and the data on this page includes the
		// hotspot passphrase.
		p.log.Error("rendering page failed", "page", name, "error", err.Error())
		http.Error(w, "Caspian could not draw this page.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// serveAsset returns a handler for one embedded file.
//
// The content type is stated rather than sniffed, because every response
// carries X-Content-Type-Options: nosniff and a stylesheet served as
// application/octet-stream is a stylesheet the browser will not apply.
func (p *Panel) serveAsset(name, contentType string) http.HandlerFunc {
	body, err := assetFS.ReadFile("assets/" + name)
	if err != nil {
		// Unreachable: go:embed above names these files, so a missing one is a
		// build failure. The panic is here rather than an ignored error so
		// that if that ever stops being true it is loud.
		panic(fmt.Sprintf("panel: embedded asset %q is missing: %v", name, err))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

// assetNames lists the embedded files, for tests.
func assetNames() []string {
	entries, err := fs.Glob(assetFS, "assets/*")
	if err != nil {
		return nil
	}
	return entries
}

// templateNames lists the embedded templates, for tests.
func templateNames() []string {
	entries, err := fs.Glob(templateFS, "templates/*")
	if err != nil {
		return nil
	}
	return entries
}
