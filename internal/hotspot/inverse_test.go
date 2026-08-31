// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// docs/DEFECTS.md D2: two changes this package makes to a machine had no
// inverse. A box found with its radio switched off was left with it switched
// on, and the generated configuration, one of which carries the WPA2
// passphrase, survived a stop.

// rfkillListing renders "rfkill list" output for a set of devices, in the
// layout the real one prints and the layout parseRfkillList reads.
func rfkillListing(devs ...RfkillDevice) string {
	yes := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	var b strings.Builder
	for _, d := range devs {
		typ := d.Type
		if typ == "" {
			typ = "Wireless LAN"
		}
		fmt.Fprintf(&b, "%d: %s: %s\n\tSoft blocked: %s\n\tHard blocked: %s\n",
			d.Index, d.Name, typ, yes(d.SoftBlocked), yes(d.HardBlocked))
	}
	return b.String()
}

// radioMachine is an rfkill this test can drive: it answers "list" from state
// and applies "block" and "unblock" to it, so a test asserts on what the
// machine ENDED UP as and not merely on which commands were sent.
type radioMachine struct {
	devs    []RfkillDevice
	blocks  []string // the arguments of every block command, in order
	refuse  bool     // block runs and changes nothing, which is the failure a readback catches
	missing bool     // no rfkill on this machine at all
}

func (m *radioMachine) install(rec *Recorder) {
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if !strings.HasSuffix(name, "rfkill") {
			return base(r, name, args)
		}
		if m.missing {
			return Result{}, fmt.Errorf("exec: %q: executable file not found", name)
		}
		switch {
		case len(args) > 0 && args[0] == "list":
			return Result{Stdout: rfkillListing(m.devs...)}, nil
		case len(args) > 1 && args[0] == "unblock":
			for i := range m.devs {
				m.devs[i].SoftBlocked = false
			}
			return Result{}, nil
		case len(args) > 1 && args[0] == "block":
			m.blocks = append(m.blocks, args[1])
			if m.refuse {
				return Result{}, nil
			}
			for i := range m.devs {
				if fmt.Sprint(m.devs[i].Index) == args[1] {
					m.devs[i].SoftBlocked = true
				}
			}
			return Result{}, nil
		}
		return Result{}, nil
	}
}

// TestARadioThisProgramSwitchedOnIsSwitchedBackOffByStop.
//
// The whole of D2a. This platform is frequently found soft blocked, so this is
// the ordinary path and not an edge case.
func TestARadioThisProgramSwitchedOnIsSwitchedBackOffByStop(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	st, err := s.Start(ctx, testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !st.Radio.Unblocked {
		t.Fatal("the radio was not recorded as switched on by this program, so this test proves nothing")
	}

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.blocks) != 1 || m.blocks[0] != "0" {
		t.Fatalf("blocks = %v, want exactly one, of device 0: a box found with its radio off must be left "+
			"with it off", m.blocks)
	}
	if !m.devs[0].SoftBlocked {
		t.Error("the machine still reports the radio switched on after a stop")
	}
}

// TestARadioTheUserHadSwitchedOnIsLeftAloneByStop.
//
// The other half, and the one that decides whether this is an inverse or a
// change of its own. Nothing was unblocked, so nothing may be blocked: a user
// whose radio was on before they installed this must not find it off after
// switching the appliance off.
func TestARadioTheUserHadSwitchedOnIsLeftAloneByStop(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0"}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.blocks) != 0 {
		t.Errorf("a radio this program never switched on was switched off by a stop: %v", m.blocks)
	}
	if m.devs[0].SoftBlocked {
		t.Error("the machine reports the radio switched off after a stop that should have left it alone")
	}
}

// TestOnlyTheDevicesThisProgramUnblockedAreBlockedBack.
//
// "rfkill unblock wifi" clears every wireless device, so on a machine with a
// second adapter the inverse cannot be "rfkill block wifi": that would switch
// off an adapter that was never off, which is the same class of defect this is
// closing. The devices are recorded individually and put back individually.
func TestOnlyTheDevicesThisProgramUnblockedAreBlockedBack(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{
		{Index: 0, Name: "phy0", SoftBlocked: true},
		{Index: 1, Name: "phy1"}, // a USB adapter the user had switched on
	}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.blocks) != 1 || m.blocks[0] != "0" {
		t.Fatalf("blocks = %v, want only device 0", m.blocks)
	}
	if m.devs[1].SoftBlocked {
		t.Error("the second adapter was switched off, and this program never switched it on")
	}
}

// TestARadioSomebodyElseBlockedWhileRunningIsNotBlockedAgain.
//
// The state is read at stop rather than assumed from what it was at start. A
// device already blocked needs nothing, and issuing the command anyway would
// be this program acting on a picture of the machine instead of the machine.
func TestARadioSomebodyElseBlockedWhileRunningIsNotBlockedAgain(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Somebody switches it off underneath us while the appliance runs.
	m.devs[0].SoftBlocked = true

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.blocks) != 0 {
		t.Errorf("a device that was already switched off was switched off again: %v", m.blocks)
	}
}

// TestASecondStopDoesNotSwitchTheRadioOffAgain.
//
// Stop is called on a box that is already stopped by every rollback path in
// the privileged service, so a second call must not act on a record it has
// already used.
func TestASecondStopDoesNotSwitchTheRadioOffAgain(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	// The user switches the radio back on by hand between the two stops.
	m.devs[0].SoftBlocked = false

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	if len(m.blocks) != 1 {
		t.Errorf("the radio was switched off %d times for one start: %v", len(m.blocks), m.blocks)
	}
	if m.devs[0].SoftBlocked {
		t.Error("a second stop switched off a radio the user had switched back on")
	}
}

// TestABlockThatReturnedSuccessAndChangedNothingIsReported.
//
// The same failure the unblock already guards against, in the other direction:
// only a second read catches a command that succeeded and did nothing. Leaving
// the box with its radio on is a change this appliance made and did not undo,
// and it must not be reported as a clean stop.
func TestABlockThatReturnedSuccessAndChangedNothingIsReported(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}, refuse: true}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := s.Stop(ctx)
	if err == nil {
		t.Fatal("a stop that could not switch the radio back off reported success")
	}
	if !strings.Contains(err.Error(), "state it did not find it in") {
		t.Errorf("the failure does not say what was left behind: %v", err)
	}
}

// TestAMachineWithNoRfkillStopsCleanly.
//
// rfkill missing is not fatal to a start, so it must not be fatal to a stop
// either. Nothing was unblocked, so there is nothing to put back and no
// command to fail.
func TestAMachineWithNoRfkillStopsCleanly(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{missing: true}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop on a machine with no rfkill: %v", err)
	}
}

// TestStopRemovesTheGeneratedConfigurationFiles.
//
// docs/DEFECTS.md D2b. The hostapd file carries the WPA2 passphrase, and it
// outlived the thing that needed it.
func TestStopRemovesTheGeneratedConfigurationFiles(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	ctx := context.Background()

	plan := testPlan(t)
	if _, err := s.Start(ctx, plan); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// The passphrase really is in there, or the removal is protecting nothing.
	conf, ok := rec.Files[testPaths().HostapdConf]
	if !ok {
		t.Fatal("no hostapd configuration was written")
	}
	if !strings.Contains(string(conf), plan.AP.Passphrase) {
		t.Fatal("the generated configuration does not carry the passphrase, so this test is not about D2b")
	}

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	for _, p := range []string{testPaths().HostapdConf, testPaths().DnsmasqConf} {
		if !contains(rec.Removed, p) {
			t.Errorf("%s was not removed, so what this appliance wrote outlives it", p)
		}
		if _, still := rec.Files[p]; still {
			t.Errorf("%s is still on the machine after a stop", p)
		}
	}
}

// TestAStartAfterAStopWritesItsConfigurationAgain.
//
// The removal must not break the next start. Supervisor.Start decides whether
// to restart a daemon by comparing the file on disk with the plan, so a file
// that is gone has to read as changed rather than as unchanged.
func TestAStartAfterAStopWritesItsConfigurationAgain(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, err := s.Start(ctx, testPlan(t))
	if err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if !st.Running {
		t.Fatalf("the hotspot did not come back after a stop: %+v", st)
	}
	for _, p := range []string{testPaths().HostapdConf, testPaths().DnsmasqConf} {
		if _, ok := rec.Files[p]; !ok {
			t.Errorf("%s was not written again by the second start", p)
		}
	}
}

// listFailsAfter makes "rfkill list" work for the first n calls and fail after
// that, so a test can put the failure on either side of the block command
// without touching anything else about the machine.
type listFailsAfter struct {
	m *radioMachine
	n int
	c int
}

func (f *listFailsAfter) install(rec *Recorder) {
	f.m.install(rec)
	inner := rec.Responder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "list" {
			f.c++
			if f.c > f.n {
				return Result{ExitCode: 1, Stderr: "rfkill: cannot open /dev/rfkill"},
					fmt.Errorf("rfkill list refused")
			}
		}
		return inner(r, name, args)
	}
}

// TestAStopThatCannotReadTheRadioSaysSoRatherThanGuessing.
//
// The re-block reads the current state first, so that a device somebody else
// has already switched off, or that has been unplugged, is left alone. If that
// read fails there is no safe assumption available: blocking anyway could
// switch off an adapter this program never touched, and skipping silently
// leaves the machine changed with nobody told. It reports.
func TestAStopThatCannotReadTheRadioSaysSoRatherThanGuessing(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	// Two list calls happen during Start, before and after the unblock. The
	// third is the one at the top of the re-block.
	f := &listFailsAfter{m: m, n: 2}
	f.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := s.Stop(ctx)
	if err == nil {
		t.Fatal("a stop that could not read the radio state reported success")
	}
	if !strings.Contains(err.Error(), "before switching it back off") {
		t.Errorf("the error does not say which read failed: %v", err)
	}
	if len(m.blocks) != 0 {
		t.Errorf("the radio was switched off from a state nobody could read: %v", m.blocks)
	}
}

// TestAStopThatCannotConfirmTheRadioWentOffSaysSo.
//
// The second read is the one that catches a block command which returns
// success and changes nothing, which is the failure the whole readback exists
// for. Losing the ability to make that check is itself worth reporting: the
// block was sent and nobody can say whether it landed.
func TestAStopThatCannotConfirmTheRadioWentOffSaysSo(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	// Three succeed: two during Start, one at the top of the re-block. The
	// fourth, the confirmation, fails.
	f := &listFailsAfter{m: m, n: 3}
	f.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := s.Stop(ctx)
	if err == nil {
		t.Fatal("a stop that could not confirm the radio went off reported success")
	}
	if !strings.Contains(err.Error(), "confirming") {
		t.Errorf("the error does not say the confirmation was what failed: %v", err)
	}
	if len(m.blocks) == 0 {
		t.Error("the block was never sent, so this test is not covering the confirmation path")
	}
}

// TestAnAdapterUnpluggedWhileRunningIsNotAFailure.
//
// The re-block walks the devices it recorded and reads the current state to
// decide what to do with each. A device that is no longer in that reading has
// been unplugged, and there is nothing to put back. Saying so is not a failure,
// and reporting one would make a stop look broken because somebody pulled a USB
// adapter out.
func TestAnAdapterUnpluggedWhileRunningIsNotAFailure(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// The adapter leaves the machine between the start and the stop.
	m.devs = nil

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("a stop after an adapter was unplugged reported a failure: %v", err)
	}
	if len(m.blocks) != 0 {
		t.Errorf("a block was sent for a device that is not on the machine: %v", m.blocks)
	}
}

// TestARadioBlockedBySomebodyElseIsLeftAlone covers the other reason to skip a
// device: it is already off. Blocking it again would be harmless and pointless,
// and the branch exists so the count of commands sent stays honest.
func TestARadioBlockedBySomebodyElseIsLeftAlone(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Somebody switches the radio off while the appliance is running.
	m.devs[0].SoftBlocked = true

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.blocks) != 0 {
		t.Errorf("a device that was already switched off was switched off again: %v", m.blocks)
	}
}

// TestAStopReportsAConfigurationItCouldNotRemove.
//
// The generated hotspot configuration holds the WPA passphrase, so a stop that
// leaves it behind has left a credential on the machine. That is worth a
// message rather than a silent skip, and the message names which file.
func TestAStopReportsAConfigurationItCouldNotRemove(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0"}}}
	m.install(rec)
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rec.RemoveErr = fmt.Errorf("read-only file system")

	err := s.Stop(ctx)
	if err == nil {
		t.Fatal("a stop that could not remove the generated configuration reported success")
	}
	if !strings.Contains(err.Error(), "configuration") {
		t.Errorf("the error does not say a configuration file was left behind: %v", err)
	}
}

// TestAStopReportsARadioItCouldNotSwitchOff.
//
// The block command itself failing is different from the confirmation failing:
// here nothing was even attempted successfully, and the machine is left with a
// radio this program switched on. It is reported rather than swallowed, because
// the user's machine has been changed and nobody else is going to notice.
func TestAStopReportsARadioItCouldNotSwitchOff(t *testing.T) {
	rec := NewRecorder()
	m := &radioMachine{devs: []RfkillDevice{{Index: 0, Name: "phy0", SoftBlocked: true}}}
	m.install(rec)
	inner := rec.Responder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") && len(args) > 1 && args[0] == "block" {
			return Result{ExitCode: 1, Stderr: "rfkill: cannot block device"},
				fmt.Errorf("rfkill block refused")
		}
		return inner(r, name, args)
	}
	s := newTestSupervisor(rec)
	ctx := context.Background()

	if _, err := s.Start(ctx, testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := s.Stop(ctx)
	if err == nil {
		t.Fatal("a stop that could not switch the radio back off reported success")
	}
	if !strings.Contains(err.Error(), "switching the wireless adapter back off") {
		t.Errorf("the error does not name what failed: %v", err)
	}
}
