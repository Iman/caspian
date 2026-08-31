// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// MinLeaseTime is dnsmasq's own floor. Anything shorter is refused by dnsmasq
// at startup, which would show as a hotspot that hands out no addresses.
const MinLeaseTime = 2 * time.Minute

// DefaultServiceUser and DefaultServiceGroup are the service account this
// appliance runs unprivileged work as, fixed in docs/LAYOUT.md ("Service user
// and group | caspian, a system account with no login shell") and created by
// the installer. They are defaults for DNSConfig.ServiceUser and
// DNSConfig.ServiceGroup rather than constants written into the renderer, so
// that an installer that ever changes the account name changes one value and
// the generated file follows it. A name hardcoded in two places is a name that
// drifts.
const (
	DefaultServiceUser  = "caspian"
	DefaultServiceGroup = "caspian"
)

// bannedResolvers are addresses that must never be written into the generated
// dnsmasq configuration as a forwarding target.
//
// Two independent reasons, and either alone is enough:
//
//   - docs/2026-08-29-design.md section 2 and section 6: Google is not used
//     anywhere in this project, including as a resolver default. The reference
//     implementation used 8.8.8.8 in three places
//     (004-hotspot/install.sh:143 and :339, xray-hotspot-fixed.sh:253).
//   - Any address here at all that is not the local resolver defeats the
//     point. Client DNS is answered on the box and resolved through the
//     tunnel; a public resolver in this file is a plaintext query leaving
//     beside the tunnel, from the box, for every name every client looks up.
//
// Upstream is required to be a loopback address, so this list is a second
// gate rather than the only one. It is here because a named check fails
// loudly and a general rule can be quietly relaxed.
var bannedResolvers = []string{
	"8.8.8.8", "8.8.4.4",
	"2001:4860:4860::8888", "2001:4860:4860::8844",
}

// DNSConfig is everything needed to render a dnsmasq configuration that serves
// DHCP and DNS to the hotspot.
type DNSConfig struct {
	// Interface is the hotspot interface. Supplied by the caller.
	Interface string

	// Subnet is the hotspot subnet, for example 192.168.66.0/24. IPv4 only in
	// v1: see the comment in RenderDnsmasq about IPv6.
	Subnet netip.Prefix

	// Gateway is this box's address on the hotspot. It is handed to clients
	// as both their router and their DNS server.
	Gateway netip.Addr

	// RangeStart and RangeEnd bound the DHCP pool.
	RangeStart, RangeEnd netip.Addr

	// LeaseTime is the DHCP lease length, at least MinLeaseTime.
	LeaseTime time.Duration

	// LeaseFile is where dnsmasq records leases. ReadLeaseFile parses it.
	LeaseFile string

	// Upstream is the local resolver this program runs, the one that sends
	// queries through the tunnel. It MUST be a loopback address: see
	// bannedResolvers and the Validate check.
	Upstream netip.AddrPort

	// CacheSize is dnsmasq's in-memory answer cache. 0 disables caching.
	CacheSize int

	// ServiceUser and ServiceGroup are the account dnsmasq drops to after it
	// has bound port 53. They are required: see the user= block in
	// RenderDnsmasq for the failure that an unset one causes. Use
	// DefaultServiceUser and DefaultServiceGroup unless the installer says
	// otherwise; NewPlan fills them in when they are empty.
	ServiceUser  string
	ServiceGroup string

	// FilterAAAA makes dnsmasq drop AAAA answers, so clients do not try to
	// reach IPv6 destinations the v1 tunnel does not carry. Off by default:
	// the option is a dnsmasq 2.81 addition and an older dnsmasq treats an
	// unknown option as fatal, so the hotspot would not start at all. Turn it
	// on only where the dnsmasq version has been checked.
	//
	// MEASURED on the reference Raspberry Pi on 2026-08-30:
	//
	//	/usr/sbin/dnsmasq --version
	//	Dnsmasq version 2.91  Copyright (c) 2000-2025 Simon Kelley
	//
	// 2.91 is well past the 2.81 floor, so the option would be accepted rather
	// than fatal THERE. That settles one box and no others, which is exactly
	// why the default stays off and this field stays a caller's decision: the
	// appliance is installed on machines nobody has measured, and an unknown
	// option is fatal on every one of them that is older.
	FilterAAAA bool
}

// Validate reports whether the configuration can be rendered.
func (c DNSConfig) Validate() error {
	if err := validConfigToken("interface", c.Interface); err != nil {
		return err
	}

	if !c.Subnet.IsValid() {
		return errors.New("hotspot: no hotspot subnet was set")
	}
	if !c.Subnet.Addr().Is4() {
		return errors.New("hotspot: the hotspot subnet must be an IPv4 network in this version")
	}
	if c.Subnet.Bits() < 8 || c.Subnet.Bits() > 30 {
		return fmt.Errorf("hotspot: a /%d hotspot subnet has no usable range of addresses", c.Subnet.Bits())
	}
	if c.Subnet.Addr() != c.Subnet.Masked().Addr() {
		return fmt.Errorf("hotspot: %s is a host address, not a network (did you mean %s?)", c.Subnet, c.Subnet.Masked())
	}

	// A slice, not a map: the order errors are reported in has to be the same
	// on every run, or the message the panel shows depends on map iteration.
	for _, f := range []struct {
		name string
		addr netip.Addr
	}{
		{"gateway address", c.Gateway},
		{"first DHCP address", c.RangeStart},
		{"last DHCP address", c.RangeEnd},
	} {
		if !f.addr.IsValid() || !f.addr.Is4() {
			return fmt.Errorf("hotspot: the %s is not an IPv4 address", f.name)
		}
		if !c.Subnet.Contains(f.addr) {
			return fmt.Errorf("hotspot: the %s %s is not inside the hotspot subnet %s", f.name, f.addr, c.Subnet)
		}
	}
	if c.RangeStart.Compare(c.RangeEnd) > 0 {
		return fmt.Errorf("hotspot: the DHCP range %s to %s runs backwards", c.RangeStart, c.RangeEnd)
	}
	// The box's own address inside the pool would be handed to a client, and
	// two machines answering for the gateway takes the hotspot down in a way
	// that looks intermittent.
	if c.Gateway.Compare(c.RangeStart) >= 0 && c.Gateway.Compare(c.RangeEnd) <= 0 {
		return fmt.Errorf("hotspot: the gateway address %s is inside the DHCP range %s to %s and would be given away",
			c.Gateway, c.RangeStart, c.RangeEnd)
	}

	if c.LeaseTime < MinLeaseTime {
		return fmt.Errorf("hotspot: a DHCP lease of %s is shorter than the %s minimum", c.LeaseTime, MinLeaseTime)
	}
	if err := validAbsPath("lease file", c.LeaseFile); err != nil {
		return err
	}

	if !c.Upstream.IsValid() || c.Upstream.Port() == 0 {
		return errors.New("hotspot: no local resolver address was set for client DNS")
	}
	up := c.Upstream.Addr()
	for _, banned := range bannedResolvers {
		if up.String() == banned {
			return fmt.Errorf("hotspot: %s must never be used as a resolver by this program", banned)
		}
	}
	if !up.IsLoopback() {
		// The design's fail-closed rule (section 6 and section 7) is that
		// client DNS is answered on the box and resolved through the tunnel.
		// A non-loopback forwarding target here is a query leaving the box
		// outside the tunnel, for every name every client asks for.
		return fmt.Errorf("hotspot: client DNS must forward to a resolver on this machine, and %s is not one", up)
	}

	// The account dnsmasq ends up running as decides whether it can write the
	// lease file at all, so an unnamed one is refused rather than defaulted
	// here: NewPlan supplies the documented value, and a caller building a
	// DNSConfig by hand has to say who.
	if err := validServiceAccount("service user", c.ServiceUser); err != nil {
		return err
	}
	if err := validServiceAccount("service group", c.ServiceGroup); err != nil {
		return err
	}

	if c.CacheSize < 0 {
		return fmt.Errorf("hotspot: a DNS cache size of %d makes no sense", c.CacheSize)
	}
	return nil
}

// RenderDnsmasq turns a validated DNSConfig into a dnsmasq configuration file.
//
// Pure: no files, no processes, no network. Fixed by testdata/dnsmasq.golden.
func RenderDnsmasq(c DNSConfig) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}

	var b strings.Builder
	p := func(format string, args ...any) {
		fmt.Fprintf(&b, format+"\n", args...)
	}

	p("# dnsmasq configuration generated by Caspian-BYOC.")
	p("# Rewritten from scratch on every start. Edits here are lost.")
	p("#")
	p("# This file serves DHCP and DNS to the hotspot only. Two things in it")
	p("# are not tuning and must not be relaxed: what it refuses to log, and")
	p("# where it is allowed to send a query.")
	p("")
	p("# WHO THIS PROCESS RUNS AS.")
	p("#")
	p("# dnsmasq needs root only to bind port 53 and to open the DHCP socket.")
	p("# It then drops to an unprivileged account for the rest of its life,")
	p("# which is right, and which is exactly why this line has to be here.")
	p("#")
	p("# THE FAILURE THIS PREVENTS IS SILENCE, NOT AN ERROR. With no user=")
	p("# line dnsmasq drops to its own compiled-in default account, which on a")
	p("# Debian-derived system is neither root nor this appliance's service")
	p("# user. The lease file below lives in a directory that is mode 0700 and")
	p("# owned by the service user (docs/LAYOUT.md: /var/lib/caspian, 0700,")
	p("# caspian), and a process running as somebody else cannot traverse it.")
	p("# So dnsmasq keeps answering DNS, keeps handing out addresses, and")
	p("# keeps failing to record a single lease. The hotspot works. The panel")
	p("# reads an empty lease file and reports zero connected devices for")
	p("# ever, on a network people are actively using. Nothing logs an error,")
	p("# nothing crashes, and a non-technical user is left comparing a screen")
	p("# that says nobody is connected against a phone that is plainly online.")
	p("#")
	p("# group= is set for the same reason and is not implied by user=:")
	p("# dnsmasq's default group is a compiled-in value of its own, so leaving")
	p("# it out leaves half the identity unstated.")
	p("user=%s", c.ServiceUser)
	p("group=%s", c.ServiceGroup)
	p("")
	p("# Listen on the hotspot interface only.")
	p("#")
	p("# bind-interfaces stops dnsmasq binding the wildcard address. Without")
	p("# it dnsmasq listens on 0.0.0.0 and filters by interface in userspace,")
	p("# which means the port is open on the uplink and on any interface that")
	p("# appears later, and this box would be an open resolver to whatever")
	p("# network it is plugged into.")
	p("interface=%s", c.Interface)
	p("bind-interfaces")
	p("listen-address=%s", c.Gateway)
	p("port=53")
	p("")
	p("# PRIVACY, NOT TUNING: no query logging.")
	p("#")
	p("# There is deliberately no log-queries line. With it, dnsmasq writes one")
	p("# syslog line per lookup, which on this box is a dated record of every")
	p("# site every person on the hotspot visited, kept on the appliance that")
	p("# exists to stop exactly that being collected elsewhere. The reference")
	p("# implementation set log-queries (004-hotspot/install.sh:352).")
	p("#")
	p("# The cache below is in memory only. It is lost when the process stops")
	p("# and is never written to disk.")
	if c.CacheSize > 0 {
		p("cache-size=%d", c.CacheSize)
	} else {
		p("cache-size=0")
	}
	p("")
	p("# PRIVACY, NOT TUNING: no DHCP logging.")
	p("#")
	p("# Leaving out log-dhcp is not enough. dnsmasq logs every DHCPDISCOVER,")
	p("# DHCPOFFER, DHCPREQUEST and DHCPACK by default, each carrying a MAC")
	p("# address and a hostname, so the default configuration keeps a dated")
	p("# record of every device that joined and when. quiet-dhcp turns that")
	p("# off. The reference implementation went the other way and set log-dhcp")
	p("# (004-hotspot/install.sh:381).")
	p("#")
	p("# The lease file remains, because the panel has to show the user how")
	p("# many devices are connected. It holds current leases only and dnsmasq")
	p("# removes an entry when it expires; it is not a history.")
	p("quiet-dhcp")
	p("quiet-dhcp6")
	p("quiet-ra")
	p("")
	p("# The only place a client query may go.")
	p("#")
	p("# no-resolv stops dnsmasq reading /etc/resolv.conf. Without it dnsmasq")
	p("# inherits whatever resolver the uplink's DHCP handed this box, and")
	p("# every client lookup would leave in plaintext beside the tunnel")
	p("# instead of through it. That is the leak the whole appliance exists to")
	p("# prevent, and it would be caused by a file this program never wrote.")
	p("#")
	p("# The server line points at a resolver on this machine, which the rest")
	p("# of this program routes through the tunnel. A public resolver must")
	p("# never appear here; the renderer refuses to emit one that is not a")
	p("# loopback address.")
	p("no-resolv")
	p("server=%s#%d", c.Upstream.Addr(), c.Upstream.Port())
	p("")
	p("# Do not send a bare name with no dot upstream, and do not send reverse")
	p("# lookups for private address space upstream. Both would hand the names")
	p("# and addresses of devices on this hotspot to a stranger, and neither")
	p("# has a useful answer.")
	p("domain-needed")
	p("bogus-priv")
	p("")
	p("# Do not answer clients from this box's /etc/hosts.")
	p("no-hosts")
	if c.FilterAAAA {
		p("")
		p("# Drop AAAA answers. The v1 tunnel carries IPv4, so an IPv6 address")
		p("# handed to a client is a destination it will try first and cannot")
		p("# reach, which shows up as every page taking seconds to load.")
		p("# Off unless the caller turned it on: this option needs dnsmasq")
		p("# 2.81 or newer and an older dnsmasq treats an unknown option as")
		p("# fatal, so guessing here would stop the hotspot from starting.")
		p("filter-AAAA")
	}
	p("")
	p("# DHCP.")
	p("#")
	p("# dhcp-authoritative is correct because this box is the only DHCP")
	p("# server on its own hotspot: it lets dnsmasq answer a client that")
	p("# arrives asking to renew a lease from some other network immediately,")
	p("# instead of leaving it to time out.")
	p("#")
	p("# option:dns-server points at this box, not at a public resolver, so a")
	p("# client that honours DHCP asks us. A client with a hardcoded resolver")
	p("# ignores this line; catching that one is internal/netcfg's redirect,")
	p("# not this file's job.")
	p("dhcp-authoritative")
	p("dhcp-range=%s,%s,%s,%s", c.RangeStart, c.RangeEnd, ipv4Netmask(c.Subnet.Bits()), leaseTimeString(c.LeaseTime))
	p("dhcp-option=option:router,%s", c.Gateway)
	p("dhcp-option=option:dns-server,%s", c.Gateway)
	p("# The lease file is the panel's only source for the connected-device")
	p("# count, and it is writable only because of the user= line at the top")
	p("# of this file. Change one without the other and the count silently")
	p("# stays at zero.")
	p("dhcp-leasefile=%s", c.LeaseFile)

	return b.String(), nil
}

// ipv4Netmask renders a prefix length as a dotted-quad mask, which is the form
// dnsmasq's dhcp-range takes.
func ipv4Netmask(bits int) string {
	mask := ^uint32(0) << (32 - bits)
	return fmt.Sprintf("%d.%d.%d.%d", byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask))
}

// leaseTimeString renders a duration the way dnsmasq's dhcp-range expects.
// Whole hours and whole minutes get their suffix; anything else goes as
// seconds, which dnsmasq accepts as a bare number.
func leaseTimeString(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int64(d/time.Hour))
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int64(d/time.Minute))
	default:
		return fmt.Sprintf("%d", int64(d/time.Second))
	}
}
