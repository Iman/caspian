// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// The panel fetches nothing (design section 5.7). These tests are the half of
// that rule that can be checked in Go; the other half is the
// Content-Security-Policy, checked in TestSecurityHeadersOnEveryResponse.

// externalURL matches anything that would make a browser open a connection to
// somewhere else: an absolute URL with any scheme, a protocol-relative URL in
// an attribute, or a CSS @import.
//
// It is deliberately broader than "https://". A stylesheet pulled over http, a
// websocket, a font over a scheme nobody thought about, and //fonts.example are
// all the same mistake.
var (
	absoluteURL      = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^"'\s)>]+`)
	protocolRelative = regexp.MustCompile(`(?i)(?:href|src|action|content)\s*=\s*["']//[^"']+`)
	cssImport        = regexp.MustCompile(`(?i)@import`)
	cssURL           = regexp.MustCompile(`(?i)url\(\s*["']?[a-z][a-z0-9+.-]*:`)
)

// allowedAbsolute is the one absolute URL that may appear, and the reason it
// may.
//
// An xmlns is an identifier, not a location. The SVG namespace string is how a
// parser is told "these elements are SVG"; no browser dereferences it, and
// removing it would stop the QR code rendering at all in an XML context. It is
// listed here rather than being quietly skipped by a narrower regexp, so that
// the exception is visible and anything else added later has to be argued for
// in this list.
var allowedAbsolute = map[string]bool{
	"http://www.w3.org/2000/svg": true,

	// The one link that leaves the box, in the rail.
	//
	// The guarantee this file protects is that the panel FETCHES nothing from
	// the internet: it must render identically on a box with no uplink, and
	// opening it must tell no third party that it was opened. A navigation
	// link breaks neither. Nothing is requested until a person clicks, and by
	// then they have chosen to go.
	//
	// Allowing it here is not enough on its own, because this map permits the
	// string in ANY position, including a script src. So
	// TestTheOutsideLinkIsANavigationLinkAndNeverAFetch asserts the narrower
	// property that actually matters, and that test is the guard; this entry
	// only stops the broad scanner reporting it twice.
	"https://javidnetworkwatch.com/":  true,
	"https://github.com/Iman/caspian": true,
}

// findExternalReferences returns every external reference in text.
func findExternalReferences(text string) []string {
	var found []string
	for _, m := range absoluteURL.FindAllString(text, -1) {
		if allowedAbsolute[strings.TrimRight(m, `"'`)] {
			continue
		}
		found = append(found, m)
	}
	found = append(found, protocolRelative.FindAllString(text, -1)...)
	found = append(found, cssImport.FindAllString(text, -1)...)
	found = append(found, cssURL.FindAllString(text, -1)...)
	return found
}

// TestTheExternalURLScannerWorks is the positive control for every assertion
// below.
//
// Without it, a regexp that matched nothing would make this whole file pass
// while the panel loaded half the internet. That failure reports success, which
// is the worst kind.
func TestTheExternalURLScannerWorks(t *testing.T) {
	planted := []string{
		`<link rel="stylesheet" href="https://cdn.example.com/x.css">`,
		`<script src="http://evil.example/x.js"></script>`,
		`<img src="//images.example.com/a.png">`,
		`@import url("https://fonts.example.com/x.css");`,
		`body { background: url(https://example.com/bg.png); }`,
		`<link href="https://fonts.googleapis.com/css2?family=Inter" rel="stylesheet">`,
		`fetch("https://telemetry.example.com/report")`,
		`<form action="https://example.com/post">`,
	}
	for _, p := range planted {
		if got := findExternalReferences(p); len(got) == 0 {
			t.Errorf("the scanner did not flag %q, so it cannot be trusted to flag anything", p)
		}
	}
	// And it must not flag the one thing that is allowed, or every page fails
	// for a reason that is not a fetch.
	if got := findExternalReferences(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">`); len(got) != 0 {
		t.Errorf("the scanner flagged the SVG namespace: %v", got)
	}
}

// TestNoAssetReferencesAnExternalURL scans the embedded files.
func TestNoAssetReferencesAnExternalURL(t *testing.T) {
	files := append(assetNames(), templateNames()...)
	if len(files) == 0 {
		t.Fatal("no embedded files were found, so this test scanned nothing")
	}
	scanned := 0
	for _, name := range files {
		var body []byte
		var err error
		if strings.HasPrefix(name, "assets/") {
			body, err = assetFS.ReadFile(name)
		} else {
			body, err = templateFS.ReadFile(name)
		}
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if len(body) == 0 {
			t.Errorf("%s is empty", name)
			continue
		}
		scanned++
		if found := findExternalReferences(string(body)); len(found) > 0 {
			t.Errorf("%s refers to something outside this box: %v", name, found)
		}
	}
	if scanned < 6 {
		t.Errorf("only %d files were scanned; the stylesheet, the script, the icon and the four templates are expected", scanned)
	}
}

// TestNoRenderedPageReferencesAnExternalURL scans what the panel actually
// serves, which is not the same thing as what it embeds: a URL could be built
// at render time out of data.
func TestNoRenderedPageReferencesAnExternalURL(t *testing.T) {
	h := newHarness(t)

	// Signed out first, so the login and setup pages are covered.
	for _, path := range []string{"/setup", "/login", "/"} {
		_, body := h.get(path)
		if found := findExternalReferences(body); len(found) > 0 {
			t.Errorf("%s (signed out) refers to something outside this box: %v", path, found)
		}
	}

	h.ready()
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test", Devices: 1})
	if res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}}); res.StatusCode != 303 {
		t.Fatal("could not switch on")
	}

	pages := []string{
		"/", "/?advanced=1", "/status.json",
		"/assets/panel.css", "/assets/panel.js", "/favicon.svg",
		"/wp-admin",
	}
	for _, path := range pages {
		_, body := h.get(path)
		if body == "" && path != "/wp-admin" {
			t.Errorf("%s served an empty body", path)
		}
		if found := findExternalReferences(body); len(found) > 0 {
			t.Errorf("%s refers to something outside this box: %v", path, found)
		}
	}

	// The page with a problem on it is a page too, and an error template is a
	// classic place for a "report this at https://..." link to appear.
	if res, _ := h.postForm("/config", url.Values{"csrf": {h.tokenOn("/")}, "config": {unparseableConfig()}}); res.StatusCode != 303 {
		t.Fatal("could not reach the failure path")
	}
	_, body := h.get("/?advanced=1")
	if found := findExternalReferences(body); len(found) > 0 {
		t.Errorf("the page showing a failure refers to something outside this box: %v", found)
	}
}

// TestEveryAssetTheHTMLNamesIsServed catches the other half of the same
// problem: a page that names /assets/whatever.css which does not exist loads
// unstyled, and nothing else notices.
func TestEveryAssetTheHTMLNamesIsServed(t *testing.T) {
	h := newHarness(t)
	h.ready()
	_, body := h.get("/")

	refs := regexp.MustCompile(`(?:href|src)="(/[^"]*)"`).FindAllStringSubmatch(body, -1)
	if len(refs) == 0 {
		t.Fatal("the page names no local resources at all, which cannot be right")
	}
	checked := 0
	for _, m := range refs {
		path := m[1]
		// Skip the in-page navigation and the anchors.
		if path == "/" || strings.HasPrefix(path, "/?") || strings.HasPrefix(path, "/#") {
			continue
		}
		res, _ := h.get(path)
		if res.StatusCode != 200 {
			t.Errorf("the page names %s, which the panel answers with %d", path, res.StatusCode)
		}
		checked++
	}
	if checked == 0 {
		t.Error("no asset reference was checked")
	}
}

// TestScriptDisplaysNoTextOfItsOwn is the guard left behind by a defect that
// shipped and that only a Persian reader would have reported.
//
// panel.js relabelled the switch on every poll with the words "Switch off" and
// "Switch on" written into the file. On a Persian page, which is the default,
// the button read correctly for five seconds and then turned English and
// stayed English until the page was reloaded. The file's own comment two
// blocks above claimed it "has no idea which language the page is in and must
// not learn", which was true of every other field it touched.
//
// The rule the comment states is now checked: every string the script puts on
// the page comes from the server, already translated. A literal reaching a
// display call fails here rather than in a screenshot somebody has to read
// Persian to judge.
func TestScriptDisplaysNoTextOfItsOwn(t *testing.T) {
	body, err := assetFS.ReadFile("assets/panel.js")
	if err != nil {
		t.Fatal(err)
	}
	// Comments are stripped first: this file explains the rule in prose, and
	// the prose quotes the English words on purpose.
	src := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(body), " ")
	src = regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(src, " ")

	// A display call is setText, or an assignment to textContent or innerHTML.
	// The argument has to be an expression over the polled status, never a
	// literal with words in it.
	display := regexp.MustCompile(`setText\s*\([^;]*?\)|\.(?:textContent|innerHTML|innerText)\s*=\s*[^;]*;`)
	words := regexp.MustCompile(`"[^"]*\p{L}{3,}[^"]*"|'[^']*\p{L}{3,}[^']*'`)

	found := 0
	for _, call := range display.FindAllString(src, -1) {
		found++
		if lit := words.FindString(call); lit != "" {
			t.Errorf("panel.js puts the literal %s on the page: %s\n"+
				"every displayed string comes from the server already translated, "+
				"because this file cannot know which language the page is in",
				lit, strings.TrimSpace(call))
		}
	}
	if found == 0 {
		t.Fatal("no display calls were found in panel.js, so this test checked nothing")
	}
	t.Logf("%d display calls checked", found)
}

// The outside link must be a link a person follows, never something the page
// loads.
//
// TestNoAssetReferencesAnExternalURL cannot tell those apart: it flags any
// absolute URL wherever it sits, so allowlisting this one to add it to the rail
// would also have permitted it as a script src, a stylesheet, a background
// image or a form action. Each of those would fetch on render, which is exactly
// the thing the panel promises not to do, and the promise is not decoration: a
// box with no uplink has to draw its own panel, and opening the page must tell
// nobody that it was opened.
//
// So this checks the position rather than the presence.
func TestTheOutsideLinkIsANavigationLinkAndNeverAFetch(t *testing.T) {
	outsideLinks := []string{"javidnetworkwatch.com", "github.com/Iman/caspian"}

	check := func(where, body string) {
		for _, outside := range outsideLinks {
			// Every way a browser is made to request something without being asked.
			fetching := []*regexp.Regexp{
				regexp.MustCompile(`(?i)src\s*=\s*["']?[^"'>]*` + regexp.QuoteMeta(outside)),
				regexp.MustCompile(`(?i)<link[^>]*` + regexp.QuoteMeta(outside)),
				regexp.MustCompile(`(?i)@import[^;]*` + regexp.QuoteMeta(outside)),
				regexp.MustCompile(`(?i)url\s*\([^)]*` + regexp.QuoteMeta(outside)),
				regexp.MustCompile(`(?i)fetch\s*\([^)]*` + regexp.QuoteMeta(outside)),
				regexp.MustCompile(`(?i)<form[^>]*action\s*=\s*["']?[^"'>]*` + regexp.QuoteMeta(outside)),
				regexp.MustCompile(`(?i)<iframe[^>]*` + regexp.QuoteMeta(outside)),
			}
			for _, re := range fetching {
				if m := re.FindString(body); m != "" {
					t.Errorf("%s loads %s on render: %q. The rail link is allowed because a "+
						"person has to click it. Anything the page requests by itself breaks the "+
						"promise that this panel works with no uplink and tells nobody it was opened.",
						where, outside, m)
				}
			}
			// And where it IS present, it carries noreferrer, or the site is told
			// the panel's own address on the hotspot.
			if strings.Contains(body, outside) {
				// The window has to reach FORWARD far enough, because rel comes
				// after href in the attribute order actually used. The first
				// version looked 240 characters back and 40 forward, and failed on
				// correct markup: "noreferrer" sits about 55 characters past the
				// host. A guard that fails on the thing it is meant to permit
				// teaches people to delete it.
				i := strings.Index(body, outside)
				start := i - 240
				if start < 0 {
					start = 0
				}
				end := i + 240
				if end > len(body) {
					end = len(body)
				}
				around := body[start:end]
				if !strings.Contains(around, "noreferrer") {
					t.Errorf("%s links to %s without rel=\"noreferrer\", so a click tells that "+
						"site the address of this panel", where, outside)
				}
			}
		}
	}

	for _, name := range append(assetNames(), templateNames()...) {
		var body []byte
		var err error
		if strings.HasPrefix(name, "assets/") {
			body, err = assetFS.ReadFile(name)
		} else {
			body, err = templateFS.ReadFile(name)
		}
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		check(name, string(body))
	}

	// And what is actually served, which is not the same as what is embedded.
	h := newHarness(t)
	for _, path := range []string{"/", "/help", "/login"} {
		_, body := h.get(path)
		check(path+" (as served)", body)
	}
}
