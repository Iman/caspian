// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

// The pre-engine sequence, asserted exactly. Order is part of the contract:
// the firewall is first so there is never a moment with forwarding on and no
// block, conf.default.rp_filter is set before the engine creates the tunnel
// device that inherits it, and the pinned host route is in place before the
// engine opens its first connection.
func TestPreEngineSteps_ExactSequence(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	steps := p.PreEngineSteps(f.Sysctl)

	want := []string{
		"nft -f -",
		"sysctl -w net.ipv4.ip_forward=1",
		"sysctl -w net.ipv4.conf.all.rp_filter=2",
		"sysctl -w net.ipv4.conf.default.rp_filter=2",
		"sysctl -w net.ipv6.conf.all.forwarding=0",
		"iw phy phy0 interface add ap0 type __ap",
		// Immediately after the interface is created and BEFORE an address
		// goes on it. MEASURED on the target 2026-08-30: without this,
		// NetworkManager claimed the new device and the address was gone
		// 0.7 seconds later, and dnsmasq died with "Cannot assign requested
		// address".
		"nmcli device set ap0 managed no",
		// Up first, address second. The address used to go on while the
		// interface was still down, which is the state it was measured
		// disappearing from.
		"ip link set dev ap0 up",
		"ip address add 10.83.51.1/24 dev ap0",
		"ip route add 203.0.113.10/32 via 192.168.1.1 dev eth0 proto static metric 5",
	}
	wantSequence(t, "pre-engine steps", stepKeys(steps), want)
}

// Every pre-engine change has an inverse, and the one exception says so.
func TestPreEngineSteps_Inverses(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	steps := p.PreEngineSteps(f.Sysctl)

	want := []string{
		"nft -f -",
		"sysctl -w net.ipv4.ip_forward=0",
		"sysctl -w net.ipv4.conf.all.rp_filter=1",
		"sysctl -w net.ipv4.conf.default.rp_filter=1",
		"sysctl -w net.ipv6.conf.all.forwarding=0",
		"iw dev ap0 del",
		// The release of a CREATED interface has no inverse: the interface is
		// removed by the inverse above and a reboot destroys it outright, so
		// there is no device left to give management back to. The takeover's
		// release, on a device the user owns, keeps its inverse.
		"(no inverse)",
		// Bringing the link up has no inverse either, which it says in its
		// own comment.
		"(no inverse)",
		"ip address del 10.83.51.1/24 dev ap0",
		"ip route del 203.0.113.10/32 via 192.168.1.1 dev eth0 metric 5",
	}
	wantSequence(t, "pre-engine inverses", undoKeys(steps), want)
}

// A knob whose value could not be read gets no inverse rather than a guessed
// one, and the step says why. Writing back a value nobody measured leaves the
// machine in a state it was never in.
func TestSysctlStep_NoPreviousValueMeansNoInverse(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	steps := p.SysctlSteps(map[string]string{}) // nothing was read
	for _, s := range steps {
		if !s.Undo.IsZero() {
			t.Fatalf("step %s invented an inverse from no measurement", s.Do)
		}
		if !strings.Contains(s.Why, "no recorded inverse") {
			t.Errorf("step %s does not say it has no inverse: %q", s.Do, s.Why)
		}
	}
}

// The post-engine sequence names the tunnel device in every command, which is
// why it cannot run before the engine has created it.
func TestPostEngineSteps_ExactSequence(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	steps := p.PostEngineSteps(f.Sysctl)

	want := []string{
		"ip address add 198.18.51.1/30 dev xray0",
		"ip route add 10.83.51.0/24 dev ap0 scope link table 8410",
		"ip route add default dev xray0 proto static table 8410",
		"ip rule add from 10.83.51.0/24 lookup 8410 priority 8410",
		"ip rule add fwmark 0x20da lookup 8410 priority 8409",
	}
	wantSequence(t, "post-engine steps", stepKeys(steps), want)

	wantUndo := []string{
		"ip address del 198.18.51.1/30 dev xray0",
		"ip route del 10.83.51.0/24 dev ap0 table 8410",
		"ip route del default dev xray0 table 8410",
		"ip rule del from 10.83.51.0/24 lookup 8410 priority 8410",
		"ip rule del fwmark 0x20da lookup 8410 priority 8409",
	}
	wantSequence(t, "post-engine inverses", undoKeys(steps), wantUndo)
}

// Without a route for the hotspot subnet inside the tunnel table, traffic
// between two clients matches the policy rule, finds only the default route,
// and disappears into the tunnel.
func TestTunnelRouteSteps_HotspotSubnetRouteComesBeforeTheDefault(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	keys := stepKeys(p.TunnelRouteSteps())
	subnetAt, defaultAt := -1, -1
	for i, k := range keys {
		if strings.Contains(k, "route add 10.83.51.0/24") {
			subnetAt = i
		}
		if strings.Contains(k, "route add default") {
			defaultAt = i
		}
	}
	if subnetAt < 0 || defaultAt < 0 || subnetAt > defaultAt {
		t.Fatalf("hotspot subnet route at %d, default at %d, in %v", subnetAt, defaultAt, keys)
	}
}

// The pinned host route is the one that stops the engine reaching its own
// uplink through the tunnel it is building.
func TestServerRouteSteps_PinsThroughTheRealGateway(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	steps := p.ServerRouteSteps()
	if len(steps) != 1 {
		t.Fatalf("want one pinned route, got %v", stepKeys(steps))
	}
	got := RunnerKey(steps[0].Do)
	want := "ip route add 203.0.113.10/32 via 192.168.1.1 dev eth0 proto static metric 5"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
	contains(t, steps[0].Why, "loop")
}

// A point-to-point uplink has no gateway, so the route is written with a
// device and no "via". A route with a "via" and an invalid address would be a
// malformed command.
func TestServerRouteSteps_OnLinkUplink(t *testing.T) {
	s := modeAScenario()
	s.route = "scenario-ip-route-default-onlink.txt"
	f := s.facts(t, BaseSysctlKnobs())
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	got := RunnerKey(p.ServerRouteSteps()[0].Do)
	want := "ip route add 203.0.113.10/32 dev ppp0 proto static metric 5"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

// A server reachable only over IPv6, on a box with no IPv6 default route, is
// not a plan with a caveat: the engine cannot reach it at all, because a server
// outside the local network needs a default route. Refusing here is the
// difference between a plain sentence the user can act on and a box that
// reports connected and carries nothing.
//
// The captured target has no IPv6 default route, which is what made this path
// reachable in the first place; the authored mode A machine has one.
func TestPlanNetwork_RefusesAnIPv6OnlyServerWithNoIPv6Route(t *testing.T) {
	f := pi5Captured().facts(t, BaseSysctlKnobs())
	_, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("2001:db8::1")}, DefaultOptions())
	if !errors.Is(err, ErrServerFamilyUnreachable) {
		t.Fatalf("err = %v, want ErrServerFamilyUnreachable", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("refusal must carry wording for the panel")
	}
	contains(t, pe.UserMessage(), "IPv6")
	notContains(t, pe.UserMessage(), "default route")
	notContains(t, pe.UserMessage(), "host route")
}

// With one address in each family and no IPv6 route, the plan proceeds on the
// IPv4 address and records the other as unpinnable. That is a field rather
// than only a note, so a caller can branch on it instead of reading English.
func TestPlanNetwork_RecordsAnUnpinnableServerStructurally(t *testing.T) {
	f := pi5Captured().facts(t, BaseSysctlKnobs())
	v6 := netip.MustParseAddr("2001:db8::1")
	p, err := PlanNetwork(f, []netip.Addr{testServer, v6}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.UnpinnableServers) != 1 || p.UnpinnableServers[0] != v6 {
		t.Fatalf("UnpinnableServers = %v, want exactly [%v]", p.UnpinnableServers, v6)
	}
	// Only the address that can be pinned gets a route, and no malformed
	// route is emitted for the one that cannot.
	wantSequence(t, "pinned routes", stepKeys(p.ServerRouteSteps()), []string{
		"ip route add 203.0.113.10/32 via 10.0.0.1 dev eth0 proto static metric 5",
	})
	contains(t, strings.Join(p.Notes, "\n"), "no IPv6 default route")
}

// The positive IPv6 path, on the authored machine that has an IPv6 default.
// Without this fixture the pinning code for IPv6 has no coverage at all.
func TestServerRouteSteps_IPv6ServerWithAnIPv6Route(t *testing.T) {
	f := modeAScenario().facts(t, BaseSysctlKnobs())
	v6 := netip.MustParseAddr("2001:db8::1")
	p, err := PlanNetwork(f, []netip.Addr{v6}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.UnpinnableServers) != 0 {
		t.Fatalf("UnpinnableServers = %v, want none", p.UnpinnableServers)
	}
	wantSequence(t, "pinned routes", stepKeys(p.ServerRouteSteps()), []string{
		"ip -6 route add 2001:db8::1/128 via fe80::1 dev eth0 proto static metric 5",
	})
	wantSequence(t, "inverses", undoKeys(p.ServerRouteSteps()), []string{
		"ip -6 route del 2001:db8::1/128 via fe80::1 dev eth0 metric 5",
	})
}

func TestTunnelRouteSteps_SplitDefaultStrategy(t *testing.T) {
	o := DefaultOptions()
	o.Strategy = StrategySplitDefault
	_, p := mustPlan(t, modeAScenario(), o)

	want := []string{
		"ip route add 0.0.0.0/1 dev xray0 proto static",
		"ip route add 128.0.0.0/1 dev xray0 proto static",
	}
	wantSequence(t, "split default", stepKeys(p.TunnelRouteSteps()), want)

	// Two half defaults rather than replacing 0.0.0.0/0: the uplink's own
	// default is left alone, so teardown is a deletion and not a restoration.
	for _, s := range p.TunnelRouteSteps() {
		notContains(t, RunnerKey(s.Do), "0.0.0.0/0")
	}
}

func TestIPv6Forward_ChangesTheSysctls(t *testing.T) {
	o := DefaultOptions()
	o.IPv6 = IPv6Forward
	f, p := mustPlan(t, modeAScenario(), o)
	keys := strings.Join(stepKeys(p.SysctlSteps(f.Sysctl)), "\n")
	contains(t, keys, "net.ipv6.conf.all.forwarding=1")
	notContains(t, keys, "disable_ipv6")
}

// The knobs read during detection must be exactly the knobs the plan changes,
// and every one of them must be global.
//
// A knob naming an interface is the shape that produced three separate
// failures on the target: no measurable prior value, a write ordered before
// the interface exists, and a guessed inverse that would have turned
// reverse-path filtering off on the uplink. The guarantee never needed one:
// the kernel uses the maximum of conf.all and the per-interface value, and
// loose is the larger value, so conf.all decides the outcome alone.
func TestSysctlKnobs_AreGlobalAndFullyMeasured(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			f, p := mustPlan(t, sc, DefaultOptions())

			declared := map[string]bool{}
			for _, k := range p.SysctlKnobs() {
				declared[k] = true
				if named := interfaceNameIn(k, p); named != "" {
					t.Errorf("declared knob %q names the interface %q; see the note above Plan.SysctlKnobs", k, named)
				}
			}

			seen := 0
			for _, st := range p.AllSteps(f.Sysctl) {
				if st.Do.Path != BinSysctl {
					continue
				}
				if len(st.Do.Args) < 2 {
					t.Errorf("sysctl step has too few arguments: %v", st.Do.Args)
					continue
				}
				seen++
				knob, _, _ := strings.Cut(st.Do.Args[1], "=")
				if !declared[knob] {
					t.Errorf("step changes %q, which SysctlKnobs does not list, so its inverse is never measured", knob)
				}
				if named := interfaceNameIn(knob, p); named != "" {
					t.Errorf("step changes %q, which names the interface %q", knob, named)
				}
				if _, ok := f.Sysctl[knob]; !ok {
					t.Errorf("step changes %q, which the read never returned a value for", knob)
				}
				if st.Undo.IsZero() {
					t.Errorf("step changes %q with no recorded inverse", knob)
				}
			}
			if seen == 0 {
				t.Fatal("no sysctl step was generated at all")
			}
		})
	}
}

// interfaceNameIn reports which of the plan's interface names appears in a
// knob, or "" if none does.
func interfaceNameIn(knob string, p *Plan) string {
	for _, dev := range []string{p.Uplink, p.Hotspot, p.Tun, p.HotspotParent} {
		if dev == "" {
			continue
		}
		if strings.Contains(knob, "."+dev+".") {
			return dev
		}
	}
	return ""
}

// The write that could not have run at all.
//
// The generated sequence used to set net.ipv4.conf.ap0.rp_filter at step 6 and
// create ap0 at step 9. "sysctl -w" on a knob whose interface does not exist
// fails, sysctlStep passes no "-e", and Applier.Apply stops at the first
// failure, so the appliance would not have started on a box where the hotspot
// interface has to be created. No sysctl step may name an interface that a
// later step brings into existence.
func TestSysctlSteps_NeverPrecedeTheInterfaceTheyName(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			f, p := mustPlan(t, sc, DefaultOptions())
			steps := p.AllSteps(f.Sysctl)

			created := map[string]int{}
			for i, st := range steps {
				if st.Do.Path == BinIw && len(st.Do.Args) >= 5 && st.Do.Args[2] == "interface" && st.Do.Args[3] == "add" {
					created[st.Do.Args[4]] = i
				}
			}
			for i, st := range steps {
				if st.Do.Path != BinSysctl || len(st.Do.Args) < 2 {
					continue
				}
				for dev, at := range created {
					if strings.Contains(st.Do.Args[1], "."+dev+".") && i < at {
						t.Errorf("step %d writes %q but %s is not created until step %d; the write fails and Apply stops there",
							i, st.Do.Args[1], dev, at)
					}
				}
			}
		})
	}
}

// The guarantee itself, stated as a property rather than as a list of writes:
// conf.all must be set to loose, because that is what pins every interface.
func TestSysctlSteps_ConfAllIsSetToLoose(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	found := false
	for _, st := range p.AllSteps(f.Sysctl) {
		if RunnerKey(st.Do) == "sysctl -w net.ipv4.conf.all.rp_filter=2" {
			found = true
		}
	}
	if !found {
		t.Error("conf.all.rp_filter is not set to 2 (loose); nothing else guarantees loose mode on the tunnel path")
	}
	// 2 is loose and 1 is strict, and the kernel takes the maximum, so any
	// value below 2 leaves an interface holding 1 in strict mode.
	if p.Opts.RPFilter != 2 {
		t.Errorf("RPFilter = %d; only 2 pins every interface to loose under the maximum rule", p.Opts.RPFilter)
	}
}

// Every routing rule this package generates must carry an explicit priority,
// on the add AND on the inverse.
//
// This is the guarded invariant that replaced a defensive query. Measured on
// the target on 2026-08-30: the kernel REFUSES an identical rule at the same
// explicit priority with "File exists", so a re-apply cannot duplicate one.
// Duplicates arise only where the priority is omitted, because the kernel then
// assigns one and two adds become two rules at different priorities. Which
// rule wins is then a matter of order, and nothing announces it: no error, no
// log line, just traffic matched against a different table than intended.
//
// The inverse needs it just as much. "ip rule del" without a priority removes
// the first rule matching the selector, which on a box where somebody else
// installed a similar rule is a rule this package never added.
//
// A query before each add would have defended the same thing at runtime, on a
// shape the generator cannot currently produce. This fails in a test run
// instead, which is earlier and cheaper.
func TestRuleSteps_AlwaysCarryAnExplicitPriority(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			f, p := mustPlan(t, sc, DefaultOptions())

			checked := 0
			check := func(what string, c Command) {
				if c.IsZero() || c.Path != BinIP {
					return
				}
				if len(c.Args) == 0 || c.Args[0] != "rule" {
					return
				}
				checked++
				prio := ""
				for i, a := range c.Args {
					if a == "priority" && i+1 < len(c.Args) {
						prio = c.Args[i+1]
					}
				}
				if prio == "" {
					t.Errorf("%s has no explicit priority: %s\n"+
						"Without one the kernel assigns it, two adds become two rules, and "+
						"which table wins changes with nothing to announce it.", what, RunnerKey(c))
					return
				}
				if _, err := strconv.Atoi(prio); err != nil {
					t.Errorf("%s has a non-numeric priority %q: %s", what, prio, RunnerKey(c))
				}
			}

			for _, s := range p.AllSteps(f.Sysctl) {
				check("rule step", s.Do)
				check("rule inverse", s.Undo)
			}
			// Re-derivation after an uplink change generates rules too.
			_, redo, _, _ := p.RederiveForUplink(UplinkState{
				Interface: "usb0", Gateway: netip.MustParseAddr("192.168.42.1"),
			})
			for _, s := range redo {
				check("re-derived rule step", s.Do)
				check("re-derived rule inverse", s.Undo)
			}

			if checked == 0 {
				t.Fatal("no rule commands were examined, so this test is guarding nothing")
			}
		})
	}
}
