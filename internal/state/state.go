// SPDX-License-Identifier: AGPL-3.0-or-later

// Package state is the only place in Caspian that writes persistent
// configuration. Everything that has to survive a reboot lives here, and no
// other package touches the disk for configuration.
//
// Two things it stores are credentials: the proxy config the user pasted, and
// the hotspot passphrase. A third, the panel password, is never stored at all,
// only a verifier for it. The protections are therefore structural rather than
// advisory:
//
//   - the file is 0600 inside a 0700 directory, and Load refuses a file that any
//     other user on the box could read (see perm_unix.go),
//   - the credential-bearing types render as "[redacted]" through fmt, so a
//     stray %v in a log line cannot leak them (see Secret),
//   - the package imports no logger and writes nothing to stdout or stderr.
//
// Design references are to docs/2026-08-29-design.md.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// CurrentVersion is the schema version this build writes. Load accepts anything
// at or below it and migrates forward; anything above it is refused, because a
// newer release may have moved a field this build would silently drop on the
// next Save. See migrate.go.
//
// Raised from 1 to 2 on 2026-08-30 by the addition of Advanced.ClientIPv6. A v1
// file has no such key, so it decodes empty, and empty is exactly the reading
// this package promises no policy field will ever produce. The migration makes
// the blocking value explicit. A v1 file therefore still loads; only a v2 file
// read by a v1 build is refused, which is ErrFutureVersion telling the user to
// update rather than a silent field drop.
const CurrentVersion = 2

// DefaultDir is where the appliance keeps its state. Callers may override it;
// nothing in this package assumes it.
const DefaultDir = "/var/lib/caspian"

// FileName is the state file inside the state directory.
const FileName = "state.json"

// redacted is what every credential-bearing type renders as through fmt.
const redacted = "[redacted]"

// Policy defaults. These are the fail-closed positions the design commits to,
// and they are the values a first run and a migrated v0 file receive.
//
// The set of *other* legal values is deliberately not enumerated here. Choosing
// DNS behaviour and drop behaviour is internal/netcfg's job; state's job is to
// guarantee that neither field is ever empty, because an empty value read by a
// downstream package must never be able to mean "let client traffic out".
const (
	// DNSModeTunnel answers client DNS on the box and resolves it through the
	// tunnel. Design section 6.
	DNSModeTunnel = "tunnel"

	// OnTunnelDownBlock stops forwarded client traffic when the tunnel drops.
	// Design sections 6 and 7: a TUN device disappearing takes its routes with
	// it, so the block has to be a firewall rule rather than the absence of a
	// route.
	OnTunnelDownBlock = "block"

	// ClientIPv6Block stops hotspot clients getting IPv6 at all. Design
	// section 7 requires IPv6 to have its own rules, and internal/netcfg
	// implements the block as three mechanisms rather than one: forwarding off,
	// the firewall dropping forwarded IPv6 on the hotspot in both directions,
	// and the firewall dropping router advertisements out of the hotspot so a
	// client cannot autoconfigure an address in the first place.
	//
	// It is the only value this build supports, and internal/privsvc names the
	// refusal for anything else. That is not a placeholder: carrying client
	// IPv6 needs client addressing and IPv6 routing into the tunnel, and
	// neither exists. netcfg.TestIPv6Forward_InstallsNoIPv6AddressingOrRouting
	// records exactly what is missing.
	ClientIPv6Block = "block"
)

// Secret is a string that will not print by accident.
//
// The requirement was "no String method that could print it". A type with no
// String method at all is the weaker choice: fmt falls back to the underlying
// string kind and prints the credential in full. Defining String (and GoString,
// for %#v) to return a fixed placeholder is strictly safer, because fmt's
// handleMethods is consulted for %v, %s, %q, %x and %X, and is also consulted
// for exported struct fields nested inside a larger value. So a State printed
// with %v redacts its secrets without the caller doing anything.
//
// encoding/json does not consult String, so the real value still round-trips to
// disk. Reading the real value in Go requires the explicit Reveal call, which is
// greppable in review.
type Secret string

// String satisfies fmt.Stringer with a placeholder. See the type comment.
func (Secret) String() string { return redacted }

// GoString satisfies fmt.GoStringer so %#v redacts too.
func (Secret) GoString() string { return redacted }

// Reveal returns the underlying value. Every call site is a place where a
// credential leaves this package, so this name exists to make them findable.
func (s Secret) Reveal() string { return string(s) }

// IsZero reports whether the secret is unset, without revealing it.
func (s Secret) IsZero() bool { return len(s) == 0 }

// State is the whole of the persistent configuration.
//
// It contains no slices, maps or pointers on purpose. That makes a struct
// assignment a complete deep copy, which is what lets Store hand out snapshots
// to concurrent readers with no lock and no aliasing risk. Adding a reference
// type here breaks that guarantee, so add a value type or fix Snapshot.
type State struct {
	Version   int           `json:"version"`
	Proxy     ProxyConfig   `json:"proxy"`
	Hotspot   HotspotConfig `json:"hotspot"`
	Panel     PanelAuth     `json:"panel"`
	Advanced  Advanced      `json:"advanced"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ProxyConfig is the config the user pasted, plus what was detected from it.
// Design sections 5.2 and 5.4.
type ProxyConfig struct {
	// Raw is the pasted share link, raw xray JSON, or subscription text,
	// stored verbatim. It is untrusted input (design section 6) and it is a
	// credential: it carries the UUID, the REALITY private material and the
	// server address.
	Raw Secret `json:"raw"`

	// Scheme is what detection made of it, for example "vless". Not secret:
	// basic mode shows it on the status line.
	Scheme string `json:"scheme,omitempty"`

	// Label is the user's own name for this config. Not secret.
	Label string `json:"label,omitempty"`

	AddedAt time.Time `json:"added_at,omitzero"`
}

// Fingerprint identifies a stored config in diagnostics without revealing any
// part of it: the first eight hex digits of the SHA-256 of the raw text. Two
// log lines can be compared for "same config or not" with no credential in
// either. Empty when no config is stored.
func (p ProxyConfig) Fingerprint() string {
	if p.Raw.IsZero() {
		return ""
	}
	sum := sha256.Sum256([]byte(p.Raw))
	return hex.EncodeToString(sum[:4])
}

// IsConfigured reports whether a config has been pasted.
func (p ProxyConfig) IsConfigured() bool { return !p.Raw.IsZero() }

// HotspotConfig is the access point the box publishes. Design section 5.2.
type HotspotConfig struct {
	// SSID is shown in basic mode and printed into a join QR code, so it is
	// not secret.
	SSID string `json:"ssid,omitempty"`

	// Passphrase is the WPA passphrase. It is shown on the panel to the person
	// standing at the box, and it must never reach a log.
	Passphrase Secret `json:"passphrase"`
}

// PanelAuth holds the verifier for the panel password. Design section 5.6
// requires a password in every case, on the first screen.
//
// The plaintext is never a field here, never a parameter beyond
// Store.SetPanelPassword and Store.VerifyPanelPassword, and never persisted.
type PanelAuth struct {
	// PasswordHash is a PHC-format argon2id string. See password.go.
	//
	// It is a Secret rather than a string, even though it is not the password.
	// It is an offline cracking target, so disclosing it costs the strength of
	// the user's password choice and nothing more, which is not nothing. Making
	// it a Secret also closes a real gap: %#v walks a struct's exported fields
	// and would have printed a plain string in full, where Secret's GoString
	// redacts it.
	PasswordHash Secret `json:"password_hash,omitempty"`
}

// IsSet reports whether a panel password has been chosen.
func (p PanelAuth) IsSet() bool { return !p.PasswordHash.IsZero() }

// Advanced holds every override the advanced-mode toggle exposes. Design
// section 5.3 lists them: which interface is which, channel, band and country,
// the hotspot subnet, DNS behaviour, the engine log, and what happens when the
// tunnel drops.
//
// Two different zero-value conventions apply here, and the split is deliberate.
//
// Detection fields (interfaces, channel, band, country, subnet, log level) use
// the zero value to mean "not overridden, detect this". That is unambiguous
// because no valid setting for any of them has a zero value: there is no
// interface named "", and channel 0 does not exist. Design section 5.4 requires
// that every automatic choice remain overridable, which is exactly what a
// non-zero value here records.
//
// Policy fields (DNSMode, OnTunnelDown, ClientIPv6) never hold a zero value,
// because empty is not a safe reading for any of them. Save refuses to persist
// them empty.
type Advanced struct {
	// InternetInterface is the uplink, normally derived from the default
	// route. Design section 4.7.
	InternetInterface string `json:"internet_interface,omitempty"`

	// HotspotInterface must report AP support. Design section 4.7: interface
	// names are never hardcoded.
	HotspotInterface string `json:"hotspot_interface,omitempty"`

	// Channel is the 802.11 channel. On the measured Pi 5 the built-in radio
	// reports "#{ AP } <= 1" and is pinned to the client link's channel
	// (design section 4.6), so an override here can be impossible to honour.
	// Recording it is still correct; refusing it is netcfg's call, not state's.
	Channel int `json:"channel,omitempty"`

	// Band is the radio band. The values are "2.4GHz" and "5GHz", spelled
	// exactly as internal/hotspot declares them in Band2GHz and Band5GHz.
	//
	// This comment said "2.4" and "5" until 2026-08-30, which was wrong and
	// wrong in a way that hides. APConfig.Validate refuses an unrecognised
	// band, so a caller that believed the comment would store a value that
	// makes the access point fail to start, and the symptom is a hotspot that
	// never appears rather than an error naming the field.
	Band string `json:"band,omitempty"`

	// Country is the regulatory domain, an ISO 3166-1 alpha-2 code.
	Country string `json:"country,omitempty"`

	// Subnet is the hotspot subnet in CIDR form. Detection picks one that does
	// not clash with the network the box is already on (design section 5.4).
	Subnet string `json:"subnet,omitempty"`

	// DNSMode is how client DNS is handled. Never empty; see the type comment.
	DNSMode string `json:"dns_mode"`

	// OnTunnelDown is what happens to forwarded client traffic when the tunnel
	// drops. Never empty; see the type comment.
	OnTunnelDown string `json:"on_tunnel_down"`

	// ClientIPv6 is whether hotspot clients get IPv6. Never empty, same reason
	// as the two above, and with the sharpest version of it: a client with a
	// working IPv6 path prefers it over IPv4 and would bypass the tunnel
	// entirely, so an empty value read as "no policy configured" is a leak
	// rather than an inconvenience.
	//
	// ClientIPv6Block is the only value this build supports. It is stored
	// rather than assumed so that a future release that can carry client IPv6
	// has somewhere to record the choice, and so that turning it on is one
	// deliberate edit in one place rather than a rebuild.
	ClientIPv6 string `json:"client_ipv6"`

	// EngineLogLevel is the xray-core log level. Design section 9 warns that
	// engine error strings embed the private key, the seed, the short id and
	// the UUID, so raising this is a decision with a disclosure cost. state
	// records the choice; redaction of the output is the panel's job.
	EngineLogLevel string `json:"engine_log_level,omitempty"`

	// PanelOnLAN opens the panel on the local network as well as the hotspot
	// interface. Design section 5.6: the hotspot interface always, the local
	// network only if the user turns it on, never the uplink.
	//
	// This is the one field where false is both the default and a meaningful
	// user choice. That is safe here only because the two coincide: false is
	// the closed position.
	PanelOnLAN bool `json:"panel_on_lan,omitempty"`
}

// defaultState is the usable starting point a first run gets. It is not Go's
// zero value: the policy fields carry their fail-closed defaults, so a caller
// that reads state before the user has configured anything still reads a safe
// answer rather than an empty one.
func defaultState() State {
	return State{
		Version: CurrentVersion,
		Advanced: Advanced{
			DNSMode:      DNSModeTunnel,
			OnTunnelDown: OnTunnelDownBlock,
			ClientIPv6:   ClientIPv6Block,
		},
	}
}

// Redacted renders the state for diagnostics with every credential replaced.
// It is the only rendering of State that is safe to log or to show in a support
// bundle.
//
// It is built field by field rather than with %v on the whole struct, both
// because that would recurse through String and because an explicit list is
// what makes "no secret can appear here" reviewable: a field added to State
// does not appear in this output until someone adds it deliberately.
func (s State) Redacted() string {
	var b strings.Builder
	fmt.Fprintf(&b, "version=%d", s.Version)
	fmt.Fprintf(&b, " updated_at=%s", formatTime(s.UpdatedAt))

	fmt.Fprintf(&b, " proxy.configured=%t", s.Proxy.IsConfigured())
	if s.Proxy.Scheme != "" {
		fmt.Fprintf(&b, " proxy.scheme=%s", s.Proxy.Scheme)
	}
	if s.Proxy.Label != "" {
		fmt.Fprintf(&b, " proxy.label=%q", s.Proxy.Label)
	}
	if fp := s.Proxy.Fingerprint(); fp != "" {
		// A hash prefix, not the config: enough to tell two configs apart in a
		// log without disclosing either.
		fmt.Fprintf(&b, " proxy.fingerprint=%s", fp)
	}
	fmt.Fprintf(&b, " proxy.raw=%s", redacted)

	fmt.Fprintf(&b, " hotspot.ssid=%q", s.Hotspot.SSID)
	fmt.Fprintf(&b, " hotspot.passphrase=%s", redacted)

	// The stored hash is not the password, but it is an offline cracking
	// target, so only its presence and its algorithm parameters are reported.
	fmt.Fprintf(&b, " panel.password_set=%t", s.Panel.IsSet())
	if s.Panel.IsSet() {
		fmt.Fprintf(&b, " panel.password_kdf=%s", describeHash(s.Panel.PasswordHash.Reveal()))
	}

	fmt.Fprintf(&b, " adv.internet_if=%q", s.Advanced.InternetInterface)
	fmt.Fprintf(&b, " adv.hotspot_if=%q", s.Advanced.HotspotInterface)
	fmt.Fprintf(&b, " adv.channel=%d", s.Advanced.Channel)
	fmt.Fprintf(&b, " adv.band=%q", s.Advanced.Band)
	fmt.Fprintf(&b, " adv.country=%q", s.Advanced.Country)
	fmt.Fprintf(&b, " adv.subnet=%q", s.Advanced.Subnet)
	fmt.Fprintf(&b, " adv.dns_mode=%q", s.Advanced.DNSMode)
	fmt.Fprintf(&b, " adv.on_tunnel_down=%q", s.Advanced.OnTunnelDown)
	fmt.Fprintf(&b, " adv.client_ipv6=%q", s.Advanced.ClientIPv6)
	fmt.Fprintf(&b, " adv.engine_log_level=%q", s.Advanced.EngineLogLevel)
	fmt.Fprintf(&b, " adv.panel_on_lan=%t", s.Advanced.PanelOnLAN)

	return b.String()
}

// String makes the redacted rendering the default one, so that a State reaching
// fmt by accident, directly or nested inside something else, is already safe.
func (s State) String() string { return s.Redacted() }

// GoString does the same for %#v, which would otherwise walk the struct field
// by field and print whatever a future field happens to hold. Covering both
// verbs means the safety does not depend on every field being a Secret.
func (s State) GoString() string { return s.Redacted() }

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.UTC().Format(time.RFC3339)
}
