// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"errors"
	"fmt"
	"net/netip"
)

// ErrNoFreeSubnet is returned when every candidate in the pool collides with a
// network the box is already on.
var ErrNoFreeSubnet = errors.New("netcfg: no candidate subnet is free of the networks this machine is already on")

// UserMessage returns wording for the panel. The panel shows plain words and
// never a term like "prefix collision" (design 5.2).
func (e subnetError) UserMessage() string { return e.user }

type subnetError struct {
	err  error
	user string
}

func (e subnetError) Error() string { return e.err.Error() }
func (e subnetError) Unwrap() error { return e.err }

// Overlaps reports whether two prefixes share any address. It masks both
// first, so an unmasked prefix such as 192.168.1.42/24 compares as the network
// it belongs to rather than as a host address.
//
// Prefixes of different families never overlap.
func Overlaps(a, b netip.Prefix) bool {
	if !a.IsValid() || !b.IsValid() {
		return false
	}
	if a.Addr().Is4() != b.Addr().Is4() {
		return false
	}
	return a.Masked().Overlaps(b.Masked())
}

// DefaultHotspotPool is the ordered list of candidate hotspot subnets.
//
// The order is deliberate. Every entry is RFC 1918, and the common ones are
// left out on purpose: 192.168.0.0/24, 192.168.1.0/24 and 10.0.0.0/24 are what
// domestic routers hand out, 172.17.0.0/16 is the Docker default, and 10.8.0.0
// and 10.9.0.0 are the OpenVPN and WireGuard examples that most tutorials
// copy. Choosing one of those is choosing a collision with the network the box
// is plugged into or with a VPN a client is already running, and the symptom is
// a client that reaches nothing while everything reports healthy.
func DefaultHotspotPool() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("10.83.51.0/24"),
		netip.MustParsePrefix("10.174.29.0/24"),
		netip.MustParsePrefix("172.28.113.0/24"),
		netip.MustParsePrefix("172.19.207.0/24"),
		netip.MustParsePrefix("192.168.173.0/24"),
		netip.MustParsePrefix("192.168.221.0/24"),
		netip.MustParsePrefix("10.41.62.0/24"),
		netip.MustParsePrefix("10.216.7.0/24"),
	}
}

// DefaultTunnelPool is the ordered list of candidate subnets for the tunnel
// device itself. A /30 is enough: the device is point to point and the netstack
// behind it terminates flows, so nothing else lives on this network.
//
// 198.18.0.0/15 is the RFC 2544 benchmarking range. It is used here because it
// is routable-looking, is not RFC 1918, and so cannot collide with the home
// network, the uplink or a client's own VPN, which are all the things that
// collide with an RFC 1918 choice.
func DefaultTunnelPool() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("198.18.51.0/30"),
		netip.MustParsePrefix("198.18.83.0/30"),
		netip.MustParsePrefix("198.19.29.0/30"),
	}
}

// ChooseSubnet returns the first prefix in pool that overlaps nothing in taken.
func ChooseSubnet(pool, taken []netip.Prefix) (netip.Prefix, error) {
	for _, cand := range pool {
		clash := false
		for _, t := range taken {
			if Overlaps(cand, t) {
				clash = true
				break
			}
		}
		if !clash {
			return cand.Masked(), nil
		}
	}
	return netip.Prefix{}, subnetError{
		err: fmt.Errorf("%w (%d candidates, %d networks in use)", ErrNoFreeSubnet, len(pool), len(taken)),
		user: "This machine is already connected to too many networks for the hotspot to pick " +
			"an address range of its own. Choose one by hand in advanced settings.",
	}
}

// GatewayAddr returns the address the box takes on a chosen subnet: the first
// address after the network address.
func GatewayAddr(p netip.Prefix) (netip.Addr, error) {
	if !p.IsValid() {
		return netip.Addr{}, errors.New("netcfg: invalid prefix")
	}
	if p.Addr().Is4() && p.Bits() > 30 {
		return netip.Addr{}, fmt.Errorf("netcfg: %v is too small for a gateway address", p)
	}
	a := p.Masked().Addr().Next()
	if !a.IsValid() || !p.Contains(a) {
		return netip.Addr{}, fmt.Errorf("netcfg: %v has no usable host address", p)
	}
	return a, nil
}

// PeerAddr returns the second host address on a prefix. It is the address the
// tunnel device's peer would hold on a point-to-point link.
func PeerAddr(p netip.Prefix) (netip.Addr, error) {
	gw, err := GatewayAddr(p)
	if err != nil {
		return netip.Addr{}, err
	}
	a := gw.Next()
	if !a.IsValid() || !p.Contains(a) {
		return netip.Addr{}, fmt.Errorf("netcfg: %v has no second host address", p)
	}
	return a, nil
}
