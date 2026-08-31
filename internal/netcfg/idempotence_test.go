// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"testing"
)

// capturedKernel builds a machine in the state the target is in before
// anything is applied: the two real interfaces, the tunnel device the engine
// has already created, and the knob values measured on the box.
func capturedKernel(t *testing.T) *SimulatedKernel {
	t.Helper()
	k := NewSimulatedKernel("lo", "eth0", "wlan0", "xray0")
	k.Preload("sysctl", "net.ipv4.ip_forward", "0")
	k.Preload("sysctl", "net.ipv4.conf.all.rp_filter", "0")
	k.Preload("sysctl", "net.ipv4.conf.default.rp_filter", "2")
	k.Preload("sysctl", "net.ipv6.conf.all.forwarding", "0")
	return k
}

// Before trusting anything below: the double must actually refuse.
//
// A test suite that runs against a permissive double proves nothing about
// re-apply, and that is precisely how this class stayed invisible. So the
// first assertion is that this kernel says no.
func TestSimulatedKernel_RefusesTheWayTheKernelDoes(t *testing.T) {
	ctx := context.Background()
	k := NewSimulatedKernel("eth0")

	add := Command{Path: BinIP, Args: []string{"address", "add", "10.0.0.5/24", "dev", "eth0"}}
	if _, err := k.Run(ctx, add); err != nil {
		t.Fatalf("first address add should succeed: %v", err)
	}
	res, err := k.Run(ctx, add)
	if err == nil {
		t.Fatal("second address add must fail: a permissive double cannot see this class at all")
	}
	if !IsAlreadyExists(res, err) {
		t.Errorf("second add failed with %q, which IsAlreadyExists does not recognise", res.Stderr)
	}

	route := Command{Path: BinIP, Args: []string{"route", "add", "198.51.100.0/24", "dev", "eth0", "table", "8410"}}
	if _, err := k.Run(ctx, route); err != nil {
		t.Fatal(err)
	}
	if res, err := k.Run(ctx, route); err == nil || !IsAlreadyExists(res, err) {
		t.Errorf("second route add: err=%v stderr=%q, want a File exists refusal", err, res.Stderr)
	}

	// Rules. MEASURED on the target: an identical selector at the same
	// explicit priority is REFUSED, not duplicated. An earlier version of
	// this double modelled it as succeeding, which taught a false model in
	// the safe direction. Wrong in the safe direction is still wrong.
	rule := Command{Path: BinIP, Args: []string{"rule", "add", "from", "10.0.0.0/24", "lookup", "8410", "priority", "8410"}}
	if _, err := k.Run(ctx, rule); err != nil {
		t.Fatalf("first rule add should succeed: %v", err)
	}
	if res, err := k.Run(ctx, rule); err == nil || !IsAlreadyExists(res, err) {
		t.Errorf("second identical rule add: err=%v stderr=%q, want a File exists refusal", err, res.Stderr)
	}
	if got := len(k.Rules()); got != 1 {
		t.Fatalf("rules = %v, want exactly one: the kernel refuses the duplicate", k.Rules())
	}

	// The shape that DOES duplicate, and the reason every rule this package
	// generates carries an explicit priority: with none given, the kernel
	// assigns one, so two adds become two rules at different priorities and
	// which table wins changes.
	noPrio := Command{Path: BinIP, Args: []string{"rule", "add", "from", "10.9.9.0/24", "lookup", "8410"}}
	for i := 0; i < 2; i++ {
		if _, err := k.Run(ctx, noPrio); err != nil {
			t.Fatalf("an add with no priority must succeed every time: %v", err)
		}
	}
	if got := len(k.Rules()); got != 3 {
		t.Fatalf("rules = %v, want 3: two priority-less adds are two rules", k.Rules())
	}

	// iw. MEASURED on an adjacent case: the refusal is "Invalid exchange
	// (-52)", which IsAlreadyExists deliberately does NOT match, because the
	// wording for the case this package reaches was never measured and
	// guessing an errno from a neighbouring one is how a confident wrong
	// sentence gets written. A second apply survives this because the step
	// asks before it acts, not because a string comparison happened to work.
	iface := Command{Path: BinIw, Args: []string{"phy", "phy0", "interface", "add", "ap0", "type", "__ap"}}
	if _, err := k.Run(ctx, iface); err != nil {
		t.Fatal(err)
	}
	res, err = k.Run(ctx, iface)
	if err == nil {
		t.Fatal("second interface add must fail")
	}
	if IsAlreadyExists(res, err) {
		t.Errorf("iw's refusal %q is being matched as an already-exists error; that match is "+
			"unconfirmed for iw and nothing should depend on it", res.Stderr)
	}

	// And the query the creation step relies on instead.
	if r, err := k.Run(ctx, Command{Path: BinIP, Args: []string{"-br", "link", "show", "dev", "ap0"}}); err != nil || r.Stdout == "" {
		t.Errorf("existence query for an interface that exists: err=%v stdout=%q", err, r.Stdout)
	}
	if _, err := k.Run(ctx, Command{Path: BinIP, Args: []string{"-br", "link", "show", "dev", "nope0"}}); err == nil {
		t.Error("existence query for an absent interface must fail, which is how absence is detected")
	}
}

// Pressing connect twice must converge. A person pressing a button twice is
// not an error.
func TestApply_TwiceConverges(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	k := capturedKernel(t)
	before := k.Snapshot()

	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	first, err := a.Apply(ctx, p.AllSteps(f.Sysctl))
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if first.Skipped != 0 {
		t.Errorf("first apply skipped %d steps on an empty machine", first.Skipped)
	}
	afterFirst := k.Snapshot()

	second, err := a.Apply(ctx, p.AllSteps(f.Sysctl))
	if err != nil {
		t.Fatalf("second apply must converge, not fail: %v", err)
	}
	if second.Failed != 0 {
		t.Errorf("second apply reported %d failures: %v", second.Failed, second.Err())
	}
	if second.Skipped != len(second.Results) {
		t.Errorf("second apply changed something: %d of %d steps were not skipped",
			len(second.Results)-second.Skipped, len(second.Results))
	}

	// The machine must be identical after the second apply.
	afterSecond := k.Snapshot()
	wantSequence(t, "machine state after a second apply", afterSecond, afterFirst)

	// And the duplicate that would announce itself nowhere.
	rules := k.Rules()
	seen := map[string]int{}
	for _, r := range rules {
		seen[r]++
	}
	for r, n := range seen {
		if n != 1 {
			t.Errorf("rule %q installed %d times; duplicates change which table wins", r, n)
		}
	}
	if len(rules) != 2 {
		t.Errorf("rules = %v, want exactly the two this plan installs", rules)
	}

	// Teardown must put the machine back exactly.
	rep, err := a.Teardown(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 {
		t.Errorf("teardown failures: %v", rep.Err())
	}
	wantSequence(t, "machine state after teardown", k.Snapshot(), before)
}

// The trap a second apply sets for the journal.
//
// After the first apply the machine reports the values this package wrote. A
// second plan built from a fresh detection therefore derives its inverse from
// those, so "restore rp_filter" would mean "set it to 2" rather than "set it
// to 0". Teardown would leave the box changed. This is the same defect as the
// sysctl read that returned nothing, arriving through a different door.
func TestApply_SecondApplyDoesNotOverwriteTheHonestInverse(t *testing.T) {
	ctx := context.Background()
	sc := pi5Captured()
	k := capturedKernel(t)
	k.Reads = sc.runner(t) // detection fixtures for everything the kernel does not model
	before := k.Snapshot()

	path := tmpJournal(t)
	a, err := NewApplier(k, path)
	if err != nil {
		t.Fatal(err)
	}

	// First connect: detect, plan, apply.
	f1, p1, err := DetectAndPlan(ctx, k, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if f1.Sysctl["net.ipv4.conf.all.rp_filter"] != "0" {
		t.Fatalf("first detection read rp_filter as %q, want the machine's 0", f1.Sysctl["net.ipv4.conf.all.rp_filter"])
	}
	if _, err := a.Apply(ctx, p1.AllSteps(f1.Sysctl)); err != nil {
		t.Fatal(err)
	}

	// Second connect: detection now sees what we wrote.
	f2, p2, err := DetectAndPlan(ctx, k, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if f2.Sysctl["net.ipv4.conf.all.rp_filter"] != "2" {
		t.Fatalf("second detection read rp_filter as %q, want the 2 this package wrote; "+
			"the trap this test guards does not exist if detection cannot see it",
			f2.Sysctl["net.ipv4.conf.all.rp_filter"])
	}
	// The freshly derived step would indeed record the wrong inverse.
	for _, s := range p2.AllSteps(f2.Sysctl) {
		if RunnerKey(s.Do) == "sysctl -w net.ipv4.conf.all.rp_filter=2" {
			if RunnerKey(s.Undo) != "sysctl -w net.ipv4.conf.all.rp_filter=2" {
				t.Fatalf("expected the second plan to derive the wrong inverse %q; if it no longer does, "+
					"this test is guarding nothing", RunnerKey(s.Undo))
			}
		}
	}
	if _, err := a.Apply(ctx, p2.AllSteps(f2.Sysctl)); err != nil {
		t.Fatal(err)
	}

	// The journal must still carry the first, honest inverse and only that.
	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	var inverses []string
	for _, e := range entries {
		if e.NeedsUndo() && strings.Contains(RunnerKey(e.Do), "conf.all.rp_filter") {
			inverses = append(inverses, RunnerKey(e.Undo))
		}
	}
	wantSequence(t, "recorded inverses for conf.all.rp_filter", inverses,
		[]string{"sysctl -w net.ipv4.conf.all.rp_filter=0"})

	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	wantSequence(t, "machine state after teardown", k.Snapshot(), before)
}

// What was already on the machine and is not ours must be left alone, and must
// not be journalled. Undoing it would delete an address, route or rule that
// existed before this program ran, which is the same harm as the guessed
// sysctl inverse that was retired.
func TestApply_LeavesPreexistingObjectsAloneAndDoesNotJournalThem(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	k := capturedKernel(t)
	// Somebody else's route to the same server address, and somebody else's
	// rule at the priority this package uses.
	k.Preload("route", "main", "203.0.113.10/32")
	k.Preload("rule", "8410", "from 10.83.51.0/24 lookup 8410")
	before := k.Snapshot()

	path := tmpJournal(t)
	a, err := NewApplier(k, path)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := a.Apply(ctx, p.AllSteps(f.Sysctl))
	if err != nil {
		t.Fatalf("a pre-existing object must not stop the apply: %v", err)
	}
	if rep.Skipped != 2 {
		t.Errorf("skipped %d steps, want the 2 that were already present", rep.Skipped)
	}

	// Neither may have an inverse recorded against it.
	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.NeedsUndo() {
			continue
		}
		u := RunnerKey(e.Undo)
		if strings.Contains(u, "route del 203.0.113.10/32") {
			t.Error("journalled a delete for a route this package did not add")
		}
		if strings.Contains(u, "rule del from 10.83.51.0/24") {
			t.Error("journalled a delete for a rule this package did not add")
		}
	}

	// The rule must not have been duplicated either.
	if got := len(k.Rules()); got != 2 {
		t.Errorf("rules = %v, want the pre-existing one plus the fwmark rule only", k.Rules())
	}

	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}

	// Teardown must leave the pre-existing objects exactly where they were.
	after := k.Snapshot()
	for _, want := range []string{"route main 203.0.113.10/32", "rule 8410 from 10.83.51.0/24 lookup 8410"} {
		found := false
		for _, got := range after {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("teardown removed %q, which was on the machine before Caspian ran", want)
		}
	}
	wantSequence(t, "machine state after teardown", after, before)
}

// Apply, teardown, apply again: the second cycle must work on a machine that
// has been returned to its original state.
func TestApply_TeardownThenApplyAgain(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	k := capturedKernel(t)
	before := k.Snapshot()

	for cycle := 1; cycle <= 2; cycle++ {
		path := tmpJournal(t)
		a, err := NewApplier(k, path)
		if err != nil {
			t.Fatalf("cycle %d: %v", cycle, err)
		}
		if _, err := a.Apply(ctx, p.AllSteps(f.Sysctl)); err != nil {
			t.Fatalf("cycle %d apply: %v", cycle, err)
		}
		rep, err := a.Teardown(ctx)
		if err != nil {
			t.Fatalf("cycle %d teardown: %v", cycle, err)
		}
		if rep.Failed != 0 {
			t.Errorf("cycle %d teardown failures: %v", cycle, rep.Err())
		}
		wantSequence(t, "machine state after cycle", k.Snapshot(), before)
	}
}

// A crash between the journal entry and the command leaves the state unknown.
// Recovery replays the inverse, and the inverse may find nothing to remove;
// that must not stop the rest of the teardown.
func TestRecover_AfterACrashMidApplyReturnsTheMachine(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	k := capturedKernel(t)
	before := k.Snapshot()
	path := tmpJournal(t)

	func() {
		a, err := NewApplier(k, path)
		if err != nil {
			t.Fatal(err)
		}
		steps := p.AllSteps(f.Sysctl)
		if _, err := a.Apply(ctx, steps[:6]); err != nil {
			t.Fatal(err)
		}
		// Killed: no Close, no Teardown, and one step half-recorded.
		if _, err := a.j.Begin(steps[6]); err != nil {
			t.Fatal(err)
		}
	}()

	rep, err := Recover(ctx, k, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) == 0 {
		t.Fatal("recovery replayed nothing")
	}
	wantSequence(t, "machine state after recovery", k.Snapshot(), before)
}

// The case the interface-existence precondition exists for.
//
// The access point interface survives, and the journal does not: /var/lib was
// wiped, or the box came up after an unclean shutdown that Recover never got
// to. Nothing then records that ap0 is ours, so the journal-aware skip cannot
// help, and "iw phy ... interface add" refuses with a wording IsAlreadyExists
// deliberately does not match. Without the precondition the apply stops there
// and the appliance does not start.
//
// This is the scenario that a test run against an always-succeeding double
// cannot produce at all: it needs a kernel that refuses, in a wording nobody
// has measured for this exact call.
func TestApply_ConvergesWhenTheInterfaceExistsButTheJournalDoesNot(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	k := capturedKernel(t)
	k.Preload("iface", "ap0", "") // left behind by a previous run
	before := k.Snapshot()

	path := tmpJournal(t)
	a, err := NewApplier(k, path) // a journal that knows nothing
	if err != nil {
		t.Fatal(err)
	}

	rep, err := a.Apply(ctx, p.AllSteps(f.Sysctl))
	if err != nil {
		t.Fatalf("apply must converge when the interface is already there: %v", err)
	}
	if rep.Failed != 0 {
		t.Errorf("failures: %v", rep.Err())
	}

	// The creation step must have been skipped by asking, not by the journal.
	skipped := false
	for _, r := range rep.Results {
		if strings.Contains(RunnerKey(r.Step.Do), "interface add ap0") {
			if !r.Skipped {
				t.Error("the interface creation was not skipped")
			}
			skipped = true
			if !strings.Contains(r.Reason, "already present on this machine") {
				t.Errorf("skipped for the wrong reason: %q", r.Reason)
			}
		}
	}
	if !skipped {
		t.Fatal("no interface creation step was generated, so this test guards nothing")
	}

	// Nothing may be journalled against it, so teardown must leave it.
	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.NeedsUndo() && strings.Contains(RunnerKey(e.Undo), "dev ap0 del") {
			t.Error("journalled a delete for an interface this run did not create")
		}
	}

	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	after := k.Snapshot()
	found := false
	for _, l := range after {
		if l == "iface ap0" {
			found = true
		}
	}
	if !found {
		t.Error("teardown removed an interface this run did not create")
	}

	// ONE difference from the state this run found, and it is deliberate.
	//
	// The interface was already there, so this run did not create it and
	// teardown does not remove it. The release from NetworkManager still ran,
	// and it has no inverse, so the leftover interface is left unmanaged.
	//
	// That is the better of the two outcomes rather than an oversight. Handing
	// a stale access point vif back to NetworkManager is what caused the
	// 2026-08-30 failure in the first place: NetworkManager takes such a
	// device to full management and flushes the address off it. The device is
	// this package's own ap0, not anything the user owns, and it disappears
	// with the next reboot.
	//
	// In the normal case there is no residue at all: the interface is created
	// by this run, teardown deletes it, and a device that does not exist has
	// no managed state. TestApply_TwiceConverges covers that.
	want := append(append([]string{}, before...), "unmanaged ap0")
	sort.Strings(want)
	wantSequence(t, "machine state after teardown", after, want)
}

// The existence predicate, pinned in both directions.
//
// MEASURED on the target 2026-08-30: an absent device exits 1 and prints
// `Device "nope0" does not exist.` to STDERR; a present device exits 0 and
// prints the interface line to stdout. So the absent case is not silent, and a
// predicate that asked "is the output empty?" against a combined stream would
// read that message and conclude the device exists.
//
// Both clauses of the predicate are load-bearing, in opposite directions, and
// each case below fails if the other clause is removed.
func TestIfacePresent_KeysOnStatusAndOnEvidence(t *testing.T) {
	sat := ifacePresent("ap0").Satisfied

	cases := []struct {
		name string
		res  Result
		err  error
		want bool
		why  string
	}{
		{
			name: "absent, message on stderr as the box prints it",
			res:  Result{Stderr: `Device "ap0" does not exist.`, ExitCode: 1},
			err:  errors.New(`netcfg: ip exited 1: Device "ap0" does not exist.`),
			want: false,
			why:  "a non-zero exit is absence, whatever was printed",
		},
		{
			name: "absent, but a runner that merged stderr into stdout",
			res:  Result{Stdout: `Device "ap0" does not exist.`, ExitCode: 1},
			err:  errors.New(`netcfg: ip exited 1: Device "ap0" does not exist.`),
			want: false,
			why:  "the status clause must come first, or the error text reads as evidence of presence",
		},
		{
			name: "present",
			res:  Result{Stdout: "ap0              DOWN           <BROADCAST,MULTICAST>\n"},
			err:  nil,
			want: true,
			why:  "exit 0 with the interface line is the only definite yes",
		},
		{
			name: "a runner that reports success without answering",
			res:  Result{},
			err:  nil,
			want: false,
			why: "RecordingRunner returns an empty successful Result for any command it has no " +
				"response for; a status-only predicate would skip creating the interface",
		},
		{
			name: "success with only whitespace",
			res:  Result{Stdout: "   \n"},
			err:  nil,
			want: false,
			why:  "whitespace is not evidence",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sat(c.res, c.err); got != c.want {
				t.Errorf("Satisfied = %v, want %v: %s", got, c.want, c.why)
			}
		})
	}
}

// The integration form of the last case above: against a runner that says yes
// to everything and answers nothing, the interface must still be created. A
// status-only predicate would skip it and the plan would go on to address an
// interface that does not exist.
func TestApply_PermissiveRunnerStillCreatesTheInterface(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	r := NewRecordingRunner() // no responses registered: success, no output
	a, err := NewApplier(r, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if _, err := a.Apply(ctx, p.AllSteps(f.Sysctl)); err != nil {
		t.Fatal(err)
	}
	created := false
	for _, line := range r.Lines() {
		if line == "iw phy phy0 interface add ap0 type __ap" {
			created = true
		}
	}
	if !created {
		t.Errorf("the interface was never created against a runner that answers nothing:\n  %s",
			strings.Join(r.Lines(), "\n  "))
	}
}
