// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package engine

import "strings"

// Reason is the coarse class of a refusal from this package: not what the
// engine said, which is redacted text for advanced mode, but which of a few
// known situations the text describes, so that the panel can give a sentence
// with a remedy instead of the general "the engine would not load it".
//
// The classes are the ones measured, not the ones imagined. On 2026-09-12 a
// Windows 11 report (issue #2) failed with a listener that could not bind
// because another proxy program held port 10808, and a survey of 8,600 public
// share links found 247 refused for allowInsecure and 24 for a Shadowsocks
// cipher xray-core no longer ships. Everything else stays ReasonOther and
// keeps the general sentence.
type Reason string

const (
	// ReasonOther is every refusal this package has no better word for.
	ReasonOther Reason = "other"

	// ReasonPortInUse: an inbound could not listen because the address was
	// taken. The config is fine; another program on the machine is not.
	ReasonPortInUse Reason = "port-in-use"

	// ReasonInsecureRemoved: the config carries tlsSettings.allowInsecure,
	// which xray-core removed in favour of pinnedPeerCertSha256 and now
	// refuses outright. Share links write it as insecure=1.
	ReasonInsecureRemoved Reason = "insecure-removed"

	// ReasonCipherRemoved: a Shadowsocks method the engine no longer knows,
	// such as aes-256-cfb or rc4-md5.
	ReasonCipherRemoved Reason = "cipher-removed"
)

// ReasonOf classifies an error from Validate or Start. It works on the text
// because that is all xray-core gives back: its errors are strings joined with
// " > ", not typed values. Redact leaves every marker below in place, which
// TestRedactKeepsTheReasonMarkers pins, so the classification is the same
// before and after redaction.
func ReasonOf(err error) Reason {
	if err == nil {
		return ReasonOther
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "failed to listen"):
		// "transport/internet: failed to listen on address: 127.0.0.1:10808
		// > ... > bind: address already in use" on Linux and macOS; on Windows
		// the last segment reads "bind: Only one usage of each socket address
		// (protocol/network address/port) is normally permitted."
		return ReasonPortInUse
	case strings.Contains(msg, `"allowInsecure" has been removed`):
		return ReasonInsecureRemoved
	case strings.Contains(msg, "unknown cipher method"):
		return ReasonCipherRemoved
	}
	return ReasonOther
}
