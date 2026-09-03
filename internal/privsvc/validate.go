// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"errors"
	"fmt"
	"net/netip"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/state"
)

// maxConfigBytes bounds StartRequest.ConfigJSON.
//
// The document internal/link produces holds ONE outbound. The largest of the
// protocols the vendored parser emits is a few kilobytes with every REALITY
// field set; 64 KiB is far more than any of them and small enough that a
// hostile or faulty caller cannot spend the box's memory through this door. The
// panel bounds its own HTTP bodies at 256 KiB for the same reason
// (internal/panel/panel.go, maxBodyBytes); this is the second gate, on the
// other side of the socket, because the panel is not trusted here.
const maxConfigBytes = 64 << 10

// configFromRequest turns the document that crossed the socket back into a
// parsed link.
//
// # Why this is a re-parse and not a decode
//
// internal/xcfg.Build composes the complete engine document from a *link.Link,
// and a *link.Link can only be produced by link.Parse: its outbound is an
// unexported field, on purpose, so that the credential material cannot be
// reached by encoding/json or by a fmt verb. What crosses the socket is the
// OUTPUT of that type, panel.StartRequest.ConfigJSON, which internal/link
// documents as "a JSON document that internal/link built from parsed
// structures, never the text the user pasted".
//
// The vendored parser accepts a raw Xray JSON object as one of its four input
// shapes (third_party/libxray-share/parse_share.go, ConvertShareLinksToXrayJson:
// "a single Xray JSON object (starts with '{')"), so feeding that document back
// in returns the same link. Measured in TestConfigDocumentSurvivesTheRoundTrip:
// the protocol, address, port and security survive, and a second XrayConfig
// call is byte-identical to the first.
//
// That round trip is not a workaround, it is the validation this boundary is
// supposed to perform. Every check internal/link makes runs again on this side:
// the address is present, the port is not zero, the UUID is a real UUID rather
// than one of the 1-to-30-character strings xray-core silently SHA-1s into a
// different valid identity (common/uuid/uuid.go), trojan carries TLS, and a TLS
// stream has a server name. The panel runs them too. The privileged side does
// not trust that it did.
func configFromRequest(configJSON []byte) (*link.Link, error) {
	switch {
	case len(configJSON) == 0:
		return nil, fail("config", panel.FaultEngineRejectedConfig,
			errors.New("the request carried no configuration document"))
	case len(configJSON) > maxConfigBytes:
		return nil, fail("config", panel.FaultEngineRejectedConfig,
			fmt.Errorf("the configuration document is %d bytes and the limit is %d", len(configJSON), maxConfigBytes))
	}
	l, err := link.Parse(string(configJSON))
	if err != nil {
		// internal/link's errors are written not to quote the user's input;
		// see internal/link/errors.go. It is still not sent across the socket:
		// only the Fault word travels.
		return nil, fail("config", panel.FaultEngineRejectedConfig, err)
	}
	return l, nil
}

// validateStatic checks the fields of a request that need no knowledge of this
// machine. It runs before detection so that a request that could never work is
// refused without running a single command.
func (s *Service) validateStatic(req panel.StartRequest) error {
	if !panel.ValidEngineLogLevel(req.EngineLogLevel) {
		// The value is not quoted. It is a caller-supplied string, and a
		// refusal that echoes what it was given is a way to get arbitrary text
		// into a root process's log.
		return fail("engine log level", panel.FaultUnknown,
			errors.New("the engine log level is not one of the levels this appliance sends"))
	}

	// The two policy fields. internal/state refuses to persist either of them
	// empty, "because empty must never be readable as 'let client traffic
	// out'", and this side refuses the same thing rather than defaulting: a
	// default here would make a caller that forgot the field indistinguishable
	// from one that meant the safe value.
	if req.Network.DNSMode != state.DNSModeTunnel {
		return fail("dns mode", panel.FaultUnknown,
			errors.New("client DNS is answered on the box and resolved through the tunnel, and this build supports nothing else"))
	}
	if req.Network.OnTunnelDown != state.OnTunnelDownBlock {
		return fail("tunnel-down policy", panel.FaultUnknown,
			errors.New("forwarded client traffic is blocked when the tunnel drops, and this build supports nothing else"))
	}

	// The third policy field, refused the same way and for a sharper reason.
	//
	// Empty is refused rather than defaulted, exactly as above: a caller that
	// forgot the field and one that meant the safe value must not be the same
	// request. And every non-blocking value is refused because this build
	// cannot carry client IPv6, whatever the firewall is told:
	// internal/netcfg puts no IPv6 address on the hotspot or the tunnel
	// device, installs no IPv6 route into the tunnel table and no IPv6 policy
	// rule, and nothing in this repository advertises a prefix, so a client
	// could neither address nor route a v6 packet. netcfg.IPv6Forward flips
	// the firewall and one sysctl and stops there;
	// netcfg.TestIPv6Forward_InstallsNoIPv6AddressingOrRouting is the record
	// of what is missing, and lifting this refusal means landing that work
	// first, not deleting these four lines.
	//
	// FaultIPv6Unsupported rather than FaultUnknown, because the user changed
	// one setting and can put it back. A generic fault would send them to
	// their config, which is not what is wrong.
	if req.Network.ClientIPv6 != state.ClientIPv6Block {
		return fail("client IPv6 policy", panel.FaultIPv6Unsupported,
			errors.New("hotspot clients get no IPv6, and this build supports nothing else"))
	}

	if err := hotspot.ValidatePassphrase(req.Hotspot.Passphrase); err != nil {
		// internal/hotspot decides. Its message names no value.
		return fail("hotspot password", panel.FaultUnknown, err)
	}
	switch {
	case req.Hotspot.SSID == "":
		return fail("hotspot name", panel.FaultUnknown, errors.New("the hotspot has no name"))
	case len(req.Hotspot.SSID) > hotspot.MaxSSIDLen:
		return fail("hotspot name", panel.FaultUnknown,
			fmt.Errorf("the hotspot name is %d bytes and the 802.11 limit is %d", len(req.Hotspot.SSID), hotspot.MaxSSIDLen))
	}

	if req.Hotspot.Country != "" && !isUpperAlpha2(req.Hotspot.Country) {
		return fail("country", panel.FaultUnknown,
			errors.New("the country override is not a two-letter code"))
	}
	if req.Hotspot.Band != "" {
		switch hotspot.Band(req.Hotspot.Band) {
		case hotspot.Band2GHz, hotspot.Band5GHz:
		default:
			return fail("band", panel.FaultUnknown, errors.New("the band override is not a band this appliance uses"))
		}
	}
	if req.Hotspot.Subnet != "" {
		p, err := netip.ParsePrefix(req.Hotspot.Subnet)
		if err != nil {
			return fail("hotspot subnet", panel.FaultUnknown, errors.New("the hotspot subnet override is not a network in CIDR form"))
		}
		switch {
		case !p.Addr().Is4():
			return fail("hotspot subnet", panel.FaultUnknown, errors.New("the hotspot subnet must be an IPv4 network"))
		case !p.Addr().IsPrivate():
			// A public range here would put the hotspot on addresses that
			// belong to somebody else and make every client's return traffic
			// somebody else's problem.
			return fail("hotspot subnet", panel.FaultUnknown, errors.New("the hotspot subnet must be a private network"))
		case p.Bits() < 8 || p.Bits() > 30:
			return fail("hotspot subnet", panel.FaultUnknown, errors.New("the hotspot subnet has no usable range of addresses in it"))
		}
	}
	return nil
}

// validateAgainstFacts checks the fields that name something on this machine,
// against what this machine reported about itself.
//
// This is the check internal/panel/priv.go asks for by name: "the privileged
// side is expected to validate each one against what it detected for itself
// rather than trusting it."
func (s *Service) validateAgainstFacts(req panel.StartRequest, f netcfg.Facts) error {
	if name := req.Network.InternetInterface; name != "" {
		if !netcfg.ValidInterfaceNameOn(s.cfg.netOptions().Platform, name) {
			return fail("internet interface", panel.FaultNoInternetInterface,
				errors.New("the chosen internet interface is not a usable interface name"))
		}
		if _, ok := f.LinkByName(name); !ok {
			return fail("internet interface", panel.FaultNoInternetInterface,
				errors.New("the chosen internet interface is not on this machine"))
		}
		carries := false
		for _, r := range f.Routes {
			if r.Family == 4 && r.Dev == name && !r.LinkDown {
				carries = true
				break
			}
		}
		if !carries {
			return fail("internet interface", panel.FaultNoInternetInterface,
				errors.New("the chosen internet interface has no default route on it"))
		}
	}

	if name := req.Hotspot.Interface; name != "" && !s.isVirtualAPName(name) {
		if !netcfg.ValidInterfaceNameOn(s.cfg.netOptions().Platform, name) {
			return fail("hotspot interface", panel.FaultNoAPAdapter,
				errors.New("the chosen hotspot interface is not a usable interface name"))
		}
		if !apCapable(f, name) {
			return fail("hotspot interface", panel.FaultNoAPAdapter,
				errors.New("the chosen adapter did not report that it can create a hotspot"))
		}
	}
	return nil
}

// isVirtualAPName reports whether the name is the one this appliance CREATES
// for the access point when the radio already carries a station link.
//
// Such an interface does not exist yet, so internal/netcfg's HotspotOverride
// would not match it and PlanNetwork would refuse. It is nevertheless a name
// the panel can legitimately send back, because detectionFrom lists it among
// the AP candidates so that basic mode can name the interface in force. It is
// therefore read as "no override": choosing the interface the planner would
// have created anyway is not an override at all.
func (s *Service) isVirtualAPName(name string) bool {
	return name != "" && name == s.cfg.netOptions().APIfaceName
}

// apCapable reports whether name is an interface on, or the name of, a radio
// that reported AP support in this machine's own "iw list" output.
//
// Both spellings are accepted because internal/netcfg accepts both:
// chooseHotspot matches an override against the interface name and the radio
// name.
func apCapable(f netcfg.Facts, name string) bool {
	for _, phy := range f.Phys {
		if !phy.SupportsAP() {
			continue
		}
		if phy.Name == name {
			return true
		}
		for _, w := range f.Wireless {
			if w.Phy == phy.Name && w.Name == name {
				return true
			}
		}
	}
	return false
}

// validateAgainstPlan checks the settings that only mean something once the
// radio has been chosen.
//
// The channel is the one that matters. internal/netcfg reads the radio's own
// interface combinations and reports ChannelPinned when the radio can only be
// on one channel at a time, which on the measured Raspberry Pi 5 built-in radio
// is the normal case. Asking hostapd for a different channel there either makes
// it refuse to start or makes the driver move the station link, dropping the
// box's own internet. Neither is readable from the outside, so it is refused
// here with the one Fault that says exactly this.
func (s *Service) validateAgainstPlan(req panel.StartRequest, p *netcfg.Plan) error {
	ch := req.Hotspot.Channel
	if ch == 0 {
		return nil
	}
	if p.ChannelPinned && ch != p.Channel {
		return fail("channel", panel.FaultChannelRefused,
			fmt.Errorf("the radio can only be on one channel at a time and is on %d", p.Channel))
	}
	if len(p.UsableChannel) > 0 {
		for _, u := range p.UsableChannel {
			if u == ch {
				return nil
			}
		}
		return fail("channel", panel.FaultChannelRefused,
			errors.New("the chosen channel is not one this adapter reported it can start an access point on"))
	}
	return nil
}
