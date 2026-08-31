// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Every value in this file is invented. Nothing here is, or has ever been, a
// working credential.
//
// They are built out of their parts rather than pasted as one string, so that
// the reason each one has the length it has is written next to it, and so that
// a reader can see at a glance that they are fake. internal/link's own
// fixtures_test.go follows the same discipline; these are separate because
// those are unexported to that package.

const (
	// fakeUUIDForPanel is a syntactically valid UUID: 36 characters, 8-4-4-4-12.
	// internal/link validates the shape itself, because the engine will SHA-1
	// derive a different valid UUID from a truncated one and report no error.
	fakeUUIDForPanel = "11111111-2222-4333-8444-555555555555"

	// fakeShortIDForPanel is ten hexadecimal characters, inside the engine's
	// limit of sixteen.
	fakeShortIDForPanel = "0a1b2c3d4e"

	fakeHostForPanel = "example.invalid"
	fakeSNIForPanel  = "www.fake-front.invalid"

	// fakeLabel is the user's own name for the config. It is arbitrary user
	// text that gets rendered, which makes it the escaping test as well.
	fakeLabel = `Living room <box> & "friends"`
)

// fakePublicKeyForPanel is 43 characters of base64url decoding to exactly 32
// bytes, which is what the engine requires of a REALITY public key.
func fakePublicKeyForPanel() string {
	return base64.RawURLEncoding.EncodeToString([]byte("CASPIAN-FAKE-REALITY-PUBKEY-3232"))
}

// testLink is a REALITY vless link carrying every parameter the panel displays.
// It is the config the secrets tests submit and then hunt for.
func testLink() string {
	return "vless://" + fakeUUIDForPanel + "@" + fakeHostForPanel + ":443" +
		"?security=reality" +
		"&type=raw" +
		"&flow=xtls-rprx-vision" +
		"&sni=" + fakeSNIForPanel +
		"&fp=chrome" +
		"&pbk=" + fakePublicKeyForPanel() +
		"&sid=" + fakeShortIDForPanel +
		"&spx=%2Fspider" +
		"#Living%20room%20box"
}

// testLinkSecrets is every value from testLink that must never appear in a
// response body or a log line.
//
// The whole link is in the list as well as its parts, because "the config was
// not echoed" and "no piece of the config was echoed" are different claims and
// the second is the one that matters.
func testLinkSecrets() []string {
	return []string{
		testLink(),
		fakeUUIDForPanel,
		fakePublicKeyForPanel(),
		fakeShortIDForPanel,
	}
}

// unparseableConfig is text that internal/link refuses. It is a supported
// scheme so that it gets past the scheme check and fails in the parser proper,
// which is the path a user who pasted half a link takes.
func unparseableConfig() string {
	return "vless://this-is-not-a-link"
}

// engineRejectedConfig parses cleanly in internal/link and is refused by the
// engine. It is how the second of the three failure states is reached.
//
// Finding one is harder than it looks, which is worth recording. The obvious
// candidates do not work: internal/link validates the address, the port, the
// UUID, the REALITY public key and the short id itself, precisely because the
// engine would accept a bad one silently, so all of those fail at the FIRST
// state instead. Raw xray JSON with an invented protocol fails at the first
// state too, because internal/link finds no address in it.
//
// insecure=1 is the case that does reach the engine, and internal/link's own
// TestAllowInsecureIsRejectedByTheEngine_KnownTrap documents why: the vendored
// parser carries insecure=1 through to tlsSettings.allowInsecure, and
// xray-core v1.260327.0 refuses such a config at
// transport_internet.go:709-716 with a removed-feature error.
//
// The trap that comes with it, recorded by that same test: the gate is the WALL
// CLOCK, not the engine version. The same binary accepted allowInsecure before
// 2026-06-01 and refuses it after. So this fixture stops being an
// engine-rejected config on a box whose clock has come up wrong, which is
// exactly the condition design section 9 warns about. The test that uses it
// checks the premise rather than assuming it.
func engineRejectedConfig() string {
	return "hysteria2://not-a-real-auth-string@" + fakeHostForPanel + ":443?insecure=1#Insecure"
}

// containsAny reports the first needle found in haystack, for error messages
// that say which secret leaked rather than only that one did.
func containsAny(haystack string, needles []string) (string, bool) {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(haystack, n) {
			return n, true
		}
	}
	return "", false
}

// sprintf is fmt.Sprintf with the format chosen by the caller. It exists so a
// test can loop over the verbs that walk a struct, which is the whole point of
// the redaction tests: %v and %s are the ones people write by accident, and
// %+v and %#v are the ones that print every field.
func sprintf(format string, v any) string { return fmt.Sprintf(format, v) }

// urlValues is url.Values from alternating key and value arguments, so a test
// that posts four fields does not need six lines to say so.
func urlValues(kv ...string) url.Values {
	out := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		out.Set(kv[i], kv[i+1])
	}
	return out
}

// readStateFile reads the persisted state, for the tests that assert something
// did NOT reach it.
func readStateFile(h *harness) ([]byte, error) { return os.ReadFile(h.store.Path()) }
