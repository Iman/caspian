// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// The client DNS chain, tested where the halves are joined.
//
// A query from a joined device passes through three components that are
// written in three packages and agree about two numbers:
//
//	device --53--> dnsmasq --LocalDNSPort--> the engine's listener --> tunnel
//	          ^                          ^
//	          |                          the 5354 pairing
//	          the 53 pairing
//
// Neither number is owned by a shared constant. internal/netcfg decides where
// the prerouting redirect lands and which port the input chain permits;
// internal/hotspot decides which port dnsmasq binds and which port it forwards
// to; internal/xcfg decides which port the engine listens on. This package is
// the only one that hands values to all three, so it is the only place either
// pairing can be checked at all.
//
// Both tests below drive Service.Start and read what actually reached the
// wire: the nft input this service loaded and the dnsmasq configuration it
// wrote. Neither reads a golden file or a fixture, so neither can pass by two
// halves of a fixture agreeing with each other.

// redirectPort pulls the port a prerouting redirect lands on out of a loaded
// nftables ruleset.
func redirectPort(t *testing.T, ruleset string) int {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*iifname "[^"]+" udp dport 53 redirect to :(\d+)\s*$`)
	m := re.FindStringSubmatch(ruleset)
	if m == nil {
		t.Fatalf("the loaded ruleset has no \"udp dport 53 redirect to :<port>\" rule, so client DNS is not " +
			"redirected at all and a device with a resolver hardcoded into it reaches the one it was told to use")
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("unparsable redirect port %q", m[1])
	}
	return n
}

// dnsmasqDirective pulls the single value of a "key=value" dnsmasq directive.
func dnsmasqDirective(t *testing.T, conf, key string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			found = append(found, strings.TrimPrefix(line, key+"="))
		}
	}
	switch len(found) {
	case 0:
		t.Fatalf("the generated dnsmasq configuration has no %q directive:\n%s", key, conf)
	case 1:
		return found[0]
	default:
		t.Fatalf("the generated dnsmasq configuration has %d %q directives (%v); the effective one is not "+
			"decidable by reading it", len(found), key, found)
	}
	return ""
}

// renderedDnsmasqPort renders a dnsmasq configuration through internal/hotspot
// and returns the port it binds. It is read from the generator rather than
// written down here, so this file holds no copy of the number under test.
func renderedDnsmasqPort(t *testing.T) int {
	t.Helper()
	conf, err := hotspot.RenderDnsmasq(hotspot.DNSConfig{
		Interface:    "ap0",
		Subnet:       netip.MustParsePrefix("10.83.51.0/24"),
		Gateway:      netip.MustParseAddr("10.83.51.1"),
		RangeStart:   netip.MustParseAddr("10.83.51.50"),
		RangeEnd:     netip.MustParseAddr("10.83.51.200"),
		LeaseTime:    12 * time.Hour,
		LeaseFile:    "/var/lib/caspian/dnsmasq.leases",
		Upstream:     netip.MustParseAddrPort("127.0.0.1:5354"),
		CacheSize:    150,
		ServiceUser:  hotspot.DefaultServiceUser,
		ServiceGroup: hotspot.DefaultServiceGroup,
	})
	if err != nil {
		t.Fatalf("rendering a dnsmasq configuration: %v", err)
	}
	n, err := strconv.Atoi(dnsmasqDirective(t, conf, "port"))
	if err != nil {
		t.Fatalf("internal/hotspot rendered a non-numeric dnsmasq port")
	}
	return n
}

// TestTheDNSRedirectLandsOnThePortDnsmasqBinds is the 53 pairing, and it is
// unguarded everywhere else.
//
// internal/netcfg takes the redirect target from Options.DNSPort, which is a
// settable field with a default of 53 (internal/netcfg/plan.go, DefaultOptions)
// and which its own TestRuleset_RedirectsToACustomResolverPort exercises at
// 5353. internal/hotspot does NOT take dnsmasq's listening port from anything:
// RenderDnsmasq emits "port=53" as a literal and DNSConfig has no field for it.
//
// So the two agree today because two independent literals happen to read 53,
// which is precisely the shape docs/LAYOUT.md warns about for the other pairing.
// If they ever disagree the redirect DNATs every client query to a port nothing
// is bound to, and every joined device stops resolving while the hotspot, the
// tunnel and the firewall all look healthy.
//
// This asserts on the bytes the service put on the wire, so it fails for a
// drift introduced on either side.
func TestTheDNSRedirectLandsOnThePortDnsmasqBinds(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	loaded := rulesetsLoaded(w, 0)
	if len(loaded) == 0 {
		t.Fatalf("no nftables ruleset was loaded at all")
	}
	redirect := redirectPort(t, loaded[len(loaded)-1])

	conf, ok := w.sys.Files[w.cfg.HotspotPaths.DnsmasqConf]
	if !ok {
		t.Fatalf("no dnsmasq configuration was written")
	}
	bindStr := dnsmasqDirective(t, string(conf), "port")
	bind, err := strconv.Atoi(bindStr)
	if err != nil {
		t.Fatalf("dnsmasq was told to bind port %q, which is not a number", bindStr)
	}

	if redirect != bind {
		t.Errorf(
			"the firewall redirects client DNS to port %d and dnsmasq binds port %d. Every query from a "+
				"joined device would be DNATted to a port nothing is listening on, and resolution would "+
				"stop for every device while the hotspot, the tunnel and the firewall all reported healthy.\n"+
				"  the redirect target comes from netcfg.Options.DNSPort, which this service sets from "+
				"Config.DNSPort (currently %d)\n"+
				"  the bind port is a literal in internal/hotspot's RenderDnsmasq and is not configurable\n"+
				"If DNSPort is meant to be settable, internal/hotspot needs a matching field; until then it "+
				"has exactly one correct value.",
			redirect, bind, w.cfg.DNSPort)
	}

	// The same port has to be permitted INBOUND, or the redirect lands on a
	// socket the filter chain then drops. Three values, not two.
	want := "udp dport " + strconv.Itoa(redirect) + " accept"
	if !strings.Contains(loaded[len(loaded)-1], want) {
		t.Errorf("the input chain does not accept %q, so the redirected query is dropped after being "+
			"rewritten", want)
	}
}

// TestTheOnlyCorrectDNSPortIsThePortDnsmasqBinds ties the two literals together
// where both are visible, which is the only place they can be compared without
// starting anything.
//
// The test above proves the pair agree in the composition this service actually
// builds. It cannot prove they agree for a DNSPort somebody else passes in, and
// MEASURED on 2026-08-30 they do not: a service configured with DNSPort 5353
// starts without complaint, loads a firewall that redirects client DNS to 5353
// and permits 5353 inbound, and writes a dnsmasq configuration bound to 53.
// The firewall is self-consistent, the daemon is bound elsewhere, and every
// joined device stops resolving with nothing reporting an error. That defect is
// open; refusing the configuration needs a decision this test does not make.
//
// What this pins meanwhile is the contract as it stands: internal/hotspot emits
// its listening port as a literal and offers no field to change it, so
// internal/netcfg's default has exactly one correct value. Change either
// literal and this goes red naming the other.
func TestTheOnlyCorrectDNSPortIsThePortDnsmasqBinds(t *testing.T) {
	rendered := renderedDnsmasqPort(t)
	if got := netcfg.DefaultOptions().DNSPort; got != rendered {
		t.Errorf(
			"netcfg.DefaultOptions().DNSPort is %d and internal/hotspot renders dnsmasq on port %d.\n"+
				"These are two independent literals in two packages with no shared constant between them. "+
				"While they disagree, the prerouting redirect DNATs every client query to a port dnsmasq is "+
				"not bound to, and resolution stops for every joined device while the hotspot, the tunnel "+
				"and the firewall all report healthy.\n"+
				"internal/hotspot's port is not configurable, so it is the one that decides.",
			got, rendered)
	}
}

// TestClientDNSPortPairingIsDerivedRatherThanCoincidental is the 5354 pairing,
// the one docs/LAYOUT.md calls "the one that breaks quietly".
//
// It is deliberately NOT a second copy of
// TestTheHotspotForwardsDNSToThePortTheEngineListensOn. That test asserts the
// literal 5354 on both sides while the world's Config carries the literal 5354,
// so it passes whether the two consumers read Config.LocalDNSPort or hardcode
// the number. This one moves the port to a value that appears nowhere else in
// the tree, so it can only pass if BOTH halves are derived from the one field
// that plans.go says they are:
//
//	engineDocument   o.LocalDNS.Port = s.cfg.LocalDNSPort
//	hotspotPlanFor   Upstream: 127.0.0.1:s.cfg.LocalDNSPort
//
// A hardcoded 5354 on either side turns this red and leaves the existing test
// green, which is the whole reason to have it.
func TestClientDNSPortPairingIsDerivedRatherThanCoincidental(t *testing.T) {
	const oddPort = 15353

	w := newWorld(t, func(w *world) { w.cfg.LocalDNSPort = oddPort })
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	conf, ok := w.sys.Files[w.cfg.HotspotPaths.DnsmasqConf]
	if !ok {
		t.Fatalf("no dnsmasq configuration was written")
	}
	server := dnsmasqDirective(t, string(conf), "server")
	wantServer := "127.0.0.1#" + strconv.Itoa(oddPort)
	if server != wantServer {
		t.Errorf(
			"the service was configured with LocalDNSPort %d and dnsmasq was told to forward to %q, "+
				"want %q. The forwarding target is not derived from Config.LocalDNSPort, so the two ends "+
				"of the client DNS chain agree by coincidence and will drift the first time either moves.",
			oddPort, server, wantServer)
	}

	docs := w.eng.documents()
	if len(docs) != 1 {
		t.Fatalf("the engine was handed %d documents, want 1", len(docs))
	}
	doc := string(docs[0])
	wantListen := `"port": ` + strconv.Itoa(oddPort)
	if !strings.Contains(doc, wantListen) {
		t.Errorf(
			"the engine's document has no listener carrying %s. dnsmasq was told to forward client DNS to "+
				"port %d and nothing in the engine answers there, so every joined device would stop "+
				"resolving while the hotspot and the tunnel both looked healthy.",
			wantListen, oddPort)
	}

	// And the listener must still be on loopback. A listener that moved to a
	// wildcard while the port was being made configurable would be an open
	// resolver on whatever network the box is plugged into.
	if !strings.Contains(doc, `"listen": "127.0.0.1"`) {
		t.Errorf("the engine's local DNS listener is not bound to 127.0.0.1; on any other address this box " +
			"answers DNS for the network it is plugged into")
	}
}

// TestTheResolverPathDoesNotDependOnAPacketMarkNothingSets.
//
// internal/netcfg installs a policy rule "ip rule add fwmark <FwMark> lookup
// <table>", and the reason recorded beside it in TunnelRouteSteps says it is
// "how the resolver on the box resolves client queries through the tunnel
// instead of leaking them out of the uplink".
//
// Measured on 2026-08-30: nothing in this repository sets that mark. No
// generated ruleset contains a mark expression, the engine document carries no
// socket-mark option, and FwMark is read in exactly one place, which is the
// rule that CONSUMES it. The rule is therefore inert, and the sentence beside
// it describes a mechanism that does not exist.
//
// That is not a leak, because client DNS never takes the kernel route the mark
// would select: dnsmasq forwards to a loopback address, so the query reaches
// the engine over loopback and the engine sends it out through the proxy
// outbound under its own routing rule. The correction to the comment is in
// route.go; this is the tripwire that goes with it.
//
// It fails the day somebody starts setting a mark, which is the day the inert
// rule becomes load bearing and its comment has to be read again rather than
// trusted.
func TestTheResolverPathDoesNotDependOnAPacketMarkNothingSets(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	for i, rs := range rulesetsLoaded(w, 0) {
		for _, line := range strings.Split(rs, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "mark") {
				t.Errorf(
					"ruleset %d contains a mark expression: %q.\n"+
						"Something now sets or matches a packet mark, so the fwmark policy rule that "+
						"internal/netcfg installs is no longer inert. Re-read the reason recorded beside it "+
						"in TunnelRouteSteps before trusting it: as of 2026-08-30 that rule had no producer "+
						"anywhere in this repository and the client DNS path did not use it.",
					i, trimmed)
			}
		}
	}

	docs := w.eng.documents()
	if len(docs) != 1 {
		t.Fatalf("the engine was handed %d documents, want 1", len(docs))
	}
	if strings.Contains(string(docs[0]), "sockopt") || strings.Contains(string(docs[0]), `"mark"`) {
		t.Errorf("the engine's document now carries a socket option or a mark. The fwmark policy rule is " +
			"no longer inert and the reason recorded beside it has to be re-read")
	}

	// The rule itself is still expected, so that this test says what it means:
	// the mark has no producer, not that the rule was quietly removed.
	found := false
	for _, c := range w.runner.Commands() {
		if c.Path != netcfg.BinIP {
			continue
		}
		if len(c.Args) >= 3 && c.Args[0] == "rule" && c.Args[1] == "add" && c.Args[2] == "fwmark" {
			found = true
		}
	}
	if !found {
		t.Log("no fwmark policy rule was installed; if it was removed deliberately, this test and the " +
			"comment in netcfg.TunnelRouteSteps can both go")
	}
}

// TestAAAAQueriesAreAnsweredAndNotSuppressed pins a behaviour that no document
// described until it was measured, and that the panel's own wording invites a
// reader to get wrong.
//
// The panel tells users "IPv6 for your devices is always blocked". That is true
// of ROUTING: the forward chain drops client IPv6 in both directions, the box
// advertises no prefix, and nothing assigns a client a v6 address. It is NOT
// true of RESOLUTION. An AAAA query from a joined device is forwarded to the
// engine and answered with real AAAA records, because the engine document
// carries queryStrategy UseIP and dnsmasq sets no filter-AAAA.
//
// That is inert today. A client holding an AAAA record it cannot route simply
// falls back to IPv4. It stops being inert the moment anything gives a client a
// working v6 path, because RFC 6724 and Happy Eyeballs make a client PREFER the
// v6 answer, and it would then leave by a route this appliance does not carry.
//
// So this test does not assert that answering AAAA is correct. It asserts that
// the behaviour is what the documentation now says it is, in both halves of the
// chain, so that changing it is a decision somebody makes rather than a side
// effect somebody ships. If you change either half, change the documentation in
// the same commit: README.md, README.fa.md and docs/BEHAVIOUR.md all describe
// this.
func TestAAAAQueriesAreAnsweredAndNotSuppressed(t *testing.T) {
	// Half one: dnsmasq does not filter AAAA out on the way past.
	conf, err := hotspot.RenderDnsmasq(hotspot.DNSConfig{
		Interface:    "ap0",
		Subnet:       netip.MustParsePrefix("10.83.51.0/24"),
		Gateway:      netip.MustParseAddr("10.83.51.1"),
		RangeStart:   netip.MustParseAddr("10.83.51.50"),
		RangeEnd:     netip.MustParseAddr("10.83.51.200"),
		LeaseTime:    12 * time.Hour,
		LeaseFile:    "/var/lib/caspian/dnsmasq.leases",
		Upstream:     netip.MustParseAddrPort("127.0.0.1:5354"),
		CacheSize:    150,
		ServiceUser:  hotspot.DefaultServiceUser,
		ServiceGroup: hotspot.DefaultServiceGroup,
	})
	if err != nil {
		t.Fatalf("rendering a dnsmasq configuration: %v", err)
	}
	if strings.Contains(conf, "filter-AAAA") {
		t.Error("the generated dnsmasq configuration now sets filter-AAAA. That changes " +
			"what a client gets back for an AAAA query, so it is a behaviour change and " +
			"not a tidy-up. Note also that filter-AAAA is a dnsmasq 2.81 addition and an " +
			"unknown option is fatal, so an older dnsmasq would refuse to start and the " +
			"hotspot would not come up at all.")
	}

	// Half two: the engine is told to resolve every family.
	l, err := link.Parse("vless://b7f8c2a1-4d3e-4f5a-9b8c-1d2e3f4a5b6c@front.invalid:443" +
		"?type=ws&security=tls&sni=front.invalid&host=front.invalid&path=%2Fw#box")
	if err != nil {
		t.Fatalf("parsing the probe link: %v", err)
	}
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = true
	o.LocalDNS.Enabled = true
	o.LocalDNS.Port = 5354
	doc, err := xcfg.Build(o)
	if err != nil {
		t.Fatalf("building the engine document: %v", err)
	}
	if !strings.Contains(string(doc), `"queryStrategy": "UseIP"`) &&
		!strings.Contains(string(doc), `"queryStrategy":"UseIP"`) {
		t.Error("the engine document no longer asks for UseIP. If it was changed to " +
			"UseIPv4 then AAAA queries now come back empty, which is a user-visible " +
			"change and the opposite of what the documentation describes. Update the " +
			"documentation in the same commit, or put this back.")
	}
}
