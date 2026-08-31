// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests written during the 2026-08-30 external-findings audit. They are
// EVIDENCE, not fixes: nothing in production changed. Delete them if the audit
// is rejected.

// TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed.
//
// THE fail-closed teardown test, and the mirror of
// TestRuleset_StillBlocksWhenTheTunnelIsAbsent: that one proves the ruleset
// still blocks with the tunnel gone, this one proves the ruleset is still
// THERE when the machine is still carrying what it blocks.
//
// MEASURED on 2026-08-30, before the two-pass replay existed: with every "ip"
// and "sysctl" inverse refusing, the table was deleted anyway, leaving an
// access point beaconing, ip_forward at 1, the hotspot address in place, the
// policy rule and the tunnel routes installed, and no block. That is a leak,
// and it was the only path in this design known to produce one.
func TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed(t *testing.T) {
	ctx := context.Background()
	sc := modeAScenario()
	facts, plan := mustPlan(t, sc, DefaultOptions())

	// A permissive machine: every change succeeds. The scenario's own runner
	// is deliberately strict about the sysctl READ and refuses to answer a
	// sysctl WRITE at all, so it cannot stand in for the apply path here.
	r := NewRecordingRunner()
	r.Fallback = func(Command) (Result, error) { return Result{}, nil }

	journal := filepath.Join(t.TempDir(), "netcfg.journal")
	ap, err := NewApplier(r, journal)
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	steps := plan.PreEngineSteps(facts.Sysctl)
	if _, err := ap.Apply(ctx, steps); err != nil {
		t.Fatalf("applying the pre-engine steps: %v", err)
	}

	// One inverse refuses, and it is not the firewall's. The wording is
	// deliberately not one of notFoundMarkers, so it counts as a real failure
	// rather than as "already gone".
	refused := errors.New("RTNETLINK answers: Operation not permitted")
	var held string
	for _, s := range steps {
		if s.Undo.IsZero() || s.Op == OpNft {
			continue
		}
		held = RunnerKey(s.Undo)
		r.SetError(held, refused)
		break
	}
	if held == "" {
		t.Fatal("no non-firewall inverse to fail, so this test proves nothing")
	}

	before := len(r.Commands())
	rep, err := ap.Teardown(ctx)
	if err != nil {
		t.Fatalf("teardown: %v", err)
	}

	// The firewall's inverse was never run.
	for _, c := range r.Commands()[before:] {
		if c.Path == BinNft {
			t.Fatalf("the generated table was deleted while %q was still in force. "+
				"A machine still carrying routes, addresses or forwarding was left with no block",
				held)
		}
	}

	// It is reported as held, not as failed, because the two need opposite
	// responses from whoever reads the report.
	var heldReason string
	for _, res := range rep.Results {
		if res.Step.Op == OpNft {
			if res.Err != nil {
				t.Fatalf("the firewall inverse is reported as failed; it was never attempted: %v", res.Err)
			}
			if !res.Skipped {
				t.Fatal("the firewall inverse is reported as neither run, skipped, nor failed")
			}
			heldReason = res.Reason
		}
	}
	if !strings.Contains(heldReason, "held") {
		t.Fatalf("the report does not say the firewall was held and why: %q", heldReason)
	}

	// And it stays in the journal, so the next start retries it rather than
	// the block being stranded for ever.
	left, err := LoadJournal(journal)
	if err != nil {
		t.Fatalf("reading the journal: %v", err)
	}
	var kept []string
	for _, e := range left {
		kept = append(kept, e.Op)
	}
	foundNft := false
	for _, op := range kept {
		if op == OpNft {
			foundNft = true
		}
	}
	if !foundNft {
		t.Fatalf("the firewall inverse was held but dropped from the journal, so nothing will ever "+
			"remove the table. journal holds: %v", kept)
	}
}

// TestAHeldFirewallIsReplayedOnceTheRestSucceeds proves the hold converges.
//
// A rule that keeps the block whenever anything failed would be a rule that
// can strand a table on the machine for ever. It does not: the entries that
// failed stay in the journal beside the firewall's, the next replay retries
// them, and the moment that sweep is clean the table comes out.
func TestAHeldFirewallIsReplayedOnceTheRestSucceeds(t *testing.T) {
	ctx := context.Background()
	sc := modeAScenario()
	facts, plan := mustPlan(t, sc, DefaultOptions())

	r := NewRecordingRunner()
	r.Fallback = func(Command) (Result, error) { return Result{}, nil }
	journal := filepath.Join(t.TempDir(), "netcfg.journal")
	ap, err := NewApplier(r, journal)
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	steps := plan.PreEngineSteps(facts.Sysctl)
	if _, err := ap.Apply(ctx, steps); err != nil {
		t.Fatalf("apply: %v", err)
	}

	refused := errors.New("RTNETLINK answers: Operation not permitted")
	var held string
	for _, s := range steps {
		if s.Undo.IsZero() || s.Op == OpNft {
			continue
		}
		held = RunnerKey(s.Undo)
		r.SetError(held, refused)
		break
	}
	if _, err := ap.Teardown(ctx); err != nil {
		t.Fatalf("first teardown: %v", err)
	}

	// The obstruction clears, which is what a next start finds.
	delete(r.Errors, held)
	r.Reset()

	rep, err := Recover(ctx, r, journal)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if rep.Failed != 0 {
		t.Fatalf("the retry still reports %d failures", rep.Failed)
	}
	sawNft := false
	for _, c := range r.Commands() {
		if c.Path == BinNft {
			sawNft = true
		}
	}
	if !sawNft {
		t.Fatal("the held firewall inverse was never retried, so the table is stranded on the machine")
	}
	if _, statErr := os.Stat(journal); !errors.Is(statErr, os.ErrNotExist) {
		left, _ := LoadJournal(journal)
		t.Fatalf("the journal survived a clean sweep with %d entries", len(left))
	}
}

// TestTeardownReportsFailedInversesInItsReportAndNotInItsError.
//
// Applier.Teardown returns a non-nil error only when the journal FILE cannot be
// closed or rewritten. A replay in which inverses failed returns (rep, nil),
// and the count lives in the report.
//
// This is not a defect, it is the contract, and it is pinned because a caller
// that checks only the error believes the machine was restored. One did:
// internal/privsvc's applyPreEngine, under a comment asserting that "a teardown
// that failed returns above and never reaches here". Its guard now reads
// rep.Failed as well, and TestTheFallbackIsRefusedWhenTheFirstPlanCouldNotBeUndone
// is the guard on that side.
func TestTeardownReportsFailedInversesInItsReportAndNotInItsError(t *testing.T) {
	ctx := context.Background()
	sc := modeAScenario()
	facts, plan := mustPlan(t, sc, DefaultOptions())

	r := NewRecordingRunner()
	r.Fallback = func(Command) (Result, error) { return Result{}, nil }
	ap, err := NewApplier(r, filepath.Join(t.TempDir(), "netcfg.journal"))
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	steps := plan.PreEngineSteps(facts.Sysctl)
	if _, err := ap.Apply(ctx, steps); err != nil {
		t.Fatalf("apply: %v", err)
	}

	refused := errors.New("RTNETLINK answers: Operation not permitted")
	inverses := 0
	for _, s := range steps {
		if s.Undo.IsZero() || s.Op == OpNft {
			continue
		}
		r.SetError(RunnerKey(s.Undo), refused)
		inverses++
	}
	if inverses == 0 {
		t.Fatal("no inverse to fail, so this test proves nothing")
	}

	rep, tErr := ap.Teardown(ctx)
	if tErr != nil {
		t.Fatalf("Teardown returned an error for failed inverses (%v). If that is now the "+
			"contract, internal/privsvc's applyPreEngine can go back to checking the error alone", tErr)
	}
	if rep.Failed != inverses {
		t.Fatalf("the report says %d of %d inverses failed", rep.Failed, inverses)
	}
}

// TestRulesetStillBlocksWhenTheUplinkIsRenamed.
//
// The mirror of TestRuleset_StillBlocksWhenTheTunnelIsAbsent, aimed at the
// OTHER interface. docs/BEHAVIOUR.md, "a change of uplink moves the pinned
// route and the block with it", says: "the block names the interface the
// internet arrives on, so a ruleset still naming the old one stops blocking the
// moment traffic starts leaving by the new one." internal/netcfg/uplink.go's
// RederiveForUplink doc comment says the same.
//
// Removing every rule that names the uplink models a ruleset whose uplink name
// no longer matches anything, which is exactly what an uplink that moved to a
// different interface produces. If those two sentences were right, what is left
// would permit forwarded client traffic. It does not.
func TestRulesetStillBlocksWhenTheUplinkIsRenamed(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			_, p := mustPlan(t, sc, DefaultOptions())
			absent := stripInterfaceRules(p.Ruleset(), p.Uplink)

			if strings.Contains(absent, "\""+p.Uplink+"\"") {
				t.Fatal("the model of a renamed uplink still mentions the old name")
			}

			fwd := chainBody(t, absent, "forward")
			if len(fwd) == 0 {
				t.Fatal("forward chain is empty with the uplink absent")
			}
			if fwd[0] != "type filter hook forward priority filter; policy drop;" {
				t.Errorf("forward chain does not open with a drop policy: %q", fwd[0])
			}
			for _, l := range fwd {
				if strings.HasSuffix(l, "accept") || strings.Contains(l, " accept ") {
					// An accept is only safe if it names the tunnel, which no
					// packet heading for a NEW uplink can match.
					if !strings.Contains(l, "\""+p.Tun+"\"") {
						t.Errorf("with the uplink absent the forward chain accepts something that does not name the tunnel: %q", l)
					}
				}
			}
			t.Logf("EVIDENCE: with every rule naming %q removed, the forward chain is still policy drop "+
				"and its only accepts name %q. Client traffic leaving by a DIFFERENT interface is dropped "+
				"by the policy, so the leak block is redundant and the quoted sentence is wrong.",
				p.Uplink, p.Tun)
		})
	}
}

// TestTheHotspotInterfaceIsReleasedFromNetworkManagerOnEveryPathThatNamesOne.
//
// The 2026-08-30 incident this package spent a day on is NetworkManager acting
// on an interface the appliance is serving on. The answer, "nmcli device set
// <iface> managed no", used to be emitted only when Plan.HotspotTakenOver was
// true, so the two paths that name an interface WITHOUT taking it over started
// hostapd on an interface NetworkManager still held. Mode B, a USB adapter, is
// the configuration this product tells people to buy.
//
// This pins both halves of the split in HotspotReleaseSteps:
//
//   - the RELEASE follows the measured manager, on any path naming an
//     interface that already exists
//   - the STATION TEARDOWN, which strips addresses and changes the interface
//     type, stays with the takeover, because there is no station link to undo
//     anywhere else
func TestTheHotspotInterfaceIsReleasedFromNetworkManagerOnEveryPathThatNamesOne(t *testing.T) {
	// NetworkManager holds every wireless interface, including the idle USB
	// adapter mode B chooses. "disconnected" is the state that matters: it is
	// not in use, and NetworkManager can still connect it at any moment.
	const nmcliHoldsEverything = "eth0:connected\nwlan0:connected\nwlan1:disconnected\n"

	for _, sc := range []scenario{modeBScenario()} {
		t.Run(sc.name+"/free interface used directly", func(t *testing.T) {
			r := sc.runner(t)
			r.SetOutput("nmcli -t -f DEVICE,STATE device status", nmcliHoldsEverything)
			f, err := Detect(context.Background(), r, BaseSysctlKnobs())
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if p.HotspotTakenOver || p.HotspotIsVirtual {
				t.Fatalf("this scenario no longer exercises the free-interface path "+
					"(taken over=%v virtual=%v)", p.HotspotTakenOver, p.HotspotIsVirtual)
			}
			if p.HotspotManager != ManagedByNetworkManager {
				t.Fatalf("the measured manager was dropped on the way into the plan: "+
					"Plan.HotspotManager is %q for %s, which detection reported as NetworkManager. "+
					"hostapd would be started on an interface NetworkManager still holds",
					p.HotspotManager, p.Hotspot)
			}

			want := "nmcli device set " + p.Hotspot + " managed no"
			var got []string
			for _, s := range p.PreEngineSteps(f.Sysctl) {
				got = append(got, RunnerKey(s.Do))
			}
			found := false
			for _, k := range got {
				if k == want {
					found = true
				}
			}
			if !found {
				t.Fatalf("no release step for %s. NetworkManager can connect it to a remembered "+
					"network while hostapd beacons on it.\nwant: %s\npre-engine steps:\n  %s",
					p.Hotspot, want, strings.Join(got, "\n  "))
			}

			// The inverse must be there too. A user whose Pi permanently
			// stopped joining their WiFi has lost more than a hotspot.
			var undo []string
			for _, s := range p.PreEngineSteps(f.Sysctl) {
				if RunnerKey(s.Do) == want {
					undo = append(undo, RunnerKey(s.Undo))
				}
			}
			if len(undo) != 1 || undo[0] != "nmcli device set "+p.Hotspot+" managed yes" {
				t.Fatalf("the release has no inverse that gives the interface back: %v", undo)
			}

			// And the station teardown stayed with the takeover: nothing here
			// strips an address or retypes an interface that has no station
			// link to undo.
			for _, s := range p.HotspotReleaseSteps() {
				if s.Do.Path == BinNmcli {
					// The release itself, which is the point of this path.
					continue
				}
				t.Errorf("the station teardown leaked onto a path with no station link: %s (%s)",
					RunnerKey(s.Do), s.Op)
			}
		})
	}

	// The takeover keeps its full sequence, in its original order.
	t.Run("takeover keeps the whole release sequence", func(t *testing.T) {
		sc := pi5Captured()
		f, p := mustPlan(t, sc, DefaultOptions())
		fb, err := p.HotspotTakeover(f)
		if err != nil {
			t.Skipf("this scenario cannot take over: %v", err)
		}
		var ops []string
		for _, s := range fb.HotspotReleaseSteps() {
			ops = append(ops, s.Op)
		}
		want := []string{OpLink, OpAddr, OpLink, OpCreateIface}
		if len(ops) < len(want) {
			t.Fatalf("the takeover release sequence lost steps: %v", ops)
		}
		if ops[0] != OpLink || ops[len(ops)-1] != OpCreateIface {
			t.Fatalf("the takeover release no longer starts with the manager release and end with the type change: %v", ops)
		}
	})
}

// TestACreatedHotspotInterfaceIsReleasedFromNetworkManager replaces
// TestACreatedHotspotInterfaceHasNoMeasuredManagerAndIsNotReleased, which
// pinned the gap this closes.
//
// That test recorded a deliberate gap: on the paths where this package CREATES
// the access point's interface, detection ran before the interface existed, so
// nothing measured who manages it, and no release was emitted. It said in its
// own comment that if the open question in docs/DEFECTS.md D3 were ever
// answered, the right response was to delete it rather than make it pass.
//
// It was answered on the target on 2026-08-30, by logs rather than inference.
// NetworkManager claims a wifi device the moment it appears:
//
//	NetworkManager: manager: (ap0): new 802.11 Wi-Fi device
//	NetworkManager: device (ap0): state change: unmanaged -> unavailable
//	                              (reason 'managed', managed-type: 'external')
//	NetworkManager: device (ap0): state change: unavailable -> disconnected
//	                              (reason 'supplicant-available', managed-type: 'full')
//
// and the address went with it, so dnsmasq could not bind and the start failed.
//
// Note what did NOT change: Plan.HotspotManager is still unknown for a created
// interface, because nothing measured it and a guess would be a lie. The
// release is gated on NetworkManager being present on the MACHINE, which is a
// different fact and one detection really does measure.
func TestACreatedHotspotInterfaceIsReleasedFromNetworkManager(t *testing.T) {
	const nmcliHoldsEverything = "eth0:connected\nwlan0:connected\nwlan1:disconnected\n"

	sc := modeAScenario()
	r := sc.runner(t)
	r.SetOutput("nmcli -t -f DEVICE,STATE device status", nmcliHoldsEverything)
	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !f.NetworkManagerPresent {
		t.Fatal("a listing that named four devices must count as NetworkManager being present")
	}
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !p.HotspotIsVirtual {
		t.Skipf("this scenario no longer creates the interface")
	}
	if p.HotspotManager != ManagedByUnknown {
		t.Fatalf("Plan.HotspotManager is %q for %s, an interface this package creates. "+
			"Nothing measured it, so a value here was guessed", p.HotspotManager, p.Hotspot)
	}

	// The order is the whole point: created, released, and only then does an
	// address go on it.
	keys := stepKeys(p.PreEngineSteps(f.Sysctl))
	create := indexOf(keys, "iw phy phy0 interface add "+p.Hotspot+" type __ap")
	release := indexOf(keys, "nmcli device set "+p.Hotspot+" managed no")
	addr := indexOf(keys, "ip address add 10.83.51.1/24 dev "+p.Hotspot)
	if create < 0 || release < 0 || addr < 0 {
		t.Fatalf("missing a step: create=%d release=%d addr=%d in %v", create, release, addr, keys)
	}
	if !(create < release && release < addr) {
		t.Fatalf("wrong order: create=%d release=%d addr=%d. The release must sit between them, "+
			"or NetworkManager takes the device and flushes the address", create, release, addr)
	}
}

// And on a machine with no NetworkManager, nothing runs nmcli at all.
//
// This is the half that keeps the fix from becoming a new failure. A box where
// nmcli is absent answers the detection probe with an error, no device is
// named, and a step that shells out to a binary that is not there would stop
// the start on a machine that has nothing wrong with it.
func TestNoNmcliOnTheCreatedPathWhenNetworkManagerIsNotThere(t *testing.T) {
	sc := modeAScenario()
	r := sc.runner(t)
	r.SetError("nmcli -t -f DEVICE,STATE device status", errors.New("exec: \"nmcli\": executable file not found in $PATH"))
	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if f.NetworkManagerPresent {
		t.Fatal("nmcli failing must not be read as NetworkManager being present")
	}
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !p.HotspotIsVirtual {
		t.Skipf("this scenario no longer creates the interface")
	}
	for _, s := range p.AllSteps(f.Sysctl) {
		if s.Do.Path == BinNmcli {
			t.Fatalf("ran nmcli on a machine where it is not installed: %s", RunnerKey(s.Do))
		}
	}
}
