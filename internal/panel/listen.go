// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
)

// DefaultPort is the port the panel listens on.
//
// It is not 80. The panel runs as the unprivileged caspian user
// (docs/LAYOUT.md), and binding below 1024 needs either root or a capability on
// the binary; taking either to save the user typing four characters would put
// privilege back into the process whose entire design is to have none.
const DefaultPort = 8088

// ErrWildcardBind is returned for an address that would listen on every
// interface.
//
// This is refused rather than warned about because of what the box is. The
// machine has an uplink to a network the user does not control, and design
// section 5.6 is explicit: the hotspot interface always, the local network only
// if the user turns it on, never the uplink. A wildcard bind ignores all three
// of those clauses at once, and the mistake is invisible in testing because
// everything works.
var ErrWildcardBind = errors.New("panel: refusing to listen on every interface")

// ErrPublicBind is returned for an address that is globally routable.
var ErrPublicBind = errors.New("panel: refusing to listen on a public address")

// ErrNoBindAddress is returned when there is nothing to listen on.
var ErrNoBindAddress = errors.New("panel: no address to listen on")

// ValidateBindAddr checks one host:port the panel has been asked to listen on.
//
// What it refuses, and why each one:
//
//   - an empty host, "0.0.0.0", "[::]", or any other unspecified address, since
//     each of those listens on every interface including the uplink,
//   - a host that is not an IP literal at all. A name is refused because what
//     it resolves to is decided elsewhere and can change under the process; the
//     bind address of this panel is not something DNS gets a vote on,
//   - a globally routable address, which is the "never the uplink" clause with
//     a definition attached. Private, loopback, link-local and unique-local
//     addresses pass.
//
// A multicast address is refused as well, as a nonsense case that would
// otherwise produce a confusing error from the kernel.
func ValidateBindAddr(addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("panel: %q is not an address and port: %w", addr, err)
	}
	// The host is checked before the port, and the order is deliberate. When
	// both are wrong, "this listens on every interface" is the answer the
	// reader needs; a message about the port would send them to fix the
	// harmless half of "0.0.0.0:0".
	if host == "" {
		return fmt.Errorf("%w: %q has no address, which listens everywhere", ErrWildcardBind, addr)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("panel: %q is not an IP address; the panel binds to an address, not a name", host)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("%w: %q listens on every interface, including the uplink", ErrWildcardBind, host)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("panel: %q is a multicast address, which is not something to listen on", host)
	}
	if isGloballyRoutable(ip) {
		return fmt.Errorf("%w: %q is reachable from outside this network", ErrPublicBind, host)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		// Port 0 is refused along with the nonsense values, and that is a
		// product decision rather than an oversight. Port 0 asks the kernel to
		// pick a free port, and the installer has to print an address the user
		// can type into a phone (docs/LAYOUT.md). A panel on a port that
		// changes at every boot is a panel nobody can find, and the failure
		// would appear as "Caspian stopped working" after a power cut.
		return fmt.Errorf("panel: %q does not have a usable port", addr)
	}
	return nil
}

// isGloballyRoutable reports whether an address is one the wider internet can
// reach.
//
// netip.Addr.IsPrivate covers RFC 1918 and RFC 4193 unique-local, and the
// remaining local categories have their own predicates. Carrier-grade NAT
// space, 100.64.0.0/10, is added by hand: it has no predicate in net/netip, a
// box on a mobile or cable uplink can genuinely hold one, and it is not
// reachable from outside the carrier's network.
func isGloballyRoutable(ip netip.Addr) bool {
	switch {
	case ip.IsPrivate(),
		ip.IsLoopback(),
		ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(),
		ip.IsUnspecified():
		return false
	}
	if ip.Is4() && cgnat.Contains(ip) {
		return false
	}
	return true
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// BindAddrs is where the panel listens, given what was detected and whether the
// user has opened it on the local network.
//
// The default is the hotspot interface only. That is the whole of it when
// onLAN is false, and it is deliberately a single address rather than a
// wildcard: the difference between them is the entire security property.
//
// When onLAN is true the box's address on the network it is attached to is
// added, and only if that address is private. A box whose uplink holds a
// globally routable address is exactly the case the "never the uplink" rule
// exists for, and quietly binding to it because the user ticked a box marked
// "local network" would be the opposite of what they asked for.
//
// The hotspot address is empty until the access point has been started, which
// is the hazard design section 5.6 names and does not solve. This function
// reports that as an error rather than silently falling back to a wildcard,
// which is the shape that bug always takes.
func BindAddrs(d Detection, port int, onLAN bool) ([]string, error) {
	if port == 0 {
		port = DefaultPort
	}
	var addrs []string
	add := func(host string) error {
		if host == "" {
			return nil
		}
		a := net.JoinHostPort(host, strconv.Itoa(port))
		if err := ValidateBindAddr(a); err != nil {
			return err
		}
		addrs = append(addrs, a)
		return nil
	}

	if err := add(d.HotspotAddress); err != nil {
		return nil, err
	}
	if onLAN {
		if err := add(d.LocalNetworkAddress); err != nil {
			return nil, err
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: the hotspot has no address yet, so there is nowhere to serve the panel", ErrNoBindAddress)
	}
	return addrs, nil
}

// Listen opens a listener on one validated address.
//
// The validation is repeated here rather than trusted from the caller. This is
// the last point before a socket exists, and the cost of the check is nothing
// against the cost of a panel that is quietly reachable from the uplink.
func Listen(addr string) (net.Listener, error) {
	if err := ValidateBindAddr(addr); err != nil {
		return nil, err
	}
	return net.Listen("tcp", addr)
}
