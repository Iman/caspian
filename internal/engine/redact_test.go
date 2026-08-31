// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"strings"
	"testing"
)

// The secret-shaped values used below. They are not real credentials; they are
// the right SHAPE, which is what the redaction is written against.
const (
	// 43 characters of base64url, the RawURLEncoding length of a 32-byte
	// X25519 key. Used wherever the engine wants privateKey, publicKey,
	// password or mldsa65Seed.
	key43 = "SEfVpkVvBFVfKzBOe1c-U0zZQEOZTHnnkbLrPZQCLXQ"
	// 43 characters, distinct from key43, for the "same value" check.
	otherKey43 = "aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789_-abcdE"
	// A short id: hex, at most 16 characters
	// (infra/conf/transport_internet.go:949).
	shortIDHex = "0123456789abcdef"
	// 17 hex characters, one over the length gate.
	shortIDTooLong = "0123456789abcdef0"
	// A well-formed UUID, the shape proxy/vless/inbound/inbound.go:196 prints
	// in prose.
	validUUID = "6f9c2b1a-4d3e-4a7b-9c8d-1e2f3a4b5c6d"
)

// TestRedactAgainstEngineErrors is the load-bearing redaction test. Every case
// here builds a config, hands it to the engine's own loader, and asserts three
// things in order:
//
//  1. the engine really does emit the secret (if it stopped doing so, the case
//     is testing nothing and should fail loudly rather than pass quietly),
//  2. Redact removes it,
//  3. Validate, which is what the panel calls, also does not return it.
//
// The point of driving the engine rather than writing the expected strings by
// hand is that a message reworded upstream changes this test's input
// automatically. A transcribed string would keep passing against text the
// engine no longer produces.
func TestRedactAgainstEngineErrors(t *testing.T) {
	cases := []struct {
		name string
		// where the error text comes from, so a failure points at the code
		// that produced it rather than at this table.
		source string
		config string
		secret string
	}{
		{
			name:   "reality client bad password",
			source: "infra/conf/transport_internet.go:944",
			config: realityClientConfig(`"password": "` + key43 + `!"`),
			secret: key43,
		},
		{
			name:   "reality client bad publicKey",
			source: "infra/conf/transport_internet.go:944 via :937-938",
			config: realityClientConfig(`"publicKey": "` + key43 + `!"`),
			secret: key43,
		},
		{
			name:   "reality client bad shortId",
			source: "infra/conf/transport_internet.go:954",
			config: realityClientConfig(`"password": "` + key43 + `", "shortId": "zzzz"`),
			secret: "zzzz",
		},
		{
			name:   "reality client too long shortId",
			source: "infra/conf/transport_internet.go:950",
			config: realityClientConfig(`"password": "` + key43 + `", "shortId": "` + shortIDTooLong + `"`),
			secret: shortIDTooLong,
		},
		{
			name:   "reality client bad mldsa65Verify",
			source: "infra/conf/transport_internet.go:958",
			config: realityClientConfig(`"password": "` + key43 + `", "shortId": "` + shortIDHex + `", "mldsa65Verify": "` + key43 + `"`),
			secret: key43,
		},
		{
			name:   "reality server bad privateKey",
			source: "infra/conf/transport_internet.go:854",
			config: realityServerConfig(`"privateKey": "` + key43 + `!"`),
			secret: key43,
		},
		{
			name:   "reality server bad shortIds element",
			source: "infra/conf/transport_internet.go:894",
			config: realityServerConfig(`"privateKey": "` + key43 + `", "shortIds": ["zzzz"]`),
			secret: "zzzz",
		},
		{
			name:   "reality server too long shortIds element",
			source: "infra/conf/transport_internet.go:890",
			config: realityServerConfig(`"privateKey": "` + key43 + `", "shortIds": ["` + shortIDTooLong + `"]`),
			secret: shortIDTooLong,
		},
		{
			name:   "reality server mldsa65Seed equals privateKey",
			source: "infra/conf/transport_internet.go:905",
			config: realityServerConfig(`"privateKey": "` + key43 + `", "shortIds": ["` + shortIDHex + `"], "mldsa65Seed": "` + key43 + `"`),
			secret: key43,
		},
		{
			name:   "reality server bad mldsa65Seed",
			source: "infra/conf/transport_internet.go:908",
			config: realityServerConfig(`"privateKey": "` + key43 + `", "shortIds": ["` + shortIDHex + `"], "mldsa65Seed": "` + otherKey43 + `!"`),
			secret: otherKey43,
		},
		{
			name:   "vless outbound bad uuid",
			source: "common/uuid/uuid.go:73",
			config: vlessConfig(strings.Repeat("z", 31)),
			secret: strings.Repeat("z", 31),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, rawErr := loadConfig([]byte(tc.config))
			if rawErr == nil {
				t.Fatalf("engine accepted the config; this case no longer exercises %s", tc.source)
			}
			raw := rawErr.Error()

			// 1. The leak is real. If this fails the engine changed and the
			// case needs rewriting, not deleting.
			if !strings.Contains(raw, tc.secret) {
				t.Fatalf("engine error from %s no longer contains the secret; got %q", tc.source, raw)
			}

			// 2. Redact removes it.
			red := Redact(raw)
			if strings.Contains(red, tc.secret) {
				t.Errorf("Redact left the secret in place\n raw: %q\n red: %q", raw, red)
			}
			if !strings.Contains(red, markerPrefix) {
				t.Errorf("Redact removed the secret without saying so: %q", red)
			}

			// 3. The public entry point the panel uses is safe too.
			verr := Validate([]byte(tc.config))
			if verr == nil {
				t.Fatal("Validate accepted a config the engine rejects")
			}
			if strings.Contains(verr.Error(), tc.secret) {
				t.Errorf("Validate returned the secret: %q", verr.Error())
			}

			// 4. Redaction is idempotent, because a message can pass through
			// more than one boundary.
			if got := Redact(red); got != red {
				t.Errorf("Redact is not idempotent\n once: %q\ntwice: %q", red, got)
			}
		})
	}
}

// TestRedactUnkeyedUUID covers the shape rule rather than a keyed rule.
//
// These strings are replicas of engine source lines, not engine output: the
// paths that produce them are VLESS inbound request handling, which a unit
// test cannot reach without standing up a server and a client. They are
// included because they carry a VALID credential in ordinary prose with no
// JSON key in front of it, which is the case the keyed rules cannot catch.
func TestRedactUnkeyedUUID(t *testing.T) {
	cases := []struct {
		source string
		line   string
	}{
		{
			source: "proxy/vless/inbound/inbound.go:196",
			line:   "vless/inbound: reverse: user " + validUUID + " doesn't exist anymore",
		},
		{
			source: "proxy/vless/inbound/inbound.go:543",
			line:   "vless/inbound: for safety reasons, user " + validUUID + " is not allowed to use forward proxy",
		},
		{
			source: "proxy/vless/inbound/inbound.go:589",
			line:   "vless/inbound: account " + validUUID + " is not able to use the flow xtls-rprx-vision",
		},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := Redact(tc.line)
			if strings.Contains(got, validUUID) {
				t.Errorf("UUID survived redaction: %q", got)
			}
			if !strings.Contains(got, markerPrefix) {
				t.Errorf("no marker written: %q", got)
			}
			// The surrounding prose is the diagnosis and must survive.
			if !strings.Contains(got, "vless/inbound") {
				t.Errorf("redaction ate the context: %q", got)
			}
		})
	}
}

func TestRedactUnits(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		absent  []string
		present []string
	}{
		{
			name:    "empty string",
			in:      "",
			present: []string{},
		},
		{
			name:    "message with no secret is untouched",
			in:      "infra/conf: failed to read config file at line 3 char 12",
			present: []string{"failed to read config file at line 3 char 12"},
			absent:  []string{markerPrefix},
		},
		{
			name:    "wrapped error keeps the inner cause",
			in:      `infra/conf: invalid "privateKey": ` + key43 + " > some inner reason",
			absent:  []string{key43},
			present: []string{"some inner reason", `invalid "privateKey": `, markerPrefix},
		},
		{
			name:    "secret in the inner segment is also removed",
			in:      "outer failed > infra/conf: " + `invalid "password": ` + key43,
			absent:  []string{key43},
			present: []string{"outer failed", markerPrefix},
		},
		{
			name:    "multiple lines are each handled",
			in:      `invalid "shortId": deadbeefdeadbeef` + "\n" + `invalid "privateKey": ` + key43,
			absent:  []string{"deadbeefdeadbeef", key43},
			present: []string{markerPrefix},
		},
		{
			name:    "uuid byte-slice form from ParseBytes",
			in:      "common/uuid: invalid UUID: [1 2 3 4]",
			absent:  []string{"[1 2 3 4]"},
			present: []string{"invalid UUID: ", markerPrefix},
		},
		{
			name:    "length is reported so a truncated key is diagnosable",
			in:      `invalid "password": ` + key43,
			present: []string{"[redacted 43 chars]"},
		},
		{
			name: "short id is not redacted by shape alone",
			// A bare 16-hex run with no key in front of it is left alone on
			// purpose; redacting every such run would swallow unrelated
			// numbers across every message. See the base64URLShape comment.
			in:      "connection to 0123456789abcdef failed",
			absent:  []string{markerPrefix},
			present: []string{"0123456789abcdef"},
		},
		{
			name:    "spiderX is deliberately left readable",
			in:      `infra/conf: invalid "spiderX": notaslash`,
			absent:  []string{markerPrefix},
			present: []string{"notaslash"},
		},
		{
			name:    "long base64url run is caught with no key present",
			in:      "handshake rejected: " + key43,
			absent:  []string{key43},
			present: []string{"handshake rejected", markerPrefix},
		},
		{
			name:    "ordinary prose is not over-redacted",
			in:      "engine: failed to bind 127.0.0.1:1080, address already in use",
			absent:  []string{markerPrefix},
			present: []string{"address already in use", "127.0.0.1:1080"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Redact(tc.in)
			for _, s := range tc.absent {
				if strings.Contains(got, s) {
					t.Errorf("expected %q to be absent from %q", s, got)
				}
			}
			for _, s := range tc.present {
				if !strings.Contains(got, s) {
					t.Errorf("expected %q to be present in %q", s, got)
				}
			}
			if got2 := Redact(got); got2 != got {
				t.Errorf("not idempotent\n once: %q\ntwice: %q", got, got2)
			}
		})
	}
}

func TestContainsSecretShape(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"nothing to see here", false},
		{"127.0.0.1:1080", false},
		{"0123456789abcdef", false}, // a short id is below the shape threshold
		{key43, true},
		{validUUID, true},
		{"[redacted 43 chars]", false}, // Redact's own output is clean
		{Redact(`invalid "password": ` + key43), false},
	}
	for _, tc := range cases {
		if got := ContainsSecretShape(tc.in); got != tc.want {
			t.Errorf("ContainsSecretShape(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// --- config builders -------------------------------------------------------

// realityClientConfig produces an outbound whose REALITY settings take the
// client branch of REALITYConfig.Build, which is the branch reached when
// "dest"/"target" is absent (infra/conf/transport_internet.go:818, :925).
// This is the branch a config pasted into the panel goes through.
func realityClientConfig(realitySettings string) string {
	return `{
  "outbounds": [{
    "protocol": "vless",
    "settings": {"vnext": [{
      "address": "192.0.2.10",
      "port": 443,
      "users": [{"id": "` + validUUID + `", "encryption": "none"}]
    }]},
    "streamSettings": {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {"serverName": "example.invalid", "fingerprint": "chrome", ` + realitySettings + `}
    }
  }]
}`
}

// realityServerConfig produces an inbound whose REALITY settings take the
// server branch, reached because "dest" is set
// (infra/conf/transport_internet.go:818-820).
func realityServerConfig(realitySettings string) string {
	return `{
  "inbounds": [{
    "listen": "127.0.0.1",
    "port": 1,
    "protocol": "vless",
    "settings": {"clients": [{"id": "` + validUUID + `"}], "decryption": "none"},
    "streamSettings": {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {"dest": "example.invalid:443", "serverNames": ["example.invalid"], ` + realitySettings + `}
    }
  }],
  "outbounds": [{"protocol": "freedom"}]
}`
}

// vlessConfig produces an outbound with the given user id, to reach
// common/uuid ParseString.
func vlessConfig(id string) string {
	return `{
  "outbounds": [{
    "protocol": "vless",
    "settings": {"vnext": [{
      "address": "192.0.2.10",
      "port": 443,
      "users": [{"id": "` + id + `", "encryption": "none"}]
    }]}
  }]
}`
}
