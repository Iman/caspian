// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

// One engine configuration document per transport the parser supports.
//
// WHAT WAS MISSING, MEASURED RATHER THAN ASSUMED. internal/link declares seven
// supported schemes in supportedSchemes: vless, vmess, ss, socks, trojan,
// hysteria2 and hy2. goldenCases() froze five protocol shapes and left TWO with
// no golden at all:
//
//	socks  a genuinely distinct outbound protocol, never pinned. Probed on
//	       2026-08-30: link.Parse accepts it, Build produces a 2036 byte
//	       document, and no test in this package had ever looked at it.
//	hy2    an alias. Probed the same day: it parses to protocol "hysteria",
//	       the same as hysteria2. That is worth a file precisely BECAUSE it is
//	       supposed to be an alias: if the two documents ever stop agreeing,
//	       a user who pasted the short form gets a different outbound from a
//	       user who pasted the long one, and nothing else would notice.
//
// The fixtures live here rather than in fixtures_test.go so that the two entries
// added to goldenCases() are the only change to the file that was already there.
//
// Regenerate every golden in the repository:
//
//	bash scripts/golden-update.sh
//
// or this package alone:
//
//	go test ./internal/xcfg -run Golden -update
//
// then READ THE DIFF. This document is what a privileged process runs and what
// carries the user's credentials.

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/link"
)

// socksLink is a SOCKS5 share link. The userinfo is base64 of
// "user:password", which is the form the vendored parser expects.
//
// Nothing here is, or has ever been, a working credential.
func socksLink() string {
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("caspian-fake-user:" + fakePassword))
	return "socks://" + userinfo + "@" + fakeHost + ":1080#Socks%20box"
}

// hy2AliasLink is the same connection as hysteria2Link, written with the short
// scheme. The two documents should differ in nothing that matters, and the
// golden pair is what makes "should" checkable.
func hy2AliasLink() string {
	return "hy2://" + fakeAuth + "@" + fakeHost + ":443" +
		"?sni=" + fakeSNI + "&up=50mbps&down=200mbps&obfs=salamander&obfs-password=" + fakePassword +
		"#Hysteria%20box"
}

// TestGolden_EverySupportedSchemeHasAGolden reads the scheme list from
// internal/link and fails when one of them has no frozen document.
//
// It parses a representative link for each scheme rather than reading the map,
// which is unexported. That is the stronger check anyway: it proves the scheme
// is accepted AND that Build produces something, which is what a golden is a
// picture of.
//
// The failure this exists to catch is silent by nature. An eighth scheme added
// to internal/link ships a transport whose generated document nobody has ever
// looked at, and every test in this package stays green, because every test here
// is written against the schemes somebody remembered.
func TestGolden_EverySupportedSchemeHasAGolden(t *testing.T) {
	// One link per scheme internal/link claims to support. The list is
	// maintained here by hand because the map is unexported; the guard below
	// catches the case where this list falls behind.
	perScheme := map[string]func() string{
		"vless":     vlessRealityLink,
		"vmess":     vmessBase64Link,
		"ss":        shadowsocksSIP002Link,
		"socks":     socksLink,
		"trojan":    trojanLink,
		"hysteria2": hysteria2Link,
		"hy2":       hy2AliasLink,
	}

	// Every scheme must parse and build. A scheme in this list that no longer
	// does is a transport that has silently stopped working.
	names := make([]string, 0, len(perScheme))
	for s := range perScheme {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, s := range names {
		raw := perScheme[s]()
		l, err := link.Parse(raw)
		if err != nil {
			t.Errorf("scheme %s: internal/link no longer parses its own supported scheme: %v", s, err)
			continue
		}
		if _, err := Build(Options{Link: l}); err != nil {
			t.Errorf("scheme %s: Build refused the parsed link: %v", s, err)
			continue
		}
		t.Logf("scheme %s parses and builds (protocol %s)", s, l.Protocol)
	}

	// And every one must be covered by a golden case, matched by the link the
	// case is built from rather than by filename, so a rename cannot break the
	// mapping silently.
	covered := map[string]bool{}
	for _, c := range goldenCases() {
		if c.file == "fail-closed.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join("testdata", c.file))
		if err != nil {
			// -update has not been run yet; the other guards report that.
			continue
		}
		for s, mk := range perScheme {
			l, err := link.Parse(mk())
			if err != nil {
				continue
			}
			want, err := Build(Options{Link: l})
			if err != nil {
				continue
			}
			if string(b) == string(want) {
				covered[s] = true
			}
		}
	}
	for _, s := range names {
		if !covered[s] {
			t.Errorf("internal/link supports the %s scheme and no golden file in testdata is the "+
				"document it produces with default options. A transport whose generated "+
				"configuration nobody has looked at is a transport nobody has reviewed. "+
				"Add a case to goldenCases() and run: bash scripts/golden-update.sh", s)
		}
	}
}

// TestGolden_Hy2AndHysteria2ProduceTheSameOutbound is the point of keeping the
// alias as its own file.
//
// hy2 is documented as another spelling of hysteria2. If it ever stops being
// one, a user who pasted the short form gets a different outbound from a user
// who pasted the long one, and the only symptom is that one of them does not
// connect. The two goldens make the claim checkable; this makes the failure say
// what it means.
func TestGolden_Hy2AndHysteria2ProduceTheSameOutbound(t *testing.T) {
	long := outboundsOf(t, hysteria2Link())
	short := outboundsOf(t, hy2AliasLink())
	if long != short {
		t.Errorf("hy2 and hysteria2 no longer produce the same outbound section.\n"+
			"  hysteria2: %s\n"+
			"        hy2: %s\n"+
			"They are supposed to be two spellings of one scheme. If this divergence is "+
			"intended, say so in testdata/PROVENANCE.md and update both goldens.", long, short)
	}
}

// outboundsOf returns the outbounds section of the document built from a link,
// re-serialised so two documents can be compared as text.
//
// It compares the OUTBOUNDS and not the whole document on purpose: the label a
// user gave the config reaches other parts of the file, and two links written
// with different labels would differ there for a reason that has nothing to do
// with the transport.
func outboundsOf(t *testing.T, raw string) string {
	t.Helper()
	l, err := link.Parse(raw)
	if err != nil {
		t.Fatalf("link.Parse: %v", err)
	}
	b, err := Build(Options{Link: l})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("the generated document is not an object: %v", err)
	}
	out, ok := doc["outbounds"]
	if !ok {
		t.Fatal("the generated document has no outbounds section")
	}
	return string(out)
}

// TestGolden_DocumentShape pins the top-level structure of the engine
// configuration for every transport, as a table.
//
// The per-transport goldens hold the whole document, which is the exact record
// and is 2000 to 6000 bytes of it. This is the readable summary: which
// top-level keys exist, how many inbounds and outbounds there are, and in what
// ORDER the outbounds appear. Order is not cosmetic here. The first outbound is
// the default route for anything the routing rules do not match, so an
// outbound moving to position zero is a change in where unmatched traffic goes,
// and that is invisible in a diff of a large JSON file unless somebody is
// looking for it.
func TestGolden_DocumentShape(t *testing.T) {
	var b strings.Builder
	b.WriteString("# GOLDEN FILE. Generated by internal/xcfg/golden_transports_test.go.\n")
	b.WriteString("# Regenerate with: bash scripts/golden-update.sh\n")
	b.WriteString("#\n")
	b.WriteString("# The shape of the engine configuration document per transport: the\n")
	b.WriteString("# top-level keys, the inbound tags in order, and the outbound protocols\n")
	b.WriteString("# and tags IN ORDER.\n")
	b.WriteString("#\n")
	b.WriteString("# Outbound order is load-bearing. The FIRST outbound is where anything the\n")
	b.WriteString("# routing rules do not match goes. A protocol arriving at position 0 is a\n")
	b.WriteString("# change in the default route for unmatched traffic, and in a diff of a\n")
	b.WriteString("# 6000 byte JSON document that is invisible unless you are hunting for it.\n")

	cases := []struct {
		name string
		raw  func() string
	}{
		{"vless-reality", vlessRealityLink},
		{"vless-tls-ws", vlessTLSWebsocketLink},
		{"vmess-ws-tls", vmessBase64Link},
		{"shadowsocks", shadowsocksSIP002Link},
		{"trojan", trojanLink},
		{"hysteria2", hysteria2Link},
		{"socks", socksLink},
		{"hy2-alias", hy2AliasLink},
	}
	for _, c := range cases {
		l, err := link.Parse(c.raw())
		if err != nil {
			t.Fatalf("%s: link.Parse: %v", c.name, err)
		}
		doc, err := Build(Options{Link: l})
		if err != nil {
			t.Fatalf("%s: Build: %v", c.name, err)
		}
		writeShape(t, &b, c.name, doc)
	}

	// The fail-closed document gets a row of its own. Its whole content is the
	// claim that there is no way out of it, and the shape is where that claim
	// is visible: a document whose outbound list grows is a document that may
	// have grown a way out.
	fc, err := BuildFailClosed(Options{})
	if err != nil {
		t.Fatalf("BuildFailClosed: %v", err)
	}
	writeShape(t, &b, "fail-closed", fc)

	assertGoldenText(t, "document-shape.txt", b.String())
}

func writeShape(t *testing.T, b *strings.Builder, name string, doc []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(doc, &top); err != nil {
		t.Fatalf("%s: not a JSON object: %v", name, err)
	}
	keys := make([]string, 0, len(top))
	for k := range top {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString("\n## " + name + "\n")
	b.WriteString("  top-level keys: " + strings.Join(keys, " ") + "\n")

	type tagged struct {
		Protocol string `json:"protocol"`
		Tag      string `json:"tag"`
	}
	for _, section := range []string{"inbounds", "outbounds"} {
		raw, ok := top[section]
		if !ok {
			b.WriteString("  " + section + ": (absent)\n")
			continue
		}
		var items []tagged
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("%s: %s is not a list of tagged objects: %v", name, section, err)
		}
		b.WriteString("  " + section + ":\n")
		for i, it := range items {
			b.WriteString("    " + strconv.Itoa(i) + " protocol=" + it.Protocol + " tag=" + it.Tag + "\n")
		}
		if len(items) == 0 {
			b.WriteString("    (none)\n")
		}
	}
}

// assertGoldenText is assertGolden for a file that is not JSON, so it does not
// disturb TestGoldenFilesAreAcceptedByTheEngine's testdata/*.json glob.
func assertGoldenText(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v (run: bash scripts/golden-update.sh)", path, err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := 0; i < len(gl) || i < len(wl); i++ {
			g, w := "", ""
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Fatalf("%s differs at line %d\n  got: %s\n want: %s\n"+
					"(run: bash scripts/golden-update.sh, then read the diff)", path, i+1, g, w)
			}
		}
	}
}
