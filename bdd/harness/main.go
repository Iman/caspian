// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// This file is part of Caspian-BYOC.

// Command caspian-bdd-harness serves the real panel, against fakes, so that the
// Cucumber suites under bdd/web and bdd/api have something to drive.
//
// # This is a test fixture and it is not the appliance
//
// It exists because the BDD suites are written in JavaScript and the panel is
// written in Go, so the two need a socket between them. Everything above that
// socket is the shipped code: the real Panel, the real templates, the real
// stylesheet, the real script, the real QR encoder and the real message
// catalogue, all loaded through the same embedded filesystem the appliance
// uses. What is faked is the machine underneath, through the FakePrivileged
// that internal/panel already ships for its own tests, and the state store,
// which is a real store over a temporary directory.
//
// So: no Raspberry Pi, no root, no radio, no /dev/net/tun and no network beyond
// a loopback listener.
//
// # The control API, and why it is safe that it exists
//
// The suites need to put the box into a state and to inject a fault. They do
// that through /__control/..., which is mounted BY THIS COMMAND and by nothing
// else. internal/panel has no knowledge of it: Panel.ServeHTTP never sees a
// request whose path begins with /__control/, because this file answers those
// before delegating. Nothing in the appliance links this package, and
// TestNothingInTheApplianceWatchesTheUplink's walk over the repository will
// find no forbidden call here.
//
// # The defects, and why a test fixture injects them
//
// A scenario nobody has watched fail is not evidence. test/bdd makes that point
// in behaviour_test.go and enforces it with TestEveryScenarioCanFail, which
// runs every scenario a second time with a named fault injected and requires
// red. bdd/mutation.sh does the same job for the Cucumber suites, and this is
// where the faults are applied.
//
// A defect changes WHAT THE BROWSER RECEIVES. Some are applied to the machine
// underneath (an interface list that comes back empty), some to the request on
// its way in (a language forced, a password swapped) and some to the bytes on
// their way out (a stylesheet rule appended, an attribute removed). All three
// are legitimate, because what a scenario asserts is what the browser shows,
// and those are the three ways it can go wrong.
//
// One defect deserves naming. DefectHeroGroundOverridden appends
//
//	.hero { background: var(--ground); }
//
// to the served stylesheet. That is not invented. `git show 5c51497` removes
// exactly that declaration from the .hero rule, and its message records that
// with it present "Connected, starting, switched off and cut all drew the page
// ground, while the comment above the state rules explained in prose that they
// did not. Nothing failed, because nothing checked." This is the thing that
// checks.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/state"
)

// The credentials the suites use. Every value here is invented and none of them
// is, or has ever been, a working credential. Same discipline as
// internal/panel/fixtures_test.go, which explains why they are built out of
// their parts rather than pasted as one string.
const (
	PanelPassword     = "correct-horse-battery"
	HotspotSSID       = "Caspian-test"
	HotspotPassphrase = "sun-rope-glass-mint"
)

// testLink is a REALITY vless link carrying every parameter the panel displays.
// It exists so the appliance has a config and the power control has something
// to start.
func testLink() string {
	const (
		uuid    = "11111111-2222-4333-8444-555555555555"
		shortID = "0a1b2c3d4e"
		host    = "example.invalid"
		sni     = "www.fake-front.invalid"
	)
	// 43 characters of base64url decoding to exactly 32 bytes, which is what
	// the engine requires of a REALITY public key.
	pbk := "Q0FTUElBTi1GQUtFLVJFQUxJVFktUFVCS0VZLTMyMzI"
	return "vless://" + uuid + "@" + host + ":443" +
		"?security=reality&type=raw&flow=xtls-rprx-vision" +
		"&sni=" + sni + "&fp=chrome&pbk=" + pbk + "&sid=" + shortID +
		"&spx=%2Fspider#Living%20room%20box"
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "address to listen on")
	flag.Parse()

	h := &harness{}
	if err := h.reset(""); err != nil {
		log.Fatalf("harness: %v", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("harness: listen: %v", err)
	}

	// One line of JSON on stdout, so the Cucumber support code can read the
	// address rather than guess a port. A guessed port is the kind of thing
	// that works on a developer machine and collides in CI.
	out, _ := json.Marshal(map[string]string{
		"url":      "http://" + ln.Addr().String(),
		"password": PanelPassword,
		"ssid":     HotspotSSID,
	})
	fmt.Println(string(out))
	_ = os.Stdout.Sync()

	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.Serve(ln); err != nil {
		log.Fatalf("harness: serve: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The harness
// ---------------------------------------------------------------------------

// harness holds one appliance at a time. A scenario resets it, which throws the
// old store, the old sessions and the old fake away and builds new ones, so no
// scenario can see anything the one before it did.
type harness struct {
	mu   sync.RWMutex
	cur  *appliance
	dirs []string
}

type appliance struct {
	store  *state.Store
	priv   *panel.FakePrivileged
	panel  *panel.Panel
	defect defect
}

func (h *harness) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/__control/") {
		h.control(w, r)
		return
	}
	h.mu.RLock()
	app := h.cur
	h.mu.RUnlock()
	faulty{inner: app.panel, d: app.defect}.ServeHTTP(w, r)
}

// reset builds a fresh appliance: set up, with a hotspot and a config, switched
// off.
func (h *harness) reset(defectName string) error {
	d, ok := defectsByName[defectName]
	if !ok {
		return fmt.Errorf("no defect called %q", defectName)
	}

	// internal/state refuses a state directory any other user on the box could
	// read and enforces that on every load, so the temporary directory has to
	// be tightened before Load will look at it.
	dir, err := os.MkdirTemp("", "caspian-bdd-")
	if err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	store, err := state.Load(dir)
	if err != nil {
		return fmt.Errorf("state.Load: %w", err)
	}
	if err := store.SetPanelPassword(PanelPassword); err != nil {
		return fmt.Errorf("SetPanelPassword: %w", err)
	}
	if err := store.SetHotspot(HotspotSSID, HotspotPassphrase); err != nil {
		return fmt.Errorf("SetHotspot: %w", err)
	}
	if err := store.SetProxyConfig(testLink(), "vless", "Home"); err != nil {
		return fmt.Errorf("SetProxyConfig: %w", err)
	}

	priv := panel.NewFakePrivileged()
	if d.noInterfacesReported {
		det, derr := priv.Detect(context.Background())
		if derr != nil {
			return derr
		}
		det.Interfaces = nil
		priv.SetDetection(det)
	}

	// The panel talks to this, which is the fake unless a defect wraps it.
	// deviceCountGated is applied HERE rather than by rewriting the rendered
	// HTML, because this is the seam a real fix would use, and because a
	// rewrite would have to reproduce the message in the right language and
	// would then be testing the rewriter.
	var served panel.Privileged = priv
	if d.deviceCountGated {
		served = gatedDeviceCount{priv}
	}

	p, err := panel.New(panel.Config{
		Store:  store,
		Priv:   served,
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		return fmt.Errorf("panel.New: %w", err)
	}

	h.mu.Lock()
	h.cur = &appliance{store: store, priv: priv, panel: p, defect: d}
	h.dirs = append(h.dirs, dir)
	h.mu.Unlock()
	return nil
}

// controlState is the body of POST /__control/state. Every field is a pointer,
// so a scenario changes one thing without having to restate the rest.
type controlState struct {
	Engine  *string `json:"engine"`
	Hotspot *bool   `json:"hotspot"`
	Devices *int    `json:"devices"`
	Cut     *bool   `json:"cut"`
	SSID    *string `json:"ssid"`
}

func (h *harness) control(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/__control/health":
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case "/__control/reset":
		var body struct {
			Defect string `json:"defect"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if err := h.reset(body.Defect); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "defect": body.Defect})

	case "/__control/messages":
		// The suites assert on the words the panel actually ships, resolved
		// from the Go catalogue, rather than on Persian and English strings
		// copied into JavaScript. A copy would be a second catalogue that
		// nothing keeps in step, and the first reworded message would turn
		// every affected scenario red for no reason.
		lang := panel.Lang(r.URL.Query().Get("lang"))
		if lang != panel.LangEN {
			lang = panel.LangFA
		}
		out := map[string]string{}
		for _, k := range exportedKeys {
			out[string(k)] = panel.T(lang, k)
		}
		writeJSON(w, http.StatusOK, out)

	case "/__control/state":
		var body controlState
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if err := h.applyState(body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no such control"})
	}
}

// applyState puts the fake machine into the state a scenario asked for.
//
// It reads the CURRENT status first and writes back a modified copy, so that a
// scenario that sets only the device count does not silently switch the engine
// off as a side effect.
func (h *harness) applyState(c controlState) error {
	h.mu.RLock()
	app := h.cur
	h.mu.RUnlock()

	st, err := app.priv.Status(context.Background())
	if err != nil {
		return err
	}
	eng := st.Engine
	hs := st.Hotspot

	if c.Engine != nil {
		phase, perr := phaseNamed(*c.Engine)
		if perr != nil {
			return perr
		}
		eng = engine.State{Phase: phase, Since: time.Now().Add(-3 * time.Minute)}
	}
	if c.Hotspot != nil {
		hs.Running = *c.Hotspot
	}
	if c.Devices != nil {
		hs.Devices = *c.Devices
	}
	if c.SSID != nil {
		hs.SSID = *c.SSID
	}

	app.priv.SetEngineState(eng)
	app.priv.SetHotspot(hs)

	if c.Cut != nil {
		// Cut and Restore are refused while the box is not running, which is
		// the privileged side's own rule and not something to work around
		// here: a scenario that wants the cut state switches the box on first.
		if *c.Cut {
			if err := app.priv.Cut(context.Background()); err != nil {
				return fmt.Errorf("cut: %w", err)
			}
		} else if err := app.priv.Restore(context.Background()); err != nil {
			return fmt.Errorf("restore: %w", err)
		}
	}
	return nil
}

func phaseNamed(s string) (engine.Phase, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stopped":
		return engine.PhaseStopped, nil
	case "starting":
		return engine.PhaseStarting, nil
	case "running":
		return engine.PhaseRunning, nil
	case "failed":
		return engine.PhaseFailed, nil
	default:
		return engine.PhaseStopped, fmt.Errorf("no engine phase called %q", s)
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// ---------------------------------------------------------------------------
// The defects
// ---------------------------------------------------------------------------

type defect struct {
	anyPasswordAccepted   bool
	everyPasswordRejected bool
	alwaysEnglish         bool
	languageChoiceIgnored bool
	heroGroundOverridden  bool
	cutStateFlattened     bool
	quietZonePaintedDark  bool
	powerLabelFrozen      bool
	cutRoleRemoved        bool
	cutStateFrozen        bool
	helpPageBroken        bool
	noInterfacesReported  bool
	skipLinkRemoved       bool
	labelsUnhooked        bool
	deviceCountGated      bool
	statusJSONFieldLost   bool
	advancedSaveIgnored   bool
	csrfCheckDisabled     bool
	sessionGateOpen       bool
	secretsEchoed         bool
}

// defectsByName is the registry. bdd/mutation.sh names one of these per
// scenario tag, and the empty name is the healthy appliance.
var defectsByName = map[string]defect{
	"": {},

	"any-password-accepted":   {anyPasswordAccepted: true},
	"every-password-rejected": {everyPasswordRejected: true},
	"always-english":          {alwaysEnglish: true},
	"language-choice-ignored": {languageChoiceIgnored: true},
	"hero-ground-overridden":  {heroGroundOverridden: true},
	"cut-state-flattened":     {cutStateFlattened: true},
	"quiet-zone-painted-dark": {quietZonePaintedDark: true},
	"power-label-frozen":      {powerLabelFrozen: true},
	"cut-role-removed":        {cutRoleRemoved: true},
	"cut-state-frozen":        {cutStateFrozen: true},
	"help-page-broken":        {helpPageBroken: true},
	"no-interfaces-reported":  {noInterfacesReported: true},
	"skip-link-removed":       {skipLinkRemoved: true},
	"labels-unhooked":         {labelsUnhooked: true},
	// This one is not like the others, and the difference matters.
	//
	// Every other entry here BREAKS something, so that a scenario which passes
	// on the real build can be watched going red. The device count scenario is
	// already red on the real build, because it pins an open defect, so
	// breaking something more would prove nothing.
	//
	// This entry does the opposite: it forces the count to zero, which in the
	// state that scenario drives is what a corrected panel would report. So the
	// scenario is shown to go GREEN when the panel tells the truth, which is
	// the evidence that it is a real assertion and not a scenario that can
	// never pass.
	//
	// It is a crude stand-in for the fix and not the fix. It zeroes the count
	// unconditionally, and a real fix would gate it on the hotspot running, so
	// the two agree only in the state the scenario puts the box into.
	"device-count-gated": {deviceCountGated: true},

	"status-json-field-lost": {statusJSONFieldLost: true},
	"advanced-save-ignored":  {advancedSaveIgnored: true},
	"csrf-check-disabled":    {csrfCheckDisabled: true},
	"session-gate-open":      {sessionGateOpen: true},
	"secrets-echoed":         {secretsEchoed: true},
}

// gatedDeviceCount is the device-count-gated defect: a privileged service that
// reports no joined devices while the hotspot is not being broadcast.
//
// It is a crude stand-in for a fix and not the fix. A real one has to decide
// what the dashboard should SAY in that state, and "0 devices" and "the hotspot
// is off" are different answers. What this is for is proving that the scenario
// which pins the open defect goes green when the count tells the truth, and is
// therefore a real assertion rather than one that can never pass.
type gatedDeviceCount struct{ panel.Privileged }

func (g gatedDeviceCount) Status(ctx context.Context) (panel.SystemStatus, error) {
	st, err := g.Privileged.Status(ctx)
	if err != nil {
		return st, err
	}
	if !st.Hotspot.Running {
		st.Hotspot.Devices = 0
	}
	return st, nil
}

// faulty wraps the panel and applies the request-side and response-side parts
// of a defect. With the zero defect it is a pass-through.
type faulty struct {
	inner http.Handler
	d     defect
}

func (f faulty) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.d.helpPageBroken && r.URL.Path == "/help" {
		// What an unregistered page actually did before "help" was added to
		// pageNames in internal/panel/assets.go: the render failed and the
		// server answered 500.
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	if f.d.advancedSaveIgnored && r.Method == http.MethodPost && r.URL.Path == "/advanced" {
		// Accepted, redirected, and nothing written. This is the shape of a
		// save that reports success and loses the setting.
		http.Redirect(w, r, "/?advanced=1", http.StatusSeeOther)
		return
	}
	if f.d.csrfCheckDisabled && r.Method == http.MethodPost {
		// A panel with no cross-site check answers a tokenless POST the way it
		// answers a good one: it does the thing and redirects home. The token
		// cannot be forged from out here, because it belongs to a session this
		// wrapper has no access to, so the ANSWER is forged instead. That is
		// enough for what this defect is for, which is proving that the
		// scenario asserting a 403 can actually go red.
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if f.d.sessionGateOpen && r.URL.Path == "/status.json" {
		// A panel with no session gate on the polled document answers it to
		// anybody who asks. The body is a plausible one rather than an empty
		// object, so the scenario fails on the status code it asserts and not
		// on a parse error somewhere further down.
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": false, "running": false, "heroClass": "off", "devices": 0,
		})
		return
	}

	f.mutateRequest(r)

	rec := &recorder{header: http.Header{}, status: http.StatusOK}
	f.inner.ServeHTTP(rec, r)

	body := rec.body.Bytes()
	if f.d.needsBodyRewrite() {
		ct := rec.header.Get("Content-Type")
		switch {
		case strings.Contains(ct, "text/html"):
			body = []byte(f.mutateHTML(string(body)))
		case strings.Contains(ct, "text/css"):
			body = []byte(f.mutateCSS(string(body)))
		case strings.Contains(ct, "application/json"):
			body = []byte(f.mutateJSON(string(body)))
		}
	}

	for k, vs := range rec.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	// Set after copying, because the rewrite changed the length and a stale
	// Content-Length makes the browser truncate the page, which would fail
	// every scenario for a reason that is not the defect.
	if rec.header.Get("Content-Length") != "" {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.WriteHeader(rec.status)
	_, _ = w.Write(body)
}

func (d defect) needsBodyRewrite() bool {
	return d.heroGroundOverridden || d.cutStateFlattened || d.quietZonePaintedDark ||
		d.powerLabelFrozen || d.cutRoleRemoved || d.cutStateFrozen ||
		d.skipLinkRemoved || d.labelsUnhooked ||
		d.statusJSONFieldLost || d.secretsEchoed
}

func (f faulty) mutateRequest(r *http.Request) {
	if f.d.alwaysEnglish {
		q := r.URL.Query()
		q.Set("lang", "en")
		r.URL.RawQuery = q.Encode()
	}
	if f.d.languageChoiceIgnored {
		q := r.URL.Query()
		q.Del("lang")
		r.URL.RawQuery = q.Encode()
	}
	if !f.d.anyPasswordAccepted && !f.d.everyPasswordRejected {
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/login" {
		return
	}
	// The body is read, rewritten and put back, so the panel's own form parsing
	// runs on it exactly as it would on a real submission.
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return
	}
	_ = r.Body.Close()
	form, err := url.ParseQuery(buf.String())
	if err != nil {
		return
	}
	if f.d.anyPasswordAccepted {
		form.Set("password", PanelPassword)
	} else {
		form.Set("password", PanelPassword+"-not")
	}
	encoded := form.Encode()
	r.Body = noCloseReader{strings.NewReader(encoded)}
	r.ContentLength = int64(len(encoded))
}

func (f faulty) mutateHTML(s string) string {
	if f.d.powerLabelFrozen {
		// The ENGLISH word, in every state and every language. The defect it
		// models is a real one: the script carried its own copy of the two
		// words and relabelled a Persian page in English five seconds after it
		// loaded. See the note on PowerLabel in internal/panel/view.go.
		s = replaceSpanText(s, "power-label", panel.T(panel.LangEN, panel.MsgSwitchOn))
	}
	if f.d.cutRoleRemoved {
		s = strings.ReplaceAll(s, `role="switch"`, `role="button"`)
	}
	if f.d.cutStateFrozen {
		s = strings.ReplaceAll(s, `aria-checked="false"`, `aria-checked="true"`)
	}
	if f.d.skipLinkRemoved {
		if i := strings.Index(s, `<a class="skip"`); i >= 0 {
			if j := strings.Index(s[i:], "</a>"); j >= 0 {
				s = s[:i] + s[i+j+len("</a>"):]
			}
		}
	}
	if f.d.labelsUnhooked {
		// The label element stays, and stays visible, and stops being a label
		// for anything. This is the version of the fault that survives review.
		s = strings.ReplaceAll(s, `<label for="`, `<label data-for="`)
	}
	return s
}

func (f faulty) mutateCSS(s string) string {
	if f.d.heroGroundOverridden {
		s += "\n.hero { background: var(--ground); }\n"
	}
	if f.d.cutStateFlattened {
		s = strings.ReplaceAll(s, "animation: cut-pulse 2.4s ease-in-out infinite;", "")
		s += "\n.hero-cut { background: var(--ground); }\n"
	}
	if f.d.quietZonePaintedDark {
		s = strings.ReplaceAll(s, ".qr-bg { fill: #FFFFFF; }", ".qr-bg { fill: currentColor; }")
	}
	return s
}

func (f faulty) mutateJSON(s string) string {
	if f.d.statusJSONFieldLost {
		s = strings.ReplaceAll(s, `"heroClass"`, `"heroKlass"`)
	}
	if f.d.secretsEchoed {
		// A credential in the polled document. This is the shape of the leak
		// the panel's rule exists to stop: not a deliberate echo, but a field
		// added for debugging that carries more than it meant to.
		s = strings.TrimSuffix(strings.TrimSpace(s), "}") +
			`,"debugPanelPassword":"` + PanelPassword + `"}`
	}
	return s
}

// replaceSpanText replaces the text inside <span id="<id>"> ... </span>.
func replaceSpanText(s, id, text string) string { return replaceTagText(s, "span", id, text) }

func replaceTagText(s, tag, id, text string) string {
	// The id may carry other attributes before or after it, so the opening tag
	// is found by its id and then closed at the first ">".
	needle := ` id="` + id + `"`
	i := strings.Index(s, needle)
	if i < 0 {
		return s
	}
	open := strings.Index(s[i:], ">")
	if open < 0 {
		return s
	}
	start := i + open + 1
	end := strings.Index(s[start:], "</"+tag+">")
	if end < 0 {
		return s
	}
	return s[:start] + text + s[start+end:]
}

// recorder buffers a response so the body can be rewritten before it goes out.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
	wrote  bool
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if r.wrote {
		return
	}
	r.status = code
	r.wrote = true
}

func (r *recorder) Write(p []byte) (int, error) {
	r.wrote = true
	return r.body.Write(p)
}

type noCloseReader struct{ *strings.Reader }

func (noCloseReader) Close() error { return nil }

// exportedKeys are the catalogue entries the Cucumber suites assert on. Adding
// a scenario that needs a new word means adding its key here, which is a
// deliberate speed bump: it keeps the list of words the suites depend on
// visible in one place rather than scattered through JavaScript string
// literals.
var exportedKeys = []panel.Key{
	panel.MsgAppName,
	panel.MsgSkipToMain,
	panel.MsgLoginHeading,
	panel.MsgLoginPassword,
	panel.MsgLoginWrong,
	panel.MsgSwitchOn,
	panel.MsgSwitchOff,
	panel.MsgCutButton,
	panel.MsgCutRestoreButton,
	panel.MsgCutSwitchLabel,
	panel.MsgStatusConnected,
	panel.MsgStatusOff,
	panel.MsgStatusCut,
	panel.MsgDevicesNone,
	panel.MsgDevicesOne,
	panel.MsgAdvancedHeading,
	panel.MsgAdvInternet,
	panel.MsgHelpHeading,
	panel.MsgHelpTitle,
	panel.MsgWifiQRCaption,
	panel.MsgBadForm,
	panel.MsgAdvBadInternet,
}
