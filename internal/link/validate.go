// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/xtls/xray-core/infra/conf"
)

// mldsa65VerifyLen is the decoded byte length the engine requires of a REALITY
// mldsa65Verify value. Read from
// github.com/xtls/xray-core/infra/conf/transport_internet.go:957, which decodes
// with base64.RawURLEncoding and rejects anything whose length is not this.
const mldsa65VerifyLen = 1952

// realityPublicKeyLen is the decoded byte length the engine requires of a
// REALITY public key (the "pbk" URI parameter), from
// transport_internet.go:943.
const realityPublicKeyLen = 32

// shortIDMaxChars is the longest REALITY short id the engine accepts, counted
// in hex characters, from transport_internet.go:949.
const shortIDMaxChars = 16

// needsUUID reports whether a protocol authenticates with a UUID.
//
// The protocol names are the ones the vendored parser writes into
// OutboundDetourConfig.Protocol: third_party/libxray-share/parse_share.go:196
// ("vmess") and :233 ("vless"). Shadowsocks, trojan, socks and hysteria
// authenticate with a free-form password or auth string, so an equivalent
// check would be wrong for them.
func needsUUID(protocol string) bool {
	return protocol == "vless" || protocol == "vmess"
}

// validUUID reports whether s is a UUID the engine will read as the exact
// 128-bit value the user pasted.
//
// This exists because the engine will NOT tell us when it is not.
// common/uuid/uuid.go:71-83 takes any string of 1 to 30 bytes that is not a
// UUID, SHA-1 derives a different valid UUID from it, and returns no error. A
// mistyped or truncated id therefore builds a config that authenticates as
// somebody else, and the user sees "connected but nothing works" with nothing
// in any log to explain it. Measured 2026-08-30: the id "1111111" produced a
// config that built without error.
//
// Two forms are accepted, and both are decoded by the engine as hex rather
// than derived: the canonical 36-character dashed form, and the 32-character
// undashed form. Every other length that the engine tolerates (31 and 33 to 35
// characters, where its loop strips optional dashes) is rejected here, because
// a share link that carries one is malformed and the user is better served by
// being told so.
func validUUID(s string) bool {
	switch len(s) {
	case 32:
		return isHex(s)
	case 36:
		for i, c := range s {
			if i == 8 || i == 13 || i == 18 || i == 23 {
				if c != '-' {
					return false
				}
				continue
			}
			if !isHexDigit(byte(c)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isHexDigit(s[i]) {
			return false
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// checkReality mirrors the client-side checks the engine performs in
// REALITYConfig.Build, transport_internet.go:926-960, so that a malformed
// value is reported in plain words here instead of coming back from the engine
// inside an error string that quotes the key.
//
// It deliberately does not check the fingerprint: an empty fingerprint is
// valid and means Chrome (transport/internet/tls/tls.go:180-183).
func checkReality(r *conf.REALITYConfig) error {
	if r == nil {
		return nil
	}
	// The vendored parser writes the "pbk" parameter to both fields
	// (third_party/libxray-share/stream.go:54-55); the engine copies Password
	// over PublicKey when Password is set (transport_internet.go:937-939).
	key := r.PublicKey
	if r.Password != "" {
		key = r.Password
	}
	if key == "" {
		return fmt.Errorf("%w: the REALITY public key is missing", ErrBadReality)
	}
	if b, err := base64.RawURLEncoding.DecodeString(key); err != nil || len(b) != realityPublicKeyLen {
		return fmt.Errorf("%w: the REALITY public key is not %d bytes of base64url", ErrBadReality, realityPublicKeyLen)
	}
	if err := checkShortID(r.ShortId); err != nil {
		return err
	}
	if r.Mldsa65Verify != "" {
		b, err := base64.RawURLEncoding.DecodeString(r.Mldsa65Verify)
		if err != nil || len(b) != mldsa65VerifyLen {
			return fmt.Errorf("%w: the REALITY post-quantum verify key is not %d bytes of base64url", ErrBadReality, mldsa65VerifyLen)
		}
	}
	return nil
}

// checkShortID applies the engine's own shortId rules: at most 16 hex
// characters, and hex-decodable, from transport_internet.go:949-955. An empty
// short id is legal there and stays legal here.
func checkShortID(id string) error {
	if id == "" {
		return nil
	}
	if len(id) > shortIDMaxChars {
		return fmt.Errorf("%w: the REALITY short id is longer than %d characters", ErrBadReality, shortIDMaxChars)
	}
	if len(id)%2 != 0 {
		return fmt.Errorf("%w: the REALITY short id has an odd number of characters", ErrBadReality)
	}
	if _, err := hex.DecodeString(id); err != nil {
		return fmt.Errorf("%w: the REALITY short id is not hexadecimal", ErrBadReality)
	}
	return nil
}
