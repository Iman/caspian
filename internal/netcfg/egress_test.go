// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"strings"
	"testing"
)

// Established must be the FIRST rule, not the second.
//
// Every outbound reply to an inbound connection lives there, the
// administrator's SSH session included. A drop policy without it, or with it
// behind anything that could drop first, kills that session the moment the
// ruleset loads. This is the same failure as the INPUT policy that locked the
// box out, one chain over.
func TestRestrictedEgress_AcceptsEstablishedBeforeItDropsAnything(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			_, p := mustPlan(t, sc, DefaultOptions())
			out := chainBody(t, p.Ruleset(), "output")
			if len(out) < 2 {
				t.Fatalf("output chain too short: %v", out)
			}
			if out[0] != "type filter hook output priority filter; policy drop;" {
				t.Fatalf("output policy is %q, want drop", out[0])
			}
			if out[1] != "ct state established,related accept" {
				t.Fatalf("first rule is %q, want the established accept.\n"+
					"Anything before it can drop a reply to an inbound connection, "+
					"which ends the administrator's session as the ruleset loads.", out[1])
			}
			// And nothing may drop or reject ahead of it.
			for i, l := range out[1:] {
				if strings.Contains(l, " drop") || strings.Contains(l, " reject") {
					if i+1 < 1 {
						t.Errorf("rule %q drops before established is accepted", l)
					}
					break
				}
			}
		})
	}
}

// Every permit, with the measurement that justifies it.
func TestRestrictedEgress_PermitList(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	out := strings.Join(chainBody(t, p.Ruleset(), "output"), "\n")

	for _, want := range []string{
		"ct state established,related accept",
		`oifname "lo" accept`,
		`oifname "xray0" accept`,
		// DHCP in BOTH directions. The server half is the one that is not
		// covered by established.
		"udp sport 68 udp dport 67 accept",
		"udp sport 67 udp dport 68 accept",
		"udp dport 123 accept",
		"udp dport 53 accept",
		"tcp dport 53 accept",
		// IPv6 needs neighbour discovery to work at all.
		"meta nfproto ipv6 icmpv6 type { nd-neighbor-solicit",
	} {
		contains(t, out, want)
	}
}

// The DHCP server permit is the one that is easy to delete as a duplicate of
// DNS. It is not: DNS answers match the conntrack entry the query made, and a
// DHCP reply goes to a broadcast address or to a client with no address yet,
// so there is no tuple to match.
func TestRestrictedEgress_ExplainsWhyDHCPNeedsAPermitAndDNSDoesNot(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	rs := p.Ruleset()
	contains(t, rs, "share no tuple for conntrack to match")
	contains(t, rs, "not one of them gets an")
	contains(t, rs, "DNS needs no such line")
}

// The engine's connection is permitted by ADDRESS. Some transports are UDP on
// 443, so a TCP-only permit would break them silently.
func TestRestrictedEgress_ServerIsPermittedByAddressNotPort(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	out := chainBody(t, p.Ruleset(), "output")
	joined := strings.Join(out, "\n")

	contains(t, joined, "ip daddr 203.0.113.10 accept")

	// No rule may permit the server by port, in either protocol.
	for _, l := range out {
		if !strings.Contains(l, "203.0.113.10") {
			continue
		}
		if strings.Contains(l, "dport") || strings.Contains(l, "sport") {
			t.Errorf("the server permit is qualified by port: %q\n"+
				"Some transports are UDP on 443; a port-qualified permit breaks them silently.", l)
		}
	}
	// And the pinned host route names the same address, for the same reason.
	f, _ := mustPlan(t, pi5Captured(), DefaultOptions())
	contains(t, strings.Join(stepKeys(p.ServerRouteSteps()), "\n"), "203.0.113.10/32")
	_ = f
}

// The cost the user accepted has to be readable by the person who hits it.
func TestRestrictedEgress_StatesTheCostAndTheResidual(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	rs := p.Ruleset()

	// In the generated header.
	contains(t, rs, "egress=restricted")
	contains(t, rs, "apt update")
	// Both residuals, in the words they were agreed in.
	contains(t, rs, "still reach the network on port 53")
	contains(t, rs, "resolved in the clear on the local network")
	// And deliberately absent things named as decisions.
	contains(t, rs, "Deliberately absent: mDNS")

	// In the sentence the panel shows.
	e := p.Explain()
	contains(t, e, "cannot reach the internet directly")
	contains(t, e, "updating its software will not work")
}

// The way back, without a rebuild.
func TestEgressOpen_RestoresThePreviousBehaviour(t *testing.T) {
	o := DefaultOptions()
	o.Egress = EgressOpen
	_, p := mustPlan(t, pi5Captured(), o)
	out := chainBody(t, p.Ruleset(), "output")

	if out[0] != "type filter hook output priority filter; policy accept;" {
		t.Fatalf("output policy = %q, want accept under EgressOpen", out[0])
	}
	joined := strings.Join(out, "\n")
	notContains(t, joined, "udp dport 123 accept")
	notContains(t, joined, "ip daddr 203.0.113.10 accept")
	contains(t, p.Ruleset(), "egress=open")
	// And no misleading cost claim when there is no cost.
	notContains(t, p.Explain(), "updating its software")
}

// The cut: client traffic stops, everything a joined device needs to stay
// joined and reach the panel keeps working.
func TestForwardCut_StopsClientsAndKeepsThePanelReachable(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	cut := q.RulesetFor(ForwardCut)

	fwd := chainBody(t, cut, "forward")
	joinedFwd := strings.Join(fwd, "\n")
	// Nothing forwards.
	for _, l := range fwd {
		if strings.HasSuffix(l, "accept") || strings.Contains(l, " accept ") {
			t.Errorf("the forward chain still accepts something while cut: %q", l)
		}
	}
	// And the reason is visible to somebody reading the live ruleset.
	contains(t, joinedFwd, `iifname "wlan0" drop comment "client traffic cut by the user"`)
	contains(t, cut, "CLIENT TRAFFIC IS CUT")
	contains(t, cut, "forward=CUT by the user")

	// The device stays joined and can still reach the panel: all of that is
	// INPUT to the box, untouched by the cut.
	in := strings.Join(chainBody(t, cut, "input"), "\n")
	for _, want := range []string{
		`iifname "wlan0" udp dport 67 accept`,
		`iifname "wlan0" udp dport 53 accept`,
		`iifname "wlan0" tcp dport 8088 accept`,
	} {
		contains(t, in, want)
	}

	// The leak block does not go away because forwarding is cut.
	contains(t, joinedFwd, `iifname "wlan0" oifname "eth0" drop`)
}

// The cut is a flag on one ruleset, not a second one. The two states must
// differ in the forward chain and nowhere else, or they can drift.
func TestForwardCut_DiffersFromNormalOnlyInTheForwardChain(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}

	section := func(rs string) map[string][]string {
		out := map[string][]string{}
		cur := ""
		for _, l := range ruleLines(rs) {
			if strings.HasPrefix(l, "chain ") {
				cur = strings.TrimSuffix(strings.TrimPrefix(l, "chain "), " {")
				continue
			}
			if l == "}" {
				cur = ""
				continue
			}
			if cur != "" {
				out[cur] = append(out[cur], l)
			}
		}
		return out
	}
	normal := section(q.RulesetFor(ForwardNormal))
	cut := section(q.RulesetFor(ForwardCut))

	for _, chain := range []string{"input", "output", "prerouting", "postrouting"} {
		wantSequence(t, "chain "+chain+" must be identical in both states", cut[chain], normal[chain])
	}
	if strings.Join(cut["forward"], "\n") == strings.Join(normal["forward"], "\n") {
		t.Error("the forward chain is identical in both states, so the cut does nothing")
	}
}

// Both steps carry the same inverse as the firewall step that installed the
// table, so teardown does not grow a second path.
func TestCutAndRestoreSteps_ShareTheFirewallInverse(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	want := p.FirewallStep().Undo

	for name, s := range map[string]Step{"cut": p.CutStep(), "restore": p.RestoreStep()} {
		if s.Op != OpNft {
			t.Errorf("%s step op = %q, want %q", name, s.Op, OpNft)
		}
		if RunnerKey(s.Undo) != RunnerKey(want) || s.Undo.Stdin != want.Stdin {
			t.Errorf("%s step inverse differs from the firewall step's:\n got: %s\nwant: %s",
				name, s.Undo.Stdin, want.Stdin)
		}
	}
	if p.CutStep().Do.Stdin == p.RestoreStep().Do.Stdin {
		t.Error("cut and restore load the same ruleset")
	}
	contains(t, p.CutStep().Do.Stdin, "client traffic cut by the user")
	notContains(t, p.RestoreStep().Do.Stdin, "client traffic cut by the user")
}

// Nothing in this package writes the cut anywhere it would survive a restart.
func TestForwardCut_IsNotPersistedAnywhere(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	// The steps a plan generates never mention the cut: it is applied by a
	// caller at runtime, and the ruleset is regenerated from the plan on
	// every start.
	for _, s := range p.AllSteps(f.Sysctl) {
		if strings.Contains(s.Do.Stdin, "client traffic cut") {
			t.Errorf("a planned step installs the cut ruleset: %s", s.Op)
		}
	}
	if strings.Contains(p.Ruleset(), "client traffic cut") {
		t.Error("the default ruleset is the cut one")
	}
}
