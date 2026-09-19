// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// The tests in this file were written BEFORE ParseAll, Select, Entry and List
// existed, on 2026-09-08. The first run of this file was a compile failure
// naming every one of those identifiers; that failure is the "red" of this
// change, and it is recorded in the report accompanying it.

// hostedLink returns a link of the given scheme pointing at its own host and
// carrying its own #name, so that a test over several entries can tell which
// one it is looking at. Every value is invented; see fixtures_test.go.
func hostedLink(scheme, host, name string) string {
	switch scheme {
	case "vless":
		return "vless://" + fakeUUID + "@" + host + ":443?security=tls&type=raw&sni=" + fakeSNI + "#" + name
	case "trojan":
		return "trojan://" + fakePassword + "@" + host + ":443?security=tls&type=raw&sni=" + fakeSNI + "#" + name
	case "ss":
		return "ss://" + base64Raw("aes-256-gcm:"+fakePassword) + "@" + host + ":8388#" + name
	}
	panic("hostedLink: unknown scheme " + scheme)
}

const (
	hostA = "a.example.invalid"
	hostB = "b.example.invalid"
	hostC = "c.example.invalid"
)

// threeLinks is one entry per line, three different protocols, three hosts.
func threeLinks() string {
	return strings.Join([]string{
		hostedLink("vless", hostA, "alpha"),
		hostedLink("trojan", hostB, "beta"),
		hostedLink("ss", hostC, "gamma"),
	}, "\n")
}

func mustParseAll(t *testing.T, raw string) *List {
	t.Helper()
	list, err := ParseAll(raw)
	if err != nil {
		t.Fatalf("ParseAll returned %v", err)
	}
	if list == nil {
		t.Fatal("ParseAll returned a nil list and no error")
	}
	return list
}

// --- ParseAll, happy paths ---------------------------------------------------

func TestParseAllListsEveryEntryInOrder(t *testing.T) {
	list := mustParseAll(t, threeLinks())

	if len(list.Entries) != 3 {
		t.Fatalf("Entries = %d, want 3", len(list.Entries))
	}
	if list.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0: every line was a usable link", list.Dropped)
	}
	// The shadowsocks fixture carries no query, so it is a plain TCP stream with
	// no security layer; the other two ask for TLS with an sni.
	want := []struct {
		protocol, host, tag string
		port                uint16
		security            Security
		sni                 string
	}{
		{"vless", hostA, "alpha", 443, SecurityTLS, fakeSNI},
		{"trojan", hostB, "beta", 443, SecurityTLS, fakeSNI},
		{"shadowsocks", hostC, "gamma", 8388, SecurityNone, ""},
	}
	for i, w := range want {
		e := list.Entries[i]
		if e.Index != i {
			t.Errorf("Entries[%d].Index = %d, want %d", i, e.Index, i)
		}
		if e.Protocol != w.protocol || e.Address != w.host || e.Port != w.port || e.Tag != w.tag {
			t.Errorf("Entries[%d] = %s %s:%d %q, want %s %s:%d %q",
				i, e.Protocol, e.Address, e.Port, e.Tag, w.protocol, w.host, w.port, w.tag)
		}
		if e.Security != w.security {
			t.Errorf("Entries[%d].Security = %q, want %q", i, e.Security, w.security)
		}
		if e.ServerName != w.sni {
			t.Errorf("Entries[%d].ServerName = %q, want %q", i, e.ServerName, w.sni)
		}
		if e.Network != "tcp" {
			t.Errorf("Entries[%d].Network = %q, want tcp", i, e.Network)
		}
	}
}

func TestParseAllReadsABase64Blob(t *testing.T) {
	list := mustParseAll(t, base64Std(threeLinks()))
	if len(list.Entries) != 3 || list.Dropped != 0 {
		t.Fatalf("base64 blob: Entries=%d Dropped=%d, want 3 and 0", len(list.Entries), list.Dropped)
	}
	if list.Entries[1].Address != hostB {
		t.Errorf("Entries[1].Address = %q, want %q", list.Entries[1].Address, hostB)
	}
}

// TestParseAllReadsRawXrayJSON covers the third input shape the vendored parser
// accepts. The document is this package's own output, so it is exactly the
// shape the engine would be handed; Dropped has to be zero because a JSON
// document has no lines to drop.
func TestParseAllReadsRawXrayJSON(t *testing.T) {
	doc, err := mustParse(t, hostedLink("vless", hostA, "alpha")).XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	list := mustParseAll(t, string(doc))
	if len(list.Entries) != 1 || list.Dropped != 0 {
		t.Fatalf("raw JSON: Entries=%d Dropped=%d, want 1 and 0", len(list.Entries), list.Dropped)
	}
	if list.Entries[0].Address != hostA {
		t.Errorf("Entries[0].Address = %q, want %q", list.Entries[0].Address, hostA)
	}
}

// TestParseAllCountsALineTheParserDropped is acceptance item 4 of the proposal.
// The vendored parser drops the middle line silently
// (third_party/libxray-share/parse_share.go:107-109, a port that does not fit
// in sixteen bits), and until ParseAll existed nothing could say so.
func TestParseAllCountsALineTheParserDropped(t *testing.T) {
	raw := strings.Join([]string{
		hostedLink("vless", hostA, "alpha"),
		"vless://" + fakeUUID + "@" + hostB + ":99999?security=tls#broken",
		hostedLink("ss", hostC, "gamma"),
	}, "\n")

	list := mustParseAll(t, raw)
	if len(list.Entries) != 2 {
		t.Fatalf("Entries = %d, want 2", len(list.Entries))
	}
	if list.Dropped != 1 {
		t.Errorf("Dropped = %d, want 1", list.Dropped)
	}
	// The parser never emitted the broken line, so the surviving entries are
	// consecutive: what was the third line is outbound 1.
	if list.Entries[0].Address != hostA || list.Entries[1].Address != hostC {
		t.Errorf("surviving entries are %q and %q, want %q and %q",
			list.Entries[0].Address, list.Entries[1].Address, hostA, hostC)
	}
	if list.Entries[1].Index != 1 {
		t.Errorf("Entries[1].Index = %d, want 1", list.Entries[1].Index)
	}

	// The same through the base64 form, which decodes to the same lines.
	blob := mustParseAll(t, base64Std(raw))
	if len(blob.Entries) != 2 || blob.Dropped != 1 {
		t.Errorf("base64: Entries=%d Dropped=%d, want 2 and 1", len(blob.Entries), blob.Dropped)
	}
}

// TestParseAllCountsAnEntryThisPackageRefuses is the second class of drop: the
// vendored parser accepted the line but this package's own checks did not. The
// entry is not listed, it is counted, and the Index of the entries around it
// keeps the parser's numbering so that Select can still reach them.
func TestParseAllCountsAnEntryThisPackageRefuses(t *testing.T) {
	raw := strings.Join([]string{
		hostedLink("vless", hostA, "alpha"),
		"vless://" + truncatedUUID + "@" + hostB + ":443?security=tls#badid",
		hostedLink("ss", hostC, "gamma"),
	}, "\n")

	list := mustParseAll(t, raw)
	if len(list.Entries) != 2 || list.Dropped != 1 {
		t.Fatalf("Entries=%d Dropped=%d, want 2 and 1", len(list.Entries), list.Dropped)
	}
	if list.Entries[0].Index != 0 || list.Entries[1].Index != 2 {
		t.Errorf("Index values are %d and %d, want 0 and 2: the refused entry keeps its slot",
			list.Entries[0].Index, list.Entries[1].Index)
	}
	if list.Entries[1].Address != hostC {
		t.Errorf("Entries[1].Address = %q, want %q", list.Entries[1].Address, hostC)
	}

	// Selecting the refused slot reports the refusal rather than sliding to a
	// neighbour: the user asked for that entry and it is not usable.
	if _, _, err := Select(raw, 1); !errors.Is(err, ErrBadUUID) {
		t.Errorf("Select of the refused entry returned %v, want ErrBadUUID", err)
	}
	// And Parse is untouched by the list machinery: it reads entry 0.
	l := mustParse(t, raw)
	if l.Address != hostA || l.Count != 3 {
		t.Errorf("Parse = %s Count %d, want %s and 3", l.Address, l.Count, hostA)
	}
	// A paste whose FIRST entry is the refused one still fails, as it always
	// did, so an existing caller sees no change.
	if _, err := Parse(strings.Join([]string{
		"vless://" + truncatedUUID + "@" + hostB + ":443?security=tls#badid",
		hostedLink("ss", hostC, "gamma"),
	}, "\n")); !errors.Is(err, ErrBadUUID) {
		t.Errorf("Parse with a refused first entry returned %v, want ErrBadUUID", err)
	}
}

// TestParseAllClashWithGroupsListsTheProxies is acceptance item 5. The vendored
// Clash reader has no field for proxy-groups or rules (clash_meta.go:13-15), so
// both must be ignored rather than refused, and Dropped must be zero because
// no line of a YAML document is a share link.
func TestParseAllClashWithGroupsListsTheProxies(t *testing.T) {
	doc := "proxies:\n" +
		"  - name: alpha\n    type: vless\n    server: " + hostA + "\n    port: 443\n    uuid: " + fakeUUID + "\n    tls: true\n    servername: " + fakeSNI + "\n    network: tcp\n" +
		"  - name: beta\n    type: trojan\n    server: " + hostB + "\n    port: 443\n    password: " + fakePassword + "\n    sni: " + fakeSNI + "\n    network: tcp\n" +
		"  - name: gamma\n    type: ss\n    server: " + hostC + "\n    port: 8388\n    cipher: aes-256-gcm\n    password: " + fakePassword + "\n" +
		"proxy-groups:\n" +
		"  - name: choose\n    type: select\n    proxies: [alpha, beta, gamma]\n" +
		"  - name: fastest\n    type: url-test\n    url: http://cp.example.invalid/generate_204\n    interval: 300\n    proxies: [alpha, beta]\n" +
		"rules:\n" +
		"  - MATCH,choose\n"

	list := mustParseAll(t, doc)
	if len(list.Entries) != 3 {
		t.Fatalf("Entries = %d, want 3", len(list.Entries))
	}
	if list.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0 for a YAML document", list.Dropped)
	}
	for i, want := range []string{"alpha", "beta", "gamma"} {
		if list.Entries[i].Tag != want {
			t.Errorf("Entries[%d].Tag = %q, want %q", i, list.Entries[i].Tag, want)
		}
	}
	if list.Entries[2].Protocol != "shadowsocks" || list.Entries[2].Address != hostC {
		t.Errorf("Entries[2] = %s %s, want shadowsocks %s", list.Entries[2].Protocol, list.Entries[2].Address, hostC)
	}
}

// --- ParseAll, unhappy paths -------------------------------------------------

func TestParseAllRefusesWhatParseRefuses(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"empty", "   \n ", ErrEmpty},
		{"unsupported scheme", "wireguard://key@" + fakeHost + ":51820#WG", ErrUnsupportedScheme},
		{"removed transport", "vless://" + fakeUUID + "@" + fakeHost + ":443?security=tls&type=quic#q", ErrUnsupportedTransport},
		{"nothing usable", "just some notes", ErrNoLink},
		{"every line dropped by the parser", "vless://" + fakeUUID + "@" + fakeHost + ":99999#a\nvless://" + fakeUUID + "@" + fakeHost + ":70000#b", ErrNoLink},
		{"the only entry has a bad id", "vless://" + truncatedUUID + "@" + fakeHost + ":443?security=tls#x", ErrBadUUID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			list, err := ParseAll(c.raw)
			if !errors.Is(err, c.want) {
				t.Errorf("ParseAll error = %v, want %v", err, c.want)
			}
			if list != nil {
				t.Errorf("ParseAll returned a list alongside the error")
			}
			// Parse must agree, or the two entry points have drifted.
			if _, perr := Parse(c.raw); !errors.Is(perr, c.want) {
				t.Errorf("Parse error = %v, want %v", perr, c.want)
			}
		})
	}
}

// --- Select ------------------------------------------------------------------

func TestSelectReturnsTheChosenEntryWithItsOutbound(t *testing.T) {
	raw := threeLinks()
	for i, host := range []string{hostA, hostB, hostC} {
		l, clamped, err := Select(raw, i)
		if err != nil {
			t.Fatalf("Select(%d): %v", i, err)
		}
		if clamped {
			t.Errorf("Select(%d) reported a clamp for an in-range index", i)
		}
		if l.Address != host || l.Index != i || l.Count != 3 {
			t.Errorf("Select(%d) = %s index %d count %d, want %s %d 3", i, l.Address, l.Index, l.Count, host, i)
		}
		b, err := l.XrayConfig()
		if err != nil {
			t.Fatalf("Select(%d).XrayConfig: %v", i, err)
		}
		m := mapOf(t, b)
		if got := digString(t, m, "outbounds", "0", "settings", "address"); got != host {
			t.Errorf("Select(%d) emitted address %q, want %q", i, got, host)
		}
		if n := len(dig(t, m, "outbounds").([]any)); n != 1 {
			t.Errorf("Select(%d) emitted %d outbounds, want exactly 1", i, n)
		}
		if err := buildConfig(t, b); err != nil {
			t.Errorf("Select(%d): emitted config does not build: %v", i, err)
		}
	}
}

// TestSelectAppliesTheCorrectionsToTheChosenEntry: the trojan entry with no
// query would lose TLS in the vendored parser; requireTLSForTrojan has to run
// on entry 1 when entry 1 is what was chosen, not only on entry 0.
func TestSelectAppliesTheCorrectionsToTheChosenEntry(t *testing.T) {
	raw := hostedLink("vless", hostA, "alpha") + "\n" +
		"trojan://" + fakePassword + "@" + hostB + ":443#bare"
	l, _, err := Select(raw, 1)
	if err != nil {
		t.Fatalf("Select(1): %v", err)
	}
	if l.Security != SecurityTLS {
		t.Errorf("Security = %q, want tls: the trojan correction did not run on the chosen entry", l.Security)
	}
	if l.ServerName != hostB {
		t.Errorf("ServerName = %q, want the address %q filled in", l.ServerName, hostB)
	}
	m := configMap(t, l)
	if got := digString(t, m, "outbounds", "0", "streamSettings", "security"); got != "tls" {
		t.Errorf("emitted security = %q, want tls", got)
	}
	assertAbsentOrNull(t, m, "outbounds", "0", "sendThrough")
	if got := digString(t, m, "outbounds", "0", "tag"); got != OutboundTag {
		t.Errorf("emitted tag = %q, want %q: the display name must never become the tag", got, OutboundTag)
	}
}

func TestSelectOutOfRangeClampsToTheFirstEntryAndSaysSo(t *testing.T) {
	raw := threeLinks()
	for _, i := range []int{3, 7, -1, -100} {
		l, clamped, err := Select(raw, i)
		if err != nil {
			t.Fatalf("Select(%d): %v", i, err)
		}
		if !clamped {
			t.Errorf("Select(%d) did not report the clamp", i)
		}
		if l.Address != hostA || l.Index != 0 {
			t.Errorf("Select(%d) = %s index %d, want entry 0 (%s)", i, l.Address, l.Index, hostA)
		}
	}
	// The last valid index is not a clamp.
	if _, clamped, err := Select(raw, 2); err != nil || clamped {
		t.Errorf("Select(2) = clamped %t, err %v; want in range", clamped, err)
	}
}

func TestSelectRefusesWhatParseRefuses(t *testing.T) {
	for _, raw := range []string{"", "wireguard://key@" + fakeHost + ":51820", "notes only"} {
		l, clamped, err := Select(raw, 0)
		if err == nil {
			t.Errorf("Select(%q, 0) returned no error", raw)
		}
		if l != nil || clamped {
			t.Errorf("Select(%q, 0) returned a link or a clamp alongside the error", raw)
		}
	}
}

// TestParseIsSelectOfEntryZero pins the promise that Parse did not change: for
// every fixture, Parse and Select(raw, 0) produce the same description and the
// same document, byte for byte.
func TestParseIsSelectOfEntryZero(t *testing.T) {
	for _, raw := range []string{
		vlessRealityLink(), vlessTLSWebsocketLink(), vmessBase64Link(),
		shadowsocksSIP002Link(), trojanLink(), hysteria2Link(), threeLinks(), base64Std(threeLinks()),
	} {
		p := mustParse(t, raw)
		s, clamped, err := Select(raw, 0)
		if err != nil || clamped {
			t.Fatalf("Select(%.16s..., 0): clamped %t, err %v", raw, clamped, err)
		}
		if p.Redacted() != s.Redacted() {
			t.Errorf("Parse and Select(0) describe %.16s... differently:\n %s\n %s", raw, p.Redacted(), s.Redacted())
		}
		pb, _ := p.XrayConfig()
		sb, _ := s.XrayConfig()
		if !bytes.Equal(pb, sb) {
			t.Errorf("Parse and Select(0) emit different documents for %.16s...", raw)
		}
		if p.Index != 0 {
			t.Errorf("Parse(%.16s...).Index = %d, want 0", raw, p.Index)
		}
	}
}

// TestRedactedNamesTheChosenEntry keeps the existing "first of N" wording for
// entry 0, which TestSeveralLinksUsesTheFirst pins, and says which entry it is
// for any other.
func TestRedactedNamesTheChosenEntry(t *testing.T) {
	raw := threeLinks()
	first, _, _ := Select(raw, 0)
	if !strings.Contains(first.Redacted(), "first of 3 links found") {
		t.Errorf("entry 0: %s", first.Redacted())
	}
	second, _, _ := Select(raw, 1)
	if !strings.Contains(second.Redacted(), "entry 2 of 3 links found") {
		t.Errorf("entry 1: %s", second.Redacted())
	}
	for _, l := range []*Link{first, second} {
		for _, secret := range secretsIn() {
			if strings.Contains(l.Redacted(), secret) {
				t.Errorf("Redacted quotes a secret: %s", l.Redacted())
			}
		}
	}
}

// --- the display name ------------------------------------------------------

// TestEntryTagIsCappedAndStrippedOfControlCharacters is acceptance item 7.
// The fragment is provider text: right-to-left words, a percent-encoded
// control character, and well over the cap. The Entry must carry it capped at
// 64 bytes on a rune boundary with the control character gone, and nothing
// else removed. The Link keeps the raw fragment, because Parse is unchanged
// and Redacted already quotes it with %q.
func TestEntryTagIsCappedAndStrippedOfControlCharacters(t *testing.T) {
	// Twenty-five copies of an eight-byte Persian word is 200 bytes.
	rtl := strings.Repeat("سلام", 25)
	if len(rtl) != 200 {
		t.Fatalf("fixture is %d bytes, want 200", len(rtl))
	}
	raw := "vless://" + fakeUUID + "@" + hostA + ":443?security=tls&type=raw&sni=" + fakeSNI +
		"#%01" + rtl

	list := mustParseAll(t, raw)
	tag := list.Entries[0].Tag
	if len(tag) > maxTagBytes {
		t.Errorf("Tag is %d bytes, want at most %d", len(tag), maxTagBytes)
	}
	if !utf8.ValidString(tag) {
		t.Errorf("Tag was cut in the middle of a rune: %q", tag)
	}
	if strings.ContainsRune(tag, '\x01') {
		t.Errorf("Tag still carries the control character: %q", tag)
	}
	for _, r := range tag {
		if unicode.IsControl(r) {
			t.Errorf("Tag carries control character %U", r)
		}
	}
	if !strings.HasPrefix(tag, "سلام") {
		t.Errorf("Tag lost its leading word: %q", tag)
	}
	// Everything that was not a control character survives up to the cap:
	// the first 64 bytes of the 200-byte word run are eight whole words.
	if want := strings.Repeat("سلام", 8); tag != want {
		t.Errorf("Tag = %q, want the first eight words %q", tag, want)
	}

	// Short and clean is untouched, including punctuation and spaces.
	clean := mustParseAll(t, "vless://"+fakeUUID+"@"+hostA+":443?security=tls#Living%20room%20box%20%28UK%29%2C%20fast")
	if got := clean.Entries[0].Tag; got != "Living room box (UK), fast" {
		t.Errorf("a clean tag was altered: %q", got)
	}
	// Exactly at the cap is kept whole.
	at := strings.Repeat("a", maxTagBytes)
	if got := mustParseAll(t, "vless://"+fakeUUID+"@"+hostA+":443?security=tls#"+at).Entries[0].Tag; got != at {
		t.Errorf("a tag of exactly %d bytes was altered to %d bytes", maxTagBytes, len(got))
	}
}

func TestCleanTagDirectly(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"plain", "plain"},
		{"tab\there", "tabhere"},
		{"\x7fdel", "del"},
		{"nel", "nel"},
		{"a\r\nb", "ab"},
		// A multi-byte rune straddling the cap is dropped whole, not split.
		{strings.Repeat("a", maxTagBytes-1) + "é", strings.Repeat("a", maxTagBytes-1)},
	}
	for _, c := range cases {
		if got := cleanTag(c.in); got != c.want {
			t.Errorf("cleanTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- no credential in the list -----------------------------------------------

// TestListCarriesNoSecret renders a List every way a careless caller might and
// asserts no credential appears. It is the runtime half of the structural
// guard in TestLinkTypeHasNoSecretFields.
func TestListCarriesNoSecret(t *testing.T) {
	list := mustParseAll(t, threeLinks()+"\n"+vlessRealityLink()+"\n"+hysteria2Link())
	if len(list.Entries) != 5 {
		t.Fatalf("Entries = %d, want 5", len(list.Entries))
	}
	js, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	renderings := map[string]string{
		"json":      string(js),
		"fmt %v":    fmt.Sprintf("%v", list),
		"fmt %+v":   fmt.Sprintf("%+v", *list),
		"fmt %#v":   fmt.Sprintf("%#v", list.Entries),
		"entry %+v": fmt.Sprintf("%+v", list.Entries[3]),
	}
	for how, out := range renderings {
		for _, secret := range secretsIn() {
			if strings.Contains(out, secret) {
				t.Errorf("%s of a List leaked a secret", how)
			}
		}
	}
	// The REALITY entry records presence, not value, like Link does.
	if r := list.Entries[3].Reality; !r.HasPublicKey || !r.HasShortID || !r.HasMldsa65Verify {
		t.Errorf("Entries[3].Reality = %+v, want every flag set", r)
	}
}

// --- the drop counter's retrace of the vendored dispatch ---------------------

// TestDroppedLinesFollowsEveryBase64Alphabet covers the three decodings the
// vendored parser tries (parse_share.go:20-42), each through ParseAll so that
// the retrace and the parser are shown to agree, not merely to run.
func TestDroppedLinesFollowsEveryBase64Alphabet(t *testing.T) {
	raw := hostedLink("vless", hostA, "alpha") + "\n" +
		"vless://" + fakeUUID + "@" + hostB + ":99999#broken\n" +
		hostedLink("ss", hostC, "gamma")
	for name, blob := range map[string]string{
		"standard":            base64Std(raw),
		"url-safe padded":     base64.URLEncoding.EncodeToString([]byte(raw)),
		"url-safe raw":        base64Raw(raw),
		"standard with crlf":  base64Std(strings.ReplaceAll(raw, "\n", "\r\n")),
		"surrounded by space": "  \n" + base64Std(raw) + "\n ",
	} {
		t.Run(name, func(t *testing.T) {
			list := mustParseAll(t, blob)
			if len(list.Entries) != 2 || list.Dropped != 1 {
				t.Errorf("Entries=%d Dropped=%d, want 2 and 1", len(list.Entries), list.Dropped)
			}
		})
	}

	// Directly, for the branches ParseAll cannot reach: an empty text is not
	// base64, and a count below the accepted number is clamped rather than
	// reported negative.
	if _, ok := decodeBase64Like(""); ok {
		t.Error("decodeBase64Like(\"\") reported a decode")
	}
	if _, ok := decodeBase64Like("not base64 at all!"); ok {
		t.Error("decodeBase64Like accepted text that is not base64")
	}
	if got := droppedLines("vless://x@y:1#a", 5); got != 0 {
		t.Errorf("droppedLines with more accepted than seen = %d, want 0", got)
	}
	if got := droppedLines(`{"outbounds":[]}`, 0); got != 0 {
		t.Errorf("droppedLines on JSON = %d, want 0", got)
	}
	if got := droppedLines("proxies:\n  - name: x\n", 1); got != 0 {
		t.Errorf("droppedLines on YAML = %d, want 0", got)
	}
}
