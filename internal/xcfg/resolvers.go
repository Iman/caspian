// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"fmt"
	"net/netip"
)

// Resolver policy: Google Public DNS is excluded by the product design.
// The defaults use Quad9 filtered, Cloudflare Family and CleanBrowsing Security.
// Queries travel through the configured tunnel; availability depends on the
// chosen exit server. Operators can configure a different permitted list.

// DefaultResolvers returns the resolver chain this appliance uses when the
// operator has not chosen one. Order is the order the engine tries them in.
//
// It returns a fresh slice on every call so a caller that appends to it cannot
// change the default for everybody else.
func DefaultResolvers() []string {
	return []string{
		"9.9.9.9",       // Quad9, filtered, DNSSEC validating
		"1.1.1.3",       // Cloudflare FAMILY (malware and adult), not 1.1.1.1 and not 1.1.1.2
		"185.228.168.9", // CleanBrowsing Security
	}
}

// googlePrefixes is every network Google Public DNS answers on, as prefixes
// rather than as the four well-known addresses.
//
// A prefix rather than an address list because the check has to survive
// somebody reaching for a neighbour of the famous address: 8.8.4.4 and
// 8.8.8.8 are the documented pair, 2001:4860:4860::8888 and ::8844 the v6
// pair, and ::64 and ::6464 the DNS64 service on the same v6 network. Matching
// the enclosing prefixes covers all of them and anything else that turns up
// there.
var googlePrefixes = []netip.Prefix{
	netip.MustParsePrefix("8.8.8.0/24"),
	netip.MustParsePrefix("8.8.4.0/24"),
	netip.MustParsePrefix("2001:4860:4860::/48"),
}

// checkResolvers validates a resolver list and rejects Google.
//
// Only bare IP literals are accepted. A hostname or a DoH URL would make the
// no-Google rule a question about what a name resolves to at some future
// moment, on a box whose whole job is that its DNS answers are not the ones
// the local network would have given it. A literal makes it a set-membership
// test on the document, which is a test that can actually be written.
//
// Errors name the position, never the value. See errors.go.
func checkResolvers(servers []string) error {
	if len(servers) == 0 {
		return ErrNoResolvers
	}
	for i, s := range servers {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return fmt.Errorf("%w: resolver %d of %d", ErrResolverNotIP, i+1, len(servers))
		}
		// Unmap so that a v4-mapped v6 form of a Google address cannot walk
		// past a v4 prefix check.
		addr = addr.Unmap()
		for _, p := range googlePrefixes {
			if p.Contains(addr) {
				return fmt.Errorf("%w: resolver %d of %d", ErrGoogleResolver, i+1, len(servers))
			}
		}
	}
	return nil
}
