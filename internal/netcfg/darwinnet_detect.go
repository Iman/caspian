// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// darwinPfStatusKnob is a pseudo knob recorded by darwinDetect beside the real
// sysctl values: "1" when pf reports Enabled, "0" when Disabled. It lives in
// Facts.Sysctl so that the pf-enable step gets an exact inverse the same way a
// sysctl change does, and is absent when "pfctl -s info" could not be read.
const darwinPfStatusKnob = "pf.status"

// darwinDetect reads a Mac and fills the same Facts a Raspberry Pi fills, so
// that PlanNetwork's decision ladder runs unchanged.
//
// Every command is read-only. The radio is modelled as one Phy per Wi-Fi
// hardware port with AP among its modes and NO station-plus-AP combination,
// which is the measured truth for Internet Sharing: the built-in radio hosts
// the network or joins one, and the planner therefore refuses the built-in
// Wi-Fi as the hotspot whenever it is also the uplink.
func darwinDetect(ctx context.Context, r Runner, knobs []string) (Facts, error) {
	f := Facts{CapturedAt: time.Now().UTC(), Sysctl: map[string]string{}}

	res, err := r.Run(ctx, Command{
		Path: BinIfconfig, Args: []string{"-a"},
		Why: "interfaces and the addresses on them, which a hotspot subnet must not collide with",
	})
	if err != nil {
		return f, fmt.Errorf("netcfg: list interfaces: %w", err)
	}
	f.Links = ParseIfconfig(res.Stdout)

	// route(8) exits 1 with "not in table" when there is no default route, and
	// the runner reports a non-zero exit as an error. That is a machine with
	// no uplink, which PlanNetwork says in words; it is not a detection
	// failure.
	if res, err := r.Run(ctx, Command{
		Path: BinRoute, Args: []string{"-n", "get", "default"},
		Why: "the uplink is whichever interface carries the default route",
	}); err == nil {
		if dr, ok := ParseRouteGet(res.Stdout); ok {
			dr.Family = 4
			f.Routes = append(f.Routes, dr)
		}
	}
	if res, err := r.Run(ctx, Command{
		Path: BinRoute, Args: []string{"-n", "get", "-inet6", "default"},
		Why: "an IPv6 server address needs an IPv6 gateway to pin a host route through",
	}); err == nil {
		if dr, ok := ParseRouteGet(res.Stdout); ok {
			dr.Family = 6
			f.Routes = append(f.Routes, dr)
		}
	}

	res, err = r.Run(ctx, Command{
		Path: BinNetworksetup, Args: []string{"-listallhardwareports"},
		Why: "which interface is the Wi-Fi radio; on a Mac only the built-in radio can host the access point",
	})
	if err != nil {
		return f, fmt.Errorf("netcfg: list hardware ports: %w", err)
	}
	for _, hp := range ParseHardwarePorts(res.Stdout) {
		if hp.Device == "" {
			continue
		}
		// Best effort, name based, the same standing iproute2's "parentbus"
		// has on Linux: a port Apple names "USB ..." is a USB adapter and an
		// empty string means "not reported", never "not USB".
		if strings.HasPrefix(hp.Port, "USB ") {
			for i := range f.Links {
				if f.Links[i].Name == hp.Device {
					f.Links[i].Bus = "usb"
				}
			}
		}
		if !hp.IsWiFi() {
			continue
		}
		w := WirelessIface{
			Name:    hp.Device,
			Phy:     darwinPhyName(hp.Device),
			Type:    "managed",
			Manager: ManagedByNothing,
		}
		if res, err := r.Run(ctx, Command{
			Path: BinNetworksetup, Args: []string{"-getairportnetwork", hp.Device},
			Why: "whether the radio is joined to a network, asked directly rather than inferred",
		}); err == nil {
			ssid, associated, known := ParseAirportNetwork(res.Stdout)
			if known {
				w.LinkKnown = true
				w.Associated = associated
				w.SSID = ssid
			}
		}
		f.Wireless = append(f.Wireless, w)
		// Internet Sharing can only put Apple's built-in radio into AP mode.
		// USB Wi-Fi devices may still be detected and shown to the user, but
		// must not become candidates handed to Apple.
		f.Phys = append(f.Phys, darwinPhy(w.Phy, hp.IsBuiltInWiFi()))
	}

	if len(knobs) > 0 {
		args := append([]string{"-e", "--"}, knobs...)
		if res, err := r.Run(ctx, Command{
			Path: BinSysctl, Args: args,
			Why: "read the values to restore on teardown",
		}); err == nil {
			f.Sysctl = ParseSysctl(res.Stdout)
		}
	}
	if res, err := r.Run(ctx, Command{
		Path: BinPfctl, Args: []string{"-s", "info"},
		Why: "whether pf is enabled; an anchor loaded into a disabled pf enforces nothing",
	}); err == nil {
		if enabled, ok := ParsePfStatus(res.Stdout); ok {
			if enabled {
				f.Sysctl[darwinPfStatusKnob] = "1"
			} else {
				f.Sysctl[darwinPfStatusKnob] = "0"
			}
		}
	}
	return f, nil
}

// darwinPhyName is the radio name this package gives a Wi-Fi hardware port.
// macOS has no phy concept; the name only has to be stable and distinct.
func darwinPhyName(device string) string { return "radio-" + device }

// darwinPhy describes what Internet Sharing lets the built-in radio do.
//
// Modes list AP because the radio can host a network, and the combination list
// is EMPTY because it cannot host one while joined to one: the planner reads
// an empty list as "AP only on a free radio", which is the honest model.
//
// The channels are the ones Internet Sharing's channel menu offers on a
// current Mac: the 2.4 GHz channels 1 to 11 and the non-DFS 5 GHz channels.
// DFS channels are deliberately absent, as they are on Linux.
func darwinPhy(name string, apCapable bool) Phy {
	var b24, b5 Band
	b24.Number = 1
	for ch := 1; ch <= 11; ch++ {
		b24.Frequencies = append(b24.Frequencies, Frequency{MHz: 2407 + 5*ch, Channel: ch})
	}
	b5.Number = 2
	for _, ch := range []int{36, 40, 44, 48, 149, 153, 157, 161} {
		b5.Frequencies = append(b5.Frequencies, Frequency{MHz: 5000 + 5*ch, Channel: ch})
	}
	modes := []string{"managed"}
	if apCapable {
		modes = append(modes, "AP")
	}
	return Phy{Name: name, Modes: modes, Bands: []Band{b24, b5}}
}

// HardwarePort is one block of "networksetup -listallhardwareports".
type HardwarePort struct {
	Port   string // "Wi-Fi", "Ethernet", "USB 10/100/1000 LAN", "Thunderbolt Bridge"
	Device string // "en0"
}

// IsWiFi reports whether the port is a Wi-Fi radio. Apple has named it
// "Wi-Fi" since 10.7 and "AirPort" before that.
func (h HardwarePort) IsWiFi() bool {
	return h.Port == "Wi-Fi" || h.Port == "AirPort" || strings.HasPrefix(h.Port, "Wi-Fi ") ||
		strings.HasPrefix(h.Port, "USB Wi-Fi")
}

// IsBuiltInWiFi identifies the radio Apple Internet Sharing can host. A USB
// Wi-Fi driver can appear in this listing as a wireless hardware port, but
// macOS has no supported AP backend for those devices.
func (h HardwarePort) IsBuiltInWiFi() bool {
	return h.Port == "Wi-Fi" || h.Port == "AirPort"
}

// ParseHardwarePorts reads "networksetup -listallhardwareports". The Ethernet
// Address lines are read past and never kept: a MAC identifies one machine
// and nothing here needs it.
func ParseHardwarePorts(out string) []HardwarePort {
	var ports []HardwarePort
	var cur *HardwarePort
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "Hardware Port: "):
			ports = append(ports, HardwarePort{Port: strings.TrimPrefix(line, "Hardware Port: ")})
			cur = &ports[len(ports)-1]
		case strings.HasPrefix(line, "Device: ") && cur != nil:
			cur.Device = strings.TrimPrefix(line, "Device: ")
		}
	}
	return ports
}

// ParseAirportNetwork reads "networksetup -getairportnetwork <dev>".
//
//	Current Wi-Fi Network: <name>
//	You are not associated with an AirPort network.
//
// known is false for any other wording, including a powered-off radio, so a
// caller never reads silence as "not joined".
func ParseAirportNetwork(out string) (ssid string, associated bool, known bool) {
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if rest, ok := strings.CutPrefix(line, "Current Wi-Fi Network: "); ok {
			return rest, true, true
		}
		if strings.HasPrefix(line, "You are not associated with") {
			return "", false, true
		}
	}
	return "", false, false
}

// ParseRouteGet reads "route -n get default".
//
//	   route to: default
//	destination: default
//	       mask: default
//	    gateway: 192.0.2.1
//	  interface: en7
//	      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
//
// A default route with an interface and no gateway line is on-link, which the
// planner handles the way it handles a Linux "default dev X" route.
func ParseRouteGet(out string) (DefaultRoute, bool) {
	var dr DefaultRoute
	for _, raw := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(raw, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "gateway":
			// A link-local IPv6 gateway prints as fe80::1%en0.
			if i := strings.IndexByte(v, '%'); i > 0 {
				v = v[:i]
			}
			if a, err := netip.ParseAddr(v); err == nil {
				dr.Gateway = a
			}
		case "interface":
			dr.Dev = v
		}
	}
	if dr.Dev == "" {
		return DefaultRoute{}, false
	}
	return dr, true
}

// ParsePfStatus reads the first line of "pfctl -s info": "Status: Enabled for
// 0 days 00:01:02" or "Status: Disabled".
func ParsePfStatus(out string) (enabled bool, ok bool) {
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if rest, found := strings.CutPrefix(line, "Status: "); found {
			switch {
			case strings.HasPrefix(rest, "Enabled"):
				return true, true
			case strings.HasPrefix(rest, "Disabled"):
				return false, true
			}
		}
	}
	return false, false
}

var ifconfigHeader = regexp.MustCompile(`^([A-Za-z0-9._-]+): flags=[0-9a-fA-F]+<([A-Z0-9_,]*)>`)

// ParseIfconfig reads "ifconfig -a" into Links.
//
// Only the header, inet and inet6 lines are read. The ether line, which is the
// MAC address, is skipped on purpose. State is "UP" when the UP flag is set,
// which is what the planner's LinkDown logic needs; a point-to-point utun is
// UP without RUNNING and is still a usable interface.
func ParseIfconfig(out string) []Link {
	var links []Link
	var cur *Link
	for _, raw := range strings.Split(out, "\n") {
		if m := ifconfigHeader.FindStringSubmatch(raw); m != nil {
			state := "DOWN"
			for _, fl := range strings.Split(m[2], ",") {
				if fl == "UP" {
					state = "UP"
				}
			}
			links = append(links, Link{Name: m[1], State: state})
			cur = &links[len(links)-1]
			continue
		}
		if cur == nil {
			continue
		}
		fields := strings.Fields(raw)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "inet":
			addr, err := netip.ParseAddr(fields[1])
			if err != nil {
				continue
			}
			bits := 32
			for i := 2; i+1 < len(fields); i++ {
				if fields[i] == "netmask" {
					if b, ok := maskBits(fields[i+1]); ok {
						bits = b
					}
				}
			}
			cur.Prefixes = append(cur.Prefixes, netip.PrefixFrom(addr, bits))
		case "inet6":
			a := fields[1]
			if i := strings.IndexByte(a, '%'); i > 0 {
				a = a[:i]
			}
			addr, err := netip.ParseAddr(a)
			if err != nil {
				continue
			}
			bits := 128
			for i := 2; i+1 < len(fields); i++ {
				if fields[i] == "prefixlen" {
					if b, err := strconv.Atoi(fields[i+1]); err == nil {
						bits = b
					}
				}
			}
			cur.Prefixes = append(cur.Prefixes, netip.PrefixFrom(addr, bits))
		}
	}
	return links
}

// maskBits turns ifconfig's hexadecimal netmask (0xffffff00) into a prefix
// length. A mask that is not contiguous is refused rather than rounded.
func maskBits(hex string) (int, bool) {
	v, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(hex), "0x"), 16, 32)
	if err != nil {
		return 0, false
	}
	m := uint32(v)
	bits := 0
	for m&0x80000000 != 0 {
		bits++
		m <<= 1
	}
	if m != 0 {
		return 0, false
	}
	return bits, true
}
