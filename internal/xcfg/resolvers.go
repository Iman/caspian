// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"fmt"
	"net/netip"
)

// The resolver policy.
//
// docs/2026-08-29-design.md fixes two things: section 2 and section 6 both say
// Google is not used anywhere, INCLUDING as a resolver default; and section 6
// says client DNS is resolved through the tunnel. The first is enforced here
// and in TestNoGoogleAnywhereInGeneratedConfigs. The second is a routing
// question and lives in build.go.
//
// # Where the shape of this list comes from, and where it deliberately stops
//
// The chain in the sibling project this design borrows its shape from is
// Quad9, then Cloudflare FAMILY, then Google, and its third tier is Google BY
// AN EXPLICIT AND DOCUMENTED DECISION. Read on 2026-08-30 at
// javidgorz-deploy/inventory/group_vars/gorz_servers.yml:211-218, which says
// tier 3 "was reinstated by the maintainer on 2026-08-12 ... the last resort
// optimises for AVAILABILITY, and resolving something beats resolving
// nothing", and lists 8.8.8.8, 8.8.4.4 and their v6 pair at :304-307.
//
// That decision is not this product's decision. Caspian-BYOC's design forbids
// Google outright, so this package copies the first two tiers and replaces the
// third. Copying the whole chain would have imported the one entry the design
// prohibits, which is exactly the failure mode that makes borrowed defaults
// dangerous.
//
// # The three, and why each one
//
//   - 9.9.9.9, Quad9's filtered service. Swiss non-profit foundation, filters
//     known malware and phishing, validates DNSSEC. Deliberately not 9.9.9.10,
//     which the same file records at :196-200 as a DIFFERENT service that does
//     neither, rather than a backup for the .9 address.
//   - 1.1.1.3, the Cloudflare FAMILY variant. Deliberately not 1.1.1.1, which
//     filters nothing, and deliberately not 1.1.1.2, which is malware-only.
//     The three addresses differ by one character and the difference is
//     invisible at runtime, which is why the variant is named in this comment
//     and checked in a test rather than left to whoever edits the list next.
//   - 185.228.168.9, CleanBrowsing Security, the address named at
//     javidgorz-deploy/test/resolver-policy-check.sh:162. A third operator in a
//     third jurisdiction, so a failure of one provider is not a failure of
//     the chain.
//
// # What is NOT claimed about the third entry
//
// CleanBrowsing is chosen from a measurement made somewhere else, for a
// different network. javidgorz-deploy/inventory/group_vars/gorz_servers.yml
// :226-231, dated 2026-08-09 and taken from four fleet boxes (fsn1-1,
// do-1-1-ams2, 5tqv, 5tqs), records that dns0.eu was dead, Mullvad refused
// plain UDP/53, AdGuard had no malware-only variant, DNS4EU sinkholed
// psiphon3.com, psiphon.ca and ultrasurf.us, and "CleanBrowsing Security
// remains measured-good at what it does".
//
// Reachability and filtering are properties of the network path, so the
// question is WHICH path these queries take. An earlier draft of this comment
// said the missing vantage was the Pi's. That was wrong, and the correction is
// worth keeping because it changes what would be worth measuring.
//
// These queries traverse the tunnel. The routing rule tagged
// ruleTagResolvers sends every query the built-in resolver makes to the proxy
// outbound, above the private rule, so it leaves from the user's exit server
// and not from their local network. A resolver that is sinkholed, hijacked or
// blocked on the network the Pi is plugged into is therefore irrelevant: the
// query never touches that network. The Pi's vantage does not decide this.
//
// The vantage that DOES decide it is the exit server's, and that cannot be
// known in advance, because the user chooses the server. There is no
// measurement to take here that would settle the question for everybody: a
// resolver that answers well from one exit may be filtered from another.
//
// That is an argument for the list being configurable, which it is, and not
// for a different default. The measurement above still says what it says about
// these operators; it just describes a vantage that is nobody's exit in
// particular.

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
