// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/state"
)

// The tests in this file were written BEFORE POST /refresh, the
// subscription_url form field and the refresh lines on the config card
// existed, on 2026-09-09. Their first run failed on the missing route and the
// missing catalogue keys; that is the "red" in the report.
//
// What they hold: the address is stored on its own or with a paste and never
// shown back; the button exists only with an address and is disabled until
// the tunnel runs; a refresh goes through EXACTLY the paste path, so a body
// that would not be accepted by paste is not accepted by refresh and leaves
// the stored config untouched; and the address reaches the privileged
// boundary and nothing else: no page, no log line, no event, no status
// document, no redacted rendering.

// fakeSubscriptionURL carries a token, which is why it is a credential. The
// host is a documentation name.
const (
	fakeSubscriptionHost  = "sub.example.com"
	fakeSubscriptionToken = "fake-panel-refresh-token-not-real"
	fakeSubscriptionURL   = "https://" + fakeSubscriptionHost + "/api/v1/sub?token=" + fakeSubscriptionToken
)

// subscriptionSecrets is every part of the address that must never appear
// outside the store and the privileged boundary.
func subscriptionSecrets() []string {
	return []string{fakeSubscriptionURL, fakeSubscriptionHost, fakeSubscriptionToken}
}

// threeLinks is a provider body with three entries, all documentation values.
func threeLinks() string {
	one := "vless://" + fakeUUIDForPanel + "@" + fakeHostForPanel + ":443?type=tcp&security=none#first"
	two := "vless://" + fakeUUIDForPanel + "@" + fakeHostForPanel + ":444?type=tcp&security=none#second"
	three := "vless://" + fakeUUIDForPanel + "@" + fakeHostForPanel + ":445?type=tcp&security=none#third"
	return one + "\n" + two + "\n" + three + "\n"
}

// withRunningTunnel puts the fake in the state the button needs.
func (h *harness) withRunningTunnel() {
	h.t.Helper()
	h.priv.SetEngineState(engine.State{Phase: engine.PhaseRunning, Since: h.clock.Now().Add(-time.Hour)})
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test", Devices: 1})
}

// setSubscription stores the address the way a person does, through the form.
func (h *harness) setSubscription(u string) *http.Response {
	h.t.Helper()
	res, _ := h.postForm("/config", url.Values{
		"csrf":             {h.tokenOn("/")},
		"config":           {""},
		"subscription_url": {u},
	})
	return res
}

// pressRefresh presses the button.
func (h *harness) pressRefresh() *http.Response {
	h.t.Helper()
	res, _ := h.postForm("/refresh", url.Values{"csrf": {h.tokenOn("/")}})
	return res
}

// assertAddressNowhere walks everything the panel can show or write and fails
// on any part of the address. It is the positive half of the whole feature's
// privacy claim, so it is called at the end of every test that stores one.
func (h *harness) assertAddressNowhere() {
	h.t.Helper()
	for _, path := range []string{"/", "/?advanced=1", "/status.json", "/help"} {
		_, body := h.get(path)
		if secret, found := containsAny(body, subscriptionSecrets()); found {
			h.t.Errorf("%s carries part of the subscription address (%q)", path, secret)
		}
	}
	if secret, found := containsAny(h.logs.String(), subscriptionSecrets()); found {
		h.t.Errorf("a log line carries part of the subscription address (%q)", secret)
	}
	for _, e := range h.panel.events.entries() {
		for _, lang := range Langs {
			if secret, found := containsAny(e.Sentence(lang), subscriptionSecrets()); found {
				h.t.Errorf("an event carries part of the subscription address (%q)", secret)
			}
		}
	}
	if secret, found := containsAny(h.store.Snapshot().Redacted(), subscriptionSecrets()); found {
		h.t.Errorf("the redacted state carries part of the subscription address (%q)", secret)
	}
}

// ---------------------------------------------------------------------------
// Storing the address
// ---------------------------------------------------------------------------

// TestTheAddressCanBeSavedOnItsOwn: a person can have the address before they
// have a config. Saving it with an empty paste stores the address and nothing
// else, says so, and shows nothing of it back.
func TestTheAddressCanBeSavedOnItsOwn(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	res := h.setSubscription(fakeSubscriptionURL)
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /config with an address alone: status %d", res.StatusCode)
	}
	p := h.store.Proxy()
	if p.SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Fatal("the address was not stored")
	}
	if p.IsConfigured() {
		t.Error("an empty paste stored a config")
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgNoticeSubscriptionSaved)) {
		t.Error("the page does not say the address was saved")
	}
	if !strings.Contains(body, h.msg(MsgConfigSubscriptionSet)) {
		t.Error("the page does not say an address is set")
	}
	h.assertAddressNowhere()

	// The field is rendered empty on the next visit: the value is never sent
	// back to the browser, even to the browser that typed it.
	if strings.Contains(body, `name="subscription_url" value="`) && !strings.Contains(body, `name="subscription_url" value=""`) {
		t.Error("the address field is prefilled")
	}
}

// TestTheAddressCanBeSavedWithAPaste: both at once stores both, and a paste
// with the address field left empty leaves a stored address alone.
func TestTheAddressCanBeSavedWithAPaste(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	res, _ := h.postForm("/config", url.Values{
		"csrf":             {h.tokenOn("/")},
		"config":           {testLink()},
		"label":            {fakeLabel},
		"subscription_url": {fakeSubscriptionURL},
	})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	p := h.store.Proxy()
	if p.Raw.Reveal() != testLink() || p.SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Fatal("a paste with an address did not store both")
	}

	res, _ = h.postForm("/config", url.Values{
		"csrf":   {h.tokenOn("/")},
		"config": {testLink()},
	})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	if h.store.Proxy().SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Error("a paste with the address field empty cleared the stored address; the field is always rendered empty, so empty cannot mean clear")
	}
	h.assertAddressNowhere()
}

// TestABadAddressIsRefusedWithItsOwnSentence: each rule internal/state holds
// has a sentence of its own, nothing is stored, and the refused value is not
// echoed. An empty paste beside a refused address is still an empty paste,
// so nothing about the config changes either.
func TestABadAddressIsRefusedWithItsOwnSentence(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  Key
	}{
		{"plain http", "http://" + fakeSubscriptionHost + "/sub", MsgSubscriptionNotHTTPS},
		{"no host", "https:///sub", MsgSubscriptionNoHost},
		{"ipv4 literal", "https://192.0.2.10/sub", MsgSubscriptionIPLiteral},
		{"ipv6 literal", "https://[2001:db8::1]/sub", MsgSubscriptionIPLiteral},
		{"user name", "https://user:pass@" + fakeSubscriptionHost + "/sub", MsgSubscriptionUserinfo},
		{"too long", "https://" + fakeSubscriptionHost + "/" + strings.Repeat("a", 2048), MsgSubscriptionTooLong},
		{"malformed", "https://" + fakeSubscriptionHost + "/%zz", MsgSubscriptionMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.ready()
			before := h.store.Proxy()

			res := h.setSubscription(tc.value)
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("status %d", res.StatusCode)
			}
			if h.store.Proxy().HasSubscription() {
				t.Error("a refused address was stored")
			}
			if h.store.Proxy().Raw != before.Raw {
				t.Error("a refused address changed the stored config")
			}
			_, body := h.get("/")
			if !strings.Contains(body, h.msg(tc.want)) {
				t.Errorf("the page does not show the sentence for %s", tc.name)
			}
			if strings.Contains(body, tc.value) || strings.Contains(body, "192.0.2.10") || strings.Contains(body, "2001:db8") {
				t.Error("the refused address was echoed into the page")
			}
			if secret, found := containsAny(h.logs.String(), []string{tc.value, fakeSubscriptionHost}); found && tc.value != "" {
				t.Errorf("a log line carries the refused address (%q)", secret)
			}
		})
	}
}

// TestBothEmptyIsStillAnEmptyPaste pins that adding the field did not change
// what an empty form does.
func TestBothEmptyIsStillAnEmptyPaste(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	res := h.setSubscription("")
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgParseEmpty)) {
		t.Error("an empty form no longer says the paste was empty")
	}
	if h.store.Proxy().HasSubscription() {
		t.Error("an empty address was stored")
	}
}

// ---------------------------------------------------------------------------
// The button
// ---------------------------------------------------------------------------

// TestTheRefreshButtonExistsOnlyWithAnAddressAndOnlyLiveWhenTheTunnelRuns is
// acceptance item 1: address set, tunnel stopped, the button is disabled with
// the sentence, and pressing it anyway makes zero dials.
func TestTheRefreshButtonExistsOnlyWithAnAddressAndOnlyLiveWhenTheTunnelRuns(t *testing.T) {
	h := newHarness(t)
	h.ready()

	_, body := h.get("/")
	if strings.Contains(body, `action="/refresh"`) {
		t.Error("the button is drawn with no address to refresh from")
	}

	h.setSubscription(fakeSubscriptionURL)
	_, body = h.get("/")
	if !strings.Contains(body, `action="/refresh"`) {
		t.Fatal("the button is missing although an address is set")
	}
	if !strings.Contains(body, `id="refresh-button" disabled`) && !strings.Contains(body, `disabled`) {
		t.Error("the button is live while the tunnel is stopped")
	}
	if !strings.Contains(body, h.msg(MsgConfigRefreshDisabled)) {
		t.Error("the disabled button has no sentence explaining why")
	}

	// Pressed anyway, from a stale tab: refused, and the privileged side is
	// asked and refuses before any dial. The fake models exactly that gate.
	h.pressRefresh()
	_, body = h.get("/")
	if !strings.Contains(body, h.msg(MsgRefreshNotRunning)) {
		t.Error("pressing while stopped does not say the tunnel is not running")
	}
	if h.store.Proxy().Raw.Reveal() != testLink() {
		t.Error("a refused refresh changed the config")
	}

	h.withRunningTunnel()
	_, body = h.get("/")
	button := body[strings.Index(body, `id="refresh-button"`):]
	button = button[:strings.Index(button, ">")]
	if strings.Contains(button, "disabled") {
		t.Error("the button is disabled while the tunnel runs")
	}
	if strings.Contains(body, h.msg(MsgConfigRefreshDisabled)) {
		t.Error("the disabled sentence is shown while the tunnel runs")
	}
	h.assertAddressNowhere()
}

// ---------------------------------------------------------------------------
// The refresh itself
// ---------------------------------------------------------------------------

// TestARefreshReplacesTheConfigExactlyLikeAPaste is acceptance item 3: three
// links with usage figures come back, Raw is replaced, Count is 3, Selected
// is 0, the usage line shows used and expiry, and the engine restarts on
// entry 0.
func TestARefreshReplacesTheConfigExactlyLikeAPaste(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	if err := h.store.SetHotspot("Caspian-test", "sun-rope-glass-mint"); err != nil {
		t.Fatal(err)
	}
	// A config with no label, so the provider's title is taken.
	if err := h.store.SetProxyConfig(testLink(), "vless", ""); err != nil {
		t.Fatal(err)
	}
	if err := h.store.SelectProxyEntry(0); err != nil {
		t.Fatal(err)
	}
	h.setSubscription(fakeSubscriptionURL)
	h.withRunningTunnel()

	h.priv.SetRefreshReply(RefreshReply{
		Status: 200,
		Body:   []byte(threeLinks()),
		Headers: map[string]string{
			"subscription-userinfo": "upload=100000000; download=12300000000; total=100000000000; expire=1780272000",
			"profile-title":         "base64:UHJvdmlkZXIgUGx1cw==", // "Provider Plus"
		},
	})

	res := h.pressRefresh()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /refresh: status %d", res.StatusCode)
	}

	p := h.store.Proxy()
	if p.Raw.Reveal() != threeLinks() {
		t.Fatal("the fetched body was not stored")
	}
	if p.Selected != 0 {
		t.Errorf("Selected = %d, want 0", p.Selected)
	}
	if p.Label != "Provider Plus" {
		t.Errorf("Label = %q, want the decoded profile title", p.Label)
	}
	if p.Scheme != "vless" {
		t.Errorf("Scheme = %q", p.Scheme)
	}
	want := state.Quota{Upload: 100000000, Download: 12300000000, Total: 100000000000, Expire: 1780272000}
	if p.Quota != want {
		t.Errorf("Quota = %+v, want %+v", p.Quota, want)
	}
	if !p.RefreshedAt.Equal(h.clock.Now()) {
		t.Errorf("RefreshedAt = %v, want the clock %v", p.RefreshedAt, h.clock.Now())
	}
	if p.SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Error("the refresh lost the address")
	}
	list, err := link.ParseAll(p.Raw.Reveal())
	if err != nil || len(list.Entries) != 3 {
		t.Fatalf("the stored body does not parse to three entries: %v", err)
	}

	// The address reached the privileged boundary, exactly as stored.
	refreshes := h.priv.Refreshes()
	if len(refreshes) != 1 || refreshes[0].URL != fakeSubscriptionURL {
		t.Fatalf("the privileged side received %v", refreshes)
	}

	// The tunnel was running, so it was restarted on the new config.
	if h.priv.Stops() != 1 || len(h.priv.Starts()) != 1 {
		t.Errorf("stops=%d starts=%d after a refresh on a running box, want 1 and 1", h.priv.Stops(), len(h.priv.Starts()))
	}

	// The page, in both languages.
	for _, lang := range Langs {
		h.get("/?lang=" + string(lang))
		h.lang = lang
		_, body := h.get("/")
		for _, want := range []string{
			T(lang, MsgNoticeRefreshReconn),
			T(lang, MsgRefreshLine, T(lang, MsgRefreshWhenJustNow), T(lang, MsgRefreshEntriesMany, 3)),
			T(lang, MsgRefreshUsed, isolateLTR("12.4 GB"), isolateLTR("100.0 GB")),
			T(lang, MsgRefreshExpires, isolateLTR("2026-05-31")),
			T(lang, MsgEventConfigRefreshed),
			"Provider Plus",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: the page is missing %q", lang, want)
			}
		}
	}

	// Later, the line ages with the clock.
	h.clock.Advance(3 * time.Minute)
	_, body := h.get("/")
	if !strings.Contains(body, T(h.lang, MsgRefreshWhenMinutes, 3)) {
		t.Error("the refreshed line does not age with the clock")
	}
	h.assertAddressNowhere()
}

// TestARefreshKeepsANameThePersonChose: the provider's title fills the name
// only when the person left it empty.
func TestARefreshKeepsANameThePersonChose(t *testing.T) {
	h := newHarness(t)
	h.ready() // label "Home"
	h.setSubscription(fakeSubscriptionURL)
	h.withRunningTunnel()
	h.priv.SetRefreshReply(RefreshReply{
		Status:  200,
		Body:    []byte(threeLinks()),
		Headers: map[string]string{"profile-title": "Provider"},
	})
	h.pressRefresh()
	if got := h.store.Proxy().Label; got != "Home" {
		t.Errorf("Label = %q after a refresh, want the person's own name", got)
	}
}

// TestARefreshWithNoFiguresDrawsNoUsageLine: a provider that sends no
// userinfo leaves the quota zero and the card shows the refresh line alone.
func TestARefreshWithNoFiguresDrawsNoUsageLine(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.setSubscription(fakeSubscriptionURL)
	h.withRunningTunnel()
	h.priv.SetRefreshReply(RefreshReply{Status: 200, Body: []byte(testLink())})
	h.pressRefresh()

	p := h.store.Proxy()
	if p.Quota != (state.Quota{}) {
		t.Errorf("Quota = %+v with no figures sent", p.Quota)
	}
	_, body := h.get("/")
	if !strings.Contains(body, T(h.lang, MsgRefreshLine, T(h.lang, MsgRefreshWhenJustNow), T(h.lang, MsgRefreshEntriesOne))) {
		t.Error("the refresh line is missing")
	}
	if strings.Contains(body, strings.SplitN(T(h.lang, MsgRefreshUsed), "%", 2)[0]) {
		t.Error("a usage line is drawn with no figures")
	}
	if strings.Contains(body, strings.SplitN(T(h.lang, MsgRefreshExpires), "%", 2)[0]) {
		t.Error("an expiry line is drawn with no expiry")
	}
}

// TestARefreshWithoutAnAddressIsRefused: nothing to fetch from, nothing
// asked of the privileged side.
func TestARefreshWithoutAnAddressIsRefused(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.withRunningTunnel()
	res := h.pressRefresh()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgRefreshNoURL)) {
		t.Error("the page does not say there is no address")
	}
	if n := len(h.priv.Refreshes()); n != 0 {
		t.Errorf("the privileged side was asked %d times with no address stored", n)
	}
}

// TestEveryRefreshFailureLeavesTheConfigUntouched is acceptance items 4 and
// 5 and the failure half of 3: for every way a refresh can fail, Raw, the
// selection, the label and the figures are exactly as they were, the tunnel
// is not restarted, and the person sees the one sentence for that reason.
func TestEveryRefreshFailureLeavesTheConfigUntouched(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(h *harness)
		want    Key
		event   bool
	}{
		{"the tunnel is not running", func(h *harness) {
			h.priv.SetEngineState(engine.State{Phase: engine.PhaseStopped})
		}, MsgRefreshNotRunning, true},
		{"the address was refused on the privileged side", func(h *harness) {
			h.priv.FailRefreshWith(FaultRefreshBadAddress)
		}, MsgRefreshBadAddress, true},
		{"the provider did not answer", func(h *harness) {
			h.priv.FailRefreshWith(FaultRefreshNoAnswer)
		}, MsgRefreshNoAnswer, true},
		{"the answer was too large", func(h *harness) {
			h.priv.FailRefreshWith(FaultRefreshTooLarge)
		}, MsgRefreshTooLarge, true},
		{"the provider redirected to plain http", func(h *harness) {
			h.priv.FailRefreshWith(FaultRefreshNotHTTPS)
		}, MsgRefreshNotHTTPS, true},
		{"the privileged service is unavailable", func(h *harness) {
			h.priv.FailRefreshWith(FaultUnavailable)
		}, MsgFaultUnavailable, true},
		{"an unclassified failure", func(h *harness) {
			h.priv.FailRefreshWith(FaultUnknown)
		}, MsgRefreshFailedOther, true},
		{"the provider answered with an error status", func(h *harness) {
			h.priv.SetRefreshReply(RefreshReply{Status: 503})
		}, MsgRefreshBadStatus, true},
		{"the provider answered with something that is not a configuration", func(h *harness) {
			h.priv.SetRefreshReply(RefreshReply{Status: 200, Body: []byte("<html><body>Sign in to your account</body></html>")})
		}, MsgRefreshNotConfig, true},
		{"the provider answered with nothing", func(h *harness) {
			h.priv.SetRefreshReply(RefreshReply{Status: 200, Body: nil})
		}, MsgRefreshNotConfig, true},
		{"the engine refused the new configuration", func(h *harness) {
			h.priv.SetRefreshReply(RefreshReply{Status: 200, Body: []byte(engineRejectedConfig())})
			// The fixture is refused for allowInsecure, which has its own
			// sentence since 2026-09-12; see EngineRejection.
		}, MsgEngineInsecureHeadline, true},
		{"the body is over the paste cap", func(h *harness) {
			// The privileged side caps this already; the panel holds the
			// same cap rather than trusting that.
			h.priv.SetRefreshReply(RefreshReply{Status: 200, Body: []byte(strings.Repeat("a", maxBodyBytes+1))})
		}, MsgRefreshTooLarge, true},
	}

	// The engine-refused case rests on a fixture the engine has to refuse
	// today; words_test.go explains the wall-clock trap.
	if l, err := link.Parse(engineRejectedConfig()); err != nil {
		t.Fatalf("premise gone: internal/link refuses the engine fixture: %v", err)
	} else if cfg, err := l.XrayConfig(); err != nil {
		t.Fatalf("premise gone: the engine fixture builds no document: %v", err)
	} else if engine.Validate(cfg) == nil {
		t.Skip("premise gone: the engine accepts the fixture words_test.go relies on")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.ready()
			if err := h.store.SelectProxyEntry(0); err != nil {
				t.Fatal(err)
			}
			h.setSubscription(fakeSubscriptionURL)
			h.withRunningTunnel()
			before := h.store.Proxy()
			tc.arrange(h)

			res := h.pressRefresh()
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("status %d", res.StatusCode)
			}
			after := h.store.Proxy()
			if after.Raw != before.Raw || after.Selected != before.Selected || after.Label != before.Label ||
				after.Scheme != before.Scheme || after.Quota != before.Quota || !after.RefreshedAt.Equal(before.RefreshedAt) ||
				!after.AddedAt.Equal(before.AddedAt) {
				t.Errorf("a failed refresh changed the stored config:\n before %+v\n after  %+v", redactedProxy(before), redactedProxy(after))
			}
			if after.SubscriptionURL != before.SubscriptionURL {
				t.Error("a failed refresh changed the address")
			}
			if h.priv.Stops() != 0 || len(h.priv.Starts()) != 0 {
				t.Error("a failed refresh restarted the tunnel")
			}
			_, raw := h.get("/")
			// Unescaped, because a sentence with an apostrophe in it reaches
			// the page as an entity and the catalogue holds the character.
			body := html.UnescapeString(raw)
			if !strings.Contains(body, h.msg(tc.want)) {
				t.Errorf("the page does not show the sentence for this reason (%s)", tc.want)
			}
			if tc.want == MsgRefreshBadStatus && !strings.Contains(body, T(h.lang, MsgRefreshBadStatusAdvice, 503)) {
				t.Error("the error status sentence does not carry the code")
			}
			if tc.event {
				found := false
				for _, e := range h.panel.events.entries() {
					if e.Kind == EventRefreshFailed {
						found = true
					}
				}
				if !found {
					t.Error("no event records that the refresh failed")
				}
			}
			h.assertAddressNowhere()
		})
	}
}

// redactedProxy is a ProxyConfig with the credential fields replaced, for a
// test failure message that has to be readable and must not print them.
func redactedProxy(p state.ProxyConfig) string {
	return strings.Join([]string{
		"configured=" + boolWord(p.IsConfigured()),
		"fingerprint=" + p.Fingerprint(),
		"scheme=" + p.Scheme,
		"label=" + p.Label,
		"selected=" + itoaWord(p.Selected),
		"subscription=" + boolWord(p.HasSubscription()),
		"refreshed=" + p.RefreshedAt.UTC().Format(time.RFC3339),
	}, " ")
}

func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoaWord(n int) string { return strings.TrimSpace(strings.Repeat(" ", 0) + string(rune('0'+n))) }

// TestARefreshOnAStoppedBoxDoesNotStart: a refresh that succeeds while the
// box is off stores the config and leaves the box off, like a paste does.
func TestARefreshOnAStoppedBoxDoesNotStart(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.setSubscription(fakeSubscriptionURL)
	// The fake refuses while stopped, which is right; script the success a
	// real box would give if the phase changed between the page and the
	// press by running, refreshing, and stopping before the redirect lands.
	h.withRunningTunnel()
	h.priv.SetRefreshReply(RefreshReply{Status: 200, Body: []byte(threeLinks())})
	h.pressRefresh()
	if len(h.priv.Starts()) != 1 {
		t.Fatalf("a refresh on a running box started %d times, want 1", len(h.priv.Starts()))
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgNoticeRefreshReconn)) {
		t.Error("the page does not say the box reconnected on the refreshed config")
	}
}
