// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// Evidence for an OPEN defect, measured on the target 2026-08-30. Nothing in
// production is fixed by these tests; they exist so the diagnosis cannot be
// lost and so a fix has something to turn green.
//
// A radio can refuse a second interface in two different ways, and this
// package can only detect one of them:
//
//	phy0, brcmfmac:  "iw phy phy0 interface add" answers "Input/output error
//	                 (-5)" and creates nothing, WHILE WLAN0 IS ASSOCIATED. The
//	                 step fails with op=iface, which the caller can see, so the
//	                 takeover fallback fires. The condition matters: measured
//	                 again on 2026-08-30 with wlan0 NOT joined to a network,
//	                 the same command returned 0 and the interface appeared.
//	                 This driver refuses a second interface beside a live
//	                 station link, not every second interface.
//	phy1, rtl8xxxu:  the add SUCCEEDS. The new interface inherits its parent's
//	                 MAC verbatim, so bringing it up is refused with "Name not
//	                 unique on network"; given a distinct MAC it is refused
//	                 with "Device or resource busy". The step fails with
//	                 op=link, which no fallback watches, so the start dies.
//
// A created-and-unusable interface is a different failure from a refused
// create, and today the plan cannot tell them apart.

// The two walls, as the kernel answers them.
func TestVifEvidence_ARadioCanRefuseAfterTheInterfaceExists(t *testing.T) {
	ctx := context.Background()
	k := NewSimulatedKernel("wlan1")
	k.InheritsParentMAC = true
	k.Preload("mac", "wlan1", "02:00:5e:00:00:12")
	k.Preload("ifacephy", "wlan1", "phy1")

	add := Command{Path: BinIw, Args: []string{"phy", "phy1", "interface", "add", "captest", "type", "__ap"}}
	if _, err := k.Run(ctx, add); err != nil {
		t.Fatalf("the add succeeds on this radio: %v", err)
	}

	// Wall one: the created interface carries its parent's address.
	res, err := k.Run(ctx, Command{Path: BinIP, Args: []string{"link", "set", "dev", "captest", "up"}})
	if err == nil {
		t.Fatal("bringing up an interface with a duplicate MAC must fail")
	}
	contains(t, res.Stderr, "Name not unique on network")
	// It is the ADDRESS that is duplicated, not the name. The kernel's wording
	// sends a reader to the wrong place, which is worth knowing when reading a
	// log at speed.
	if !strings.Contains(res.Stderr, "Name") {
		t.Error("the measured wording names the NAME even though the address is the duplicate")
	}

	// Wall two: with a distinct MAC the radio itself refuses.
	k2 := NewSimulatedKernel("wlan1")
	k2.Preload("ifacephy", "wlan1", "phy1")
	k2.RefuseLinkUp = "RTNETLINK answers: Device or resource busy"
	if _, err := k2.Run(ctx, add); err != nil {
		t.Fatal(err)
	}
	res, err = k2.Run(ctx, Command{Path: BinIP, Args: []string{"link", "set", "dev", "captest", "up"}})
	if err == nil {
		t.Fatal("the radio cannot hold an access point beside an associated station")
	}
	contains(t, res.Stderr, "Device or resource busy")

	// And neither wording is an already-exists condition, so neither can be
	// tolerated by the apply path.
	if IsAlreadyExists(res, err) {
		t.Error("a busy radio is not an object that already exists")
	}
}

// The defect, and the plan that now avoids it entirely.
//
// Two halves, and both are the point:
//
//  1. The planner does not create a second interface on this radio at all. It
//     ends the station and takes the interface over, which is the sequence
//     proved by hand on the box.
//  2. What the old plan did, kept because it is the reason half 1 has to be a
//     PLAN decision and not a runtime fallback: the second interface is
//     created successfully and then cannot be brought up, so the failure
//     carries op=link, and the fallback watches op=iface. Nothing falls back
//     and the start dies with the machine half configured.
//
// Half 2 constructs the old plan by hand, because the planner will not produce
// one any more. That is the improvement, and it is why the evidence has to be
// built rather than obtained.
func TestVifEvidence_ThePlanTakesOverRatherThanFailingAtTheLinkUp(t *testing.T) {
	ctx := context.Background()
	s := twoRadioScenario("capture-pi5-ip-route-default.txt")
	f := s.facts(t, BaseSysctlKnobs())
	o := DefaultOptions()
	o.HotspotOverride = "wlan1"

	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if p.HotspotIsVirtual {
		t.Fatalf("the plan still creates a second interface on a radio that cannot hold one: hotspot=%s parent=%s",
			p.Hotspot, p.HotspotParent)
	}
	if !p.HotspotTakenOver || p.Hotspot != "wlan1" {
		t.Fatalf("hotspot=%s takenOver=%v, want a takeover of wlan1", p.Hotspot, p.HotspotTakenOver)
	}
	// No step of the real plan creates an interface, which is what makes the
	// failure below unreachable.
	for _, st := range p.AllSteps(f.Sysctl) {
		if strings.Contains(RunnerKey(st.Do), "interface add") {
			t.Fatalf("the plan still adds an interface: %s", RunnerKey(st.Do))
		}
	}

	// Half 2. The plan is built by hand, because the planner will not produce
	// one any more, and that is the whole improvement.
	doomed := &Plan{
		Tun:              "xray0",
		Opts:             DefaultOptions(),
		Hotspot:          "ap0",
		HotspotIsVirtual: true,
		HotspotPhy:       "phy1",
		HotspotParent:    "wlan1",
		HotspotSubnet:    netip.MustParsePrefix("10.83.51.0/24"),
		HotspotGateway:   netip.MustParseAddr("10.83.51.1"),
	}
	steps := append(doomed.VirtualIfaceSteps(), doomed.HotspotAddrSteps()...)

	k := NewSimulatedKernel("lo", "eth0", "wlan0", "wlan1", "xray0")
	k.Reads = s.runner(t)
	k.InheritsParentMAC = true
	k.Preload("mac", "wlan1", "02:00:5e:00:00:12")
	k.Preload("ifacephy", "wlan1", "phy1")
	before := k.Snapshot()

	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := a.Apply(ctx, steps)
	if err == nil {
		t.Fatal("the link-up cannot succeed on this radio")
	}
	failed, ok := rep.FailedStep()
	if !ok {
		t.Fatal("the report must name the failing step")
	}
	contains(t, CommandLine(failed.Do), "link set dev")
	contains(t, err.Error(), "Name not unique on network")

	// THE DEFECT the refusal exists for. The caller's fallback rule watches
	// OpCreateIface. This failure carries OpLink, so nothing falls back.
	if failed.Op == OpCreateIface {
		t.Error("the failing step now carries the op a fallback watches; the defect is fixed " +
			"and this half of the test should go")
	}
	if failed.Op != OpLink {
		t.Errorf("failing op = %q, want %q as measured", failed.Op, OpLink)
	}

	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "machine state after the failed start", k.Snapshot(), before)
}
