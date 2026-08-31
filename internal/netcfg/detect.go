// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"fmt"
	"net/netip"
	"time"
)

// Detect asks the machine what it looks like and returns the facts a plan is
// built from. Every command it runs is read-only.
//
// It does not fail when a wireless tool is missing or a radio enumeration
// comes back empty: a machine with no radio is a real machine, and the refusal
// belongs to PlanNetwork, which can say "no adapter on this machine can create
// a hotspot" instead of "iw exited 1". The two commands that must succeed are
// the ones that describe interfaces and routes, because facts without those
// are not facts.
func Detect(ctx context.Context, r Runner, knobs []string) (Facts, error) {
	f := Facts{CapturedAt: time.Now().UTC(), Sysctl: map[string]string{}}

	res, err := r.Run(ctx, Command{
		Path: BinIP, Args: []string{"-br", "addr"},
		Why: "interfaces and the addresses on them, which a hotspot subnet must not collide with",
	})
	if err != nil {
		return f, fmt.Errorf("netcfg: list interfaces: %w", err)
	}
	if f.Links, err = ParseBriefAddr(res.Stdout); err != nil {
		return f, err
	}

	res, err = r.Run(ctx, Command{
		Path: BinIP, Args: []string{"route", "show", "default"},
		Why: "the uplink is whichever interface carries the default route",
	})
	if err != nil {
		return f, fmt.Errorf("netcfg: read default route: %w", err)
	}
	v4, err := ParseDefaultRoutes(res.Stdout)
	if err != nil {
		return f, err
	}
	f.Routes = append(f.Routes, v4...)

	// The IPv6 default is optional: plenty of networks have none, and a server
	// reached over IPv4 does not need one. It is read because a server given
	// as an IPv6 address needs a pinned host route through an IPv6 gateway.
	if res, err := r.Run(ctx, Command{
		Path: BinIP, Args: []string{"-6", "route", "show", "default"},
		Why: "an IPv6 server address needs an IPv6 gateway to pin a host route through",
	}); err == nil {
		if v6, err := ParseDefaultRoutes(res.Stdout); err == nil {
			for i := range v6 {
				v6[i].Family = 6
			}
			f.Routes = append(f.Routes, v6...)
		}
	}

	// Parent bus is best effort; see ParseLinkDetail.
	if res, err := r.Run(ctx, Command{
		Path: BinIP, Args: []string{"-d", "link", "show"},
		Why: "tells a USB adapter from the radio on the board, which decides which one runs the access point in mode B",
	}); err == nil {
		buses := ParseLinkDetail(res.Stdout)
		for i := range f.Links {
			if bus, ok := buses[f.Links[i].Name]; ok {
				f.Links[i].Bus = bus
			}
		}
	}

	// A machine with no wireless tooling is a machine that cannot run a
	// hotspot, which PlanNetwork reports in plain words. It is not a detection
	// failure.
	if res, err := r.Run(ctx, Command{
		Path: BinIw, Args: []string{"dev"},
		Why: "which interfaces are wireless, and which radio each one belongs to",
	}); err == nil {
		if ifaces, err := ParseIwDev(res.Stdout); err == nil {
			f.Wireless = ifaces
		}
	}
	// Whether each station interface is actually JOINED to a network, asked of
	// the machine rather than inferred from what "iw dev" happens to print.
	//
	// This is the fact the channel pin and the takeover both turn on, and
	// every cheaper substitute for it has now been measured wrong. A channel
	// survives the connection that set it: on 2026-08-30 wlan0 sat down and
	// unassociated reporting channel 36 from the last hotspot it hosted, and
	// the planner pinned a 5GHz channel onto a radio the user had asked to run
	// on 2.4GHz.
	//
	// Three deliberate limits:
	//
	//   - Access point interfaces are NOT probed. "Is this a station" is not a
	//     question about an access point, and what the command prints for one
	//     with hostapd running has never been measured here. Nothing is asked
	//     that nothing needs.
	//   - A probe that fails or prints something unrecognised leaves LinkKnown
	//     false. Detection gathers facts and must not refuse to produce any;
	//     StationLink then falls back to the SSID, which is where it stood
	//     before this existed.
	//   - The SSID is taken from the link only when the link HAS one. An
	//     answer of "Not connected." carries no name and must not erase one.
	for i := range f.Wireless {
		if f.Wireless[i].IsAccessPoint() {
			continue
		}
		st, err := readLinkState(ctx, r, f.Wireless[i].Name)
		if err != nil {
			continue
		}
		f.Wireless[i].LinkKnown = true
		f.Wireless[i].Associated = st.Connected
		if st.SSID != "" {
			f.Wireless[i].SSID = st.SSID
		}
	}

	if res, err := r.Run(ctx, Command{
		Path: BinIw, Args: []string{"list"},
		Why: "AP support and the interface combination limits, read from the radio rather than assumed",
	}); err == nil {
		if phys, err := ParseIwList(res.Stdout); err == nil {
			f.Phys = phys
		}
	}

	// What owns each wireless interface. Asked, never assumed: an interface
	// this package plans to turn into an access point may be held by a
	// manager that will fight it, keep it joined to another network, and
	// leave a DHCP server answering on a LAN that is not ours.
	//
	// nmcli failing, or being absent, is not an error. It means
	// NetworkManager is not managing anything here, which is a real state on
	// a real machine. What is NOT inferred from that is "nothing owns it":
	// an interface that is still associated is owned by something, and the
	// planner treats an undetected manager as unknown rather than absent.
	if res, err := r.Run(ctx, Command{
		Path: BinNmcli, Args: []string{"-t", "-f", "DEVICE,STATE", "device", "status"},
		Why: "an interface has to be released by whatever manages it before it can become an access point",
	}); err == nil {
		owners := ParseNmcliDeviceStatus(res.Stdout)
		// An empty answer is not an answer. A command that succeeds and
		// reports nothing tells us nothing about who owns what, and reading
		// it as "nothing owns it" is the same unsafe inference as reading a
		// runner's empty success as evidence. Only a listing that named at
		// least one device is allowed to settle the question, and then an
		// interface absent from it really is one NetworkManager does not hold.
		if len(owners) > 0 {
			// The listing answered and named devices, so NetworkManager is
			// running here. This is what the created-interface unmanage step
			// is gated on.
			f.NetworkManagerPresent = true
			for i := range f.Wireless {
				if m, ok := owners[f.Wireless[i].Name]; ok {
					f.Wireless[i].Manager = m
				} else {
					f.Wireless[i].Manager = ManagedByNothing
				}
			}
		}
	}

	// The knobs are read so that every change to one has an exact inverse. A
	// knob that cannot be read is left out of the map, and the step that
	// changes it says in its Why that it has no recorded inverse.
	if len(knobs) > 0 {
		// No "-n". That flag prints values without names, so the output is a
		// column of bare numbers, ParseSysctl finds no "=" on any line and
		// returns an empty map, and every sysctl step then takes its
		// no-inverse branch. The visible symptom is the one that matters:
		// uninstall leaves ip_forward and rp_filter changed on a box it
		// promised to return to how it was found. "-e" is kept so a knob that
		// does not exist on this kernel is skipped instead of failing the
		// whole read.
		args := append([]string{"-e", "--"}, knobs...)
		if res, err := r.Run(ctx, Command{
			Path: BinSysctl, Args: args,
			Why: "read the values to restore on teardown",
		}); err == nil {
			f.Sysctl = ParseSysctl(res.Stdout)
		}
	}
	return f, nil
}

// BaseSysctlKnobs are the knobs that can be read before a plan exists, which
// is every knob whose name does not contain an interface name.
func BaseSysctlKnobs() []string {
	return []string{
		"net.ipv4.ip_forward",
		"net.ipv4.conf.all.rp_filter",
		"net.ipv4.conf.default.rp_filter",
		"net.ipv6.conf.all.forwarding",
	}
}

// DetectAndPlan is the whole read-then-decide path in one call.
//
// It reads the kernel knobs once, plans, and reads a second time only if the
// plan turns out to need a knob the first read did not fetch. Today it never
// does: every knob a plan changes is global, so the second read is skipped and
// the facts from the first are returned unchanged.
//
// The second read is kept rather than deleted because the shape is the honest
// one. Detection cannot know which knobs matter until a plan has chosen the
// interfaces, and the day a plan needs a knob that names one, this is where
// that is noticed instead of the knob being changed with no measured value.
func DetectAndPlan(ctx context.Context, r Runner, servers []netip.Addr, o Options) (Facts, *Plan, error) {
	base := BaseSysctlKnobs()
	f, err := Detect(ctx, r, base)
	if err != nil {
		return f, nil, err
	}
	p, err := PlanNetwork(f, servers, o)
	if err != nil {
		return f, nil, err
	}
	missing := false
	for _, k := range p.SysctlKnobs() {
		if _, ok := f.Sysctl[k]; !ok {
			missing = true
			break
		}
	}
	if !missing {
		return f, p, nil
	}
	full, err := Detect(ctx, r, p.SysctlKnobs())
	if err != nil {
		return f, p, err
	}
	return full, p, nil
}
