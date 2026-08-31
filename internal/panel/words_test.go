// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
)

// Design section 8, step 11: "The three failure states are distinguished in
// plain words: link did not parse, engine rejected it, server did not answer."
//
// The distinction is the requirement, not the individual sentences. Those three
// states need three different actions from the user: fix the text, get a
// different config, or check the machine's own internet connection. Collapsing
// them into one "it did not work" makes all three look like the same problem,
// and it is most often the third, so the user throws away a config that was
// never broken.

// TestTheThreeFailureStatesAreDistinguished checks the wording at the level the
// requirement is written at.
func TestTheThreeFailureStatesAreDistinguished(t *testing.T) {
	parse := ParseProblem(link.ErrNoLink)
	refused := EngineProblem()
	server := ServerProblem()

	if parse.Stage != StageParse {
		t.Errorf("a parse failure reports stage %v", parse.Stage)
	}
	if refused.Stage != StageEngine {
		t.Errorf("an engine rejection reports stage %v", refused.Stage)
	}
	if server.Stage != StageServer {
		t.Errorf("a silent server reports stage %v", server.Stage)
	}

	all := []Problem{parse, refused, server}
	for i, p := range all {
		if p.Headline == "" {
			t.Errorf("state %d has no headline key", i)
		}
		if p.Advice == "" {
			t.Errorf("state %d has no advice key, so it tells the user nothing to do", i)
		}
		for j, q := range all {
			if i != j && p.Headline == q.Headline {
				t.Errorf("states %d and %d share a headline key %q, so they are not distinguished", i, j, p.Headline)
			}
		}
	}

	// The distinction has to survive translation, so it is checked in every
	// language rather than in English only. This is the assertion that would
	// have caught a Persian catalogue where two of the three were rendered
	// with the same sentence.
	for _, lang := range Langs {
		seen := map[string]Key{}
		for _, p := range all {
			text := T(lang, p.Headline)
			if strings.Contains(text, missingMarker) {
				t.Errorf("%s: headline %q has no message", lang, p.Headline)
				continue
			}
			if other, dup := seen[text]; dup {
				t.Errorf("%s: %q and %q render the same sentence %q, so the three states are not distinguished in this language",
					lang, p.Headline, other, text)
			}
			seen[text] = p.Headline
			if advice := T(lang, p.Advice); strings.Contains(advice, missingMarker) {
				t.Errorf("%s: advice %q has no message", lang, p.Advice)
			}
		}
	}

	// And the third must not blame the config, because it is usually the
	// machine's own connection. Checked in English, where the phrase is
	// assertable; the Persian carries the same clause and is checked by eye in
	// the catalogue.
	if !strings.Contains(T(LangEN, server.Advice), "link itself is fine") {
		t.Errorf("the server advice does not clear the config: %q", T(LangEN, server.Advice))
	}
}

// TestNoErrorSentenceUsesJargon is a blunt check against the failure the design
// names by example: "No AP-capable phy" is not an error message for this
// audience.
func TestNoErrorSentenceUsesJargon(t *testing.T) {
	jargon := []string{
		"phy", "AP-capable", "nl80211", "rfkill", "hostapd", "dnsmasq",
		"nftables", "iptables", "netlink", "TUN", "xray", "SIGTERM",
		"errno", "EPERM", "stderr", "socket", "goroutine",
		"unmarshal", "parse error", "0x",
	}
	all := append([]Fault{}, faults...)
	all = append(all, Fault("something-this-build-has-never-heard-of"))

	for _, lang := range Langs {
		for _, f := range all {
			k := f.Key()
			if k == "" {
				t.Errorf("%s: fault %q has no message key", lang, f)
				continue
			}
			s := T(lang, k)
			if strings.Contains(s, missingMarker) {
				t.Errorf("%s: fault %q has no message", lang, f)
				continue
			}
			lower := strings.ToLower(s)
			for _, j := range jargon {
				if strings.Contains(lower, strings.ToLower(j)) {
					t.Errorf("%s: the sentence for %q uses %q: %s", lang, f, j, s)
				}
			}
			// The code itself must never be the message.
			if strings.Contains(s, string(f)) {
				t.Errorf("%s: the sentence for %q is the code: %s", lang, f, s)
			}
		}
		// An unrecognised fault must say it is unrecognised rather than
		// picking the nearest sentence, which would send somebody to fix the
		// wrong thing.
		if got := Fault("brand-new").Key(); got != MsgFaultUnrecognised {
			t.Errorf("an unrecognised fault maps to %q, want the unrecognised message", got)
		}
	}
}

// TestEveryLinkSentinelHasItsOwnWords makes sure the parse wording branches on
// internal/link's sentinels rather than falling through to one message.
func TestEveryLinkSentinelHasItsOwnWords(t *testing.T) {
	sentinels := map[string]error{
		"ErrEmpty":                link.ErrEmpty,
		"ErrUnsupportedScheme":    link.ErrUnsupportedScheme,
		"ErrNoLink":               link.ErrNoLink,
		"ErrBadUUID":              link.ErrBadUUID,
		"ErrBadAddress":           link.ErrBadAddress,
		"ErrBadPort":              link.ErrBadPort,
		"ErrBadReality":           link.ErrBadReality,
		"ErrUnsupportedTransport": link.ErrUnsupportedTransport,
	}
	seen := map[string]string{}
	for name, err := range sentinels {
		p := ParseProblem(err)
		if p.Advice == "" {
			t.Errorf("%s produces no advice", name)
			continue
		}
		if other, dup := seen[string(p.Advice)]; dup {
			t.Errorf("%s and %s produce the same advice key, so one of them fell through to the default", name, other)
		}
		seen[string(p.Advice)] = name
	}
	if len(seen) != len(sentinels) {
		t.Errorf("%d of %d sentinels produced distinct advice", len(seen), len(sentinels))
	}
	// A nil error is not a problem.
	if !ParseProblem(nil).Empty() {
		t.Error("a nil error produced a problem")
	}
}

// TestClockFailureIsNotBlamedOnTheConfig is the hazard design section 9 names:
// "The symptom is an authentication failure that the panel will blame on the
// config."
func TestClockFailureIsNotBlamedOnTheConfig(t *testing.T) {
	p := StartProblem(FaultClockImplausible)
	if p.Stage != StageNone {
		t.Errorf("a clock failure is reported as config stage %v", p.Stage)
	}
	if p.Headline != MsgFaultClock {
		t.Errorf("a clock failure uses headline %q, want the clock message", p.Headline)
	}
	en := T(LangEN, p.Headline)
	if !strings.Contains(en, "clock") {
		t.Errorf("a clock failure does not mention the clock: %q", en)
	}
	if !strings.Contains(en, "not a problem with your config") {
		t.Errorf("a clock failure does not clear the config: %q", en)
	}
}

// ---------------------------------------------------------------------------
// The same three states, end to end through the panel
// ---------------------------------------------------------------------------

// TestTheThreeFailureStatesReachTheUser drives each of the three through real
// HTTP and reads the page a user would see.
//
// The unit test above proves the sentences differ. This one proves the panel
// picks the right one of the three for the right cause, which is the half that
// a refactor breaks.
func TestTheThreeFailureStatesReachTheUser(t *testing.T) {
	t.Run("the link did not parse", func(t *testing.T) {
		h := newHarness(t)
		h.setup(testPassword)

		res, _ := h.postForm("/config", url.Values{
			"csrf":   {h.tokenOn("/")},
			"config": {unparseableConfig()},
		})
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("status %d", res.StatusCode)
		}
		_, body := h.get("/")
		assertHeadline(t, h.lang, body, MsgParseHeadline, MsgEngineHeadline, MsgServerHeadline)
		if h.store.Proxy().IsConfigured() {
			t.Error("a config that does not parse was stored anyway")
		}
	})

	t.Run("the engine rejected it", func(t *testing.T) {
		// The premise is checked rather than assumed. This fixture reaches the
		// engine only because internal/link accepts it and the engine does not,
		// and the engine's half of that is gated on the wall clock rather than
		// on its version: allowInsecure was accepted before 2026-06-01 and is
		// refused after. On a box whose clock has come up wrong, this config is
		// a perfectly good one and the test would be asserting the wrong state.
		if l, err := link.Parse(engineRejectedConfig()); err != nil {
			t.Fatalf("premise gone: internal/link now refuses this fixture (%v), so it "+
				"tests the parse state and not the engine state", err)
		} else if cfg, cfgErr := l.XrayConfig(); cfgErr != nil {
			t.Fatalf("premise gone: the fixture no longer builds a config document: %v", cfgErr)
		} else if engine.Validate(cfg) == nil {
			t.Skipf("premise gone: the engine now accepts this fixture. If this machine's " +
				"clock reads before 2026-06-01 that is the documented trap; otherwise the " +
				"engine has changed and this fixture needs replacing")
		}

		h := newHarness(t)
		h.setup(testPassword)

		res, _ := h.postForm("/config", url.Values{
			"csrf":   {h.tokenOn("/")},
			"config": {engineRejectedConfig()},
		})
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("status %d", res.StatusCode)
		}
		_, body := h.get("/")
		assertHeadline(t, h.lang, body, MsgEngineHeadline, MsgParseHeadline, MsgServerHeadline)
		if h.store.Proxy().IsConfigured() {
			t.Error("a config the engine refused was stored anyway")
		}

		// In advanced mode the engine's own words are available, which is what
		// the advice promises. In basic mode they are not.
		_, advanced := h.get("/?advanced=1")
		if !strings.Contains(advanced, T(h.lang, MsgProblemDetail)) {
			t.Error("advanced mode does not show the engine's own message, which the advice says it will")
		}
		if strings.Contains(body, T(h.lang, MsgProblemDetail)) {
			t.Error("basic mode is showing engine vocabulary")
		}
	})

	t.Run("the server did not answer", func(t *testing.T) {
		h := newHarness(t)
		h.ready()
		// The config is good and the engine accepts it. The privileged side
		// reports that nothing answered.
		h.priv.FailStartWith(FaultServerNoAnswer)

		res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
		if res.StatusCode != http.StatusSeeOther {
			t.Fatalf("status %d", res.StatusCode)
		}
		_, body := h.get("/")
		assertHeadline(t, h.lang, body, MsgServerHeadline, MsgParseHeadline, MsgEngineHeadline)

		// It must have got as far as actually trying, or this is the wrong
		// state being reported for the right reason.
		if len(h.priv.Starts()) != 1 {
			t.Errorf("the privileged service was asked to start %d times, want 1", len(h.priv.Starts()))
		}
		// And the config is still stored: nothing about a silent server means
		// the config is bad.
		if !h.store.Proxy().IsConfigured() {
			t.Error("the stored config was discarded because a server did not answer")
		}
	})
}

// assertHeadline checks that the page shows want and neither of the others.
func assertHeadline(t *testing.T, lang Lang, body string, want Key, notWanted ...Key) {
	t.Helper()
	if w := T(lang, want); !strings.Contains(body, w) {
		t.Errorf("the page does not say %q", w)
	}
	for _, other := range notWanted {
		if o := T(lang, other); strings.Contains(body, o) {
			t.Errorf("the page also says %q, so the three states are not being told apart", o)
		}
	}
}

// TestDetectedLineNamesThingsInPlainWords covers the one line design section
// 5.2 asks for by example.
func TestDetectedLineNamesThingsInPlainWords(t *testing.T) {
	cases := []struct {
		name string
		d    Detection
		want string
	}{
		{
			name: "ethernet uplink, built-in radio",
			d: Detection{
				InternetInterface: "eth0", HotspotInterface: "wlan0",
				Interfaces: []InterfaceInfo{
					{Name: "eth0", Kind: KindEthernet},
					{Name: "wlan0", Kind: KindBuiltinWiFi},
				},
			},
			want: "Internet: Ethernet. Hotspot: built-in WiFi.",
		},
		{
			name: "wifi uplink, usb adapter",
			d: Detection{
				InternetInterface: "wlan0", HotspotInterface: "wlan1",
				Interfaces: []InterfaceInfo{
					{Name: "wlan0", Kind: KindBuiltinWiFi},
					{Name: "wlan1", Kind: KindUSBWiFi},
				},
			},
			want: "Internet: built-in WiFi. Hotspot: a USB WiFi adapter.",
		},
		{
			name: "nothing found",
			d:    Detection{},
			want: "Internet: not found. Hotspot: not found.",
		},
		{
			// A kind this build does not recognise falls back to the kernel
			// name, which the user can at least compare against something.
			name: "unknown kind",
			d: Detection{
				InternetInterface: "end0", HotspotInterface: "wlx00",
				Interfaces: []InterfaceInfo{{Name: "end0"}, {Name: "wlx00"}},
			},
			want: "Internet: end0. Hotspot: wlx00.",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectedLineIn(LangEN, c.d); got != c.want {
				t.Errorf("DetectedLine\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestDeviceCountLine(t *testing.T) {
	cases := []struct {
		h    HotspotStatus
		want string
	}{
		{HotspotStatus{Devices: 0}, "No devices have joined yet"},
		{HotspotStatus{Devices: 1}, "1 device connected"},
		{HotspotStatus{Devices: 4}, "4 devices connected"},
		{
			// Under-reporting silently is worse than saying the count may be
			// short: the user counts the phones in the room, gets a different
			// answer, and stops trusting the screen.
			HotspotStatus{Devices: 2, UnreadableLeaseLines: 1},
			"2 devices connected (there may be more; part of the record could not be read)",
		},
	}
	for _, c := range cases {
		if got := DeviceCountLine(LangEN, c.h); got != c.want {
			t.Errorf("DeviceCountLine\n got %q\nwant %q", got, c.want)
		}
	}
}

// TestHotspotWordsComeFromInternalHotspot checks the delegation rather than the
// sentences: the panel must refuse whatever internal/hotspot refuses, including
// rules this file has never heard of.
func TestHotspotWordsComeFromInternalHotspot(t *testing.T) {
	// A well-known default. internal/hotspot refuses it because the
	// implementation this project replaces shipped it on every box; nothing in
	// internal/panel knows that list exists.
	if p := ValidatePassphrase("SecurePass123"); p.Empty() {
		t.Error("the panel accepted a passphrase internal/hotspot refuses as a known default")
	}
	if p := ValidatePassphrase("short"); p.Empty() {
		t.Error("the panel accepted a passphrase below the WPA2 minimum")
	}
	if p := ValidatePassphrase(strings.Repeat("a", 64)); p.Empty() {
		t.Error("the panel accepted a passphrase above the WPA2 maximum")
	}
	if p := ValidatePassphrase("has a\nnewline in it"); p.Empty() {
		t.Error("the panel accepted a passphrase with a newline, which would end the wpa_passphrase line")
	}
	// The control: a good one is accepted, or the checks above prove nothing.
	if p := ValidatePassphrase("sun-rope-glass-mint"); !p.Empty() {
		t.Errorf("the panel refused a good passphrase: %s", T(LangEN, p.Headline))
	}

	// The generated suggestion has to pass the validator that guards the form.
	for i := 0; i < 20; i++ {
		s := SuggestPassphrase()
		if s == "" {
			t.Fatal("SuggestPassphrase returned nothing")
		}
		if p := ValidatePassphrase(s); !p.Empty() {
			t.Fatalf("the suggested passphrase %q is refused by the panel's own check: %s", s, T(LangEN, p.Headline))
		}
	}
	if a, b := SuggestPassphrase(), SuggestPassphrase(); a == b {
		t.Error("two suggested passphrases are identical")
	}
	if a, b := SuggestSSID(), SuggestSSID(); a == b {
		t.Error("two suggested network names are identical")
	}
	if p := ValidateSSID(SuggestSSID()); !p.Empty() {
		t.Errorf("the suggested network name is refused: %s", T(LangEN, p.Headline))
	}
}
