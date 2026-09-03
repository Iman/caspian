// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"fmt"
	"strconv"
	"strings"
)

// TableName is the single nftables table this package owns. Filter and NAT
// chains live in one table so that teardown is one delete and so that a
// partially removed firewall is not a state the machine can be in.
const TableName = "caspian"

// Ruleset returns the complete nftables ruleset as text, ready for "nft -f -".
//
// # The property this generator exists to preserve
//
// If the tunnel device disappears the kernel withdraws every route through it,
// and forwarded client traffic falls back to the main table and leaves by the
// uplink. So "the engine stopped" can produce a leak rather than a stop, and
// the block cannot be anything that depends on the tunnel existing.
//
// Whether it disappears is NOT settled. Observed on the target on 2026-08-30:
// xray0 was present in NetworkManager's device list with the service switched
// off, so the device may outlive the engine. That is somebody else's question
// and it does not change this ruleset, which is the point of building the
// block the way it is built: if the device is gone, routes are withdrawn and
// the leak block catches the fallback; if it persists with nothing servicing
// it, traffic entering it is dropped there instead. Neither branch leaks, and
// neither depends on knowing which one happens.
//
// Two things follow, and both are asserted by tests:
//
//   - The FORWARD policy is drop, and the leak block is an explicit rule that
//     names only the hotspot and the uplink. Deleting every rule that mentions
//     the tunnel device leaves a ruleset that still blocks.
//   - Every interface match is by name (iifname, oifname) and never by index
//     (iif, oif). Index matching is resolved when the ruleset is loaded, so a
//     ruleset naming the tunnel by index cannot even be loaded while the
//     tunnel is down, which is exactly when it has to be in force. Name
//     matching is resolved per packet and loads with no tunnel present.
func (p *Plan) Ruleset() string { return p.RulesetFor(ForwardNormal) }

// RulesetFor returns the ruleset for a given forward state.
//
// ForwardCut is the user cutting client traffic without switching the
// appliance off, which would take the hotspot down and disconnect every joined
// device including the one they are holding. It is a flag on this ruleset and
// not a second table: one atomic replace, the same interface names, ports and
// subnet, so the two states cannot drift apart, and the cut differs from
// normal by exactly the forward accepts.
//
// Nothing here persists it. The ruleset is regenerated and reloaded on every
// start, so a cut cannot survive a restart unless a caller deliberately stores
// it, and netcfg does not.
func (p *Plan) RulesetFor(state ForwardState) string {
	var b strings.Builder
	o := p.Opts
	hot, up, tun := p.Hotspot, p.Uplink, p.Tun
	dns := strconv.Itoa(o.DNSPort)

	fmt.Fprintf(&b, "# Caspian-BYOC generated firewall ruleset. Do not edit by hand:\n")
	fmt.Fprintf(&b, "# it is regenerated and replaced atomically on every start.\n")
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# uplink=%s hotspot=%s tunnel=%s hotspot-subnet=%s\n", up, hot, tun, p.HotspotSubnet)
	egress := "restricted"
	if o.Egress == EgressOpen {
		egress = "open"
	}
	forward := "normal"
	if state == ForwardCut {
		forward = "CUT by the user"
	}
	fmt.Fprintf(&b, "# egress=%s forward=%s\n", egress, forward)
	if o.Egress == EgressRestricted {
		fmt.Fprintf(&b, "#\n")
		fmt.Fprintf(&b, "# egress=restricted means this box's OWN outbound traffic is dropped\n")
		fmt.Fprintf(&b, "# unless the output chain names it. \"apt update\" from a shell here will\n")
		fmt.Fprintf(&b, "# fail while the appliance is on. That is intended and was accepted.\n")
	}
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# Every interface match below is by NAME and never by index. Index matching\n")
	fmt.Fprintf(&b, "# is resolved when the ruleset loads, so a ruleset naming %s by index cannot\n", tun)
	fmt.Fprintf(&b, "# be loaded while the tunnel is down, and that is exactly when it has to be\n")
	fmt.Fprintf(&b, "# in force. Name matching is resolved per packet and loads with no tunnel.\n")
	fmt.Fprintf(&b, "\n")

	// The create-then-delete pair makes the load idempotent: the bare
	// declaration creates the table if it is absent so that the delete cannot
	// fail, and the definition that follows replaces it in one transaction.
	fmt.Fprintf(&b, "table inet %s\n", TableName)
	fmt.Fprintf(&b, "delete table inet %s\n\n", TableName)

	fmt.Fprintf(&b, "table inet %s {\n", TableName)

	// ---------------------------------------------------------------- input
	fmt.Fprintf(&b, "\tchain input {\n")
	fmt.Fprintf(&b, "\t\ttype filter hook input priority filter; policy accept;\n\n")
	fmt.Fprintf(&b, "\t\t# THE POLICY IS ACCEPT, DELIBERATELY, AND THIS IS THE PART TO READ\n")
	fmt.Fprintf(&b, "\t\t# BEFORE CHANGING ANYTHING IN THIS CHAIN.\n")
	fmt.Fprintf(&b, "\t\t#\n")
	fmt.Fprintf(&b, "\t\t# This appliance controls FORWARDED client traffic. It does not decide\n")
	fmt.Fprintf(&b, "\t\t# what the owner may run on their own machine, and it does not close\n")
	fmt.Fprintf(&b, "\t\t# their administrative access to it.\n")
	fmt.Fprintf(&b, "\t\t#\n")
	fmt.Fprintf(&b, "\t\t# An earlier version had policy drop and accepted nothing arriving on\n")
	fmt.Fprintf(&b, "\t\t# %s. Measured on the target on 2026-08-30: the moment it loaded,\n", up)
	fmt.Fprintf(&b, "\t\t# every NEW inbound connection to the box was dropped and SSH stopped\n")
	fmt.Fprintf(&b, "\t\t# answering. Existing connections kept working, because of the\n")
	fmt.Fprintf(&b, "\t\t# conntrack rule that used to sit here, so the box looked healthy from\n")
	fmt.Fprintf(&b, "\t\t# the session that was already open while being unreachable to every\n")
	fmt.Fprintf(&b, "\t\t# new one. On a headless machine in another room that is indistinguishable\n")
	fmt.Fprintf(&b, "\t\t# from a crash, and the remaining recovery is a power cycle and a card\n")
	fmt.Fprintf(&b, "\t\t# reader.\n")
	fmt.Fprintf(&b, "\t\t#\n")
	fmt.Fprintf(&b, "\t\t# Accepting here does NOT weaken a host firewall the owner installs.\n")
	fmt.Fprintf(&b, "\t\t# Every base chain registered at this hook is traversed, so a drop in\n")
	fmt.Fprintf(&b, "\t\t# their own table still drops. This policy only stops THIS program\n")
	fmt.Fprintf(&b, "\t\t# from installing a host firewall nobody asked it for.\n")
	fmt.Fprintf(&b, "\t\t#\n")
	fmt.Fprintf(&b, "\t\t# What is still guarded is the hotspot, where the untrusted devices\n")
	fmt.Fprintf(&b, "\t\t# are. A joined device reaches the services this box offers it and\n")
	fmt.Fprintf(&b, "\t\t# nothing else.\n\n")

	fmt.Fprintf(&b, "\t\t# Before the conntrack check, so loopback is never subject to a\n")
	fmt.Fprintf(&b, "\t\t# conntrack edge case.\n")
	fmt.Fprintf(&b, "\t\tiifname \"lo\" accept\n")
	fmt.Fprintf(&b, "\t\tct state invalid drop comment \"malformed or out-of-state, not new inbound\"\n\n")

	fmt.Fprintf(&b, "\t\t# The tunnel device. Redundant under an accept policy and kept\n")
	fmt.Fprintf(&b, "\t\t# explicit: the design calls for an INPUT permit for it, and it is\n")
	fmt.Fprintf(&b, "\t\t# what would carry the engine's own traffic if this policy were ever\n")
	fmt.Fprintf(&b, "\t\t# tightened.\n")
	fmt.Fprintf(&b, "\t\tiifname %q accept comment \"tunnel: a router's own traffic is INPUT, not FORWARD\"\n\n", tun)

	fmt.Fprintf(&b, "\t\t# The hotspot side, and the only place this chain restricts anything.\n")
	fmt.Fprintf(&b, "\t\t# These are the services the box actually serves to joined devices.\n")
	fmt.Fprintf(&b, "\t\tiifname %q ct state established,related accept\n", hot)
	fmt.Fprintf(&b, "\t\tiifname %q udp dport 67 accept comment \"DHCP server for clients\"\n", hot)
	fmt.Fprintf(&b, "\t\tiifname %q udp dport %s accept comment \"client DNS, after the prerouting redirect\"\n", hot, dns)
	fmt.Fprintf(&b, "\t\tiifname %q tcp dport %s accept comment \"client DNS over TCP\"\n", hot, dns)
	fmt.Fprintf(&b, "\t\tiifname %q tcp dport %d accept comment \"panel\"\n", hot, o.PanelPort)
	fmt.Fprintf(&b, "\t\tmeta nfproto ipv4 iifname %q icmp type echo-request accept\n", hot)
	fmt.Fprintf(&b, "\t\tiifname %q drop comment \"a joined device reaches nothing else on this box\"\n", hot)
	fmt.Fprintf(&b, "\t}\n\n")

	// -------------------------------------------------------------- forward
	fmt.Fprintf(&b, "\tchain forward {\n")
	fmt.Fprintf(&b, "\t\ttype filter hook forward priority filter; policy drop;\n\n")

	fmt.Fprintf(&b, "\t\t# THE LEAK BLOCK, and the reason this whole file exists.\n")
	fmt.Fprintf(&b, "\t\t# When the tunnel device disappears the kernel withdraws every route\n")
	fmt.Fprintf(&b, "\t\t# through it and client traffic falls back to the main table and out of\n")
	fmt.Fprintf(&b, "\t\t# the uplink, so \"the engine stopped\" leaks by default instead of\n")
	fmt.Fprintf(&b, "\t\t# stopping. This rule names only the hotspot and the uplink, so the\n")
	fmt.Fprintf(&b, "\t\t# tunnel's absence cannot affect it, and it is first so that nothing\n")
	fmt.Fprintf(&b, "\t\t# added below can precede it.\n")
	fmt.Fprintf(&b, "\t\tiifname %q oifname %q drop comment \"fail-closed: client traffic never leaves by the uplink\"\n\n", hot, up)

	if o.IPv6 == IPv6Block {
		fmt.Fprintf(&b, "\t\t# There is no IPv6 tunnel. A client with a working IPv6 path prefers\n")
		fmt.Fprintf(&b, "\t\t# it over IPv4 and would bypass the tunnel entirely.\n")
		fmt.Fprintf(&b, "\t\tmeta nfproto ipv6 iifname %q drop comment \"no IPv6 tunnel: clients get no IPv6\"\n", hot)
		fmt.Fprintf(&b, "\t\tmeta nfproto ipv6 oifname %q drop\n\n", hot)
	}

	fmt.Fprintf(&b, "\t\t# DNS over TLS and DNS over QUIC would carry a client's queries past the\n")
	fmt.Fprintf(&b, "\t\t# resolver on this box. Rejecting 853 makes a client fall back to port\n")
	fmt.Fprintf(&b, "\t\t# 53, which prerouting redirects. DNS over HTTPS on 443 is not\n")
	fmt.Fprintf(&b, "\t\t# distinguishable from other HTTPS and is carried through the tunnel\n")
	fmt.Fprintf(&b, "\t\t# like anything else, which is a limit of this design and not an\n")
	fmt.Fprintf(&b, "\t\t# oversight.\n")
	fmt.Fprintf(&b, "\t\tiifname %q tcp dport 853 reject with tcp reset comment \"DNS over TLS\"\n", hot)
	fmt.Fprintf(&b, "\t\tiifname %q udp dport 853 drop comment \"DNS over QUIC\"\n\n", hot)

	if o.ClientIsolation {
		fmt.Fprintf(&b, "\t\tiifname %q oifname %q drop comment \"client isolation\"\n\n", hot, hot)
	}

	fmt.Fprintf(&b, "\t\tct state invalid drop\n\n")

	if state == ForwardCut {
		fmt.Fprintf(&b, "\t\t# CLIENT TRAFFIC IS CUT.\n")
		fmt.Fprintf(&b, "\t\t#\n")
		fmt.Fprintf(&b, "\t\t# The user asked for client traffic to stop without switching the\n")
		fmt.Fprintf(&b, "\t\t# appliance off, which would take the hotspot down and disconnect\n")
		fmt.Fprintf(&b, "\t\t# every joined device including the one they are holding.\n")
		fmt.Fprintf(&b, "\t\t#\n")
		fmt.Fprintf(&b, "\t\t# This rule is explicit rather than left to the policy above, so an\n")
		fmt.Fprintf(&b, "\t\t# operator reading \"nft list ruleset\" sees a reason and not an\n")
		fmt.Fprintf(&b, "\t\t# absence. The hotspot, DHCP, DNS and the panel keep working: all of\n")
		fmt.Fprintf(&b, "\t\t# those are INPUT to this box and its OUTPUT replies, and none of\n")
		fmt.Fprintf(&b, "\t\t# them is FORWARD.\n")
		fmt.Fprintf(&b, "\t\t#\n")
		fmt.Fprintf(&b, "\t\t# Nothing persists this. The ruleset is regenerated on every start,\n")
		fmt.Fprintf(&b, "\t\t# so a restart returns to forwarding.\n")
		fmt.Fprintf(&b, "\t\tiifname %q drop comment \"client traffic cut by the user\"\n", hot)
		fmt.Fprintf(&b, "\t}\n\n")
	} else {
		fmt.Fprintf(&b, "\t\t# The only way out for a client, and the only way back in. Both name\n")
		fmt.Fprintf(&b, "\t\t# the tunnel, so both stop matching when it is gone, leaving the policy\n")
		fmt.Fprintf(&b, "\t\t# above to drop everything.\n")
		fmt.Fprintf(&b, "\t\tmeta nfproto ipv4 iifname %q oifname %q ip saddr %s accept\n", hot, tun, p.HotspotSubnet)
		fmt.Fprintf(&b, "\t\tmeta nfproto ipv4 iifname %q oifname %q ip daddr %s accept\n", tun, hot, p.HotspotSubnet)
		if o.IPv6 == IPv6Forward {
			// NO ACCEPT IS EMITTED HERE, and that is deliberate.
			//
			// The IPv4 pair above constrains both the source and the
			// destination to the hotspot's own subnet. There is nothing to
			// constrain an IPv6 rule to: the plan carries no v6 prefix,
			// because nothing in this appliance assigns one, which is what
			// TestIPv6Forward_InstallsNoIPv6AddressingOrRouting records.
			//
			// A rule naming only the two interfaces would therefore accept any
			// source address a client cared to write, while the IPv4 rule
			// beside it accepts one subnet. That asymmetry is not what the flag
			// is meant to mean, and a rule that looks like a constraint and is
			// not one is worse than no rule, because it reads as safe.
			//
			// So IPv6Forward changes one sysctl and nothing in this chain, and
			// the drop policy above still covers v6. When a v6 prefix exists,
			// the accepts belong here constrained exactly as the v4 pair is.
			// TestRuleset_NoUnconstrainedIPv6AcceptInForward fails if they are
			// added without that constraint.
			fmt.Fprintf(&b, "\t\t# IPv6Forward emits no accept here: there is no v6 prefix to\n")
			fmt.Fprintf(&b, "\t\t# constrain one to, so the drop policy above stands for IPv6.\n")
		}
		fmt.Fprintf(&b, "\t}\n\n")
	}

	// --------------------------------------------------------------- output
	fmt.Fprintf(&b, "\tchain output {\n")
	if o.Egress == EgressOpen {
		fmt.Fprintf(&b, "\t\ttype filter hook output priority filter; policy accept;\n\n")
		fmt.Fprintf(&b, "\t\t# Stated explicitly, and accept rather than drop. The box's own traffic\n")
		fmt.Fprintf(&b, "\t\t# is outside the fail-closed guarantee, because the engine has to reach\n")
		fmt.Fprintf(&b, "\t\t# the server through the uplink for the tunnel to exist at all. The\n")
		fmt.Fprintf(&b, "\t\t# promise this ruleset makes covers forwarded client traffic only.\n")
		fmt.Fprintf(&b, "\t\t#\n")
		fmt.Fprintf(&b, "\t\t# This is EgressOpen, which is not the default. It exists as the way\n")
		fmt.Fprintf(&b, "\t\t# back for a user on a network nobody thought about, without a rebuild.\n")
		fmt.Fprintf(&b, "\t\toifname %q accept comment \"tunnel: a router's own traffic is OUTPUT, not FORWARD\"\n", tun)
		// UNCONDITIONAL, and this branch is the one where it matters. The
		// policy here is accept, so this rule is the only thing in the whole
		// ruleset that stops a router advertisement leaving the hotspot.
		// See the note above the same rule in writeRestrictedOutput.
		fmt.Fprintf(&b, "\n\t\t# Never advertise an IPv6 prefix to clients while there is no IPv6\n")
		fmt.Fprintf(&b, "\t\t# tunnel: an autoconfigured client would prefer v6 and bypass it.\n")
		fmt.Fprintf(&b, "\t\toifname %q icmpv6 type nd-router-advert drop\n", hot)
		fmt.Fprintf(&b, "\t}\n\n")
	} else {
		p.writeRestrictedOutput(&b)
	}

	// ----------------------------------------------------------- prerouting
	fmt.Fprintf(&b, "\tchain prerouting {\n")
	fmt.Fprintf(&b, "\t\ttype nat hook prerouting priority dstnat; policy accept;\n\n")
	fmt.Fprintf(&b, "\t\t# Redirected, not merely permitted. A client with a resolver hardcoded\n")
	fmt.Fprintf(&b, "\t\t# into it must be answered by this box rather than allowed out to reach\n")
	fmt.Fprintf(&b, "\t\t# the one it was told to use: redirect rewrites the destination address\n")
	fmt.Fprintf(&b, "\t\t# to this box, so the query is answered here whatever it was aimed at.\n")
	fmt.Fprintf(&b, "\t\tiifname %q udp dport 53 redirect to :%s\n", hot, dns)
	fmt.Fprintf(&b, "\t\tiifname %q tcp dport 53 redirect to :%s\n", hot, dns)
	fmt.Fprintf(&b, "\t}\n\n")

	// ---------------------------------------------------------- postrouting
	fmt.Fprintf(&b, "\tchain postrouting {\n")
	fmt.Fprintf(&b, "\t\ttype nat hook postrouting priority srcnat; policy accept;\n\n")
	if o.MasqueradeToTunnel {
		fmt.Fprintf(&b, "\t\toifname %q masquerade comment \"enabled by configuration\"\n", tun)
	} else {
		fmt.Fprintf(&b, "\t\t# Deliberately empty.\n")
		fmt.Fprintf(&b, "\t\t# No masquerade towards %s: source NAT there is the leak this whole\n", up)
		fmt.Fprintf(&b, "\t\t# ruleset exists to prevent, and it is the single line that would\n")
		fmt.Fprintf(&b, "\t\t# quietly turn the appliance into an ordinary router.\n")
		fmt.Fprintf(&b, "\t\t# No masquerade towards %s either: that device is a userspace netstack\n", tun)
		fmt.Fprintf(&b, "\t\t# which terminates TCP and UDP flows and dials out itself, so a\n")
		fmt.Fprintf(&b, "\t\t# client's source address never reaches the wire and there is nothing\n")
		fmt.Fprintf(&b, "\t\t# to translate. Options.MasqueradeToTunnel turns it on for the case\n")
		fmt.Fprintf(&b, "\t\t# where the tunnel is a real layer 3 interface.\n")
	}
	fmt.Fprintf(&b, "\t}\n")

	fmt.Fprintf(&b, "}\n")
	return b.String()
}

// RulesetTeardown is the inverse of Ruleset: it removes the table this package
// owns and touches nothing else. The bare declaration before the delete makes
// it idempotent, so tearing down twice, or tearing down a machine where the
// ruleset was never loaded, succeeds rather than failing on a missing table.
func (p *Plan) RulesetTeardown() string {
	return fmt.Sprintf(
		"# Caspian-BYOC firewall teardown. The bare declaration makes the delete\n"+
			"# idempotent: it creates the table if it is absent so that removing it\n"+
			"# always succeeds, including on a machine where nothing was ever loaded.\n"+
			"table inet %s\ndelete table inet %s\n", TableName, TableName)
}

// stripInterfaceRules removes every line naming the given interface. It models
// the ruleset as it behaves when that interface does not exist: the rules that
// name it stop matching, and everything else, including the chain policies, is
// untouched. It is used by the test that proves the ruleset still blocks with
// the tunnel absent.
func stripInterfaceRules(ruleset, iface string) string {
	var kept []string
	for _, line := range strings.Split(ruleset, "\n") {
		if strings.Contains(line, "\""+iface+"\"") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// writeRestrictedOutput emits the OUTPUT chain that extends the kill switch to
// the box's own traffic.
//
// # Why the policy is drop here and accept on input
//
// They are different questions. INPUT is about who may reach the owner's
// machine, and closing that closed their administrative access silently, which
// is not this appliance's decision to make. OUTPUT is about what leaves the box
// outside the tunnel, which is the same guarantee the appliance already makes
// for its clients and which the user asked to extend.
//
// # The cost, which the user knows and accepted
//
// With this in force, anything on the Pi that reaches the internet directly
// fails while the appliance is on. "apt update" from a shell is the one people
// will hit. It is stated in the generated header and in Plan.Explain, because
// the person who hits it will be reading one of those two.
//
// # The residual, which is not a leak but will be noticed
//
// Permitting DNS means anything on the box can still reach the network on port
// 53, and the server's hostname is resolved in the clear on the local network
// before the tunnel exists. Neither is made worse by this chain, and once the
// kill switch claims to cover the box's own traffic the exception should be
// visible rather than implied.
//
// # Order
//
// Established is FIRST, not second. Every outbound reply to an inbound
// connection lives there, the administrator's SSH session included, and a drop
// policy without it kills that session the moment the ruleset loads.
func (p *Plan) writeRestrictedOutput(b *strings.Builder) {
	hot, tun := p.Hotspot, p.Tun

	fmt.Fprintf(b, "\t\ttype filter hook output priority filter; policy drop;\n\n")
	fmt.Fprintf(b, "\t\t# THE KILL SWITCH COVERS THIS BOX TOO.\n")
	fmt.Fprintf(b, "\t\t#\n")
	fmt.Fprintf(b, "\t\t# Everything the Pi sends that is not named below is dropped while the\n")
	fmt.Fprintf(b, "\t\t# appliance is on. That includes apt: \"apt update\" from a shell on this\n")
	fmt.Fprintf(b, "\t\t# box will fail until the appliance is switched off. That cost is known\n")
	fmt.Fprintf(b, "\t\t# and was accepted; it is not a fault to be worked around by widening\n")
	fmt.Fprintf(b, "\t\t# this list.\n")
	fmt.Fprintf(b, "\t\t#\n")
	fmt.Fprintf(b, "\t\t# Residual worth knowing: DNS below is a hole. Anything on this box can\n")
	fmt.Fprintf(b, "\t\t# still reach the network on port 53, and the server's hostname is\n")
	fmt.Fprintf(b, "\t\t# resolved in the clear on the local network before any tunnel exists.\n")
	fmt.Fprintf(b, "\t\t# Neither is a leak of client traffic and neither is made worse here.\n\n")

	fmt.Fprintf(b, "\t\t# FIRST, and it has to be first. Every outbound reply to an inbound\n")
	fmt.Fprintf(b, "\t\t# connection is here, including the administrator's SSH session. A drop\n")
	fmt.Fprintf(b, "\t\t# policy without this line kills that session the moment it loads.\n")
	fmt.Fprintf(b, "\t\tct state established,related accept\n")
	fmt.Fprintf(b, "\t\toifname \"lo\" accept\n")
	fmt.Fprintf(b, "\t\toifname %q accept comment \"tunnel: a router's own traffic is OUTPUT, not FORWARD\"\n\n", tun)

	// UNCONDITIONAL, and NOT guarded on Options.IPv6. It was, until 2026-08-30.
	//
	// Advertising a prefix is what lets a client give itself a routable IPv6
	// address, and this box has none to advertise: nothing in this repository
	// sends a router advertisement, there is no radvd, internal/hotspot renders
	// no dnsmasq ra- option, and hotspot.DNSConfig.Validate refuses a hotspot
	// subnet that is not IPv4. So the rule costs nothing under either policy,
	// and making it conditional put the one mechanism that stops a client
	// autoconfiguring behind a setting.
	//
	// The combination where that was load bearing is EgressOpen, whose output
	// policy is accept: there this rule is the only thing in the ruleset that
	// stops an advertisement leaving the hotspot.
	//
	// The emitted text is unchanged from when this was conditional, so the
	// rulesets a real nft has parsed keep their digests. TestRuleset_
	// RouterAdvertisementDropIsUnconditional is the guard.
	fmt.Fprintf(b, "\t\t# Never advertise an IPv6 prefix to clients while there is no IPv6\n")
	fmt.Fprintf(b, "\t\t# tunnel. Redundant under the drop policy above and kept explicit\n")
	fmt.Fprintf(b, "\t\t# because it is load-bearing if that policy is ever loosened.\n")
	fmt.Fprintf(b, "\t\toifname %q icmpv6 type nd-router-advert drop\n\n", hot)

	fmt.Fprintf(b, "\t\t# DHCP, in BOTH directions, and the asymmetry with DNS below is the\n")
	fmt.Fprintf(b, "\t\t# point rather than an oversight.\n")
	fmt.Fprintf(b, "\t\t#\n")
	fmt.Fprintf(b, "\t\t# As a CLIENT: measured on the target, NetworkManager holds a DHCP\n")
	fmt.Fprintf(b, "\t\t# socket per interface. Blocking this costs the box its address at the\n")
	fmt.Fprintf(b, "\t\t# next renewal, hours after the rule loads, with nothing to connect the\n")
	fmt.Fprintf(b, "\t\t# two events.\n")
	fmt.Fprintf(b, "\t\t#\n")
	fmt.Fprintf(b, "\t\t# As a SERVER on the hotspot: these replies are NOT covered by the\n")
	fmt.Fprintf(b, "\t\t# established rule above, because a DHCP reply goes to a broadcast\n")
	fmt.Fprintf(b, "\t\t# address or to a client that has no address yet, so the request and the\n")
	fmt.Fprintf(b, "\t\t# reply share no tuple for conntrack to match. Without this line the\n")
	fmt.Fprintf(b, "\t\t# hotspot beacons, devices associate, and not one of them gets an\n")
	fmt.Fprintf(b, "\t\t# address, which looks like a broken radio rather than a firewall.\n")
	fmt.Fprintf(b, "\t\t#\n")
	fmt.Fprintf(b, "\t\t# DNS needs no such line: a client's query creates the conntrack entry\n")
	fmt.Fprintf(b, "\t\t# and the answer matches its reply direction. That is the difference.\n")
	fmt.Fprintf(b, "\t\tudp sport 68 udp dport 67 accept comment \"DHCP client: renew this box's own lease\"\n")
	fmt.Fprintf(b, "\t\tudp sport 67 udp dport 68 accept comment \"DHCP server: answer joined devices\"\n\n")

	fmt.Fprintf(b, "\t\t# systemd-timesyncd. Measured active on the target with NTP=yes. A\n")
	fmt.Fprintf(b, "\t\t# drifted clock fails REALITY and TLS in a way that reads as a rejected\n")
	fmt.Fprintf(b, "\t\t# configuration, so the symptom points at the user's config rather than\n")
	fmt.Fprintf(b, "\t\t# at this rule.\n")
	fmt.Fprintf(b, "\t\tudp dport 123 accept comment \"NTP\"\n\n")

	fmt.Fprintf(b, "\t\t# DNS. The box resolves the server's hostname, and timesyncd resolves\n")
	fmt.Fprintf(b, "\t\t# its pool names. See the residual in the header.\n")
	fmt.Fprintf(b, "\t\tudp dport 53 accept\n")
	fmt.Fprintf(b, "\t\ttcp dport 53 accept\n\n")

	fmt.Fprintf(b, "\t\t# The engine's own connection to the server, by ADDRESS and not by\n")
	fmt.Fprintf(b, "\t\t# port. Some transports are UDP on 443, so a TCP-only permit would\n")
	fmt.Fprintf(b, "\t\t# break them silently. This is the same address the plan pins a host\n")
	fmt.Fprintf(b, "\t\t# route to, for the same reason: it is the one connection that must\n")
	fmt.Fprintf(b, "\t\t# reach the uplink directly or there is no tunnel at all.\n")
	if len(p.ServerAddr) == 0 {
		fmt.Fprintf(b, "\t\t# (no server address in this plan)\n")
	}
	for _, sa := range p.ServerAddr {
		if sa.Is6() {
			fmt.Fprintf(b, "\t\tip6 daddr %s accept comment \"the proxy server\"\n", sa)
			continue
		}
		fmt.Fprintf(b, "\t\tip daddr %s accept comment \"the proxy server\"\n", sa)
	}
	fmt.Fprintf(b, "\n")

	fmt.Fprintf(b, "\t\t# IPv6 does not work at all without neighbour discovery, and an inet\n")
	fmt.Fprintf(b, "\t\t# table's output chain filters IPv6. Blocking this breaks the box's\n")
	fmt.Fprintf(b, "\t\t# own IPv6 silently. It costs nothing: these are link-local and cannot\n")
	fmt.Fprintf(b, "\t\t# carry a client's traffic. Router advertisement is deliberately not in\n")
	fmt.Fprintf(b, "\t\t# the set; the rule above drops it on the hotspot.\n")
	fmt.Fprintf(b, "\t\tmeta nfproto ipv6 icmpv6 type { nd-neighbor-solicit, nd-neighbor-advert, nd-router-solicit, destination-unreachable, packet-too-big, time-exceeded, parameter-problem } accept\n\n")

	fmt.Fprintf(b, "\t\t# Deliberately absent: mDNS and the IGMP membership reports avahi sends\n")
	fmt.Fprintf(b, "\t\t# for 224.0.0.251. avahi will not reach the network. Nothing this\n")
	fmt.Fprintf(b, "\t\t# product does needs it, and that is a decision rather than an\n")
	fmt.Fprintf(b, "\t\t# oversight.\n")
	fmt.Fprintf(b, "\t}\n\n")
}

// CutStep loads the ruleset with client traffic cut, and RestoreStep loads the
// ordinary one.
//
// Both carry the SAME inverse as the firewall step that installed the table in
// the first place: delete it. There is no second teardown path, so the INVERSE
// direction is correct however a caller journals it: the entry already in the
// journal removes the table whatever state it is in.
//
// The FORWARD direction is not free of the journal, and an earlier version of
// this comment said it was. A caller that applies these through Applier.Apply
// gets the journal-aware skip, and until RunnerKey included standard input
// that skip could not tell a cut from the firewall load that installed the
// table: the argument vector is "nft -f -" for both and the whole difference
// is on stdin. MEASURED on hardware: the cut was applied that way, zero
// rulesets were loaded, success was reported and the box went on forwarding.
// See RunnerKey.
//
// The action that decides when to cut belongs to the privileged service. This
// is only the ruleset side.
func (p *Plan) CutStep() Step { return p.backend().CutStep(p) }

// linuxCutStep reloads the nftables ruleset with the forward accepts removed.
func (p *Plan) linuxCutStep() Step {
	why := "cut forwarded client traffic without taking the hotspot down, so a joined device stays " +
		"joined and can still reach the panel to turn it back on"
	return Step{
		Op:   OpNft,
		Why:  why,
		Do:   Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.RulesetFor(ForwardCut), Why: why},
		Undo: Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.RulesetTeardown(), Why: "remove the generated tables"},
	}
}

// RestoreStep puts forwarding back.
func (p *Plan) RestoreStep() Step { return p.backend().RestoreStep(p) }

// linuxRestoreStep reloads the normal nftables ruleset.
func (p *Plan) linuxRestoreStep() Step {
	why := "resume forwarding client traffic through the tunnel"
	return Step{
		Op:   OpNft,
		Why:  why,
		Do:   Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.RulesetFor(ForwardNormal), Why: why},
		Undo: Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.RulesetTeardown(), Why: "remove the generated tables"},
	}
}
