// SPDX-License-Identifier: AGPL-3.0-or-later
package panel

import (
	"caspianbyoc.org/caspian/internal/state"
	"net/url"
	"strings"
	"testing"
)

func TestCountryRecoveryAndSavedOverride(t *testing.T) {
	for _, lang := range Langs {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			h.ready()
			if h.store.Advanced().Country != "" {
				t.Fatal("default must remain automatic")
			}
			h.priv.FailStartWith(FaultCountryMissing)
			h.get("/?lang=" + string(lang))
			h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
			_, body := h.get("/")
			for _, want := range []string{T(lang, MsgCountryMissing), T(lang, MsgCountryAdvice), `href="/?advanced=1#country"`} {
				if !strings.Contains(body, want) {
					t.Fatalf("missing recovery instruction: %s", want)
				}
			}
			_, body = h.get("/?advanced=1")
			if !strings.Contains(body, `id="country" name="country" type="text"`) {
				t.Fatal("country is not editable")
			}
			res, _ := h.postForm("/advanced", url.Values{"csrf": {h.tokenOn("/?advanced=1")}, "country": {"ie"}})
			if res.StatusCode != 303 {
				t.Fatalf("save: %d", res.StatusCode)
			}
			reloaded, err := state.Load(h.store.Dir())
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.Advanced().Country != "IE" {
				t.Fatal("country did not survive store reload")
			}
			h.priv.FailStartWith(FaultNone)
			h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
			starts := h.priv.Starts()
			if len(starts) == 0 || starts[len(starts)-1].Hotspot.Country != "IE" {
				t.Fatal("retry did not carry saved country")
			}
			h.postForm("/advanced", url.Values{"csrf": {h.tokenOn("/?advanced=1")}, "country": {""}})
			if h.store.Advanced().Country != "" {
				t.Fatal("clearing country did not restore automatic mode")
			}
		})
	}
}
