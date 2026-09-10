// SPDX-License-Identifier: AGPL-3.0-or-later
package panel

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestSNIFormPreservesRealConfigAndReconnects(t *testing.T) {
	h := newHarness(t)
	raw := entryLink("vless", entryHostA, "test")
	h.readyWith(raw)
	h.switchOn()
	for _, name := range []string{" COVER.Example.Invalid. ", ""} {
		res, _ := h.postForm("/sni", url.Values{"csrf": {h.tokenOn("/")}, "spoof_sni": {name}})
		if res.StatusCode != http.StatusSeeOther {
			t.Fatal(res.StatusCode)
		}
		expected := "cover.example.invalid"
		if name == "" {
			expected = ""
		}
		if h.store.Proxy().SpoofSNI.Reveal() != expected || h.store.Proxy().Raw.Reveal() != raw {
			t.Fatal("incorrect stored fields")
		}
		starts := h.priv.Starts()
		last := starts[len(starts)-1]
		if last.SpoofSNI != expected || !strings.Contains(string(last.ConfigJSON), fakeSNIForPanel) {
			t.Fatal("start lost spoof name or real SNI")
		}
	}
	if len(h.priv.Starts()) != 3 {
		t.Fatal("changing SNI did not reconnect")
	}
}
func TestSNIFormRejectsBadInputAndCSRF(t *testing.T) {
	h := newHarness(t)
	h.readyWith(entryLink("vless", entryHostA, "test"))
	h.switchOn()
	for _, name := range []string{"https://cover.invalid", "127.0.0.1", "<script>alert(1)</script>", strings.Repeat("a", 220)} {
		h.postForm("/sni", url.Values{"csrf": {h.tokenOn("/")}, "spoof_sni": {name}})
		if h.store.Proxy().SpoofSNI.Reveal() != "" || len(h.priv.Starts()) != 1 {
			t.Fatal("bad input changed running configuration")
		}
	}
	res, _ := h.postForm("/sni", url.Values{"spoof_sni": {"cover.example.invalid"}})
	if res.StatusCode != http.StatusForbidden {
		t.Fatal("missing CSRF accepted", res.StatusCode)
	}
}
func TestSNIFormRejectsQUIC(t *testing.T) {
	h := newHarness(t)
	h.readyWith("hysteria2://not-a-real-password@a.example.invalid:443?sni=real.example.invalid")
	h.postForm("/sni", url.Values{"csrf": {h.tokenOn("/")}, "spoof_sni": {"cover.example.invalid"}})
	if h.store.Proxy().SpoofSNI.Reveal() != "" {
		t.Fatal("enabled for QUIC")
	}
}

func TestSNIFormRequiresAStoredConfig(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	h.postForm("/sni", url.Values{"csrf": {h.tokenOn("/")}, "spoof_sni": {"cover.example.invalid"}})
	if h.store.Proxy().SpoofSNI.Reveal() != "" || len(h.priv.Starts()) != 0 {
		t.Fatal("enabled SNI without config")
	}
}
func TestSNIFormSaveFailureLeavesTheRunningConfigUntouched(t *testing.T) {
	h := newHarness(t)
	h.readyWith(entryLink("vless", entryHostA, "test"))
	h.switchOn()
	token := h.tokenOn("/")
	before := h.store.Proxy()
	// Replace only this harness's temporary state file with a directory.
	if err := os.Remove(h.store.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(h.store.Path(), 0700); err != nil {
		t.Fatal(err)
	}
	h.postForm("/sni", url.Values{"csrf": {token}, "spoof_sni": {"cover.example.invalid"}})
	if h.store.Proxy() != before || len(h.priv.Starts()) != 1 {
		t.Fatal("failed save changed active settings")
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgSaveConfigFailed)) {
		t.Fatal("save failure hidden")
	}
}

func TestDPIFormKeepsOptionsIndependentAndReconnects(t *testing.T) {
	h := newHarness(t)
	raw := entryLink("vless", entryHostA, "test")
	h.readyWith(raw)
	h.switchOn()
	for mask := 0; mask < 8; mask++ {
		form := url.Values{"csrf": {h.tokenOn("/")}}
		if mask&1 != 0 {
			form.Set("spoof_sni", "cover.example.invalid")
		}
		if mask&2 != 0 {
			form.Set("tcp_split", "on")
		}
		if mask&4 != 0 {
			form.Set("tls_record_split", "on")
		}
		res, _ := h.postForm("/sni", form)
		if res.StatusCode != http.StatusSeeOther {
			t.Fatal(res.StatusCode)
		}
		p := h.store.Proxy()
		if p.TCPSplit != (mask&2 != 0) || p.TLSRecordSplit != (mask&4 != 0) || p.SpoofSNI.Reveal() != form.Get("spoof_sni") || p.Raw.Reveal() != raw {
			t.Fatal("form lost independent options")
		}
		starts := h.priv.Starts()
		last := starts[len(starts)-1]
		if last.TCPSplit != p.TCPSplit || last.TLSRecordSplit != p.TLSRecordSplit {
			t.Fatal("reconnect lost split settings")
		}
	}
	before := h.store.Proxy()
	starts := len(h.priv.Starts())
	for _, field := range []string{"tcp_split", "tls_record_split"} {
		h.postForm("/sni", url.Values{"csrf": {h.tokenOn("/")}, field: {"invalid"}})
		if h.store.Proxy() != before || len(h.priv.Starts()) != starts {
			t.Fatal("invalid flag changed configuration")
		}
	}
}
