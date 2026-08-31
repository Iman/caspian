// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestLocalDNSQueriesCannotFallOutToTheUplink is the same property asserted for
// the resolver rule, applied to the loopback listener: a query that arrives on
// it goes through the tunnel and can reach the network by no other path.
//
// Three things have to hold together, and any one of them alone is not enough:
//
//  1. The listener's rule matches on inboundTag and targets the dns outbound.
//  2. That rule is ABOVE every rule that could answer locally, which in this
//     document means above the private rule. The listener stamps a fixed
//     destination on each query (proxy/dns/dns.go:115-124 keeps it), and if
//     that destination were private and the private rule came first, the query
//     would go to the freedom outbound and be resolved on the local network.
//  3. The answers come from the built-in resolver, whose own upstream queries
//     are pinned to the proxy by the resolver rule. Checked here so the chain
//     is asserted end to end rather than one link at a time.
func TestLocalDNSQueriesCannotFallOutToTheUplink(t *testing.T) {
	l := mustParse(t, vlessRealityLink())

	for _, c := range combinations() {
		o := c.opts
		o.Link = l
		raw, err := Build(o)
		if err != nil {
			t.Fatalf("%s: Build: %v", c.name, err)
		}
		p := decode(t, raw)

		localIdx, resolverIdx, privateIdx := -1, -1, -1
		for i, r := range p.Routing.Rules {
			switch r.RuleTag {
			case ruleTagLocalDNS:
				localIdx = i
			case ruleTagResolvers:
				resolverIdx = i
			case ruleTagPrivate:
				privateIdx = i
			}
		}

		if !o.LocalDNS.Enabled {
			if localIdx >= 0 {
				t.Errorf("%s: a local DNS rule was emitted with LocalDNS.Enabled unset", c.name)
			}
			for _, in := range p.Inbounds {
				if in.Tag == TagLocalDNSIn {
					t.Errorf("%s: a local DNS inbound was emitted with LocalDNS.Enabled unset", c.name)
				}
			}
			continue
		}

		if localIdx < 0 {
			t.Fatalf("%s: LocalDNS.Enabled is set but no %q rule was emitted", c.name, ruleTagLocalDNS)
		}
		r := p.Routing.Rules[localIdx]
		if len(r.InboundTag) != 1 || r.InboundTag[0] != TagLocalDNSIn {
			t.Errorf("%s: the local DNS rule matches inboundTag %v, want [%s]", c.name, r.InboundTag, TagLocalDNSIn)
		}
		if r.OutboundTag != TagDNSOut {
			t.Errorf("%s: the local DNS rule sends queries to %q, want %q", c.name, r.OutboundTag, TagDNSOut)
		}
		if privateIdx >= 0 && localIdx > privateIdx {
			t.Errorf("%s: the local DNS rule is at %d and the private rule at %d; that order lets a query whose stamped destination is private be resolved on the local network",
				c.name, localIdx, privateIdx)
		}
		if resolverIdx < 0 {
			t.Fatalf("%s: no resolver rule, so the answers have no pinned path", c.name)
		}
		if privateIdx >= 0 && resolverIdx > privateIdx {
			t.Errorf("%s: the resolver rule is at %d and the private rule at %d; a resolver on a private address would then be queried outside the tunnel",
				c.name, resolverIdx, privateIdx)
		}
		if p.Routing.Rules[resolverIdx].OutboundTag != TagProxy {
			t.Errorf("%s: resolver queries go to %q, want %q", c.name, p.Routing.Rules[resolverIdx].OutboundTag, TagProxy)
		}

		// The dns outbound must exist, and it must reject rather than forward
		// what the built-in resolver cannot answer. proxy/dns/dns.go:229-231
		// dials the stamped destination for anything that reaches it, and
		// "reject" at :219-226 is what stops anything reaching it.
		foundDNSOut := false
		for _, ob := range p.Outbounds {
			if ob.Tag != TagDNSOut {
				continue
			}
			foundDNSOut = true
			var s struct {
				NonIPQuery string `json:"nonIPQuery"`
			}
			if err := json.Unmarshal(ob.Settings, &s); err != nil {
				t.Fatalf("%s: dns outbound settings: %v", c.name, err)
			}
			if s.NonIPQuery != "reject" {
				t.Errorf("%s: the dns outbound has nonIPQuery %q; anything but reject or drop forwards a non-A/AAAA query to the stamped destination",
					c.name, s.NonIPQuery)
			}
		}
		if !foundDNSOut {
			t.Fatalf("%s: LocalDNS.Enabled is set but there is no %q outbound to answer with", c.name, TagDNSOut)
		}
	}
}

// TestLocalDNSListenerCarriesUDP is a small check with a large consequence.
//
// DNS is mostly UDP. DokodemoConfig.Build calls v.Network.Build() at
// infra/conf/dokodemo.go:31, and NetworkList.Build returns TCP ONLY for a nil
// receiver (infra/conf/common.go:110-112). A listener that omitted "network"
// would build, validate, start, bind, and then ignore every ordinary query
// while answering the TCP retry, which is close to the worst failure shape
// available: intermittent, and it looks like a network problem.
func TestLocalDNSListenerCarriesUDP(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	raw, err := Build(Options{Link: l, LocalDNS: LocalDNS{Enabled: true}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	p := decode(t, raw)

	found := false
	for _, in := range p.Inbounds {
		if in.Tag != TagLocalDNSIn {
			continue
		}
		found = true
		if in.Protocol != "dokodemo-door" {
			t.Errorf("the local DNS inbound is protocol %q, want dokodemo-door", in.Protocol)
		}
		var s struct {
			Address string `json:"address"`
			Port    uint16 `json:"port"`
			Network string `json:"network"`
		}
		if err := json.Unmarshal(in.Settings, &s); err != nil {
			t.Fatalf("local DNS settings: %v", err)
		}
		if !strings.Contains(s.Network, "udp") {
			t.Errorf("the local DNS listener's network is %q, which does not carry UDP; a nil network means TCP only", s.Network)
		}
		if !strings.Contains(s.Network, "tcp") {
			t.Errorf("the local DNS listener's network is %q, which does not carry TCP; a truncated answer retries over TCP", s.Network)
		}
		if s.Port != 53 {
			t.Errorf("the local DNS listener stamps destination port %d, want 53", s.Port)
		}
		if s.Address != DefaultResolvers()[0] {
			t.Errorf("the local DNS listener stamps destination %q, want the first configured resolver %q",
				s.Address, DefaultResolvers()[0])
		}
		if in.Listen != DefaultLocalDNSListen || in.Port != DefaultLocalDNSPort {
			t.Errorf("the local DNS listener binds %s:%d, want the defaults %s:%d",
				in.Listen, in.Port, DefaultLocalDNSListen, DefaultLocalDNSPort)
		}
	}
	if !found {
		t.Fatal("no local DNS inbound was emitted")
	}
}

// TestLocalDNSDefaultMatchesTheHotspotUpstream is the cross-package contract.
//
// internal/hotspot does not define a constant for this: Config.Upstream comes
// from its caller (internal/hotspot/dnsmasq.go:75-78) and the value appears
// only in that package's test fixture and the golden it generates. So the two
// halves of the client-DNS chain agree by way of a test file, which is an
// agreement that breaks without anything going red.
//
// This reads the hotspot golden and compares the address and port it forwards
// to against the defaults here. It does not decide which package should own
// the constant; it makes the divergence visible the day it happens.
func TestLocalDNSDefaultMatchesTheHotspotUpstream(t *testing.T) {
	const path = "../hotspot/testdata/dnsmasq.golden"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read %s (%v); the hotspot package may have moved, in which case this contract needs rechecking by hand", path, err)
	}

	// dnsmasq spells a forwarding target "server=<addr>#<port>", which is what
	// internal/hotspot/dnsmasq.go:289 emits.
	re := regexp.MustCompile(`(?m)^server=([^#\s]+)#(\d+)\s*$`)
	m := re.FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("%s has no server=<addr>#<port> line; internal/hotspot may have changed how it names its upstream, and this contract must be rechecked", path)
	}
	addr, portStr := m[1], m[2]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("%s: unparsable port %q", path, portStr)
	}

	if addr != DefaultLocalDNSListen {
		t.Errorf("internal/hotspot forwards client DNS to %s, but DefaultLocalDNSListen is %s. "+
			"dnsmasq would be talking to an address nothing in this program listens on",
			addr, DefaultLocalDNSListen)
	}
	if port != DefaultLocalDNSPort {
		t.Errorf("internal/hotspot forwards client DNS to port %d, but DefaultLocalDNSPort is %d. "+
			"dnsmasq would be talking to a port nothing in this program listens on",
			port, DefaultLocalDNSPort)
	}
}

// TestInboundCollisionIsRejected covers the failure engine.Validate cannot
// see. Two inbounds on one address and port build cleanly and fail at Start
// with a bind error, after the panel has told the user the config was fine.
func TestInboundCollisionIsRejected(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	cases := []struct {
		name string
		o    Options
	}{
		{"same v4 literal", Options{
			Link:     l,
			SOCKS:    SOCKS{Listen: "127.0.0.1", Port: 5354},
			LocalDNS: LocalDNS{Enabled: true, Listen: "127.0.0.1", Port: 5354},
		}},
		{"same address spelled differently", Options{
			Link:     l,
			SOCKS:    SOCKS{Listen: "::1", Port: 5354},
			LocalDNS: LocalDNS{Enabled: true, Listen: "0:0:0:0:0:0:0:1", Port: 5354},
		}},
		{"socks on the local DNS default", Options{
			Link:     l,
			SOCKS:    SOCKS{Listen: DefaultLocalDNSListen, Port: DefaultLocalDNSPort},
			LocalDNS: LocalDNS{Enabled: true},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Build(c.o); !errors.Is(err, ErrInboundCollision) {
				t.Fatalf("Build accepted two inbounds on one address and port, or failed for the wrong reason: %v", err)
			}
		})
	}

	// Different ports on the same address must still be fine, or the check is
	// too aggressive and would block the ordinary configuration.
	ok := Options{
		Link:     l,
		SOCKS:    SOCKS{Listen: "127.0.0.1", Port: 10808},
		LocalDNS: LocalDNS{Enabled: true, Listen: "127.0.0.1", Port: 5354},
	}
	if _, err := Build(ok); err != nil {
		t.Fatalf("Build rejected two inbounds on different ports: %v", err)
	}
	// And the collision must not be reported when the listener is off, since
	// then only one of the two inbounds exists.
	off := Options{
		Link:     l,
		SOCKS:    SOCKS{Listen: "127.0.0.1", Port: 5354},
		LocalDNS: LocalDNS{Listen: "127.0.0.1", Port: 5354},
	}
	if _, err := Build(off); err != nil {
		t.Fatalf("Build reported a collision with the local DNS listener disabled: %v", err)
	}
}

// TestLocalDNSListenIsNotAWildcard mirrors the SOCKS check on the listener's
// own validation path. A DNS listener on a non-loopback address is an open
// resolver on whatever network the box is plugged into, which is a sharper
// consequence than the SOCKS case and the same rule.
func TestLocalDNSListenIsNotAWildcard(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	bad := []string{
		"0.0.0.0",
		"::",
		"::ffff:0.0.0.0",
		"0000:0000:0000:0000:0000:0000:0000:0000",
		"localhost",
		"192.168.66.1", // the hotspot address: plausible, and wrong
		"0.0.0.0.0",
		"127.0.0.1:5354",
	}
	for _, listen := range bad {
		o := Options{Link: l, LocalDNS: LocalDNS{Enabled: true, Listen: listen}}
		if _, err := Build(o); !errors.Is(err, ErrLocalDNSAddress) {
			t.Errorf("Build accepted a local DNS listener on %q, or failed for the wrong reason: %v", listen, err)
		}
	}
	// Empty must normalise to the loopback default rather than being rejected.
	if _, err := Build(Options{Link: l, LocalDNS: LocalDNS{Enabled: true}}); err != nil {
		t.Errorf("an empty local DNS listen address should normalise to the default, got %v", err)
	}
	// A disabled listener with a bad address is not an error, because nothing
	// is emitted for it.
	if _, err := Build(Options{Link: l, LocalDNS: LocalDNS{Listen: "0.0.0.0"}}); err != nil {
		t.Errorf("a disabled listener with a wildcard address should not fail the build, got %v", err)
	}
}
