// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
)

var (
	pinV4 = netip.MustParseAddr("203.0.113.10")
	pinV6 = netip.MustParseAddr("2001:db8::10")
)

// proxyOf returns the proxy outbound of a built document as generic JSON, for
// comparing two documents field by field.
func proxyOf(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var d struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, ob := range d.Outbounds {
		if ob["tag"] == TagProxy {
			return ob
		}
	}
	t.Fatal("no proxy outbound")
	return nil
}

func hostsOf(t *testing.T, raw []byte) map[string][]string {
	t.Helper()
	var d struct {
		DNS struct {
			Hosts map[string][]string `json:"hosts"`
		} `json:"dns"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return d.DNS.Hosts
}

func sockoptStrategyOf(ob map[string]any) string {
	stream, _ := ob["streamSettings"].(map[string]any)
	sockopt, _ := stream["sockopt"].(map[string]any)
	s, _ := sockopt["domainStrategy"].(string)
	return s
}

// withoutSockopt returns a deep copy of an outbound with the one key this
// change adds removed, and streamSettings removed too if the change created it.
func withoutSockopt(ob map[string]any, hadStream bool) map[string]any {
	b, _ := json.Marshal(ob)
	var c map[string]any
	_ = json.Unmarshal(b, &c)
	stream, _ := c["streamSettings"].(map[string]any)
	delete(stream, "sockopt")
	if !hadStream {
		delete(c, "streamSettings")
	}
	return c
}

// TestAnIPLiteralServerIsByteIdenticalWithOrWithoutPinnedAddresses: the change
// must not touch a link that already names its server by address.
func TestAnIPLiteralServerIsByteIdenticalWithOrWithoutPinnedAddresses(t *testing.T) {
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			raw := f.raw()
			if !strings.Contains(raw, fakeHost) {
				t.Skip("this fixture's server is not spelled as fakeHost, so it cannot be turned into an IP literal here")
			}
			l := mustParse(t, strings.Replace(raw, fakeHost, "198.51.100.1", 1))
			if _, err := netip.ParseAddr(l.Address); err != nil {
				t.Fatalf("the substitution did not produce an IP-literal server: %q", l.Address)
			}
			plain, err := Build(Options{Link: l})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			pinned, err := Build(Options{Link: l, PinnedServer: []netip.Addr{pinV4, pinV6}})
			if err != nil {
				t.Fatalf("Build with PinnedServer: %v", err)
			}
			if !bytes.Equal(plain, pinned) {
				t.Fatalf("an IP-literal document changed when pinned addresses were supplied")
			}
		})
	}
}

// TestADomainServerIsPinnedAndNothingElseInTheOutboundChanges covers every
// protocol fixture: dns.hosts maps exactly the server name to the pinned
// addresses, the proxy outbound dials by ForceIP, and apart from that one
// sockopt key the outbound is the one internal/link emitted, so the address,
// the TLS or REALITY server name, the websocket Host and every credential are
// untouched.
func TestADomainServerIsPinnedAndNothingElseInTheOutboundChanges(t *testing.T) {
	checked := 0
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			l := mustParse(t, f.raw())
			if _, err := netip.ParseAddr(l.Address); err == nil {
				t.Skip("an IP-literal fixture; covered by the test above")
			}
			plainRaw, err := Build(Options{Link: l})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			pinnedRaw, err := Build(Options{Link: l, PinnedServer: []netip.Addr{pinV4, pinV6}})
			if err != nil {
				t.Fatalf("Build with PinnedServer: %v", err)
			}
			if err := engine.Validate(pinnedRaw); err != nil {
				t.Fatalf("the engine refused the pinned document: %v", err)
			}

			hosts := hostsOf(t, pinnedRaw)
			want := map[string][]string{"full:" + strings.ToLower(l.Address): {pinV4.String(), pinV6.String()}}
			if !reflect.DeepEqual(hosts, want) {
				t.Fatalf("dns.hosts = %v, want %v", hosts, want)
			}
			if hostsOf(t, plainRaw) != nil {
				t.Fatal("the unpinned document carries dns.hosts")
			}

			plain, pinned := proxyOf(t, plainRaw), proxyOf(t, pinnedRaw)
			if got := sockoptStrategyOf(pinned); got != "ForceIP" {
				t.Fatalf("sockopt.domainStrategy = %q, want ForceIP", got)
			}
			if got := sockoptStrategyOf(plain); got != "" {
				t.Fatalf("the unpinned outbound already carries sockopt.domainStrategy %q", got)
			}
			_, hadStream := plain["streamSettings"]
			if !reflect.DeepEqual(withoutSockopt(pinned, hadStream), plain) {
				t.Fatalf("pinning changed the proxy outbound beyond sockopt.domainStrategy:\nunpinned: %v\npinned:   %v",
					plain, pinned)
			}
			checked++
		})
	}
	if checked == 0 {
		t.Fatal("no fixture names its server by a domain; the test checked nothing")
	}
}

// TestTheServerNameSurvivesPinning names the fields directly, because "the
// outbound is otherwise unchanged" is a comparison and a reader should not have
// to trust it to know that SNI and Host were looked at.
func TestTheServerNameSurvivesPinning(t *testing.T) {
	for _, c := range []struct {
		name, raw string
		path      []string
		want      string
	}{
		{"tls serverName", vlessTLSWebsocketLink(), []string{"streamSettings", "tlsSettings", "serverName"}, "cdn.fake.invalid"},
		{"websocket host", vlessTLSWebsocketLink(), []string{"streamSettings", "wsSettings", "host"}, "cdn.fake.invalid"},
		{"reality serverName", vlessRealityLink(), []string{"streamSettings", "realitySettings", "serverName"}, fakeSNI},
		// A trojan link with no query at all: no sni, so this is the name
		// link.fillMissingServerName filled in from the address, and the one
		// most exposed to an address change.
		{"filled tls serverName", "trojan://" + fakePassword + "@" + fakeHost + ":443#Trojan%20box",
			[]string{"streamSettings", "tlsSettings", "serverName"}, fakeHost},
	} {
		t.Run(c.name, func(t *testing.T) {
			raw, err := Build(Options{Link: mustParse(t, c.raw), PinnedServer: []netip.Addr{pinV4}})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			var v any = proxyOf(t, raw)
			for _, k := range c.path {
				m, _ := v.(map[string]any)
				v = m[k]
			}
			if v != c.want {
				t.Fatalf("%s = %v after pinning, want %q", strings.Join(c.path, "."), v, c.want)
			}
		})
	}
}

// TestPinnedServerIsTheOnlyWayTheProxyDialConsultsTheDNSApp holds the fifth
// path in the DNS.Intercept enumeration to its condition: the proxy outbound's
// sockopt carries a domainStrategy if and only if a domain server has pinned
// addresses. An upstream SOCKS5 adds sockopt.dialerProxy, which hands the name
// to the front proxy and never consults the DNS app.
func TestPinnedServerIsTheOnlyWayTheProxyDialConsultsTheDNSApp(t *testing.T) {
	combos := combinations()
	for _, f := range fixtures() {
		l := mustParse(t, f.raw())
		_, ipErr := netip.ParseAddr(l.Address)
		for _, c := range combos {
			o := c.opts
			o.Link = l
			raw, err := Build(o)
			if err != nil {
				t.Fatalf("%s/%s: Build: %v", f.name, c.name, err)
			}
			// The proxy outbound only: the direct outbound can carry a sockopt
			// with an interface binding (Direct.Interface), which never
			// consults the DNS app. The ForceIP check keeps the whole
			// document to the same condition.
			stream, _ := proxyOf(t, raw)["streamSettings"].(map[string]any)
			sockopt, _ := stream["sockopt"].(map[string]any)
			_, has := sockopt["domainStrategy"]
			forced := strings.Contains(string(raw), `"ForceIP"`)
			want := ipErr != nil && len(o.PinnedServer) > 0
			if has != want || forced != want {
				t.Fatalf("%s/%s: proxy sockopt.domainStrategy present is %v and ForceIP present is %v, want %v",
					f.name, c.name, has, forced, want)
			}
		}
	}
}

// TestPinnedServerRefusals: the two inputs pinServerName refuses rather than
// emit a document that could not dial.
func TestPinnedServerRefusals(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	if _, err := Build(Options{Link: l, PinnedServer: []netip.Addr{{}}}); !errors.Is(err, ErrPinnedServerAddress) {
		t.Errorf("a zero address: got %v, want ErrPinnedServerAddress", err)
	}
	if _, err := Build(Options{Link: l, DNS: DNS{Strategy: QueryUseIPv4}, PinnedServer: []netip.Addr{pinV6}}); !errors.Is(err, ErrPinnedServerFamily) {
		t.Errorf("IPv6 only under UseIPv4: got %v, want ErrPinnedServerFamily", err)
	}
	if _, err := Build(Options{Link: l, DNS: DNS{Strategy: QueryUseIPv6}, PinnedServer: []netip.Addr{pinV4}}); !errors.Is(err, ErrPinnedServerFamily) {
		t.Errorf("IPv4 only under UseIPv6: got %v, want ErrPinnedServerFamily", err)
	}
	if _, err := Build(Options{Link: l, DNS: DNS{Strategy: QueryUseIPv4}, PinnedServer: []netip.Addr{pinV6, pinV4}}); err != nil {
		t.Errorf("one usable family among two was refused: %v", err)
	}
}

// TestPinnedAddressesAreUnmappedAndDeduplicated: a v4-mapped v6 spelling of an
// address the list already holds is the same address, and the engine should
// see it once.
func TestPinnedAddressesAreUnmappedAndDeduplicated(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	mapped := netip.AddrFrom16(pinV4.As16())
	raw, err := Build(Options{Link: l, PinnedServer: []netip.Addr{pinV4, mapped, pinV4}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := hostsOf(t, raw)["full:"+fakeHost]
	if !reflect.DeepEqual(got, []string{pinV4.String()}) {
		t.Fatalf("dns.hosts entry = %v, want [%s]", got, pinV4)
	}
}

// TestTheHostsKeyIsTheNameTheEngineWillMatch: the engine lowercases and strips
// one trailing dot from the query before matching (app/dns/dns.go:217,
// hosts.go:98), so the key is written in that form.
func TestTheHostsKeyIsTheNameTheEngineWillMatch(t *testing.T) {
	for _, c := range []struct{ address, want string }{
		{"Proxy.Example.INVALID", "proxy.example.invalid"},
		{"proxy.example.invalid.", "proxy.example.invalid"},
		{"203.0.113.9", ""},
		{"2001:db8::9", ""},
		{"[2001:db8::9]", ""},
		{"", ""},
	} {
		got := serverNameOf(Options{Link: &link.Link{Address: c.address}})
		if got != c.want {
			t.Errorf("serverNameOf(%q) = %q, want %q", c.address, got, c.want)
		}
	}
	if got := serverNameOf(Options{}); got != "" {
		t.Errorf("serverNameOf with no link = %q, want empty", got)
	}
}

// TestPinServerNameRefusesAnOutboundItCannotRead: the outbound bytes come from
// internal/link and are always decodable from Build, so this is reached only by
// calling the helper directly. It is here so that a future caller handing it
// something else gets a refusal rather than a document missing its sockopt.
func TestPinServerNameRefusesAnOutboundItCannotRead(t *testing.T) {
	o := Options{Link: &link.Link{Address: "proxy.example.invalid"}, PinnedServer: []netip.Addr{pinV4}}
	o = o.normalise()
	if _, _, err := pinServerName(o, json.RawMessage(`not json`)); err == nil {
		t.Fatal("an undecodable outbound was accepted")
	}
}

// TestForceIPKeepsAnExistingSockopt: a pasted raw JSON outbound can carry a
// sockopt of its own. Every key in it is kept and only domainStrategy is
// replaced, because any other value lets the dial reach the system resolver.
func TestForceIPKeepsAnExistingSockopt(t *testing.T) {
	in := []byte(`{"protocol":"vless","streamSettings":{"network":"raw","sockopt":{"domainStrategy":"UseIP","mark":255,"tcpFastOpen":true}},"tag":"proxy"}`)
	out, err := forceIPDial(in)
	if err != nil {
		t.Fatalf("forceIPDial: %v", err)
	}
	want := `{"protocol":"vless","streamSettings":{"network":"raw","sockopt":{"domainStrategy":"ForceIP","mark":255,"tcpFastOpen":true}},"tag":"proxy"}`
	if string(out) != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
	if _, err := forceIPDial([]byte(`not json`)); err == nil {
		t.Fatal("undecodable bytes were accepted")
	}
}
