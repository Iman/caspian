// SPDX-License-Identifier: AGPL-3.0-or-later
package panel

import (
	"html"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestRecoveryPreservesSettingsAndReportsOutcome(t *testing.T) {
	for _, fails := range []bool{false, true} {
		name := "success"
		if fails {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.ready()
			before := h.store.Snapshot()
			if fails {
				h.priv.SetRecoverError(&FaultError{Fault: FaultUnavailable})
			}
			res, _ := h.postForm("/recover", url.Values{"csrf": {h.tokenOn("/")}})
			if res.StatusCode != http.StatusSeeOther || h.priv.Recovers() != 1 {
				t.Fatal("recovery did not run exactly once and redirect")
			}
			if !reflect.DeepEqual(before, h.store.Snapshot()) {
				t.Fatal("recovery changed saved settings")
			}
			_, body := h.get("/")
			body = html.UnescapeString(body)
			if fails {
				if len(h.priv.Starts()) != 0 {
					t.Error("failed recovery started the tunnel")
				}
				if !strings.Contains(body, h.msg(MsgFaultUnavailable)) {
					t.Error("recovery failure was not shown")
				}
			} else {
				if len(h.priv.Starts()) != 1 {
					t.Error("recovery did not restart the tunnel")
				}
				if !strings.Contains(body, h.msg(MsgNoticeRecovered)) {
					t.Error("recovery confirmation was not shown")
				}
			}
			for _, secret := range []string{testPassword, before.Proxy.Raw.Reveal(), before.Hotspot.Passphrase.Reveal()} {
				if strings.Contains(h.logs.String(), secret) {
					t.Error("recovery logged a credential")
				}
			}
		})
	}
}

func TestRecoveryRejectsBadConfigAndUntrustedRequests(t *testing.T) {
	for _, mode := range []string{"no session", "no csrf", "bad config"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			h.ready()
			form := url.Values{"csrf": {h.tokenOn("/")}}
			switch mode {
			case "no session":
				h.signedOut()
			case "no csrf":
				form.Del("csrf")
			case "bad config":
				if err := h.store.SetProxyConfig("invalid-test-config", "vless", "test"); err != nil {
					t.Fatal(err)
				}
			}
			res, _ := h.postForm("/recover", form)
			if h.priv.Recovers() != 0 || len(h.priv.Starts()) != 0 {
				t.Fatal("invalid request reached recovery")
			}
			if mode == "no csrf" && res.StatusCode != http.StatusForbidden {
				t.Error("missing CSRF token was not refused")
			}
			if mode == "no session" && res.Header.Get("Location") != "/login" {
				t.Error("unauthenticated recovery did not require login")
			}
		})
	}
}
