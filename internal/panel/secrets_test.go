// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The pasted config is a credential (design section 6, and section 9 lists log
// redaction as a known hazard). These tests are the ones that hold the rule.

// TestPastedConfigNeverAppearsInAResponseOrALog submits a config and then walks
// every page the panel can draw, in both modes, looking for it.
//
// Two things make this more than a spot check. The needles are the whole link
// AND its individual parts, because "the config was not echoed" and "no piece of
// the config was echoed" are different claims and only the second matters. And
// the positive control at the end proves the search would have found something:
// a test that greps for a string it can never find passes for the wrong reason,
// and that failure mode is silent.
func TestPastedConfigNeverAppearsInAResponseOrALog(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	if err := h.store.SetHotspot("Caspian-test", "sun-rope-glass-mint"); err != nil {
		t.Fatal(err)
	}

	// Submit it the way a user does.
	token := h.tokenOn("/")
	res, body := h.postForm("/config", url.Values{
		"csrf":   {token},
		"config": {testLink()},
		"label":  {fakeLabel},
	})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /config: status %d, body %s", res.StatusCode, body)
	}
	if !h.store.Proxy().IsConfigured() {
		t.Fatal("the config was not stored, so the rest of this test would prove nothing")
	}

	// It has to have reached the privileged boundary intact when switched on,
	// or the panel is not leaking it because it is not using it.
	if res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}}); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("switching on returned %d", res.StatusCode)
	}
	starts := h.priv.Starts()
	if len(starts) != 1 {
		t.Fatalf("the privileged service received %d start requests, want 1", len(starts))
	}
	if len(starts[0].ConfigJSON) == 0 {
		t.Fatal("the start request carried no config document")
	}
	// What crossed the boundary is a re-serialised document, never the pasted
	// text (design section 6: parse and re-serialise, never interpolate).
	if strings.Contains(string(starts[0].ConfigJSON), testLink()) {
		t.Error("the pasted text itself was interpolated into the config document")
	}
	if !strings.Contains(string(starts[0].ConfigJSON), fakeUUIDForPanel) {
		t.Error("the config document does not carry the user id, so it is not the real config and this test is checking nothing")
	}

	// Now walk everything the panel will draw.
	paths := []string{
		"/", "/?advanced=1", "/?advanced=0", "/status.json",
		"/assets/panel.css", "/assets/panel.js", "/favicon.svg",
		"/wp-admin", // the not-found page
	}
	for _, path := range paths {
		res, body := h.get(path)
		if secret, found := containsAny(body, testLinkSecrets()); found {
			t.Errorf("%s (status %d) contains %q from the pasted config", path, res.StatusCode, secret)
		}
		// The redirect target is part of the response too, and a query
		// parameter is the classic way a config ends up in a URL.
		if loc := res.Header.Get("Location"); loc != "" {
			if secret, found := containsAny(loc, testLinkSecrets()); found {
				t.Errorf("%s redirects to a URL containing %q", path, secret)
			}
		}
	}

	// Signed out, too: an error or login page must not carry it either.
	h.signedOut()
	for _, path := range []string{"/", "/login", "/setup", "/status.json"} {
		res, body := h.get(path)
		if secret, found := containsAny(body, testLinkSecrets()); found {
			t.Errorf("signed out, %s (status %d) contains %q", path, res.StatusCode, secret)
		}
	}

	// And the log.
	logs := h.logs.String()
	if logs == "" {
		t.Fatal("nothing was logged at all, so the log check below proves nothing")
	}
	if secret, found := containsAny(logs, testLinkSecrets()); found {
		t.Errorf("the log contains %q from the pasted config.\nLog was:\n%s", secret, logs)
	}
	// The hotspot passphrase is a credential on the same footing, and unlike
	// the config it IS rendered on the page, so only the log is checked.
	if strings.Contains(logs, "sun-rope-glass-mint") {
		t.Errorf("the log contains the hotspot passphrase.\nLog was:\n%s", logs)
	}
	// The panel password must not be there either.
	if strings.Contains(logs, testPassword) {
		t.Errorf("the log contains the panel password.\nLog was:\n%s", logs)
	}

	// The positive control. The same search, over a body that does contain the
	// secret, must find it. Without this, a typo in containsAny or an empty
	// needle list would make every assertion above vacuous.
	if _, found := containsAny("prefix "+fakeUUIDForPanel+" suffix", testLinkSecrets()); !found {
		t.Fatal("the search cannot find a secret that is present, so the assertions above prove nothing")
	}
}

// TestFailedConfigPathsDoNotEchoTheInput covers the paths a user actually
// reaches by mistake, which are the paths where a naive handler quotes the input
// back inside the error.
func TestFailedConfigPathsDoNotEchoTheInput(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	// A plausible mistake: half a link, with the credential in it.
	halfPasted := "vless://" + fakeUUIDForPanel + "@" + fakeHostForPanel
	needles := []string{halfPasted, fakeUUIDForPanel}

	for _, bad := range []struct {
		name  string
		value string
	}{
		{"half a link", halfPasted},
		{"an unsupported scheme", "tuic://" + fakeUUIDForPanel + "@" + fakeHostForPanel + ":443"},
		{"nothing at all", "   "},
		{"raw json the engine refuses", engineRejectedConfig()},
	} {
		t.Run(bad.name, func(t *testing.T) {
			res, _ := h.postForm("/config", url.Values{
				"csrf":   {h.tokenOn("/")},
				"config": {bad.value},
			})
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("status %d, want a redirect", res.StatusCode)
			}
			if loc := res.Header.Get("Location"); strings.Contains(loc, "vless") || strings.Contains(loc, fakeUUIDForPanel) {
				t.Errorf("the redirect URL carries the input: %q", loc)
			}
			// The page that follows shows the message.
			_, body := h.get("/?advanced=1")
			if secret, found := containsAny(body, needles); found {
				t.Errorf("the failure page contains %q from the input", secret)
			}
		})
	}

	if secret, found := containsAny(h.logs.String(), needles); found {
		t.Errorf("the log contains %q from a rejected input", secret)
	}
}

// TestStartRequestRedactsItself is the second lock: even if a future handler
// hands the whole request to a log line, nothing comes out.
func TestStartRequestRedactsItself(t *testing.T) {
	req := StartRequest{
		ConfigJSON: []byte(`{"outbounds":[{"settings":{"id":"` + fakeUUIDForPanel + `"}}]}`),
		Hotspot: HotspotSpec{
			SSID:       "Caspian-test",
			Passphrase: "sun-rope-glass-mint",
			Interface:  "wlan0",
		},
		Network: NetworkSpec{InternetInterface: "eth0", DNSMode: "tunnel", OnTunnelDown: "block"},
	}

	for _, format := range []string{"%v", "%s", "%+v", "%#v"} {
		rendered := sprintf(format, req)
		if strings.Contains(rendered, fakeUUIDForPanel) {
			t.Errorf("%s printed the config: %s", format, rendered)
		}
		if strings.Contains(rendered, "sun-rope-glass-mint") {
			t.Errorf("%s printed the hotspot passphrase: %s", format, rendered)
		}
		// The parts that are not credentials should still be there, or this
		// type is useless for diagnosis.
		if !strings.Contains(rendered, "Caspian-test") {
			t.Errorf("%s printed nothing useful: %s", format, rendered)
		}
	}

	// HotspotSpec on its own, since it is passed around separately.
	for _, format := range []string{"%v", "%s", "%+v", "%#v"} {
		if rendered := sprintf(format, req.Hotspot); strings.Contains(rendered, "sun-rope-glass-mint") {
			t.Errorf("HotspotSpec %s printed the passphrase: %s", format, rendered)
		}
	}
}
