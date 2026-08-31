// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// wirelessUplinkScenario is the captured radio with the default route moved
// onto it, so the only AP-capable radio is the one carrying the internet.
func wirelessUplinkScenario() scenario {
	s := pi5Captured()
	s.name = "captured radio, wireless uplink"
	s.route = "scenario-modeb-ip-route-default.txt"
	return s
}

// The plan the capability table implies, and the fallback it carries.
func TestPlan_CarriesATakeoverFallback(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())

	if !p.HotspotIsVirtual || p.Hotspot != "ap0" {
		t.Fatalf("first choice should still be a second interface: hotspot=%q virtual=%v", p.Hotspot, p.HotspotIsVirtual)
	}
	// The uplink is eth0, so wlan0 is free to be taken over.
	if p.HotspotFallback != "wlan0" {
		t.Errorf("HotspotFallback = %q, want wlan0", p.HotspotFallback)
	}
	if p.HotspotFallback == p.Uplink {
		t.Error("the fallback must never be the interface carrying the uplink")
	}
}

// The fallback plan, and what it costs.
func TestPlan_HotspotTakeover(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}

	if q.Hotspot != "wlan0" || q.HotspotIsVirtual || q.HotspotParent != "" {
		t.Errorf("takeover plan: hotspot=%q virtual=%v parent=%q, want wlan0 taken over directly",
			q.Hotspot, q.HotspotIsVirtual, q.HotspotParent)
	}
	if !q.HotspotTakenOver {
		t.Error("the plan must record that it is the fallback")
	}
	if q.Uplink != "eth0" {
		t.Errorf("uplink changed to %q; the fallback must not touch the internet connection", q.Uplink)
	}

	// With the station gone the radio has one interface, so "#channels <= 1"
	// no longer constrains anything and the channel is free again.
	if q.ChannelPinned {
		t.Error("the channel is no longer pinned once the station link is gone")
	}
	if q.Channel != 1 {
		t.Errorf("channel = %d, want the first usable channel", q.Channel)
	}

	// No step may add an interface any more, though one now SETS the type.
	for _, s := range q.AllSteps(f.Sysctl) {
		if strings.Contains(RunnerKey(s.Do), "interface add") {
			t.Errorf("the fallback plan still tries to create an interface: %s", RunnerKey(s.Do))
		}
	}

	// The cost has to be recorded, not hidden.
	// The note must describe what has to be DONE, not assert an effect that
	// nobody produced. The previous wording said the connection was already
	// ended, and was read as a description of the outcome.
	notes := strings.Join(q.Notes, "\n")
	contains(t, notes, "planned on wlan0")
	contains(t, notes, "must first be released by whatever manages it")
	contains(t, notes, "read back from the kernel")
	notContains(t, notes, "#channels <= 1")
	notContains(t, notes, "this ends the WiFi connection")

	// And it has to be in the sentence a person reads, because the panel
	// shows that and not the notes.
	e := q.Explain()
	contains(t, e, "Hotspot: WiFi on wlan0")
	contains(t, e, "has to be disconnected from the WiFi network it is on now")
	notContains(t, e, "second connection on the same radio")

	// The original plan must be unchanged: the caller still has to tear it
	// down, and a mutated plan would tear down the wrong things.
	if p.Hotspot != "ap0" || p.HotspotTakenOver {
		t.Errorf("HotspotTakeover mutated the original plan: hotspot=%q takenOver=%v", p.Hotspot, p.HotspotTakenOver)
	}
	origNotes := strings.Join(p.Notes, "\n")
	contains(t, origNotes, "#channels <= 1")
	notContains(t, origNotes, "takes over wlan0")
}

// Taking over the uplink would cut the box off from the internet it exists to
// share, so it is refused in plain words instead.
func TestPlan_HotspotTakeoverRefusesToTakeTheUplink(t *testing.T) {
	f := wirelessUplinkScenario().facts(t, BaseSysctlKnobs())
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.Uplink != "wlan0" {
		t.Fatalf("uplink = %q, want wlan0 for this scenario to mean anything", p.Uplink)
	}
	if p.HotspotFallback != "" {
		t.Fatalf("HotspotFallback = %q, but that interface carries the uplink", p.HotspotFallback)
	}

	_, err = p.HotspotTakeover(f)
	if !errors.Is(err, ErrNoTakeoverCandidate) {
		t.Fatalf("err = %v, want ErrNoTakeoverCandidate", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("the refusal must carry wording for the panel")
	}
	contains(t, pe.UserMessage(), "cut off the internet")
	contains(t, pe.UserMessage(), "cable")
	notContains(t, pe.UserMessage(), "phy")
	notContains(t, pe.UserMessage(), "__ap")
}

// End to end, against a driver that refuses the combination its own capability
// table advertises. This is what the target actually did.
func TestApply_FallsBackWhenTheDriverRefusesToCreateTheInterface(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	k := capturedKernel(t)
	k.Reads = pi5Captured().runner(t)
	k.RefuseIfaceAdd = SimIwDriverRefuses
	// The state the box was actually in: wlan0 joined to the house network,
	// holding that network's address. Leaving this out of the model is how
	// the address-removal step went missing in the first place.
	k.Preload("addr", "wlan0", "10.0.0.222/24")
	before := k.Snapshot()

	path := tmpJournal(t)
	a, err := NewApplier(k, path)
	if err != nil {
		t.Fatal(err)
	}

	// The first plan gets as far as creating the interface and stops.
	rep, err := a.Apply(ctx, p.AllSteps(f.Sysctl))
	if err == nil {
		t.Fatal("the driver refuses, so the first plan must fail")
	}
	failed, ok := rep.FailedStep()
	if !ok {
		t.Fatal("the report must name the step that failed")
	}
	if failed.Op != OpCreateIface {
		t.Fatalf("failed step op = %q, want %q; a caller cannot know to fall back otherwise", failed.Op, OpCreateIface)
	}
	contains(t, err.Error(), "Input/output error")

	// Roll back what did land. This is what the privileged service did, and
	// it must return the machine exactly.
	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "machine state after rollback", k.Snapshot(), before)

	// Now the fallback.
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatalf("a fallback must exist here: the uplink is eth0 and wlan0 is releasable: %v", err)
	}

	a2, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	// Only the commands issued from here on belong to the fallback; the
	// kernel has been recording since the first attempt.
	mark := len(k.Lines())
	rep2, err := a2.Apply(ctx, q.AllSteps(f.Sysctl))
	if err != nil {
		t.Fatalf("the fallback plan must apply cleanly: %v", err)
	}
	if rep2.Failed != 0 {
		t.Errorf("fallback failures: %v", rep2.Err())
	}
	for _, l := range k.Lines()[mark:] {
		if strings.Contains(l, "interface add") {
			t.Errorf("the fallback still tried to create an interface: %s", l)
		}
	}

	// The hotspot is on wlan0 and the uplink is untouched.
	snap := strings.Join(k.Snapshot(), "\n")
	contains(t, snap, "addr wlan0 10.83.51.1/24")
	notContains(t, snap, "iface ap0")

	// And the firewall follows the interface: the leak block must name wlan0
	// now, or client traffic has no block on the path it actually uses.
	contains(t, q.Ruleset(), `iifname "wlan0" oifname "eth0" drop`)
	notContains(t, q.Ruleset(), `iifname "ap0"`)

	// Teardown returns the machine.
	if _, err := a2.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "machine state after fallback teardown", k.Snapshot(), before)
}

// A radio that accepts the creation must not fall back: the fallback costs the
// user a WiFi connection and is only worth paying for when it is necessary.
func TestApply_NoFallbackWhenTheDriverAccepts(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	k := capturedKernel(t)
	k.Reads = pi5Captured().runner(t)
	// RefuseIfaceAdd unset: the driver does what its table advertises.

	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ctx, p.AllSteps(f.Sysctl)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	snap := strings.Join(k.Snapshot(), "\n")
	contains(t, snap, "iface ap0")
	contains(t, snap, "addr ap0 10.83.51.1/24")
	notContains(t, snap, "addr wlan0 10.83.51.1/24")
}
