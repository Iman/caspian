// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// pinnedByRouteSteps reads back which server addresses a set of route steps
// actually installs a host route to, by looking for each address in the
// arguments of each step's forward command. It reads the commands rather than
// calling canPin, so it is evidence about what reaches the machine and not a
// restatement of the rule PinnedServers applies.
func pinnedByRouteSteps(p *Plan, steps []Step) []netip.Addr {
	var out []netip.Addr
	for _, s := range p.ServerAddr {
		bits := 32
		if s.Is6() {
			bits = 128
		}
		forms := []string{s.String(), netip.PrefixFrom(s, bits).String()}
		for _, st := range steps {
			if slices.ContainsFunc(st.Do.Args, func(a string) bool { return slices.Contains(forms, a) }) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// TestPinnedServersIsExactlyWhatTheRouteStepsPin guards the one property
// internal/privsvc relies on when it hands PinnedServers to the engine: that
// the engine is told to dial exactly the addresses the machine has a pinned
// host route to. If the two drift, the engine dials an address whose only
// route is the tunnel, which is the loop the pinned route exists to prevent.
//
// Every platform is checked, each with the three shapes the family rules
// distinguish: IPv4 only, both families with an IPv6 default route, and both
// families without one.
func TestPinnedServersIsExactlyWhatTheRouteStepsPin(t *testing.T) {
	v4 := netip.MustParseAddr("203.0.113.10")
	v4b := netip.MustParseAddr("198.51.100.20")
	v6 := netip.MustParseAddr("2001:db8::10")
	gw := netip.MustParseAddr("192.168.1.1")
	v6gw := netip.MustParseAddr("fe80::1")

	shapes := []struct {
		name  string
		addrs []netip.Addr
		v6gw  netip.Addr
	}{
		{"ipv4-only", []netip.Addr{v4, v4b}, netip.Addr{}},
		{"dual-stack-with-v6-route", []netip.Addr{v4, v6, v4b}, v6gw},
		{"dual-stack-without-v6-route", []netip.Addr{v6, v4}, netip.Addr{}},
	}
	platforms := []struct {
		platform Platform
		steps    func(*Plan) []Step
	}{
		{PlatformLinux, (*Plan).ServerRouteSteps},
		{PlatformDarwin, (*Plan).darwinServerRouteSteps},
		{PlatformWindows, (*Plan).windowsServerRouteSteps},
	}
	for _, pl := range platforms {
		for _, sh := range shapes {
			t.Run(string(pl.platform)+"/"+sh.name, func(t *testing.T) {
				p := &Plan{
					Platform:      pl.platform,
					Uplink:        "eth0",
					UplinkGateway: gw,
					UplinkV6Gw:    sh.v6gw,
					ServerAddr:    sh.addrs,
				}
				got := p.PinnedServers()
				want := pinnedByRouteSteps(p, pl.steps(p))
				if !slices.Equal(got, want) {
					t.Fatalf("PinnedServers() = %v, but the route steps pin %v", got, want)
				}
				if len(want) == 0 && sh.name == "ipv4-only" {
					t.Fatal("no address was pinned for an IPv4-only server; the comparison above proves nothing")
				}
			})
		}
	}
}

// TestPinnedServersOnWindowsExcludesIPv6 pins the documented edge: Windows
// pins IPv4 only, so a server with only IPv6 addresses has an empty set even
// though the plan was accepted.
func TestPinnedServersOnWindowsExcludesIPv6(t *testing.T) {
	p := &Plan{
		Platform:      PlatformWindows,
		Uplink:        "Ethernet",
		UplinkGateway: netip.MustParseAddr("192.168.1.1"),
		UplinkV6Gw:    netip.MustParseAddr("fe80::1"),
		ServerAddr:    []netip.Addr{netip.MustParseAddr("2001:db8::10")},
	}
	if got := p.PinnedServers(); len(got) != 0 {
		t.Fatalf("PinnedServers() = %v on Windows for an IPv6-only server; windowsServerRouteSteps pins none", got)
	}
	for _, st := range p.windowsServerRouteSteps() {
		if strings.Contains(strings.Join(st.Do.Args, " "), "2001:db8::10") {
			t.Fatal("windowsServerRouteSteps now pins IPv6; PinnedServers must follow it")
		}
	}
}
