// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/link"
)

// The fixtures below are COPIED, value for value, from
// internal/link/fixtures_test.go. Nothing here is invented and nothing here
// is, or has ever been, a working credential.
//
// Why they are copied rather than imported: they live in a _test.go file in
// package link, so the Go toolchain will not let another package reach them,
// and this package must not edit internal/link to export them. The copy is
// guarded by TestFixturesStillMatchTheLinkPackage below, which reads the
// original file and fails if any of these literals has changed there. That
// turns "a copy will drift" from a certainty into a test failure.

const (
	// fakeUUID is a syntactically valid UUID, 36 characters, 8-4-4-4-12.
	fakeUUID = "11111111-2222-4333-8444-555555555555"

	// fakeShortID is ten hexadecimal characters, within the engine's limit of
	// sixteen (infra/conf/transport_internet.go:949).
	fakeShortID = "0a1b2c3d4e"

	fakeHost     = "example.invalid"
	fakeSNI      = "www.fake-front.invalid"
	fakePassword = "not-a-real-password"
	fakeAuth     = "not-a-real-auth-string"

	// mldsa65VerifyLen is the decoded byte length the engine requires of a
	// REALITY mldsa65Verify value (infra/conf/transport_internet.go:957).
	mldsa65VerifyLen = 1952
)

func fakePublicKey() string {
	return base64.RawURLEncoding.EncodeToString([]byte("CASPIAN-FAKE-REALITY-PUBKEY-3232"))
}

func fakeMldsa65Verify() string {
	const unit = "CASPIAN-FAKE-MLDSA65-VERIFY-KEY-" // 32 bytes
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat(unit, mldsa65VerifyLen/len(unit))))
}

func vlessRealityLink() string {
	return "vless://" + fakeUUID + "@" + fakeHost + ":443" +
		"?security=reality" +
		"&type=raw" +
		"&flow=xtls-rprx-vision" +
		"&sni=" + fakeSNI +
		"&fp=chrome" +
		"&pbk=" + fakePublicKey() +
		"&sid=" + fakeShortID +
		"&spx=" + "%2Fspider" +
		"&pqv=" + fakeMldsa65Verify() +
		"#Living%20room%20box"
}

func vlessTLSWebsocketLink() string {
	return "vless://" + fakeUUID + "@" + fakeHost + ":8443" +
		"?security=tls&type=ws&path=%2Fws&host=cdn.fake.invalid&sni=cdn.fake.invalid&fp=firefox" +
		"#Websocket%20box"
}

func vmessBase64Link() string {
	const qr = `{"v":"2","ps":"VMess box","add":"example.invalid","port":"443",` +
		`"id":"11111111-2222-4333-8444-555555555555","aid":"0","scy":"auto",` +
		`"net":"ws","type":"none","host":"cdn.fake.invalid","path":"/vm",` +
		`"tls":"tls","sni":"cdn.fake.invalid","fp":"chrome"}`
	return "vmess://" + base64.StdEncoding.EncodeToString([]byte(qr))
}

func shadowsocksSIP002Link() string {
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:" + fakePassword))
	return "ss://" + userinfo + "@" + fakeHost + ":8388#Shadowsocks%20box"
}

func trojanLink() string {
	return "trojan://" + fakePassword + "@" + fakeHost + ":443" +
		"?security=tls&type=raw&sni=" + fakeSNI + "&fp=chrome#Trojan%20box"
}

func hysteria2Link() string {
	return "hysteria2://" + fakeAuth + "@" + fakeHost + ":443" +
		"?sni=" + fakeSNI + "&up=50mbps&down=200mbps&obfs=salamander&obfs-password=" + fakePassword +
		"#Hysteria%20box"
}

// secretsIn lists every value that must never appear in an error this package
// returns. It is the same list as internal/link/fixtures_test.go:107-116.
//
// Note that these values DO appear in the generated config, which is the whole
// point of the config. What must not carry them is an error string.
func secretsIn() []string {
	return []string{
		fakeUUID,
		fakePublicKey(),
		fakeMldsa65Verify(),
		fakeShortID,
		fakePassword,
		fakeAuth,
	}
}

// fixture is one named share link and the parsed Link it produces.
type fixture struct {
	name string
	raw  func() string
}

// fixtures is every protocol shape internal/link accepts, so that "every
// option combination validates" means every combination against every
// protocol, not against one convenient one.
func fixtures() []fixture {
	return []fixture{
		{"vless-reality", vlessRealityLink},
		{"vless-tls-ws", vlessTLSWebsocketLink},
		{"vmess-ws-tls", vmessBase64Link},
		{"shadowsocks", shadowsocksSIP002Link},
		{"trojan", trojanLink},
		{"hysteria2", hysteria2Link},
	}
}

func mustParse(t *testing.T, raw string) *link.Link {
	t.Helper()
	l, err := link.Parse(raw)
	if err != nil {
		t.Fatalf("link.Parse: %v", err)
	}
	return l
}

// TestFixturesStillMatchTheLinkPackage is the guard on the copy above.
//
// It reads internal/link/fixtures_test.go as text and checks that every
// literal this file duplicates is still present there. It cannot check the
// generated links themselves, because the constructors are unexported in a
// test file; what it can check is that the VALUES have not moved underneath
// the copy, which is the drift that matters. A rename in the other package
// leaves this test green and is harmless; a change of value turns it red,
// which is exactly when this file needs editing.
func TestFixturesStillMatchTheLinkPackage(t *testing.T) {
	const path = "../link/fixtures_test.go"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	src := string(b)

	literals := []string{
		fakeUUID,
		fakeShortID,
		fakeHost,
		fakeSNI,
		fakePassword,
		fakeAuth,
		`"CASPIAN-FAKE-REALITY-PUBKEY-3232"`,
		`"CASPIAN-FAKE-MLDSA65-VERIFY-KEY-"`,
		"aes-256-gcm:",
		"up=50mbps&down=200mbps",
	}
	for _, want := range literals {
		if !strings.Contains(src, want) {
			t.Errorf("%s no longer contains the fixture literal %q; the copy in "+
				"internal/xcfg/fixtures_test.go has drifted and must be updated", path, want)
		}
	}
	if !strings.Contains(src, "mldsa65VerifyLen") {
		t.Errorf("%s no longer references mldsa65VerifyLen; the copied constant may be wrong", path)
	}
}
