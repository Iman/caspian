// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import (
	"net/netip"
	"strings"
	"testing"
)

// What this box OFFERS a joining device, as opposed to what it permits.
//
// The two are different guarantees and they fail differently. internal/netcfg's
// prerouting redirect catches a device that ignores what it was offered; these
// tests cover what a device that HONOURS the offer is told to do, which is the
// path almost every device actually takes and the one that leaves no trace in
// the firewall when it is wrong.

// dhcpOptions returns every dhcp-option directive in a rendered configuration.
func dhcpOptions(t *testing.T, conf, option string) []string {
	t.Helper()
	prefix := "dhcp-option=option:" + option + ","
	var out []string
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			out = append(out, strings.TrimPrefix(line, prefix))
		}
	}
	return out
}

// TestTheOnlyResolverOfferedToAClientIsThisBox.
//
// DHCP option 6 is what a device is told to use, and it is the whole of what a
// well-behaved device does. If it named a public resolver, every such device
// would send its queries there; the prerouting redirect would still catch the
// packets, so the leak would not show up on the wire, and the box would answer
// queries addressed to somebody else for ever with nothing reporting anything
// wrong. That is a defect that only a test of the offer can see.
//
// Asserted three ways, because "the right value is present" is not the same
// claim as "no wrong value is present": the option exists, it is the gateway,
// and there is exactly one of it. A second dhcp-option line naming a public
// resolver would leave the first assertion true.
func TestTheOnlyResolverOfferedToAClientIsThisBox(t *testing.T) {
	cfg := testDNS()
	conf, err := RenderDnsmasq(cfg)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}

	got := dhcpOptions(t, conf, "dns-server")
	switch len(got) {
	case 0:
		t.Fatalf("no dhcp-option=option:dns-server is offered at all. A joining device falls back to " +
			"whatever it has cached or compiled in, and the only thing left stopping it is the " +
			"prerouting redirect in internal/netcfg, with no second line of defence behind it")
	case 1:
	default:
		t.Fatalf("%d dhcp-option=option:dns-server directives are offered (%v). Which one a device uses "+
			"is dnsmasq's business and not a property this configuration states", len(got), got)
	}

	if got[0] != cfg.Gateway.String() {
		t.Errorf(
			"joining devices are offered resolver %q, and this box is %q. Every device that honours DHCP "+
				"would address its queries elsewhere; internal/netcfg's redirect would still rewrite them, "+
				"so nothing on the wire would look wrong while the offer was.",
			got[0], cfg.Gateway)
	}

	// And the router option, for the same reason: a device offered a different
	// gateway sends nothing through this box at all.
	routers := dhcpOptions(t, conf, "router")
	if len(routers) != 1 || routers[0] != cfg.Gateway.String() {
		t.Errorf("joining devices are offered router %v, want exactly [%s]", routers, cfg.Gateway)
	}
}

// TestDnsmasqNeitherAdvertisesNorServesIPv6.
//
// docs/BEHAVIOUR.md, "clients are never offered the IPv6 the tunnel cannot
// carry", rests on three mechanisms. Two of them are internal/netcfg's and are
// covered by that package and by the behaviour suite: the box does not forward
// IPv6, and the firewall drops forwarded IPv6 on the hotspot in both
// directions. The third is the one that decides whether a client can obtain a
// routable IPv6 address in the first place, and half of it lives here.
//
// dnsmasq is a router advertisement daemon and a DHCPv6 server as well as a
// DHCP server. Neither is on by default, and the configuration this package
// renders must not turn either on: enable-ra would advertise a prefix on the
// hotspot, and a v6 dhcp-range would hand out addresses. Either one gives a
// client a v6 path it prefers over the tunnelled v4.
//
// quiet-ra and quiet-dhcp6 are present and are NOT that. They suppress logging
// of features that are off, and reading them as evidence the features are off
// is the mistake this test exists to make impossible.
func TestDnsmasqNeitherAdvertisesNorServesIPv6(t *testing.T) {
	conf, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}

	for _, bad := range []struct{ directive, consequence string }{
		{"enable-ra", "dnsmasq advertises an IPv6 prefix on the hotspot, and a client that autoconfigures " +
			"an address from it prefers IPv6 over the tunnelled IPv4 and bypasses the tunnel"},
		{"dhcp-range=::", "dnsmasq hands out IPv6 addresses, which the v1 tunnel does not carry"},
		{"ra-only", "dnsmasq is put into a router-advertisement mode"},
		{"slaac", "dnsmasq advertises a prefix for stateless autoconfiguration"},
		{"ra-names", "dnsmasq derives and serves IPv6 names, which requires router advertisements"},
		{"dhcp-option=option6:", "a DHCPv6 option is offered, which only means anything if DHCPv6 is served"},
	} {
		for _, line := range strings.Split(conf, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, bad.directive) {
				t.Errorf("the generated configuration contains %q: %s", line, bad.consequence)
			}
		}
	}

	// A v6 address anywhere in a dhcp-range is the same defect wearing a
	// different spelling, so the ranges are parsed rather than pattern-matched.
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "dhcp-range=") {
			continue
		}
		for _, field := range strings.Split(strings.TrimPrefix(line, "dhcp-range="), ",") {
			addr, err := netip.ParseAddr(strings.TrimSpace(field))
			if err != nil {
				continue
			}
			if addr.Is6() && !addr.Is4In6() {
				t.Errorf("the DHCP range %q contains the IPv6 address %s, so joined devices are given a "+
					"v6 path the tunnel does not carry", line, addr)
			}
		}
	}
}

// TestNoDirectiveAppearsThatThisRendererWasNotWrittenToEmit.
//
// The leak argument for client DNS is an argument about a CLOSED set. It says:
// no-resolv means dnsmasq reads no resolver but the one named here, and the one
// named here is on this box. That argument is only as good as the claim that
// nothing else in the file can send a query somewhere, and every existing test
// checks a directive it already knows about. None of them can see a directive
// nobody thought of.
//
// So this asserts the whole set of directive keys rather than a list of bad
// ones. A new key goes red whatever it is, and whoever added it has to say what
// it does to the closed set before this test will accept it.
//
// It is deliberately NOT a test of dnsmasq's semantics. Whether some particular
// directive would override no-resolv, add an upstream, or do nothing at all is
// a question about dnsmasq that this repository has not measured, and a test
// that asserted an answer would be stating something nobody here has checked.
// What it asserts instead is that the question does not arise unnoticed.
//
// It overlaps testdata/dnsmasq.golden by design and is not redundant with it.
// The golden fails on a comment edit and is regenerated as a matter of course;
// this fails only on a change to the directives, and says why that matters.
func TestNoDirectiveAppearsThatThisRendererWasNotWrittenToEmit(t *testing.T) {
	// Every key the renderer is known to emit, with why it is allowed to be
	// there. FilterAAAA is included because it is emitted under an option.
	allowed := map[string]string{
		"user":               "the account dnsmasq drops to",
		"group":              "the group dnsmasq drops to",
		"interface":          "serve the hotspot only",
		"bind-interfaces":    "do not bind the wildcard address",
		"listen-address":     "the address on the hotspot to answer on",
		"port":               "the port to bind",
		"cache-size":         "the in-memory answer cache",
		"quiet-dhcp":         "do not log DHCP",
		"quiet-dhcp6":        "do not log DHCPv6",
		"quiet-ra":           "do not log router advertisements",
		"no-resolv":          "do not read the system resolver list",
		"server":             "the one permitted forwarding target",
		"domain-needed":      "do not send bare names upstream",
		"bogus-priv":         "do not send private reverse lookups upstream",
		"no-hosts":           "do not answer from the box's /etc/hosts",
		"filter-AAAA":        "drop AAAA answers, when the caller asked for it",
		"dhcp-authoritative": "answer a client renewing a lease from another network",
		"dhcp-range":         "the DHCP pool",
		"dhcp-option":        "what a joining device is told, checked by its own test above",
		"dhcp-leasefile":     "where leases are recorded",
	}

	cfg := testDNS()
	cfg.FilterAAAA = true
	conf, err := RenderDnsmasq(cfg)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}

	seen := map[string]bool{}
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key := line
		if i := strings.IndexByte(line, '='); i >= 0 {
			key = line[:i]
		}
		seen[key] = true
		if _, ok := allowed[key]; !ok {
			t.Errorf(
				"the generated dnsmasq configuration contains the directive %q, which this renderer was "+
					"not written to emit.\n"+
					"  full line: %s\n"+
					"This file's whole leak argument is that no-resolv plus one loopback server= is the "+
					"COMPLETE set of places a client query can go. A directive nobody enumerated is a hole "+
					"in that argument whether or not it turns out to be harmless. Establish what it does to "+
					"the set, then add it to the allowed map in this test with the reason.",
				key, line)
		}
	}

	// The other direction, so the test cannot be satisfied by a renderer that
	// stopped emitting the directives the argument depends on.
	for _, load := range []string{"no-resolv", "server", "bind-interfaces", "interface", "listen-address"} {
		if !seen[load] {
			t.Errorf("%q is no longer emitted; it is load bearing for where a client query may go", load)
		}
	}
}
