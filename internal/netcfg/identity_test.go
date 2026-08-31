// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Two steps that differ only in what they put on standard input are two steps.
//
// Every nftables load here is "nft -f -", so the argument vector is identical
// for the ordinary ruleset, the cut one and the teardown. A key that ignores
// stdin makes them one command.
func TestRunnerKey_DistinguishesCommandsThatDifferOnlyInStdin(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	_ = f

	firewall := p.FirewallStep().Do
	cut := p.CutStep().Do
	teardown := p.FirewallStep().Undo

	// Identical to a reader, and to any key built from path and arguments.
	if CommandLine(firewall) != CommandLine(cut) || CommandLine(cut) != CommandLine(teardown) {
		t.Fatalf("these three no longer share a command line, so this test guards nothing:\n %s\n %s\n %s",
			CommandLine(firewall), CommandLine(cut), CommandLine(teardown))
	}

	keys := map[string]string{
		RunnerKey(firewall): "firewall",
		RunnerKey(cut):      "cut",
		RunnerKey(teardown): "teardown",
	}
	if len(keys) != 3 {
		t.Fatalf("three different rulesets collapsed to %d identity keys: %v", len(keys), keys)
	}

	// A command with no stdin is unchanged, so existing keys keep working.
	plain := Command{Path: BinIP, Args: []string{"-br", "addr"}}
	if RunnerKey(plain) != "ip -br addr" || CommandLine(plain) != "ip -br addr" {
		t.Errorf("a command with no stdin should key as before: %q / %q", RunnerKey(plain), CommandLine(plain))
	}
}

// The false green, end to end.
//
// MEASURED on hardware: the cut was applied through Applier.Apply after the
// firewall step was already journalled, the applier skipped it as already
// done, zero rulesets were loaded, success was reported, and the box went on
// forwarding client traffic.
func TestApply_CutIsNotSkippedAsTheFirewallStep(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	k := capturedKernel(t)
	k.Reads = pi5Captured().runner(t)
	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ctx, p.AllSteps(f.Sysctl)); err != nil {
		t.Fatal(err)
	}

	// The firewall is now in the journal as done. Cut, through the same
	// applier, exactly as the privileged service does it.
	rep, err := a.Apply(ctx, []Step{p.CutStep()})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("the cut was skipped as already applied: %v", rep.Lines())
	}

	// And the kernel actually received the cut ruleset.
	var loaded []string
	for _, c := range k.Commands() {
		if c.Path == BinNft {
			loaded = append(loaded, c.Stdin)
		}
	}
	if len(loaded) != 2 {
		t.Fatalf("nft was asked to load %d rulesets, want 2 (firewall then cut)", len(loaded))
	}
	if !strings.Contains(loaded[1], "client traffic cut by the user") {
		t.Error("the second load was not the cut ruleset")
	}

	// Restoring afterwards must also run, for the same reason.
	rep, err = a.Apply(ctx, []Step{p.RestoreStep()})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("the restore was skipped: %v", rep.Lines())
	}
}

// "No such object" means opposite things in the two directions, so the
// tolerance for it must know which way the command points.
func TestCommandRemoves_DirectionDecidesWhetherNotFoundIsSuccess(t *testing.T) {
	removals := []Command{
		{Path: BinIP, Args: []string{"address", "del", "10.0.0.222/24", "dev", "wlan0"}},
		{Path: BinIP, Args: []string{"route", "del", "default", "dev", "xray0", "table", "8410"}},
		{Path: BinIP, Args: []string{"-6", "route", "del", "2001:db8::1/128", "dev", "eth0"}},
		{Path: BinIP, Args: []string{"rule", "del", "from", "10.83.51.0/24", "lookup", "8410"}},
		{Path: BinIw, Args: []string{"dev", "ap0", "del"}},
	}
	for _, c := range removals {
		if !commandRemoves(c) {
			t.Errorf("%s should be recognised as a removal", CommandLine(c))
		}
	}

	additions := []Command{
		{Path: BinIP, Args: []string{"address", "add", "10.83.51.1/24", "dev", "ap0"}},
		{Path: BinIP, Args: []string{"route", "add", "default", "dev", "xray0", "table", "8410"}},
		{Path: BinIP, Args: []string{"rule", "add", "from", "10.83.51.0/24", "lookup", "8410"}},
		{Path: BinIw, Args: []string{"phy", "phy0", "interface", "add", "ap0", "type", "__ap"}},
		{Path: BinIw, Args: []string{"dev", "wlan0", "set", "type", "__ap"}},
		{Path: BinSysctl, Args: []string{"-w", "net.ipv4.ip_forward=1"}},
		{Path: BinNmcli, Args: []string{"device", "set", "wlan0", "managed", "no"}},
	}
	for _, c := range additions {
		if commandRemoves(c) {
			t.Errorf("%s must NOT be treated as a removal: 'no such object' there means the "+
				"thing being added to is missing, which is a real failure", CommandLine(c))
		}
	}
}

// An inverse that ADDS must not be marked undone because the device is gone.
// Doing so drops the entry and loses the restoration silently.
func TestTeardown_AFailedRestoreIsNotMistakenForNothingToDo(t *testing.T) {
	ctx := context.Background()
	r := NewRecordingRunner()
	r.SetError("ip address add 10.0.0.222/24 dev wlan0",
		errors.New("Cannot find device \"wlan0\""))
	path := tmpJournal(t)
	a, _ := NewApplier(r, path)

	restore := Step{
		Op:   OpAddr,
		Do:   Command{Path: BinIP, Args: []string{"address", "del", "10.0.0.222/24", "dev", "wlan0"}},
		Undo: Command{Path: BinIP, Args: []string{"address", "add", "10.0.0.222/24", "dev", "wlan0"}},
	}
	if _, err := a.Apply(ctx, []Step{restore}); err != nil {
		t.Fatal(err)
	}

	rep, err := a.Teardown(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 {
		t.Errorf("failed = %d, want 1: an address that could not be put back is a failure, "+
			"not nothing to do", rep.Failed)
	}
	left, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 {
		t.Errorf("the journal dropped an entry whose restoration never happened: %+v", left)
	}
}

// D1: the journal is not the kernel.
//
// Applier.Apply skips a step whose change the journal records as in force.
// That is a claim about a file this package wrote. If the ruleset is gone and
// the journal is not, the appliance believes it is protected while nothing
// filters. Nothing else in this package reads the firewall back.
func TestAssertFirewallLoaded(t *testing.T) {
	ctx := context.Background()
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	// Loaded, by the plan's own step.
	k := capturedKernel(t)
	k.Reads = pi5Captured().runner(t)
	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ctx, p.AllSteps(f.Sysctl)); err != nil {
		t.Fatal(err)
	}
	if err := AssertFirewallLoaded(ctx, k, p); err != nil {
		t.Fatalf("the firewall was just loaded and the readback denies it: %v", err)
	}

	// The failure D1 names: the journal still says loaded, the kernel does
	// not have it. Anything flushing the ruleset from outside produces this.
	if _, err := k.Run(ctx, Command{Path: BinNft, Args: []string{"-f", "-"}, Stdin: p.RulesetTeardown()}); err != nil {
		t.Fatal(err)
	}
	err = AssertFirewallLoaded(ctx, k, p)
	if !errors.Is(err, ErrFirewallNotLoaded) {
		t.Fatalf("err = %v, want ErrFirewallNotLoaded", err)
	}
	// And the journal, which the applier trusts, still claims otherwise.
	state, err := a.currentState()
	if err != nil {
		t.Fatal(err)
	}
	if state[CommandLine(p.FirewallStep().Do)] != RunnerKey(p.FirewallStep().Do) {
		t.Error("the journal no longer claims the firewall is in force, so this test guards nothing")
	}

	// The two checks are independent and both are pinned. A command that
	// fails while still printing something must be read as "not loaded",
	// which the stdout check alone would miss.
	noisy := NewRecordingRunner()
	noisy.Responses["nft list table inet "+TableName] = Result{
		Stdout: "table inet " + TableName + " {\n", ExitCode: 1}
	noisy.SetError("nft list table inet "+TableName, errors.New("Error: No such file or directory"))
	err = AssertFirewallLoaded(ctx, noisy, p)
	if !errors.Is(err, ErrFirewallNotLoaded) {
		t.Fatalf("err = %v, want ErrFirewallNotLoaded when the listing command itself failed", err)
	}

	// A table of the right name carrying something else is a different fault
	// from an absent one, and must be reported as such.
	other := NewRecordingRunner()
	other.SetOutput("nft list table inet "+TableName,
		"table inet "+TableName+" {\n\tchain forward {\n\t\ttype filter hook forward priority filter; policy accept;\n\t}\n}\n")
	err = AssertFirewallLoaded(ctx, other, p)
	if !errors.Is(err, ErrFirewallUnrecognised) {
		t.Fatalf("err = %v, want ErrFirewallUnrecognised", err)
	}
	contains(t, err.Error(), "nft listed:")

	// The cut ruleset still carries the leak block, so a cut box must pass.
	cutK := capturedKernel(t)
	cutK.Reads = pi5Captured().runner(t)
	if _, err := cutK.Run(ctx, p.CutStep().Do); err != nil {
		t.Fatal(err)
	}
	if err := AssertFirewallLoaded(ctx, cutK, p); err != nil {
		t.Errorf("a cut box is still protected and must pass: %v", err)
	}
}
