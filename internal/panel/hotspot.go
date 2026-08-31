// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"unicode"
	"unicode/utf8"

	"caspianbyoc.org/caspian/internal/hotspot"
)

// The hotspot name and passphrase, as the panel presents them.
//
// This file owns the WORDS and nothing else. The rules, the limits, the
// alphabet and the generator all live in internal/hotspot, which is the package
// that renders the hostapd configuration and therefore the package that knows
// what hostapd will accept.
//
// That split is deliberate and it replaced a second implementation of the same
// thing. An earlier draft of this file generated its own passphrase from its
// own 32-symbol alphabet, with its own note about masking rather than modulo,
// character for character the same as hotspot.passphraseAlphabet. Two
// implementations of one security-relevant rule do not stay identical: the day
// somebody widens or narrows that alphabet, one of them changes, both test
// suites stay green, and the difference is found by a user whose passphrase the
// two halves of the program disagree about. The panel's copy was the weaker one
// as well, at 80 bits against 100 and with no list of banned defaults.
//
// So: internal/hotspot decides whether a value is legal, and this file decides
// what to say about it. Where the two could drift, the code below is written so
// that internal/hotspot always wins.

// SuggestPassphrase returns a generated WPA passphrase for the form.
//
// It is hotspot.GeneratePassphrase: 20 symbols from a 32-symbol alphabet, so
// 100 bits, from an alphabet with the shapes people misread on a screen taken
// out. That length is chosen against the real attack, which is a captured WPA2
// handshake ground offline, and no rate limit helps with that.
//
// A failure returns the empty string rather than a fallback. There is no safe
// constant to fall back to: internal/hotspot keeps a list of banned
// passphrases precisely because the implementation this project replaces
// shipped one fixed default on every box. An empty suggestion leaves the user
// to type their own, which is a worse experience and not a weaker box.
func SuggestPassphrase() string {
	p, err := hotspot.GeneratePassphrase()
	if err != nil {
		return ""
	}
	return p
}

// SuggestSSID returns a generated network name.
//
// The random suffix is so that two boxes in one building do not publish the
// same name, which produces a roaming mess a non-technical user cannot
// diagnose. It is drawn from the same generator, so there is still only one
// source of randomness and one alphabet in the program.
func SuggestSSID() string {
	p, err := hotspot.GeneratePassphrase()
	if err != nil || len(p) < 4 {
		return "Caspian"
	}
	return "Caspian-" + p[:4]
}

// BandChoice is one option on the band selector.
//
// The values come from internal/hotspot rather than being written out here.
// That matters more than it looks: hotspot.Band2GHz is the string "2.4GHz",
// and internal/state's comment on Advanced.Band gives "2.4" and "5" as its
// examples. Those are not the same strings. A panel that stored "2.4" would be
// storing a value that hotspot.APConfig.Validate refuses, and the failure would
// surface as a hotspot that never appears. See the note in the package report.
type BandChoice struct {
	Value string
	// WordsKey is a message key rather than a sentence, so the band selector
	// reads in the user's language like everything else.
	WordsKey Key
}

// BandChoices are the bands the panel offers.
var BandChoices = []BandChoice{
	{Value: string(hotspot.Band2GHz), WordsKey: MsgAdvBand24},
	{Value: string(hotspot.Band5GHz), WordsKey: MsgAdvBand5},
}

// ValidBand reports whether v is a band the panel will store. Empty means "let
// Caspian decide", which is internal/state's convention for every detection
// field.
func ValidBand(v string) bool {
	if v == "" {
		return true
	}
	for _, b := range BandChoices {
		if b.Value == v {
			return true
		}
	}
	return false
}

// ValidateSSID checks a network name and returns words for the user.
//
// The length limit is hotspot.MaxSSIDLen rather than a number written here, so
// the panel cannot come to disagree with the package that enforces it. The
// remaining rules are duplicated rather than called, because internal/hotspot
// applies them inside APConfig.Validate and does not export an SSID validator
// on its own; building a throwaway APConfig with invented values for the
// interface, country, channel and band, purely to have the SSID checked, would
// fail on the invented values rather than on the name. That is a seam worth
// closing in internal/hotspot with an exported ValidateSSID, and it is not this
// package's to close.
func ValidateSSID(ssid string) Problem {
	switch {
	case ssid == "":
		return Problem{Headline: MsgSSIDMissing, Advice: MsgSSIDMissingAdvice}
	case len(ssid) > hotspot.MaxSSIDLen:
		// The limit is 32 OCTETS rather than 32 characters, which is why the
		// message talks about non-English letters: a Persian or Arabic name
		// reaches the limit in about 16 characters, and a user who counted
		// characters would think the panel was wrong.
		return Problem{Headline: MsgSSIDTooLong, Advice: MsgSSIDTooLongAdvice}
	case !utf8.ValidString(ssid):
		return Problem{Headline: MsgSSIDBadChars, Advice: MsgSSIDBadCharsAdvice}
	case hasControlChars(ssid):
		return Problem{Headline: MsgSSIDBadChars, Advice: MsgSSIDBadCharsAdvice}
	case onlySpaces(ssid):
		return Problem{Headline: MsgSSIDSpaces, Advice: MsgSSIDSpacesAdvice}
	}
	return Problem{}
}

// ValidatePassphrase checks a WPA passphrase and returns words for the user.
//
// internal/hotspot decides. This function asks it first and refuses whatever it
// refuses; the switch afterwards only chooses which sentence to show. The
// default branch is what makes that safe against drift: a rule added to
// internal/hotspot that this file has never heard of still refuses the value
// here, with a sentence that is true whatever the reason was, instead of the
// panel quietly accepting something the hotspot will not start with.
func ValidatePassphrase(pass string) Problem {
	if err := hotspot.ValidatePassphrase(pass); err == nil {
		return Problem{}
	}
	switch {
	case len(pass) < hotspot.MinPassphraseLen:
		return Problem{Headline: MsgPassTooShort, Advice: MsgPassTooShortAdvice}
	case len(pass) > hotspot.MaxPassphraseLen:
		return Problem{Headline: MsgPassTooLong, Advice: MsgPassTooLongAdvice}
	case !isASCIIPrintable(pass):
		return Problem{Headline: MsgPassBadChars, Advice: MsgPassBadCharsAdvice}
	default:
		// Reached today by the banned-defaults list, and tomorrow by whatever
		// internal/hotspot adds next.
		return Problem{Headline: MsgPassRefused, Advice: MsgPassRefusedAdvice}
	}
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func onlySpaces(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// isASCIIPrintable reports whether every byte is printable ASCII.
//
// It exists to pick a sentence, not to decide legality; internal/hotspot has
// already decided that. The rule it mirrors is at
// internal/hotspot/apconfig.go, in ValidatePassphrase, and the reason there is
// sharper than "some devices mangle it": a newline in a passphrase would end
// the wpa_passphrase line in the generated hostapd configuration and let the
// rest of the value be read as further hostapd directives.
func isASCIIPrintable(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}
