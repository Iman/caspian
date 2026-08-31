// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Package engine owns the lifetime of the xray-core proxy engine inside this
// process: it loads a JSON config, starts the engine, stops it, reports what
// state it is in, and validates a config without starting anything.
//
// It takes JSON bytes. It does not know about share links, and it does not
// import internal/link. Turning a pasted link into JSON belongs to that
// package; this one is the boundary where JSON becomes a running engine.
//
// # Why this package exists at all
//
// Three engine behaviours make a thin wrapper the wrong answer, and each is
// verified against the engine source in this file's siblings:
//
//  1. A core.Instance can be started once and once only. The engine says so
//     itself at core/xray.go:381-382: "A Xray instance can be started only
//     once. Upon closing, the instance is not guaranteed to start again." So
//     restart means construct a new instance, and something has to own that.
//
//  2. The engine's own error strings embed the credentials the user pasted.
//     See redact.go for the enumerated paths. Every error and every log line
//     leaving this package goes through Redact first.
//
//  3. The engine logs to stdout by default (infra/conf/log.go DefaultLogConfig
//     sets ErrorLogType: LogType_Console, and common/log/logger.go:182-184
//     registers a stdout handler at package init). On an appliance whose only
//     interface is a web panel, stdout is a hole, not an output. See logring.go.
//
// # No Google
//
// This package generates no config. It loads what it is handed and never
// synthesises a DNS server, a resolver address or an outbound. There is
// therefore no resolver default here to get wrong. TestNoGoogleAnywhere in
// engine_test.go enforces that no Google address or hostname appears in this
// package's source, so the property survives later edits.
//
// # One engine per process
//
// The engine's log plumbing is process-global on both routes this package
// uses: common/log.RegisterHandler (common/log/log.go:37-42) replaces a single
// package-level handler, and app/log.RegisterHandlerCreator
// (app/log/log_creator.go:21-30) writes into a single package-level map. Two
// Engine values in one process would therefore share one log destination:
// whichever started last wins the ring buffer. That is acceptable because the
// appliance runs exactly one engine, and it is stated here rather than
// discovered later.
package engine

// What was learned about the TUN inbound, recorded here because step 6 of the
// build plan in docs/2026-08-29-design.md needs it and this package is where
// the reader will come looking. All of it was read from
// github.com/xtls/xray-core v1.260327.0 in the module cache on 2026-08-30.
//
// Routing is deliberately NOT implemented in this package. It belongs to
// internal/netcfg. What follows is the evidence for why that split is forced
// rather than chosen.
//
// Registration
//
//	The inbound is registered for JSON under the name "tun" at
//	infra/conf/xray.go:37, in the same ConfigCreatorCache as "socks", "vless"
//	and the rest:
//	    "tun": func() interface{} { return new(TunConfig) },
//
//	It is live in an ordinary build even though main/distro/all does not
//	import proxy/tun directly: infra/conf/tun.go:4 imports it, and infra/conf
//	is on the path from main.
//
// The whole JSON surface is three fields
//
//	infra/conf/tun.go:8-12 declares exactly:
//	    Name      string `json:"name"`
//	    MTU       uint32 `json:"MTU"`
//	    UserLevel uint32 `json:"userLevel"`
//
//	Note the capitalised "MTU" JSON key. Defaults are applied at
//	infra/conf/tun.go:21-27: name "xray0", MTU 1500. There is no address
//	field, no routes field, no rules field, and no gateway field. Build()
//	copies the three values into a tun.Config and returns; it validates
//	nothing else.
//
// What the Linux implementation actually does
//
//	proxy/tun/tun_linux.go, three operations and no more:
//	  - open() at :50-77 opens /dev/net/tun O_RDWR, sets the interface flags
//	    to unix.IFF_TUN|unix.IFF_NO_PI via TUNSETIFF, and sets the fd
//	    non-blocking.
//	  - setup() at :80-93 looks the link up by name and calls
//	    netlink.LinkSetMTU. That is the only netlink write it performs.
//	  - Start() at :96-103 calls netlink.LinkSetUp.
//
//	Verified absent from the whole of proxy/tun: netlink.AddrAdd,
//	netlink.RouteAdd, netlink.RuleAdd, and any use of os/exec. The package
//	does not assign the interface an address, so the device comes up with no
//	IP at all.
//
//	The package README is explicit and agrees: "Current implementation does
//	not contain options to configure network level addresses, routing or
//	rules. Enabling the feature will result only tun interface up, and that's
//	it."
//
// No ICMP, so ping cannot be the health check
//
//	proxy/tun/stack_gvisor.go:204-209 builds the gVisor stack with
//	    NetworkProtocols:   ipv4.NewProtocol, ipv6.NewProtocol
//	    TransportProtocols: tcp.NewProtocol, udp.NewProtocol
//	There is no ICMP transport protocol registered, and the README lists "No
//	ICMP support" under LIMITATION. A reachability check for this appliance
//	has to be a TCP connect or a UDP exchange.
//
//	A second trap from the same README, worth carrying into whatever writes
//	the health check: because the inbound is a proxy and not a real layer 3
//	VPN, a TCP connect to a host that does not exist still completes the
//	handshake. Connection success proves the packet was accepted for
//	proxying, not that anything is on the other end. That is the same shape
//	as the project rule that a connect is not a result.
//
// Consequences that land on internal/netcfg, not here
//
//	Because the engine only brings the link up, every one of these is ours:
//	an address on xray0; a pinned host route to the user's server via the
//	real gateway, or the engine tries to reach its own uplink through the
//	tunnel it is providing; a default route or a policy rule plus a second
//	table; rp_filter handling, since two route tables make Linux drop return
//	packets as martians and the tunnel then connects while carrying nothing;
//	and INPUT and OUTPUT firewall permits for xray0, because the box's own
//	traffic is not FORWARD.
