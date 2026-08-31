// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"encoding/base64"
	"fmt"
)

// protocolCase is one protocol, described from both ends.
//
// The two halves are deliberately built from the SAME constant: inbound puts
// the credential into the server, shareLink puts it into the text a user
// pastes. A defect that changes one and not the other is what
// TestEveryCarriageProofCanFail uses to prove the proof has teeth.
type protocolCase struct {
	// name is the protocol as internal/link reports it, except for the two
	// alias rows, which name the scheme so a failure says which spelling
	// broke.
	name string

	// scheme is the URI scheme the pasted link uses.
	scheme string

	// secret is the credential both sides agree on.
	secret string

	// wrongSecret is a credential of the same shape the server will not
	// accept. It is the subject of the wrong-credential defect.
	wrongSecret string

	// usesTLS records whether the link carries a certificate pin, which
	// decides whether the wrong-pin defect applies.
	usesTLS bool

	// inbound builds the server's inbound object. It is handed the real
	// credential, never the client's, so a defect cannot reach it.
	inbound func(port int, cert serverCert) string

	// shareLink builds the text a user pastes. Everything the client will do
	// is derived from this string and nothing else.
	shareLink func(port int, secret, pin string) string
}

// protocolCases is the table.
//
// vless is here as well as the six the work was commissioned for. It is the one
// protocol that was known to carry traffic, and holding it by the same evidence
// as the others is what stops "vless works" being a claim resting on a
// different kind of proof from the rest of the table.
func protocolCases() []protocolCase {
	return []protocolCase{
		{
			name:        "vless",
			scheme:      "vless",
			secret:      credVLess,
			wrongSecret: wrongUUID,
			inbound: func(port int, _ serverCert) string {
				return fmt.Sprintf(`{
    "tag": "in",
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "vless",
    "settings": {"clients": [{"id": "%s"}], "decryption": "none"}
  }`, port, credVLess)
			},
			shareLink: func(port int, secret, _ string) string {
				return fmt.Sprintf("vless://%s@127.0.0.1:%d?encryption=none&type=raw&security=none#Caspian%%20test",
					secret, port)
			},
		},
		{
			name:        "vmess",
			scheme:      "vmess",
			secret:      credVMess,
			wrongSecret: wrongUUID,
			inbound: func(port int, _ serverCert) string {
				return fmt.Sprintf(`{
    "tag": "in",
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "vmess",
    "settings": {"clients": [{"id": "%s"}]}
  }`, port, credVMess)
			},
			// The v2rayN QR form: the scheme followed by base64 of a JSON
			// object (third_party/libxray-share/vmess.go, parseVMessQrCode).
			// It is the form the parser reaches first for a vmess link, so it
			// is the form worth exercising.
			shareLink: func(port int, secret, _ string) string {
				qr := fmt.Sprintf(
					`{"v":"2","ps":"Caspian test","add":"127.0.0.1","port":"%d","id":"%s",`+
						`"aid":"0","scy":"auto","net":"tcp","type":"none","tls":""}`, port, secret)
				return "vmess://" + base64.StdEncoding.EncodeToString([]byte(qr))
			},
		},
		{
			name:        "shadowsocks",
			scheme:      "ss",
			secret:      credSSPassword,
			wrongSecret: wrongCredential,
			inbound: func(port int, _ serverCert) string {
				return fmt.Sprintf(`{
    "tag": "in",
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "shadowsocks",
    "settings": {"method": "%s", "password": "%s", "network": "tcp"}
  }`, port, credSSMethod, credSSPassword)
			},
			// SIP002: base64url of "method:password" as the userinfo.
			shareLink: func(port int, secret, _ string) string {
				userinfo := base64.RawURLEncoding.EncodeToString([]byte(credSSMethod + ":" + secret))
				return fmt.Sprintf("ss://%s@127.0.0.1:%d#Caspian%%20test", userinfo, port)
			},
		},
		{
			name:        "socks",
			scheme:      "socks",
			secret:      credSocksPass,
			wrongSecret: wrongCredential,
			inbound: func(port int, _ serverCert) string {
				return fmt.Sprintf(`{
    "tag": "in",
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "socks",
    "settings": {"auth": "password", "accounts": [{"user": "%s", "pass": "%s"}], "udp": false}
  }`, port, credSocksUser, credSocksPass)
			},
			// The parser reads the userinfo as base64 of "user:password"
			// (third_party/libxray-share/parse_share.go, socksOutbound).
			shareLink: func(port int, secret, _ string) string {
				userinfo := base64.RawURLEncoding.EncodeToString([]byte(credSocksUser + ":" + secret))
				return fmt.Sprintf("socks://%s@127.0.0.1:%d#Caspian%%20test", userinfo, port)
			},
		},
		{
			name:        "trojan",
			scheme:      "trojan",
			secret:      credTrojan,
			wrongSecret: wrongCredential,
			usesTLS:     true,
			inbound: func(port int, cert serverCert) string {
				return fmt.Sprintf(`{
    "tag": "in",
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "trojan",
    "settings": {"clients": [{"password": "%s"}]},
    "streamSettings": {
      "network": "raw",
      "security": "tls",
      "tlsSettings": {"certificates": [{"usage": "encipherment", "certificate": %s, "key": %s}]}
    }
  }`, port, credTrojan, jsonArray(cert.certPEM), jsonArray(cert.keyPEM))
			},
			shareLink: func(port int, secret, pin string) string {
				return fmt.Sprintf("trojan://%s@127.0.0.1:%d?security=tls&type=raw&pcs=%s#Caspian%%20test",
					secret, port, pin)
			},
		},
		{
			name:        "hysteria2",
			scheme:      "hysteria2",
			secret:      credHysteria,
			wrongSecret: wrongCredential,
			usesTLS:     true,
			inbound:     hysteriaInbound,
			shareLink: func(port int, secret, pin string) string {
				return hysteriaShareLink("hysteria2", port, secret, pin)
			},
		},
		{
			// The alias. It parses to the same protocol as hysteria2
			// (third_party/libxray-share/parse_share.go, the "hysteria2",
			// "hy2" case in outbound), and internal/xcfg holds
			// TestGolden_Hy2AndHysteria2ProduceTheSameOutbound, which asserts
			// the two spellings produce an identical OUTBOUND SECTION. That is
			// a comparison, not a carriage proof, and it would stay green if
			// both spellings were equally broken. This row is the other half:
			// the alias is driven end to end in its own right.
			name:        "hy2 (alias for hysteria2)",
			scheme:      "hy2",
			secret:      credHysteria,
			wrongSecret: wrongCredential,
			usesTLS:     true,
			inbound:     hysteriaInbound,
			shareLink: func(port int, secret, pin string) string {
				return hysteriaShareLink("hy2", port, secret, pin)
			},
		},
	}
}

// hysteriaInbound is shared by the hysteria2 and hy2 rows: one server, two
// spellings of the link that reaches it.
//
// The inbound protocol name is "hysteria", not "hysteria2"
// (infra/conf/xray.go registers HysteriaServerConfig under "hysteria"), and
// the version is carried in the settings instead.
func hysteriaInbound(port int, cert serverCert) string {
	return fmt.Sprintf(`{
    "tag": "in",
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "hysteria",
    "settings": {"version": 2, "clients": [{"auth": "%s"}]},
    "streamSettings": {
      "network": "hysteria",
      "security": "tls",
      "hysteriaSettings": {"version": 2},
      "tlsSettings": {
        "alpn": ["h3"],
        "certificates": [{"usage": "encipherment", "certificate": %s, "key": %s}]
      }
    }
  }`, port, credHysteria, jsonArray(cert.certPEM), jsonArray(cert.keyPEM))
}

// hysteriaShareLink builds a hysteria2 link under either of its two schemes.
//
// # Why alpn=h3 is in the link
//
// Neither side of the hysteria transport supplies an ALPN of its own: nothing
// in transport/internet/hysteria's dialer.go or hub.go touches NextProtos, and
// both take whatever tlsSettings carried (they call
// tls.ConfigFromStreamSettings then GetTLSConfig). QUIC requires ALPN, so a
// link with no alpn parameter fails the handshake against this server with
// "CRYPTO_ERROR 0x178 (remote): tls: no application protocol". Measured
// 2026-08-30 while building this suite: without alpn=h3 the hysteria2 row
// failed with exactly that message, and with it the row passes.
//
// That is a property of this pairing, not a defect this suite is entitled to
// hide: the server here is configured with alpn h3 because that is what a
// hysteria2 server uses, and the link says the same. A hysteria2 link that
// carries no alpn is UNTESTED against a server that requires one.
//
// # Why there is no up= or down=
//
// They are optional, and a bare number in either is read as bits per second and
// then rejected by the engine, which internal/link already holds a test for
// (TestHysteria2BareBandwidthIsRejectedByTheEngine_KnownTrap). Leaving them out
// keeps this row about carriage.
func hysteriaShareLink(scheme string, port int, secret, pin string) string {
	return fmt.Sprintf("%s://%s@127.0.0.1:%d?alpn=h3&pcs=%s#Caspian%%20test", scheme, secret, port, pin)
}
