// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
	"github.com/xtls/xray-core/infra/conf"
)

// The fixtures here have the SHAPE of the two documents GitHub issue 7 was
// raised with, a v2rayN "Custom" config and a BPB ?app=xray array, and none of
// their values. Every address is example.invalid or an RFC 5737 address, every
// id and password is the invented one from fixtures_test.go.

// customConfig wraps outbounds in the surrounding sections a v2rayN custom
// config carries, so a test proves those sections are ignored rather than
// merely absent. The inbound, dns and routing values are ones this box would
// never accept from pasted text.
func customConfig(remarks string, outbounds ...string) string {
	quoted, _ := json.Marshal(remarks)
	return fmt.Sprintf(`{
  "remarks": %s,
  "version": {"min": "26.2.6"},
  "log": {"loglevel": "debug", "access": "/tmp/pasted-access.log"},
  "policy": {"levels": {"8": {"handshake": 4}}},
  "inbounds": [{"tag": "socks", "port": 10808, "listen": "0.0.0.0", "protocol": "socks"}],
  "outbounds": [%s],
  "dns": {"servers": ["198.51.100.53"]},
  "routing": {"domainStrategy": "IPIfNonMatch", "rules": [{"type": "field", "outboundTag": "direct", "port": "0-65535"}]}
}`, quoted, strings.Join(outbounds, ",\n"))
}

const (
	freedomOut   = `{"tag": "direct", "protocol": "freedom", "settings": {}}`
	blackholeOut = `{"tag": "block", "protocol": "blackhole", "settings": {}}`
	dnsOut       = `{"tag": "dns-out", "protocol": "dns"}`
)

// vlessVnextTCPHTTP is the v2rayN shape: vless in the vnext form over raw TCP
// with HTTP header obfuscation, request and response headers both set.
func vlessVnextTCPHTTP(id string) string {
	return `{
  "tag": "proxy", "protocol": "vless",
  "settings": {"vnext": [{"address": "` + fakeHost + `", "port": 8080,
    "users": [{"id": "` + id + `", "encryption": "none", "level": 8}]}]},
  "streamSettings": {"network": "tcp", "tcpSettings": {"header": {"type": "http",
    "request": {"version": "1.1", "method": "GET", "path": ["/"],
      "headers": {"Host": ["www.example.invalid"], "Accept-Encoding": ["gzip, deflate"], "Connection": ["keep-alive"], "Pragma": "no-cache"}},
    "response": {"version": "1.1", "status": "200", "reason": "OK",
      "headers": {"Content-Type": ["application/octet-stream"], "Connection": ["keep-alive"], "Pragma": "no-cache"}}}}}
}`
}

// bpbProxy is the BPB shape: vless in the vnext form or trojan in the servers
// form, over websocket and TLS with a mixed-case server name, a fingerprint,
// ALPN, a sockopt with happy eyeballs, and a finalmask ClientHello fragment.
func bpbProxy(protocol string) string {
	settings := `{"vnext": [{"address": "` + fakeHost + `", "port": 443, "users": [{"id": "` + fakeUUID + `", "encryption": "none"}]}]}`
	if protocol == "trojan" {
		settings = `{"servers": [{"address": "` + fakeHost + `", "port": 443, "password": "` + fakePassword + `"}]}`
	}
	return `{
  "protocol": "` + protocol + `", "settings": ` + settings + `,
  "streamSettings": {
    "network": "ws", "wsSettings": {"host": "Front.Example.Invalid", "path": "/ed=2560"},
    "security": "tls",
    "tlsSettings": {"serverName": "Front.Example.Invalid", "fingerprint": "chrome", "alpn": ["http/1.1"]},
    "sockopt": {"domainStrategy": "UseIP", "happyEyeballs": {"tryDelayMs": 250, "prioritizeIPv6": false, "interleave": 2, "maxConcurrentTry": 4}},
    "finalmask": {"tcp": [{"type": "fragment", "settings": {"packets": "tlshello", "length": "100-200", "delay": "1"}}]}
  },
  "tag": "proxy"
}`
}

// bpbFreedomProxy is the other BPB shape, measured in 2 of the 208 sample
// entries: the outbound tagged "proxy" is a freedom with a fragment, so the
// config sends traffic straight out and there is no server at all.
const bpbFreedomProxy = `{"tag": "proxy", "protocol": "freedom", "settings": {},
  "streamSettings": {"finalmask": {"tcp": [{"type": "fragment", "settings": {"packets": "tlshello", "length": "100-200", "delay": "1"}}]}}}`

// emitted parses raw, selects entry i, validates the document through the
// engine's own loader, and returns the one outbound as generic JSON.
func emitted(t *testing.T, raw string, i int) (*Link, map[string]any) {
	t.Helper()
	l, clamped, err := Select(raw, i)
	if err != nil {
		t.Fatalf("Select(%d): %v", i, err)
	}
	if clamped {
		t.Fatalf("Select(%d) clamped", i)
	}
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	if err := engine.Validate(b); err != nil {
		t.Fatalf("engine.Validate refused the emitted document: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc) != 1 {
		t.Errorf("the emitted document has sections %v, want outbounds only", keysOf(doc))
	}
	obs := doc["outbounds"].([]any)
	if len(obs) != 1 {
		t.Fatalf("the emitted document has %d outbounds, want exactly one", len(obs))
	}
	return l, obs[0].(map[string]any)
}

func keysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// path walks generic JSON by keys and array indices.
func path(v any, keys ...any) any {
	for _, k := range keys {
		switch key := k.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[key]
		case int:
			a, ok := v.([]any)
			if !ok || key >= len(a) {
				return nil
			}
			v = a[key]
		}
	}
	return v
}

// TestXrayJSONEveryServerFormBuildsFlat is the core of the change: every
// protocol, in the form a full config writes it, parses, reports its server,
// builds in the engine, and is emitted in the flat form only.
func TestXrayJSONEveryServerFormBuildsFlat(t *testing.T) {
	cases := []struct {
		name, outbound, protocol, secretKey, secret string
	}{
		{"vless vnext", `{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"` + fakeHost + `","port":443,"users":[{"id":"` + fakeUUID + `","encryption":"none","flow":"xtls-rprx-vision"}]}]},"streamSettings":{"network":"tcp","security":"tls"}}`,
			"vless", "id", fakeUUID},
		{"vmess vnext", `{"tag":"proxy","protocol":"vmess","settings":{"vnext":[{"address":"` + fakeHost + `","port":443,"users":[{"id":"` + fakeUUID + `","security":"auto"}]}]}}`,
			"vmess", "id", fakeUUID},
		{"trojan servers", `{"tag":"proxy","protocol":"trojan","settings":{"servers":[{"address":"` + fakeHost + `","port":443,"password":"` + fakePassword + `"}]}}`,
			"trojan", "password", fakePassword},
		{"shadowsocks servers", `{"tag":"proxy","protocol":"shadowsocks","settings":{"servers":[{"address":"` + fakeHost + `","port":443,"method":"aes-256-gcm","password":"` + fakePassword + `"}]}}`,
			"shadowsocks", "password", fakePassword},
		{"socks servers with a user", `{"tag":"proxy","protocol":"socks","settings":{"servers":[{"address":"` + fakeHost + `","port":443,"users":[{"user":"someone","pass":"` + fakePassword + `"}]}]}}`,
			"socks", "pass", fakePassword},
		{"vless flat", `{"tag":"proxy","protocol":"vless","settings":{"address":"` + fakeHost + `","port":443,"id":"` + fakeUUID + `","encryption":"none"}}`,
			"vless", "id", fakeUUID},
		{"hysteria flat", `{"tag":"proxy","protocol":"hysteria","settings":{"version":2,"address":"` + fakeHost + `","port":443},"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2,"auth":"` + fakeAuth + `"}}}`,
			"hysteria", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, ob := emitted(t, customConfig("an entry", c.outbound, freedomOut, blackholeOut), 0)
			if l.Protocol != c.protocol || l.Address != fakeHost || l.Port != 443 {
				t.Errorf("read %s to %s:%d, want %s to %s:443", l.Protocol, l.Address, l.Port, c.protocol, fakeHost)
			}
			s := ob["settings"].(map[string]any)
			for _, form := range []string{"vnext", "servers"} {
				if _, ok := s[form]; ok {
					t.Errorf("the emitted settings still carry %q; the flat form must be the only one", form)
				}
			}
			if s["address"] != fakeHost {
				t.Errorf("flat address = %v, want %s", s["address"], fakeHost)
			}
			if c.secretKey != "" && s[c.secretKey] != c.secret {
				t.Errorf("flat %s did not survive flattening", c.secretKey)
			}
			if ob["tag"] != OutboundTag {
				t.Errorf("tag = %v, want %s", ob["tag"], OutboundTag)
			}
		})
	}
}

// TestXrayJSONCustomConfigKeepsTheTCPHTTPHeader is the v2rayN sample's shape.
func TestXrayJSONCustomConfigKeepsTheTCPHTTPHeader(t *testing.T) {
	raw := customConfig("My server", vlessVnextTCPHTTP(fakeUUID), freedomOut, blackholeOut)
	list := mustParseAll(t, raw)
	if len(list.Entries) != 1 || list.Dropped != 0 {
		t.Fatalf("Entries=%d Dropped=%d, want 1 and 0", len(list.Entries), list.Dropped)
	}
	e := list.Entries[0]
	if e.Tag != "My server" || e.Protocol != "vless" || e.Network != "tcp" || e.Security != SecurityNone || e.Port != 8080 {
		t.Errorf("entry = %+v", e)
	}
	_, ob := emitted(t, raw, 0)
	if got := path(ob, "streamSettings", "tcpSettings", "header", "type"); got != "http" {
		t.Errorf("tcp header type = %v, want http", got)
	}
	if got := path(ob, "streamSettings", "tcpSettings", "header", "request", "headers", "Host", 0); got != "www.example.invalid" {
		t.Errorf("request Host header = %v", got)
	}
	if got := path(ob, "streamSettings", "tcpSettings", "header", "response", "status"); got != "200" {
		t.Errorf("response status = %v", got)
	}
	if got := path(ob, "settings", "level"); fmt.Sprint(got) != "8" {
		t.Errorf("user level = %v, want 8", got)
	}
}

// TestXrayJSONBPBArrayKeepsStreamSettingsAsGiven is the BPB sample's shape:
// an array, remarks as tags, a freedom-only entry refused and counted, and
// the proxy's stream settings carried untouched.
func TestXrayJSONBPBArrayKeepsStreamSettingsAsGiven(t *testing.T) {
	raw := "[" + strings.Join([]string{
		customConfig("\u0633\u0631\u0648\u0631 1 - VLESS - tls", bpbProxy("vless"), dnsOut, freedomOut, blackholeOut),
		customConfig("\u0633\u0631\u0648\u0631 2 - Trojan - tls", bpbProxy("trojan"), dnsOut, freedomOut, blackholeOut),
		customConfig("\u0633\u0631\u0648\u0631 Fragment only", bpbFreedomProxy, dnsOut, freedomOut, blackholeOut),
		customConfig("\u0633\u0631\u0648\u0631 4 - VLESS again", bpbProxy("vless"), freedomOut),
	}, ",") + "]"

	list := mustParseAll(t, raw)
	if len(list.Entries) != 3 || list.Dropped != 1 {
		t.Fatalf("Entries=%d Dropped=%d, want 3 and 1", len(list.Entries), list.Dropped)
	}
	// The refused entry keeps its slot, so the one after it is still index 3.
	for i, want := range []int{0, 1, 3} {
		if list.Entries[i].Index != want {
			t.Errorf("Entries[%d].Index = %d, want %d", i, list.Entries[i].Index, want)
		}
	}
	if list.Entries[1].Tag != "\u0633\u0631\u0648\u0631 2 - Trojan - tls" {
		t.Errorf("tag from remarks = %q", list.Entries[1].Tag)
	}
	if _, _, err := Select(raw, 2); !errors.Is(err, ErrNoProxyOutbound) {
		t.Errorf("Select of the freedom-only entry returned %v, want ErrNoProxyOutbound", err)
	}

	for _, i := range []int{0, 1} {
		l, ob := emitted(t, raw, i)
		if l.ServerName != "Front.Example.Invalid" || l.Fingerprint != "chrome" || l.Network != "websocket" {
			t.Errorf("entry %d: sni %q fingerprint %q network %q", i, l.ServerName, l.Fingerprint, l.Network)
		}
		ss := ob["streamSettings"]
		if got := path(ss, "tlsSettings", "serverName"); got != "Front.Example.Invalid" {
			t.Errorf("entry %d: serverName = %v, want the mixed case kept", i, got)
		}
		if got := path(ss, "tlsSettings", "alpn", 0); got != "http/1.1" {
			t.Errorf("entry %d: alpn = %v", i, got)
		}
		if got := path(ss, "finalmask", "tcp", 0, "type"); got != "fragment" {
			t.Errorf("entry %d: finalmask tcp[0].type = %v, want fragment", i, got)
		}
		if got := path(ss, "finalmask", "tcp", 0, "settings", "packets"); got != "tlshello" {
			t.Errorf("entry %d: finalmask packets = %v, want tlshello", i, got)
		}
		if got := path(ss, "sockopt", "happyEyeballs", "tryDelayMs"); fmt.Sprint(got) != "250" {
			t.Errorf("entry %d: sockopt.happyEyeballs.tryDelayMs = %v", i, got)
		}
	}
}

// TestXrayJSONRemarksAreCleanedLikeEveryOtherTag holds remarks to cleanTag.
func TestXrayJSONRemarksAreCleanedLikeEveryOtherTag(t *testing.T) {
	long := "\x01red" + strings.Repeat("م", 60)
	list := mustParseAll(t, customConfig(long, vlessVnextTCPHTTP(fakeUUID)))
	got := list.Entries[0].Tag
	if strings.ContainsRune(got, 0x01) || len(got) > maxTagBytes || got != cleanTag(long) {
		t.Errorf("tag = %q, want cleanTag of the remarks", got)
	}
	l := mustParse(t, customConfig(long, vlessVnextTCPHTTP(fakeUUID)))
	if strings.ContainsRune(l.Redacted(), 0x01) {
		t.Error("Redacted carries a raw control byte from remarks")
	}
}

// TestXrayJSONPicksTheProxy covers the choice of outbound.
func TestXrayJSONPicksTheProxy(t *testing.T) {
	other := strings.Replace(vlessVnextTCPHTTP(fakeUUID), `"tag": "proxy"`, `"tag": "backup"`, 1)
	other = strings.Replace(other, fakeHost, "198.51.100.7", 1)

	// "proxy" wins over an earlier proxy-protocol outbound.
	l := mustParse(t, customConfig("x", other, vlessVnextTCPHTTP(fakeUUID)))
	if l.Address != fakeHost {
		t.Errorf("chose %s, want the outbound tagged proxy", l.Address)
	}
	// Without a "proxy" tag, the first proxy-protocol outbound, after
	// non-proxy ones.
	l = mustParse(t, customConfig("x", freedomOut, other))
	if l.Address != "198.51.100.7" {
		t.Errorf("chose %s, want the first proxy-protocol outbound", l.Address)
	}
	// A "proxy" that is a freedom is passed over.
	l = mustParse(t, customConfig("x", bpbFreedomProxy, other))
	if l.Address != "198.51.100.7" {
		t.Errorf("chose %s, want the vless outbound after the freedom tagged proxy", l.Address)
	}
}

// TestXrayJSONRefusals: every refusal names a sentinel, and none quotes the
// pasted text.
func TestXrayJSONRefusals(t *testing.T) {
	chained := strings.Replace(bpbProxy("vless"), `"sockopt": {`, `"sockopt": {"dialerProxy": "direct", `, 1)
	proxySettings := strings.Replace(bpbProxy("trojan"), `"tag": "proxy"`, `"tag": "proxy", "proxySettings": {"tag": "fragment-hop"}`, 1)
	twoServers := `{"tag":"proxy","protocol":"trojan","settings":{"servers":[{"address":"` + fakeHost + `","port":443,"password":"` + fakePassword + `"},{"address":"203.0.113.9","port":443,"password":"` + fakePassword + `"}]}}`
	twoUsers := `{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"` + fakeHost + `","port":443,"users":[{"id":"` + fakeUUID + `"},{"id":"` + fakeUUID + `"}]}]}}`
	noPort := strings.Replace(vlessVnextTCPHTTP(fakeUUID), `"port": 8080`, `"port": 0`, 1)
	removed := strings.Replace(vlessVnextTCPHTTP(fakeUUID), `"network": "tcp"`, `"network": "quic"`, 1)
	badReality := `{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"` + fakeHost + `","port":443,"users":[{"id":"` + fakeUUID + `"}]}]},"streamSettings":{"security":"reality","realitySettings":{"serverName":"` + fakeSNI + `","publicKey":"` + fakePublicKey() + `","shortId":"not-hex-` + fakePassword + `"}}}`

	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"only freedom and blackhole", customConfig("x", freedomOut, blackholeOut), ErrNoProxyOutbound},
		{"only an unsupported protocol", customConfig("x", `{"tag":"proxy","protocol":"wireguard","settings":{"secretKey":"`+fakePassword+`"}}`), ErrNoProxyOutbound},
		{"no outbounds at all", `{"remarks":"x","inbounds":[]}`, ErrNoProxyOutbound},
		{"dialerProxy chain", customConfig("x", chained, `{"tag":"fragment","protocol":"freedom","settings":{"fragment":{"packets":"tlshello","length":"100-200","interval":"1"}}}`), ErrChainedOutbound},
		{"proxySettings chain", customConfig("x", proxySettings), ErrChainedOutbound},
		{"two servers", customConfig("x", twoServers), ErrManyServers},
		{"two users", customConfig("x", twoUsers), ErrManyServers},
		{"bad UUID under vnext", customConfig("x", vlessVnextTCPHTTP(truncatedUUID)), ErrBadUUID},
		{"port zero under vnext", customConfig("x", noPort), ErrBadPort},
		{"removed transport", customConfig("x", removed), ErrUnsupportedTransport},
		{"malformed REALITY under vnext", customConfig("x", badReality), ErrBadReality},
		{"truncated JSON carrying a secret", `{"outbounds":[{"protocol":"trojan","settings":{"password":"` + fakePassword + `"`, ErrNoLink},
		{"array of non-objects", `["` + fakePassword + `", 7]`, ErrNoLink},
		{"empty array", `[]`, ErrNoLink},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseAll(c.raw)
			if !errors.Is(err, c.want) {
				t.Fatalf("ParseAll returned %v, want %v", err, c.want)
			}
			for _, secret := range append(secretsIn(), fakeHost, "fragment-hop", "direct\"", "not-hex") {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("the refusal quotes pasted text %q: %v", secret, err)
				}
			}
		})
	}
}

// TestXrayJSONCorrectionsStillRun: the corrections fill makes on share links
// are made on a JSON-sourced outbound too.
func TestXrayJSONCorrectionsStillRun(t *testing.T) {
	// A trojan outbound with no stream settings at all gets TLS and a server
	// name, exactly as a query-less trojan link does.
	bare := `{"tag":"proxy","protocol":"trojan","sendThrough":"` + fakeSNI + `","settings":{"servers":[{"address":"` + fakeHost + `","port":443,"password":"` + fakePassword + `"}]}}`
	l, ob := emitted(t, customConfig("bare trojan", bare), 0)
	if l.Security != SecurityTLS || path(ob, "streamSettings", "security") != "tls" {
		t.Errorf("trojan security = %s, want tls", l.Security)
	}
	if got := path(ob, "streamSettings", "tlsSettings", "serverName"); got != fakeHost {
		t.Errorf("serverName = %v, want the address filled in", got)
	}
	if _, ok := ob["sendThrough"]; ok {
		t.Error("the pasted sendThrough reached the engine document")
	}
	if l.Tag != "bare trojan" {
		t.Errorf("tag = %q, want the remarks, not the sendThrough value", l.Tag)
	}
}

// TestXrayJSONVLESSEncryptionSurvivesFlattening is the vnext twin of
// TestVLESSEncryptionSurvivesIntoTheEngineDocument: flattening rebuilds the
// settings, so this is the one place the parameter could be dropped.
func TestXrayJSONVLESSEncryptionSurvivesFlattening(t *testing.T) {
	const enc = "mlkem768x25519plus.random.1rtt.100-111-1111.75-0-111.50-0-3333.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ob := `{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"` + fakeHost + `","port":443,"users":[{"id":"` + fakeUUID + `","encryption":"` + enc + `"}]}]},"streamSettings":{"network":"ws","security":"none"}}`
	_, got := emitted(t, customConfig("x", ob), 0)
	if path(got, "settings", "encryption") != enc {
		t.Errorf("encryption after flattening = %v", path(got, "settings", "encryption"))
	}
}

// TestXrayJSONThroughLoopback is why flattening is a rewrite and not only a
// read: ThroughLoopback overwrites the flat address, and on a vnext outbound
// that would leave the id behind in vnext.
func TestXrayJSONThroughLoopback(t *testing.T) {
	l := mustParse(t, customConfig("x", vlessVnextTCPHTTP(fakeUUID)))
	fwd, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345"))
	if err != nil {
		t.Fatalf("ThroughLoopback: %v", err)
	}
	b, err := fwd.XrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(b); err != nil {
		t.Fatalf("the forwarded document does not load: %v", err)
	}
	if fwd.Address != "127.0.0.1" || !strings.Contains(string(b), fakeUUID) {
		t.Errorf("forwarded to %s, id kept %v", fwd.Address, strings.Contains(string(b), fakeUUID))
	}
}

// TestXrayJSONFlattenRefusesEachMalformedShape drives flattenSettings through
// every protocol's refusal branches directly. Each shape is one a pasted
// document can carry and the engine would reject later, or worse, accept with
// the wrong server: settings of the wrong type, more than one server or user,
// and a user entry that is not an object. The flat form must pass through
// untouched, because that is the form every share link already produces.
func TestXrayJSONFlattenRefusesEachMalformedShape(t *testing.T) {
	srv := `"address":"` + fakeHost + `","port":443`
	user := func(s string) string { return `{` + s + `}` }
	cases := []struct {
		protocol, settings string
		want               error
	}{
		{"vless", `7`, ErrNoLink},
		{"vless", `{"vnext":[{` + srv + `,"users":[7]}]}`, ErrNoLink},
		{"vless", `{"vnext":[{` + srv + `,"users":[]}]}`, ErrManyServers},
		{"vmess", `7`, ErrNoLink},
		{"vmess", `{"vnext":[{` + srv + `,"users":[7]}]}`, ErrNoLink},
		{"vmess", `{"vnext":[{` + srv + `,"users":[` + user(`"id":"`+fakeUUID+`"`) + `,` + user(`"id":"`+fakeUUID+`"`) + `]}]}`, ErrManyServers},
		{"trojan", `7`, ErrNoLink},
		{"shadowsocks", `7`, ErrNoLink},
		{"shadowsocks", `{"servers":[{` + srv + `,"method":"aes-128-gcm","password":"` + fakePassword + `"},{` + srv + `,"method":"aes-128-gcm","password":"` + fakePassword + `"}]}`, ErrManyServers},
		{"socks", `7`, ErrNoLink},
		{"socks", `{"servers":[{` + srv + `},{` + srv + `}]}`, ErrManyServers},
		{"socks", `{"servers":[{` + srv + `,"users":[7]}]}`, ErrNoLink},
		{"socks", `{"servers":[{` + srv + `,"users":[` + user(`"user":"a"`) + `,` + user(`"user":"b"`) + `]}]}`, ErrManyServers},
	}
	for _, c := range cases {
		t.Run(c.protocol+" "+c.settings, func(t *testing.T) {
			msg := json.RawMessage(c.settings)
			ob := conf.OutboundDetourConfig{Protocol: c.protocol, Settings: &msg}
			if err := flattenSettings(&ob); !errors.Is(err, c.want) {
				t.Fatalf("flattenSettings returned %v, want %v", err, c.want)
			}
		})
	}

	for _, protocol := range []string{"vless", "vmess", "trojan", "shadowsocks", "socks", "hysteria"} {
		t.Run(protocol+" flat form is left alone", func(t *testing.T) {
			flat := `{"address":"` + fakeHost + `","port":443}`
			msg := json.RawMessage(flat)
			ob := conf.OutboundDetourConfig{Protocol: protocol, Settings: &msg}
			if err := flattenSettings(&ob); err != nil {
				t.Fatalf("flattenSettings refused the flat form: %v", err)
			}
			if string(*ob.Settings) != flat {
				t.Errorf("the flat form was rewritten to %s", *ob.Settings)
			}
		})
	}
	t.Run("no settings at all", func(t *testing.T) {
		ob := conf.OutboundDetourConfig{Protocol: "vless"}
		if err := flattenSettings(&ob); err != nil || ob.Settings != nil {
			t.Fatalf("flattenSettings on nil settings: err %v, settings %v", err, ob.Settings)
		}
	})
}

// TestXrayJSONSocksServerWithoutUsersFlattens covers the one server form that
// carries no credential: a socks server with no users list is a valid
// anonymous proxy and must keep its address and port.
func TestXrayJSONSocksServerWithoutUsersFlattens(t *testing.T) {
	msg := json.RawMessage(`{"servers":[{"address":"` + fakeHost + `","port":1080}]}`)
	ob := conf.OutboundDetourConfig{Protocol: "socks", Settings: &msg}
	if err := flattenSettings(&ob); err != nil {
		t.Fatalf("flattenSettings: %v", err)
	}
	var got struct {
		Address string `json:"address"`
		Port    uint16 `json:"port"`
	}
	if err := json.Unmarshal(*ob.Settings, &got); err != nil || got.Address != fakeHost || got.Port != 1080 {
		t.Fatalf("flattened socks settings %s, decode err %v", *ob.Settings, err)
	}
}

// TestXrayJSONDocumentLevelRefusals covers the refusals that happen before an
// outbound is chosen or after it is chosen and cannot be decoded, and one
// that fill makes on a protocol with no server list to flatten.
func TestXrayJSONDocumentLevelRefusals(t *testing.T) {
	cases := []struct {
		name, raw string
		want      error
	}{
		{"array that is not valid JSON", `[{"outbounds":[` + bpbProxy("vless") + `]}`, ErrNoLink},
		{"proxy whose stream settings are the wrong type", customConfig("x", `{"tag":"proxy","protocol":"vless","streamSettings":7}`), ErrNoLink},
		{"hysteria whose address is not a string", customConfig("x", `{"tag":"proxy","protocol":"hysteria","settings":{"address":7,"port":443}}`), ErrNoLink},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ParseAll(c.raw); !errors.Is(err, c.want) {
				t.Fatalf("ParseAll returned %v, want %v", err, c.want)
			}
		})
	}
}
