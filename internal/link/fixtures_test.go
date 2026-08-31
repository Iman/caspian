// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/base64"
	"strings"
)

// Every value in this file is invented. Nothing here is, or has ever been, a
// working credential. The values are built rather than pasted so that the
// length requirement each one satisfies is stated next to it, and so that a
// reader can see at a glance that they are fake.

const (
	// fakeUUID is a syntactically valid UUID, 36 characters, 8-4-4-4-12.
	fakeUUID = "11111111-2222-4333-8444-555555555555"

	// truncatedUUID is seven characters. The engine does not reject it: it
	// SHA-1 derives a different valid UUID from it and returns no error
	// (common/uuid/uuid.go:71-83). That is the whole reason validUUID exists.
	truncatedUUID = "1111111"

	// fakeShortID is ten hexadecimal characters, within the engine's limit of
	// sixteen (transport_internet.go:949).
	fakeShortID = "0a1b2c3d4e"

	fakeHost     = "example.invalid"
	fakeSNI      = "www.fake-front.invalid"
	fakeSpiderX  = "/spider"
	fakePassword = "not-a-real-password"
	fakeAuth     = "not-a-real-auth-string"
)

// fakePublicKey is 43 characters of base64url, decoding to exactly 32 bytes,
// which is what the engine requires of a REALITY public key
// (transport_internet.go:943).
func fakePublicKey() string {
	return base64.RawURLEncoding.EncodeToString([]byte("CASPIAN-FAKE-REALITY-PUBKEY-3232"))
}

// fakeMldsa65Verify is 2603 characters of base64url, decoding to exactly 1952
// bytes, which is what the engine requires of mldsa65Verify
// (transport_internet.go:957). The length is the point of the fixture: a
// shorter value is rejected by the engine and would not exercise the mapping.
func fakeMldsa65Verify() string {
	const unit = "CASPIAN-FAKE-MLDSA65-VERIFY-KEY-" // 32 bytes
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat(unit, mldsa65VerifyLen/len(unit))))
}

// vlessRealityLink carries all six REALITY parameters whose URI names differ
// from their config keys, plus a #fragment.
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

// vmessBase64Link is the v2rayN QR form: the scheme followed by base64 of a
// JSON object (third_party/libxray-share/vmess.go:12-27).
func vmessBase64Link() string {
	const qr = `{"v":"2","ps":"VMess box","add":"example.invalid","port":"443",` +
		`"id":"11111111-2222-4333-8444-555555555555","aid":"0","scy":"auto",` +
		`"net":"ws","type":"none","host":"cdn.fake.invalid","path":"/vm",` +
		`"tls":"tls","sni":"cdn.fake.invalid","fp":"chrome"}`
	return "vmess://" + base64.StdEncoding.EncodeToString([]byte(qr))
}

// shadowsocksSIP002Link is SIP002: base64url of "method:password" as userinfo.
func shadowsocksSIP002Link() string {
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:" + fakePassword))
	return "ss://" + userinfo + "@" + fakeHost + ":8388#Shadowsocks%20box"
}

func trojanLink() string {
	return "trojan://" + fakePassword + "@" + fakeHost + ":443" +
		"?security=tls&type=raw&sni=" + fakeSNI + "&fp=chrome#Trojan%20box"
}

// hysteria2Link spells the bandwidth units out. A bare number in up or down is
// read by the engine as bits per second, not the megabits the hysteria2 link
// convention means, and the engine then rejects it. See
// TestHysteria2BareBandwidthIsRejectedByTheEngine_KnownTrap.
func hysteria2Link() string {
	return "hysteria2://" + fakeAuth + "@" + fakeHost + ":443" +
		"?sni=" + fakeSNI + "&up=50mbps&down=200mbps&obfs=salamander&obfs-password=" + fakePassword +
		"#Hysteria%20box"
}

// secretsIn lists every value that must never appear in a Link, in its
// rendered forms, or in any error this package returns.
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

func base64Std(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func base64Raw(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
