// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
)

// This file is the whole of the text-to-facts conversion, and it is pure: it
// takes the bytes a command printed and returns values. Everything here is
// tested against captured output in testdata, so a change to any of these
// output formats fails a test on the development machine rather than producing
// a silently empty result on the appliance.

// ParseDefaultRoutes parses "ip route show default" or "ip -6 route show
// default". Both families are accepted in one call because the family is
// deduced from the addresses rather than from which command produced the line.
//
// Shapes handled:
//
//	default via 192.168.1.1 dev eth0 proto dhcp src 192.168.1.42 metric 100
//	default dev ppp0 scope link
//	default via fe80::1 dev eth0 proto ra metric 1024 pref medium
//
// A route with no "via" is recorded with OnLink set, because the pinned host
// route to the user's server then has to be written without a gateway.
func ParseDefaultRoutes(out string) ([]DefaultRoute, error) {
	var routes []DefaultRoute
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "default" {
			continue
		}
		r := DefaultRoute{Family: 4, OnLink: true}
		for i := 1; i < len(fields); i++ {
			switch fields[i] {
			case "via":
				if i+1 >= len(fields) {
					return nil, fmt.Errorf("netcfg: default route %q: via with no address", line)
				}
				gw, err := netip.ParseAddr(fields[i+1])
				if err != nil {
					return nil, fmt.Errorf("netcfg: default route %q: %w", line, err)
				}
				r.Gateway = gw
				r.OnLink = false
				i++
			case "dev":
				if i+1 >= len(fields) {
					return nil, fmt.Errorf("netcfg: default route %q: dev with no name", line)
				}
				r.Dev = fields[i+1]
				i++
			case "proto":
				if i+1 < len(fields) {
					r.Proto = fields[i+1]
					i++
				}
			case "src":
				if i+1 < len(fields) {
					if a, err := netip.ParseAddr(fields[i+1]); err == nil {
						r.Src = a
					}
					i++
				}
			case "metric":
				if i+1 < len(fields) {
					m, err := strconv.Atoi(fields[i+1])
					if err != nil {
						return nil, fmt.Errorf("netcfg: default route %q: metric %q: %w", line, fields[i+1], err)
					}
					r.Metric = m
					i++
				}
			case "linkdown":
				r.LinkDown = true
			}
		}
		if r.Dev == "" {
			return nil, fmt.Errorf("netcfg: default route %q names no device", line)
		}
		if !ValidInterfaceName(r.Dev) {
			return nil, fmt.Errorf("netcfg: default route %q names an implausible device %q", line, r.Dev)
		}
		if r.Gateway.IsValid() && r.Gateway.Is6() {
			r.Family = 6
		} else if !r.Gateway.IsValid() && r.Src.IsValid() && r.Src.Is6() {
			r.Family = 6
		}
		routes = append(routes, r)
	}
	return routes, nil
}

// ParseBriefAddr parses "ip -br addr":
//
//	lo               UNKNOWN        127.0.0.1/8 ::1/128
//	eth0             UP             192.168.1.42/24 fe80::.../64
//
// A name printed as "eth0@if12" is recorded as "eth0"; the part after the at
// sign is the peer index of a veth and is not part of the name.
func ParseBriefAddr(out string) ([]Link, error) {
	var links []Link
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("netcfg: ip -br addr line %q has too few fields", line)
		}
		name := fields[0]
		if i := strings.IndexByte(name, '@'); i > 0 {
			name = name[:i]
		}
		if !ValidInterfaceName(name) {
			return nil, fmt.Errorf("netcfg: ip -br addr line %q names an implausible device %q", line, name)
		}
		l := Link{Name: name, State: fields[1]}
		for _, f := range fields[2:] {
			p, err := netip.ParsePrefix(f)
			if err != nil {
				// "ip -br addr" also prints things like "metric 1024" for
				// some address flavours. Skip anything that is not a prefix
				// rather than failing the whole parse.
				continue
			}
			l.Prefixes = append(l.Prefixes, p)
		}
		links = append(links, l)
	}
	return links, nil
}

// ParseLinkDetail extracts the parent bus per interface from "ip -d link
// show". It is best effort by design: iproute2 prints "parentbus usb" only on
// versions and kernels that expose it, so an interface missing from the result
// means "not reported" and never "not USB". The planner treats it as a
// preference and never as a requirement.
func ParseLinkDetail(out string) map[string]string {
	buses := map[string]string{}
	current := ""
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimRight(raw, " \t")
		if line == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		// A new interface block starts at column 0 with "<index>: <name>:".
		if line == trimmed && len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			parts := strings.SplitN(trimmed, ":", 3)
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[1])
				if i := strings.IndexByte(name, '@'); i > 0 {
					name = name[:i]
				}
				if ValidInterfaceName(name) {
					current = name
				} else {
					current = ""
				}
			}
			continue
		}
		if current == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		for i, f := range fields {
			if f == "parentbus" && i+1 < len(fields) {
				buses[current] = fields[i+1]
			}
		}
	}
	return buses
}

// ParseIwDev parses "iw dev":
//
//	phy#0
//		Interface wlan0
//			ifindex 3
//			addr 02:00:5e:01:00:11
//			ssid HomeNet
//			type managed
//			channel 10 (2457 MHz), width: 20 MHz, center1: 2457 MHz
//
// The channel line is the one that matters for mode B: a radio whose interface
// combination says "#channels <= 1" pins any access point to whatever channel
// the station link is already on.
func ParseIwDev(out string) ([]WirelessIface, error) {
	var ifaces []WirelessIface
	// "iw dev" nests by tabs: phy#N at column 0, each interface stanza one
	// tab in, its fields two tabs in.
	const interfaceIndent = 1
	phy := ""
	idx := -1
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimRight(raw, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "phy#") {
			phy = "phy" + strings.TrimPrefix(trimmed, "phy#")
			continue
		}
		if strings.HasPrefix(trimmed, "Interface ") {
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "Interface "))
			if !ValidInterfaceName(name) {
				return nil, fmt.Errorf("netcfg: iw dev names an implausible interface %q", name)
			}
			ifaces = append(ifaces, WirelessIface{Name: name, Phy: phy})
			idx = len(ifaces) - 1
			continue
		}
		// A radio can also list a stanza with no netdev at all. The target
		// prints "Unnamed/non-netdev interface" for its P2P device, with the
		// same fields at the same depth as a real interface, including a
		// "type" line. Without this reset those fields are attributed to
		// whichever interface was parsed last, so a radio that lists the
		// unnamed stanza after wlan0 would report wlan0's type as
		// P2P-device and its channel as absent. It reads correctly on the
		// captured output only because the stanza happens to come first.
		if indentOf(line) == interfaceIndent {
			idx = -1
			continue
		}
		if idx < 0 {
			continue
		}
		fields := strings.Fields(trimmed)
		switch fields[0] {
		case "type":
			if len(fields) > 1 {
				ifaces[idx].Type = fields[1]
			}
		case "ssid":
			if len(fields) > 1 {
				ifaces[idx].SSID = strings.TrimSpace(strings.TrimPrefix(trimmed, "ssid "))
			}
		case "addr":
			if len(fields) > 1 {
				ifaces[idx].MAC = fields[1]
			}
		case "channel":
			// "channel 10 (2457 MHz), width: 20 MHz, center1: 2457 MHz"
			if len(fields) > 1 {
				if n, err := strconv.Atoi(fields[1]); err == nil {
					ifaces[idx].Channel = n
				}
			}
			if len(fields) > 2 {
				mhz := strings.TrimPrefix(fields[2], "(")
				if n, err := strconv.Atoi(mhz); err == nil {
					ifaces[idx].FreqMHz = n
				}
			}
		}
	}
	return ifaces, nil
}

// indentOf counts leading whitespace runes. "iw list" nests by tabs and then
// by spaces within a tab level, so counting runes rather than expanding tabs
// is enough to tell a section header from its contents, and it is what the
// section walker below relies on.
func indentOf(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
}

// ParseIwList parses "iw list". It reads the sections this package needs and
// ignores the rest:
//
//   - Supported interface modes, which is where AP support is stated
//   - valid interface combinations, which is where concurrency and the
//     #channels limit are stated
//   - Band N frequencies, which is where a usable channel comes from
//
// Section boundaries are found by indentation rather than by a list of known
// headings, so an unrecognised section between two known ones does not swallow
// the one after it.
func ParseIwList(out string) ([]Phy, error) {
	var phys []Phy
	cur := -1

	const (
		sectNone = iota
		sectModes
		sectCommands
		sectCombos
		sectBand
	)
	section := sectNone
	sectionIndent := 0
	bandIdx := -1
	inFrequencies := false

	lines := strings.Split(out, "\n")
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		ind := indentOf(line)
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Wiphy ") && ind == 0 {
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "Wiphy "))
			phys = append(phys, Phy{Name: name, Index: -1})
			cur = len(phys) - 1
			section = sectNone
			bandIdx = -1
			continue
		}
		if cur < 0 {
			continue
		}

		// A line at or above the section header's indentation ends the
		// section, whatever it says.
		if section != sectNone && ind <= sectionIndent {
			section = sectNone
			inFrequencies = false
		}

		if section == sectNone {
			switch {
			case strings.HasPrefix(trimmed, "wiphy index:"):
				if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, "wiphy index:"))); err == nil {
					phys[cur].Index = n
				}
				continue
			case trimmed == "Supported interface modes:":
				section, sectionIndent = sectModes, ind
				continue
			case trimmed == "Supported commands:":
				section, sectionIndent = sectCommands, ind
				continue
			case trimmed == "valid interface combinations:":
				section, sectionIndent = sectCombos, ind
				continue
			case strings.HasPrefix(trimmed, "Band ") && strings.HasSuffix(trimmed, ":"):
				numStr := strings.TrimSuffix(strings.TrimPrefix(trimmed, "Band "), ":")
				n, err := strconv.Atoi(strings.TrimSpace(numStr))
				if err != nil {
					continue
				}
				phys[cur].Bands = append(phys[cur].Bands, Band{Number: n})
				bandIdx = len(phys[cur].Bands) - 1
				section, sectionIndent = sectBand, ind
				inFrequencies = false
				continue
			default:
				continue
			}
		}

		switch section {
		case sectModes, sectCommands:
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
			if item == "" || item == trimmed {
				continue
			}
			if section == sectModes {
				phys[cur].Modes = append(phys[cur].Modes, item)
			} else {
				phys[cur].Commands = append(phys[cur].Commands, item)
			}

		case sectCombos:
			if strings.HasPrefix(trimmed, "*") {
				body := strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
				phys[cur].Combinations = append(phys[cur].Combinations, Combination{Raw: body})
				continue
			}
			// Continuation of the entry above: "total <= 4, #channels <= 1".
			if n := len(phys[cur].Combinations); n > 0 {
				phys[cur].Combinations[n-1].Raw += " " + trimmed
			}

		case sectBand:
			if trimmed == "Frequencies:" {
				inFrequencies = true
				continue
			}
			if strings.HasSuffix(trimmed, ":") {
				inFrequencies = false
				continue
			}
			if !inFrequencies || !strings.HasPrefix(trimmed, "*") || bandIdx < 0 {
				continue
			}
			f, ok := parseFrequency(strings.TrimSpace(strings.TrimPrefix(trimmed, "*")))
			if ok {
				phys[cur].Bands[bandIdx].Frequencies = append(phys[cur].Bands[bandIdx].Frequencies, f)
			}
		}
	}

	for i := range phys {
		for j := range phys[i].Combinations {
			c, err := ParseCombination(phys[i].Combinations[j].Raw)
			if err != nil {
				return nil, fmt.Errorf("netcfg: %s: %w", phys[i].Name, err)
			}
			phys[i].Combinations[j] = c
		}
	}
	return phys, nil
}

// parseFrequency reads one frequency bullet. Both spellings of the megahertz
// value are accepted, because iw changed it:
//
//	2457 MHz [10] (20.0 dBm)              // integer, older iw
//	2412.0 MHz [1] (20.0 dBm)             // one decimal place, iw 6.9
//	5260.0 MHz [52] (20.0 dBm) (no IR, radar detection)
//	5745.0 MHz [149] (disabled)
//
// The decimal form is what iw version 6.9 prints on the target: measured on
// 2026-08-30 from caspian-box, a Raspberry Pi 5 on Debian 13, and captured in
// testdata/iw-list-pi5-builtin.txt. Reading it with strconv.Atoi failed on
// every line, which left Band.Frequencies empty and so made
// [Phy.UsableChannels] return nothing at all. That is not a cosmetic loss: a
// radio whose combination does not pin the channel then has no channel to
// offer and the access point is planned without one.
func parseFrequency(s string) (Frequency, bool) {
	fields := strings.Fields(s)
	if len(fields) < 2 || fields[1] != "MHz" {
		return Frequency{}, false
	}
	mhz, ok := parseMHz(fields[0])
	if !ok {
		return Frequency{}, false
	}
	f := Frequency{MHz: mhz}
	if i := strings.IndexByte(s, '['); i >= 0 {
		if j := strings.IndexByte(s[i:], ']'); j > 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(s[i+1 : i+j])); err == nil {
				f.Channel = n
			}
		}
	}
	lower := strings.ToLower(s)
	f.NoIR = strings.Contains(lower, "no ir")
	f.Disabled = strings.Contains(lower, "disabled")
	f.Radar = strings.Contains(lower, "radar detection")
	for i, fl := range fields {
		if fl == "dBm)" && i > 0 {
			if v, err := strconv.ParseFloat(strings.TrimPrefix(fields[i-1], "("), 64); err == nil {
				f.MaxDBm = v
			}
		}
	}
	return f, true
}

// parseMHz reads the megahertz field of a frequency bullet, which iw prints
// either as an integer ("2457") or with one decimal place ("2412.0"). A
// fractional value is rounded to the nearest megahertz because Frequency.MHz
// is a whole number and the channel index, not the exact carrier, is what this
// package selects on.
func parseMHz(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int(math.Round(v)), true
}

// ParseCombination reads one interface-combination entry, already joined onto
// a single line:
//
//	#{ managed } <= 1, #{ AP } <= 1, #{ P2P-client } <= 1, #{ P2P-device } <= 1,
//	total <= 4, #channels <= 1, STA/AP BI must match
//
// Anything that is not a "#{...} <= n", a "total <= n" or a "#channels <= n"
// is kept in Notes rather than discarded, because the notes are what an
// operator needs when a plan is refused.
func ParseCombination(raw string) (Combination, error) {
	c := Combination{Raw: strings.Join(strings.Fields(raw), " ")}
	s := c.Raw

	// Split on commas that are not inside a "#{ ... }" group.
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch {
		case strings.HasPrefix(p, "#{"):
			end := strings.IndexByte(p, '}')
			if end < 0 {
				return Combination{}, fmt.Errorf("interface combination %q: unterminated group in %q", raw, p)
			}
			inner := p[2:end]
			var types []string
			for _, t := range strings.Split(inner, ",") {
				if t = strings.TrimSpace(t); t != "" {
					types = append(types, t)
				}
			}
			max, err := parseLimitValue(p[end+1:])
			if err != nil {
				return Combination{}, fmt.Errorf("interface combination %q: %w", raw, err)
			}
			c.Limits = append(c.Limits, ComboLimit{Max: max, Types: types})
		case strings.HasPrefix(p, "total"):
			n, err := parseLimitValue(strings.TrimPrefix(p, "total"))
			if err != nil {
				return Combination{}, fmt.Errorf("interface combination %q: %w", raw, err)
			}
			c.Total = n
		case strings.HasPrefix(p, "#channels"):
			n, err := parseLimitValue(strings.TrimPrefix(p, "#channels"))
			if err != nil {
				return Combination{}, fmt.Errorf("interface combination %q: %w", raw, err)
			}
			c.Channels = n
		default:
			c.Notes = append(c.Notes, p)
		}
	}
	return c, nil
}

// parseLimitValue reads the " <= 4" tail of a limit clause.
func parseLimitValue(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<=")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("limit has no value")
	}
	fields := strings.Fields(s)
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, fmt.Errorf("limit value %q: %w", fields[0], err)
	}
	return n, nil
}

// ParseNmcliDeviceStatus reads "nmcli -t -f DEVICE,STATE device status".
//
// Terse mode separates fields with a colon, one device per line. CAPTURED from
// the target on 2026-08-30, nmcli 1.52.1:
//
//	eth0:connected
//	wlan0:connected
//	lo:connected (externally)
//	xray0:connected (externally)
//	p2p-dev-wlan0:disconnected
//
// Two things in those bytes that an authored guess did not have, and both
// matter:
//
//   - A state can be "connected (externally)", with a space and a
//     parenthetical. Anything comparing the whole field against "connected"
//     gets a different answer for lo and xray0 than for eth0. The state is
//     therefore reduced to its first word before it is classified, so the
//     parenthetical is handled deliberately rather than by luck.
//   - The radio presents a SECOND device, "p2p-dev-wlan0", which is not wlan0.
//     Devices are keyed by exact name and looked up by exact name; nothing in
//     this package matches an interface by prefix or substring. A "contains"
//     here would let the P2P device decide what is true of the radio's real
//     interface.
//
// The classification is deliberately coarse: a device NetworkManager reports
// as anything other than "unmanaged" is one it has a claim on, and one this
// package must ask it to release before using. "connected (externally)" means
// NetworkManager did not configure the device but does track it, and it is
// treated as managed because releasing it is still the safe action.
// Anything not listed is not known to NetworkManager.
func ParseNmcliDeviceStatus(out string) map[string]InterfaceManager {
	owners := map[string]InterfaceManager{}
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		name, state, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			continue
		}
		if !ValidInterfaceName(name) {
			continue
		}
		// "connected (externally)" reduces to "connected".
		base := strings.TrimSpace(state)
		if i := strings.IndexByte(base, ' '); i >= 0 {
			base = base[:i]
		}
		if base == "" {
			// A device with no state at all says nothing about ownership, and
			// silence must not be read as "nobody owns it".
			continue
		}
		if strings.EqualFold(base, "unmanaged") {
			owners[name] = ManagedByNothing
			continue
		}
		owners[name] = ManagedByNetworkManager
	}
	return owners
}

// ParseSysctl reads the output of "sysctl <names...>", which prints one
// "name = value" per line. Values are returned verbatim so the applier can
// write back exactly what it found.
func ParseSysctl(out string) map[string]string {
	vals := map[string]string{}
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vals[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return vals
}
