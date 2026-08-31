// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// systemResolver looks the server name up with the host's own resolver.
type systemResolver struct{}

// Resolve implements Resolver.
//
// It asks for both families. A server with only an AAAA record on a box with no
// IPv6 default route is refused by internal/netcfg with wording for the user
// (ErrServerFamilyUnreachable), which is a better answer than silently pinning
// nothing and reporting a config problem that is not one.
func (systemResolver) Resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		// An IP literal needs no lookup, and skipping it is the one shape that
		// puts nothing about the user's server on the local network.
		return []netip.Addr{addr.Unmap()}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		// The host name comes from the user's config. It is not quoted here:
		// this error is logged, and docs/LAYOUT.md says the config is never
		// printed or logged. The name alone would identify the server.
		return nil, fmt.Errorf("privsvc: the proxy server's address could not be looked up")
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.Unmap())
	}
	return out, nil
}

// tcpReachability dials the server and reports whether anything answered.
//
// WHAT A SUCCESS HERE PROVES, stated because the word "connected" is the
// easiest thing in this product to claim without evidence: a TCP connection was
// accepted at that address and port. It is not a proxy handshake, it is not an
// authenticated session, and it is not an exit IP captured from real traffic,
// which is what docs/2026-08-29-design.md section 6 requires before anything is
// called working. Nothing in this package captures one.
//
// It dials out of the uplink, not through the tunnel, which is the same path
// the engine's own connection to the server takes once the pinned host route is
// in place.
type tcpReachability struct{}

// Probe implements Reachability.
func (tcpReachability) Probe(ctx context.Context, addr netip.Addr, port uint16) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(addr, port).String())
	if err != nil {
		return fmt.Errorf("privsvc: the proxy server did not answer")
	}
	return c.Close()
}
