// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"strings"
	"testing"
)

// Router advertisements are dropped on the hotspot whatever IPv6Policy says,
// and this test is the guard on that being unconditional.
//
// The reasoning, stated here because it is the one place someone will look
// before making it conditional again. Advertising a prefix is what lets a
// client give itself a routable IPv6 address. This box has no prefix to
// advertise: nothing in this repository sends a router advertisement, there is
// no radvd, internal/hotspot renders no dnsmasq ra- option, and
// hotspot.DNSConfig.Validate refuses a hotspot subnet that is not IPv4. So the
// drop costs nothing in either policy, and removing it under IPv6Forward would
// have made the ONE mechanism that stops a client autoconfiguring depend on a
// setting.
//
// It mattered in exactly one combination, and that combination is why this is
// a test rather than a comment: under EgressOpen the output chain policy is
// accept, so the explicit drop is the only thing in the ruleset that stops a
// router advertisement leaving the hotspot. IPv6Forward used to remove it.
func TestRuleset_RouterAdvertisementDropIsUnconditional(t *testing.T) {
	for _, v6 := range []struct {
		name string
		pol  IPv6Policy
	}{{"ipv6 blocked", IPv6Block}, {"ipv6 forwarded", IPv6Forward}} {
		for _, eg := range []struct {
			name string
			pol  EgressPolicy
		}{{"egress restricted", EgressRestricted}, {"egress open", EgressOpen}} {
			t.Run(v6.name+", "+eg.name, func(t *testing.T) {
				o := DefaultOptions()
				o.IPv6, o.Egress = v6.pol, eg.pol
				_, p := mustPlan(t, modeAScenario(), o)
				out := strings.Join(chainBody(t, p.Ruleset(), "output"), "\n")
				contains(t, out, `oifname "ap0" icmpv6 type nd-router-advert drop`)
			})
		}
	}
}

// The leak block is the first rule in the forward chain and matches no address
// family, so it covers IPv6 as well as IPv4 whatever IPv6Policy says.
//
// This is the rule the whole ruleset exists for, and the one an IPv6 feature is
// most likely to break by "tidying" it into a family-specific pair. An nfproto
// match here would mean client IPv6 reached the uplink the moment somebody
// turned the setting on.
func TestRuleset_TheLeakBlockCoversBothFamiliesWhateverTheIPv6Setting(t *testing.T) {
	for _, pol := range []IPv6Policy{IPv6Block, IPv6Forward} {
		o := DefaultOptions()
		o.IPv6 = pol
		_, p := mustPlan(t, modeAScenario(), o)
		body := chainBody(t, p.Ruleset(), "forward")

		var first string
		for _, line := range body {
			s := strings.TrimSpace(line)
			if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "type filter") {
				continue
			}
			first = s
			break
		}
		if first != `iifname "ap0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"` {
			t.Errorf("ipv6=%v: the first rule in the forward chain is %q, not the leak block", pol, first)
		}
		if strings.Contains(first, "nfproto") {
			t.Errorf("ipv6=%v: the leak block names an address family (%q), so it no longer covers both", pol, first)
		}
	}
}

// IPv6Forward flips the firewall and one sysctl and nothing else, and this
// records the four things it does NOT do so that the gap is a test result
// rather than a paragraph.
//
// Every one of them is needed before a client can have an IPv6 address that
// reaches the tunnel, and none of them is generated anywhere in this package:
//
//  1. no IPv6 address is put on the hotspot interface,
//  2. no IPv6 address is put on the tunnel device,
//  3. no IPv6 default route is installed into the tunnel table,
//  4. no IPv6 policy rule selects that table for client traffic.
//
// So IPv6Forward today permits traffic that cannot be addressed or routed. It
// is reachable from this package and NOT reachable from the product:
// internal/privsvc refuses any client-IPv6 policy but the blocking one, with a
// named fault. If that refusal is ever lifted, this test is what says which
// work has to land first.
func TestIPv6Forward_InstallsNoIPv6AddressingOrRouting(t *testing.T) {
	o := DefaultOptions()
	o.IPv6 = IPv6Forward
	f, p := mustPlan(t, modeAScenario(), o)

	var cmds []string
	for _, s := range p.AllSteps(f.Sysctl) {
		cmds = append(cmds, RunnerKey(s.Do))
	}
	all := strings.Join(cmds, "\n")

	// The pinned host route to an IPv6 server address is the one legitimate
	// use of "ip -6" here, and this scenario has an IPv4 server, so there is
	// none to allow for.
	for _, unwanted := range []string{"-6 route", "-6 rule", "-6 address", "-6 addr"} {
		if strings.Contains(all, unwanted) {
			t.Errorf("IPv6Forward now emits %q. If IPv6 addressing and routing have been built, "+
				"rewrite this test to assert what they do, and re-read the refusal in "+
				"internal/privsvc/validate.go before lifting it.", unwanted)
		}
	}
}
