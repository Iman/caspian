// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	share "caspianbyoc.org/caspian/third_party/libxray-share"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/infra/conf"
	xraytls "github.com/xtls/xray-core/transport/internet/tls"
)

// These tests assert on the GENERATED CONFIG, not on a re-emitted URI. A URI
// round trip cannot catch a mapping error, because a parser that misreads a
// field re-emits it identically and passes.

// --- helpers ---------------------------------------------------------------

func mustParse(t *testing.T, raw string) *Link {
	t.Helper()
	l, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return l
}

// configMap returns the emitted config as generic JSON, so that assertions are
// made against the actual key names in the document rather than against Go
// struct fields that could be renamed without the document changing.
func configMap(t *testing.T, l *Link) map[string]any {
	t.Helper()
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("emitted config is not JSON: %v", err)
	}
	return m
}

// dig walks the generic JSON by key, treating a numeric key as an array index.
func dig(t *testing.T, m any, path ...string) any {
	t.Helper()
	cur := m
	for i, p := range path {
		switch v := cur.(type) {
		case map[string]any:
			got, ok := v[p]
			if !ok {
				t.Fatalf("key %q missing at %v", p, path[:i+1])
			}
			cur = got
		case []any:
			var idx int
			if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx >= len(v) {
				t.Fatalf("bad index %q at %v", p, path[:i+1])
			}
			cur = v[idx]
		default:
			t.Fatalf("cannot descend into %T at %v", cur, path[:i+1])
		}
	}
	return cur
}

// assertAbsentOrNull passes when the key is missing or holds JSON null. The
// emitted document drops null keys, so a cleared field shows up as an absent
// key rather than a null one, and both are the same thing to the engine.
func assertAbsentOrNull(t *testing.T, m any, path ...string) {
	t.Helper()
	parent := dig(t, m, path[:len(path)-1]...)
	obj, ok := parent.(map[string]any)
	if !ok {
		t.Fatalf("%v is %T, want an object", path[:len(path)-1], parent)
	}
	last := path[len(path)-1]
	if v, present := obj[last]; present && v != nil {
		t.Errorf("%v = %v, want absent or null", path, v)
	}
}

func digString(t *testing.T, m any, path ...string) string {
	t.Helper()
	v := dig(t, m, path...)
	s, ok := v.(string)
	if !ok {
		t.Fatalf("%v is %T, want string", path, v)
	}
	return s
}

// buildConfig decodes the emitted document exactly as the engine's loader
// would and runs Build on it. Build is the engine's own acceptance step, so a
// pass here means the document the engine will read is one it can turn into a
// running outbound.
func buildConfig(t *testing.T, b []byte) error {
	t.Helper()
	var c conf.Config
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("emitted config does not decode into conf.Config: %v", err)
	}
	_, err := c.Build()
	return err
}

// --- the mapping table -----------------------------------------------------

// TestRealityURIParameterToConfigKey is the test the whole package exists for.
// Not one of the six URI parameter names matches its config key, and a link
// that loses one of them still connects to something, so the failure is silent.
//
// It fails if any single mapping is dropped (the value assertion) or renamed
// to the URI spelling (the absent-key assertion).
func TestRealityURIParameterToConfigKey(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	m := configMap(t, l)
	reality := dig(t, m, "outbounds", "0", "streamSettings", "realitySettings")

	mapping := []struct {
		uriParam  string
		configKey string
		want      string
	}{
		{"pbk", "publicKey", fakePublicKey()},
		{"sid", "shortId", fakeShortID},
		{"sni", "serverName", fakeSNI},
		{"fp", "fingerprint", "chrome"},
		{"spx", "spiderX", fakeSpiderX},
		{"pqv", "mldsa65Verify", fakeMldsa65Verify()},
	}
	for _, c := range mapping {
		t.Run(c.uriParam+"_to_"+c.configKey, func(t *testing.T) {
			got := digString(t, reality, c.configKey)
			if got != c.want {
				t.Errorf("realitySettings.%s: got %d chars, want the value of the %s parameter (%d chars)",
					c.configKey, len(got), c.uriParam, len(c.want))
			}
			// The URI spelling must not appear as a config key. The engine has
			// no DisallowUnknownFields anywhere, so a key by the wrong name is
			// accepted and its value dropped in silence.
			if rm, ok := reality.(map[string]any); ok {
				if _, present := rm[c.uriParam]; present {
					t.Errorf("realitySettings carries the URI spelling %q as a config key", c.uriParam)
				}
			}
		})
	}
}

// TestRealityConfigKeysStillExistInEngine pins the mapping to the engine
// version. The table above is only correct for the conf.REALITYConfig this
// module is built against, so when the engine moves this fails and the table
// gets rechecked instead of quietly rotting.
func TestRealityConfigKeysStillExistInEngine(t *testing.T) {
	want := []string{"publicKey", "shortId", "serverName", "fingerprint", "spiderX", "mldsa65Verify"}
	tags := map[string]bool{}
	rt := reflect.TypeOf(conf.REALITYConfig{})
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			tags[name] = true
		}
	}
	for _, k := range want {
		if !tags[k] {
			t.Errorf("conf.REALITYConfig no longer has a field tagged %q; recheck the mapping table", k)
		}
	}
}

// --- the sendThrough regression --------------------------------------------

// TestSendThroughIsClearedAndConfigBuilds shows the defect and the fix in one
// run: the vendored parser's own output fails to build, and the output of this
// package builds.
func TestSendThroughIsClearedAndConfigBuilds(t *testing.T) {
	raw := vlessRealityLink()

	// Before. The parser puts the #fragment in SendThrough
	// (third_party/libxray-share/xray_json.go:28-30) and the engine reads that
	// field as a bind address (infra/conf/xray.go:267-281).
	unfixed, err := share.ConvertShareLinksToXrayJson(raw)
	if err != nil {
		t.Fatalf("vendored parser: %v", err)
	}
	st := unfixed.OutboundConfigs[0].SendThrough
	if st == nil {
		t.Fatal("premise gone: the vendored parser no longer sets SendThrough, so this test proves nothing")
	}
	if *st != "Living room box" {
		t.Fatalf("SendThrough holds %q, expected the #fragment", *st)
	}
	if _, err := unfixed.Build(); err == nil {
		t.Fatal("premise gone: the unfixed config now builds, so this test proves nothing")
	} else if !strings.Contains(err.Error(), "unable to send through") {
		t.Fatalf("unfixed config failed for some other reason: %v", err)
	}

	// After.
	l := mustParse(t, raw)
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	m := configMap(t, l)
	assertAbsentOrNull(t, m, "outbounds", "0", "sendThrough")
	if err := buildConfig(t, b); err != nil {
		t.Fatalf("emitted config does not build: %v", err)
	}
	if l.Tag != "Living room box" {
		t.Errorf("Tag = %q, want the #fragment as the display name", l.Tag)
	}
}

// TestSendThroughClearedWithNoFragment covers the case that is easy to miss: a
// link with no #fragment still fails, because the parser stores a pointer to
// the empty string and a non-nil pointer is enough to reach the check.
func TestSendThroughClearedWithNoFragment(t *testing.T) {
	raw := "vless://" + fakeUUID + "@" + fakeHost + ":443?security=none&type=raw"

	unfixed, err := share.ConvertShareLinksToXrayJson(raw)
	if err != nil {
		t.Fatalf("vendored parser: %v", err)
	}
	if st := unfixed.OutboundConfigs[0].SendThrough; st == nil || *st != "" {
		t.Fatalf("expected a non-nil pointer to the empty string, got %v", st)
	}
	if _, err := unfixed.Build(); err == nil {
		t.Fatal("premise gone: a fragmentless link now builds unfixed")
	}

	l := mustParse(t, raw)
	if l.Tag != "" {
		t.Errorf("Tag = %q, want empty", l.Tag)
	}
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	if err := buildConfig(t, b); err != nil {
		t.Fatalf("emitted config does not build: %v", err)
	}
}

// --- the json.RawMessage null round trip --------------------------------------

// TestNullKeysAreDroppedFromTheEmittedConfig shows the defect and the fix in
// one run, the same way the sendThrough test does.
//
// A config that builds in memory is not a config that builds after a trip
// through JSON, and JSON is how the engine receives it. conf.REALITYConfig
// holds Target and Dest as json.RawMessage (transport_internet.go:783-784);
// marshalling writes them as null, unmarshalling turns each null into the
// four bytes "null", and Build branches on "c.Dest != nil" at :815 to decide
// between the server and client shapes. So a plain marshal of a client REALITY
// outbound comes back as a server one and is rejected.
func TestNullKeysAreDroppedFromTheEmittedConfig(t *testing.T) {
	l := mustParse(t, vlessRealityLink())

	// Before: the same document with its nulls left in.
	ob := *l.outbound
	unstripped, err := json.Marshal(xrayConfig{Outbounds: []conf.OutboundDetourConfig{ob}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(unstripped), `"dest":null`) {
		t.Fatal("premise gone: the marshalled outbound no longer carries a null dest")
	}
	var before conf.Config
	if err := json.Unmarshal(unstripped, &before); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := before.Build(); err == nil {
		t.Fatal("premise gone: the unstripped document now builds, so this test proves nothing")
	} else if !strings.Contains(err.Error(), `empty "serverNames"`) {
		t.Fatalf("unstripped document failed for some other reason: %v", err)
	}

	// After.
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("emitted config still carries a null: %s", b)
	}
	if err := buildConfig(t, b); err != nil {
		t.Fatalf("emitted config does not build: %v", err)
	}
}

// TestEmittedConfigIsStableUnderReserialisation: the document has to survive
// being read and written again, because that is what a stored config does.
func TestEmittedConfigIsStableUnderReserialisation(t *testing.T) {
	for _, raw := range []string{vlessRealityLink(), vlessTLSWebsocketLink(), hysteria2Link()} {
		l := mustParse(t, raw)
		first, err := l.XrayConfig()
		if err != nil {
			t.Fatalf("XrayConfig: %v", err)
		}
		var c conf.Config
		if err := json.Unmarshal(first, &c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, err := c.Build(); err != nil {
			t.Fatalf("first build: %v", err)
		}
		// Build a second time from the same bytes: Build must not be
		// destructive of the decoded config in a way that matters.
		var again conf.Config
		if err := json.Unmarshal(first, &again); err != nil {
			t.Fatalf("decode again: %v", err)
		}
		if _, err := again.Build(); err != nil {
			t.Fatalf("second build: %v", err)
		}
	}
}

// --- the user id ------------------------------------------------------------

// TestTruncatedUUIDRejected also proves the check is load-bearing, by showing
// that the engine accepts the same truncated id without complaint.
func TestTruncatedUUIDRejected(t *testing.T) {
	raw := "vless://" + truncatedUUID + "@" + fakeHost + ":443?security=none&type=raw#Typo"

	// What the engine does with it: nothing. It derives a different UUID and
	// builds a config that authenticates as somebody else.
	unfixed, err := share.ConvertShareLinksToXrayJson(raw)
	if err != nil {
		t.Fatalf("vendored parser: %v", err)
	}
	unfixed.OutboundConfigs[0].SendThrough = nil
	if _, err := unfixed.Build(); err != nil {
		t.Fatalf("premise gone: the engine now rejects a truncated id (%v), so this check may be redundant", err)
	}

	// What this package does with it.
	if _, err := Parse(raw); !errors.Is(err, ErrBadUUID) {
		t.Fatalf("Parse error = %v, want ErrBadUUID", err)
	}
}

func TestUUIDForms(t *testing.T) {
	cases := []struct {
		name string
		id   string
		ok   bool
	}{
		{"canonical", fakeUUID, true},
		{"undashed_32", "11111111222243338444555555555555", true},
		{"uppercase", strings.ToUpper(fakeUUID), true},
		{"truncated_7", truncatedUUID, false},
		{"empty", "", false},
		{"one_char", "a", false},
		{"thirty_chars", strings.Repeat("a", 30), false},
		{"non_hex_canonical", "gggggggg-2222-4333-8444-555555555555", false},
		{"missing_a_dash", "111111112222-4333-8444-555555555555", false},
		{"too_long", fakeUUID + "0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validUUID(c.id); got != c.ok {
				t.Errorf("validUUID(%d chars) = %v, want %v", len(c.id), got, c.ok)
			}
		})
	}
}

// TestUUIDCheckAppliesOnlyWhereRequired: shadowsocks, trojan and hysteria
// authenticate with a free-form secret, so applying the UUID rule to them
// would reject every valid link.
func TestUUIDCheckAppliesOnlyWhereRequired(t *testing.T) {
	for _, p := range []string{"vless", "vmess"} {
		if !needsUUID(p) {
			t.Errorf("needsUUID(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"shadowsocks", "trojan", "socks", "hysteria"} {
		if needsUUID(p) {
			t.Errorf("needsUUID(%q) = true, want false", p)
		}
	}
}

// --- rejection ---------------------------------------------------------------

func TestEmptyInputRejected(t *testing.T) {
	for _, raw := range []string{"", "   ", "\n\n\t", " \r\n "} {
		if _, err := Parse(raw); !errors.Is(err, ErrEmpty) {
			t.Errorf("Parse(%q) error = %v, want ErrEmpty", raw, err)
		}
	}
}

// TestEmptyInputIsNotRejectedByTheParser records why the check above cannot be
// delegated: the vendored parser returns a config with zero outbounds and no
// error at all.
func TestEmptyInputIsNotRejectedByTheParser(t *testing.T) {
	cfg, err := share.ConvertShareLinksToXrayJson("")
	if err != nil {
		t.Skipf("premise gone: the vendored parser now rejects empty input (%v)", err)
	}
	if len(cfg.OutboundConfigs) != 0 {
		t.Fatalf("expected zero outbounds, got %d", len(cfg.OutboundConfigs))
	}
}

func TestUnknownSchemeRejected(t *testing.T) {
	for _, raw := range []string{
		"wireguard://key@" + fakeHost + ":51820#WG",
		"tuic://" + fakeUUID + ":" + fakePassword + "@" + fakeHost + ":443",
		"ssr://" + fakePassword,
		"https://example.invalid/subscription",
	} {
		_, err := Parse(raw)
		if !errors.Is(err, ErrUnsupportedScheme) {
			t.Errorf("Parse(%.20s...) error = %v, want ErrUnsupportedScheme", raw, err)
		}
	}
}

// TestTextWithNoLinkRejected covers input that is neither empty nor
// URI-shaped: the parser reads it as Clash YAML and returns zero outbounds.
func TestTextWithNoLinkRejected(t *testing.T) {
	for _, raw := range []string{"proxies: []", "hello", "just some notes\nand more notes"} {
		if _, err := Parse(raw); !errors.Is(err, ErrNoLink) {
			t.Errorf("Parse(%q) error = %v, want ErrNoLink", raw, err)
		}
	}
}

func TestMalformedRealityRejected(t *testing.T) {
	base := "vless://" + fakeUUID + "@" + fakeHost + ":443?security=reality&type=raw&fp=chrome&sni=" + fakeSNI
	cases := []struct {
		name string
		raw  string
	}{
		{"public key too short", base + "&pbk=TOOSHORT&sid=" + fakeShortID},
		{"public key missing", base + "&sid=" + fakeShortID},
		{"short id not hex", base + "&pbk=" + fakePublicKey() + "&sid=zzzzzzzz"},
		{"short id odd length", base + "&pbk=" + fakePublicKey() + "&sid=012"},
		{"short id too long", base + "&pbk=" + fakePublicKey() + "&sid=" + strings.Repeat("ab", 9)},
		{"pqv wrong length", base + "&pbk=" + fakePublicKey() + "&sid=" + fakeShortID + "&pqv=c2hvcnQ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.raw)
			if !errors.Is(err, ErrBadReality) {
				t.Fatalf("error = %v, want ErrBadReality", err)
			}
		})
	}
}

func TestPortZeroRejected(t *testing.T) {
	// The engine accepts port 0 and then dials nothing useful.
	raw := "vless://" + fakeUUID + "@" + fakeHost + ":0?security=none&type=raw#Zero"
	if _, err := Parse(raw); !errors.Is(err, ErrBadPort) {
		t.Fatalf("error = %v, want ErrBadPort", err)
	}
}

// --- the protocols ------------------------------------------------------------

func TestEveryProtocolParsesAndBuilds(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		protocol string
		network  string
		security Security
		port     uint16
	}{
		{"vless_reality", vlessRealityLink(), "vless", "tcp", SecurityReality, 443},
		{"vless_tls_ws", vlessTLSWebsocketLink(), "vless", "websocket", SecurityTLS, 8443},
		{"vmess_base64", vmessBase64Link(), "vmess", "websocket", SecurityTLS, 443},
		{"shadowsocks_sip002", shadowsocksSIP002Link(), "shadowsocks", "tcp", SecurityNone, 8388},
		{"trojan", trojanLink(), "trojan", "tcp", SecurityTLS, 443},
		{"hysteria2", hysteria2Link(), "hysteria", "hysteria", SecurityTLS, 443},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := mustParse(t, c.raw)
			if l.Protocol != c.protocol {
				t.Errorf("Protocol = %q, want %q", l.Protocol, c.protocol)
			}
			if l.Network != c.network {
				t.Errorf("Network = %q, want %q", l.Network, c.network)
			}
			if l.Security != c.security {
				t.Errorf("Security = %q, want %q", l.Security, c.security)
			}
			if l.Address != fakeHost {
				t.Errorf("Address = %q, want %q", l.Address, fakeHost)
			}
			if l.Port != c.port {
				t.Errorf("Port = %d, want %d", l.Port, c.port)
			}
			if l.Count != 1 {
				t.Errorf("Count = %d, want 1", l.Count)
			}

			b, err := l.XrayConfig()
			if err != nil {
				t.Fatalf("XrayConfig: %v", err)
			}
			m := configMap(t, l)
			if got := digString(t, m, "outbounds", "0", "protocol"); got != c.protocol {
				t.Errorf("emitted protocol = %q, want %q", got, c.protocol)
			}
			if got := digString(t, m, "outbounds", "0", "tag"); got != OutboundTag {
				t.Errorf("emitted tag = %q, want %q", got, OutboundTag)
			}
			assertAbsentOrNull(t, m, "outbounds", "0", "sendThrough")
			if got := digString(t, m, "outbounds", "0", "settings", "address"); got != fakeHost {
				t.Errorf("emitted settings.address = %q, want %q", got, fakeHost)
			}
			if err := buildConfig(t, b); err != nil {
				t.Fatalf("emitted config does not build: %v", err)
			}
		})
	}
}

// TestVlessTLSWebsocketFieldByField asserts the non-REALITY half of the
// mapping: a websocket link's path and host, and the tls serverName, all of
// which land under different keys from their URI names.
func TestVlessTLSWebsocketFieldByField(t *testing.T) {
	l := mustParse(t, vlessTLSWebsocketLink())
	m := configMap(t, l)
	ss := dig(t, m, "outbounds", "0", "streamSettings")

	if got := digString(t, ss, "network"); got != "ws" {
		t.Errorf("streamSettings.network = %q, want %q", got, "ws")
	}
	if got := digString(t, ss, "security"); got != "tls" {
		t.Errorf("streamSettings.security = %q, want %q", got, "tls")
	}
	if got := digString(t, ss, "tlsSettings", "serverName"); got != "cdn.fake.invalid" {
		t.Errorf("tlsSettings.serverName = %q, want the sni parameter", got)
	}
	if got := digString(t, ss, "tlsSettings", "fingerprint"); got != "firefox" {
		t.Errorf("tlsSettings.fingerprint = %q, want the fp parameter", got)
	}
	if got := digString(t, ss, "wsSettings", "path"); got != "/ws" {
		t.Errorf("wsSettings.path = %q, want the path parameter", got)
	}
	if got := digString(t, ss, "wsSettings", "host"); got != "cdn.fake.invalid" {
		t.Errorf("wsSettings.host = %q, want the host parameter", got)
	}
	assertAbsentOrNull(t, ss, "realitySettings")
	if got := digString(t, m, "outbounds", "0", "settings", "id"); got != fakeUUID {
		t.Errorf("settings.id = %q, want the link's uuid", got)
	}
}

// TestSeveralLinksUsesTheFirst covers the newline-separated and base64
// subscription forms.
func TestSeveralLinksUsesTheFirst(t *testing.T) {
	joined := strings.Join([]string{vlessRealityLink(), trojanLink(), shadowsocksSIP002Link()}, "\n")

	l := mustParse(t, joined)
	if l.Protocol != "vless" || l.Count != 3 {
		t.Fatalf("Protocol=%q Count=%d, want vless and 3", l.Protocol, l.Count)
	}
	if !strings.Contains(l.Redacted(), "first of 3 links found") {
		t.Errorf("Redacted does not say how many links were pasted: %s", l.Redacted())
	}

	// The same three, base64 encoded, as a subscription blob.
	blob := base64Std(joined)
	l2 := mustParse(t, blob)
	if l2.Protocol != "vless" || l2.Count != 3 {
		t.Fatalf("base64 blob: Protocol=%q Count=%d, want vless and 3", l2.Protocol, l2.Count)
	}
	b, err := l2.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	obs := dig(t, mapOf(t, b), "outbounds")
	if n := len(obs.([]any)); n != 1 {
		t.Errorf("emitted config holds %d outbounds, want exactly 1", n)
	}
	if err := buildConfig(t, b); err != nil {
		t.Fatalf("emitted config does not build: %v", err)
	}
}

// --- redaction ---------------------------------------------------------------

// TestNothingRendersASecret checks every way this type is likely to reach a
// log or a page: Redacted, String, the fmt verbs that walk a struct, and
// encoding/json.
func TestNothingRendersASecret(t *testing.T) {
	links := []string{
		vlessRealityLink(), vlessTLSWebsocketLink(), vmessBase64Link(),
		shadowsocksSIP002Link(), trojanLink(), hysteria2Link(),
	}
	for _, raw := range links {
		l := mustParse(t, raw)
		j, err := json.Marshal(l)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		renderings := map[string]string{
			"Redacted":            l.Redacted(),
			"String":              l.String(),
			"fmt %v on pointer":   fmt.Sprintf("%v", l),
			"fmt %+v on pointer":  fmt.Sprintf("%+v", l),
			"fmt %s on pointer":   l.String(),
			"fmt %v on value":     fmt.Sprintf("%v", *l),
			"fmt %+v on value":    fmt.Sprintf("%+v", *l),
			"encoding/json":       string(j),
			"fmt %#v on the type": fmt.Sprintf("%T", *l),
		}
		for how, out := range renderings {
			for _, secret := range secretsIn() {
				if strings.Contains(out, secret) {
					t.Errorf("%s leaked a secret for %.24s...", how, raw)
				}
			}
		}
	}
}

func TestRedactedNamesRealityPresenceNotValues(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	got := l.Redacted()
	for _, want := range []string{
		"vless to " + fakeHost + ":443",
		"security reality",
		"publicKey set",
		"shortId set",
		"mldsa65Verify set",
		`name "Living room box"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Redacted missing %q\ngot: %s", want, got)
		}
	}
}

func TestRedactedSaysWhenRealityFieldsAreAbsent(t *testing.T) {
	raw := "vless://" + fakeUUID + "@" + fakeHost + ":443?security=reality&type=raw&fp=chrome&sni=" +
		fakeSNI + "&pbk=" + fakePublicKey() + "#Minimal"
	l := mustParse(t, raw)
	for _, want := range []string{"publicKey set", "shortId absent", "mldsa65Verify absent"} {
		if !strings.Contains(l.Redacted(), want) {
			t.Errorf("Redacted missing %q\ngot: %s", want, l.Redacted())
		}
	}
}

// TestRedactedEscapesTheDisplayName: the #fragment is arbitrary user text and
// ends up in logs. Quoting it keeps an escape sequence from reaching a
// terminal that would act on it.
func TestRedactedEscapesTheDisplayName(t *testing.T) {
	raw := "vless://" + fakeUUID + "@" + fakeHost + ":443?security=none&type=raw#%1B%5B31mred%1B%5B0m"
	l := mustParse(t, raw)
	if strings.ContainsRune(l.Redacted(), 0x1b) {
		t.Errorf("Redacted passed an escape character through: %q", l.Redacted())
	}
	if !strings.Contains(l.Redacted(), `\x1b`) {
		t.Errorf("expected the escape to be shown escaped, got %q", l.Redacted())
	}
}

// TestErrorsQuoteNoValue checks the errors this package returns against the
// values that produced them.
func TestErrorsQuoteNoValue(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		secret string
	}{
		{"bad uuid", "vless://" + truncatedUUID + "@" + fakeHost + ":443?security=none&type=raw", truncatedUUID},
		{
			"bad public key",
			"vless://" + fakeUUID + "@" + fakeHost + ":443?security=reality&type=raw&pbk=NOTAKEY&sid=" + fakeShortID,
			"NOTAKEY",
		},
		{
			"bad short id",
			"vless://" + fakeUUID + "@" + fakeHost + ":443?security=reality&type=raw&pbk=" + fakePublicKey() + "&sid=zzzz",
			"zzzz",
		},
		{
			"shadowsocks userinfo without a colon",
			"ss://" + base64Raw("no-colon-here-"+fakePassword) + "@" + fakeHost + ":8388",
			fakePassword,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.raw)
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), c.secret) {
				t.Errorf("error quotes the offending value: %v", err)
			}
			for _, s := range secretsIn() {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error quotes a secret: %v", err)
				}
			}
		})
	}
}

// TestLinkTypeHasNoSecretFields is a structural guard: it fails when a field
// that could hold credential material is added to the exported surface.
func TestLinkTypeHasNoSecretFields(t *testing.T) {
	// Matched as a suffix so that UserID, PublicKey and the like are caught
	// without banning ServerName or Fingerprint.
	bannedSuffix := []string{"id", "uuid", "key", "password", "secret", "token", "seed", "auth", "credential"}
	bannedAnywhere := []string{"password", "secret", "privatekey", "publickey", "shortid", "token", "credential"}

	rt := reflect.TypeOf(Link{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue // unexported, and unreachable by encoding/json
		}
		name := strings.ToLower(f.Name)
		for _, b := range bannedSuffix {
			if strings.HasSuffix(name, b) {
				t.Errorf("Link has an exported field %q, whose name ends in %q: "+
					"credential material must be recorded as presence, not value", f.Name, b)
			}
		}
		for _, b := range bannedAnywhere {
			if strings.Contains(name, b) {
				t.Errorf("Link has an exported field %q containing %q", f.Name, b)
			}
		}
	}
}

// --- known defect in the vendored parser --------------------------------------

// TestTrojanWithoutQueryLosesTLS_KnownDefect records what the VENDORED PARSER
// does, and nothing about what this package does. Its pair is
// TestTrojanAlwaysGetsTLS below, which records ours. Kept as two tests so that
// if upstream ever fixes this, only this one changes.
//
// trojan://password@host:443#name is the standard minimal trojan link and
// trojan is TLS by definition. The parser holds the right rule and does not
// reach it: parseSecurityFromURL sets security to tls for trojan
// (third_party/libxray-share/stream.go:66-71), but streamSettings returns
// (nil, nil) before calling it when the URI has no query parameters
// (stream.go:11-14). So the minimal form yields an outbound with no stream
// settings at all, and the engine's default for that is a plain TCP stream
// with no security: the password goes out in the clear.
func TestTrojanWithoutQueryLosesTLS_KnownDefect(t *testing.T) {
	cfg, err := share.ConvertShareLinksToXrayJson("trojan://" + fakePassword + "@" + fakeHost + ":443#Minimal")
	if err != nil {
		t.Fatalf("vendored parser: %v", err)
	}
	ob := cfg.OutboundConfigs[0]
	if ob.Protocol != "trojan" {
		t.Fatalf("protocol = %q, want trojan", ob.Protocol)
	}
	if ob.StreamSetting != nil {
		t.Fatalf("the vendored parser now returns stream settings (%+v) for a query-less "+
			"trojan link. If it also sets security to tls, upstream has fixed this: "+
			"update this test to record the new upstream behaviour, and leave "+
			"TestTrojanAlwaysGetsTLS alone", ob.StreamSetting)
	}
}

// TestTrojanAlwaysGetsTLS records what THIS PACKAGE emits, which is the
// invariant that matters: no trojan outbound leaves here without TLS.
func TestTrojanAlwaysGetsTLS(t *testing.T) {
	cases := []struct {
		name           string
		raw            string
		wantServerName string
	}{
		{
			"no query at all, the case upstream loses",
			"trojan://" + fakePassword + "@" + fakeHost + ":443#Minimal",
			fakeHost,
		},
		{
			"security=none written out explicitly",
			"trojan://" + fakePassword + "@" + fakeHost + ":443?security=none#Explicit",
			fakeHost,
		},
		{
			"a query but no security and no sni",
			"trojan://" + fakePassword + "@" + fakeHost + ":443?type=ws&path=%2Fx#Ws",
			fakeHost,
		},
		{
			"an sni is present and must be kept",
			trojanLink(),
			fakeSNI,
		},
		{
			"an IP address, which becomes the server name",
			"trojan://" + fakePassword + "@203.0.113.9:443#ByIP",
			"203.0.113.9",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := mustParse(t, c.raw)
			if l.Security != SecurityTLS {
				t.Fatalf("Security = %q, want tls: a trojan outbound without TLS "+
					"sends the password in the clear", l.Security)
			}
			if l.ServerName != c.wantServerName {
				t.Errorf("ServerName = %q, want %q", l.ServerName, c.wantServerName)
			}

			m := configMap(t, l)
			ss := dig(t, m, "outbounds", "0", "streamSettings")
			if got := digString(t, ss, "security"); got != "tls" {
				t.Errorf("emitted streamSettings.security = %q, want tls", got)
			}
			if got := digString(t, ss, "tlsSettings", "serverName"); got != c.wantServerName {
				t.Errorf("emitted tlsSettings.serverName = %q, want %q", got, c.wantServerName)
			}

			b, err := l.XrayConfig()
			if err != nil {
				t.Fatalf("XrayConfig: %v", err)
			}
			if err := buildConfig(t, b); err != nil {
				t.Fatalf("emitted config does not build: %v", err)
			}
		})
	}
}

// TestNonTrojanProtocolsAreNotGivenTLS: the correction above is keyed on the
// protocol, and must not quietly upgrade anything else. Shadowsocks in
// particular is not a TLS protocol and a link that asks for no security must
// keep none.
func TestNonTrojanProtocolsAreNotGivenTLS(t *testing.T) {
	l := mustParse(t, shadowsocksSIP002Link())
	if l.Security != SecurityNone {
		t.Errorf("shadowsocks Security = %q, want none", l.Security)
	}
	l2 := mustParse(t, "vless://"+fakeUUID+"@"+fakeHost+":443?security=none&type=raw#Plain")
	if l2.Security != SecurityNone {
		t.Errorf("vless Security = %q, want none", l2.Security)
	}
}

// TestTrojanWithRealityIsLeftAlone: neither correction may touch a trojan link
// that asks for reality. It is not plaintext, so requireTLSForTrojan has no
// leak to stop, and its server name lives in realitySettings rather than
// tlsSettings, so the fallback must not invent a second one.
func TestTrojanWithRealityIsLeftAlone(t *testing.T) {
	raw := "trojan://" + fakePassword + "@" + fakeHost + ":443" +
		"?security=reality&type=raw&fp=chrome&sni=" + fakeSNI +
		"&pbk=" + fakePublicKey() + "&sid=" + fakeShortID + "#TrojanReality"
	l := mustParse(t, raw)
	if l.Security != SecurityReality {
		t.Fatalf("Security = %q, want reality: the corrections must not overwrite it", l.Security)
	}
	if l.ServerName != fakeSNI {
		t.Errorf("ServerName = %q, want %q", l.ServerName, fakeSNI)
	}
	m := configMap(t, l)
	ss := dig(t, m, "outbounds", "0", "streamSettings")
	if got := digString(t, ss, "security"); got != "reality" {
		t.Errorf("emitted security = %q, want reality", got)
	}
	assertAbsentOrNull(t, ss, "tlsSettings")
	if got := digString(t, ss, "realitySettings", "serverName"); got != fakeSNI {
		t.Errorf("realitySettings.serverName = %q, want %q", got, fakeSNI)
	}
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	if err := buildConfig(t, b); err != nil {
		t.Fatalf("emitted config does not build: %v", err)
	}
}

// TestHysteria2WithoutSNIHasNoServerName_KnownGap records what the VENDORED
// PARSER does, and nothing about what this package does. Its pair is
// TestHysteria2AlwaysGetsAServerName below. Kept as two tests so that if
// upstream ever fills the server name in, only this one changes.
//
// hysteria2 does get security=tls from the parser on every path, including
// with no query at all, because hysteriaOutbound calls parseSecurityFromURL
// directly (third_party/libxray-share/parse_share.go:387) rather than through
// streamSettings. So it never had the trojan problem. What it does not get is
// a server name.
func TestHysteria2WithoutSNIHasNoServerName_KnownGap(t *testing.T) {
	cfg, err := share.ConvertShareLinksToXrayJson("hysteria2://" + fakeAuth + "@" + fakeHost + ":443#NoSNI")
	if err != nil {
		t.Fatalf("vendored parser: %v", err)
	}
	ss := cfg.OutboundConfigs[0].StreamSetting
	if ss == nil {
		t.Fatal("premise gone: hysteria2 no longer gets stream settings")
	}
	if ss.Security != "tls" {
		t.Fatalf("upstream security = %q, want tls", ss.Security)
	}
	if ss.TLSSettings == nil {
		t.Fatal("premise gone: hysteria2 no longer gets tls settings")
	}
	if ss.TLSSettings.ServerName != "" {
		t.Fatalf("the vendored parser now fills the server name in as %q. If upstream "+
			"has fixed this, update this test to record the new upstream behaviour "+
			"and leave TestHysteria2AlwaysGetsAServerName alone", ss.TLSSettings.ServerName)
	}
}

// TestEmptyTLSServerNameIsAHardError pins the premise the whole server-name
// fallback rests on, so that it is a running check rather than a sentence in a
// comment that nobody re-measures.
//
// An empty server name is not a soft degradation. crypto/tls refuses the
// config outright, before the network is touched. Combined with
// transport/internet/hysteria/dialer.go:457, which calls GetTLSConfig with no
// options and so never fills one in, that makes a hysteria2 link with no sni
// parameter dead on arrival rather than merely weaker.
func TestEmptyTLSServerNameIsAHardError(t *testing.T) {
	handshake := func(cfg *tls.Config) error {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()
		go func() { _ = tls.Server(server, &tls.Config{}).Handshake() }()
		return tls.Client(client, cfg).Handshake()
	}

	err := handshake(&tls.Config{})
	if err == nil {
		t.Fatal("premise gone: crypto/tls now accepts an empty ServerName")
	}
	if !strings.Contains(err.Error(), "either ServerName or InsecureSkipVerify") {
		t.Fatalf("premise gone: an empty ServerName now fails for some other reason: %v", err)
	}

	// A domain and an IP literal both get past the config check and reach the
	// handshake. The IP case is why this package fills an IP in rather than
	// refusing it.
	for _, name := range []string{"example.invalid", "203.0.113.9"} {
		err := handshake(&tls.Config{ServerName: name})
		if err != nil && strings.Contains(err.Error(), "either ServerName or InsecureSkipVerify") {
			t.Errorf("ServerName %q was rejected as if it were empty: %v", name, err)
		}
	}
}

// TestHysteria2AlwaysGetsAServerName records what THIS PACKAGE emits.
func TestHysteria2AlwaysGetsAServerName(t *testing.T) {
	cases := []struct {
		name           string
		raw            string
		wantServerName string
	}{
		{
			"no query at all, the case the engine never fills in",
			"hysteria2://" + fakeAuth + "@" + fakeHost + ":443#NoSNI",
			fakeHost,
		},
		{
			"hy2 scheme spelling, same treatment",
			"hy2://" + fakeAuth + "@" + fakeHost + ":443#ShortScheme",
			fakeHost,
		},
		{
			"an sni is present and must be kept",
			hysteria2Link(),
			fakeSNI,
		},
		{
			"an IP address, which becomes the server name",
			"hysteria2://" + fakeAuth + "@203.0.113.9:443#ByIP",
			"203.0.113.9",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := mustParse(t, c.raw)
			if l.Security != SecurityTLS {
				t.Fatalf("Security = %q, want tls", l.Security)
			}
			if l.ServerName != c.wantServerName {
				t.Errorf("ServerName = %q, want %q: an empty one fails inside "+
					"crypto/tls before a byte leaves the box", l.ServerName, c.wantServerName)
			}
			m := configMap(t, l)
			ss := dig(t, m, "outbounds", "0", "streamSettings")
			if got := digString(t, ss, "network"); got != "hysteria" {
				t.Errorf("emitted network = %q, want hysteria", got)
			}
			if got := digString(t, ss, "security"); got != "tls" {
				t.Errorf("emitted security = %q, want tls", got)
			}
			if got := digString(t, ss, "tlsSettings", "serverName"); got != c.wantServerName {
				t.Errorf("emitted tlsSettings.serverName = %q, want %q", got, c.wantServerName)
			}
			b, err := l.XrayConfig()
			if err != nil {
				t.Fatalf("XrayConfig: %v", err)
			}
			if err := buildConfig(t, b); err != nil {
				t.Fatalf("emitted config does not build: %v", err)
			}
		})
	}
}

// TestServerNameFilledForEveryTLSOutbound covers the generalised rule: if the
// outbound is TLS and the link gave no sni, the address is written in. The rule
// is keyed on neither protocol nor transport, so the table walks both.
//
// The gRPC rows are the case that motivated generalising. grpc/dial.go:139-142
// fills the name only when the address is a domain, so gRPC to an IP literal
// with no sni is the one combination where a single transport behaves two ways.
func TestServerNameFilledForEveryTLSOutbound(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			"vless over raw, engine would fill this one anyway",
			"vless://" + fakeUUID + "@" + fakeHost + ":443?security=tls&type=raw",
			fakeHost,
		},
		{
			"vless over grpc to a domain, engine fills this one",
			"vless://" + fakeUUID + "@" + fakeHost + ":443?security=tls&type=grpc&serviceName=gun",
			fakeHost,
		},
		{
			"vless over grpc to an IP, engine does NOT fill this one",
			"vless://" + fakeUUID + "@203.0.113.9:443?security=tls&type=grpc&serviceName=gun",
			"203.0.113.9",
		},
		{
			"vmess over websocket with no sni and no host",
			"vmess://" + base64Std(`{"v":"2","ps":"VM","add":"`+fakeHost+`","port":"443","id":"`+fakeUUID+`","scy":"auto","net":"ws","path":"/vm","tls":"tls"}`),
			fakeHost,
		},
		{
			"trojan with no query at all",
			"trojan://" + fakePassword + "@" + fakeHost + ":443#T",
			fakeHost,
		},
		{
			"hysteria2 with no query at all",
			"hysteria2://" + fakeAuth + "@" + fakeHost + ":443#H",
			fakeHost,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := mustParse(t, c.raw)
			if l.Security != SecurityTLS {
				t.Fatalf("Security = %q, want tls", l.Security)
			}
			if l.ServerName != c.want {
				t.Errorf("ServerName = %q, want %q", l.ServerName, c.want)
			}
			m := configMap(t, l)
			ss := dig(t, m, "outbounds", "0", "streamSettings")
			if got := digString(t, ss, "tlsSettings", "serverName"); got != c.want {
				t.Errorf("emitted tlsSettings.serverName = %q, want %q", got, c.want)
			}
			b, err := l.XrayConfig()
			if err != nil {
				t.Fatalf("XrayConfig: %v", err)
			}
			if err := buildConfig(t, b); err != nil {
				t.Fatalf("emitted config does not build: %v", err)
			}
		})
	}
}

// TestServerNameNotFilledWhenNotTLS: the rule is a superset of the engine's, not
// a blanket rewrite. An outbound that is not TLS gets nothing, and reality is
// excluded because its name is a different field.
func TestServerNameNotFilledWhenNotTLS(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"shadowsocks, no stream settings at all", shadowsocksSIP002Link()},
		{"vless asking for no security", "vless://" + fakeUUID + "@" + fakeHost + ":443?security=none&type=raw"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := mustParse(t, c.raw)
			if l.Security != SecurityNone {
				t.Fatalf("Security = %q, want none", l.Security)
			}
			if l.ServerName != "" {
				t.Errorf("ServerName = %q, want empty on a non-TLS outbound", l.ServerName)
			}
			m := configMap(t, l)
			if _, ok := dig(t, m, "outbounds", "0").(map[string]any)["streamSettings"]; ok {
				assertAbsentOrNull(t, dig(t, m, "outbounds", "0", "streamSettings"), "tlsSettings")
			}
		})
	}
}

// TestEngineFillsServerNameEvenWhenInsecure pins the engine behaviour the
// no-exception-for-allowInsecure decision rests on.
//
// allowInsecure is the one case where an empty server name could be deliberate,
// because InsecureSkipVerify satisfies crypto/tls on its own. The engine does
// not treat it as deliberate: tls.WithDestination
// (transport/internet/tls/config.go:490-496) tests only whether ServerName is
// empty and never looks at AllowInsecure. This calls the engine directly rather
// than reading it, so it fails if upstream changes its mind.
func TestEngineFillsServerNameEvenWhenInsecure(t *testing.T) {
	domain := xnet.TCPDestination(xnet.DomainAddress(fakeHost), xnet.Port(443))
	ip := xnet.TCPDestination(xnet.ParseAddress("203.0.113.9"), xnet.Port(443))

	cases := []struct {
		name string
		cfg  *xraytls.Config
		dest xnet.Destination
		want string
	}{
		{"secure, domain", &xraytls.Config{}, domain, fakeHost},
		{"INSECURE, domain", &xraytls.Config{AllowInsecure: true}, domain, fakeHost},
		{"INSECURE, ip", &xraytls.Config{AllowInsecure: true}, ip, "203.0.113.9"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.cfg.GetTLSConfig(xraytls.WithDestination(c.dest))
			if got.ServerName != c.want {
				t.Errorf("engine ServerName = %q, want %q. If the engine now leaves "+
					"it empty when allowInsecure is set, revisit fillMissingServerName: "+
					"it deliberately matches this", got.ServerName, c.want)
			}
		})
	}

	// And the hysteria path, which passes no options and so fills nothing.
	if got := (&xraytls.Config{AllowInsecure: true}).GetTLSConfig(); got.ServerName != "" {
		t.Errorf("engine with no options filled ServerName as %q. If hysteria now gets "+
			"a name from the engine, TestHysteria2WithoutSNIHasNoServerName_KnownGap "+
			"should be revisited", got.ServerName)
	}
}

// TestAllowInsecureIsRejectedByTheEngine_KnownTrap records why allowInsecure
// needs no exception in fillMissingServerName, and a trap that is nastier than
// it looks.
//
// insecure=1 is a common shape in share links for self-signed servers, and the
// vendored parser carries it through to tlsSettings.allowInsecure
// (third_party/libxray-share/stream.go:50-52). xray-core v1.260327.0 refuses
// such a config outright: transport_internet.go:709-716 returns a removed-feature
// error pointing at pinnedPeerCertSha256.
//
// The nasty part is that the gate is the WALL CLOCK, not the version. The same
// binary with the same config accepted allowInsecure before 2026-06-01 and
// refuses it after. On an appliance whose clock can come up wrong, that makes
// config acceptance depend on the time, which is worth knowing next to the
// clock check the design already calls for before any connect.
//
// This package does not pre-empt the rejection. That is the "engine rejected
// it" state, which the panel has to tell apart from "link did not parse", and
// conflating them would make a clear failure into a confusing one.
func TestAllowInsecureIsRejectedByTheEngine_KnownTrap(t *testing.T) {
	if !time.Now().After(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Skip("this machine's clock reads before 2026-06-01, which is exactly the " +
			"trap: the engine still accepts allowInsecure on a box whose clock is behind")
	}
	raw := "hysteria2://" + fakeAuth + "@" + fakeHost + ":443?insecure=1#Insecure"
	l := mustParse(t, raw)

	// The server-name rule still applies, because it is keyed on nothing.
	if l.ServerName != fakeHost {
		t.Errorf("ServerName = %q, want %q", l.ServerName, fakeHost)
	}
	m := configMap(t, l)
	ss := dig(t, m, "outbounds", "0", "streamSettings")
	if got := dig(t, ss, "tlsSettings", "allowInsecure"); got != true {
		t.Errorf("allowInsecure = %v, want true: the user's value must survive verbatim", got)
	}

	// And the engine refuses the whole config anyway.
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	err = buildConfig(t, b)
	if err == nil {
		t.Fatal("premise gone: the engine now accepts allowInsecure. If it has come " +
			"back, revisit whether an empty server name is deliberate in that case")
	}
	if !strings.Contains(err.Error(), "allowInsecure") {
		t.Fatalf("rejected for some other reason: %v", err)
	}
}

// TestIPLiteralServerNameSendsNoSNI is the evidence for filling an IP literal in
// rather than refusing it: on the wire it is indistinguishable from leaving the
// field empty, because crypto/tls omits the SNI extension for IP literals. So
// for an IP the fill satisfies the config check and costs nothing.
func TestIPLiteralServerNameSendsNoSNI(t *testing.T) {
	domainHello := clientHelloBytes(t, &tls.Config{InsecureSkipVerify: true, ServerName: fakeHost})
	if !strings.Contains(string(domainHello), fakeHost) {
		t.Fatalf("premise gone: a domain ServerName no longer appears in the ClientHello")
	}
	ipHello := clientHelloBytes(t, &tls.Config{InsecureSkipVerify: true, ServerName: "203.0.113.9"})
	if strings.Contains(string(ipHello), "203.0.113.9") {
		t.Errorf("crypto/tls now sends an IP literal as SNI. Filling an IP in is no " +
			"longer free, so revisit the decision in fillMissingServerName")
	}
}

// clientHelloBytes returns the client's first handshake flight, unencrypted.
func clientHelloBytes(t *testing.T, cfg *tls.Config) []byte {
	t.Helper()
	client, server := net.Pipe()
	out := make(chan []byte, 1)
	go func() {
		header := make([]byte, 5)
		if _, err := io.ReadFull(server, header); err != nil {
			out <- nil
			return
		}
		body := make([]byte, int(header[3])<<8|int(header[4]))
		_, _ = io.ReadFull(server, body)
		out <- body
		server.Close()
	}()
	_ = tls.Client(client, cfg).Handshake()
	client.Close()
	return <-out
}

// --- small helpers -------------------------------------------------------------

func mapOf(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	return m
}
