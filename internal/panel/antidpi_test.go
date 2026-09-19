// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// TestTheAntiDPIStateIsVisibleWithoutOpeningTheSection covers the change of
// 2026-09-13: the section heading says whether any anti-DPI control is saved,
// and while the box is connected a line under the status names what is in
// force. Before this, the only way to learn whether the decoy name was on was
// to open the section and look at the field.
func TestTheAntiDPIStateIsVisibleWithoutOpeningTheSection(t *testing.T) {
	valueOf := regexp.MustCompile(`id="anti-dpi-value">(.*?)</span>`)
	for _, lang := range Langs {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			h.get("/?lang=" + string(lang))
			h.lang = lang
			h.ready()
			headingOff := T(lang, MsgSNISpoofHeading) + ": " + T(lang, MsgSNIStateOff)
			headingOn := T(lang, MsgSNISpoofHeading) + ": " + T(lang, MsgSNIStateOn)

			// Nothing saved: the heading says off, the line is present and hidden.
			_, body := h.get("/")
			body = html.UnescapeString(body)
			if !strings.Contains(body, headingOff) {
				t.Errorf("with nothing saved the heading does not read %q", headingOff)
			}
			if !strings.Contains(body, `id="anti-dpi" hidden`) {
				t.Error("the anti-DPI line is not in the page hidden, so the poll could never reveal it")
			}

			// A decoy name and both splits saved, box off: the heading says on,
			// the line is rendered with every part but stays hidden.
			if err := h.store.SetDPISettings("cover.example.invalid", true, true); err != nil {
				t.Fatal(err)
			}
			_, body = h.get("/")
			body = html.UnescapeString(body)
			if !strings.Contains(body, headingOn) {
				t.Errorf("with controls saved the heading does not read %q", headingOn)
			}
			if !strings.Contains(body, `id="anti-dpi" hidden`) {
				t.Error("the anti-DPI line shows while the box is off")
			}
			m := valueOf.FindStringSubmatch(body)
			if m == nil {
				t.Fatal("no anti-dpi-value span in the page")
			}
			for _, want := range []string{T(lang, MsgSNIActiveName), `<bdi dir="ltr" class="mono">cover.example.invalid</bdi>`, T(lang, MsgTCPSplit), T(lang, MsgTLSRecordSplit)} {
				if !strings.Contains(m[1], want) {
					t.Errorf("the line lacks %q: %s", want, m[1])
				}
			}
			if got := statusOf(t, h); got.AntiDPI {
				t.Error("status.json says antiDPI while the box is off")
			}

			// Decoy name only, box on: the line is visible, names the decoy and
			// not the splits, and status.json says so.
			if err := h.store.SetDPISettings("cover.example.invalid", false, false); err != nil {
				t.Fatal(err)
			}
			res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("switching on: status %d", res.StatusCode)
			}
			_, body = h.get("/")
			body = html.UnescapeString(body)
			if strings.Contains(body, `id="anti-dpi" hidden`) {
				t.Error("the anti-DPI line is hidden while connected with a decoy name saved")
			}
			if !strings.Contains(body, `<span class="label">`+T(lang, MsgSNIActiveLabel)+`</span>`) {
				t.Errorf("the line does not carry the label %q", T(lang, MsgSNIActiveLabel))
			}
			m = valueOf.FindStringSubmatch(body)
			if m == nil || !strings.Contains(m[1], "cover.example.invalid") || strings.Contains(m[1], T(lang, MsgTCPSplit)) || strings.Contains(m[1], T(lang, MsgTLSRecordSplit)) {
				t.Errorf("the line should name the decoy and no split: %v", m)
			}
			if got := statusOf(t, h); !got.AntiDPI {
				t.Error("status.json does not say antiDPI while connected with a decoy name saved")
			}

			// Off again: hidden, and status.json agrees.
			res, _ = h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"0"}})
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("switching off: status %d", res.StatusCode)
			}
			if got := statusOf(t, h); got.AntiDPI {
				t.Error("status.json still says antiDPI after the box was switched off")
			}
			_, body = h.get("/")
			if !strings.Contains(html.UnescapeString(body), `id="anti-dpi" hidden`) {
				t.Error("the anti-DPI line still shows after the box was switched off")
			}
		})
	}
}
