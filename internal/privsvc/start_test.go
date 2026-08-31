// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// The addresses this recorded machine leads the planner to. They are asserted
// on by name rather than matched loosely, because "an address was added" and
// "the tunnel's address was added" are different claims.
const (
	hotspotAddrCmd = "ip address add 10.83.51.1/24 dev ap0"
	tunnelAddrCmd  = "ip address add 198.18.51.1/30 dev xray0"
	serverRouteCmd = "ip route add 203.0.113.7/32 via 192.168.1.1 dev eth0 proto static metric 5"
)

// TestStartAppliesThingsInTheRequiredOrder is the ordering internal/netcfg
// splits its step lists to enforce.
//
// Each assertion names the failure it prevents, cited to the package that
// imposes it, because an ordering test with no reason attached is one somebody
// will reorder to make a later change compile.
func TestStartAppliesThingsInTheRequiredOrder(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	// The firewall before forwarding is enabled. netcfg/route.go,
	// PreEngineSteps: "The firewall goes first so there is never a moment when
	// forwarding is enabled and the block is not."
	mustBefore(t, w.tl, "nft -f -", "sysctl -w net.ipv4.ip_forward=1",
		"forwarding on with the fail-closed ruleset not yet loaded is an open window in which client traffic leaves by the uplink")

	// Every kernel knob before the engine, because every one of them is in the
	// pre-engine list and this package must not move one across.
	//
	// The reason attached to this assertion used to be internal/netcfg's, that
	// conf.default is inherited when an interface is created. That package
	// RETRACTED it on 2026-08-30: conf.all decides the outcome for every
	// interface on its own. The ordering is still true and still worth
	// guarding, and it is now guarded for what it is rather than for a reason
	// that no longer holds.
	mustBefore(t, w.tl, "sysctl -w net.ipv4.conf.default.rp_filter=2", "engine: started",
		"every kernel knob this plan changes belongs to the pre-engine list, and moving one across the engine is how the two lists start to merge")

	// The pinned host route before the engine. netcfg/route.go,
	// ServerRouteSteps: without it "the engine tries to reach its own uplink
	// through the tunnel it has not built yet".
	mustBefore(t, w.tl, serverRouteCmd, "engine: started",
		"the engine's first connection to the server must already be outside the tunnel")

	// Every post-engine step after the engine. netcfg/route.go,
	// PostEngineSteps: "Every command here names that device, so every one of
	// them fails if it is run too early."
	for _, step := range []string{
		tunnelAddrCmd,
		"ip route add default dev xray0",
		"ip rule add from 10.83.51.0/24",
	} {
		mustBefore(t, w.tl, "engine: started", step,
			"this command names the tunnel device, which does not exist until the engine has created it")
	}

	// The hotspot last. A client that joins before the tunnel exists has a
	// working network connection and nowhere to go.
	mustBefore(t, w.tl, "engine: started", "hostapd -B",
		"the access point must not be broadcasting before there is anything behind it")
	mustBefore(t, w.tl, "hostapd -B", "dnsmasq --conf-file",
		"the access point comes up before the DHCP server that serves it")

	// And the firewall is the FIRST change of any kind made to the machine.
	cmds := w.mutatingCommands()
	if len(cmds) == 0 {
		t.Fatalf("nothing was applied at all\ntimeline:%s", w.tl)
	}
	if !strings.HasPrefix(cmds[0], "nft -f -") {
		t.Fatalf("the first change made to the machine was %q, and it has to be the firewall.\napplied:\n  %s",
			cmds[0], strings.Join(cmds, "\n  "))
	}
}

// TestStartCreatesTheAccessPointInterfaceBeforeAddressingIt guards the one
// ordering inside the pre-engine list that is about an interface this appliance
// creates rather than one the engine does.
func TestStartCreatesTheAccessPointInterfaceBeforeAddressingIt(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	mustBefore(t, w.tl, "iw phy phy0 interface add ap0", hotspotAddrCmd,
		"an address cannot be put on an interface that has not been created")
}

// TestTheHotspotForwardsDNSToThePortTheEngineListensOn is the pairing
// docs/LAYOUT.md calls "the one that breaks quietly".
//
// It is checked here rather than in either package because neither one can
// check it: internal/xcfg owns the listener and internal/hotspot owns the
// forwarder, and this is the only place that hands both of them a value.
func TestTheHotspotForwardsDNSToThePortTheEngineListensOn(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}

	conf, ok := w.sys.Files[w.cfg.HotspotPaths.DnsmasqConf]
	if !ok {
		t.Fatalf("no dnsmasq configuration was written")
	}
	want := "server=127.0.0.1#5354"
	if !strings.Contains(string(conf), want) {
		t.Fatalf("the dnsmasq configuration does not forward to %s, so every joined device would stop "+
			"resolving while the hotspot and the tunnel both looked healthy.\nconfiguration:\n%s", want, conf)
	}

	doc := w.eng.documents()
	if len(doc) != 1 {
		t.Fatalf("the engine was handed %d documents, want 1", len(doc))
	}
	if !strings.Contains(string(doc[0]), `"port": 5354`) {
		t.Fatalf("the engine's document has no listener on 5354, which is the port dnsmasq was told to forward to")
	}
}

// TestAFailedStartLeavesNothingApplied is the promise that a start which fails
// half way returns the machine to how it was found.
//
// The failure is injected at the LAST pre-engine step, so that a firewall, five
// kernel knobs, a created interface and an address have all been applied before
// it. A failure at the first step would pass whether or not the rollback works.
func TestAFailedStartLeavesNothingApplied(t *testing.T) {
	w := newWorld(t)
	w.runner.SetError(serverRouteCmd, errors.New("RTNETLINK answers: Network is unreachable"))

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("a start whose route step failed reported success\ntimeline:%s", w.tl)
	}

	assertMachineRestored(t, w)

	if w.eng.startCount() != 0 {
		t.Fatalf("the engine was started %d times during a start that failed before the engine step", w.eng.startCount())
	}
	if w.svc.isRunning() {
		t.Fatalf("the service reports itself as running after a start that failed")
	}
	if _, statErr := os.Stat(w.cfg.JournalPath); !errors.Is(statErr, os.ErrNotExist) {
		entries, _ := netcfg.LoadJournal(w.cfg.JournalPath)
		t.Fatalf("the journal still exists after a full rollback, holding %d entries", len(entries))
	}
}

// TestAFailedHotspotStartLeavesNothingApplied covers the other end of the
// sequence: everything applied, the engine running, and the last step failing.
//
// It is a separate test because the rollback has strictly more to undo here:
// the engine has to be stopped and the post-engine routing steps have to come
// out as well.
func TestAFailedHotspotStartLeavesNothingApplied(t *testing.T) {
	w := newWorld(t)
	failHostapd(w)

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("a start whose access point would not come up reported success\ntimeline:%s", w.tl)
	}
	if got := panel.FaultOf(err); got != panel.FaultHotspotFailed {
		t.Fatalf("fault was %q, want %q: a hostapd that exits with no recognisable reason is a hotspot failure",
			got, panel.FaultHotspotFailed)
	}

	assertMachineRestored(t, w)
	if w.svc.isRunning() {
		t.Fatalf("the service reports itself as running after a start that failed")
	}
	if w.tl.indexOf("engine: stopped") < 0 {
		t.Fatalf("the engine was left running after a start that failed at the hotspot\ntimeline:%s", w.tl)
	}
}

// TestASecondStartDoesNotApplyAnythingTwice.
//
// internal/netcfg's Applier.Apply converges on a second apply as of
// 2026-08-30, so this is no longer the difference between working and broken.
// It is still the difference between a repeat press doing nothing and a repeat
// press re-deriving a plan, re-reading the machine and re-issuing every
// command to arrive back where it already was, and the journal assertion below
// is the one that would catch a second inverse being recorded for a change
// that was made once.
func TestASecondStartDoesNotApplyAnythingTwice(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)

	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("first start: %v", err)
	}
	first := append([]string(nil), w.mutatingCommands()...)

	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("second start: %v\ntimeline:%s", err, w.tl)
	}
	second := w.mutatingCommands()

	if len(second) != len(first) {
		t.Fatalf("the second start made %d more changes to the machine than the first.\nafter one start:\n  %s\nafter two:\n  %s",
			len(second)-len(first), strings.Join(first, "\n  "), strings.Join(second, "\n  "))
	}

	// The journal holds one entry per change, so a doubled apply shows up
	// there too even if the runner were forgiving.
	entries, err := netcfg.LoadJournal(w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("reading the journal: %v", err)
	}
	seen := map[string]int{}
	for _, e := range entries {
		seen[netcfg.RunnerKey(e.Do)]++
	}
	for k, n := range seen {
		if n > 1 {
			t.Fatalf("the journal holds %d entries for %q, so the teardown would try to undo one change twice", n, k)
		}
	}

	// A repeat start is still allowed to re-assert the supervised processes,
	// which is how a daemon that died is brought back.
	if w.tl.count("hostapd -B") < 1 {
		t.Fatalf("the access point was never started at all\ntimeline:%s", w.tl)
	}
}

// TestARepeatStartDoesNotRestartAWorkingAccessPoint.
//
// internal/hotspot is explicit about why: "restarting a working access point
// disconnects every device on it". The mechanism is that the configuration on
// disk is unchanged, so this test asserts on the effect that mechanism has.
func TestARepeatStartDoesNotRestartAWorkingAccessPoint(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("first start: %v", err)
	}
	before := w.sys.CountCalls(w.cfg.HotspotPaths.HostapdBinary)

	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if after := w.sys.CountCalls(w.cfg.HotspotPaths.HostapdBinary); after != before {
		t.Fatalf("hostapd was started %d times over two identical requests, want %d: "+
			"a restart drops every device on the hotspot", after, before)
	}
}

// TestStopUndoesTheFirewallLast.
//
// The fail-closed ruleset has to stay in force while the routes come out. It
// falls out of the journal being a stack, and this is the assertion that the
// stack is not reordered by anything here.
func TestStopUndoesTheFirewallLast(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v\ntimeline:%s", err, w.tl)
	}

	cmds := w.mutatingCommands()
	lastNft := -1
	lastRoute := -1
	for i, c := range cmds {
		switch {
		case strings.HasPrefix(c, "nft -f -"):
			lastNft = i
		case strings.HasPrefix(c, "ip route del"), strings.HasPrefix(c, "ip rule del"), strings.HasPrefix(c, "ip address del"):
			lastRoute = i
		}
	}
	if lastNft < lastRoute {
		t.Fatalf("the firewall was removed before the last route came out, which is a window in which "+
			"client traffic leaves by the uplink.\napplied and undone in this order:\n  %s",
			strings.Join(cmds, "\n  "))
	}
	if _, err := os.Stat(w.cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the journal survived a teardown that undid everything")
	}
}

// TestAJournalFromAKilledProcessIsRecoveredBeforeAnythingElse.
//
// The journal exists because a stop can be a SIGKILL. The situation is built
// the way it actually arises: an applier writes real steps and the process
// "dies" without tearing them down, leaving the file on disk.
func TestAJournalFromAKilledProcessIsRecoveredBeforeAnythingElse(t *testing.T) {
	w := newWorld(t)
	leftBehind := abandonAJournal(t, w)

	rep, err := w.svc.ReplayJournal(context.Background())
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	// Counted against the steps that HAVE an inverse. One does not, on
	// purpose: netcfg's HotspotAddrSteps brings the radio up with no inverse,
	// because "bringing a radio down on teardown is worse than leaving it up".
	wantUndone := 0
	for _, s := range leftBehind {
		if !s.Undo.IsZero() {
			wantUndone++
		}
	}
	if len(rep.Results) != wantUndone {
		t.Fatalf("recovery replayed %d inverses and the killed process left %d changes behind that have one",
			len(rep.Results), wantUndone)
	}
	if rep.Failed != 0 {
		t.Fatalf("%d of the inverses failed, so the box is still carrying those changes", rep.Failed)
	}

	ran := map[string]bool{}
	for _, c := range w.runner.Commands() {
		ran[netcfg.RunnerKey(c)] = true
	}
	for _, s := range leftBehind {
		if s.Undo.IsZero() {
			continue
		}
		if !ran[netcfg.RunnerKey(s.Undo)] {
			t.Fatalf("recovery did not run %q, so the box is still carrying a change from a process that was killed",
				netcfg.RunnerKey(s.Undo))
		}
	}
	if _, err := os.Stat(w.cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the journal survived a recovery that undid everything in it")
	}
}

// TestAStartRecoversAJournalBeforeItPlans is the same rule at the other entry
// point: a start has to reach a machine in the state its detection describes.
func TestAStartRecoversAJournalBeforeItPlans(t *testing.T) {
	w := newWorld(t)
	leftBehind := abandonAJournal(t, w)
	undoOfFirst := netcfg.RunnerKey(leftBehind[0].Undo)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	// The inverse ran, and it ran before the new plan's own firewall load.
	if w.tl.indexOf("net: "+undoOfFirst) < 0 {
		t.Fatalf("the leftover change %q was never undone\ntimeline:%s", undoOfFirst, w.tl)
	}
	firstNft := w.tl.indexOf("net: nft -f -")
	lastUndo := w.tl.lastIndexOf("net: " + undoOfFirst)
	if lastUndo > firstNft {
		t.Fatalf("recovery ran after the new plan started being applied, so the plan was made against a "+
			"machine that was still carrying somebody else's changes\ntimeline:%s", w.tl)
	}
}

// abandonAJournal applies real steps through a real Applier and then walks away
// from it, which is exactly what a killed process leaves behind.
func abandonAJournal(t *testing.T, w *world) []netcfg.Step {
	t.Helper()
	ctx := context.Background()

	facts, plan, err := netcfg.DetectAndPlan(ctx, w.runner,
		[]netip.Addr{netip.MustParseAddr("203.0.113.7")}, w.cfg.netOptions())
	if err != nil {
		t.Fatalf("planning the previous run: %v", err)
	}
	steps := plan.PreEngineSteps(facts.Sysctl)

	ap, err := netcfg.NewApplier(w.runner, w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	if _, err := ap.Apply(ctx, steps); err != nil {
		t.Fatalf("applying the previous run's steps: %v", err)
	}
	// No Teardown and no Discard: the process was killed.
	if err := ap.Close(); err != nil {
		t.Fatalf("closing the journal: %v", err)
	}
	w.runner.Reset()

	entries, err := netcfg.LoadJournal(w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("reading the journal back: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("the abandoned journal is empty, so this test would prove nothing")
	}
	// Return them newest first, which is the order a replay runs them in.
	out := make([]netcfg.Step, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		out = append(out, netcfg.Step{Op: e.Op, Why: e.Why, Do: e.Do, Undo: e.Undo})
	}
	return out
}

// TestAStartCutOffByItsOwnDeadlineStillLeavesNothingApplied.
//
// This is the failure the other two rollback tests do not reach, and it is the
// likeliest one on a real box: the caller's deadline runs out part way through.
// A rollback that ran on the caller's context would then run every inverse
// against a context that is already cancelled, every one would fail at once,
// and the box would be left carrying exactly the changes the rollback exists to
// remove, while the log said a rollback had been attempted.
//
// The cancellation is triggered by the machine itself rather than by a timer,
// so the test cuts the start at a known point every time instead of racing one.
func TestAStartCutOffByItsOwnDeadlineStillLeavesNothingApplied(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel the moment the hotspot's address goes on, which is well inside
	// the pre-engine list: the firewall, five kernel knobs and the created
	// interface are all applied by then.
	w.cfg.Runner = cancellingRunner{
		inner: tracedRunner{inner: w.runner, tl: w.tl},
		on:    hotspotAddrCmd,
		stop:  cancel,
	}
	svc, err := New(w.cfg)
	if err != nil {
		t.Fatalf("building the service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	if err := svc.Start(ctx, startRequest(t)); err == nil {
		t.Fatalf("a start that was cancelled part way through reported success\ntimeline:%s", w.tl)
	}

	assertMachineRestored(t, w)
	if svc.isRunning() {
		t.Fatalf("the service reports itself as running after a start that was cancelled")
	}
	if _, statErr := os.Stat(w.cfg.JournalPath); !errors.Is(statErr, os.ErrNotExist) {
		entries, _ := netcfg.LoadJournal(w.cfg.JournalPath)
		t.Fatalf("the journal still exists after a rollback, holding %d entries", len(entries))
	}
}

// cancellingRunner cancels the start's context when a named command runs. It
// stands in for a deadline expiring at a moment this test can name.
type cancellingRunner struct {
	inner netcfg.Runner
	on    string
	stop  func()
}

func (r cancellingRunner) Run(ctx context.Context, c netcfg.Command) (netcfg.Result, error) {
	res, err := r.inner.Run(ctx, c)
	if netcfg.RunnerKey(c) == r.on {
		r.stop()
	}
	return res, err
}

// assertMachineRestored checks that every change that was made has had its
// inverse run.
//
// The expected step list is derived independently, by planning the same
// recorded machine again, rather than read back out of the service. A check
// that asked the service what it applied would agree with itself.
func assertMachineRestored(t *testing.T, w *world) {
	t.Helper()
	ctx := context.Background()

	probe := newRecordedMachine(fixtureIWList, fixtureIWRegGet)
	facts, plan, err := netcfg.DetectAndPlan(ctx, probe,
		[]netip.Addr{netip.MustParseAddr("203.0.113.7")}, w.cfg.netOptions())
	if err != nil {
		t.Fatalf("planning independently to work out what should have been undone: %v", err)
	}

	ran := map[string]int{}
	for _, c := range w.runner.Commands() {
		ran[netcfg.RunnerKey(c)]++
	}

	var missing []string
	for _, s := range plan.AllSteps(facts.Sysctl) {
		if s.Do.IsZero() || s.Undo.IsZero() {
			continue
		}
		if ran[netcfg.RunnerKey(s.Do)] == 0 {
			// Never applied, so nothing to undo.
			continue
		}
		if ran[netcfg.RunnerKey(s.Undo)] == 0 {
			missing = append(missing, netcfg.RunnerKey(s.Undo))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("these inverses were never run, so the box is left carrying the changes they undo:\n  %s\ntimeline:%s",
			strings.Join(missing, "\n  "), w.tl)
	}
}

// failHostapd makes the access point exit with a reason internal/hotspot does
// not recognise, which is the branch that has to report a hotspot failure
// rather than inventing a cause.
func failHostapd(w *world) {
	base := hotspot.DefaultResponder
	bin := w.cfg.HotspotPaths.HostapdBinary
	w.sys.Responder = func(rec *hotspot.Recorder, name string, args []string) (hotspot.Result, error) {
		if name == bin {
			return hotspot.Result{ExitCode: 1, Stderr: "hostapd: it did not work\n"}, nil
		}
		return base(rec, name, args)
	}
}
