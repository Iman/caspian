// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// markerPrefix opens every replacement Redact writes. It is also how Redact
// recognises its own previous output, which is what makes Redact idempotent:
// running it twice must not turn "[redacted 43 chars]" into
// "[redacted 19 chars]".
const markerPrefix = "[redacted "

// Redact removes credential material from an engine message so it can be
// logged or handed to the panel.
//
// # Why this is in the engine package and not left to the caller
//
// The most likely thing a user of this appliance ever does wrong is paste a
// config with a typo in it. That single action walks straight into the engine
// error paths below, every one of which prints the offending value verbatim.
// A caller that forgets to redact does not get a warning; it gets a panel
// showing somebody's private key. So the redaction lives next to the code that
// produces the errors, and Start, Stop, Validate and the log ring all route
// through it rather than offering it as an option.
//
// # The paths this was built against
//
// All read from github.com/xtls/xray-core v1.260327.0 on 2026-08-30.
//
// REALITY server side, reached when "dest"/"target" is present
// (infra/conf/transport_internet.go:818, the branch at :845 onward):
//
//	:854  invalid "privateKey": <value>
//	:890  too long "shortIds[<i>]": <value>
//	:894  invalid "shortIds[<i>]": <value>
//	:905  "mldsa65Seed" and "privateKey" can not be the same value: <value>
//	:908  invalid "mldsa65Seed": <value>
//
// REALITY client side, the else branch at :925 onward, which is the one an
// appliance user's pasted link reaches:
//
//	:944  invalid "password": <value>
//	:950  too long "shortId": <value>
//	:954  invalid "shortId": <value>
//	:958  invalid "mldsa65Verify": <value>
//
// Note :937-938, which is why the key name in the message is not the key name
// in the JSON: if "password" is set it is copied over PublicKey, so the error
// at :944 prints whichever of "password" or "publicKey" the user supplied.
// Both names are therefore in the keyed set below.
//
// UUIDs, common/uuid/uuid.go:
//
//	:60  invalid UUID: <bytes>   (ParseBytes, renders as "[1 2 3 ...]")
//	:73  invalid UUID: <value>   (ParseString, length 0 or above 30)
//	:93  invalid UUID: <value>   (ParseString, malformed 32-36 char form)
//
// And UUIDs that appear in ordinary prose with no key in front of them, which
// is why the shape rule below exists as well as the keyed rules. These are
// runtime log lines rather than config errors, and they carry a valid
// credential, not a rejected one:
//
//	proxy/vless/inbound/inbound.go:196  "reverse: user <uuid> doesn't exist anymore"
//	proxy/vless/inbound/inbound.go:200, :543, :589, :594  same shape
//
// # What is deliberately not redacted
//
// spiderX, at infra/conf/transport_internet.go:965. It is a decoy URL path,
// not key material, and the error for it ("invalid \"spiderX\": foo") is only
// useful if the reader can see the value that failed the leading-slash check.
//
// Shadowsocks and Trojan passwords, because the engine never prints them:
// infra/conf/shadowsocks.go:68, :86, :215, :241 and infra/conf/trojan.go:70
// all say only "password is not specified". If that ever changes, the keyed
// rule for "password" below already covers the JSON key.
func Redact(msg string) string {
	if msg == "" {
		return msg
	}
	// xray composes a wrapped error as "<outer> > <inner>"
	// (common/errors/errors.go:50-53), and errors.New puts the secret last in
	// its own message (common/serial/string.go:29-35 concatenates with no
	// separator). So within one " > " segment a keyed secret always runs to
	// the end of the segment. Splitting first removes the guesswork about
	// where a value ends, and it preserves the inner cause instead of
	// swallowing it.
	lines := strings.Split(msg, "\n")
	for i, line := range lines {
		segments := strings.Split(line, " > ")
		for j, segment := range segments {
			segments[j] = redactSegment(segment)
		}
		lines[i] = strings.Join(segments, " > ")
	}
	return strings.Join(lines, "\n")
}

// keyedTail matches a JSON key known to carry key material, followed by the
// colon the engine writes, followed by everything to the end of the segment.
//
// "id" is included because that is the JSON key a UUID arrives under, and
// shortIds[N] because the server-side loop at
// infra/conf/transport_internet.go:890 and :894 indexes the key name.
var keyedTail = regexp.MustCompile(
	`(?i)("(?:privateKey|publicKey|password|mldsa65Seed|mldsa65Verify|shortIds\[\d+\]|shortIds|shortId|id)"\s*:\s*)(\S.*)$`)

// unkeyedTail matches the two engine messages that print a secret without the
// JSON key immediately in front of it.
var unkeyedTail = regexp.MustCompile(
	`(?i)((?:can not be the same value|invalid UUID)\s*:\s*)(\S.*)$`)

// uuidShape matches a well-formed UUID anywhere in a message. This is the net
// for valid credentials logged in prose, such as
// proxy/vless/inbound/inbound.go:196, where no key precedes the value.
var uuidShape = regexp.MustCompile(
	`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)

// base64URLShape matches an unbroken run of base64url characters long enough
// that nothing but key material realistically produces it.
//
// The threshold is 32 rather than 43. 43 is the exact RawURLEncoding length of
// a 32-byte X25519 key, and 2603 that of a 1952-byte ML-DSA-65 verify key, so
// both are covered; but the engine prints the value the user typed, not a
// valid key, and a truncated key is still a secret. 32 is low enough to catch
// a mistyped key and high enough that ordinary message text cannot reach it:
// English words, host names, file paths and URLs all break the run on a dot,
// a slash or a space. Hex strings are a subset of this character class, so a
// long hex digest needs no separate rule.
//
// A short id is at most 16 hex characters (the length gate at
// infra/conf/transport_internet.go:889 and :949) and so is deliberately below
// this threshold. Redacting every 16-character hex run would swallow ports,
// counters and addresses across every unrelated message. Short ids are covered
// by the keyed rules instead, which is sufficient because every engine path
// that prints one names the key first.
var base64URLShape = regexp.MustCompile(`[A-Za-z0-9_-]{32,}`)

func redactSegment(segment string) string {
	segment = replaceTail(segment, keyedTail)
	segment = replaceTail(segment, unkeyedTail)
	segment = replaceShape(segment, uuidShape)
	segment = replaceShape(segment, base64URLShape)
	return segment
}

// replaceTail rewrites the trailing value captured by re, leaving an existing
// marker alone so Redact stays idempotent.
func replaceTail(segment string, re *regexp.Regexp) string {
	m := re.FindStringSubmatchIndex(segment)
	if m == nil {
		return segment
	}
	value := segment[m[4]:m[5]]
	if strings.HasPrefix(value, markerPrefix) {
		return segment
	}
	return segment[:m[4]] + mask(value)
}

func replaceShape(segment string, re *regexp.Regexp) string {
	return re.ReplaceAllStringFunc(segment, func(v string) string {
		return mask(v)
	})
}

// mask reports the length of what was removed and nothing else.
//
// The length is kept on purpose. For the REALITY keys it is fixed by the
// format, so it discloses nothing, and it is the single most useful thing to
// tell someone who pasted a key that lost a character: "43 chars" against
// "42 chars" is the whole diagnosis. The cost is that for a free-form secret
// the length does leak, which is why nothing else about the value survives.
func mask(v string) string {
	return fmt.Sprintf("%s%d chars]", markerPrefix, len([]rune(v)))
}

// ContainsSecretShape reports whether s still looks like it carries key
// material. It exists so tests and callers can assert the negative rather than
// eyeballing a string, and it is intentionally the same shape rules Redact
// applies, so a rule added to one is checked by the other.
func ContainsSecretShape(s string) bool {
	return uuidShape.MatchString(s) || base64URLShape.MatchString(s)
}
