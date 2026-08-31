// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// cutMarker is the comment internal/netcfg puts on the drop rule. It is the
// one string that tells a cut ruleset from a normal one, so it is also what
// these tests look for on disk.
const cutMarker = "client traffic cut by the user"

// rulesetsLoaded returns the nft input this service has put on the wire, in
// order, from index n onwards.
func rulesetsLoaded(w *world, n int) []string {
	var out []string
	for _, c := range w.runner.Commands()[n:] {
		if c.Path == netcfg.BinNft && c.Stdin != "" {
			out = append(out, c.Stdin)
		}
	}
	return out
}

// TestCuttingClientTrafficLeavesTheWayBack.
//
// A cut that also cuts the way back is a cut nobody can reverse from the sofa.
// The user is holding a phone that is joined to this hotspot, and the only way
// they can undo this is the page that phone is looking at.
//
// The assertions are on the bytes THIS service put on the wire, not on
// internal/netcfg's own unit test of the same ruleset. That package proves the
// two states differ by exactly the forward rules; this proves the box loaded
// the one that keeps the panel reachable.
func TestCuttingClientTrafficLeavesTheWayBack(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()

	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}
	apBefore := w.sys.CountCalls(w.cfg.HotspotPaths.HostapdBinary)
	dhcpBefore := w.sys.CountCalls(w.cfg.HotspotPaths.DnsmasqBinary)
	engineStops := w.eng.stops
	n := len(w.runner.Commands())

	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("cut: %v", err)
	}
	if !w.svc.ClientTrafficCut() {
		t.Fatal("the service does not report client traffic as cut")
	}

	loaded := rulesetsLoaded(w, n)
	if len(loaded) != 1 {
		t.Fatalf("a cut loaded %d rulesets, want exactly one atomic replace", len(loaded))
	}
	rs := loaded[0]

	// Forwarded client traffic is dropped, and the rule says why rather than
	// leaving an operator to infer it from an absence.
	if !strings.Contains(rs, cutMarker) {
		t.Errorf("the loaded ruleset does not drop forwarded client traffic:\n%s", rs)
	}

	// And every path a joined device needs to reach this box is still open.
	// Without the panel line the user cannot undo this from the phone in their
	// hand; without DHCP and DNS the device drops off the network on its own
	// and the cut becomes indistinguishable from switching the box off.
	for _, want := range []struct{ what, rule string }{
		{"the panel", `iifname "wlan0" tcp dport 8088 accept`},
		{"DHCP", `iifname "wlan0" udp dport 67 accept`},
		{"client DNS", `iifname "wlan0" udp dport 53 accept`},
	} {
		if !strings.Contains(rs, want.rule) {
			t.Errorf("a cut box no longer accepts %s from a joined device (%q):\n%s", want.what, want.rule, rs)
		}
	}

	// Nothing that serves the joined device was touched. A cut that restarts
	// hostapd drops every device off the WiFi, which is the thing it exists to
	// avoid.
	if after := w.sys.CountCalls(w.cfg.HotspotPaths.HostapdBinary); after != apBefore {
		t.Errorf("the access point was restarted by a cut (%d then %d)", apBefore, after)
	}
	if after := w.sys.CountCalls(w.cfg.HotspotPaths.DnsmasqBinary); after != dhcpBefore {
		t.Errorf("the DHCP and DNS server was restarted by a cut (%d then %d)", dhcpBefore, after)
	}
	if w.eng.stops != engineStops {
		t.Errorf("the engine was stopped by a cut")
	}

	// And the panel still sees a working hotspot, because it is working.
	st, err := w.svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Hotspot.Running {
		t.Errorf("the hotspot is reported down after a cut: %+v", st.Hotspot)
	}
}

// TestRestoringPutsForwardingBack.
func TestRestoringPutsForwardingBack(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("cut: %v", err)
	}

	n := len(w.runner.Commands())
	if err := w.svc.Restore(ctx); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if w.svc.ClientTrafficCut() {
		t.Fatal("the service still reports client traffic as cut after restoring it")
	}
	loaded := rulesetsLoaded(w, n)
	if len(loaded) != 1 {
		t.Fatalf("a restore loaded %d rulesets, want exactly one", len(loaded))
	}
	if strings.Contains(loaded[0], cutMarker) {
		t.Errorf("the restored ruleset still drops client traffic:\n%s", loaded[0])
	}
}

// TestCuttingTwiceIsNotAnErrorAndLoadsNothingTheSecondTime.
//
// Two browser tabs, or one impatient person, must not turn the second press
// into a failure. Loading nothing is the other half: an atomic replace of the
// ruleset with an identical ruleset is work the box does not need to do.
func TestCuttingTwiceIsNotAnErrorAndLoadsNothingTheSecondTime(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("first cut: %v", err)
	}

	n := len(w.runner.Commands())
	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("second cut: %v", err)
	}
	if loaded := rulesetsLoaded(w, n); len(loaded) != 0 {
		t.Errorf("cutting an already cut box loaded %d rulesets, want none", len(loaded))
	}
	if !w.svc.ClientTrafficCut() {
		t.Error("the second cut cleared the state the first one set")
	}

	// The same for a restore that has nothing to restore.
	if err := w.svc.Restore(ctx); err != nil {
		t.Fatalf("restore: %v", err)
	}
	n = len(w.runner.Commands())
	if err := w.svc.Restore(ctx); err != nil {
		t.Fatalf("second restore: %v", err)
	}
	if loaded := rulesetsLoaded(w, n); len(loaded) != 0 {
		t.Errorf("restoring an already forwarding box loaded %d rulesets, want none", len(loaded))
	}
}

// TestCuttingABoxThatIsNotRunningIsRefused.
//
// Producing the state would mean detecting and planning purely to load a table
// naming a hotspot interface that has not been created, on a machine whose
// invariant while off is that it was left as it was found. A box that is off
// forwards nothing already, so the request is more work for a smaller result,
// and the only way to honour it would be to remember it until the next start,
// which is a cut surviving an off and on cycle.
func TestCuttingABoxThatIsNotRunningIsRefused(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"cut", func() error { return w.svc.Cut(ctx) }},
		{"restore", func() error { return w.svc.Restore(ctx) }},
	} {
		t.Run("before any start: "+tc.name, func(t *testing.T) {
			n := len(w.runner.Commands())
			if err := tc.call(); err == nil {
				t.Fatal("a box that was never started accepted the request")
			}
			if spent := len(w.runner.Commands()) - n; spent != 0 {
				t.Errorf("the refusal ran %d commands; it must change nothing", spent)
			}
			if w.svc.ClientTrafficCut() {
				t.Error("the refusal left the service reporting a cut")
			}
		})
	}

	// And after the box has been switched off again, which is the state a
	// stale browser tab is most likely to post into.
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	n := len(w.runner.Commands())
	if err := w.svc.Cut(ctx); err == nil {
		t.Fatal("a box that has been switched off accepted a cut")
	}
	if loaded := rulesetsLoaded(w, n); len(loaded) != 0 {
		t.Errorf("cutting a switched-off box loaded %d rulesets onto a machine that was left as it was found",
			len(loaded))
	}
}

// TestSwitchingOffClearsACut.
func TestSwitchingOffClearsACut(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("cut: %v", err)
	}
	if err := w.svc.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if w.svc.ClientTrafficCut() {
		t.Error("a box that has been switched off still reports client traffic as cut")
	}
}

// TestAConfigurationChangeLiftsACutAndARepeatPressDoesNot.
//
// Two shapes of pressing "on" while cut, and they differ on purpose.
//
// A DIFFERENT configuration is stopped and applied from scratch, and applying
// it loads the normal ruleset. Carrying a cut across that would leave the
// ruleset in force disagreeing with the plan just applied.
//
// The SAME configuration is a repair: internal/hotspot re-asserts the two
// daemons and the network configuration is not touched, so the cut stays. The
// on switch means "be running", not "resume forwarding"; the cut has its own
// control. This is only safe because the status carries the cut state, so the
// panel can say the box is on AND cut rather than showing a green light over a
// box that forwards nothing.
func TestAConfigurationChangeLiftsACutAndARepeatPressDoesNot(t *testing.T) {
	ctx := context.Background()

	t.Run("a repeat press of the same configuration leaves the cut in force", func(t *testing.T) {
		w := newWorld(t)
		refuseSecondInterface(w)
		req := startRequest(t)
		if err := w.svc.Start(ctx, req); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := w.svc.Cut(ctx); err != nil {
			t.Fatalf("cut: %v", err)
		}
		if err := w.svc.Start(ctx, req); err != nil {
			t.Fatalf("repeat press: %v", err)
		}
		if !w.svc.ClientTrafficCut() {
			t.Error("a repeat press of the same configuration lifted the cut, so the emergency stop is undone " +
				"by the button next to it")
		}
	})

	t.Run("a changed configuration lifts it", func(t *testing.T) {
		w := newWorld(t)
		refuseSecondInterface(w)
		if err := w.svc.Start(ctx, startRequest(t)); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := w.svc.Cut(ctx); err != nil {
			t.Fatalf("cut: %v", err)
		}
		changed := startRequest(t)
		changed.Hotspot.SSID = "Caspian-Kitchen"
		if err := w.svc.Start(ctx, changed); err != nil {
			t.Fatalf("start with a changed configuration: %v\ntimeline:%s", err, w.tl)
		}
		if w.svc.ClientTrafficCut() {
			t.Error("a configuration applied from scratch left the service reporting a cut, so the ruleset " +
				"in force disagrees with the plan that was just applied")
		}
	})
}

// TestACutIsNeverWrittenDown.
//
// The property the design rests on: somebody who cannot work out why their
// internet has stopped gets it back by pulling the plug. That only holds while
// nothing writes the cut down, so this walks every file this service can write
// and fails if the cut ruleset is in any of them.
//
// docs/LAYOUT.md puts state.json under the panel process and says the
// privileged side reads no state file, so the files this can reach are the
// teardown journal and the two generated configurations.
func TestACutIsNeverWrittenDown(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()

	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("cut: %v", err)
	}

	checked := 0
	err := filepath.WalkDir(w.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		checked++
		if strings.Contains(string(b), cutMarker) {
			t.Errorf("%s holds the cut, so it would survive a restart", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the service's directory: %v", err)
	}
	if checked == 0 {
		t.Fatal("no file was checked, so this test would pass on a service that wrote the cut to every one of them")
	}

	// And the files internal/hotspot writes, which the recorder holds rather
	// than putting on disk.
	for path, content := range w.sys.Files {
		if strings.Contains(string(content), cutMarker) {
			t.Errorf("%s holds the cut", path)
		}
		checked++
	}
	t.Logf("checked %d files written by this service", checked)
}

// TestACutDoesNotSurviveARestart.
//
// The property asserted through the thing that actually has to hold: a second
// Service over the same directory and the same journal, which is what systemd
// gives you after a restart. It comes up forwarding, and the ruleset it loads
// carries no cut.
func TestACutDoesNotSurviveARestart(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()

	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Cut(ctx); err != nil {
		t.Fatalf("cut: %v", err)
	}
	if !w.svc.ClientTrafficCut() {
		t.Fatal("the cut did not take, so this test is not about what it is named for")
	}

	// The process goes away without stopping anything, which is what a crash
	// or a systemctl restart looks like, and comes back over the same files.
	if err := w.svc.Close(); err != nil {
		t.Fatalf("closing the old service: %v", err)
	}
	restarted, err := New(w.cfg)
	if err != nil {
		t.Fatalf("building the restarted service: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })

	if restarted.ClientTrafficCut() {
		t.Fatal("a freshly built service reports client traffic as cut, so the cut was read from somewhere")
	}

	n := len(w.runner.Commands())
	if _, err := restarted.ReplayJournal(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	for _, rs := range rulesetsLoaded(w, n) {
		if strings.Contains(rs, cutMarker) {
			t.Errorf("recovery loaded a ruleset that cuts client traffic:\n%s", rs)
		}
	}
	if restarted.ClientTrafficCut() {
		t.Error("recovery left the restarted service reporting a cut")
	}
}

// TestTheCutRefusalCarriesAFaultTheSocketCanReport.
//
// Every refusal this service returns has to reduce to one word from the
// panel's closed vocabulary, because that word is all that crosses the socket.
func TestTheCutRefusalCarriesAFaultTheSocketCanReport(t *testing.T) {
	w := newWorld(t)
	err := w.svc.Cut(context.Background())
	if err == nil {
		t.Fatal("cutting a box that is not running was accepted")
	}
	if got := panel.FaultOf(err); got != panel.FaultNotRunning {
		t.Errorf("fault = %q, want %q: this is the stale-tab case and the whole reason that fault exists",
			got, panel.FaultNotRunning)
	}
}

// TestTheCutCrossesTheSocketInBothDirections.
//
// Everything above drives the Service directly. This drives the thing the panel
// actually holds, through a real socket with the real peer credential check, in
// both states and in the refusal.
//
// The refusal is the half worth spelling out. A cut requested on a box that is
// not running is the stale-tab case: the panel drew a control while the box was
// on, the box was switched off, and somebody pressed it. It has to arrive as
// FaultNotRunning, because the fault word is the whole of what crosses this
// boundary and "not-running" is the one the panel has a true sentence for.
func TestTheCutCrossesTheSocketInBothDirections(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})
	c := NewClient(path)
	ctx := context.Background()

	// Before the box is on, both verbs are refused with the word the panel can
	// turn into a sentence.
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"cut", func() error { return c.Cut(ctx) }},
		{"restore", func() error { return c.Restore(ctx) }},
	} {
		err := tc.call()
		if err == nil {
			t.Fatalf("%s on a box that is not running was accepted across the socket", tc.name)
		}
		if got := panel.FaultOf(err); got != panel.FaultNotRunning {
			t.Errorf("%s: fault = %q, want %q", tc.name, got, panel.FaultNotRunning)
		}
	}

	if err := c.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}
	st, err := c.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.ClientTrafficCut {
		t.Fatal("a box that has just started reports client traffic as cut")
	}
	if !st.Connected() {
		t.Fatal("the box is not reported as carrying client traffic after a start")
	}

	if err := c.Cut(ctx); err != nil {
		t.Fatalf("cut: %v", err)
	}
	st, err = c.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ClientTrafficCut {
		t.Error("the cut did not cross the socket, so the panel cannot show which state the control is in")
	}
	// The half that matters most on the panel: a cut box is NOT connected. A
	// green light over a box that forwards nothing is the false green this
	// whole readback effort exists to remove.
	if st.Connected() {
		t.Error("a cut box is reported as connected")
	}
	// And the hotspot is still up underneath it, which is what keeps the phone
	// in the user's hand able to reach the page that undoes this.
	if !st.Hotspot.Running {
		t.Errorf("the hotspot is reported down while cut: %+v", st.Hotspot)
	}

	if err := c.Restore(ctx); err != nil {
		t.Fatalf("restore: %v", err)
	}
	st, err = c.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.ClientTrafficCut {
		t.Error("the restore did not cross the socket")
	}
	if !st.Connected() {
		t.Error("the box is not reported as carrying client traffic after a restore")
	}
}
