// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

// The behaviour-regression layer for the panel.
//
// WHAT THIS IS FOR, AND WHAT IT IS NOT FOR.
//
// The other tests in this package assert PROPERTIES: that every route needs a
// session, that no page carries a missing-message marker, that the contrast of
// every drawn pair clears WCAG. Each one names the thing it is checking, so
// each one is blind to everything it does not name. A page can lose a whole
// section, a status document can lose a field, and a message key can disappear
// from one language, without a single property test going red.
//
// This file pins the OUTPUT instead. It renders every state a user can be in,
// in both languages, and freezes the bytes. Nothing here says the output is
// right. It says the output has not changed, so that a change to it arrives as
// a diff somebody has to read and approve rather than as silence.
//
// To regenerate every golden in the repository, one command:
//
//	bash scripts/golden-update.sh
//
// or for this package alone:
//
//	go test ./internal/panel -run Golden -update
//
// then READ THE DIFF before committing it.
//
// # Credentials, and why redaction is scoped rather than global
//
// The dashboard renders the hotspot passphrase in the clear, because somebody
// standing at the box has to be able to read it, and it encodes that passphrase
// into a join QR code. Those bytes cannot be committed.
//
// The redactor below is therefore applied PER ARTEFACT and never globally:
//
//   - The rendered dashboard has the passphrase and the QR modules redacted,
//     because they belong there.
//   - status.json has NOTHING redacted, because no credential belongs in it.
//     If one ever appears it lands in the golden, and test/goldenscan finds it.
//
// A global redactor would have hidden the second case, which is the one that
// matters. What keeps the first case honest is secret-exposure.txt: it counts,
// per state and per language, how many times each credential appears in the RAW
// body before any redaction. A page that starts echoing the proxy UUID moves a
// zero to a one and the golden diffs.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/panel/qr"
)

// update rewrites the golden files. See the note at the top of the file.
var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// ---------------------------------------------------------------------------
// The sentinels
//
// Every credential this layer feeds to the panel is a sentinel: a value that
// exists nowhere else in the repository and that cannot occur by accident. That
// is what lets test/goldenscan report a hit with no false positives, and it is
// why these are not reused from panel_test.go, whose testPassword is also the
// passphrase in internal/hotspot/testdata/hostapd.golden.
//
// Nothing here is, or has ever been, a working credential.
// ---------------------------------------------------------------------------

const (
	// goldenPassphrase is the WPA passphrase the pinned pages render. Twenty
	// characters of the generator's own alphabet plus a marker word, so it is
	// a valid WPA2 passphrase (8 to 63 printable ASCII) and is unmistakable in
	// a scan.
	goldenPassphrase = "sentinelwpa-kzmqvrxt7"

	// goldenSSID is not a credential: an SSID is broadcast. It is fixed only
	// so the QR code and the page are deterministic.
	goldenSSID = "Caspian-golden"

	// goldenPanelPassword is the panel login password.
	goldenPanelPassword = "sentinelpanelpw-4bq8"

	// goldenConfigLabel is the user's own name for the config. Arbitrary user
	// text that gets rendered, so it also holds the escaping in place.
	goldenConfigLabel = `Golden <box> & "friends"`
)

// goldenSecrets is every value that must never reach a golden file unredacted,
// with the name that appears in secret-exposure.txt.
//
// The proxy values come from the package's own fixtures: they are the config a
// user pasted, and the whole question about the dashboard is whether any part
// of it is echoed back. The answer is pinned rather than asserted, so a change
// to it is visible.
func goldenSecrets() []struct{ name, value string } {
	return []struct{ name, value string }{
		{"hotspot-passphrase", goldenPassphrase},
		{"panel-password", goldenPanelPassword},
		{"proxy-raw-link", testLink()},
		{"proxy-uuid", fakeUUIDForPanel},
		{"proxy-reality-key", fakePublicKeyForPanel()},
		{"proxy-short-id", fakeShortIDForPanel},
	}
}

// ---------------------------------------------------------------------------
// The states
// ---------------------------------------------------------------------------

// goldenClockBase is the instant every pinned page is rendered at. It is the
// same value newTestClock uses, restated here so a change to that helper is a
// visible diff in this layer rather than a silent shift of every timestamp.
var goldenClockBase = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

// goldenState is one state of the appliance worth freezing.
type goldenState struct {
	// name becomes part of every golden filename.
	name string

	// path is the request the pinned page is the answer to.
	path string

	// statusJSON says whether /status.json is pinned for this state too. It is
	// false for the states with no session, where the endpoint answers 401.
	statusJSON bool

	// arrange drives the harness into the state.
	arrange func(h *harness)

	// note is written into the golden's own header, so the file says what it
	// is a picture of without anybody having to find this table.
	note string
}

// goldenStates is every state pinned. Adding one here and running -update is
// the whole procedure.
func goldenStates() []goldenState {
	// ready puts a config, a hotspot and a session in place. It is the
	// starting point for every signed-in state.
	ready := func(h *harness) {
		h.setup(goldenPanelPassword)
		if err := h.store.SetHotspot(goldenSSID, goldenPassphrase); err != nil {
			h.t.Fatalf("SetHotspot: %v", err)
		}
		if err := h.store.SetProxyConfig(testLink(), "vless", goldenConfigLabel); err != nil {
			h.t.Fatalf("SetProxyConfig: %v", err)
		}
	}
	// running puts the engine in a phase with a fixed Since, so the uptime
	// tile is a function of the test clock and not of the wall clock. The
	// fake's own Start would stamp time.Now, which would churn this layer on
	// every run.
	running := func(h *harness, phase engine.Phase, since time.Duration, devices int) {
		h.priv.SetEngineState(engine.State{Phase: phase, Since: goldenClockBase.Add(-since)})
		h.priv.SetHotspot(HotspotStatus{Running: true, SSID: goldenSSID, Devices: devices})
	}

	return []goldenState{
		{
			name: "first-run-setup",
			path: "/setup",
			note: "First run. No panel password has been chosen, so the box shows setup rather than login.",
		},
		{
			name: "signed-out-login",
			path: "/login",
			arrange: func(h *harness) {
				h.setup(goldenPanelPassword)
				h.signedOut()
			},
			note: "A password exists and this browser has no session.",
		},
		{
			name:       "signed-in-off",
			path:       "/",
			statusJSON: true,
			arrange:    ready,
			note:       "Signed in, everything configured, the appliance switched off.",
		},
		{
			name:       "running-not-connected",
			path:       "/",
			statusJSON: true,
			arrange: func(h *harness) {
				ready(h)
				running(h, engine.PhaseStarting, 30*time.Second, 0)
			},
			note: "The switch has been pressed and the tunnel is not up yet. This is the amber state.",
		},
		{
			name:       "connected",
			path:       "/",
			statusJSON: true,
			arrange: func(h *harness) {
				ready(h)
				running(h, engine.PhaseRunning, 2*time.Hour, 3)
			},
			note: "Working: tunnel up, hotspot up, three devices joined.",
		},
		{
			name:       "connected-traffic-cut",
			path:       "/",
			statusJSON: true,
			arrange: func(h *harness) {
				ready(h)
				running(h, engine.PhaseRunning, 2*time.Hour, 3)
				if err := h.priv.Cut(h.t.Context()); err != nil {
					h.t.Fatalf("Cut: %v", err)
				}
			},
			note: "Running and deliberately carrying nothing. Read cut.banner in this page against docs: " +
				"the sentence claims a restart restores traffic and it does not. See PROVENANCE.md, defect B.",
		},
		{
			name:       "advanced",
			path:       "/?advanced=1",
			statusJSON: false,
			arrange: func(h *harness) {
				ready(h)
				running(h, engine.PhaseRunning, 2*time.Hour, 3)
				h.priv.SetEngineLog(EngineLog{
					Entries: []engine.LogEntry{
						{At: goldenClockBase.Add(-90 * time.Second), Text: "engine: started"},
						{At: goldenClockBase.Add(-60 * time.Second), Text: "engine: tunnel is up"},
					},
					Dropped: 4,
				})
			},
			note: "The advanced view, with a fixed engine log so the pinned bytes do not move with the clock.",
		},
		{
			name:       "help",
			path:       "/help",
			statusJSON: false,
			arrange: func(h *harness) {
				ready(h)
				running(h, engine.PhaseRunning, 2*time.Hour, 3)
			},
			note: "The help page. help.controls.cut on this page carries defect B; see PROVENANCE.md.",
		},
		{
			name:       "defect-devices-while-off",
			path:       "/",
			statusJSON: true,
			arrange: func(h *harness) {
				ready(h)
				// Off, hotspot down, and a lease still counted. This is
				// defect A, pinned as it behaves today and not as it should.
				h.priv.SetEngineState(engine.State{Phase: engine.PhaseStopped})
				h.priv.SetHotspot(HotspotStatus{Running: false, SSID: "", Devices: 1})
			},
			note: "DEFECT A, PINNED AS-IS. The appliance is off and the hotspot is down, and the page " +
				"still reports a joined device. See PROVENANCE.md, defect A. When it is fixed this golden " +
				"MUST change; that diff is the point of the file.",
		},
	}
}

// goldenLangs is every language a page is pinned in. Persian is first because
// it is the product's default and the one most users see.
var goldenLangs = []Lang{LangFA, LangEN}

// ---------------------------------------------------------------------------
// The golden mechanism
// ---------------------------------------------------------------------------

func goldenPath(name string) string { return filepath.Join("testdata", name) }

// assertGoldenBytes is the whole comparison. On -update it writes; otherwise it
// reports the FIRST differing line, which is what makes a 400-line HTML diff
// readable in a terminal.
func assertGoldenBytes(t *testing.T, name string, got string) {
	t.Helper()
	path := goldenPath(name)

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v\n\nTo create it: bash scripts/golden-update.sh", path, err)
	}
	if got == string(want) {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		g, w := "", ""
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("%s differs at line %d\n  got: %s\n want: %s\n\n"+
				"The observable behaviour of the panel changed. If that was intended, run\n"+
				"    bash scripts/golden-update.sh\n"+
				"READ THE DIFF, then commit it.", path, i+1, g, w)
		}
	}
	t.Fatalf("%s: content differs but no differing line was found (line count %d vs %d)",
		path, len(gotLines), len(wantLines))
}

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

var (
	csrfInputRE = regexp.MustCompile(`(name="csrf" value=")[^"]*(")`)
	qrPathRE    = regexp.MustCompile(`(<path class="qr-fg" fill="currentColor" d=")([^"]*)(")`)
	// The two suggested values are freshly generated on every render of a page
	// with no hotspot saved. They are not credentials in use, and they are not
	// deterministic, so they cannot be pinned.
	suggestSSIDRE = regexp.MustCompile(`(<input id="ssid"[^>]*value=")([^"]*)(")`)
	suggestPassRE = regexp.MustCompile(`(<input id="passphrase"[^>]*value=")([^"]*)(")`)
)

// redactPage removes from a rendered page everything that is either a
// credential that belongs there or a value that changes on every run.
//
// The QR modules are replaced by a digest of themselves rather than by a
// constant. That keeps the property the netcfg layer established: a change to
// what the code produces is still a diff, while the bytes that carry the
// passphrase never land in the repository.
//
// hotspotSaved decides whether the two form inputs are redacted, and getting
// that wrong is not a small matter. The first version of this function ran the
// suggestion regexes unconditionally, and they matched the form on a page whose
// hotspot IS saved: the committed golden showed <redacted:suggested-ssid> where
// the real page shows the network name. A broadcast SSID is not a secret, and
// hiding it hid the very thing the golden exists to pin, which is WHICH value
// the form is prefilled with. Suggestions are generated fresh on every render
// and can only be pinned as a placeholder; saved values must never be.
func redactPage(body string, hotspotSaved bool) string {
	out := csrfInputRE.ReplaceAllString(body, "${1}<redacted:csrf-token>${2}")

	out = qrPathRE.ReplaceAllStringFunc(out, func(m string) string {
		g := qrPathRE.FindStringSubmatch(m)
		sum := sha256.Sum256([]byte(g[2]))
		return fmt.Sprintf("%s<redacted:qr-modules sha256=%x len=%d>%s",
			g[1], sum[:8], len(g[2]), g[3])
	})

	// The saved passphrase, wherever it is rendered. Done after the QR, so the
	// count in secret-exposure.txt and this replacement cannot disagree about
	// what is inside the SVG.
	out = strings.ReplaceAll(out, goldenPassphrase, "<redacted:hotspot-passphrase>")

	if !hotspotSaved {
		out = suggestSSIDRE.ReplaceAllString(out, "${1}<redacted:suggested-ssid>${3}")
		out = suggestPassRE.ReplaceAllString(out, "${1}<redacted:suggested-passphrase>${3}")
	}
	return out
}

// header is the comment block written at the top of every pinned page, so the
// file explains itself to somebody who opened it from a diff.
func header(state goldenState, lang Lang, kind string) string {
	return fmt.Sprintf(""+
		"<!-- GOLDEN FILE. Generated by internal/panel/golden_test.go. Do not edit by hand.\n"+
		"     Regenerate with: bash scripts/golden-update.sh\n"+
		"     state: %s\n"+
		"     lang:  %s\n"+
		"     path:  %s\n"+
		"     kind:  %s\n"+
		"     note:  %s\n"+
		"     Redacted: the CSRF token, the QR modules (kept as a digest), and the\n"+
		"     hotspot passphrase. Nothing else is redacted, so any other credential\n"+
		"     that reaches this page lands in this file and test/goldenscan fails.\n"+
		"-->\n", state.name, lang, state.path, kind, state.note)
}

// ---------------------------------------------------------------------------
// The rendered pages
// ---------------------------------------------------------------------------

// fetchGolden arranges a state, requests its page in one language, and returns
// the status code, the raw body, and whether a hotspot was saved when the page
// was rendered.
//
// The third value is read from the store rather than declared in the state
// table, so it cannot drift from what the page actually saw.
func fetchGolden(t *testing.T, s goldenState, lang Lang, path string) (int, string, bool) {
	t.Helper()
	h := newHarness(t)
	if lang == LangEN {
		h.useEnglish()
	}
	if s.arrange != nil {
		s.arrange(h)
	}
	res, body := h.get(path)
	return res.StatusCode, body, h.store.Snapshot().Hotspot.SSID != ""
}

// TestGolden_RenderedPages pins the HTML of every state in both languages.
func TestGolden_RenderedPages(t *testing.T) {
	for _, s := range goldenStates() {
		for _, lang := range goldenLangs {
			t.Run(s.name+"-"+string(lang), func(t *testing.T) {
				code, body, saved := fetchGolden(t, s, lang, s.path)
				if code != http.StatusOK {
					t.Fatalf("GET %s returned %d, want 200; the state table and the panel disagree", s.path, code)
				}
				out := header(s, lang, "rendered HTML") + redactPage(body, saved)
				assertGoldenBytes(t, fmt.Sprintf("page-%s-%s.html", s.name, lang), out)
			})
		}
	}
}

// TestGolden_SecretExposure pins how many times each credential appears in the
// RAW body of each page, before redaction.
//
// This is the guard that stops redaction from hiding a leak. redactPage removes
// the hotspot passphrase from the pinned HTML because it belongs on that page;
// without this file, a change that started printing it in six more places, or
// that started printing the proxy UUID, would be invisible in the diff.
func TestGolden_SecretExposure(t *testing.T) {
	var b strings.Builder
	b.WriteString("# GOLDEN FILE. Generated by internal/panel/golden_test.go.\n")
	b.WriteString("# Regenerate with: bash scripts/golden-update.sh\n")
	b.WriteString("#\n")
	b.WriteString("# How many times each credential appears in the RAW response body of each\n")
	b.WriteString("# page, BEFORE the golden redactor runs. A zero that becomes a non-zero is a\n")
	b.WriteString("# credential leaking into a page that did not carry it before.\n")
	b.WriteString("#\n")
	b.WriteString("# hotspot-passphrase is deliberately non-zero on the dashboard: somebody\n")
	b.WriteString("# standing at the box has to be able to read it, and it is encoded into the\n")
	b.WriteString("# join QR. Every other row must stay zero on every page, forever.\n")

	for _, s := range goldenStates() {
		for _, lang := range goldenLangs {
			_, body, _ := fetchGolden(t, s, lang, s.path)
			fmt.Fprintf(&b, "\nstate=%s lang=%s path=%s\n", s.name, lang, s.path)
			for _, sec := range goldenSecrets() {
				fmt.Fprintf(&b, "  %-20s %d\n", sec.name, strings.Count(body, sec.value))
			}
		}
	}
	assertGoldenBytes(t, "secret-exposure.txt", b.String())
}

// TestGolden_WiFiJoinString pins the join string the QR encodes, with the
// passphrase redacted.
//
// The QR modules in the page golden are a digest, which detects a change and
// explains nothing. This file is the readable half: it shows the SHAPE of what
// a phone scans, so a change to the escaping, to the security type, or to the
// hidden flag is a sentence somebody can read rather than a hash that moved.
func TestGolden_WiFiJoinString(t *testing.T) {
	var b strings.Builder
	b.WriteString("# GOLDEN FILE. Generated by internal/panel/golden_test.go.\n")
	b.WriteString("# Regenerate with: bash scripts/golden-update.sh\n")
	b.WriteString("#\n")
	b.WriteString("# The string the join QR code encodes. The passphrase is replaced by a\n")
	b.WriteString("# placeholder; everything around it is the observable format.\n\n")

	h := newHarness(t)
	h.setup(goldenPanelPassword)
	if err := h.store.SetHotspot(goldenSSID, goldenPassphrase); err != nil {
		t.Fatalf("SetHotspot: %v", err)
	}
	var d pageData
	d.Lang = LangFA
	d.fillHotspot(h.store.Snapshot().Hotspot)
	join := qr.WiFiJoin(string(d.SSID), string(d.Passphrase), false)
	fmt.Fprintf(&b, "ssid=%s\n", goldenSSID)
	fmt.Fprintf(&b, "join=%s\n", strings.ReplaceAll(join, goldenPassphrase, "<redacted:hotspot-passphrase>"))
	fmt.Fprintf(&b, "qr-svg-bytes=%d\n", len(string(d.QR)))
	assertGoldenBytes(t, "wifi-join.txt", b.String())
}

// ---------------------------------------------------------------------------
// status.json
// ---------------------------------------------------------------------------

// TestGolden_StatusJSON pins the polled document for every state that has a
// session, in both languages.
//
// The golden holds the INDENTED form, because a one-line document is not a
// reviewable diff. TestGolden_StatusJSONGoldenIsTheWireBytes proves the
// indented form and the bytes on the wire are the same document, so the
// readability costs nothing.
func TestGolden_StatusJSON(t *testing.T) {
	for _, s := range goldenStates() {
		if !s.statusJSON {
			continue
		}
		for _, lang := range goldenLangs {
			t.Run(s.name+"-"+string(lang), func(t *testing.T) {
				code, body, _ := fetchGolden(t, s, lang, "/status.json")
				if code != http.StatusOK {
					t.Fatalf("GET /status.json returned %d, want 200", code)
				}
				var pretty bytes.Buffer
				if err := json.Indent(&pretty, []byte(body), "", "  "); err != nil {
					t.Fatalf("the endpoint did not return JSON: %v", err)
				}
				assertGoldenBytes(t, fmt.Sprintf("status-%s-%s.json", s.name, lang), pretty.String()+"\n")
			})
		}
	}
}

// TestGolden_StatusJSONGoldenIsTheWireBytes keeps the indentation honest.
//
// A pretty-printed golden that no longer compacts to what the endpoint sends is
// a golden of something the product does not do. This reads the COMMITTED
// files, compacts them, and compares against a live response.
func TestGolden_StatusJSONGoldenIsTheWireBytes(t *testing.T) {
	if *update {
		t.Skip("nothing to check while rewriting the goldens")
	}
	checked := 0
	for _, s := range goldenStates() {
		if !s.statusJSON {
			continue
		}
		for _, lang := range goldenLangs {
			name := fmt.Sprintf("status-%s-%s.json", s.name, lang)
			want, err := os.ReadFile(goldenPath(name))
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, want); err != nil {
				t.Fatalf("%s is not valid JSON: %v", name, err)
			}
			_, body, _ := fetchGolden(t, s, lang, "/status.json")
			if strings.TrimSpace(body) != compact.String() {
				t.Errorf("%s does not compact to the bytes /status.json actually sends.\n"+
					"golden compacted: %s\n         on the wire: %s",
					name, compact.String(), strings.TrimSpace(body))
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no status.json goldens were checked, so this guard passed vacuously")
	}
	t.Logf("checked %d status.json goldens against live responses", checked)
}

// TestGolden_StatusJSONShape pins the field set of the document, and the exact
// value set of heroClass and powerClass.
//
// The per-state goldens above pin the values a particular state produces. This
// one pins the CONTRACT: which keys exist, what Go type each carries, and every
// class name the two functions can ever return. panel.js keys on both of those
// class names, so a value added here without a stylesheet rule behind it is a
// control that draws as nothing.
func TestGolden_StatusJSONShape(t *testing.T) {
	var b strings.Builder
	b.WriteString("# GOLDEN FILE. Generated by internal/panel/golden_test.go.\n")
	b.WriteString("# Regenerate with: bash scripts/golden-update.sh\n")
	b.WriteString("#\n")
	b.WriteString("# The contract of GET /status.json: every key, and the JSON type behind it.\n")
	b.WriteString("# A key that disappears here disappears from assets/panel.js's reach, and\n")
	b.WriteString("# the control it drives stops updating without a reload.\n\n")
	b.WriteString("## fields\n")

	// Take the shape from a live response rather than from the struct, so this
	// records what a browser receives, tags and omissions included.
	code, body, _ := fetchGolden(t, goldenStateNamed(t, "connected"), LangFA, "/status.json")
	if code != http.StatusOK {
		t.Fatalf("GET /status.json returned %d", code)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("unmarshalling the status document: %v", err)
	}
	names := make([]string, 0, len(doc))
	for k := range doc {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(&b, "  %-12s %s\n", k, jsonTypeName(doc[k]))
	}

	// The exhaustive truth tables. Three booleans is eight rows, so this is the
	// whole domain rather than a sample of it.
	b.WriteString("\n## heroClass(cut, connected, running) over its whole domain\n")
	heroSeen := map[string]bool{}
	for _, cut := range []bool{false, true} {
		for _, connected := range []bool{false, true} {
			for _, running := range []bool{false, true} {
				v := heroClass(cut, connected, running)
				heroSeen[v] = true
				fmt.Fprintf(&b, "  cut=%-5v connected=%-5v running=%-5v -> %s\n", cut, connected, running, v)
			}
		}
	}
	b.WriteString("\n## powerClass(connected, running) over its whole domain\n")
	powerSeen := map[string]bool{}
	for _, connected := range []bool{false, true} {
		for _, running := range []bool{false, true} {
			v := powerClass(connected, running)
			powerSeen[v] = true
			fmt.Fprintf(&b, "  connected=%-5v running=%-5v -> %s\n", connected, running, v)
		}
	}
	b.WriteString("\n## the value sets\n")
	fmt.Fprintf(&b, "  heroClass  %s\n", strings.Join(sortedKeys(heroSeen), " "))
	fmt.Fprintf(&b, "  powerClass %s\n", strings.Join(sortedKeys(powerSeen), " "))

	assertGoldenBytes(t, "status-shape.txt", b.String())
}

func jsonTypeName(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return fmt.Sprintf("array[%d]", len(t))
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// The message catalogue
// ---------------------------------------------------------------------------

// TestGolden_MessageKeys pins the full inventory of i18n keys per language.
//
// KEYS ONLY, never the text. The text is edited constantly and a golden of it
// would be a diff on every wording change, which trains people to run -update
// without reading. A key is different: a key that disappears from one language
// is a page that renders "!!some.key!!" to exactly the users the default
// language exists to serve, and nothing else in this package catches its
// disappearance from BOTH languages at once.
func TestGolden_MessageKeys(t *testing.T) {
	var b strings.Builder
	b.WriteString("# GOLDEN FILE. Generated by internal/panel/golden_test.go.\n")
	b.WriteString("# Regenerate with: bash scripts/golden-update.sh\n")
	b.WriteString("#\n")
	b.WriteString("# Every message key in the catalogue, per language. Text is deliberately\n")
	b.WriteString("# NOT pinned: it changes often and a churning golden trains people to\n")
	b.WriteString("# accept diffs unread. A key vanishing is the failure this file catches.\n")

	all := map[Key]map[Lang]bool{}
	for _, l := range goldenLangs {
		ks := keys(l)
		fmt.Fprintf(&b, "\n## %s: %d keys\n", l, len(ks))
		for _, k := range ks {
			fmt.Fprintf(&b, "  %s\n", k)
			if all[k] == nil {
				all[k] = map[Lang]bool{}
			}
			all[k][l] = true
		}
	}

	b.WriteString("\n## keys missing from a language\n")
	missing := 0
	names := make([]string, 0, len(all))
	for k := range all {
		names = append(names, string(k))
	}
	sort.Strings(names)
	for _, n := range names {
		for _, l := range goldenLangs {
			if !all[Key(n)][l] {
				fmt.Fprintf(&b, "  %s is missing from %s\n", n, l)
				missing++
			}
		}
	}
	if missing == 0 {
		b.WriteString("  (none)\n")
	}
	assertGoldenBytes(t, "message-keys.txt", b.String())
}

// ---------------------------------------------------------------------------
// The routes
// ---------------------------------------------------------------------------

// TestGolden_RouteInventory pins every route and what it does to a caller who
// is not signed in, and to one who is signed in without a CSRF token.
//
// It records OBSERVED status codes rather than the route table's own Public and
// JSON fields. The table is what the code intends; the codes are what a browser
// gets. TestEveryRouteRefusesWithoutASession already asserts the property; this
// pins the answer, so a route that silently changes from a redirect to a 200 is
// a diff.
func TestGolden_RouteInventory(t *testing.T) {
	var b strings.Builder
	b.WriteString("# GOLDEN FILE. Generated by internal/panel/golden_test.go.\n")
	b.WriteString("# Regenerate with: bash scripts/golden-update.sh\n")
	b.WriteString("#\n")
	b.WriteString("# Every route the panel serves, with what it answers to:\n")
	b.WriteString("#   no-session  a browser with no session cookie at all\n")
	b.WriteString("#   no-csrf     a signed-in browser posting without a token\n")
	b.WriteString("# 303 is a redirect to /login or /setup. 401 is the JSON refusal. 403 is\n")
	b.WriteString("# the CSRF refusal, deliberately not a redirect: a redirect would look\n")
	b.WriteString("# like it worked.\n")
	b.WriteString("#\n")
	b.WriteString("# GET routes have no CSRF check by design; they are marked n/a.\n\n")
	fmt.Fprintf(&b, "%-6s %-20s %-8s %-8s %-11s %s\n",
		"METHOD", "PATH", "PUBLIC", "JSON", "NO-SESSION", "NO-CSRF")

	for _, rt := range routes {
		noSession := probeRoute(t, rt, false)
		noCSRF := "n/a"
		if rt.Method != http.MethodGet {
			noCSRF = probeRoute(t, rt, true)
		}
		fmt.Fprintf(&b, "%-6s %-20s %-8v %-8v %-11s %s\n",
			rt.Method, rt.Path, rt.Public, rt.JSON, noSession, noCSRF)
	}
	assertGoldenBytes(t, "routes.txt", b.String())
}

// probeRoute makes one request and returns the status code as a string.
//
// signedIn=false drops every cookie first, which is the state of a browser that
// has never seen the panel. signedIn=true signs in and then posts with no token
// at all, which is the shape a cross-site submission takes.
func probeRoute(t *testing.T, rt routeSpec, signedIn bool) string {
	t.Helper()
	h := newHarness(t)
	h.setup(goldenPanelPassword)
	if !signedIn {
		h.signedOut()
	}
	var res *http.Response
	if rt.Method == http.MethodGet {
		res, _ = h.get(rt.Path)
	} else {
		res, _ = h.postForm(rt.Path, nil)
	}
	return fmt.Sprintf("%d", res.StatusCode)
}

// goldenStateNamed looks a state up by name rather than by index, so that
// reordering the table cannot silently repoint a test at a different state.
func goldenStateNamed(t *testing.T, name string) goldenState {
	t.Helper()
	for _, s := range goldenStates() {
		if s.name == name {
			return s
		}
	}
	t.Fatalf("no golden state named %q", name)
	return goldenState{}
}

// ---------------------------------------------------------------------------
// Guards on this layer itself
// ---------------------------------------------------------------------------

// TestGolden_RedactionDoesNotHideSavedValues is the guard for a mistake this
// layer already made once.
//
// The suggestion regexes matched the hotspot form on a page whose hotspot was
// saved, so the committed golden showed a placeholder where the real page shows
// the network name. That is worse than no golden: it pins the redactor rather
// than the product, and it silently removes the thing the file exists to show.
//
// The rule this holds: a placeholder for a GENERATED value may appear only in a
// golden whose state has nothing saved, and the saved SSID must be visible
// verbatim in every golden whose state has one.
func TestGolden_RedactionDoesNotHideSavedValues(t *testing.T) {
	if *update {
		t.Skip("nothing to check while rewriting the goldens")
	}
	// Both halves are checked, and the second one is not optional.
	//
	// The first version of this guard read only the COMMITTED files. A mutation
	// run on 2026-08-30 showed why that is not enough: changing redactPage so it
	// always redacts was NOT detected, because the committed bytes do not move
	// until somebody runs -update. The guard would have gone red only after the
	// damage was already in a golden. So the live redaction of a freshly
	// rendered page is checked as well, and that is the half that catches the
	// redactor changing.
	checked := 0
	for _, s := range goldenStates() {
		for _, lang := range goldenLangs {
			name := fmt.Sprintf("page-%s-%s.html", s.name, lang)
			raw, err := os.ReadFile(goldenPath(name))
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			_, body, saved := fetchGolden(t, s, lang, s.path)
			live := redactPage(body, saved)
			checked++

			for _, half := range []struct{ what, text string }{
				{"the committed golden", string(raw)},
				{"the live redaction of a freshly rendered page", live},
			} {
				if !saved {
					continue
				}
				if strings.Contains(half.text, "<redacted:suggested-") {
					t.Errorf("%s: the state has a saved hotspot and %s carries a "+
						"<redacted:suggested-...> placeholder, so the redactor is hiding a "+
						"saved value", name, half.what)
				}
				// Only pages that actually carry the hotspot form can be
				// over-redacted this way. The help page renders no hotspot
				// section at all, and demanding the SSID there would be a
				// guard that fails for the wrong reason.
				if strings.Contains(half.text, `<input id="ssid"`) &&
					!strings.Contains(half.text, goldenSSID) {
					t.Errorf("%s: the state has a saved hotspot named %q, %s renders the hotspot "+
						"form, and the name is not in it, so something redacted a value that is "+
						"broadcast anyway", name, goldenSSID, half.what)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no pages were checked, so this guard passed vacuously")
	}
}

// TestGolden_EveryGoldenFileIsProducedBySomeCase fails when testdata holds a
// file no case in this package generates.
//
// That is how a stale golden survives a rename and quietly stops being checked
// by anything, while still reading in a diff as though it were.
func TestGolden_EveryGoldenFileIsProducedBySomeCase(t *testing.T) {
	want := map[string]bool{
		"secret-exposure.txt": true,
		"wifi-join.txt":       true,
		"status-shape.txt":    true,
		"message-keys.txt":    true,
		"routes.txt":          true,
		"PROVENANCE.md":       true,
	}
	for _, s := range goldenStates() {
		for _, lang := range goldenLangs {
			want[fmt.Sprintf("page-%s-%s.html", s.name, lang)] = true
			if s.statusJSON {
				want[fmt.Sprintf("status-%s-%s.json", s.name, lang)] = true
			}
		}
	}
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	seen := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		seen++
		if !want[e.Name()] {
			t.Errorf("testdata/%s is produced by no case in golden_test.go, so it is stale "+
				"and nothing checks it any more", e.Name())
		}
		delete(want, e.Name())
	}
	for name := range want {
		t.Errorf("golden_test.go names testdata/%s, which does not exist. Run: bash scripts/golden-update.sh", name)
	}
	if seen == 0 {
		t.Fatal("testdata is empty, so this guard passed vacuously")
	}
}

// TestGolden_ProvenanceDocumentsEveryGolden holds the same discipline as
// internal/netcfg's TestProvenance_DocumentsEveryFixture.
//
// A golden nobody can trace is a golden whose class nobody can tell, so it
// silently becomes evidence it is not. Every file has to be named in
// PROVENANCE.md, including the two that pin a known defect, whose entries say
// which defect and what the diff will mean when it is fixed.
func TestGolden_ProvenanceDocumentsEveryGolden(t *testing.T) {
	body, err := os.ReadFile(goldenPath("PROVENANCE.md"))
	if err != nil {
		t.Fatalf("reading PROVENANCE.md: %v", err)
	}
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || e.Name() == "PROVENANCE.md" {
			continue
		}
		checked++
		if !strings.Contains(string(body), e.Name()) {
			t.Errorf("testdata/%s is not mentioned in PROVENANCE.md, so nothing records what it "+
				"is a picture of or what a diff in it means", e.Name())
		}
	}
	if checked == 0 {
		t.Fatal("no goldens found to check")
	}
}

// TestGolden_NoGoldenCarriesASentinel is the local half of the repository-wide
// scan in test/goldenscan.
//
// It runs here as well as there because a failure in this package should name
// this package. The passphrase is exempt only where the redactor put its own
// placeholder, which is why the check is for the raw value and not for the
// word.
func TestGolden_NoGoldenCarriesASentinel(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := os.ReadFile(goldenPath(e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		checked++
		for _, sec := range goldenSecrets() {
			if strings.Contains(string(body), sec.value) {
				// Never print the value.
				t.Errorf("testdata/%s carries the %s in the clear", e.Name(), sec.name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no goldens found to scan")
	}
}
