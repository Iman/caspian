// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"strings"
	"testing"
)

// The ways a query could leave a joined device, one test each, INCLUDING the
// one this design does not stop.
//
// TestRuleset_RedirectsClientDNSRatherThanAllowingIt already covers plain DNS
// on 53 and encrypted DNS on 853. What follows covers the paths that were
// enumerated and had no test behind them, and it covers them as they actually
// are rather than as it would be convenient for them to be.

// TestRuleset_DNSOverHTTPSIsCarriedByTheTunnelAndNotBlocked.
//
// THIS TEST DOES NOT ASSERT THAT DNS OVER HTTPS IS BLOCKED. It asserts the
// opposite, on purpose, because that is what is true and because a test
// claiming otherwise would be worse than no test at all.
//
// DNS over HTTPS is HTTPS. It arrives on port 443 with a TLS record layer and
// an SNI that names a resolver, and nothing about it is distinguishable at the
// packet level from any other HTTPS flow without inspecting content this
// appliance deliberately does not inspect. The generator says so in its own
// words above the 853 rules, docs/BEHAVIOUR.md says so, and
// test/hardware/steps/dnsleak.sh says so in its output, so this is the fourth
// place the same limit is recorded and the only one that is executable.
//
// What IS guaranteed, and what this pins, is the weaker and true statement: a
// client using DNS over HTTPS is inside the tunnel like everything else. Its
// queries do not reach the resolver on this box, and they do not leave beside
// the tunnel either. The provider on the far end sees them; the local network
// does not.
//
// Two assertions, and the first is the one that matters:
//
//  1. No rule anywhere in the generated ruleset matches port 443. If one ever
//     appears, the claim above stops being true in one direction or the other
//     and every one of those four documents has to be revisited.
//  2. The only forward accepts name the tunnel, so the 443 flow that is not
//     matched by any rule of its own is carried by the tunnel accept and by
//     nothing else.
func TestRuleset_DNSOverHTTPSIsCarriedByTheTunnelAndNotBlocked(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	rs := p.Ruleset()

	for _, l := range ruleLines(rs) {
		if strings.Contains(l, "dport 443") || strings.Contains(l, "sport 443") {
			t.Errorf(
				"a rule now matches port 443: %q.\n"+
					"This appliance has never filtered DNS over HTTPS and four places say so: the comment "+
					"above the 853 rules in this generator, docs/BEHAVIOUR.md, "+
					"test/hardware/steps/dnsleak.sh, and this test. A rule on 443 either blocks ordinary "+
					"web traffic or creates the appearance of filtering encrypted DNS without doing it. "+
					"Whichever it is, those documents are now wrong and have to be corrected in the same "+
					"change.", l)
		}
	}

	fwd := chainBody(t, rs, "forward")
	accepts := 0
	for _, l := range fwd {
		if !strings.HasSuffix(strings.SplitN(l, " comment ", 2)[0], "accept") &&
			!strings.Contains(l, " accept") {
			continue
		}
		if !strings.Contains(l, `"`+p.Tun+`"`) {
			t.Errorf(
				"the forward chain permits traffic without naming the tunnel %q: %q. A client's HTTPS, "+
					"and therefore its DNS over HTTPS, would have a way out that does not depend on the "+
					"tunnel existing.", p.Tun, l)
		}
		accepts++
	}
	if accepts == 0 {
		t.Fatal("the forward chain permits nothing at all, so no client reaches the internet and this " +
			"test is asserting nothing about how DNS over HTTPS is carried")
	}
}

// TestRuleset_ClientMulticastNameResolutionReachesNeitherTheBoxNorTheUplink.
//
// mDNS on 5353 and LLMNR on 5355 are name resolution that never goes near the
// configured resolver, so the redirect on 53 is irrelevant to them and the
// question they raise is different: not "where does the query go" but "who
// answers it, and can it leave".
//
// Three things have to hold, and only the third is covered anywhere else:
//
//  1. This box does not answer. The input chain ends in a drop for the
//     hotspot, so a multicast query to 5353 or 5355 reaches nothing here. That
//     matters because a Debian-derived box commonly runs avahi, which would
//     otherwise answer on the hotspot without this program ever deciding it
//     should.
//  2. It cannot be forwarded off the box. The forward policy is drop and the
//     only accepts name the tunnel.
//  3. It cannot reach another joined device, which client isolation covers and
//     which hostapd's ap_isolate covers at the layer below.
//
// The first is asserted by ORDER, not by presence. A drop for the hotspot that
// sits above the accepts is a hotspot that reaches nothing at all, and a drop
// below them that is not last is a drop with a hole under it.
func TestRuleset_ClientMulticastNameResolutionReachesNeitherTheBoxNorTheUplink(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	rs := p.Ruleset()
	hot := `"` + p.Hotspot + `"`

	in := chainBody(t, rs, "input")
	lastHotspotRule := -1
	for i, l := range in {
		if strings.Contains(l, "iifname "+hot) {
			lastHotspotRule = i
		}
	}
	if lastHotspotRule < 0 {
		t.Fatalf("the input chain has no rule naming the hotspot %s at all", hot)
	}
	if got := in[lastHotspotRule]; !strings.Contains(got, " drop") {
		t.Errorf(
			"the last input rule naming the hotspot is %q, and it is not a drop. Every service this box "+
				"offers a joined device is named above it; without a drop underneath, a device reaches "+
				"whatever else happens to be listening on this machine. On a Debian-derived box that "+
				"includes avahi on 5353, which would answer mDNS for the hotspot without this program "+
				"having decided anything.", got)
	}

	// And no service the box offers is one of the multicast resolution ports,
	// which would make the drop above unreachable for exactly the traffic it is
	// being relied on for.
	for _, l := range in {
		if !strings.Contains(l, "iifname "+hot) || !strings.Contains(l, "accept") {
			continue
		}
		for _, port := range []string{"dport 5353", "dport 5355"} {
			if strings.Contains(l, port) {
				t.Errorf("the input chain accepts %s from the hotspot: %q. This box would answer "+
					"multicast name resolution for joined devices, which it does not implement and has "+
					"not decided to offer", port, l)
			}
		}
	}

	// Nothing may be forwarded off the box except by the tunnel, which covers
	// the multicast case along with everything else. Asserted here rather than
	// borrowed, so this test fails on its own terms.
	fwd := chainBody(t, rs, "forward")
	if !strings.Contains(strings.Join(fwd, "\n"), "policy drop") {
		t.Error("the forward policy is not drop, so multicast name resolution from a joined device is " +
			"carried by whatever route exists rather than by a rule anybody wrote")
	}

	// Client to client, which is where mDNS is actually used and where the
	// only thing stopping it is the isolation rule.
	if !strings.Contains(strings.Join(fwd, "\n"), "iifname "+hot+" oifname "+hot+" drop") {
		t.Errorf("the forward chain does not drop hotspot-to-hotspot traffic, so one joined device "+
			"resolves and reaches another. DefaultOptions sets ClientIsolation true; this ruleset does "+
			"not carry it.\nforward chain:\n  %s", strings.Join(fwd, "\n  "))
	}
}

// TestRuleset_TheDNSRedirectIsNotWrittenForOneAddressFamilyOnly.
//
// The prerouting redirect rules carry no nfproto qualifier, and in the inet
// family that means they match IPv4 AND IPv6. That is not an accident to be
// tidied up: a rule qualified to ipv4 would leave a v6 query on 53 unredirected
// in a ruleset whose IPv6 story is a set of drops elsewhere, and the two would
// then have to be reasoned about together.
//
// This pins the current shape so that adding a qualifier is a decision somebody
// makes rather than a tidy-up, and records what it would mean.
//
// It says NOTHING about whether a client can reach a v6 resolver. That depends
// on whether a client can obtain a routable v6 address at all, which is decided
// by the router-advertisement drop in this ruleset and by internal/hotspot not
// serving DHCPv6, and which has never been exercised on hardware: the harness
// network has no IPv6 (test/hardware/README.md, "What this vantage cannot
// grade: IPv6").
func TestRuleset_TheDNSRedirectIsNotWrittenForOneAddressFamilyOnly(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	pre := chainBody(t, p.Ruleset(), "prerouting")

	found := 0
	for _, l := range pre {
		if !strings.Contains(l, "redirect to :") {
			continue
		}
		found++
		if strings.Contains(l, "nfproto ipv4") || strings.Contains(l, "nfproto ipv6") {
			t.Errorf(
				"the DNS redirect %q is now qualified to one address family. In the inet family an "+
					"unqualified rule redirects both, and the unqualified form is what this ruleset has "+
					"relied on. If this is deliberate, say which family is now unredirected on port 53 and "+
					"what answers it, because the drops that cover client IPv6 are in the FORWARD chain "+
					"and a redirected packet never reaches the forward chain.", l)
		}
	}
	if found != 2 {
		t.Errorf("expected two redirect rules in prerouting (udp and tcp), found %d:\n  %s",
			found, strings.Join(pre, "\n  "))
	}
}
