// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

// The private address ranges that stay off the tunnel.
//
// # Why these are written out and not fetched from geoip.dat
//
// The obvious way to write this rule is "ip": ["geoip:private"], and it is
// forbidden here. That string reaches ToCidrList at
// infra/conf/router.go:445-458, which calls loadIP("geoip.dat", "PRIVATE"),
// which is loadFile at router.go:180-192 opening the file through
// filesystem.OpenAsset. The engine embeds no such file: the only go:embed in
// xray-core v1.260327.0 is an HTML file at
// transport/internet/browser_dialer/dialer.go:18, and the search path is the
// "xray.location.asset" environment variable (common/platform/platform.go:13).
//
// So the convenient form of this rule would add a downloaded data file to a
// product whose installer verifies exactly one artefact by SHA-256. The list
// below costs twenty lines and removes that dependency entirely.
//
// # Why direct rather than blocked
//
// A client on the hotspot reaching 192.168.x.1 is reaching the box itself or
// its neighbours. Sending that into the tunnel would ask the user's proxy
// server to route to ITS own private network, which is at best a failure and
// at worst reaches a stranger's LAN. It is also how the box's own pinned host
// route to the user's server would be broken. docs/2026-08-29-design.md
// section 4.2 states the same requirement from the routing side.
//
// This is a routing decision, not a security boundary. Whether private traffic
// is permitted to LEAVE is the firewall's decision and lives in internal/netcfg.
//
// # What is deliberately NOT here
//
// The documentation ranges 192.0.2.0/24, 198.51.100.0/24 and 203.0.113.0/24
// are globally routed as far as any client is concerned, so they belong in the
// tunnel with everything else. Carrier NAT (100.64.0.0/10) IS here, because a
// box behind a CGNAT uplink genuinely reaches its provider's equipment there.

// PrivateRanges returns the CIDRs routed direct rather than into the tunnel.
//
// A fresh slice on every call, so a caller that appends cannot edit the
// default for everybody else.
func PrivateRanges() []string {
	return []string{
		// IPv4.
		"0.0.0.0/8",          // "this network", RFC 1122 section 3.2.1.3
		"10.0.0.0/8",         // private, RFC 1918
		"100.64.0.0/10",      // carrier-grade NAT, RFC 6598
		"127.0.0.0/8",        // loopback, RFC 1122
		"169.254.0.0/16",     // link-local, RFC 3927
		"172.16.0.0/12",      // private, RFC 1918
		"192.0.0.0/24",       // IETF protocol assignments, RFC 6890
		"192.168.0.0/16",     // private, RFC 1918
		"198.18.0.0/15",      // benchmarking, RFC 2544
		"224.0.0.0/4",        // multicast, RFC 5771
		"240.0.0.0/4",        // reserved, RFC 1112 section 4
		"255.255.255.255/32", // limited broadcast, RFC 8190

		// IPv6. Present because the engine's TUN inbound registers the IPv6
		// network protocol as well as IPv4 (proxy/tun/stack_gvisor.go:204-209
		// builds the stack with ipv4.NewProtocol AND ipv6.NewProtocol), so a
		// v6 packet can arrive on the tunnel whatever the uplink does.
		//
		// Stated precisely, because this is a leak-relevant path and a
		// confident wrong sentence here would be worse than none: these
		// entries route v6 private space DIRECT. They are not a v6 leak
		// control, they do not disable IPv6, and they say nothing about what
		// the firewall does. docs/2026-08-29-design.md section 7 records that
		// IPv6 needs its own firewall rules, and that work is internal/netcfg's.
		"::1/128",   // loopback, RFC 4291
		"fc00::/7",  // unique local, RFC 4193
		"fe80::/10", // link-local, RFC 4291
		"ff00::/8",  // multicast, RFC 4291
	}
}
