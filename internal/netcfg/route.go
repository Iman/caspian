// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"fmt"
	"net/netip"
	"strconv"
)

// Step is one change together with its inverse. Nothing in this package
// applies a change without knowing how to undo it: the inverse is written to
// the journal on disk before the change is made, so a process killed halfway
// through can still be undone by the next start.
//
// A zero Undo means the step has no inverse. That is a statement, not an
// oversight, and every place that produces one says why in Why.
type Step struct {
	Op   string  `json:"op"`
	Why  string  `json:"why"`
	Do   Command `json:"do"`
	Undo Command `json:"undo"`

	// Pre, when set, is a read-only query answering whether this change is
	// already in place. It is consulted before anything is written to the
	// journal. It carries a function, so it is not serialised: the journal
	// stores what was done and how to undo it, never how it was decided.
	Pre *AlreadyApplied `json:"-"`
}

// Step operation names, used in the journal and in logs.
const (
	OpSysctl = "sysctl"
	OpAddr   = "addr"
	OpLink   = "link"
	OpRoute  = "route"
	OpRule   = "rule"
	OpNft    = "nft"
	OpProxy  = "proxy"
	// OpCreateIface is the creation of the access point's own interface. It
	// has an op of its own because its failure is the one this package has a
	// planned answer for: the radio's combination table can advertise a
	// station and an access point together and the driver still refuse, so a
	// caller that sees this op fail asks the plan for its takeover fallback.
	OpCreateIface = "iface"
)

func ipCmd(why string, args ...string) Command {
	return Command{Path: BinIP, Args: args, Why: why}
}

// sysctlStep builds a knob change whose inverse restores exactly what was read
// during detection. A knob that could not be read produces a step with no
// inverse rather than a guess: writing back a value nobody measured is how a
// teardown leaves the machine in a state it was never in.
func sysctlStep(knob, value string, current map[string]string, why string) Step {
	s := Step{
		Op:  OpSysctl,
		Why: why,
		Do:  Command{Path: BinSysctl, Args: []string{"-w", knob + "=" + value}, Why: why},
	}
	if prev, ok := current[knob]; ok {
		s.Undo = Command{
			Path: BinSysctl,
			Args: []string{"-w", knob + "=" + prev},
			Why:  "restore the value read before the change",
		}
		return s
	}
	s.Why += " (no previous value was read for " + knob + ", so this change has no recorded inverse)"
	return s
}

// hostRouteArgs builds the pinned host route to one server address.
func (p *Plan) hostRouteArgs(verb string, s netip.Addr) []string {
	bits := 32
	args := []string{"route", verb}
	if s.Is6() {
		bits = 128
		args = []string{"-6", "route", verb}
	}
	args = append(args, netip.PrefixFrom(s, bits).String())

	gw, dev := p.UplinkGateway, p.Uplink
	onlink := p.UplinkOnLink
	if s.Is6() {
		gw, dev = p.UplinkV6Gw, p.UplinkV6Dev
		onlink = !gw.IsValid()
		if dev == "" {
			dev = p.Uplink
		}
	}
	if !onlink && gw.IsValid() {
		args = append(args, "via", gw.String())
	}
	args = append(args, "dev", dev)
	if verb == "add" {
		args = append(args, "proto", "static")
	}
	args = append(args, "metric", strconv.Itoa(pinnedRouteMetric))
	return args
}

// pinnedRouteMetric is low so the pinned host route always beats anything a
// DHCP client installs for the same address.
const pinnedRouteMetric = 5

// ServerRouteSteps returns the pinned host route to each server address.
//
// This is the route without which the tunnel eats itself. The engine's own
// connection to the user's server is traffic the box originates. Once a
// default route through the tunnel exists in any table that traffic can
// select, that connection matches it, and the engine tries to reach its own
// uplink through the tunnel it has not built yet. The engine's own README
// warns that a careless default route will lock the router in an infinite
// network loop; this is that loop, and a /32 through the real gateway is what
// keeps the one connection that must stay outside the tunnel outside it.
//
// Under StrategySplitDefault the route is load bearing: 0.0.0.0/1 and
// 128.0.0.0/1 in the main table cover the server address, and only a more
// specific route beats them. Under StrategyPolicy the box's own unmarked
// traffic keeps using the main table, so the loop does not arise from the
// hotspot rule alone; the route stays because the fwmark rule installed
// alongside it lets box-originated traffic select the tunnel table, and
// because a plan that is correct only while nobody adds a rule is not correct.
func (p *Plan) ServerRouteSteps() []Step {
	why := "pin the route to the proxy server through the real gateway, so the engine's own connection " +
		"to it does not match the default route through the tunnel and loop through itself"
	steps := make([]Step, 0, len(p.ServerAddr))
	for _, s := range p.ServerAddr {
		if !p.canPin(s) {
			// No default route exists in this address's family, so there is
			// no gateway to pin through. Emitting a route with no via and no
			// device would install one that silently blackholes.
			// PlanNetwork has already recorded this address in
			// UnpinnableServers and refused outright if none can be pinned.
			continue
		}
		steps = append(steps, Step{
			Op:   OpRoute,
			Why:  why,
			Do:   ipCmd(why, p.hostRouteArgs("add", s)...),
			Undo: ipCmd("remove the pinned host route", p.hostRouteArgs("del", s)...),
		})
	}
	return steps
}

// SysctlSteps returns the kernel knob changes, with current holding the values
// read during detection so each change can be inverted exactly.
func (p *Plan) SysctlSteps(current map[string]string) []Step {
	rp := strconv.Itoa(p.Opts.RPFilter)
	var steps []Step

	steps = append(steps, sysctlStep("net.ipv4.ip_forward", "1", current,
		"a box that shares its connection is a router, and a router forwards"))

	// rp_filter, and why it is set in exactly two places rather than on every
	// interface in the path.
	//
	// The failure it prevents: client traffic leaves through the tunnel by way
	// of a second routing table, and the replies come back on the tunnel
	// device while the reverse path lookup for the client's address in the
	// main table names the hotspot device. Strict reverse-path filtering
	// treats that asymmetry as a spoofed source and drops the packet as a
	// martian. The symptom is the worst kind: the tunnel connects, the panel
	// goes green, and nothing loads.
	//
	// Why conf.all alone is sufficient, and this is counter-intuitive enough
	// that it is worth stating before someone "fixes" it. The kernel's own
	// ip-sysctl documentation says of rp_filter:
	//
	//	0 - No source validation.
	//	1 - Strict mode as defined in RFC3704 Strict Reverse Path
	//	2 - Loose mode as defined in RFC3704 Loose Reverse Path
	//
	//	The max value from conf/{all,interface}/rp_filter is used
	//	when doing source validation on the {interface}.
	//
	// (Documentation/networking/ip-sysctl.txt, read 2026-08-30 from
	// kernel.org.) The combination is the numeric MAXIMUM, and loose is the
	// numerically LARGEST of the three values. So conf.all = 2 yields
	// max(2, x) = 2 for every x the interface can hold, which pins every
	// interface to loose whatever its own value is. A per-interface write
	// cannot change that outcome, in either direction.
	//
	// An earlier version of this comment asserted the opposite, that relaxing
	// conf.all alone "changes nothing while any interface still has the strict
	// value", and wrote rp_filter on the uplink, the hotspot and the tunnel to
	// compensate. That was the inverse of the documented behaviour: strict is
	// 1 and loose is 2, so the maximum favours loose rather than strict. The
	// three writes bought nothing and cost a great deal, which is recorded
	// above SysctlKnobs.
	rpWhy := "loosen reverse-path filtering; with two routing tables the return path is asymmetric " +
		"and the strict filter drops replies as martians, which presents as a tunnel that connects and carries nothing"
	steps = append(steps, sysctlStep("net.ipv4.conf.all.rp_filter", rp, current,
		rpWhy+" (the kernel uses the maximum of conf.all and the per-interface value, and loose is the "+
			"larger value, so setting conf.all pins every interface to loose on its own)"))

	// conf.default is set as well, and it is redundancy rather than a second
	// half of the mechanism. conf.all above already decides the outcome for
	// every interface. conf.default is the value an interface is created with,
	// so it keeps the guarantee if conf.all is ever lowered by something else
	// on the box after this runs.
	steps = append(steps, sysctlStep("net.ipv4.conf.default.rp_filter", rp, current,
		rpWhy+" (conf.default is what a newly created interface starts with, kept as redundancy in case conf.all is lowered later)"))

	switch p.Opts.IPv6 {
	case IPv6Block:
		// There is no IPv6 tunnel. A client that gets a working IPv6 path
		// prefers it over IPv4 and bypasses the tunnel entirely, so IPv6 is
		// switched off on the hotspot interface rather than merely filtered.
		// The firewall covers it too; this is the second of the two, because a
		// leak this shape is not worth one mechanism.
		steps = append(steps, sysctlStep("net.ipv6.conf.all.forwarding", "0", current,
			"there is no IPv6 tunnel, so forwarded IPv6 would leave outside the tunnel"))
		// No net.ipv6.conf.<hotspot>.disable_ipv6 here, deliberately.
		//
		// It is a per-interface knob with the same three problems as the
		// rp_filter writes above: on a hotspot interface this package creates
		// there is no prior value to read, so the change gets no inverse; the
		// write would be ordered before the interface exists; and on a hotspot
		// interface that already exists, failing to restore it leaves the
		// user's adapter with IPv6 switched off after an uninstall.
		//
		// What still blocks a client IPv6 path, with none of those costs: this
		// knob (forwarding off globally), the firewall dropping forwarded IPv6
		// in both directions on the hotspot, and the firewall dropping router
		// advertisements out of the hotspot so no client can autoconfigure an
		// address in the first place. Three mechanisms remain, and every one of
		// them is either global and measurable or stateless.
	case IPv6Forward:
		// MEASURED on the target on 2026-08-30, before anyone prices this:
		//
		//	net.ipv6.conf.all.forwarding = 0
		//	net.ipv6.conf.eth0.accept_ra = 0
		//
		// The kernel documents accept_ra as "enabled if local forwarding is
		// disabled, disabled if local forwarding is enabled"
		// (Documentation/networking/ip-sysctl.txt), so writing forwarding=1
		// normally costs a box its own SLAAC address and v6 default route.
		// THAT IS NOT TRUE OF THIS BOX. accept_ra is already 0 with forwarding
		// at 0, so the functional default is not in force, and the box has no
		// global v6 address to lose in the first place: ip -6 route show
		// default printed nothing and eth0 held only fe80::/64.
		//
		// So the side effect is real in general and unobservable here, which
		// means it cannot be measured on this hardware and must not be written
		// up as though it had been. testdata/PROVENANCE.md, "The measured IPv6
		// sysctls", carries the numbers.
		steps = append(steps, sysctlStep("net.ipv6.conf.all.forwarding", "1", current,
			"IPv6 is carried, so the box must forward it"))
	}
	return steps
}

// VirtualIfaceSteps create the access point's own interface on the radio, for
// the case where the radio already carries a station link that must not be
// disturbed. On a radio that is free this returns nothing, because the
// interface already on it is used directly.
func (p *Plan) VirtualIfaceSteps() []Step {
	if !p.HotspotIsVirtual || p.Hotspot == "" || p.HotspotPhy == "" {
		return nil
	}
	// MEASURED on the target on 2026-08-30: this command FAILS on the Pi 5's
	// built-in radio with "command failed: Input/output error (-5)", exit 251,
	// and no interface appears, while wlan0 is associated. The radio's own
	// combination table advertises "#{ managed } <= 1, #{ AP } <= 1" and the
	// planner reads it correctly; the brcmfmac driver refuses anyway.
	//
	// So a combination line states what the hardware could do in principle and
	// is not proof that creating the interface succeeds. Nothing short of
	// trying it settles the difference, which is why this stays the first
	// choice and Plan.HotspotTakeover is the answer when it fails rather than
	// something the planner could have decided up front.
	why := "create an interface for the access point on " + p.HotspotPhy + " rather than reconfiguring the " +
		"one already there, which would drop the network that interface is connected to"
	steps := []Step{{
		Op:   OpCreateIface,
		Why:  why,
		Do:   Command{Path: BinIw, Args: []string{"phy", p.HotspotPhy, "interface", "add", p.Hotspot, "type", "__ap"}, Why: why},
		Undo: Command{Path: BinIw, Args: []string{"dev", p.Hotspot, "del"}, Why: "remove the interface created for the access point"},
		Pre:  ifacePresent(p.Hotspot),
	}}

	// Release the interface THIS PACKAGE JUST CREATED from NetworkManager,
	// before an address goes on it and before it is brought up.
	//
	// MEASURED on the target on 2026-08-30, polling "ip -br addr" every 0.3s
	// across a start:
	//
	//	18:47:14.634   ap0   DOWN   10.83.51.1/24
	//	18:47:15.353   ap0   UP     (no address)
	//	18:47:15.960   ap0   gone
	//
	// NetworkManager saw a new 802.11 device appear, took it from unmanaged to
	// external to FULL management, and the address went on the transition to
	// disconnected. dnsmasq then exited 2 with "failed to create listening
	// socket for 10.83.51.1: Cannot assign requested address".
	//
	// The fix was validated on the hardware BEFORE it was written here. The
	// sequence below, run by hand on the box, kept the address and let a UDP
	// socket bind 10.83.51.1:67, which is what dnsmasq needs:
	//
	//	iw phy phy1 interface add ap0test type __ap
	//	nmcli device set ap0test managed no
	//	ip link set dev ap0test up
	//	ip address add 10.83.51.1/24 dev ap0test
	//
	// Two things this deliberately does NOT do. It does not touch the
	// "p2p-dev-<if>" device NetworkManager creates alongside: the measured
	// sequence above did not need it, and this package does not add
	// unmeasured commands. And it has no inverse. The interface is removed by
	// the inverse of the step above, and a reboot destroys it outright, so
	// there is nothing to give management back TO. That is the opposite of
	// the takeover path, where the interface is the user's own and its
	// release must be undone or the box never rejoins their WiFi.
	if p.NetworkManagerPresent {
		relWhy := "release " + p.Hotspot + " from NetworkManager, which claims a wifi device the moment it appears " +
			"and flushes the address off it"
		steps = append(steps, Step{
			Op:   OpLink,
			Why:  relWhy,
			Do:   Command{Path: BinNmcli, Args: []string{"device", "set", p.Hotspot, "managed", "no"}, Why: relWhy},
			Undo: Command{},
		})
	}
	return steps
}

// HotspotReleaseSteps take the hotspot interface away from whatever holds it,
// strip the addresses it carries from the network it was joined to, and put it
// into access point mode.
//
// They exist because a takeover that does not take anything is worse than no
// takeover at all. MEASURED on the target on 2026-08-30: the fallback ran, the
// service reported running with hotspot=wlan0, and thirty seconds later wlan0
// was still type managed, still associated to the house network on the
// station's channel, still holding its station address, and broadcasting
// nothing. dnsmasq bound to it anyway and answered a real device on that LAN
// with DHCPNAK, which is this appliance disrupting a network it does not own.
//
// Nothing here is optional and the order is the whole point:
//
//  1. Release it from its manager, or the manager reconnects it underneath.
//  2. Remove the station addresses, or a server bound to this interface has a
//     path onto the network it was joined to.
//  3. Bring it down, because the type cannot change while it is up.
//  4. Set the type, so the state is provable before hostapd is asked for it.
//
// Every one of them is journalled. A user whose Pi permanently stopped joining
// their WiFi has lost more than a hotspot, so the release in particular must
// come back.
//
// # Why the release is guarded on the MANAGER and the rest on the takeover
//
// The two questions are different and were conflated until 2026-08-30, when
// the whole function was guarded on HotspotTakenOver alone.
//
// Steps 2 to 4 undo a STATION LINK. They only make sense where there is one,
// which is the takeover and nowhere else: on a radio whose free interface is
// used directly there are no station addresses to strip, and on an interface
// this package just created there is nothing to put into AP mode that is not
// already in it.
//
// Step 1 answers a different question, "who else can act on this interface",
// and the answer does not depend on how the interface was chosen. A
// disconnected adapter NetworkManager still holds is one it can connect to a
// remembered network at any moment, including while hostapd is beaconing on
// it, which is the 2026-08-30 incident above arriving by another door. So the
// release follows the MEASURED manager, which Plan.acceptHotspot now carries
// on every path that names an interface it did not create.
//
// TestTheHotspotInterfaceIsReleasedFromNetworkManagerOnEveryPathThatNamesOne
// is the guard, and it fails if this goes back to asking about the takeover.
func (p *Plan) HotspotReleaseSteps() []Step {
	if p.Hotspot == "" {
		return nil
	}
	var steps []Step

	if p.HotspotManager == ManagedByNetworkManager {
		why := "release " + p.Hotspot + " from NetworkManager, which otherwise reconnects it to the network it was " +
			"joined to while the access point is meant to be running on it"
		steps = append(steps, Step{
			Op:   OpLink,
			Why:  why,
			Do:   Command{Path: BinNmcli, Args: []string{"device", "set", p.Hotspot, "managed", "no"}, Why: why},
			Undo: Command{Path: BinNmcli, Args: []string{"device", "set", p.Hotspot, "managed", "yes"}, Why: "give the interface back, or this box never rejoins the user's WiFi again"},
		})
	}

	// The addresses come off on EVERY path that names an interface which
	// already exists, not only the takeover.
	//
	// It used to be gated behind HotspotTakenOver with the reasoning that
	// everything below "undoes a station link". Two of the three steps do.
	// This one does not: it removes an address that is on the interface, and
	// an interface joined to nothing can still be holding one. Measured on
	// 2026-08-30, wlan0 sat unassociated with a house-network address on it,
	// and a DHCP server bound there answers real devices on that LAN.
	for _, pfx := range p.HotspotStationPrefixes {
		why := "take " + pfx.String() + " off " + p.Hotspot + ": it belongs to the network this interface was joined " +
			"to, and leaving it gives anything bound here a path onto that network"
		steps = append(steps, Step{
			Op:   OpAddr,
			Why:  why,
			Do:   ipCmd(why, "address", "del", pfx.String(), "dev", p.Hotspot),
			Undo: ipCmd("put the station address back", "address", "add", pfx.String(), "dev", p.Hotspot),
		})
	}

	// Bringing the interface down and retyping it IS the takeover's business
	// alone: there is no station link to end on the other paths, and hostapd
	// sets the type itself on an interface nothing is using.
	if !p.HotspotTakenOver {
		return steps
	}

	downWhy := "the interface type cannot be changed while the interface is up"
	steps = append(steps, Step{
		Op:   OpLink,
		Why:  downWhy,
		Do:   ipCmd(downWhy, "link", "set", "dev", p.Hotspot, "down"),
		Undo: ipCmd("bring the interface back up", "link", "set", "dev", p.Hotspot, "up"),
	})

	typeWhy := "put " + p.Hotspot + " into access point mode before anything is asked to serve on it, so the state " +
		"can be read back rather than assumed from a process being alive"
	steps = append(steps, Step{
		Op:   OpCreateIface,
		Why:  typeWhy,
		Do:   Command{Path: BinIw, Args: []string{"dev", p.Hotspot, "set", "type", "__ap"}, Why: typeWhy},
		Undo: Command{Path: BinIw, Args: []string{"dev", p.Hotspot, "set", "type", "managed"}, Why: "put the interface back to a station"},
	})
	return steps
}

// HotspotAddrSteps bring the hotspot interface up and put the box's address on
// it, IN THAT ORDER.
//
// The order was the other way round until 2026-08-30, when the target showed
// the address being added while the interface was still down and gone by the
// time it was up. The load-bearing fix for that is the NetworkManager release
// in VirtualIfaceSteps, not this order; this is the belt to that pair of
// braces, and it matches the sequence that was proven by hand on the box.
//
// Do NOT add a readback here that requires the interface to be UP. An access
// point interface has no carrier until hostapd starts, so it reads DOWN even
// after a successful "ip link set up", and it holds and serves its address
// anyway. MEASURED on the target: "ap0test DOWN 10.83.51.1/24" with a UDP
// socket binding 10.83.51.1:67 successfully at the same moment. The address is
// the signal; the operational state is not.
func (p *Plan) HotspotAddrSteps() []Step {
	if p.Hotspot == "" || !p.HotspotSubnet.IsValid() {
		return nil
	}
	addr := netip.PrefixFrom(p.HotspotGateway, p.HotspotSubnet.Bits()).String()
	addWhy := "the box is the default gateway for its clients, so it holds the first address on the hotspot subnet"
	return []Step{
		{
			Op:  OpLink,
			Why: "the access point cannot start on a down interface",
			Do:  ipCmd("bring the hotspot interface up", "link", "set", "dev", p.Hotspot, "up"),
			// No inverse on purpose. Bringing a radio down on teardown is
			// worse than leaving it up: the machine's own WiFi client, and
			// the panel the user is reading, may be on it. Removing the
			// address below is the inverse that matters.
			Undo: Command{},
		},
		{
			Op:   OpAddr,
			Why:  addWhy,
			Do:   ipCmd(addWhy, "address", "add", addr, "dev", p.Hotspot),
			Undo: ipCmd("take the hotspot address back off", "address", "del", addr, "dev", p.Hotspot),
		},
	}
}

// TunnelAddrSteps put an address on the tunnel device. The engine creates the
// device, sets its MTU and brings the link up, and does nothing else: no
// address, no route, no rule.
func (p *Plan) TunnelAddrSteps() []Step {
	if !p.TunSubnet.IsValid() {
		return nil
	}
	addr := netip.PrefixFrom(p.TunAddr, p.TunSubnet.Bits()).String()
	why := "the tunnel device is created with no address at all, and an interface with no address " +
		"cannot be the target of a route"
	return []Step{{
		Op:   OpAddr,
		Why:  why,
		Do:   ipCmd(why, "address", "add", addr, "dev", p.Tun),
		Undo: ipCmd("take the tunnel address back off", "address", "del", addr, "dev", p.Tun),
	}}
}

// TunnelRouteSteps put client traffic onto the tunnel, by whichever strategy
// the options chose.
func (p *Plan) TunnelRouteSteps() []Step {
	table := strconv.Itoa(p.Opts.Table)
	prio := strconv.Itoa(p.Opts.RulePrio)
	markPrio := strconv.Itoa(p.Opts.RulePrio - 1)
	mark := "0x" + strconv.FormatUint(uint64(p.Opts.FwMark), 16)

	if p.Opts.Strategy == StrategySplitDefault {
		// Two half defaults rather than one replacement default. They are more
		// specific than 0.0.0.0/0, so the uplink's own default is left in the
		// table untouched and reappears the moment these are removed, which
		// makes teardown a deletion rather than a restoration.
		var steps []Step
		for _, half := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
			why := "send everything through the tunnel using two half defaults, which are more specific " +
				"than the uplink's default route and so do not have to replace it"
			steps = append(steps, Step{
				Op:   OpRoute,
				Why:  why,
				Do:   ipCmd(why, "route", "add", half, "dev", p.Tun, "proto", "static"),
				Undo: ipCmd("remove the half default", "route", "del", half, "dev", p.Tun),
			})
		}
		return steps
	}

	subnetWhy := "keep traffic between two hotspot clients on the hotspot: without this route the rule " +
		"below sends it to the tunnel table, whose only route is the default, and client-to-client " +
		"traffic disappears into the tunnel"
	defWhy := "the tunnel is the default route for everything that selects the tunnel table"
	ruleWhy := "send traffic from hotspot clients to the tunnel table, leaving the box's own traffic " +
		"on the main table, which is what lets the engine reach the server at all"
	// NOTHING IN THIS REPOSITORY SETS THIS MARK, and the sentence that used to
	// be here said it was "how the resolver on the box resolves client queries
	// through the tunnel instead of leaking them out of the uplink". That was
	// the inverse of the shipped behaviour in a leak-relevant path, which is
	// the worst place for a confident wrong sentence: a reader concludes the
	// resolver's egress is handled here and stops looking.
	//
	// MEASURED on 2026-08-30 across the whole tree: no generated ruleset
	// contains a mark expression, the engine document carries no socket-mark
	// option, and Options.FwMark is read in exactly one place, which is this
	// rule, the thing that CONSUMES the mark. So the rule matches nothing.
	//
	// Client DNS does not need it and never took this path. dnsmasq forwards to
	// a LOOPBACK address, so the query reaches the engine without being routed
	// at all, and the engine sends it on through the proxy outbound under its
	// own routing rule (internal/xcfg, ruleTagResolvers). The kernel routing
	// table is not involved in a client's lookup.
	//
	// The rule is kept rather than removed because removing it is a decision
	// about what this package offers, not a correction of a wrong sentence, and
	// the two should not travel in one change. What it is FOR is a caller that
	// marks traffic; there is no such caller today.
	// privsvc.TestTheResolverPathDoesNotDependOnAPacketMarkNothingSets is the
	// tripwire: it goes red the day something starts setting a mark, which is
	// the day this comment has to be read again instead of trusted.
	markWhy := "select the tunnel table for traffic carrying this mark. NOTHING IN THIS PROGRAM SETS " +
		"THE MARK as of 2026-08-30, so this rule currently matches nothing; the box's own traffic, " +
		"including the engine's socket to the server, stays on the main table"

	return []Step{
		{
			Op:   OpRoute,
			Why:  subnetWhy,
			Do:   ipCmd(subnetWhy, "route", "add", p.HotspotSubnet.String(), "dev", p.Hotspot, "scope", "link", "table", table),
			Undo: ipCmd("remove the hotspot subnet route from the tunnel table", "route", "del", p.HotspotSubnet.String(), "dev", p.Hotspot, "table", table),
		},
		{
			Op:   OpRoute,
			Why:  defWhy,
			Do:   ipCmd(defWhy, "route", "add", "default", "dev", p.Tun, "proto", "static", "table", table),
			Undo: ipCmd("remove the tunnel default route", "route", "del", "default", "dev", p.Tun, "table", table),
		},
		{
			Op:   OpRule,
			Why:  ruleWhy,
			Do:   ipCmd(ruleWhy, "rule", "add", "from", p.HotspotSubnet.String(), "lookup", table, "priority", prio),
			Undo: ipCmd("remove the hotspot policy rule", "rule", "del", "from", p.HotspotSubnet.String(), "lookup", table, "priority", prio),
		},
		{
			Op:   OpRule,
			Why:  markWhy,
			Do:   ipCmd(markWhy, "rule", "add", "fwmark", mark, "lookup", table, "priority", markPrio),
			Undo: ipCmd("remove the fwmark policy rule", "rule", "del", "fwmark", mark, "lookup", table, "priority", markPrio),
		},
	}
}

// FirewallStep loads the generated ruleset.
//
// It is one step because nft loads a ruleset as one transaction: either every
// rule is in force or none is. A firewall applied rule by rule has a window in
// which the policy is drop and the permits are missing, or worse, the permits
// are in and the drops are not.
func (p *Plan) FirewallStep() Step {
	why := "install the fail-closed ruleset, which must be in force before anything can be forwarded"
	return Step{
		Op:   OpNft,
		Why:  why,
		Do:   Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.Ruleset(), Why: why},
		Undo: Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.RulesetTeardown(), Why: "remove the generated tables"},
	}
}

// PreEngineSteps are applied before the engine starts.
//
// The order is not cosmetic. The firewall goes first so there is never a
// moment when forwarding is enabled and the block is not. conf.default follows
// so that the tunnel device inherits it when the engine creates it. The pinned
// host route is last of the routing work and still before the engine, so the
// engine's very first connection to the server is already outside the tunnel.
func (p *Plan) PreEngineSteps(current map[string]string) []Step {
	return p.backend().PreEngineSteps(p, current)
}

// linuxPreEngineSteps is the Linux order. See PreEngineSteps.
func (p *Plan) linuxPreEngineSteps(current map[string]string) []Step {
	steps := []Step{p.FirewallStep()}
	steps = append(steps, p.SysctlSteps(current)...)
	steps = append(steps, p.VirtualIfaceSteps()...)
	steps = append(steps, p.HotspotReleaseSteps()...)
	steps = append(steps, p.HotspotAddrSteps()...)
	steps = append(steps, p.ServerRouteSteps()...)
	return steps
}

// PostEngineSteps are applied once the engine has created its network
// resources. Linux and route-based desktop steps name the tunnel device; the
// interim macOS system-proxy steps name the engine's SOCKS listener. Running
// either kind early fails or points applications at a listener that does not
// exist. The firewall is deliberately not in this list: it never needs an
// engine-owned resource to exist, which is the property the whole fail-closed
// design rests on.
func (p *Plan) PostEngineSteps(current map[string]string) []Step {
	return p.backend().PostEngineSteps(p, current)
}

// linuxPostEngineSteps is the Linux list. See PostEngineSteps.
func (p *Plan) linuxPostEngineSteps(current map[string]string) []Step {
	steps := p.TunnelAddrSteps()
	steps = append(steps, p.TunnelRouteSteps()...)
	return steps
}

// AllSteps is the whole plan in apply order. It is only correct if the tunnel
// device already exists.
func (p *Plan) AllSteps(current map[string]string) []Step {
	return append(p.PreEngineSteps(current), p.PostEngineSteps(current)...)
}

// SysctlKnobs lists every knob the plan will change, which is what detection
// has to read beforehand so that each change has an exact inverse.
//
// Every knob here is global. Not one names an interface, and that is the whole
// point rather than an accident of the current settings.
//
// Three things go wrong at once the moment a plan changes a knob on a named
// interface, and all three were observed on the target on 2026-08-30:
//
//   - It cannot be measured. Detection runs before the hotspot and tunnel
//     devices exist, so /proc has no entry for them. "sysctl -e" skips a knob
//     it cannot read, which is what makes the read succeed at all: eight knobs
//     were asked for and five came back. A knob with no measured value gets no
//     inverse, and teardown then cannot put it back.
//   - It cannot even be written, in the order it was generated. The write for
//     the hotspot interface was emitted before the step that creates that
//     interface. "sysctl -w" on a missing knob fails, sysctlStep does not pass
//     "-e", and Applier.Apply stops at the first failure, so the appliance
//     would have failed to start on its first run.
//   - When it can be measured, guessing its value is worse than not writing
//     it. A fixture guessed conf.eth0.rp_filter was 0 because conf.all was 0.
//     The box reported 2. The generated teardown would therefore have written
//     0 to the uplink, turning reverse-path filtering OFF on a machine that
//     had it on: an uninstall leaving the box weaker than it found it.
//
// None of that is a reason to weaken the guarantee, because the guarantee
// never needed those writes. See the citation above: conf.all pins every
// interface on its own.
// It delegates to BaseSysctlKnobs rather than repeating the list, so the set
// detection reads and the set a plan changes cannot drift apart. They are the
// same set today only because no knob names an interface; if that ever stops
// being true this method is where the difference goes, and
// TestSysctlKnobs_AreGlobalAndFullyMeasured is what will notice.
func (p *Plan) SysctlKnobs() []string { return p.backend().SysctlKnobs(p) }

// String renders a step for a log line.
func (s Step) String() string {
	return fmt.Sprintf("%s: %s", s.Op, s.Do)
}
