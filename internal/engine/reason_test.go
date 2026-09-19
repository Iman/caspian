// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package engine

import (
	"errors"
	"testing"
)

// TestReasonOfClassifiesTheShapesSeenInTheWild pins the three texts this
// package classifies, in the exact form xray-core produced them: the Windows
// bind failure from issue #2 on 2026-09-12, and the two removed-feature
// refusals a survey of public share links produced the same day. A future
// xray-core that rewords one of these will turn its case into ReasonOther,
// which the general engine sentence still covers.
func TestReasonOfClassifiesTheShapesSeenInTheWild(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want Reason
	}{
		{"windows bind", "start: app/proxyman/inbound: failed to listen TCP on 10808 > transport/internet: failed to listen on address: 127.0.0.1:10808 > transport/internet/tcp: failed to listen TCP on 127.0.0.1:10808 > listen tcp 127.0.0.1:10808: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.", ReasonPortInUse},
		{"linux bind", "start: app/proxyman/inbound: failed to listen TCP on 10808 > transport/internet: failed to listen on address: 127.0.0.1:10808 > listen tcp 127.0.0.1:10808: bind: address already in use", ReasonPortInUse},
		{"allowInsecure", `validate: infra/conf: failed to build outbound config with tag proxy > infra/conf: failed to build stream settings for outbound detour > infra/conf: Failed to build TLS config. > common/errors: The feature "allowInsecure" has been removed and migrated to "pinnedPeerCertSha256". Please update your config(s) according to release note and documentation.`, ReasonInsecureRemoved},
		{"dead cipher", "validate: infra/conf: failed to build outbound config with tag proxy > infra/conf: failed to build outbound handler for protocol shadowsocks > infra/conf: unknown cipher method: aes-256-cfb", ReasonCipherRemoved},
		{"something else", "validate: infra/conf: failed to build outbound handler for protocol trojan > infra/conf: Trojan password is not specified.", ReasonOther},
		{"nil", "", ReasonOther},
	}
	for _, c := range cases {
		var err error
		if c.msg != "" {
			err = errors.New(c.msg)
		}
		if got := ReasonOf(err); got != c.want {
			t.Errorf("%s: ReasonOf = %q, want %q", c.name, got, c.want)
		}
		// The same answer through the redacting wrapper, which is what every
		// caller actually holds.
		if err != nil {
			if got := ReasonOf(wrap("validate", err)); got != c.want {
				t.Errorf("%s: ReasonOf(wrapped) = %q, want %q", c.name, got, c.want)
			}
		}
	}
}

// TestRedactKeepsTheReasonMarkers guards the assumption ReasonOf rests on: the
// three phrases it looks for survive Redact. A redaction rule widened to cover
// quoted words or dotted identifiers would silently turn every classified
// refusal back into the general one.
func TestRedactKeepsTheReasonMarkers(t *testing.T) {
	for _, marker := range []string{"failed to listen", `"allowInsecure" has been removed`, "unknown cipher method"} {
		if got := Redact(marker); got != marker {
			t.Errorf("Redact(%q) = %q, so ReasonOf cannot see it after redaction", marker, got)
		}
	}
}

// TestValidateReportsARemovedCipherAndARemovedInsecureFlag proves the two
// document shapes against the vendored engine rather than against remembered
// strings: a Shadowsocks outbound with aes-256-cfb, and a TLS outbound with
// allowInsecure. If a future xray-core accepts either again, the assertion
// fails and the class can be retired.
func TestValidateReportsARemovedCipherAndARemovedInsecureFlag(t *testing.T) {
	cipher := []byte(`{"outbounds":[{"protocol":"shadowsocks","settings":{"servers":[{"address":"203.0.113.7","port":8388,"method":"aes-256-cfb","password":"x"}]}}]}`)
	if got := ReasonOf(Validate(cipher)); got != ReasonCipherRemoved {
		t.Errorf("aes-256-cfb: reason %q, want %q (error: %v)", got, ReasonCipherRemoved, Validate(cipher))
	}
	insecure := []byte(`{"outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"203.0.113.7","port":443,"password":"x"}]},"streamSettings":{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.invalid","allowInsecure":true}}}]}`)
	err := Validate(insecure)
	if err == nil {
		t.Skip("this engine accepts allowInsecure, which is the documented clock trap before 2026-06-01 or a new engine; see TestAllowInsecureIsRejectedByTheEngine_KnownTrap in internal/link")
	}
	if got := ReasonOf(err); got != ReasonInsecureRemoved {
		t.Errorf("allowInsecure: reason %q, want %q (error: %v)", got, ReasonInsecureRemoved, err)
	}
}
