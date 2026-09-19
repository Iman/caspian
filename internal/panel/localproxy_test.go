// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The local proxy line, added 2026-09-12 with the port fallback on the
// privileged side (issue #2: a Windows 11 box where another proxy client held
// 127.0.0.1:10808). The port is no longer always 10808, so the page has to say
// which one is in use, and it has to say it only while the address can be
// used: a stale address on a page is an address somebody types in.

// TestTheConnectedPageShowsTheLocalProxyAndTheOffPageDoesNot drives the panel
// through real HTTP in both languages. Off: the element is present, hidden,
// and carries no address, so the poll can reveal it without a reload. On: the
// label and the address are visible and status.json carries the address. Cut
// or off again: the address is gone from status.json.
func TestTheConnectedPageShowsTheLocalProxyAndTheOffPageDoesNot(t *testing.T) {
	for _, lang := range Langs {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			h.get("/?lang=" + string(lang))
			h.lang = lang
			h.ready()
			label := T(lang, MsgStatusLocalProxy)
			if strings.Contains(label, missingMarker) || label == "" {
				t.Fatalf("%s: the local proxy label has no message", lang)
			}

			// Off.
			_, off := h.get("/")
			if !strings.Contains(off, `id="local-proxy" hidden`) {
				t.Errorf("the off page does not carry the hidden local proxy element, so the poll could never reveal it")
			}
			if strings.Contains(off, fakeLocalProxy) {
				t.Errorf("the off page shows a proxy address")
			}
			if got := statusOf(t, h); got.LocalProxy != "" {
				t.Errorf("status.json carries localProxy %q while the box is off", got.LocalProxy)
			}

			// On, through the switch, which is how a real page gets there.
			res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("switching on: status %d", res.StatusCode)
			}
			_, on := h.get("/")
			if strings.Contains(on, `id="local-proxy" hidden`) {
				t.Errorf("the connected page hides the local proxy line")
			}
			if !strings.Contains(on, `id="local-proxy">`) {
				t.Errorf("the connected page has no visible local proxy element")
			}
			if !strings.Contains(on, label) {
				t.Errorf("the connected page does not carry the label %q", label)
			}
			want := `<bdi dir="ltr" class="mono" id="local-proxy-value">` + fakeLocalProxy + `</bdi>`
			if !strings.Contains(on, want) {
				t.Errorf("the connected page does not show the address isolated and in mono: want %s", want)
			}
			if got := statusOf(t, h); got.LocalProxy != fakeLocalProxy {
				t.Errorf("status.json localProxy = %q, want %q", got.LocalProxy, fakeLocalProxy)
			}

			// Cut: running, and deliberately not connected. The address goes
			// with the connection, because nothing can use it.
			if err := h.priv.Cut(t.Context()); err != nil {
				t.Fatalf("Cut: %v", err)
			}
			if got := statusOf(t, h); got.LocalProxy != "" {
				t.Errorf("status.json carries localProxy %q while client traffic is cut", got.LocalProxy)
			}
			if err := h.priv.Restore(t.Context()); err != nil {
				t.Fatalf("Restore: %v", err)
			}
			if got := statusOf(t, h); got.LocalProxy != fakeLocalProxy {
				t.Errorf("status.json localProxy = %q after restore, want %q", got.LocalProxy, fakeLocalProxy)
			}

			// Off again.
			res, _ = h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"0"}})
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("switching off: status %d", res.StatusCode)
			}
			if got := statusOf(t, h); got.LocalProxy != "" {
				t.Errorf("status.json carries localProxy %q after the box was switched off", got.LocalProxy)
			}
			_, offAgain := h.get("/")
			if !strings.Contains(offAgain, `id="local-proxy" hidden`) || strings.Contains(offAgain, fakeLocalProxy) {
				t.Errorf("the page still shows the local proxy after the box was switched off")
			}
		})
	}
}

// statusOf fetches and decodes /status.json.
func statusOf(t *testing.T, h *harness) statusJSON {
	t.Helper()
	res, body := h.get("/status.json")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /status.json: %d", res.StatusCode)
	}
	var got statusJSON
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("the status document is not JSON: %v", err)
	}
	return got
}
