// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// Package link is the adapter between a share link the user pasted and a
// config document the engine can be handed.
//
// It sits on top of the vendored MIT parser in
// caspianbyoc.org/caspian/third_party/libxray-share and does three things that
// the parser does not: it clears the field the parser puts the link's display
// name in, it validates the parts of the link the engine will silently accept
// when they are wrong, and it produces a description of the link that is safe
// to render in the panel and safe to write to a log.
package link

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	share "caspianbyoc.org/caspian/third_party/libxray-share"
	"github.com/xtls/xray-core/infra/conf"
)

// OutboundTag is the tag given to the outbound in every config this package
// emits. It is a constant rather than the user's display name so that routing
// rules written elsewhere in this program have a fixed name to refer to, and
// so that no user-supplied text ends up as a config identifier.
const OutboundTag = "proxy"

// Security is the security layer a link asks for.
type Security string

const (
	SecurityNone    Security = "none"
	SecurityTLS     Security = "tls"
	SecurityReality Security = "reality"
)

// Reality records which REALITY parameters a link carried. It records
// presence, never the value: publicKey, shortId and mldsa65Verify are
// credential material and this type is rendered by the panel.
type Reality struct {
	HasPublicKey     bool `json:"hasPublicKey"`
	HasShortID       bool `json:"hasShortId"`
	HasMldsa65Verify bool `json:"hasMldsa65Verify"`
}

// Link is a normalised, display-safe description of one parsed share link.
//
// It carries no credential material. There is no field here for the user id,
// the shadowsocks or trojan password, the hysteria auth string, the REALITY
// public key, the short id or the post-quantum verify key. The config that
// does carry them is held in an unexported field and is only ever produced as
// bytes by XrayConfig.
type Link struct {
	// Protocol is the engine's name for the protocol: vless, vmess,
	// shadowsocks, trojan, socks or hysteria.
	Protocol string `json:"protocol"`

	// Address and Port are the server the engine will dial.
	Address string `json:"address"`
	Port    uint16 `json:"port"`

	// Tag is the display name taken from the link's #fragment. It may be
	// empty and it is arbitrary user-supplied text, so anything that renders
	// it must escape it.
	Tag string `json:"tag"`

	// Network is the transport, in the engine's vocabulary rather than the
	// URI's: a link saying type=raw reports tcp here, and type=ws reports
	// websocket. The mapping is TransportProtocol.Build,
	// github.com/xtls/xray-core/infra/conf/transport_internet.go:998-1024.
	Network string `json:"network"`

	// Security is none, tls or reality.
	Security Security `json:"security"`

	// ServerName is the SNI and Fingerprint is the uTLS fingerprint. Neither
	// is secret: the first is sent in the clear in the handshake, the second
	// describes the shape of that handshake.
	ServerName  string `json:"serverName,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`

	// Flow is the VLESS flow, empty for every other protocol. Not secret, and
	// a common cause of a tunnel that connects and carries nothing.
	Flow string `json:"flow,omitempty"`

	// Reality is populated only when Security is reality.
	Reality Reality `json:"reality"`

	// Count is how many usable links were found in the pasted text. This Link
	// describes the first; the rest were discarded, and the panel needs to be
	// able to say so. It counts what the parser accepted, not what was pasted:
	// parsePlainShareLines drops a line it cannot read and says nothing
	// (third_party/libxray-share/parse_share.go:104-106), so a paste of five
	// lines of which two are malformed reports three.
	Count int `json:"count"`

	// outbound holds the credential material. It is unexported so that
	// encoding/json cannot reach it, and it is a pointer so that the fmt
	// verbs that walk a struct print an address rather than its contents.
	// String below closes that hole properly; this is the second lock.
	outbound *conf.OutboundDetourConfig
}

// schemeRE matches the scheme of a URI-shaped first line. It is deliberately
// narrower than RFC 3986 so that base64 subscription blobs and raw xray JSON,
// neither of which is URI-shaped, fall through to the parser untouched.
var schemeRE = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.\-]*)://`)

// supportedSchemes is the set the vendored parser handles, from
// third_party/libxray-share/parse_share.go:74-77 and the switch at :127-142.
var supportedSchemes = map[string]bool{
	"vless":     true,
	"vmess":     true,
	"ss":        true,
	"socks":     true,
	"trojan":    true,
	"hysteria2": true,
	"hy2":       true,
}

// Parse reads one share link, several separated by newlines, or a base64
// subscription blob, and returns a description of the first link found.
//
// The returned Link is safe to render and to log. The credential material
// stays inside it and only leaves through XrayConfig.
func Parse(raw string) (*Link, error) {
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

	ob := cfg.OutboundConfigs[0]
	name := clearSendThrough(&ob)
	ob.Tag = OutboundTag

	l := &Link{Tag: name, Count: len(cfg.OutboundConfigs), outbound: &ob}
	if err := l.fill(); err != nil {
		return nil, err
	}
	return l, nil
}

// checkScheme rejects a URI-shaped input whose scheme is not one of the
// supported ones. Without it the parser falls through to its Clash YAML branch
// and returns a YAML unmarshal error quoting the first characters of the
// user's text, which is both confusing and a small disclosure.
func checkScheme(text string) error {
	first := text
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = strings.TrimSpace(first[:i])
	}
	m := schemeRE.FindStringSubmatch(first)
	if m == nil {
		return nil
	}
	scheme := strings.ToLower(m[1])
	if supportedSchemes[scheme] {
		return nil
	}
	// The scheme is a fixed vocabulary word, not credential material, so it is
	// safe to name. Nothing after it is quoted.
	return fmt.Errorf("%w: %s", ErrUnsupportedScheme, scheme)
}

// removedTransports are transports a link may still ask for that the engine no
// longer has, mapped to the shape that replaced each one.
//
// Without this a link naming one reaches the vendored parser, which drops it
// per-line and reports the generic ErrNoLink: accurate, and no help at all to
// somebody holding a link that says quic. A Clash document naming the same
// transport already gets a transport-specific error, because that path reaches
// the engine's own TransportProtocol.Build. This gives the URI path parity.
//
// The replacement text is not a guess. The engine names it in its own removal
// message: HTTP and QUIC were "migrated to XHTTP stream-one H2 & H3", and XHTTP
// selects the HTTP version from the TLS ALPN, so alpn=h3 is what asks for the
// QUIC shape. gun is different: the vendored parser maps it onto gRPC settings
// but leaves the network name as "gun", which the engine does not know.
//
// The keys are a fixed vocabulary, so naming the matched one back to the user
// discloses nothing from the pasted text. That is the same reasoning that lets
// checkScheme name the scheme.
var removedTransports = map[string]string{
	"quic": "xhttp with security=tls and alpn=h3",
	"h3":   "xhttp with security=tls and alpn=h3",
	"h2":   "xhttp with security=tls",
	"http": "xhttp with security=tls",
	"gun":  "grpc",
}

// checkTransport reports a link asking for a removed transport, by name.
func checkTransport(text string) error {
	first := text
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = strings.TrimSpace(first[:i])
	}
	if schemeRE.FindStringSubmatch(first) == nil {
		return nil
	}
	u, err := url.Parse(first)
	if err != nil {
		// Not parseable as a URI. Leave it to the parser, which will report
		// that nothing usable was found.
		return nil
	}
	name := strings.ToLower(strings.TrimSpace(u.Query().Get("type")))
	replacement, removed := removedTransports[name]
	if !removed {
		return nil
	}
	return fmt.Errorf("%w: %s, replaced by %s", ErrUnsupportedTransport, name, replacement)
}

// clearSendThrough removes the display name the vendored parser stores in the
// outbound's SendThrough field, and returns it.
//
// This is the whole reason the function exists.
// third_party/libxray-share/xray_json.go:28-30 assigns the URI fragment to
// SendThrough for every protocol, and the engine reads that field as a bind
// address: OutboundDetourConfig.Build at
// github.com/xtls/xray-core/infra/conf/xray.go:267-281 rejects any value that
// parses as a domain other than "origin" or "srcip" with "unable to send
// through: <name>". A pointer to the empty string is still not nil, so a link
// with no fragment fails the same way. Measured 2026-08-30: eight of eight
// fixtures failed to build before this ran and all eight built after.
func clearSendThrough(ob *conf.OutboundDetourConfig) string {
	name := ""
	if ob.SendThrough != nil {
		name = *ob.SendThrough
		ob.SendThrough = nil
	}
	return name
}

// outboundSettings is the subset of the engine's client settings this adapter
// reads back. The JSON keys are the ones the engine's own structs marshal to,
// and they are the same three across every protocol the parser emits: see
// infra/conf/vless.go:233-241, vmess.go:105-113, shadowsocks.go:172-181,
// trojan.go:30-37, socks.go:72-79 and hysteria.go:12-16.
type outboundSettings struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
	ID      string `json:"id"`
}

// fill populates the display fields and runs every check that the engine would
// either not run or run too late.
func (l *Link) fill() error {
	ob := l.outbound
	l.Protocol = ob.Protocol

	var s outboundSettings
	if ob.Settings != nil {
		if err := json.Unmarshal(*ob.Settings, &s); err != nil {
			return ErrNoLink
		}
	}
	l.Address, l.Port = s.Address, s.Port
	if l.Address == "" {
		return ErrBadAddress
	}
	if l.Port == 0 {
		// The engine accepts port 0 and then dials nothing useful.
		return ErrBadPort
	}
	if needsUUID(l.Protocol) && !validUUID(s.ID) {
		return fmt.Errorf("%w: check the id in the %s link you pasted", ErrBadUUID, l.Protocol)
	}
	// Both run before fillStream so the corrected stream is what gets reported,
	// and after the address check because the second one needs the address.
	// Order matters between them: the fallback only acts on a TLS stream, and
	// for a query-less trojan link it is the first call that makes it one.
	// The first is keyed on the protocol because only trojan has the leak; the
	// second is deliberately keyed on nothing. See each function for why.
	l.requireTLSForTrojan()
	l.fillMissingServerName()
	return l.fillStream()
}

// requireTLSForTrojan makes sure this package never emits a trojan outbound
// without TLS.
//
// THIS assignment is what stops the credential leak. The server-name fallback
// in fillMissingServerName below is a different fix for a different failure and
// does NOT cover this one: an outbound with security "none" never reaches the
// TLS stack at all, so no server name, from anywhere, would help it. Do not
// remove the security assignment on the belief that the fallback has it.
//
// Trojan's entire threat model is that it is indistinguishable from ordinary
// HTTPS. Without TLS the password crosses the wire in the clear and the
// protocol has no reason to exist, so an outbound in that shape is not a
// working config with a weaker property, it is a broken one that leaks a
// credential.
//
// The vendored parser holds the right rule and does not always reach it. It
// sets security to tls for trojan in parseSecurityFromURL
// (third_party/libxray-share/stream.go:66-71), but streamSettings returns
// (nil, nil) before ever calling that when the URI carries no query parameters
// (stream.go:11-14). So the minimal and most common form,
// trojan://password@host:443#name, arrives with no stream settings at all and
// the engine's default is a plain TCP stream with no security. Measured
// 2026-08-30: that form, and only that form, loses TLS; a trojan link with any
// query parameter, including an explicit security=none, is already corrected
// upstream.
//
// The correction lives here rather than in the vendored copy so that copy stays
// pristine and pinned, and can be re-vendored from upstream without local
// patches to re-apply. It sits beside clearSendThrough and dropNulls, which are
// the other two places this package corrects what it is handed.
func (l *Link) requireTLSForTrojan() {
	if l.Protocol != "trojan" {
		return
	}
	ss := l.outbound.StreamSetting
	if ss == nil {
		ss = &conf.StreamConfig{}
		l.outbound.StreamSetting = ss
	}
	if ss.Security == "" || ss.Security == string(SecurityNone) {
		ss.Security = string(SecurityTLS)
	}
}

// fillMissingServerName writes the link's address in as the TLS server name
// whenever the outbound is TLS and the link did not carry an sni parameter.
//
// This is NOT what stops the trojan credential leak; requireTLSForTrojan above
// is. What this stops is a link that parses cleanly, looks right in the panel,
// and then dies inside the TLS stack before a byte leaves the box, which is a
// failure a user cannot diagnose.
//
// Measured 2026-08-30 against crypto/tls: a client config with no ServerName
// and InsecureSkipVerify false fails with "tls: either ServerName or
// InsecureSkipVerify must be specified in the tls.Config", raised before the
// network is touched. TestEmptyTLSServerNameIsAHardError pins that, because
// this whole function rests on it.
//
// # Why the rule is not keyed on protocol or transport
//
// It was, twice, and both times the key was a proxy for the real question:
// will anything downstream fill the name in? That answer differs per transport,
// and inside gRPC it differs per address family. Enumerated 2026-08-30 over
// every client dialer in xray-core v1.260327.0:
//
//	transport                                       fills serverName?
//	------------------------------------------------------------------
//	raw/tcp      tcp/dialer.go:43                   yes, WithDestination
//	websocket    websocket/dialer.go:77             yes, WithDestination
//	kcp          kcp/dialer.go:114                  yes, WithDestination
//	splithttp    splithttp/dialer.go:113            yes, WithDestination
//	httpupgrade  httpupgrade/dialer.go:68           yes, WithDestination
//	grpc         grpc/dial.go:139-142               only if the address is a domain
//	hysteria     hysteria/dialer.go:457             no, bare GetTLSConfig()
//
// The option itself is transport/internet/tls/config.go:490-496. A rule that
// has to track that table will rot the first time upstream changes one row, and
// it will rot silently, because the symptom is a hard TLS failure in a narrow
// combination nobody tests. "If it is TLS and the name is empty, fill it"
// cannot rot, because it does not depend on the table at all. The table stays
// here as the evidence for the shape of the rule, not as its input.
//
// # Why filling is a superset that agrees with the engine
//
// The engine fills from the destination address; this fills from the address
// the user typed. Those are the same value before routing, and this one is
// steadier after: the engine's fallback runs AFTER routing, so if
// targetStrategy ever moves off "asis" it would put a resolved IP in the SNI
// while this keeps the name the user typed. Where the engine acts, this changes
// nothing.
//
// # allowInsecure is not an exception, for two measured reasons
//
// allowInsecure is the one case where an empty server name could be deliberate,
// since InsecureSkipVerify satisfies crypto/tls on its own. It gets no special
// treatment here, and neither reason is a judgement call.
//
// First, the engine does not treat it specially either: WithDestination ignores
// AllowInsecure completely, testing only config.ServerName == ""
// (config.go:490-496). Measured 2026-08-30 by calling the engine directly, with
// AllowInsecure true and WithDestination, the result is ServerName
// "example.invalid" for a domain destination and "203.0.113.9" for an IP one,
// identical to AllowInsecure false. TestEngineFillsServerNameEvenWhenInsecure
// pins that.
//
// Second, and decisive: in xray-core v1.260327.0 a config carrying allowInsecure
// does not load at all. transport_internet.go:709-716 refuses it outright once
// the wall clock is past 2026-06-01, so the combination this exception would
// have served is unreachable. Note that it is the WALL CLOCK, not the version,
// which matters on an appliance whose clock can be wrong on boot.
// TestAllowInsecureIsRejectedByTheEngine_KnownTrap pins it.
//
// The one consequence of filling that is worth naming: where the name was empty
// and now is not, an SNI extension appears where none did before. Measured on
// the wire, a ClientHello with an empty ServerName is 1490 bytes and carries no
// hostname, and with the name filled in it is 1514. That is the same handshake
// every other transport already sends, and a handshake with no SNI is the
// unusual one for anything pretending to be ordinary HTTPS.
//
// An IP literal is filled in rather than refused, and that was measured too:
// crypto/tls accepts an IP-literal ServerName, and the ClientHello it produces
// is 1490 bytes with no SNI extension, exactly as if the field were empty. So
// for an IP the fill satisfies the config check and changes nothing on the wire.
func (l *Link) fillMissingServerName() {
	ss := l.outbound.StreamSetting
	if ss == nil || ss.Security != string(SecurityTLS) {
		// Not a TLS stream. reality is excluded here rather than handled: its
		// server name lives in REALITYSettings.ServerName, a different field
		// with a different meaning, and checkReality already covers it.
		return
	}
	if ss.TLSSettings == nil {
		ss.TLSSettings = &conf.TLSConfig{}
	}
	if ss.TLSSettings.ServerName == "" {
		ss.TLSSettings.ServerName = l.Address
	}
}

// fillStream reads the transport and security layer.
//
// A nil StreamSetting is normal, not an error: the vendored parser returns one
// for any link with no query parameters at all
// (third_party/libxray-share/stream.go:11-14). The engine's default for that
// case is a plain TCP stream with no security, which is what is reported here.
func (l *Link) fillStream() error {
	l.Network, l.Security = "tcp", SecurityNone
	l.Flow = flowOf(l.outbound)

	ss := l.outbound.StreamSetting
	if ss == nil {
		return nil
	}
	if ss.Network != nil {
		// Build is used rather than the raw string because it is also the
		// engine's own validation: it returns an error for the transports that
		// have been removed, so a link naming one is rejected here with a
		// sentence instead of failing later inside the engine.
		network, err := ss.Network.Build()
		if err != nil {
			return ErrUnsupportedTransport
		}
		l.Network = network
	}
	switch ss.Security {
	case string(SecurityTLS):
		l.Security = SecurityTLS
		if ss.TLSSettings != nil {
			l.ServerName = ss.TLSSettings.ServerName
			l.Fingerprint = ss.TLSSettings.Fingerprint
		}
	case string(SecurityReality):
		l.Security = SecurityReality
		if err := checkReality(ss.REALITYSettings); err != nil {
			return err
		}
		if r := ss.REALITYSettings; r != nil {
			l.ServerName = r.ServerName
			l.Fingerprint = r.Fingerprint
			l.Reality = Reality{
				HasPublicKey:     r.PublicKey != "" || r.Password != "",
				HasShortID:       r.ShortId != "",
				HasMldsa65Verify: r.Mldsa65Verify != "",
			}
		}
	}
	return nil
}

// flowOf reads the VLESS flow out of the settings blob. Only VLESS has one
// (infra/conf/vless.go:239); reading the key unconditionally would pick up
// nothing for the others, but the protocol check keeps the intent visible.
func flowOf(ob *conf.OutboundDetourConfig) string {
	if ob.Protocol != "vless" || ob.Settings == nil {
		return ""
	}
	var v struct {
		Flow string `json:"flow"`
	}
	if err := json.Unmarshal(*ob.Settings, &v); err != nil {
		return ""
	}
	return v.Flow
}

// xrayConfig is the document handed to the engine. It carries outbounds only.
// Inbounds, logging, DNS and routing are decided elsewhere in this program and
// must not be taken from anything the user pasted.
type xrayConfig struct {
	Outbounds []conf.OutboundDetourConfig `json:"outbounds"`
}

// XrayConfig returns the JSON config document for the engine.
//
// It holds exactly one outbound, tagged OutboundTag, with SendThrough cleared.
// Everything is re-serialised from the parsed structures; no string from the
// user is interpolated into the document.
//
// The document is guaranteed to be one the engine can decode and build. It is
// not a guarantee that the server will answer, and it is not a claim that the
// engine will accept every value in it: a value that is well formed and out of
// range, such as a hysteria2 bandwidth below the engine's floor, is the
// engine's to reject and this package does not pre-empt that.
func (l *Link) XrayConfig() ([]byte, error) {
	if l == nil || l.outbound == nil {
		return nil, ErrNoLink
	}
	ob := *l.outbound
	// Cleared again on the way out. Parse already did it, but this function is
	// the last point before the bytes reach a privileged process, and the cost
	// of the assignment is nothing next to the cost of the bug.
	ob.SendThrough = nil
	ob.Tag = OutboundTag

	raw, err := json.Marshal(xrayConfig{Outbounds: []conf.OutboundDetourConfig{ob}})
	if err != nil {
		return nil, errSerialise
	}
	return dropNulls(raw)
}

// errSerialise covers the encoding/json failures in XrayConfig. They are not
// reachable from any input this package accepts, since everything being
// marshalled was decoded from JSON moments earlier, so this is not an exported
// sentinel: nothing should be branching on it.
var errSerialise = errors.New("the config could not be serialised")

// dropNulls removes every object key whose value is JSON null.
//
// This is not tidying. A null is not the same as an absent key once the
// document goes back through encoding/json, because the engine's config
// structs hold json.RawMessage fields, and unmarshalling a JSON null into a
// json.RawMessage yields the four bytes "null" rather than a nil slice. The
// engine tests those fields for nil.
//
// The case that made this necessary: conf.REALITYConfig has Target and Dest as
// json.RawMessage (transport_internet.go:783-784), and Build branches on
// "c.Dest != nil" at :815 to decide whether it is configuring a server or a
// client. Marshalling a client REALITY config writes "dest":null; reading that
// document back makes Dest non-nil, Build takes the server branch, and the
// engine rejects the config with `empty "serverNames"`. Measured 2026-08-30:
// every REALITY link failed this way until the nulls were removed, and none
// failed after.
//
// Removing a null is safe for every other field shape: for a pointer or a map
// or a slice a null decodes to nil, which is what an absent key gives, and for
// a value field a null leaves the zero value, which is also what an absent key
// gives. Nulls inside arrays are left alone, because removing an element would
// change the meaning of the array.
func dropNulls(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Numbers stay as written rather than passing through float64, so nothing
	// is rounded on the way through.
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, errSerialise
	}
	out, err := json.Marshal(stripNulls(doc))
	if err != nil {
		return nil, errSerialise
	}
	return out, nil
}

func stripNulls(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if val == nil {
				continue
			}
			out[k] = stripNulls(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = stripNulls(val)
		}
		return out
	default:
		return v
	}
}

// Redacted returns a one-line description that is safe to show a user and safe
// to write to a log.
//
// Values are never included, only whether they are present. The display name
// is quoted with %q because it is arbitrary user text: quoting escapes control
// characters, which keeps an escape sequence in a link's #fragment from
// reaching a terminal that would act on it.
func (l *Link) Redacted() string {
	if l == nil {
		return "no link"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s to %s:%d", l.Protocol, l.Address, l.Port)
	fmt.Fprintf(&b, ", transport %s, security %s", l.Network, l.Security)
	if l.ServerName != "" {
		fmt.Fprintf(&b, ", sni %s", l.ServerName)
	}
	if l.Fingerprint != "" {
		fmt.Fprintf(&b, ", fingerprint %s", l.Fingerprint)
	}
	if l.Flow != "" {
		fmt.Fprintf(&b, ", flow %s", l.Flow)
	}
	if l.Security == SecurityReality {
		fmt.Fprintf(&b, ", reality: publicKey %s, shortId %s, mldsa65Verify %s",
			present(l.Reality.HasPublicKey),
			present(l.Reality.HasShortID),
			present(l.Reality.HasMldsa65Verify))
	}
	fmt.Fprintf(&b, ", name %q", l.Tag)
	if l.Count > 1 {
		fmt.Fprintf(&b, " (first of %d links found)", l.Count)
	}
	return b.String()
}

// String returns Redacted.
//
// It exists for one reason. Without a String method, fmt prints a struct's
// unexported fields too, so a single log.Printf("%+v", l) somewhere else in
// the program would be enough to write the user's keys to disk. With it, every
// fmt verb that would have walked the struct calls this instead. The
// requirement is that Link has no String method that prints secrets, and this
// one prints none: it is Redacted, unchanged.
func (l *Link) String() string { return l.Redacted() }

func present(ok bool) string {
	if ok {
		return "set"
	}
	return "absent"
}
