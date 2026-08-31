// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// The client DNS chain as a property of the whole appliance.
//
// The scenario already in behaviour_test.go, "a client cannot reach a resolver
// of its own choosing", is about the FIREWALL: it proves a device cannot
// address a resolver other than this box. It stops at dnsmasq's configuration
// text and says nothing about what happens to the query after dnsmasq accepts
// it.
//
// These two cover the rest of the chain, which is where a leak would actually
// have to occur:
//
//	device -> dnsmasq -> the engine's loopback listener -> the proxy outbound
//
// They became possible on 2026-08-30, when this suite started enabling the
// engine's local DNS listener and giving both halves of the chain the same
// port. Before that the World composed dnsmasq with a forwarding target of
// 127.0.0.1:15353 and built an engine document with no listener at all, so the
// appliance's actual client DNS path was not modelled here and could not be.

// ---------------------------------------------------------------------------
// Steps
// ---------------------------------------------------------------------------

// theHotspotForwardsClientDNSToTheEnginesOwnListener is the pairing
// docs/LAYOUT.md calls "the one that breaks quietly", asserted across the two
// artefacts the appliance produced rather than against a number written here.
//
// Both sides are read out of what was generated: the address and port dnsmasq
// was told to forward to, and the address and port the engine was told to
// listen on. Neither is compared against a literal, so this cannot pass by a
// fixture agreeing with itself.
func theHotspotForwardsClientDNSToTheEnginesOwnListener(w *World) error {
	conf := w.dnsmasqConf()

	var server string
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "server=") {
			if server != "" {
				return fmt.Errorf("the DHCP and DNS server has more than one forwarding target, so where a "+
					"client query goes is not decided by this configuration: %q and %q", server, line)
			}
			server = strings.TrimPrefix(line, "server=")
		}
	}
	if server == "" {
		return errors.New("the DHCP and DNS server was given no forwarding target at all, so a client query " +
			"is answered from cache or not at all")
	}

	addr, portStr, ok := strings.Cut(server, "#")
	if !ok {
		return fmt.Errorf("the forwarding target %q names no port, so it would go to 53 on whatever "+
			"address that is", server)
	}
	upstream, err := netip.ParseAddr(addr)
	if err != nil {
		return fmt.Errorf("the forwarding target %q is not an IP address; on this box it must be one, "+
			"because a name would be resolved by the resolver it is meant to configure", addr)
	}
	if !upstream.IsLoopback() {
		return fmt.Errorf("client DNS is forwarded to %s, which is not on this box. Every name every "+
			"joined device looks up would leave in plaintext beside the tunnel", upstream)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("the forwarding target names port %q, which is not a number", portStr)
	}

	// The other end. The engine's document has to carry a listener on exactly
	// that address and port, or dnsmasq is talking to nothing.
	doc, err := w.engineDocument()
	if err != nil {
		return err
	}
	for _, in := range doc.Inbounds {
		if in.Protocol != "dokodemo-door" {
			continue
		}
		if in.Listen == upstream.String() && int(in.Port) == port {
			return nil
		}
	}
	return fmt.Errorf(
		"the DHCP and DNS server forwards client queries to %s:%d and the engine has no listener there. "+
			"Every joined device would stop resolving while the hotspot, the tunnel and the firewall all "+
			"reported healthy, which is the failure docs/LAYOUT.md calls the one that breaks quietly.\n"+
			"  inbounds in the engine's document: %s",
		upstream, port, doc.inboundSummary())
}

// clientQueriesLeaveOnlyByTheTunnel is the half that decides whether the chain
// is a leak or a tunnel.
//
// A loopback listener is not by itself a guarantee of anything: it decides only
// that the query reaches the engine. What the query does next is a routing
// decision inside the engine's document, and the two rules that make it are
// ordered on purpose. Both are asserted, and so is their order.
func clientQueriesLeaveOnlyByTheTunnel(w *World) error {
	doc, err := w.engineDocument()
	if err != nil {
		return err
	}

	// The tags are written out rather than imported from internal/xcfg. A test
	// that borrows the constants it is checking cannot catch those constants
	// being changed to something no rule matches.
	const (
		localDNSRule = "local-dns-to-tunnel"
		resolverRule = "resolver-through-tunnel"
		privateRule  = "private-direct"
		proxyOut     = "proxy"
		dnsOut       = "dns-out"
	)

	at := map[string]int{}
	for i, r := range doc.Routing.Rules {
		at[r.RuleTag] = i
	}

	local, ok := at[localDNSRule]
	if !ok {
		return fmt.Errorf("the engine's document has no %q rule, so a query arriving on the loopback "+
			"listener is routed by whatever matches it next\n  rules: %s", localDNSRule, doc.ruleSummary())
	}
	if got := doc.Routing.Rules[local].OutboundTag; got != dnsOut {
		return fmt.Errorf("queries arriving on this box's DNS listener are handed to the %q outbound, "+
			"want %q", got, dnsOut)
	}

	resolver, ok := at[resolverRule]
	if !ok {
		return fmt.Errorf("the engine's document has no %q rule, so the resolver's OWN upstream queries "+
			"are routed by whatever matches them next\n  rules: %s", resolverRule, doc.ruleSummary())
	}
	if got := doc.Routing.Rules[resolver].OutboundTag; got != proxyOut {
		return fmt.Errorf(
			"the resolver's upstream queries are handed to the %q outbound, want %q. Every name every "+
				"joined device looks up would be resolved from this box's own network instead of from the "+
				"exit server, which is a leak of exactly the information the appliance exists to protect, "+
				"and it would leave no trace in the firewall because the query never reaches it.",
			got, proxyOut)
	}

	// Order. The private rule matches on destination ADDRESS, and a resolver on
	// a private address is a legitimate configuration when it is reachable only
	// through the proxy. If it came first it would match and send the query
	// direct.
	if priv, ok := at[privateRule]; ok {
		if resolver > priv {
			return fmt.Errorf(
				"the %q rule is at position %d and the %q rule at %d. Rules are evaluated in order, so a "+
					"resolver on a private address would be matched by the private rule and resolved on the "+
					"local network instead of through the tunnel",
				resolverRule, resolver, privateRule, priv)
		}
		if local > priv {
			return fmt.Errorf(
				"the %q rule is at position %d and the %q rule at %d, so a query arriving on the loopback "+
					"listener can be matched by destination before it is matched by where it came from",
				localDNSRule, local, privateRule, priv)
		}
	}
	return nil
}

// theOnlyResolverOfferedToJoiningDevicesIsThisBox covers the offer rather than
// the permission.
//
// The firewall scenario covers a device that IGNORES what it was offered. This
// covers the device that honours it, which is nearly all of them. A DHCP offer
// naming a public resolver would be rewritten by the prerouting redirect and so
// would never show up as a leak on the wire, while every honest device on the
// hotspot addressed its queries to a stranger and this box quietly answered on
// that stranger's behalf for ever.
func theOnlyResolverOfferedToJoiningDevicesIsThisBox(w *World) error {
	conf := w.dnsmasqConf()
	gateway := w.plan.HotspotGateway.String()

	var offered []string
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "dhcp-option=option:dns-server,") {
			offered = append(offered, strings.TrimPrefix(line, "dhcp-option=option:dns-server,"))
		}
	}

	switch len(offered) {
	case 0:
		return errors.New("joining devices are offered no resolver at all, so each one falls back to " +
			"whatever it has cached or compiled in and the redirect is the only thing left")
	case 1:
	default:
		return fmt.Errorf("joining devices are offered %d resolvers (%v); which one a device picks is not "+
			"a property this configuration states", len(offered), offered)
	}

	if offered[0] != gateway {
		return fmt.Errorf(
			"joining devices are told to use resolver %s, and this box is %s. Every device that honours "+
				"DHCP would address a stranger; the redirect would rewrite the packets, so nothing on the "+
				"wire would look wrong while the offer was",
			offered[0], gateway)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reading the engine's document
// ---------------------------------------------------------------------------

// engineDoc is the subset of the generated document these steps read. It is
// declared here rather than imported from internal/xcfg, which declares its
// shape unexported, and reading it back through encoding/json is what makes
// these assertions about the DOCUMENT rather than about the struct that
// produced it.
type engineDoc struct {
	Inbounds []struct {
		Tag      string `json:"tag"`
		Protocol string `json:"protocol"`
		Listen   string `json:"listen"`
		Port     uint16 `json:"port"`
	} `json:"inbounds"`
	Routing struct {
		Rules []struct {
			RuleTag     string `json:"ruleTag"`
			OutboundTag string `json:"outboundTag"`
		} `json:"rules"`
	} `json:"routing"`
}

func (d engineDoc) inboundSummary() string {
	var out []string
	for _, in := range d.Inbounds {
		out = append(out, fmt.Sprintf("%s %s %s:%d", in.Tag, in.Protocol, in.Listen, in.Port))
	}
	return strings.Join(out, ", ")
}

func (d engineDoc) ruleSummary() string {
	var out []string
	for _, r := range d.Routing.Rules {
		out = append(out, r.RuleTag+"->"+r.OutboundTag)
	}
	return strings.Join(out, ", ")
}

// engineDocument parses the configuration the appliance handed the engine, with
// any deliberate defect already applied.
func (w *World) engineDocument() (engineDoc, error) {
	var d engineDoc
	if len(w.engineCfg) == 0 {
		return d, errors.New("no engine configuration was produced")
	}
	if err := json.Unmarshal(w.engineCfg, &d); err != nil {
		return d, fmt.Errorf("the engine's configuration could not be read back: %w", err)
	}
	return d, nil
}

// dnsmasqConf is the generated DHCP and DNS configuration, with any deliberate
// defect applied.
func (w *World) dnsmasqConf() string {
	return w.defs.mutateDnsmasq(w.hotspotPlan.DnsmasqConf)
}

// ---------------------------------------------------------------------------
// The deliberate defects for the scenarios below
// ---------------------------------------------------------------------------

// resolveClientQueriesDirectInsteadOfThroughTheTunnel is the change somebody
// makes when resolution is slow and the tunnel is blamed. It reads as a
// performance fix. It sends every client's every lookup out of the box's own
// network in plaintext, and no firewall rule can see it, because the query
// never reaches the firewall as a client packet.
func resolveClientQueriesDirectInsteadOfThroughTheTunnel(cfg []byte) []byte {
	return []byte(strings.Replace(string(cfg),
		"\"ruleTag\": \"resolver-through-tunnel\",\n        \"inboundTag\": [\n          \"resolver-in\"\n        ],\n        \"outboundTag\": \"proxy\"",
		"\"ruleTag\": \"resolver-through-tunnel\",\n        \"inboundTag\": [\n          \"resolver-in\"\n        ],\n        \"outboundTag\": \"direct\"",
		1))
}

// offerAPublicResolverToJoiningDevices is the change somebody makes when a
// device on the hotspot resolves slowly, or when a well-meaning edit "adds a
// fallback". It is invisible on the wire because the redirect rewrites it.
func offerAPublicResolverToJoiningDevices(conf string) string {
	var out []string
	for _, line := range strings.Split(conf, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "dhcp-option=option:dns-server,") {
			out = append(out, "dhcp-option=option:dns-server,1.1.1.3")
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
