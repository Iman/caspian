// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/state"
)

// ---------------------------------------------------------------------------
// The harness
//
// Everything here goes through a real httptest server, a real net/http client
// and a real net/http/cookiejar, and the CSRF token is scraped out of the
// rendered HTML.
//
// That is a deliberate choice about what these tests are allowed to know. A
// test that built its own cookie with http.Request.AddCookie, using a token it
// had reached into the session store for, would be checking that this package's
// reader agrees with this package's writer: the two would agree by
// construction, and a cookie no browser would ever send back would pass. The
// cookie jar is an independent implementation of RFC 6265, so a cookie it will
// not store or will not return is a cookie a browser will not either. The same
// reasoning applies to the token: it is read from the page a browser would
// parse, not from the struct the handler wrote it into.
// ---------------------------------------------------------------------------

const testPassword = "correct-horse-battery"

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock() *testClock {
	return &testClock{t: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// syncBuffer is a log sink that can be read while handlers are writing to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type harness struct {
	t      *testing.T
	lang   Lang
	panel  *Panel
	store  *state.Store
	priv   *FakePrivileged
	logs   *syncBuffer
	srv    *httptest.Server
	client *http.Client
	clock  *testClock
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	// internal/state refuses a state directory any other user on the box could
	// read, and it enforces that on every load rather than trusting whatever
	// Save last set (internal/state/perm_unix.go). testing.T.TempDir hands back
	// a 0755 directory, so it has to be tightened before Load will look at it.
	// Measured 2026-08-30: without this chmod, Load returns "has permissions
	// 0755, which lets other users on this box read the proxy config and
	// hotspot passphrase it holds".
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("tightening the temporary state directory: %v", err)
	}
	store, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	logs := &syncBuffer{}
	clock := newTestClock()
	priv := NewFakePrivileged()

	p, err := New(Config{
		Store: store,
		Priv:  priv,
		// Debug level, so that if a credential were being logged at any level
		// the secrets test would see it.
		Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Now:    clock.Now,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return &harness{
		t: t,
		// Persian is the product default, so the harness runs in Persian
		// unless a test asks otherwise. Assertions go through h.msg, so they
		// hold in whichever language the harness is in rather than pinning the
		// English wording.
		lang:  DefaultLang,
		panel: p,
		store: store,
		priv:  priv,
		logs:  logs,
		srv:   srv,
		clock: clock,
		client: &http.Client{
			Jar: jar,
			// Redirects are followed by hand, so that a test can assert the
			// status code a handler actually returned.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// msg is the message a key resolves to in the harness's language.
func (h *harness) msg(k Key) string { return T(h.lang, k) }

// useEnglish switches this harness to English the way a browser does, by
// asking for it and keeping the cookie.
func (h *harness) useEnglish() {
	h.t.Helper()
	h.get("/?lang=en")
	h.lang = LangEN
}

func (h *harness) do(req *http.Request) (*http.Response, string) {
	h.t.Helper()
	res, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		h.t.Fatalf("reading body of %s %s: %v", req.Method, req.URL.Path, err)
	}
	return res, string(body)
}

func (h *harness) get(path string) (*http.Response, string) {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.srv.URL+path, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	return h.do(req)
}

func (h *harness) postForm(path string, form url.Values) (*http.Response, string) {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return h.do(req)
}

var csrfRE = regexp.MustCompile(`name="csrf" value="([^"]*)"`)

// tokenOn scrapes the token out of a rendered page, the way a browser gets it.
func (h *harness) tokenOn(path string) string {
	h.t.Helper()
	_, body := h.get(path)
	m := csrfRE.FindStringSubmatch(body)
	if m == nil {
		h.t.Fatalf("no csrf token in the page at %s", path)
	}
	if m[1] == "" {
		h.t.Fatalf("the csrf token on %s is empty", path)
	}
	return m[1]
}

// setup completes first run: choose a panel password and end up signed in.
func (h *harness) setup(password string) {
	h.t.Helper()
	token := h.tokenOn("/setup")
	res, _ := h.postForm("/setup", url.Values{
		"csrf":     {token},
		"password": {password},
		"confirm":  {password},
	})
	if res.StatusCode != http.StatusSeeOther {
		h.t.Fatalf("POST /setup: status %d, want %d", res.StatusCode, http.StatusSeeOther)
	}
	if !h.store.Snapshot().Panel.IsSet() {
		h.t.Fatal("POST /setup returned a redirect but no panel password was stored")
	}
}

// signIn signs in an already set-up panel.
func (h *harness) signIn(password string) *http.Response {
	h.t.Helper()
	token := h.tokenOn("/login")
	res, _ := h.postForm("/login", url.Values{"csrf": {token}, "password": {password}})
	return res
}

// signedOut drops the session cookies without touching the server, the way a
// browser would if it were closed.
func (h *harness) signedOut() {
	h.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		h.t.Fatal(err)
	}
	h.client.Jar = jar
}

// ready is a panel that is set up, signed in, and has a hotspot and a config.
func (h *harness) ready() {
	h.t.Helper()
	h.setup(testPassword)
	if err := h.store.SetHotspot("Caspian-test", "sun-rope-glass-mint"); err != nil {
		h.t.Fatalf("SetHotspot: %v", err)
	}
	if err := h.store.SetProxyConfig(testLink(), "vless", "Home"); err != nil {
		h.t.Fatalf("SetProxyConfig: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// TestEveryRouteRefusesWithoutASession walks the route table rather than a list
// written here, so a route added later is covered without anybody remembering
// to add it.
//
// The positive control at the end is the point of the test as much as the loop
// is. "Every route refused me" is also what a broken client, a wrong base URL
// or a server that never started would produce, and that failure mode reports
// success. So the same client, on the same routes, must reach them once it has
// signed in.
func TestEveryRouteRefusesWithoutASession(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	h.signedOut()

	gated := 0
	for _, rt := range routes {
		if rt.Public {
			continue
		}
		gated++
		t.Run(rt.Method+" "+rt.Path, func(t *testing.T) {
			var res *http.Response
			if rt.Method == http.MethodGet {
				res, _ = h.get(rt.Path)
			} else {
				// A plausible, well-formed request. If the gate let it
				// through, it would do something.
				res, _ = h.postForm(rt.Path, url.Values{"on": {"1"}, "config": {testLink()}})
			}
			switch {
			case rt.JSON:
				if res.StatusCode != http.StatusUnauthorized {
					t.Errorf("status %d, want %d for a JSON route with no session", res.StatusCode, http.StatusUnauthorized)
				}
			default:
				if res.StatusCode != http.StatusSeeOther {
					t.Errorf("status %d, want %d", res.StatusCode, http.StatusSeeOther)
				}
				if got := res.Header.Get("Location"); got != "/login" {
					t.Errorf("redirected to %q, want /login", got)
				}
			}
		})
	}
	if gated == 0 {
		t.Fatal("the route table has no gated routes, so this test checked nothing")
	}

	// Nothing was allowed to happen along the way.
	if n := len(h.priv.Starts()); n != 0 {
		t.Errorf("the privileged service was asked to start %d times by unauthenticated requests", n)
	}
	if h.priv.Stops() != 0 {
		t.Errorf("the privileged service was asked to stop by an unauthenticated request")
	}
	if h.store.Proxy().IsConfigured() {
		t.Error("an unauthenticated request stored a config")
	}

	// The positive control.
	h.signIn(testPassword)
	reached := 0
	for _, rt := range routes {
		if rt.Public || rt.Method != http.MethodGet {
			continue
		}
		res, _ := h.get(rt.Path)
		if res.StatusCode != http.StatusOK {
			t.Errorf("signed in, GET %s returned %d, want 200; the refusals above may be an artefact of the client rather than the gate",
				rt.Path, res.StatusCode)
			continue
		}
		reached++
	}
	if reached == 0 {
		t.Fatal("signing in reached no route, so the refusals above prove nothing")
	}
}

// TestThePublicRoutesAreTheExpectedSeven pins the set of routes reachable
// without a session, so that making one public is a deliberate act with a test
// to change rather than a line in a table nobody re-reads.
func TestThePublicRoutesAreTheExpectedSeven(t *testing.T) {
	want := map[string]bool{
		"GET /login":            true,
		"POST /login":           true,
		"GET /setup":            true,
		"POST /setup":           true,
		"GET /assets/panel.css": true,
		"GET /assets/panel.js":  true,
		"GET /favicon.svg":      true,
	}
	got := map[string]bool{}
	for _, rt := range routes {
		if rt.Public {
			got[rt.Method+" "+rt.Path] = true
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("%s is no longer public, and the two forms plus their assets have to work before anyone can log in", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("%s has been made reachable without a session; that needs a reason and a test, not a table entry", k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%d public routes, expected %d", len(got), len(want))
	}
}

// TestFirstRunShowsSetupNotLogin is the requirement that a box with no password
// shows setup. A login form there is a dead end: there is no password to type.
func TestFirstRunShowsSetupNotLogin(t *testing.T) {
	h := newHarness(t)

	if !h.store.FirstRun() {
		t.Fatal("a fresh state directory should report FirstRun")
	}
	if !h.store.NeedsSetup() {
		t.Fatal("a fresh state directory should report NeedsSetup")
	}

	// The main screen sends an unconfigured box to setup, not to login.
	res, _ := h.get("/")
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET / on a fresh box: status %d, want %d", res.StatusCode, http.StatusSeeOther)
	}
	if got := res.Header.Get("Location"); got != "/setup" {
		t.Errorf("GET / on a fresh box redirected to %q, want /setup", got)
	}

	// So does the login page itself, so a bookmark cannot strand the user.
	res, _ = h.get("/login")
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/setup" {
		t.Errorf("GET /login on a fresh box: status %d to %q, want 303 to /setup",
			res.StatusCode, res.Header.Get("Location"))
	}

	res, body := h.get("/setup")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /setup: status %d, want 200", res.StatusCode)
	}
	if !strings.Contains(body, h.msg(MsgSetupHeading)) {
		t.Error("the setup page does not look like a setup page")
	}

	// And once a password exists, setup closes and login opens.
	h.setup(testPassword)
	res, _ = h.get("/setup")
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login" {
		t.Errorf("GET /setup after setup: status %d to %q, want 303 to /login",
			res.StatusCode, res.Header.Get("Location"))
	}
}

// TestSetupCannotBeUsedToTakeOverAConfiguredBox is the attack the setup route
// would otherwise be: anything that can reach the panel posting a password of
// its choice to a box that already has one.
func TestSetupCannotBeUsedToTakeOverAConfiguredBox(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	before := h.store.Snapshot().Panel.PasswordHash

	h.signedOut()
	token := h.tokenOn("/login") // any token this client legitimately holds
	res, _ := h.postForm("/setup", url.Values{
		"csrf":     {token},
		"password": {"attacker-chosen-password"},
		"confirm":  {"attacker-chosen-password"},
	})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("POST /setup on a configured box: status %d, want %d", res.StatusCode, http.StatusForbidden)
	}
	if after := h.store.Snapshot().Panel.PasswordHash; after != before {
		t.Fatal("POST /setup changed the panel password on a box that already had one")
	}
	if ok, err := h.store.VerifyPanelPassword(testPassword); err != nil || !ok {
		t.Fatal("the original panel password no longer verifies")
	}
}

func TestLoginSucceedsAndFails(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	h.signedOut()

	t.Run("wrong password is refused", func(t *testing.T) {
		token := h.tokenOn("/login")
		res, body := h.postForm("/login", url.Values{"csrf": {token}, "password": {"not the password"}})
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("status %d, want %d", res.StatusCode, http.StatusUnauthorized)
		}
		if !strings.Contains(body, h.msg(MsgLoginWrong)) {
			t.Error("the page does not say the password was wrong")
		}
		// No session was handed out.
		res, _ = h.get("/")
		if res.StatusCode != http.StatusSeeOther {
			t.Errorf("after a refused login, GET / returned %d; a session was created anyway", res.StatusCode)
		}
	})

	t.Run("empty password is refused", func(t *testing.T) {
		token := h.tokenOn("/login")
		res, _ := h.postForm("/login", url.Values{"csrf": {token}, "password": {""}})
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("status %d, want %d", res.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("right password signs in", func(t *testing.T) {
		res := h.signIn(testPassword)
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("status %d, want %d", res.StatusCode, http.StatusSeeOther)
		}
		if got := res.Header.Get("Location"); got != "/" {
			t.Errorf("redirected to %q, want /", got)
		}
		res, body := h.get("/")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("after signing in, GET / returned %d", res.StatusCode)
		}
		if !strings.Contains(body, h.msg(MsgWifiHeading)) {
			t.Error("the main screen did not render")
		}
	})

	t.Run("signing out ends the session", func(t *testing.T) {
		token := h.tokenOn("/")
		res, _ := h.postForm("/logout", url.Values{"csrf": {token}})
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /logout: status %d", res.StatusCode)
		}
		res, _ = h.get("/")
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login" {
			t.Errorf("after signing out, GET / returned %d to %q, want 303 to /login",
				res.StatusCode, res.Header.Get("Location"))
		}
	})
}

// TestSessionCookieAttributes reads the Set-Cookie header as text rather than
// asking this package what it thinks it set.
func TestSessionCookieAttributes(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	h.signedOut()
	res := h.signIn(testPassword)

	var raw string
	for _, v := range res.Header.Values("Set-Cookie") {
		if strings.HasPrefix(v, sessionCookie+"=") {
			raw = v
		}
	}
	if raw == "" {
		t.Fatalf("signing in set no %s cookie; headers were %q", sessionCookie, res.Header.Values("Set-Cookie"))
	}
	for _, want := range []string{"HttpOnly", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(raw, want) {
			t.Errorf("the session cookie is missing %s: %q", want, raw)
		}
	}
	if strings.Contains(raw, "Expires=") || strings.Contains(raw, "Max-Age=") {
		t.Errorf("the session cookie is persistent, so it survives the browser closing: %q", raw)
	}
}

// TestRateLimitEngagesOnFailedPasswords is the requirement that failed attempts
// are limited.
//
// The control at the end matters: a limiter that refuses everything for ever
// would pass the first half of this test and lock the user out of their own
// box, which is a worse outcome than the one being defended against.
func TestRateLimitEngagesOnFailedPasswords(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	h.signedOut()

	limited := 0
	for i := 0; i < attemptBurst+3; i++ {
		token := h.tokenOn("/login")
		res, body := h.postForm("/login", url.Values{"csrf": {token}, "password": {"wrong"}})
		switch res.StatusCode {
		case http.StatusUnauthorized:
			// still being checked
		case http.StatusTooManyRequests:
			limited++
			if !strings.Contains(body, h.msg(MsgLoginTooMany)) {
				t.Error("the rate-limited page does not say so in plain words")
			}
			if res.Header.Get("Retry-After") == "" {
				t.Error("a rate-limited response has no Retry-After header")
			}
		default:
			t.Fatalf("attempt %d: unexpected status %d", i+1, res.StatusCode)
		}
	}
	if limited == 0 {
		t.Fatalf("no attempt was rate limited after %d wrong passwords", attemptBurst+3)
	}

	// While limited, even the right password is refused. That is the property
	// worth having: the limit is on attempts, not on failures, so an attacker
	// cannot use the box's Argon2id verification as a way to spend its memory.
	token := h.tokenOn("/login")
	res, _ := h.postForm("/login", url.Values{"csrf": {token}, "password": {testPassword}})
	if res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("while rate limited, the correct password returned %d, want %d",
			res.StatusCode, http.StatusTooManyRequests)
	}

	// The control: waiting clears it, and the box is not bricked.
	h.clock.Advance(attemptRefill * time.Duration(attemptBurst+1))
	if res := h.signIn(testPassword); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("after waiting, the correct password returned %d, want a redirect; the limiter never recovers",
			res.StatusCode)
	}
}

// TestFormsWithoutATokenAreRefused covers the second lock on cross-site posts.
func TestFormsWithoutATokenAreRefused(t *testing.T) {
	h := newHarness(t)
	h.ready()

	t.Run("no token", func(t *testing.T) {
		res, _ := h.postForm("/power", url.Values{"on": {"1"}})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("status %d, want %d", res.StatusCode, http.StatusForbidden)
		}
		if n := len(h.priv.Starts()); n != 0 {
			t.Errorf("a form with no token started the tunnel")
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		res, _ := h.postForm("/power", url.Values{"on": {"1"}, "csrf": {"not-the-token"}})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("status %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("cross-site origin", func(t *testing.T) {
		token := h.tokenOn("/")
		req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/power",
			strings.NewReader(url.Values{"on": {"1"}, "csrf": {token}}.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://evil.invalid")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		res, _ := h.do(req)
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("status %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	// The control: the same form with the right token, same-origin, works.
	token := h.tokenOn("/")
	res, _ := h.postForm("/power", url.Values{"on": {"1"}, "csrf": {token}})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("a well-formed power request returned %d, want a redirect", res.StatusCode)
	}
	if n := len(h.priv.Starts()); n != 1 {
		t.Fatalf("the privileged service was asked to start %d times, want 1", n)
	}
}

// ---------------------------------------------------------------------------
// The basic screen
// ---------------------------------------------------------------------------

func TestBasicScreenHasEverythingSectionFiveTwoLists(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test", Devices: 3})

	token := h.tokenOn("/")
	if res, _ := h.postForm("/power", url.Values{"on": {"1"}, "csrf": {token}}); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("switching on returned %d", res.StatusCode)
	}

	res, body := h.get("/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", res.StatusCode)
	}

	for _, want := range []struct{ what, needle string }{
		{"the switch", `action="/power"`},
		{"the status word", h.msg(MsgStatusConnected)},
		{"the hotspot name", "Caspian-test"},
		{"the hotspot password", "sun-rope-glass-mint"},
		{"a QR code", "<svg"},
		{"the device count", T(h.lang, MsgDevicesMany, 3)},
		{"the control to add a config", `action="/config"`},
		{"the detected line", DetectedLineIn(h.lang, h.priv.detection)},
		{"the advanced toggle", h.msg(MsgAdvancedShow)},
	} {
		if !strings.Contains(body, want.needle) {
			t.Errorf("the basic screen is missing %s (looked for %q)", want.what, want.needle)
		}
	}

	// The state must not be carried by colour alone: the word is present, and
	// so is a shape class that differs between states.
	if !strings.Contains(body, "dot-on") {
		t.Error("the connected state has no shape class, so it may be carried by colour alone")
	}
	if res, _ := h.postForm("/power", url.Values{"on": {"0"}, "csrf": {h.tokenOn("/")}}); res.StatusCode != http.StatusSeeOther {
		t.Fatal("switching off failed")
	}
	_, body = h.get("/")
	if !strings.Contains(body, "dot-off") {
		t.Error("the disconnected state has no shape class")
	}
	if strings.Contains(body, "dot-on") {
		t.Error("the page still shows the connected shape after switching off")
	}
}

func TestAdvancedModeRevealsAndNeverHides(t *testing.T) {
	h := newHarness(t)
	h.ready()

	_, basic := h.get("/")
	_, advanced := h.get("/?advanced=1")

	if !strings.Contains(advanced, h.msg(MsgAdvancedHeading)) {
		t.Fatal("advanced mode did not reveal the advanced section")
	}
	// Everything basic mode showed is still there.
	for _, needle := range []string{
		`action="/power"`, "Caspian-test", `action="/config"`,
		DetectedLineIn(h.lang, h.priv.detection),
	} {
		if !strings.Contains(advanced, needle) {
			t.Errorf("advanced mode hid %q, and it is supposed only to reveal", needle)
		}
	}
	if strings.Contains(basic, `name="engine_log_level"`) {
		t.Error("an advanced control is showing in basic mode")
	}

	// The fields design section 5.3 lists.
	for _, want := range []struct{ what, needle string }{
		{"which interface is which", `name="internet_interface"`},
		{"the hotspot interface", `name="hotspot_interface"`},
		{"channel", `name="channel"`},
		{"band", `name="band"`},
		{"country", `name="country"`},
		{"the hotspot subnet", `name="subnet"`},
		{"DNS behaviour", h.msg(MsgAdvDNSLabel)},
		{"what happens when the tunnel drops", h.msg(MsgAdvDropLabel)},
		{"the engine log", h.msg(MsgAdvLogHeading)},
		{"the parsed fields of the config", h.msg(MsgAdvConfigHeading)},
	} {
		if !strings.Contains(advanced, want.needle) {
			t.Errorf("advanced mode does not show %s (looked for %q)", want.what, want.needle)
		}
	}

	// And the toggle is remembered, so a form post does not lose it.
	_, again := h.get("/")
	if !strings.Contains(again, h.msg(MsgAdvancedHide)) {
		t.Error("the advanced choice was not remembered")
	}
}

func TestStatusJSONCarriesNoCredential(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test", Devices: 2})

	res, body := h.get("/status.json")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var got statusJSON
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("the status document is not JSON: %v", err)
	}
	if got.Devices != 2 {
		t.Errorf("devices %d, want 2", got.Devices)
	}
	for _, secret := range []string{"sun-rope-glass-mint", testLink(), fakeUUIDForPanel} {
		if strings.Contains(body, secret) {
			t.Errorf("the status document carries a credential: %q", secret)
		}
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	h := newHarness(t)
	h.ready()
	res, _ := h.get("/wp-admin")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want %d", res.StatusCode, http.StatusNotFound)
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	h := newHarness(t)
	h.ready()

	for _, path := range []string{"/", "/assets/panel.css", "/status.json", "/favicon.svg"} {
		res, _ := h.get(path)
		csp := res.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'none'") {
			t.Errorf("%s: Content-Security-Policy does not start from a closed default: %q", path, csp)
		}
		for _, directive := range []string{"script-src 'self'", "style-src 'self'", "connect-src 'self'", "form-action 'self'"} {
			if !strings.Contains(csp, directive) {
				t.Errorf("%s: Content-Security-Policy is missing %q", path, directive)
			}
		}
		// Nothing in the policy may name a host: the panel fetches nothing.
		if strings.Contains(csp, "http://") || strings.Contains(csp, "https://") || strings.Contains(csp, "*") {
			t.Errorf("%s: Content-Security-Policy names an external source: %q", path, csp)
		}
		if got := res.Header.Get("Cache-Control"); !strings.Contains(got, "no-store") {
			t.Errorf("%s: Cache-Control is %q, and this page can carry the hotspot password", path, got)
		}
		if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options is %q", path, got)
		}
		if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s: Referrer-Policy is %q", path, got)
		}
	}
}
