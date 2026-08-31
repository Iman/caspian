// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package qr

import (
	"strconv"
	"strings"
)

// QuietZone is the light border the standard requires around a symbol, in
// modules. A reader uses it to find the edge; without it the finder patterns
// run into whatever is beside them and many phones will not lock on.
const QuietZone = 4

// SVG renders the symbol as an SVG fragment.
//
// The fragment contains no text from the encoded data. The only variable parts
// are integers: the viewBox bounds and the module coordinates in the path. That
// is deliberate, because the caller inlines this into an HTML page as trusted
// markup, and a renderer that interpolated an SSID into an attribute would be
// an injection point in the one place the panel cannot escape it. The reader
// still needs a label; the caller supplies one in the surrounding HTML, where
// html/template escapes it.
//
// The dark modules are drawn as one path rather than as a rect each, which for
// a version 4 symbol is roughly one element instead of four hundred. Colour
// comes from CSS: the path uses currentColor and the background is left to the
// page, so the code inverts correctly if the panel is ever given a dark theme.
func (m *Matrix) SVG() string {
	span := m.size + 2*QuietZone
	var b strings.Builder
	b.Grow(m.size*m.size*8 + 256)

	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	b.WriteString(strconv.Itoa(span))
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(span))
	b.WriteString(`" shape-rendering="crispEdges" class="qr" focusable="false" aria-hidden="true">`)

	// The quiet zone is painted rather than left transparent. A transparent
	// quiet zone shows whatever is behind the element, and a phone pointed at
	// a page with a coloured background then sees no border at all.
	b.WriteString(`<rect width="`)
	b.WriteString(strconv.Itoa(span))
	b.WriteString(`" height="`)
	b.WriteString(strconv.Itoa(span))
	b.WriteString(`" class="qr-bg"/>`)

	b.WriteString(`<path class="qr-fg" fill="currentColor" d="`)
	for y := 0; y < m.size; y++ {
		for x := 0; x < m.size; x++ {
			if !m.At(x, y) {
				continue
			}
			b.WriteByte('M')
			b.WriteString(strconv.Itoa(x + QuietZone))
			b.WriteByte(' ')
			b.WriteString(strconv.Itoa(y + QuietZone))
			b.WriteString("h1v1h-1z")
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

// WiFiJoin builds the join string a phone camera understands:
//
//	WIFI:T:WPA;S:<ssid>;P:<passphrase>;H:<true|false>;;
//
// The format is the de facto one every mobile platform implements; it has no
// standards document, so the rules below were taken from the behaviour the
// platforms agree on.
//
// Escaping is the part that matters. A backslash, semicolon, comma, colon or
// double quote inside an SSID or a passphrase has to be escaped with a
// backslash, or the reader stops the field early and joins the wrong network,
// or worse, sends a truncated passphrase. WPA passphrases in particular are
// chosen by people and semicolons in them are not rare.
//
// The hex ambiguity is handled too: a value made only of hex digits is wrapped
// in double quotes, because a bare hex string is read as a raw key rather than
// as text.
func WiFiJoin(ssid, passphrase string, hidden bool) string {
	var b strings.Builder
	b.WriteString("WIFI:T:WPA;S:")
	b.WriteString(escapeWiFi(ssid))
	b.WriteString(";P:")
	b.WriteString(escapeWiFi(passphrase))
	b.WriteString(";H:")
	if hidden {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString(";;")
	return b.String()
}

func escapeWiFi(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	if isHexRun(s) {
		b.WriteByte('"')
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', ';', ',', ':', '"':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	if isHexRun(s) {
		b.WriteByte('"')
	}
	return b.String()
}

// isHexRun reports whether s would be mistaken for a raw hexadecimal key.
func isHexRun(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return len(s) > 0
}
