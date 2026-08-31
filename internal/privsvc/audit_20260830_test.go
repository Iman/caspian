// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
)

// Tests written during the 2026-08-30 external-findings audit. They are
// EVIDENCE, not fixes: nothing in production changed. Delete them if the audit
// is rejected.

// The inverse of the first pre-engine kernel-knob change. The fixture reads
// ip_forward as 0, so this is what restores it.
const firstPlanForwardUndo = "sysctl -w net.ipv4.ip_forward=0"

// TestTheFallbackIsRefusedWhenTheFirstPlanCouldNotBeUndone.
//
// The hotspot fallback is only correct on a machine the first plan has been
// taken off. Both plans touch the same firewall, the same kernel knobs and the
// same hotspot address, and the fresh detection that decides whether the
// takeover is even permitted reads a machine that must no longer be carrying
// the first plan.
//
// Until 2026-08-30 the guard tested only Teardown's error, which netcfg never
// sets for a failed inverse, so a teardown that undid nothing carried on and
// the fallback was applied on top of the first plan's changes.
func TestTheFallbackIsRefusedWhenTheFirstPlanCouldNotBeUndone(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	// One inverse of the first plan refuses. The wording is deliberately not
	// one of netcfg's notFoundMarkers, so replay counts it as a real failure
	// rather than as "already gone".
	w.runner.SetError(firstPlanForwardUndo,
		errors.New("sysctl: permission denied on key \"net.ipv4.ip_forward\""))

	err := w.svc.Start(context.Background(), startRequest(t))

	// The situation was really reached: the driver refused the interface and
	// the first plan's inverse was really attempted and really failed.
	if w.tl.indexOf("net: "+createIfaceCmd) < 0 {
		t.Fatalf("the second interface was never attempted, so the fallback path was not reached "+
			"and this test is not reproducing what it is named for\ntimeline:%s", w.tl)
	}
	if w.tl.indexOf("net: "+firstPlanForwardUndo) < 0 {
		t.Fatalf("the first plan's ip_forward inverse was never attempted\ntimeline:%s", w.tl)
	}

	// And the fallback was refused.
	if w.tl.indexOf("net: "+takeoverAddrCmd) >= 0 {
		t.Fatalf("the fallback was applied on top of a machine still carrying the first plan's "+
			"ip_forward change. The fresh detection that permits the takeover read a machine the "+
			"first plan had not been taken off\ntimeline:%s", w.tl)
	}
	if err == nil {
		t.Fatalf("the start reported success without applying either plan\ntimeline:%s", w.tl)
	}

	// The operator is told which of the two it was: nothing could be undone,
	// rather than the journal file being unreadable.
	if !strings.Contains(w.logs.String(), "could not undo the first plan") {
		t.Fatalf("the log does not say why the fallback was not attempted:\n%s", w.logs.String())
	}
	if !strings.Contains(w.logs.String(), "inverses_failed") {
		t.Fatalf("the log does not distinguish a failed inverse from a journal file error, which "+
			"are the two ways this branch is reached:\n%s", w.logs.String())
	}
}

// TestAStopThatCouldNotUndoEverythingKeepsTheBlockAndTheJournal.
//
// The narrower sibling of TestTheFirewallSurvivesAStopThatCouldNotUndoEverything:
// there the daemons also survive, here only the routing inverses fail, which is
// the more ordinary shape. Two things have to hold, and the second is what stops
// the first from stranding a table on the machine for ever:
//
//   - the generated table is not deleted while routes, rules and addresses the
//     teardown could not remove are still installed
//   - the firewall's inverse stays in the journal, so the next start retries it
func TestAStopThatCouldNotUndoEverythingKeepsTheBlockAndTheJournal(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	refused := errors.New("RTNETLINK answers: Operation not permitted")
	failed := 0
	for _, s := range currentPlanSteps(t, w) {
		if s.Undo.IsZero() || s.Undo.Path != netcfg.BinIP {
			continue
		}
		w.runner.SetError(netcfg.RunnerKey(s.Undo), refused)
		failed++
	}
	if failed == 0 {
		t.Fatal("no ip inverse was registered to fail, so this test proves nothing")
	}

	before := len(w.runner.Commands())
	stopErr := w.svc.Stop(context.Background())

	var after []string
	for _, c := range w.runner.Commands()[before:] {
		if isReadOnly(c) {
			continue
		}
		after = append(after, netcfg.RunnerKey(c))
		if c.Path == netcfg.BinNft {
			t.Errorf("the generated table was deleted while %d routing inverses had failed, so the "+
				"machine is left carrying routes and rules with no block.\nteardown order:\n  %s",
				failed, strings.Join(after, "\n  "))
		}
	}

	left, err := netcfg.LoadJournal(w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("reading the journal: %v", err)
	}
	sawNft := false
	for _, e := range left {
		if e.Op == netcfg.OpNft {
			sawNft = true
		}
	}
	if !sawNft {
		t.Fatalf("the firewall inverse was held but is not in the journal, so nothing will ever "+
			"remove the table. The journal holds %d entries", len(left))
	}

	// RECORDED, not asserted: Stop's own return is nil here, because
	// Applier.Teardown reports failed inverses in its report rather than its
	// error and stopLocked only logs the count. The panel is therefore told the
	// box was returned to how it was found while it is still fully configured.
	// That is an open defect, carried in docs/DEFECTS.md, and it is separate
	// from the leak this test guards.
	t.Logf("RECORDED GAP: Stop returned %v with %d inverses failed and %d journal entries left. "+
		"See docs/DEFECTS.md.", stopErr, failed, len(left))
}

// stubbornSystem is a machine on which the hotspot daemons refuse to die: every
// signal fails, so internal/hotspot's Stop cannot take hostapd or dnsmasq away.
type stubbornSystem struct {
	hotspot.System
}

func (stubbornSystem) SignalProcess(int, hotspot.Signal) error {
	return errors.New("operation not permitted")
}

// TestTheFirewallSurvivesAStopThatCouldNotUndoEverything.
//
// This is the Android finding transposed, and the shape the appliance was
// vulnerable to until 2026-08-30: the block removed while the path it was
// blocking is still live. Everything that could keep the path alive is made to
// fail at once and only the nft inverse is left working:
//
//   - hostapd and dnsmasq cannot be signalled, so the access point stays up
//   - every "ip" inverse refuses, so the routes, the rules and the hotspot
//     address all stay
//   - every "sysctl" inverse refuses, so ip_forward stays 1
//
// Measured before the fix: the table was deleted, correctly last, and the box
// was left forwarding client traffic out of the uplink with no block at all.
// internal/netcfg's two-pass replay now holds the firewall's inverse whenever
// an earlier one failed.
func TestTheFirewallSurvivesAStopThatCouldNotUndoEverything(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.cfg.System = tracedSystem{System: stubbornSystem{System: w.sys}, tl: w.tl}
	})
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	refused := errors.New("RTNETLINK answers: Operation not permitted")
	blocked := 0
	for _, s := range currentPlanSteps(t, w) {
		if s.Undo.IsZero() || s.Undo.Path == netcfg.BinNft {
			continue
		}
		w.runner.SetError(netcfg.RunnerKey(s.Undo), refused)
		blocked++
	}
	if blocked == 0 {
		t.Fatal("no inverse was registered to fail, so this test proves nothing")
	}

	before := len(w.runner.Commands())
	stopErr := w.svc.Stop(context.Background())

	var after []string
	for _, c := range w.runner.Commands()[before:] {
		if isReadOnly(c) {
			continue
		}
		after = append(after, netcfg.RunnerKey(c))
		if c.Path == netcfg.BinNft {
			t.Errorf("the generated table was deleted during a stop that undid nothing. "+
				"The access point is still up, forwarding is still on and the routes are still "+
				"installed, so client traffic now leaves by the uplink in the clear.\n"+
				"teardown order:\n  %s", strings.Join(after, "\n  "))
		}
	}

	// The failure is reported rather than swallowed: a caller told "stopped"
	// while the box is still configured cannot act on it.
	if stopErr == nil {
		t.Fatalf("Stop reported success after undoing nothing.\nteardown order:\n  %s",
			strings.Join(after, "\n  "))
	}

	// And the box is still fail-closed: the access point that would not die is
	// still covered by the block that was held.
	st, serr := w.svc.Status(context.Background())
	if serr == nil && st.Hotspot.Running {
		t.Logf("EVIDENCE: the access point survived the stop and the block was HELD, so the "+
			"hotspot that is still up is still covered. Stop returned: %v", stopErr)
	}
}

// TestTheConfigChangeWindowNeverForwardsWithoutABlock.
//
// A configuration change is Stop then Start, so the generated table is deleted
// and a new one loaded, and between those two commands the box carries no table
// of ours at all. MEASURED on 2026-08-30: nine read commands run in that window,
// and the resolver call, engine.Validate and the hotspot plan run inside it too
// and issue no command at all, so the real window is as long as a DNS lookup.
//
// The window is not a leak window, and this is the property that makes it one
// rather than the other: forwarding is turned off BEFORE the table goes, and the
// new table is loaded BEFORE forwarding is turned back on. At no instant is the
// box a router without a block. That is an ordering, so only the sequence shows
// it, which is why it is asserted rather than reasoned about.
func TestTheConfigChangeWindowNeverForwardsWithoutABlock(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("first start: %v\ntimeline:%s", err, w.tl)
	}

	changed := req
	changed.Hotspot.SSID = "Caspian-Kitchen"
	mark := len(w.runner.Commands())
	if err := w.svc.Start(context.Background(), changed); err != nil {
		t.Fatalf("the changed configuration did not start: %v\ntimeline:%s", err, w.tl)
	}

	var trail []string
	for _, c := range w.runner.Commands()[mark:] {
		trail = append(trail, netcfg.RunnerKey(c))
	}

	const (
		forwardOff = "sysctl -w net.ipv4.ip_forward=0"
		forwardOn  = "sysctl -w net.ipv4.ip_forward=1"
	)
	idx := func(want string) int {
		for i, c := range trail {
			if c == want {
				return i
			}
		}
		return -1
	}
	var nfts []int
	for i, c := range trail {
		if strings.HasPrefix(c, "nft -f -") {
			nfts = append(nfts, i)
		}
	}
	if len(nfts) < 2 {
		t.Fatalf("a configuration change did not delete and reload the table; %d nft commands:\n  %s",
			len(nfts), strings.Join(trail, "\n  "))
	}
	deleted, loaded := nfts[0], nfts[len(nfts)-1]
	off, on := idx(forwardOff), idx(forwardOn)
	if off < 0 || on < 0 {
		t.Fatalf("forwarding was not turned off and back on across the change:\n  %s",
			strings.Join(trail, "\n  "))
	}

	if off > deleted {
		t.Errorf("the table was deleted at %d while forwarding was still enabled until %d. "+
			"For that window the box was a router with no block.\n  %s",
			deleted, off, strings.Join(trail, "\n  "))
	}
	if on < loaded {
		t.Errorf("forwarding was enabled at %d before the new table was loaded at %d. "+
			"For that window the box was a router with no block.\n  %s",
			on, loaded, strings.Join(trail, "\n  "))
	}
	t.Logf("MEASURED: %d commands run between the table being deleted and the new one being loaded. "+
		"The resolver call, engine.Validate and the hotspot plan also run in there and issue none, "+
		"so the window is longer than this count. Forwarding is off for all of it.", loaded-deleted-1)
}

// currentPlanSteps re-derives the steps the running plan applied, so a test can
// name their inverses exactly rather than matching on command prefixes.
func currentPlanSteps(t *testing.T, w *world) []netcfg.Step {
	t.Helper()
	w.svc.mu.RLock()
	plan := w.svc.plan
	facts := w.svc.facts
	w.svc.mu.RUnlock()
	if plan == nil {
		t.Fatal("the service holds no plan, so nothing was applied")
	}
	return plan.AllSteps(facts.Sysctl)
}
