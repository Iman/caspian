// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"net/netip"

	"caspianbyoc.org/caspian/internal/link"
)

// The tags every generated document uses.
//
// They are constants rather than anything derived from what the user pasted,
// for the same reason link.OutboundTag is: a routing rule has to be able to
// name an outbound, and no user-supplied text may become a config identifier.
const (
	// TagProxy is the outbound internal/link produces. Taken from that
	// package rather than restated, so the two cannot drift apart.
	TagProxy = link.OutboundTag

	// TagDirect is the freedom outbound. Reachable ONLY from the private
	// address rule. It is never first in the outbound list and never the
	// target of a catch-all.
	TagDirect = "direct"

	// TagBlock is the blackhole outbound.
	TagBlock = "block"

	// TagDNSOut is the dns outbound: the thing that answers a DNS query from
	// the built-in resolver instead of forwarding it. Present when either
	// LocalDNS.Enabled or DNS.Intercept is set, since both need something to
	// answer with.
	TagDNSOut = "dns-out"

	// TagTUNIn is the TUN inbound: client traffic.
	TagTUNIn = "tun-in"

	// TagSOCKSIn is the loopback SOCKS inbound: diagnostics and the exit-IP
	// proof.
	TagSOCKSIn = "socks-in"

	// TagResolverIn is the tag the built-in DNS app stamps on its OWN upstream
	// queries, which is what lets a routing rule pin those queries to the
	// tunnel.
	//
	// This has to be set explicitly. app/dns/dns.go:118-121 uses config.Tag as
	// the default tag for every DNS client and calls generateRandomTag() when
	// it is empty, and app/dns/nameserver.go:274 attaches that tag to each
	// upstream query with session.ContextWithInbound. An unset dns.tag
	// therefore produces a random tag that no rule can match, and the resolver
	// traffic falls through to whatever the catch-all does.
	TagResolverIn = "resolver-in"

	// TagLocalDNSIn is the loopback DNS listener: a real socket that a local
	// caller sends a query to.
	//
	// It is NOT TagResolverIn and the two must never be conflated. This one is
	// an inbound with a listen address and a port that something on the box
	// connects to. That one is a label the DNS app puts on the queries it
	// makes on its own behalf, and no socket corresponds to it. The names were
	// close enough to confuse before this listener existed, which is why the
	// resolver tag was renamed from "dns-in" when the listener was added.
	TagLocalDNSIn = "local-dns-in"
)

// Rule tags. router.go:127 and :559 carry ruleTag into the built rule, and
// app/dispatcher/default.go:474-477 prints it on the routing decision, so
// naming the rules is what makes the engine log say WHICH rule sent a
// connection where. That is the difference between an advanced-mode log a
// person can read and a wall of "taking detour [proxy]".
const (
	ruleTagPrivate   = "private-direct"
	ruleTagLocalDNS  = "local-dns-to-tunnel"
	ruleTagResolvers = "resolver-through-tunnel"
	ruleTagDNS       = "client-dns-intercept"
	ruleTagCatchAll  = "everything-else"
	ruleTagReject    = "reject-all"
)

// QueryStrategy is which address families the built-in resolver will answer
// with.
type QueryStrategy string

// The three strategies this appliance emits. The strings are the ones
// resolveQueryStrategy accepts at infra/conf/dns.go:517-522.
//
// UseSys is deliberately not offered: it makes the engine consult the host's
// own address configuration, which on a Raspberry Pi under systemd-resolved is
// the thing this product is replacing.
const (
	// QueryUseIP answers with A and AAAA. This is the engine's own default
	// (infra/conf/dns.go:525-526 returns USE_IP for anything unrecognised)
	// and it is this package's default too.
	QueryUseIP QueryStrategy = "UseIP"

	// QueryUseIPv4 answers with A only. app/dns/dns.go:72-77 sets
	// IPv6Enable false for it.
	//
	// What this does NOT do, stated because the inverse is easy to believe:
	// it does not disable IPv6, it does not stop a client from opening a v6
	// connection to a literal address, and it is not a leak control. It
	// changes what the ENGINE'S OWN resolver answers with, which matters only
	// when something actually asks that resolver: with both LocalDNS.Enabled
	// and DNS.Intercept unset, nothing in the generated document does. See
	// DNS.Intercept for the enumeration of all four paths in.
	QueryUseIPv4 QueryStrategy = "UseIPv4"

	// QueryUseIPv6 answers with AAAA only. app/dns/dns.go:78-83.
	QueryUseIPv6 QueryStrategy = "UseIPv6"
)

// LogLevel is the engine's error-log severity.
type LogLevel string

// The four levels infra/conf/log.go:49-62 maps to a severity. "none" is not
// offered: it sets ErrorLogType to None and leaves ErrorLogLevel at the zero
// value, and internal/engine/logring.go:223 forces the type back to Console
// anyway, so the combination would mean something different from what it says.
const (
	LogDebug   LogLevel = "debug"
	LogInfo    LogLevel = "info"
	LogWarning LogLevel = "warning"
	LogError   LogLevel = "error"
)

// TUN configures the tunnel inbound: how client traffic gets in.
//
// The whole JSON surface of this inbound is three fields. Read from
// infra/conf/tun.go:8-12 on 2026-08-30:
//
//	Name      string `json:"name"`
//	MTU       uint32 `json:"MTU"`
//	UserLevel uint32 `json:"userLevel"`
//
// Note the capitalised MTU key. There is no address field, no routes field and
// no gateway field, and the Linux implementation only opens the device, sets
// the MTU and brings the link up. Addressing, routing, rp_filter and the
// firewall are internal/netcfg's; internal/engine/doc.go:50-128 records the
// evidence for that split.
type TUN struct {
	// Disabled leaves the TUN inbound out of the document.
	//
	// Not an oddity: docs/2026-08-29-design.md section 8 makes step 4 "engine
	// linked, SOCKS inbound, config from step 3" and does not add TUN until
	// step 6. A SOCKS-only document is what proves an outbound works before
	// any of the addressing exists, and it is the only form that runs on a
	// developer machine with no /dev/net/tun and no root.
	Disabled bool

	// Name is the interface name. Empty means the engine default, "xray0",
	// which this package writes out explicitly rather than leaving implicit.
	Name string

	// MTU is the interface MTU. Zero means the engine default, 1500.
	MTU uint32

	// UserLevel selects a policy entry. Zero is the engine's default level
	// and this appliance emits no policy section, so it has no effect today.
	UserLevel uint32
}

// SOCKS configures the loopback diagnostics inbound.
//
// It exists for two jobs named in the design: the exit-IP proof that every
// result in this project depends on (section 6, "Nothing is called working
// without an exit IP captured from real traffic"), and advanced-mode
// diagnostics. It is not a service for clients on the hotspot; those arrive on
// the tunnel.
type SOCKS struct {
	// Listen is the address to bind. Empty means "127.0.0.1".
	//
	// It must be a loopback IP LITERAL. A wildcard is rejected, and so is a
	// name: infra/conf/xray.go:153 accepts the string "localhost" as an
	// address, which would bind wherever the host's name resolution says, and
	// this inbound has no authentication.
	Listen string

	// Port is the port to bind. Zero means DefaultSocksPort.
	Port uint16

	// UDP enables UDP associate.
	//
	// Off by default. The exit-IP proof is an HTTP request, which is TCP, and
	// an unauthenticated UDP relay is surface this appliance does not need to
	// carry to do that job. Turn it on to prove a UDP path.
	UDP bool
}

// DNS configures the resolver policy.
type DNS struct {
	// Servers is the resolver chain, in order. Empty means
	// DefaultResolvers(). Entries must be bare IP literals and must not be
	// Google addresses; see resolvers.go.
	Servers []string

	// Strategy is which address families the resolver answers with. Empty
	// means QueryUseIP.
	Strategy QueryStrategy

	// Intercept makes the engine ANSWER client DNS itself.
	//
	// # What it changes
	//
	// It adds a "dns" outbound (infra/conf/dns_proxy.go:10-38) and a routing
	// rule sending every destination port 53, TCP and UDP, to it. A client
	// that ignores the address DHCP gave it and talks to a hardcoded resolver
	// is then answered by the box instead of reaching that resolver.
	//
	// # Why it is OFF by default, and what that costs
	//
	// docs/2026-08-29-design.md section 6 wants client DNS answered on the box
	// and hardcoded resolvers redirected, and it is explicit that the
	// mechanisms for that are dnsmasq and the firewall, which are
	// internal/hotspot's and internal/netcfg's. Turning this on puts a second
	// mechanism on the same traffic, and two mechanisms disagreeing about DNS
	// is worse than one. It is offered rather than assumed, and which layer
	// owns the job is the maintainer's decision, not this package's.
	//
	// # The consequence of leaving it off, stated plainly
	//
	// With BOTH this and LocalDNS.Enabled unset, nothing in the generated
	// document consults the built-in DNS app. That is a claim about every path
	// in, so here they are:
	//
	//   - The router asks the DNS client only when its domainStrategy is
	//     IpOnDemand or IpIfNonMatch (app/router/router.go:253 and :263).
	//     This package emits AsIs, which is neither.
	//   - The freedom outbound resolves only when its domainStrategy is
	//     something other than AsIs (infra/conf/freedom.go:49-51). This
	//     package emits AsIs explicitly on the direct outbound.
	//   - The "dns" outbound is the third path. TWO fields add it: this one,
	//     and LocalDNS.Enabled. Either is sufficient.
	//   - The fakedns sniffer is the fourth, and this package emits no
	//     sniffing section at all.
	//
	// The condition is BOTH, not this field alone. An earlier version of this
	// comment said "with Intercept unset" because the loopback listener did
	// not exist yet; leaving that wording after the listener was added would
	// have made it the inverse of shipped behaviour for every configuration
	// that turns the listener on.
	//
	// So with both unset the "dns" block is a stated policy that nothing
	// executes: it is what the panel's advanced mode shows and what the
	// operator's choice is recorded in, and it starts doing work the moment
	// either field is set. TestDNSAppIsUnreachableWithoutIntercept asserts the
	// four paths above stay closed when neither is set.
	Intercept bool
}

// Options is everything this package needs to compose a document.
//
// Every zero value is a usable default; see the package doc on why the bools
// are written the way round they are.
type Options struct {
	// Link is the parsed share link whose outbound the document carries.
	// Required by Build, ignored by BuildFailClosed.
	Link *link.Link

	// LogLevel is the engine's error-log severity. Empty means LogWarning.
	LogLevel LogLevel

	TUN      TUN
	SOCKS    SOCKS
	DNS      DNS
	LocalDNS LocalDNS
}

// DefaultSocksPort is the loopback diagnostics port.
//
// 10808 is the conventional SOCKS port for this engine family and is outside
// the range Linux hands out as an ephemeral port (net.ipv4.ip_local_port_range
// starts at 32768 on a stock kernel), so a listener started at boot does not
// race a client socket for it.
const DefaultSocksPort = 10808

// DefaultTunName and DefaultTunMTU are the engine's own defaults, applied at
// infra/conf/tun.go:21-27. They are written into the document explicitly so
// the document says what the box will do rather than relying on the reader
// knowing what the engine fills in.
const (
	DefaultTunName = "xray0"
	DefaultTunMTU  = 1500
)

// The MTU range this appliance will emit. The engine validates nothing
// (infra/conf/tun.go:14-30 copies the value straight through), so the bounds
// are ours.
//
// The floor is the IPv6 minimum link MTU from RFC 8200 section 5. Below it a
// v6 packet cannot be carried at all, and the TUN netstack registers the IPv6
// protocol (proxy/tun/stack_gvisor.go:204-209), so that is a real failure and
// not a theoretical one. The ceiling is a conventional jumbo frame; nothing on
// a Raspberry Pi's path needs more and a larger value is far more likely to be
// a typo than an intention.
const (
	MinTunMTU = 1280
	MaxTunMTU = 9000
)

// Defaults returns the Options this appliance uses when nothing has been
// chosen. It is exported so the panel can render the defaults beside the
// operator's overrides without restating them.
func Defaults() Options {
	return Options{
		LogLevel: LogWarning,
		TUN: TUN{
			Name: DefaultTunName,
			MTU:  DefaultTunMTU,
		},
		SOCKS: SOCKS{
			Listen: DefaultSocksListen,
			Port:   DefaultSocksPort,
		},
		DNS: DNS{
			Servers:  DefaultResolvers(),
			Strategy: QueryUseIP,
		},
		LocalDNS: LocalDNS{
			Listen: DefaultLocalDNSListen,
			Port:   DefaultLocalDNSPort,
		},
	}
}

// DefaultSocksListen is the loopback address the diagnostics inbound binds.
const DefaultSocksListen = "127.0.0.1"

// normalise fills in every empty field with its default and leaves everything
// else alone. It never overwrites a value the caller set, including a false
// bool, which is why the bools are written so that false is the default.
func (o Options) normalise() Options {
	if o.LogLevel == "" {
		o.LogLevel = LogWarning
	}
	if o.TUN.Name == "" {
		o.TUN.Name = DefaultTunName
	}
	if o.TUN.MTU == 0 {
		o.TUN.MTU = DefaultTunMTU
	}
	if o.SOCKS.Listen == "" {
		o.SOCKS.Listen = DefaultSocksListen
	}
	if o.SOCKS.Port == 0 {
		o.SOCKS.Port = DefaultSocksPort
	}
	if len(o.DNS.Servers) == 0 {
		o.DNS.Servers = DefaultResolvers()
	} else {
		// Copy, so that the document cannot be changed under the engine by a
		// caller mutating the slice it passed in.
		o.DNS.Servers = append([]string(nil), o.DNS.Servers...)
	}
	if o.DNS.Strategy == "" {
		o.DNS.Strategy = QueryUseIP
	}
	if o.LocalDNS.Listen == "" {
		o.LocalDNS.Listen = DefaultLocalDNSListen
	}
	if o.LocalDNS.Port == 0 {
		o.LocalDNS.Port = DefaultLocalDNSPort
	}
	return o
}

// check validates a normalised Options. Errors name the field, never a value.
func (o Options) check() error {
	switch o.LogLevel {
	case LogDebug, LogInfo, LogWarning, LogError:
	default:
		return ErrLogLevel
	}
	if !o.TUN.Disabled {
		if err := checkInterfaceName(o.TUN.Name); err != nil {
			return err
		}
		if o.TUN.MTU < MinTunMTU || o.TUN.MTU > MaxTunMTU {
			return ErrTunMTU
		}
	}
	if err := checkLoopbackListen(o.SOCKS.Listen, ErrSocksAddress); err != nil {
		return err
	}
	if o.SOCKS.Port == 0 {
		return ErrSocksPort
	}
	if o.LocalDNS.Enabled {
		if err := checkLoopbackListen(o.LocalDNS.Listen, ErrLocalDNSAddress); err != nil {
			return err
		}
		if o.LocalDNS.Port == 0 {
			return ErrLocalDNSPort
		}
		// Two inbounds on one address and port is a failure the engine will
		// NOT report here. engine.Validate stops after Build and opens no
		// socket (internal/engine/engine.go:300-322), so a collision surfaces
		// only at Start, as a bind error, after the panel has already told
		// the user the config was accepted.
		if o.LocalDNS.Port == o.SOCKS.Port && sameHost(o.LocalDNS.Listen, o.SOCKS.Listen) {
			return ErrInboundCollision
		}
	}
	switch o.DNS.Strategy {
	case QueryUseIP, QueryUseIPv4, QueryUseIPv6:
	default:
		return ErrQueryStrategy
	}
	return checkResolvers(o.DNS.Servers)
}

// checkLoopbackListen enforces "never on a wildcard address" for any inbound
// this package binds.
//
// It is written as an allow-list rather than a deny-list on purpose. Rejecting
// "0.0.0.0" and "::" by name leaves "0.0.0.0.0"-shaped typos, v4-mapped forms
// such as "::ffff:0.0.0.0", and any interface address the operator pastes in
// by mistake. Requiring an address that IS loopback rejects all of them
// including the ones nobody thought of.
func checkLoopbackListen(s string, bad error) error {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return bad
	}
	if !addr.Unmap().IsLoopback() {
		return bad
	}
	return nil
}

// sameHost reports whether two listen addresses name the same interface
// address. It compares parsed values rather than strings, so that "::1" and
// "0:0:0:0:0:0:0:1" are recognised as the same bind and a collision cannot be
// hidden behind a different spelling.
func sameHost(a, b string) bool {
	x, err1 := netip.ParseAddr(a)
	y, err2 := netip.ParseAddr(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return x.Unmap() == y.Unmap()
}

// maxInterfaceName is IFNAMSIZ minus the terminating NUL, the limit the
// kernel applies to the name passed to TUNSETIFF in proxy/tun/tun_linux.go.
const maxInterfaceName = 15

// checkInterfaceName rejects a name the kernel would refuse or that would make
// a shell-quoted command elsewhere in this program ambiguous.
//
// The engine does not check this: infra/conf/tun.go:14-30 copies the name
// through, and proxy/tun/tun_linux.go passes it to TUNSETIFF, so a bad name
// surfaces as an ioctl failure at start time with the name in the message.
func checkInterfaceName(s string) error {
	if s == "" || len(s) > maxInterfaceName {
		return ErrTunName
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.'
		if !ok {
			return ErrTunName
		}
	}
	if s == "." || s == ".." {
		return ErrTunName
	}
	return nil
}
