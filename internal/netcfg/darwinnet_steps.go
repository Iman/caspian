// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

const (
	// darwinAnchor is the pf anchor the generated rules are loaded into.
	//
	// It is a child of "com.apple/" on purpose. /etc/pf.conf evaluates
	// anchor "com.apple/*" (and the nat, rdr and scrub variants of it), so a
	// child anchor is enforced without editing the main ruleset. Editing the
	// main ruleset is exactly what must not happen: Internet Sharing inserts
	// its own anchor into the main ruleset dynamically, and reloading
	// /etc/pf.conf would throw its rules away. The number puts the anchor
	// after Apple's 200.AirDrop and before its firewall.
	darwinAnchor = "com.apple/250.CaspianBYOC"

	// darwinSharingBridge is the interface Internet Sharing gives the hotspot
	// clients. The radio (the plan's Hotspot) is a member of it; the address
	// and the packets the filter sees are on the bridge.
	darwinSharingBridge = "bridge100"

	// darwinTunPeer is the far end of the utun as xray-core creates it:
	// proxy/tun/tun_darwin.go, "gateway = 169.254.10.1/30", assigned by the
	// engine itself, which is why this backend has no tunnel-address step.
	darwinTunPeer = "169.254.10.1"

	// OpPf is the journal op for pf changes, as OpNft is for nftables.
	OpPf = "pf"
)

// darwinPreEngineSteps mirrors the Linux order: the firewall first, so there
// is never a moment when forwarding is on and the block is not; knobs next;
// the pinned host route last and still before the engine, so its first
// connection to the server is already outside the tunnel.
//
// There is no virtual-interface step and no release step. Internet Sharing
// creates the access point interface and the bridge itself, and the readback
// darwinAssertHotspotInterfaceReleased checks the bridge rather than the
// radio, for the reason given there.
func (p *Plan) darwinPreEngineSteps(current map[string]string) []Step {
	var steps []Step
	steps = append(steps, p.darwinPfEnableSteps(current)...)
	steps = append(steps, p.darwinFirewallStep())
	steps = append(steps, p.darwinSysctlSteps(current)...)
	steps = append(steps, p.darwinServerRouteSteps()...)
	return steps
}

// darwinPostEngineSteps names the tunnel device, so it runs after the engine
// created it. Under StrategyPolicy there is nothing to do: the pf rule that
// steers client traffic into the utun was loaded with the firewall and pf
// resolves the interface when the packet arrives. Under StrategySplitDefault
// the two half-default routes are installed here.
func (p *Plan) darwinPostEngineSteps(map[string]string) []Step {
	if p.Opts.Strategy != StrategySplitDefault {
		return nil
	}
	var steps []Step
	for _, half := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		why := "send the machine's own traffic through the tunnel as well as the clients'; " +
			"two halves rather than a default so the uplink's default route stays for the pinned server route"
		steps = append(steps, Step{
			Op:   OpRoute,
			Why:  why,
			Do:   Command{Path: BinRoute, Args: []string{"-n", "add", "-net", half, "-interface", p.Tun}, Why: why},
			Undo: Command{Path: BinRoute, Args: []string{"-n", "delete", "-net", half, "-interface", p.Tun}, Why: "remove the half-default route"},
		})
	}
	return steps
}

// darwinPfEnableSteps switches pf on when detection read it as off.
//
// An anchor loaded into a disabled pf enforces nothing and reports success,
// which is the false green this project has a rule against.
//
// This is the one step in the appliance with NO inverse, and the reason is
// written here rather than hidden. pf on macOS is reference counted: Internet
// Sharing, the application firewall and pfd each hold a reference through
// PacketFilter.framework, and "pfctl -d" ignores every one of them and
// switches pf off underneath whichever of them is running. Leaving pf enabled
// with no rules of ours loaded changes nothing that anyone can observe, so
// the teardown leaves it. When the status could not be read nothing is
// changed either.
func (p *Plan) darwinPfEnableSteps(current map[string]string) []Step {
	if current[darwinPfStatusKnob] != "0" {
		return nil
	}
	why := "pf was disabled; the fail-closed anchor is enforced only while pf is enabled " +
		"(left enabled on teardown: pf is reference counted by system services and pfctl -d would disable it under them)"
	return []Step{{
		Op:  OpPf,
		Why: why,
		Do:  Command{Path: BinPfctl, Args: []string{"-e"}, Why: why},
	}}
}

// darwinFirewallStep loads the anchor as one transaction, the property
// "nft -f -" gives on Linux: either every rule is in force or none is.
func (p *Plan) darwinFirewallStep() Step {
	why := "install the fail-closed ruleset, which must be in force before anything can be forwarded"
	return Step{
		Op:   OpPf,
		Why:  why,
		Do:   Command{Path: BinPfctl, Args: []string{"-a", darwinAnchor, "-f", "-"}, Stdin: p.darwinRulesetFor(ForwardNormal), Why: why},
		Undo: Command{Path: BinPfctl, Args: []string{"-a", darwinAnchor, "-F", "all"}, Why: "remove the generated anchor"},
	}
}

func (p *Plan) darwinCutStep() Step {
	why := "cut forwarded client traffic without taking the hotspot down, so a joined device stays " +
		"joined and can still reach the panel to turn it back on"
	return Step{
		Op:   OpPf,
		Why:  why,
		Do:   Command{Path: BinPfctl, Args: []string{"-a", darwinAnchor, "-f", "-"}, Stdin: p.darwinRulesetFor(ForwardCut), Why: why},
		Undo: Command{Path: BinPfctl, Args: []string{"-a", darwinAnchor, "-F", "all"}, Why: "remove the generated anchor"},
	}
}

func (p *Plan) darwinRestoreStep() Step {
	why := "resume forwarding client traffic through the tunnel"
	return Step{
		Op:   OpPf,
		Why:  why,
		Do:   Command{Path: BinPfctl, Args: []string{"-a", darwinAnchor, "-f", "-"}, Stdin: p.darwinRulesetFor(ForwardNormal), Why: why},
		Undo: Command{Path: BinPfctl, Args: []string{"-a", darwinAnchor, "-F", "all"}, Why: "remove the generated anchor"},
	}
}

// darwinSysctlSteps: forwarding on, IPv6 forwarding off. Both global, both
// read by darwinDetect beforehand so each has an exact inverse.
func (p *Plan) darwinSysctlSteps(current map[string]string) []Step {
	return []Step{
		sysctlStep("net.inet.ip.forwarding", "1", current,
			"a machine that shares its connection is a router, and a router forwards"),
		sysctlStep("net.inet6.ip6.forwarding", "0", current,
			"hotspot clients get no IPv6 (design 4.5), so nothing may forward it"),
	}
}

// darwinServerRouteSteps pins a host route to each server address through the
// real gateway, so the engine's own connection never enters its own tunnel.
// Same reasoning as ServerRouteSteps; route(8) spelling.
func (p *Plan) darwinServerRouteSteps() []Step {
	var steps []Step
	for _, s := range p.ServerAddr {
		if !p.canPin(s) {
			continue
		}
		why := "the engine's own connection to the server must leave by the uplink, or the tunnel would try to reach itself"
		steps = append(steps, Step{
			Op:   OpRoute,
			Why:  why,
			Do:   Command{Path: BinRoute, Args: p.darwinHostRouteArgs("add", s), Why: why},
			Undo: Command{Path: BinRoute, Args: p.darwinHostRouteArgs("delete", s), Why: "remove the pinned server route"},
		})
	}
	return steps
}

func (p *Plan) darwinHostRouteArgs(verb string, s netip.Addr) []string {
	args := []string{"-n", verb}
	if s.Is6() {
		args = append(args, "-inet6")
	}
	args = append(args, "-host", s.String())
	if verb == "delete" {
		return args
	}
	gw, onLink := p.UplinkGateway, p.UplinkOnLink
	if s.Is6() {
		gw = p.UplinkV6Gw
		onLink = !gw.IsValid()
	}
	if onLink || !gw.IsValid() {
		return append(args, "-interface", p.Uplink)
	}
	return append(args, gw.String())
}

// darwinRulesetFor renders the anchor. Translation rules come before filter
// rules, which is pf's grammar and not a stylistic choice.
//
// What it guarantees, rule by rule, is written beside each rule so that the
// generated text can be read on the machine with "pfctl -a <anchor> -s all".
func (p *Plan) darwinRulesetFor(state ForwardState) string {
	var b strings.Builder
	hot := p.HotspotSubnet.String()
	gw := p.HotspotGateway.String()
	fmt.Fprintf(&b, "# Caspian-BYOC, generated. Anchor %s. Loaded as one transaction.\n", darwinAnchor)
	fmt.Fprintf(&b, "# clients arrive on %s; the uplink is %s; the tunnel is %s.\n", darwinSharingBridge, p.Uplink, p.Tun)

	// Client DNS is answered on this machine by the engine's local listener,
	// whatever resolver address the client was told. bootpd hands clients the
	// bridge address and mDNSResponder's proxy holds port 53 there, so the
	// redirect is the only way to the engine; see the port research.
	fmt.Fprintf(&b, "rdr pass on %s inet proto { tcp, udp } from %s to any port 53 -> 127.0.0.1 port %d\n",
		darwinSharingBridge, hot, p.Opts.DNSPort)

	// No IPv6 for clients at all (design 4.5).
	fmt.Fprintf(&b, "block drop in quick on %s inet6 all\n", darwinSharingBridge)

	// DHCP to bootpd. A client asking for an address has no address yet.
	fmt.Fprintf(&b, "pass in quick on %s inet proto udp from any port 68 to any port 67\n", darwinSharingBridge)

	// The panel, and nothing else on this machine.
	fmt.Fprintf(&b, "pass in quick on %s inet proto tcp from %s to %s port %d\n", darwinSharingBridge, hot, gw, p.Opts.PanelPort)
	fmt.Fprintf(&b, "pass in quick on %s inet proto icmp from %s to %s\n", darwinSharingBridge, hot, gw)
	fmt.Fprintf(&b, "block drop in quick on %s inet from %s to %s\n", darwinSharingBridge, hot, gw)

	// DNS over TLS would carry names past the engine's resolver policy.
	fmt.Fprintf(&b, "block drop in quick on %s inet proto { tcp, udp } from %s to any port 853\n", darwinSharingBridge, hot)

	if state == ForwardNormal {
		// pf policy routing: every other client packet is handed to the
		// tunnel at the point it arrives, so the main routing table never
		// sees it and the machine's own default route is untouched.
		fmt.Fprintf(&b, "pass in quick on %s route-to (%s %s) inet from %s to any keep state\n",
			darwinSharingBridge, p.Tun, darwinTunPeer, hot)
	} else {
		fmt.Fprintf(&b, "# forwarding is cut: the steering rule is withheld and the block below applies\n")
	}

	// Fail closed, on the INBOUND side and nowhere else. With the tunnel down
	// the route-to rule's pool names an interface that no longer exists and
	// xnu's pf_route drops the packet (bsd/net/pf.c: ifp == NULL, goto bad);
	// with forwarding cut the pass rule is withheld and this is met first.
	//
	// There is deliberately no "block out on <uplink> from <hotspot>". pf
	// applies Internet Sharing's nat rule before outbound filtering, so by the
	// time a client packet is seen leaving the uplink its source is already the
	// uplink's own address and such a rule matches nothing. A rule that reads
	// as a guarantee and enforces nothing is worse than none.
	fmt.Fprintf(&b, "block drop in quick on %s inet from %s to any\n", darwinSharingBridge, hot)
	return b.String()
}

// darwinAssertHotspotInterfaceReleased checks the interface anything would
// bind to, which on macOS is Internet Sharing's bridge and not the radio.
//
// The radio may still be joined to the house network when this runs: Internet
// Sharing switches it into hotspot mode itself when it starts, and a Mac on
// Ethernet is very often also on Wi-Fi. What must not be true is that the
// bridge already exists carrying a network this appliance does not own, which
// is the macOS shape of the DHCPNAK incident the Linux readback exists for.
func darwinAssertHotspotInterfaceReleased(ctx context.Context, r Runner, p *Plan) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	link, present, err := darwinReadLink(ctx, r, darwinSharingBridge)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	for _, a := range link.Prefixes {
		if a.Addr().IsLinkLocalUnicast() || a.Addr().IsLoopback() {
			continue
		}
		if p.HotspotSubnet.IsValid() && p.HotspotSubnet.Contains(a.Addr()) {
			continue
		}
		return fmt.Errorf("%w: %s already carries %s, which is not in the hotspot subnet %s; "+
			"Internet Sharing is serving a network this appliance did not configure",
			ErrHotspotNotReleased, darwinSharingBridge, a, p.HotspotSubnet)
	}
	return nil
}

// darwinAssertHotspotIsAccessPoint reads the bridge back: present, up, and
// carrying the gateway address the plan chose. That is the kernel's evidence
// that Internet Sharing built the network it was asked for. The SSID cannot be
// read back with any tool that ships with macOS, so it is checked against the
// preferences by the access point driver and not here.
func darwinAssertHotspotIsAccessPoint(ctx context.Context, r Runner, p *Plan, ssid string) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	link, present, err := darwinReadLink(ctx, r, darwinSharingBridge)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("%w: %s does not exist, so Internet Sharing has not built the network",
			ErrNotAccessPoint, darwinSharingBridge)
	}
	if link.State != "UP" {
		return fmt.Errorf("%w: %s is not up", ErrNotAccessPoint, darwinSharingBridge)
	}
	for _, a := range link.Prefixes {
		if a.Addr() == p.HotspotGateway {
			return nil
		}
	}
	return fmt.Errorf("%w: %s does not carry the gateway address %s", ErrNotAccessPoint, darwinSharingBridge, p.HotspotGateway)
}

// darwinReadLink reads one interface with ifconfig. A missing interface is
// reported as absent, not as an error: ifconfig exits 1 with "interface X does
// not exist", which the runner surfaces as an error with that stderr.
func darwinReadLink(ctx context.Context, r Runner, name string) (Link, bool, error) {
	res, err := r.Run(ctx, Command{
		Path: BinIfconfig, Args: []string{name},
		Why: "read back what the interface actually is, rather than trusting that a command to change it succeeded",
	})
	if err != nil {
		if strings.Contains(strings.ToLower(res.Stderr), "does not exist") {
			return Link{}, false, nil
		}
		return Link{}, false, fmt.Errorf("netcfg: read %s: %w", name, err)
	}
	for _, l := range ParseIfconfig(res.Stdout) {
		if l.Name == name {
			return l, true, nil
		}
	}
	return Link{}, false, nil
}
