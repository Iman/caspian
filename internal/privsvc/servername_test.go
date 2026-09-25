// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// resolverAnswering is a Resolver that answers with a fixed list, for the
// shapes fakeResolver does not model: several addresses, and both families.
type resolverAnswering []netip.Addr

func (r resolverAnswering) Resolve(context.Context, string) ([]netip.Addr, error) {
	return append([]netip.Addr(nil), r...), nil
}

// startedDocument runs a real Start in the recorded world and returns the one
// document the engine was handed, decoded far enough to read the two things
// this file is about.
type startedDocument struct {
	raw   []byte
	hosts map[string][]string
	// sockoptStrategy is the proxy outbound's
	// streamSettings.sockopt.domainStrategy, "" when absent.
	sockoptStrategy string
	// pinned is what the applied plan pins host routes to.
	pinned []netip.Addr
}

func startAndReadDocument(t *testing.T, w *world, req panel.StartRequest) startedDocument {
	t.Helper()
	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("start: %v", err)
	}
	docs := w.eng.documents()
	if len(docs) != 1 {
		t.Fatalf("the engine was handed %d documents, want 1", len(docs))
	}
	var d struct {
		DNS struct {
			Hosts map[string][]string `json:"hosts"`
		} `json:"dns"`
		Outbounds []struct {
			Tag            string `json:"tag"`
			StreamSettings struct {
				Sockopt struct {
					DomainStrategy string `json:"domainStrategy"`
				} `json:"sockopt"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(docs[0], &d); err != nil {
		t.Fatalf("decoding the engine document: %v", err)
	}
	out := startedDocument{raw: docs[0], hosts: d.DNS.Hosts}
	for _, ob := range d.Outbounds {
		if ob.Tag == xcfg.TagProxy {
			out.sockoptStrategy = ob.StreamSettings.Sockopt.DomainStrategy
		}
	}
	w.svc.mu.RLock()
	plan := w.svc.plan
	w.svc.mu.RUnlock()
	if plan == nil {
		t.Fatal("no plan was applied")
	}
	out.pinned = plan.PinnedServers()
	return out
}

func requestFor(t *testing.T, shareLink string) panel.StartRequest {
	t.Helper()
	req := startRequest(t)
	l, err := link.Parse(shareLink)
	if err != nil {
		t.Fatalf("parsing the share link: %v", err)
	}
	doc, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("building the config document: %v", err)
	}
	req.ConfigJSON = doc
	return req
}

// TestTheEngineDocumentPinsTheServerNameToThePlannedAddresses is the guard on
// the privileged side of GitHub issue 7.
//
// The engine must be told the addresses this service resolved before the
// tunnel existed, and exactly the ones the plan pins a host route to. Without
// that, the engine looks the server's name up at every dial through the
// machine's own resolver, which on Windows is only reachable through the
// tunnel the dial is trying to build. test/tunnel/servername_test.go carries
// real traffic over this document shape with a resolver that cannot answer.
func TestTheEngineDocumentPinsTheServerNameToThePlannedAddresses(t *testing.T) {
	w := newWorld(t)
	got := startAndReadDocument(t, w, startRequest(t))

	if len(got.pinned) == 0 {
		t.Fatal("the plan pinned no address; this test would pass vacuously")
	}
	want := make([]string, 0, len(got.pinned))
	for _, a := range got.pinned {
		want = append(want, a.String())
		// The route is what makes the address reachable outside the tunnel;
		// the engine document is only correct if it names what was routed.
		if w.tl.indexOf("ip route add "+netip.PrefixFrom(a, 32).String()) < 0 {
			t.Errorf("the plan says %s is pinned but no host route to it was applied", a)
		}
	}
	key := "full:" + fakeHost
	if !slices.Equal(got.hosts[key], want) {
		t.Fatalf("dns.hosts[%q] = %v, want the pinned addresses %v", key, got.hosts[key], want)
	}
	if len(got.hosts) != 1 {
		t.Errorf("dns.hosts carries %d entries, want only the server name", len(got.hosts))
	}
	if got.sockoptStrategy != "ForceIP" {
		t.Fatalf("the proxy outbound's sockopt.domainStrategy is %q; without ForceIP the engine never consults "+
			"dns.hosts and resolves the server name through the system resolver at every dial", got.sockoptStrategy)
	}
	// The name still travels everywhere a name belongs. REALITY carries its
	// own server name; the address field is what the transports derive the
	// rest from, and it must still be the domain.
	// xcfg.Build indents the document, hence the space after the colon.
	if !bytes.Contains(got.raw, []byte(`"address": "`+fakeHost+`"`)) {
		t.Error("the proxy outbound no longer names the server by its domain")
	}
	if err := engine.Validate(got.raw); err != nil {
		t.Fatalf("the engine refused the document: %v", err)
	}
}

// TestOnlyPinnedAddressesReachTheEngine resolves to two IPv4 addresses and one
// IPv6 address, on a machine with an IPv6 default route and on one without.
// The set handed to the engine must be the plan's pinned set, in the plan's
// order, and not the resolver's raw answer: without an IPv6 route the IPv6
// address gets no pinned route (netcfg.Plan.canPin) and must not be dialled.
func TestOnlyPinnedAddressesReachTheEngine(t *testing.T) {
	v4 := netip.MustParseAddr("203.0.113.7")
	v4b := netip.MustParseAddr("198.51.100.9")
	v6 := netip.MustParseAddr("2001:db8::7")
	for _, c := range []struct {
		name    string
		v6Route bool
	}{
		{"with-ipv6-default-route", true},
		{"without-ipv6-default-route", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t, func(w *world) {
				w.cfg.Resolver = resolverAnswering{v4, v6, v4b}
				if !c.v6Route {
					w.runner.SetOutput("ip -6 route show default", "")
				}
			})
			got := startAndReadDocument(t, w, startRequest(t))

			var want []string
			for _, a := range got.pinned {
				want = append(want, a.String())
			}
			hosts := got.hosts["full:"+fakeHost]
			if !slices.Equal(hosts, want) {
				t.Fatalf("dns.hosts carries %v, but the plan pins %v", hosts, want)
			}
			if !slices.Contains(hosts, v4b.String()) {
				t.Fatalf("the second IPv4 address is missing, so several addresses are not exercised: %v", hosts)
			}
			if slices.Contains(hosts, v6.String()) != c.v6Route {
				t.Fatalf("IPv6 present in dns.hosts is %v, want %v: %v", !c.v6Route, c.v6Route, hosts)
			}
		})
	}
}

// TestAnIPLiteralLinkLeavesTheEngineDocumentAsItWas holds the other half of
// the change: a link that already names its server by address gets no hosts
// entry, no sockopt, and the document is byte-identical to the one built with
// no pinned addresses at all.
func TestAnIPLiteralLinkLeavesTheEngineDocumentAsItWas(t *testing.T) {
	ipLink := strings.Replace(realityShareLink(), "@"+fakeHost+":", "@203.0.113.7:", 1)
	w := newWorld(t)
	got := startAndReadDocument(t, w, requestFor(t, ipLink))

	if got.hosts != nil {
		t.Errorf("an IP-literal link produced dns.hosts %v", got.hosts)
	}
	if bytes.Contains(got.raw, []byte("sockopt")) {
		t.Error("an IP-literal link produced a sockopt section")
	}

	l, err := link.Parse(ipLink)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// The same tunnel subnet the start used, so that the pinned addresses are
	// the only input that differs between the two documents.
	w.svc.mu.RLock()
	tunSubnet := w.svc.plan.TunSubnet
	w.svc.mu.RUnlock()
	unpinned, err := w.svc.engineDocument(l, requestFor(t, ipLink), w.cfg.netOptions(), nil, tunSubnet)
	if err != nil {
		t.Fatalf("composing without pinned addresses: %v", err)
	}
	if !bytes.Equal(got.raw, unpinned) {
		t.Fatalf("the IP-literal document changed when pinned addresses were supplied:\nwith:\n%s\nwithout:\n%s",
			got.raw, unpinned)
	}
}
