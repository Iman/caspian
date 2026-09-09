// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf8"

	share "caspianbyoc.org/caspian/third_party/libxray-share"
	"github.com/xtls/xray-core/infra/conf"
)

// A pasted text often holds more than one entry: a subscription is delivered as
// a list of links or a base64 blob of them, and a Clash profile is a list of
// proxies. Parse has always taken the first and said how many there were. This
// file adds the two calls the panel needs to let a person choose instead:
// ParseAll, which describes every entry without any credential, and Select,
// which returns one entry the way Parse returns the first.
//
// Groups are not modelled. The vendored Clash reader has no field for
// proxy-groups (third_party/libxray-share/clash_meta.go:13-15), so a profile
// collapses to its flat proxy list and every group is ignored. That is the
// measured behaviour and it is left as it is.

// maxTagBytes caps a display name. It is the same bound the panel puts on the
// label a user types (internal/panel/handlers.go, maxLabel), because the two
// sit next to each other on the page and a provider's name should not be able
// to take more room than the user's own.
const maxTagBytes = 64

// Entry is a display-safe description of one entry in a pasted list.
//
// It carries the same fields as Link and nothing else: no outbound, no user
// id, no password, no key. It is a value type with no unexported field, so
// there is nowhere in it for credential material to hide, and
// TestEntryAndListHaveNoSecretFields keeps it that way.
type Entry struct {
	// Index is the entry's position among the outbounds the parser accepted,
	// counting from zero. It is the value to hand to Select, and it is NOT
	// always the position in List.Entries: an entry the parser accepted but
	// this package refused (a bad id, a missing address) is left out of the
	// list and counted in Dropped, and the entries after it keep their
	// numbering so that a stored selection still points at the same entry.
	Index int `json:"index"`

	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     uint16 `json:"port"`

	// Tag is the display name from the link's #fragment or the Clash name.
	//
	// It is PROVIDER text, not the user's: a subscription can put anything
	// here. It is cleaned before it gets here, so it is at most maxTagBytes
	// bytes, cut on a rune boundary, and carries no control character. It is
	// not otherwise filtered, so it can hold right-to-left script, punctuation
	// and bidirectional formatting characters, and anything that renders it
	// must both escape it and isolate it (a bdi element or dir="ltr") so that
	// it cannot reorder the text around it. It must never reach a log line.
	Tag string `json:"tag"`

	Network     string   `json:"network"`
	Security    Security `json:"security"`
	ServerName  string   `json:"serverName,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Flow        string   `json:"flow,omitempty"`
	Reality     Reality  `json:"reality"`
}

// List is every usable entry in a pasted text, in the parser's order.
type List struct {
	Entries []Entry `json:"entries"`

	// Dropped is how many entries were in the text and are not in Entries. It
	// counts two things, because a person who pasted five lines and sees three
	// needs to know two went missing whichever way they went:
	//
	//   - lines the vendored parser rejected without a word, which is what it
	//     does with a line url.Parse refuses or an outbound it cannot build
	//     (third_party/libxray-share/parse_share.go:102-103 and :107-109). These
	//     are counted as the lines of the text that look like a share link minus
	//     the outbounds that came back. A Clash document and a raw JSON document
	//     have no such lines, so this part is zero for both;
	//   - outbounds the parser accepted and this package refused, for the
	//     reasons fill refuses them: no address, port zero, a malformed id, a
	//     malformed REALITY parameter, a removed transport.
	Dropped int `json:"dropped"`
}

// parsed is the parser's output before any entry is chosen or described.
type parsed struct {
	outbounds []conf.OutboundDetourConfig
	// dropped is the first of the two classes List.Dropped counts.
	dropped int
}

// parseOutbounds is the front half shared by Parse, ParseAll and Select: the
// checks on the first line, then the vendored parser.
func parseOutbounds(raw string) (*parsed, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, ErrEmpty
	}
	if err := checkScheme(text); err != nil {
		return nil, err
	}
	if err := checkTransport(text); err != nil {
		return nil, err
	}

	// The vendored parser's errors quote the user's input, so its error value
	// is dropped rather than wrapped. See the comment in errors.go.
	cfg, err := share.ConvertShareLinksToXrayJson(text)
	if err != nil {
		return nil, ErrNoLink
	}
	if cfg == nil || len(cfg.OutboundConfigs) == 0 {
		return nil, ErrNoLink
	}
	return &parsed{
		outbounds: cfg.OutboundConfigs,
		dropped:   droppedLines(text, len(cfg.OutboundConfigs)),
	}, nil
}

// linkAt builds the Link for outbound i, applying the corrections this package
// makes to whatever it is handed. It is what Parse has always done to outbound
// zero, done to the outbound that was asked for.
func linkAt(p *parsed, i int) (*Link, error) {
	ob := p.outbounds[i]
	name := clearSendThrough(&ob)
	ob.Tag = OutboundTag

	l := &Link{Tag: name, Index: i, Count: len(p.outbounds), outbound: &ob}
	if err := l.fill(); err != nil {
		return nil, err
	}
	return l, nil
}

// ParseAll reads the same inputs Parse reads and describes every usable entry.
//
// It returns an error only when Parse would: nothing usable at all. A text with
// some usable entries and some that are not returns the usable ones and counts
// the rest in Dropped, because that is the case the caller exists to show.
//
// The returned List holds no credential and is safe to render. It is not safe
// to log as a whole, because Entry.Tag is arbitrary provider text.
func ParseAll(raw string) (*List, error) {
	p, err := parseOutbounds(raw)
	if err != nil {
		return nil, err
	}
	list := &List{Dropped: p.dropped}
	var firstErr error
	for i := range p.outbounds {
		l, err := linkAt(p, i)
		if err != nil {
			// Counted, not listed. The first refusal is kept so that a text
			// with no usable entry at all fails with the same error Parse
			// gives for it, rather than with a generic one.
			if firstErr == nil {
				firstErr = err
			}
			list.Dropped++
			continue
		}
		list.Entries = append(list.Entries, l.entry())
	}
	if len(list.Entries) == 0 {
		return nil, firstErr
	}
	return list, nil
}

// Select reads the same inputs Parse reads and returns entry i with its
// outbound, so that the config document XrayConfig produces names that entry.
//
// An index outside the list is not an error. It returns entry zero and reports
// the clamp in the second value, so a stored selection that outlives the list
// it was made from (a shorter re-paste, a hand-edited state file) cannot leave
// the box unable to start; the caller says so on screen instead. A negative
// index is treated the same way.
//
// An entry that is in range and that this package refuses IS an error, the same
// one Parse returns for it as a single link: the caller asked for that entry and
// sliding to a neighbour would connect through a server they did not choose.
func Select(raw string, i int) (*Link, bool, error) {
	p, err := parseOutbounds(raw)
	if err != nil {
		return nil, false, err
	}
	clamped := false
	if i < 0 || i >= len(p.outbounds) {
		i, clamped = 0, true
	}
	l, err := linkAt(p, i)
	if err != nil {
		return nil, false, err
	}
	return l, clamped, nil
}

// entry is the display-safe view of a Link. The Tag is cleaned here and only
// here; Link.Tag keeps the raw fragment because Parse is unchanged and Redacted
// already quotes it with %q.
func (l *Link) entry() Entry {
	return Entry{
		Index:       l.Index,
		Protocol:    l.Protocol,
		Address:     l.Address,
		Port:        l.Port,
		Tag:         cleanTag(l.Tag),
		Network:     l.Network,
		Security:    l.Security,
		ServerName:  l.ServerName,
		Fingerprint: l.Fingerprint,
		Flow:        l.Flow,
		Reality:     l.Reality,
	}
}

// cleanTag removes every control character and caps the result at maxTagBytes
// on a rune boundary. Nothing else is removed: the name is shown to the person
// who pasted it, and a name with its punctuation stripped is a different name.
//
// Control characters are removed rather than escaped because the value goes
// into an HTML page, where html/template escapes what needs escaping, and the
// panel's job is to show the name, not a picture of its bytes. A terminal
// escape sequence in a fragment is the case this guards against; Redacted
// handles the same case for the log with %q.
func cleanTag(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if len(out) <= maxTagBytes {
		return out
	}
	cut := maxTagBytes
	for cut > 0 && !utf8.RuneStart(out[cut]) {
		cut--
	}
	return out[:cut]
}

// vendoredShareSchemes is the list the vendored parser's line dispatch keys on,
// third_party/libxray-share/parse_share.go:74-77, repeated here so that
// droppedLines follows the same fork the parser took. It is a prefix list, not
// the scheme regex, because that is what the parser uses.
var vendoredShareSchemes = []string{
	"vless://", "vmess://", "socks://", "ss://", "trojan://",
	"hysteria2://", "hy2://",
}

// droppedLines counts the share-link-shaped lines of text that did not come
// back as outbounds. It retraces the vendored parser's dispatch
// (parse_share.go:51-61) without changing it:
//
//   - a document starting with "{" is raw JSON: no lines, nothing to count;
//   - a first line with a share scheme means the text was read line by line;
//   - otherwise the parser tried base64 and, if that decoded, read the
//     decoded text line by line; if it did not decode, the text was YAML and
//     there is nothing to count.
//
// A line "looks like a share link" when the scheme regex matches it. A line of
// notes with no scheme is also dropped by the parser and is NOT counted here,
// deliberately: the sentence the panel builds from this number says lines
// "could not be read", and a line that never claimed to be a link was not
// misread. The count is clamped at zero so a disagreement between this retrace
// and the parser can only under-report, never invent a drop.
func droppedLines(text string, accepted int) int {
	if strings.HasPrefix(text, "{") {
		return 0
	}
	lines := strings.ReplaceAll(text, "\r\n", "\n")
	if !firstLineHasVendoredScheme(lines) {
		decoded, ok := decodeBase64Like(lines)
		if !ok {
			return 0
		}
		lines = strings.ReplaceAll(decoded, "\r\n", "\n")
	}
	n := 0
	for _, line := range strings.Split(lines, "\n") {
		if schemeRE.MatchString(strings.TrimSpace(line)) {
			n++
		}
	}
	if n < accepted {
		return 0
	}
	return n - accepted
}

func firstLineHasVendoredScheme(text string) bool {
	first := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		first = text[:i]
	}
	first = strings.TrimSpace(first)
	for _, p := range vendoredShareSchemes {
		if strings.HasPrefix(first, p) {
			return true
		}
	}
	return false
}

// decodeBase64Like retraces third_party/libxray-share/parse_share.go:20-42:
// standard, then URL-safe, then URL-safe with the padding restored.
func decodeBase64Like(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if b, err := base64.StdEncoding.DecodeString(text); err == nil {
		return string(b), true
	}
	if b, err := base64.URLEncoding.DecodeString(text); err == nil {
		return string(b), true
	}
	s := strings.ReplaceAll(strings.ReplaceAll(text, "-", "+"), "_", "/")
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", false
	}
	return string(b), true
}
