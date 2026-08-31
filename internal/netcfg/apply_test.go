// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestApply_RunsInOrderAndJournalsEveryInverse(t *testing.T) {
	r := NewRecordingRunner()
	path := tmpJournal(t)
	a, err := NewApplier(r, path)
	if err != nil {
		t.Fatal(err)
	}
	steps := []Step{step(OpAddr, "a"), step(OpRoute, "b"), step(OpRule, "c")}
	if _, err := a.Apply(context.Background(), steps); err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "applied", r.Lines(), []string{"ip addr add a", "ip route add b", "ip rule add c"})

	entries, err := a.Journal().Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("journal holds %d entries, want 3", len(entries))
	}
	for i, want := range []string{"ip addr del a", "ip route del b", "ip rule del c"} {
		if got := RunnerKey(entries[i].Undo); got != want {
			t.Errorf("inverse %d = %q, want %q", i, got, want)
		}
	}
	a.Close()
}

// Teardown replays the inverses newest first. A route removed after the
// address it depends on would fail; the reverse order is the whole point.
func TestTeardown_ReplaysInExactReverseOrder(t *testing.T) {
	r := NewRecordingRunner()
	a, err := NewApplier(r, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(context.Background(), []Step{step(OpAddr, "a"), step(OpRoute, "b"), step(OpRule, "c")}); err != nil {
		t.Fatal(err)
	}
	r.Reset()

	rep, err := a.Teardown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 {
		t.Errorf("failed = %d: %v", rep.Failed, rep.Err())
	}
	wantSequence(t, "teardown", r.Lines(), []string{"ip rule del c", "ip route del b", "ip addr del a"})
}

// A single inverse can fail for reasons that say nothing about the rest: the
// route was already gone, the interface has disappeared. Stopping there would
// leave every earlier change in place, which is the opposite of a teardown.
func TestTeardown_ContinuesPastAFailingInverse(t *testing.T) {
	r := NewRecordingRunner()
	// A genuine failure, not an already-gone one. "Cannot find device" would
	// now be read as "the object is already absent, so the inverse is done",
	// which is a different path with its own test below.
	r.SetError("ip route del b", errors.New("RTNETLINK answers: Operation not permitted"))
	path := tmpJournal(t)
	a, _ := NewApplier(r, path)
	if _, err := a.Apply(context.Background(), []Step{step(OpAddr, "a"), step(OpRoute, "b"), step(OpRule, "c")}); err != nil {
		t.Fatal(err)
	}
	r.Reset()

	rep, err := a.Teardown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "teardown", r.Lines(), []string{"ip rule del c", "ip route del b", "ip addr del a"})
	if rep.Failed != 1 {
		t.Errorf("failed = %d, want 1", rep.Failed)
	}
	if rep.Err() == nil || !strings.Contains(rep.Err().Error(), "Operation not permitted") {
		t.Errorf("report error = %v", rep.Err())
	}

	// What could not be undone stays in the journal for the next start.
	left, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || RunnerKey(left[0].Undo) != "ip route del b" {
		t.Fatalf("journal left %+v, want only the inverse that failed", left)
	}
}

// Tearing down twice must be safe. The second run has nothing to do, and the
// journal is gone.
func TestTeardown_IsIdempotent(t *testing.T) {
	r := NewRecordingRunner()
	path := tmpJournal(t)
	a, _ := NewApplier(r, path)
	a.Apply(context.Background(), []Step{step(OpAddr, "a"), step(OpRoute, "b")})
	if _, err := a.Teardown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a fully undone journal must be removed, stat err = %v", err)
	}

	r.Reset()
	rep, err := Recover(context.Background(), r, path)
	if err != nil {
		t.Fatalf("a second teardown must succeed: %v", err)
	}
	if len(rep.Results) != 0 || len(r.Lines()) != 0 {
		t.Errorf("second teardown ran %v", r.Lines())
	}
}

// The case the on-disk journal exists for: a process is killed mid-apply, and
// a completely different process undoes what it did.
func TestRecover_UndoesAJournalLeftByAKilledProcess(t *testing.T) {
	path := tmpJournal(t)

	// First "process": applies two steps, then begins a third and dies before
	// it completes. Nothing is closed and no teardown runs.
	func() {
		r := NewRecordingRunner()
		a, err := NewApplier(r, path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Apply(context.Background(), []Step{step(OpAddr, "a"), step(OpRoute, "b")}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.j.Begin(step(OpRule, "c")); err != nil {
			t.Fatal(err)
		}
		// No Done, no Close: the process is gone.
	}()

	// Second process: a fresh runner and no memory of the first.
	r2 := NewRecordingRunner()
	rep, err := Recover(context.Background(), r2, path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 {
		t.Errorf("failed = %d: %v", rep.Failed, rep.Err())
	}
	// The step that was in flight is undone too: a command killed halfway can
	// have landed part of its effect.
	wantSequence(t, "recovery", r2.Lines(), []string{"ip rule del c", "ip route del b", "ip addr del a"})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a fully recovered journal must be removed, stat err = %v", err)
	}
}

// Recovery covers routes, rules AND the firewall, not routes alone. A teardown
// that leaves the firewall loaded leaves the box blocking traffic it should
// now be forwarding normally.
func TestRecover_CoversRoutesRulesAndFirewall(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	path := tmpJournal(t)

	func() {
		r := NewRecordingRunner()
		a, err := NewApplier(r, path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Apply(context.Background(), p.AllSteps(f.Sysctl)); err != nil {
			t.Fatal(err)
		}
		// Killed: no Close, no Teardown.
	}()

	r2 := NewRecordingRunner()
	if _, err := Recover(context.Background(), r2, path); err != nil {
		t.Fatal(err)
	}

	ops := map[string]int{}
	for _, c := range r2.Commands() {
		switch {
		case c.Path == BinNft:
			ops["nft"]++
		case c.Path == BinSysctl:
			ops["sysctl"]++
		case c.Path == BinIw:
			ops["iw"]++
		case len(c.Args) > 0 && c.Args[0] == "rule":
			ops["rule"]++
		case len(c.Args) > 0 && (c.Args[0] == "route" || c.Args[0] == "-6"):
			ops["route"]++
		case len(c.Args) > 0 && c.Args[0] == "address":
			ops["addr"]++
		}
	}
	for _, want := range []string{"nft", "sysctl", "iw", "rule", "route", "addr"} {
		if ops[want] == 0 {
			t.Errorf("recovery did not undo any %s change; counts = %v", want, ops)
		}
	}

	// The firewall inverse must delete the table this package owns.
	sawDelete := false
	for _, c := range r2.Commands() {
		if c.Path == BinNft && strings.Contains(c.Stdin, "delete table inet "+TableName) {
			sawDelete = true
		}
	}
	if !sawDelete {
		t.Error("recovery never removed the generated firewall table")
	}

	// The very first inverse replayed must be the last change applied, and the
	// last inverse must be the firewall, which was applied first.
	lines := r2.Lines()
	if len(lines) == 0 {
		t.Fatal("recovery replayed nothing at all")
	}
	if lines[len(lines)-1] != "nft -f -" {
		t.Errorf("last inverse = %q, want the firewall teardown", lines[len(lines)-1])
	}
}

// Apply stops at the first failure rather than piling up dependent errors, and
// the failed step's inverse is journalled so teardown still reverses it.
func TestApply_StopsAtTheFirstFailureButStillJournalsIt(t *testing.T) {
	r := NewRecordingRunner()
	r.SetError("ip route add b", errors.New("RTNETLINK answers: Network is unreachable"))
	path := tmpJournal(t)
	a, _ := NewApplier(r, path)

	_, err := a.Apply(context.Background(), []Step{step(OpAddr, "a"), step(OpRoute, "b"), step(OpRule, "c")})
	if err == nil {
		t.Fatal("Apply must report the failure")
	}
	contains(t, err.Error(), "Network is unreachable")
	wantSequence(t, "applied", r.Lines(), []string{"ip addr add a", "ip route add b"})

	entries, _ := a.Journal().Entries()
	if len(entries) != 2 {
		t.Fatalf("journal holds %d entries, want 2 including the failed step", len(entries))
	}
	if entries[1].Phase != PhaseFailed || !entries[1].NeedsUndo() {
		t.Errorf("failed step must still be undone: %+v", entries[1])
	}
	a.Close()
}

func TestApply_SkipsStepsWithNoCommand(t *testing.T) {
	r := NewRecordingRunner()
	a, _ := NewApplier(r, tmpJournal(t))
	if _, err := a.Apply(context.Background(), []Step{{Op: "noop"}, step(OpAddr, "a")}); err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "applied", r.Lines(), []string{"ip addr add a"})
	a.Close()
}

// A step with no inverse is not replayed and does not count as a failure.
func TestTeardown_SkipsStepsWithNoInverse(t *testing.T) {
	r := NewRecordingRunner()
	a, _ := NewApplier(r, tmpJournal(t))
	noInverse := Step{Op: OpLink, Do: Command{Path: BinIP, Args: []string{"link", "set", "dev", "ap0", "up"}}}
	a.Apply(context.Background(), []Step{noInverse, step(OpAddr, "a")})
	r.Reset()

	rep, err := a.Teardown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "teardown", r.Lines(), []string{"ip addr del a"})
	if rep.Failed != 0 {
		t.Errorf("failed = %d", rep.Failed)
	}
}

// An inverse whose object is already gone has achieved what it wanted.
//
// Without this the journal never empties: an entry for a step that never took
// effect is retried on every start, fails for the same reason every time, and
// is retained for ever, so every start reports a failure nothing can fix. This
// is the mirror of the EEXIST rule on the apply side.
func TestTeardown_AnAlreadyGoneObjectCountsAsUndone(t *testing.T) {
	r := NewRecordingRunner()
	r.SetError("ip route del b", errors.New("RTNETLINK answers: No such process"))
	path := tmpJournal(t)
	a, _ := NewApplier(r, path)
	if _, err := a.Apply(context.Background(), []Step{step(OpAddr, "a"), step(OpRoute, "b")}); err != nil {
		t.Fatal(err)
	}

	rep, err := a.Teardown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 {
		t.Errorf("failed = %d, want 0: an object that is already gone is not a failure to undo", rep.Failed)
	}
	if rep.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", rep.Skipped)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the journal must be removed once nothing is outstanding, stat err = %v", err)
	}
}

// The case the target actually produced: a start that failed part way left one
// entry at "begin" for an interface creation that never took effect. Recovery
// must converge rather than retaining that entry for ever.
func TestRecover_AnUndoForAStepThatNeverTookEffectConverges(t *testing.T) {
	ctx := context.Background()
	k := NewSimulatedKernel("eth0", "wlan0")
	path := tmpJournal(t)
	a, err := NewApplier(k, path)
	if err != nil {
		t.Fatal(err)
	}
	// Exactly what the privileged service left behind: a begin with no
	// completion, for a command that did not take effect.
	s := Step{
		Op:   OpCreateIface,
		Do:   Command{Path: BinIw, Args: []string{"phy", "phy0", "interface", "add", "ap0", "type", "__ap"}},
		Undo: Command{Path: BinIw, Args: []string{"dev", "ap0", "del"}},
	}
	if _, err := a.j.Begin(s); err != nil {
		t.Fatal(err)
	}
	a.Close()

	rep, err := Recover(ctx, k, path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 {
		t.Errorf("failed = %d, want 0: ap0 was never created, so there is nothing to undo", rep.Failed)
	}
	left, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("journal retained %d entries; the next start would retry them for ever: %+v", len(left), left)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("journal file still present, stat err = %v", err)
	}
}
