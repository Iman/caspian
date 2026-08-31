// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"regexp"
	"strings"
	"testing"
)

// ruleLines returns the ruleset with comment lines and blank lines removed, so
// assertions are about rules and never about prose.
func ruleLines(ruleset string) []string {
	var out []string
	for _, l := range strings.Split(ruleset, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, t)
	}
	return out
}

func chainBody(t *testing.T, ruleset, chain string) []string {
	t.Helper()
	var out []string
	in := false
	for _, l := range ruleLines(ruleset) {
		if l == "chain "+chain+" {" {
			in = true
			continue
		}
		if in {
			if l == "}" {
				return out
			}
			out = append(out, l)
		}
	}
	t.Fatalf("chain %q not found in ruleset:\n%s", chain, ruleset)
	return nil
}

// THE fail-closed test.
//
// If the tunnel device disappears the kernel withdraws every route through
// it and client traffic falls back to the main table and out of the uplink, so
// "the engine stopped" leaks by default. Removing every rule that names the
// tunnel models exactly that, and what is left must still block.
func TestRuleset_StillBlocksWhenTheTunnelIsAbsent(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			_, p := mustPlan(t, sc, DefaultOptions())
			full := p.Ruleset()
			absent := stripInterfaceRules(full, p.Tun)

			if strings.Contains(absent, "\""+p.Tun+"\"") {
				t.Fatal("the model of an absent tunnel still mentions it")
			}

			fwd := chainBody(t, absent, "forward")
			if len(fwd) == 0 {
				t.Fatal("forward chain is empty with the tunnel absent")
			}
			if fwd[0] != "type filter hook forward priority filter; policy drop;" {
				t.Errorf("forward chain does not open with a drop policy: %q", fwd[0])
			}

			// The leak block itself must survive, and it must name only the
			// hotspot and the uplink.
			wantBlock := `iifname "` + p.Hotspot + `" oifname "` + p.Uplink +
				`" drop comment "fail-closed: client traffic never leaves by the uplink"`
			found := false
			for _, l := range fwd {
				if l == wantBlock {
					found = true
				}
			}
			if !found {
				t.Errorf("the leak block did not survive the tunnel's absence.\nwant: %s\ngot forward chain:\n  %s",
					wantBlock, strings.Join(fwd, "\n  "))
			}

			// Nothing that is left may accept forwarded client traffic.
			for _, l := range fwd {
				if strings.HasSuffix(l, "accept") || strings.Contains(l, " accept ") {
					t.Errorf("with the tunnel absent the forward chain still accepts something: %q", l)
				}
			}
		})
	}
}

// Interface matching must be by name and never by index. Index matching is
// resolved when the ruleset loads, so a ruleset naming the tunnel by index
// cannot be loaded while the tunnel is down, which is exactly when it has to
// be in force.
func TestRuleset_MatchesInterfacesByNameNeverByIndex(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	byIndex := regexp.MustCompile(`(^|\s)(iif|oif)\s`)
	for _, l := range ruleLines(p.Ruleset()) {
		if byIndex.MatchString(l) {
			t.Errorf("rule matches an interface by index, which cannot load while that interface is absent: %q", l)
		}
	}
	// And the name form is actually used.
	contains(t, p.Ruleset(), `iifname "ap0"`)
	contains(t, p.Ruleset(), `oifname "eth0"`)
}

// A masquerade towards the uplink is the single line that would quietly turn
// the appliance into an ordinary router.
func TestRuleset_NeverMasqueradesToTheUplink(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	for _, l := range ruleLines(p.Ruleset()) {
		if strings.Contains(l, "masquerade") {
			t.Errorf("unexpected masquerade with the default options: %q", l)
		}
		if strings.Contains(l, "snat") {
			t.Errorf("unexpected source NAT: %q", l)
		}
	}

	o := DefaultOptions()
	o.MasqueradeToTunnel = true
	_, q := mustPlan(t, modeAScenario(), o)
	for _, l := range ruleLines(q.Ruleset()) {
		if strings.Contains(l, "masquerade") && strings.Contains(l, `"`+q.Uplink+`"`) {
			t.Errorf("masquerade towards the uplink: %q", l)
		}
	}
	contains(t, q.Ruleset(), `oifname "xray0" masquerade`)
}

// The INPUT policy is stated rather than inherited, because a router's own
// traffic is INPUT and OUTPUT and the FORWARD policy says nothing about it.
func TestRuleset_StatesEveryChainPolicy(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	rs := p.Ruleset()
	contains(t, rs, "type filter hook input priority filter; policy accept;")
	contains(t, rs, "type filter hook forward priority filter; policy drop;")
	// Output is drop by default now: the kill switch covers this box too.
	contains(t, rs, "type filter hook output priority filter; policy drop;")

	// And the way back is a config change, not a rebuild.
	o := DefaultOptions()
	o.Egress = EgressOpen
	_, q := mustPlan(t, pi5Captured(), o)
	contains(t, q.Ruleset(), "type filter hook output priority filter; policy accept;")
	contains(t, rs, "type nat hook prerouting priority dstnat; policy accept;")
	contains(t, rs, "type nat hook postrouting priority srcnat; policy accept;")
}

// A router's own traffic is INPUT and OUTPUT, not FORWARD, so the tunnel needs
// a permit in both.
func TestRuleset_PermitsTheTunnelInBothInputAndOutput(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	in := strings.Join(chainBody(t, p.Ruleset(), "input"), "\n")
	out := strings.Join(chainBody(t, p.Ruleset(), "output"), "\n")
	contains(t, in, `iifname "xray0" accept`)
	contains(t, out, `oifname "xray0" accept`)
}

// Client DNS is redirected, not merely allowed: a client with a resolver
// hardcoded into it must be answered here rather than let out to reach the one
// it was told to use.
func TestRuleset_RedirectsClientDNSRatherThanAllowingIt(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	pre := strings.Join(chainBody(t, p.Ruleset(), "prerouting"), "\n")
	contains(t, pre, `iifname "ap0" udp dport 53 redirect to :53`)
	contains(t, pre, `iifname "ap0" tcp dport 53 redirect to :53`)

	// Encrypted DNS on 853 would carry queries past the resolver on the box.
	fwd := strings.Join(chainBody(t, p.Ruleset(), "forward"), "\n")
	contains(t, fwd, `iifname "ap0" tcp dport 853 reject with tcp reset`)
	contains(t, fwd, `iifname "ap0" udp dport 853 drop`)

	// No rule anywhere may simply accept client DNS towards the uplink.
	for _, l := range ruleLines(p.Ruleset()) {
		if strings.Contains(l, "dport 53 ") && strings.Contains(l, "accept") && strings.Contains(l, `"eth0"`) {
			t.Errorf("client DNS accepted towards the uplink: %q", l)
		}
	}
}

func TestRuleset_RedirectsToACustomResolverPort(t *testing.T) {
	o := DefaultOptions()
	o.DNSPort = 5353
	_, p := mustPlan(t, modeAScenario(), o)
	pre := strings.Join(chainBody(t, p.Ruleset(), "prerouting"), "\n")
	contains(t, pre, `iifname "ap0" udp dport 53 redirect to :5353`)
	// The input chain must accept the port the redirect lands on, not 53,
	// which the client no longer reaches.
	in := strings.Join(chainBody(t, p.Ruleset(), "input"), "\n")
	contains(t, in, `iifname "ap0" udp dport 5353 accept`)
}

// The nft "inet" family covers both IPv4 and IPv6, but a rule that does not
// say which family it means covers both by accident rather than on purpose.
// IPv6 gets its own rules because there is no IPv6 tunnel to carry it.
func TestRuleset_CoversIPv6WithItsOwnRules(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	fwd := strings.Join(chainBody(t, p.Ruleset(), "forward"), "\n")
	contains(t, fwd, `meta nfproto ipv6 iifname "ap0" drop`)
	contains(t, fwd, `meta nfproto ipv6 oifname "ap0" drop`)

	// A client that autoconfigures IPv6 would prefer it over the tunnelled
	// IPv4, so the box must never advertise a prefix on the hotspot.
	out := strings.Join(chainBody(t, p.Ruleset(), "output"), "\n")
	contains(t, out, `oifname "ap0" icmpv6 type nd-router-advert drop`)

	// The box's own IPv6 keeps working, and now needs no rule to say so: the
	// input policy is accept, so nothing about the box's own traffic depends
	// on a permit that could be forgotten.
	in := strings.Join(chainBody(t, p.Ruleset(), "input"), "\n")
	notContains(t, in, `iifname "eth0"`)
}

func TestRuleset_IPv6ForwardAddsTheTunnelPermits(t *testing.T) {
	o := DefaultOptions()
	o.IPv6 = IPv6Forward
	_, p := mustPlan(t, modeAScenario(), o)
	fwd := strings.Join(chainBody(t, p.Ruleset(), "forward"), "\n")
	contains(t, fwd, `meta nfproto ipv6 iifname "ap0" oifname "xray0" accept`)
	notContains(t, fwd, `meta nfproto ipv6 iifname "ap0" drop`)
}

// The uplink side is the owner's own network and the owner's own machine, and
// this appliance does not firewall it.
//
// MEASURED on the target on 2026-08-30: with the previous policy drop and no
// uplink accepts, every new inbound connection to the box was dropped the
// moment the ruleset loaded and SSH stopped answering. Established connections
// kept working, so the box looked healthy from the session already open while
// being unreachable to every new one. On a headless machine the remaining
// recovery is a power cycle and a card reader.
//
// The appliance's job is forwarded client traffic. Closing the owner's
// administrative access is a decision it has no business making for them, and
// making it silently is worse than making it loudly.
func TestRuleset_NeverDropsNewInboundOnTheUplink(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			_, p := mustPlan(t, sc, DefaultOptions())
			in := chainBody(t, p.Ruleset(), "input")
			if len(in) == 0 {
				t.Fatal("input chain is empty")
			}
			if in[0] != "type filter hook input priority filter; policy accept;" {
				t.Fatalf("input policy is %q; anything but accept closes the owner's access to their own box", in[0])
			}
			for _, l := range in[1:] {
				if !strings.Contains(l, " drop") && !strings.Contains(l, " reject") {
					continue
				}
				// Out-of-state traffic is not new inbound, so this one is
				// allowed to be unqualified.
				if strings.HasPrefix(l, "ct state invalid drop") {
					continue
				}
				// Everything else that drops must be confined to the hotspot,
				// where the untrusted devices are.
				if !strings.Contains(l, `iifname "`+p.Hotspot+`"`) {
					t.Errorf("input chain drops traffic that is not qualified to the hotspot: %q\n"+
						"On the uplink this silently closes the owner's access to their own machine.", l)
				}
			}
			// And nothing in this chain may name the uplink at all.
			for _, l := range in {
				if strings.Contains(l, `"`+p.Uplink+`"`) {
					t.Errorf("input chain names the uplink: %q", l)
				}
			}
		})
	}
}

// The hotspot side is where the untrusted devices are, and it is the only
// place this chain restricts anything.
func TestRuleset_HotspotClientsReachOnlyWhatTheBoxServes(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	in := chainBody(t, p.Ruleset(), "input")
	joined := strings.Join(in, "\n")

	for _, want := range []string{
		`iifname "ap0" udp dport 67 accept`,
		`iifname "ap0" udp dport 53 accept`,
		`iifname "ap0" tcp dport 53 accept`,
		`iifname "ap0" tcp dport 8088 accept`,
	} {
		contains(t, joined, want)
	}

	// The catch-all drop must come after every hotspot accept, or the
	// services the box offers are unreachable.
	dropAt, lastAccept := -1, -1
	for i, l := range in {
		if !strings.Contains(l, `iifname "ap0"`) {
			continue
		}
		if strings.Contains(l, " drop") {
			dropAt = i
		} else if strings.Contains(l, " accept") {
			lastAccept = i
		}
	}
	if dropAt < 0 {
		t.Fatal("nothing stops a joined device reaching the rest of the box")
	}
	if lastAccept > dropAt {
		t.Errorf("a hotspot accept at %d comes after the catch-all drop at %d, so that service is unreachable", lastAccept, dropAt)
	}
}

func TestRuleset_ClientIsolation(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	contains(t, strings.Join(chainBody(t, p.Ruleset(), "forward"), "\n"), `iifname "ap0" oifname "ap0" drop`)

	o := DefaultOptions()
	o.ClientIsolation = false
	_, q := mustPlan(t, modeAScenario(), o)
	notContains(t, strings.Join(chainBody(t, q.Ruleset(), "forward"), "\n"), `iifname "ap0" oifname "ap0" drop`)
}

// The leak block must be the first rule, so nothing added below it can take
// precedence.
func TestRuleset_LeakBlockIsFirstInForward(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	fwd := chainBody(t, p.Ruleset(), "forward")
	if len(fwd) < 2 {
		t.Fatalf("forward chain too short: %v", fwd)
	}
	if !strings.HasPrefix(fwd[1], `iifname "ap0" oifname "eth0" drop`) {
		t.Errorf("first rule after the policy is %q, want the leak block", fwd[1])
	}
}

// Every header field must be introduced by its own protocol keyword.
//
// "udp sport 67 dport 68" reads to a person and nft rejects it: the second
// field has no base expression to resolve against, so it fails with "No symbol
// type information". The refusal is at load time and it fails the WHOLE
// ruleset, so the box is left with no firewall rather than a partial one, and
// nothing in a pure Go test can see it. This walks every generated rule and
// checks the shape directly.
func TestRuleset_EveryHeaderFieldHasItsOwnProtocolKeyword(t *testing.T) {
	// Fields that are meaningless without a base expression immediately
	// before them, and the bases that can introduce one.
	fields := map[string]bool{
		"sport": true, "dport": true, "saddr": true, "daddr": true,
		"type": true, "state": true, "protocol": true, "nfproto": true,
	}
	bases := map[string]bool{
		"udp": true, "tcp": true, "sctp": true, "udplite": true, "th": true,
		"ip": true, "ip6": true, "icmp": true, "icmpv6": true,
		"ct": true, "meta": true,
	}

	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			_, p := mustPlan(t, sc, DefaultOptions())
			for _, line := range ruleLines(p.Ruleset()) {
				// A chain declaration is not a rule, and its leading "type"
				// is the chain type rather than a header field.
				if strings.HasPrefix(line, "type ") && strings.Contains(line, " hook ") {
					continue
				}
				// Stop at a comment so its prose is not scanned as tokens.
				if i := strings.Index(line, " comment "); i >= 0 {
					line = line[:i]
				}
				tok := strings.Fields(line)
				for i, tk := range tok {
					if !fields[tk] {
						continue
					}
					if i == 0 || !bases[tok[i-1]] {
						prev := "(start of rule)"
						if i > 0 {
							prev = tok[i-1]
						}
						t.Errorf("rule %q: field %q follows %q, which is not a protocol keyword; "+
							"nft cannot resolve it and refuses the whole ruleset", line, tk, prev)
					}
				}
			}
		})
	}
}

// The load is idempotent. The bare declaration creates the table if it is
// absent so the delete that follows cannot fail, which means loading twice, or
// tearing down a machine where nothing was ever loaded, both succeed.
func TestRuleset_LoadAndTeardownAreIdempotent(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	rs := p.Ruleset()
	lines := ruleLines(rs)
	if len(lines) < 2 {
		t.Fatalf("ruleset has %d rule lines", len(lines))
	}
	if lines[0] != "table inet caspian" || lines[1] != "delete table inet caspian" {
		t.Fatalf("ruleset does not open with the create-then-delete idiom: %q, %q", lines[0], lines[1])
	}
	td := ruleLines(p.RulesetTeardown())
	wantSequence(t, "teardown", td, []string{"table inet caspian", "delete table inet caspian"})
}

// One table, so teardown is one delete and a half-removed firewall is not a
// state the machine can be in.
func TestRuleset_UsesOneTable(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	n := 0
	for _, l := range ruleLines(p.Ruleset()) {
		if strings.HasPrefix(l, "table ") && strings.HasSuffix(l, "{") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("ruleset defines %d tables, want 1", n)
	}
}

// The firewall step names no interface in a way that requires it to exist, so
// unlike every routing step it can be applied before the engine starts.
func TestFirewallStep_IsInThePreEngineList(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	pre := p.PreEngineSteps(f.Sysctl)
	if len(pre) == 0 {
		t.Fatal("no pre-engine steps generated")
	}
	if pre[0].Op != OpNft {
		t.Fatalf("first pre-engine step is %q, want the firewall", pre[0].Op)
	}
	for _, s := range p.PostEngineSteps(f.Sysctl) {
		if s.Op == OpNft {
			t.Error("the firewall must not be in the post-engine list; it must be in force before the engine starts")
		}
	}
}
