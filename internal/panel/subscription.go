// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"encoding/base64"
	"fmt"
	"math"
	"mime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"caspianbyoc.org/caspian/internal/state"
)

// This file reads the three headers a subscription provider may send beside
// the body, and renders the one figure the page shows from them.
//
// Everything here treats the provider's bytes as hostile text. The figures
// become integers or are dropped; the name becomes a short run of printable
// characters or is dropped. Nothing here can fail: a provider that sends
// nonsense simply gets the zero value, because a refresh must not be refused
// over a header the person cannot see and did not write.

// parseUserinfo reads the subscription-userinfo header.
//
// The shape providers agree on is "key=value" pairs separated by semicolons,
// with upload, download and total in bytes and expire as a Unix time in
// seconds. Anything else in the header is ignored: an unknown key, a pair
// with no equals sign, an empty key, a value that is not a whole number, a
// negative number, and a number too big for int64 all leave the keys that
// were read untouched. A repeated key takes its last value, because that is
// what a reader that assigns as it goes does and there is no better rule.
func parseUserinfo(h string) state.Quota {
	var q state.Quota
	for _, pair := range strings.Split(h, ";") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || n < 0 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "upload":
			q.Upload = n
		case "download":
			q.Download = n
		case "total":
			q.Total = n
		case "expire":
			q.Expire = n
		}
	}
	return q
}

// labelFromHeaders is the name to show for a config that arrived by refresh,
// when the person has not chosen one themselves.
//
// profile-title is the header providers use for it, optionally base64 encoded
// behind a "base64:" prefix. A title that claims to be base64 and is not is
// used as written rather than dropped, because the person is better served by
// odd text they can recognise than by no name at all. Failing that, the file
// name in content-disposition is used, which Go's mime package decodes for
// both the quoted form and the RFC 5987 form.
//
// Whatever comes out is provider text, so it is stripped of everything that
// is not printable and capped at the same length a typed name is capped at,
// on a rune boundary.
func labelFromHeaders(h map[string]string) string {
	title := strings.TrimSpace(h["profile-title"])
	if rest, ok := strings.CutPrefix(title, "base64:"); ok {
		if decoded, err := decodeLooseBase64(rest); err == nil {
			title = decoded
		}
	}
	if name := cleanLabel(title); name != "" {
		return name
	}
	_, params, err := mime.ParseMediaType(h["content-disposition"])
	if err != nil {
		return ""
	}
	return cleanLabel(params["filename"])
}

// decodeLooseBase64 accepts the padded and unpadded forms, because providers
// send both.
func decodeLooseBase64(s string) (string, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	b, err := base64.RawStdEncoding.DecodeString(s)
	return string(b), err
}

// cleanLabel removes what a name may not contain and caps what is left.
func cleanLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	name := strings.TrimSpace(b.String())
	if len(name) <= maxLabel {
		return name
	}
	cut := maxLabel
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return strings.TrimSpace(name[:cut])
}

// gigabytes renders a byte count the way a provider's own dashboard does, in
// gigabytes of a thousand million bytes with one decimal.
//
// The rounding is done on the tenths before formatting rather than left to
// the format verb, because a decimal like 12.35 is not exactly representable
// and would otherwise round down and disagree with the provider's page.
func gigabytes(b int64) string {
	return fmt.Sprintf("%.1f GB", math.Round(float64(b)/1e8)/10)
}
