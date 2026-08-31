// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
)

// The tests in this file are CHARACTERISATION tests, not test-driven ones.
// Every behaviour asserted here was already correct in the production code
// when the tests were written on 2026-08-30; what was missing was any test
// that would notice if it stopped being correct. Each one was checked by
// breaking the guard it covers and confirming the test fails; that table is in
// the report accompanying this change.
//
// They cover the branches the coverage profile showed unreached: the nil
// receivers, the defensive decode failures in fill, the transports the engine
// has removed, and the character classes validUUID rejects.

// --- nil receivers ---------------------------------------------------------

// TestNilLinkRendersAndRefuses covers the two nil-receiver paths. They exist
// because a Link travels between packages and a caller that ignored an error
// from Parse holds nil; printing it must not panic and building a config from
// it must not produce an empty document that looks valid.
func TestNilLinkRendersAndRefuses(t *testing.T) {
	var l *Link

	if got := l.Redacted(); got != "no link" {
		t.Errorf("(*Link)(nil).Redacted() = %q, want %q", got, "no link")
	}

	raw, err := l.XrayConfig()
	if !errors.Is(err, ErrNoLink) {
		t.Errorf("(*Link)(nil).XrayConfig() error = %v, want ErrNoLink", err)
	}
	if raw != nil {
		t.Errorf("(*Link)(nil).XrayConfig() returned %d bytes; a nil link must produce no document", len(raw))
	}
}

// TestUnparsedLinkProducesNoDocument is the case that reaches the same guard
// through a non-nil value. A zero Link is what a caller gets from
// &link.Link{}, and it carries no outbound at all: emitting a document from it
// would hand the engine an outbounds array with nothing in it, which builds
// and then carries no traffic.
func TestUnparsedLinkProducesNoDocument(t *testing.T) {
	l := &Link{}
	if _, err := l.XrayConfig(); !errors.Is(err, ErrNoLink) {
		t.Errorf("XrayConfig on a Link that was never parsed returned %v, want ErrNoLink", err)
	}
}

// --- fill's defensive decodes ----------------------------------------------

// settings builds an outbound whose Settings is the given JSON, for the
// branches in fill that Parse cannot reach because the vendored parser always
// emits well-formed settings. They are reached here directly rather than left
// uncovered, because "the parser always emits X" is a property of vendored
// code that upstream may change and that this package does not control.
func settings(protocol, raw string) *conf.OutboundDetourConfig {
	msg := json.RawMessage(raw)
	return &conf.OutboundDetourConfig{Protocol: protocol, Settings: &msg}
}

func TestFillRejectsUndecodableSettings(t *testing.T) {
	// A settings object whose "port" is a string cannot decode into
	// outboundSettings. The failure must be ErrNoLink, not a panic and not a
	// Link with a zero address that later dials nothing.
	l := &Link{outbound: settings("vless", `{"address":"example.invalid","port":"443"}`)}
	if err := l.fill(); !errors.Is(err, ErrNoLink) {
		t.Errorf("fill with undecodable settings returned %v, want ErrNoLink", err)
	}
}

func TestFillRejectsAMissingAddress(t *testing.T) {
	l := &Link{outbound: settings("vless", `{"port":443,"id":"`+fakeUUID+`"}`)}
	err := l.fill()
	if !errors.Is(err, ErrBadAddress) {
		t.Errorf("fill with no address returned %v, want ErrBadAddress", err)
	}
	// The error is shown to a user and written to a log, so it must not carry
	// the id that was in the settings.
	for _, secret := range secretsIn() {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("the missing-address error quotes credential material: %v", err)
		}
	}
}

// TestFillWithNoSettingsAtAll covers the branch where Settings is a nil
// pointer rather than bad JSON: the decode is skipped and the address check
// still has to refuse it.
func TestFillWithNoSettingsAtAll(t *testing.T) {
	l := &Link{outbound: &conf.OutboundDetourConfig{Protocol: "vless"}}
	if err := l.fill(); !errors.Is(err, ErrBadAddress) {
		t.Errorf("fill with no settings returned %v, want ErrBadAddress", err)
	}
}

// TestFlowOfIgnoresUndecodableSettings pins the deliberate difference between
// flowOf and fill: a flow that cannot be read is reported as absent, because
// flow is a display field, whereas an address that cannot be read is a hard
// error. Both branches are defensive and neither is reachable from Parse.
func TestFlowOfIgnoresUndecodableSettings(t *testing.T) {
	if got := flowOf(settings("vless", `{"flow":42}`)); got != "" {
		t.Errorf("flowOf with an undecodable flow = %q, want the empty string", got)
	}
	// Not vless: the field is not read at all, whatever it says.
	if got := flowOf(settings("trojan", `{"flow":"xtls-rprx-vision"}`)); got != "" {
		t.Errorf("flowOf on a trojan outbound = %q, want the empty string", got)
	}
}

// --- transports the engine has removed --------------------------------------

// clashProxy renders a one-proxy Clash.Meta document with the given transport.
func clashProxy(network string) string {
	return "proxies:\n" +
		"  - name: box\n" +
		"    type: vless\n" +
		"    server: " + fakeHost + "\n" +
		"    port: 443\n" +
		"    uuid: " + fakeUUID + "\n" +
		"    tls: true\n" +
		"    servername: " + fakeSNI + "\n" +
		"    network: " + network + "\n"
}

// TestRemovedTransportsAreRefusedWithASentence covers ErrUnsupportedTransport,
// and it reaches that branch through the CLASH importer rather than through a
// URI, because only one of the two paths reaches it.
//
// Measured 2026-08-30. The engine removed the HTTP and QUIC transports:
// infra/conf/transport_internet.go:1015-1018 returns an error for "h2", "h3",
// "http" and "quic". The two vendored paths treat that differently:
//
//   - the URI path calls Network.Build() itself, in
//     third_party/libxray-share/stream.go:73-76, and returns the error. That
//     error is then dropped per line by parsePlainShareLines
//     (parse_share.go:104-106), so the whole link disappears and Parse reports
//     ErrNoLink. fillStream is never reached.
//   - the Clash path, buildStreamFromTransportFields at
//     third_party/libxray-share/transport_build.go:71-77, assigns
//     Network WITHOUT calling Build, so the bad transport survives into the
//     outbound and fillStream is the first thing that looks at it.
//
// So this test also pins which path produces which error, and
// TestRemovedTransportInAURIIsReportedLessWell below records the gap that
// leaves.
func TestRemovedTransportsAreRefusedWithASentence(t *testing.T) {
	for _, network := range []string{"quic", "h2", "h3", "http"} {
		t.Run(network, func(t *testing.T) {
			_, err := Parse(clashProxy(network))
			if !errors.Is(err, ErrUnsupportedTransport) {
				t.Fatalf("a Clash proxy using the removed %q transport returned %v, want ErrUnsupportedTransport", network, err)
			}
			// The engine's own message for these quotes the feature name and
			// tells the user to use "XHTTP stream-one H3". This package must
			// not pass that through.
			if strings.Contains(err.Error(), "XHTTP") {
				t.Errorf("the engine's removal notice reached the user: %v", err)
			}
		})
	}

	// A transport the engine still carries must survive the same path, so this
	// test fails if the branch is widened into rejecting everything.
	if _, err := Parse(clashProxy("ws")); err != nil {
		t.Errorf("a Clash proxy using the websocket transport was refused: %v", err)
	}
}

// TestRemovedTransportInAURIIsReportedLessWell records a MEASURED GAP rather
// than a guarantee, so that it is visible and cannot be mistaken for working.
//
// A user pasting a share-link URI with type=quic gets ErrNoLink, "nothing in
// the pasted text was a proxy link this box understands". The more useful
// sentence, ErrUnsupportedTransport, is what the same configuration produces
// when it arrives as Clash YAML. Fixing that means classifying the vendored
// parser's dropped per-line errors, which is a change to the parse path and
// not to this package's guards; it is left for a decision rather than done
// quietly here.
//
// This test exists so that if somebody does fix it, this test fails and tells
// them to delete it.
func TestRemovedTransportInAURIIsReportedLessWell(t *testing.T) {
	raw := "vless://" + fakeUUID + "@" + fakeHost + ":443?security=tls&type=quic#box"
	_, err := Parse(raw)
	if errors.Is(err, ErrUnsupportedTransport) {
		t.Fatal("a URI naming a removed transport now reports ErrUnsupportedTransport; " +
			"that is an improvement, so delete this test and the note above it")
	}
	if !errors.Is(err, ErrNoLink) {
		t.Fatalf("a URI naming a removed transport returned %v, want ErrNoLink as measured on 2026-08-30", err)
	}
}

// --- validUUID character classes -------------------------------------------

// TestValidUUIDRejectsNonHex covers the two character-class branches. Both
// matter for the reason validate.go states: common/uuid/uuid.go SHA-1 derives
// a DIFFERENT valid UUID from anything it cannot parse, with no error, so a
// mistyped id authenticates as somebody else and the box reports connected
// while carrying nothing.
func TestValidUUIDRejectsNonHex(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		// 32-character undashed form, one character outside hex.
		{"undashed with a g", "1111111122224333844455555555555g"},
		{"undashed with a dash in the middle", "11111111-2222-4333-8444-55555555555"},
		// 36-character dashed form, one character outside hex in a
		// non-separator position.
		{"dashed with a g", "11111111-2222-4333-8444-55555555555g"},
		{"dashed with a z first", "z1111111-2222-4333-8444-555555555555"},
		{"dashed with a space", "11111111-2222-4333-8444-5555555 5555"},
		// A separator position holding a valid HEX digit rather than a dash.
		// This is the sharper case: it fails only if the separator positions
		// are checked as separators. A check that merely asked "is every
		// character hex or dash" would accept all four of these.
		{"hex where the first dash belongs", "11111111a2222-4333-8444-555555555555"},
		{"hex where the second dash belongs", "11111111-2222b4333-8444-555555555555"},
		{"hex where the third dash belongs", "11111111-2222-4333c8444-555555555555"},
		{"hex where the fourth dash belongs", "11111111-2222-4333-8444d555555555555"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if validUUID(tc.id) {
				t.Errorf("validUUID(%q) accepted a non-hexadecimal id", tc.id)
			}
		})
	}

	// The two accepted forms, so this test fails if the rejection is
	// tightened into rejecting everything.
	for _, ok := range []string{fakeUUID, strings.ReplaceAll(fakeUUID, "-", "")} {
		if !validUUID(ok) {
			t.Errorf("validUUID(%q) rejected a well-formed id", ok)
		}
	}
}

// TestCheckRealityOnNothing covers the nil branch. A link with no REALITY
// section is not a malformed REALITY section, and treating it as one would
// refuse every plain TLS link.
func TestCheckRealityOnNothing(t *testing.T) {
	if err := checkReality(nil); err != nil {
		t.Errorf("checkReality(nil) = %v, want nil", err)
	}
}
