// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// This file covers the Supervisor's FAILURE paths.
//
// They were unreachable from the existing suite for one structural reason: the
// Recorder's DefaultResponder emulates a machine on which everything works, so
// every "the machine said no" branch in the supervisor sat uncovered. That is
// not a criticism of the double, it is what the double is for. What was
// missing was a second double for the other half of the world.
//
// faultySystem is that. It wraps a Recorder and fails one named operation,
// leaving everything else working, so each test names exactly one thing that
// went wrong and asserts what the user is told about it.
//
// These are CHARACTERISATION tests: the production code already behaved
// correctly. What they add is that it cannot stop behaving correctly unnoticed.

// errMachine is the fault every injected failure returns, so a test can assert
// the supervisor propagated THIS error rather than inventing one.
var errMachine = errors.New("the machine said no")

// faultySystem is a System that delegates to a Recorder and fails whichever
// operations its hooks name. A nil hook means "behave normally".
type faultySystem struct {
	*Recorder

	failRun    func(name string, args []string) error
	failWrite  func(path string) error
	failRead   func(path string) error
	failRemove func(path string) error
	failAlive  func(pid int) error
	failSignal func(pid int) error
	failSleep  func() error

	// ignoresSignals models a process that will not die. The Recorder's own
	// SignalProcess marks the pid dead, which is a machine that behaves; this
	// is the other one, and it is the only way to reach the escalation to
	// SIGKILL.
	ignoresSignals bool
}

func newFaultySystem() *faultySystem { return &faultySystem{Recorder: NewRecorder()} }

func (f *faultySystem) Run(ctx context.Context, name string, args ...string) (Result, error) {
	if f.failRun != nil {
		if err := f.failRun(name, args); err != nil {
			return Result{}, err
		}
	}
	return f.Recorder.Run(ctx, name, args...)
}

func (f *faultySystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if f.failWrite != nil {
		if err := f.failWrite(path); err != nil {
			return err
		}
	}
	return f.Recorder.WriteFile(path, data, perm)
}

func (f *faultySystem) ReadFile(path string) ([]byte, error) {
	if f.failRead != nil {
		if err := f.failRead(path); err != nil {
			return nil, err
		}
	}
	return f.Recorder.ReadFile(path)
}

func (f *faultySystem) Remove(path string) error {
	if f.failRemove != nil {
		if err := f.failRemove(path); err != nil {
			return err
		}
	}
	return f.Recorder.Remove(path)
}

func (f *faultySystem) ProcessAlive(pid int) (bool, error) {
	if f.failAlive != nil {
		if err := f.failAlive(pid); err != nil {
			return false, err
		}
	}
	return f.Recorder.ProcessAlive(pid)
}

func (f *faultySystem) SignalProcess(pid int, sig Signal) error {
	if f.failSignal != nil {
		if err := f.failSignal(pid); err != nil {
			return err
		}
	}
	if err := f.Recorder.SignalProcess(pid, sig); err != nil {
		return err
	}
	if f.ignoresSignals {
		f.Recorder.SetAlive(pid, true)
	}
	return nil
}

func (f *faultySystem) Sleep(ctx context.Context, d time.Duration) error {
	if f.failSleep != nil {
		if err := f.failSleep(); err != nil {
			return err
		}
	}
	return f.Recorder.Sleep(ctx, d)
}

// onlyPath returns a hook that fails for one exact path.
func onlyPath(want string) func(string) error {
	return func(got string) error {
		if got == want {
			return errMachine
		}
		return nil
	}
}

func faultySupervisor(f *faultySystem) *Supervisor {
	s := NewSupervisor(f, testPaths())
	s.SetClock(func() time.Time { return leaseNow })
	return s
}

// --- writing the configuration ----------------------------------------------

// TestStartExplainsAFailureToSaveTheConfiguration covers both writeIfChanged
// failure branches.
//
// They are separate branches with separate sentences on purpose: "could not
// save the hotspot settings" and "could not save the DHCP and DNS settings"
// send the user to different places. A single shared message would be one
// fewer branch and one less useful answer.
func TestStartExplainsAFailureToSaveTheConfiguration(t *testing.T) {
	paths := testPaths()
	tests := []struct {
		name string
		path string
		want string
	}{
		{"the hostapd configuration", paths.HostapdConf, "hotspot settings"},
		{"the dnsmasq configuration", paths.DnsmasqConf, "DHCP and DNS settings"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFaultySystem()
			f.failWrite = onlyPath(tc.path)

			st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
			if err == nil {
				t.Fatal("Start reported success although the configuration could not be written")
			}
			if !errors.Is(err, errMachine) {
				t.Errorf("the underlying failure was replaced rather than wrapped: %v", err)
			}
			if !strings.Contains(st.Reason, tc.want) {
				t.Errorf("the reason does not say which settings could not be saved: %q", st.Reason)
			}
			if st.Running {
				t.Error("the status reports the hotspot as running after a failed write")
			}
			// The message is for a non-technical user: no paths, no errno.
			if strings.Contains(st.Reason, "/run/") {
				t.Errorf("the reason shows a file path: %q", st.Reason)
			}
		})
	}
}

// --- starting the processes -------------------------------------------------

// TestStartExplainsAProgramThatCannotBeRun covers the Run failure branch in
// startProcess, which is the "not installed" case and is distinct from a
// program that ran and exited non-zero.
func TestStartExplainsAProgramThatCannotBeRun(t *testing.T) {
	f := newFaultySystem()
	f.failRun = func(name string, _ []string) error {
		if strings.HasSuffix(name, "hostapd") {
			return errMachine
		}
		return nil
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("Start reported success although hostapd could not be run at all")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the underlying failure was replaced rather than wrapped: %v", err)
	}
	if !strings.Contains(st.Reason, "could not be started on this machine") {
		t.Errorf("the reason does not explain that the program could not be run: %q", st.Reason)
	}
	if !strings.Contains(st.Reason, unitAP) {
		t.Errorf("the reason does not name which part failed: %q", st.Reason)
	}
}

// TestStartReportsAPidFileItCannotRead covers the livePID error path.
//
// A missing pid file is zero and not an error, which is the normal case. A pid
// file that exists and cannot be read is a fault, and the difference matters:
// treating it as zero would have the supervisor start a second hostapd beside
// the one that is already holding the radio.
func TestStartReportsAPidFileItCannotRead(t *testing.T) {
	f := newFaultySystem()
	f.failRead = onlyPath(testPaths().HostapdPID)

	_, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("Start ignored a pid file it could not read")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the read failure was not propagated: %v", err)
	}
}

// TestStartReportsAStalePidFileItCannotRemove covers the Remove error path in
// startProcess. Leaving a stale pid file in place makes the next start read a
// pid that is dead, or worse has been reused by an unrelated process.
func TestStartReportsAStalePidFileItCannotRemove(t *testing.T) {
	f := newFaultySystem()
	f.failRemove = onlyPath(testPaths().HostapdPID)

	_, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("Start ignored a pid file it could not remove")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the remove failure was not propagated: %v", err)
	}
}

// TestStartGivesUpWhenNoPidFileEverAppears covers awaitPID running out of
// tries.
//
// Both daemons fork before the parent exits, so a zero exit code means the
// parent returned and not that the child came up. This is the branch that
// turns "the parent exited 0 and nothing is running" into a failure instead of
// a hotspot reported as up.
func TestStartGivesUpWhenNoPidFileEverAppears(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "hostapd") {
			// Exits 0 and writes no pid file, which is what a daemon that
			// forked and then died looks like from here.
			return Result{}, nil
		}
		return base(r, name, args)
	}

	s := faultySupervisor(f)
	s.StartTries = 3
	s.StartSettle = time.Millisecond

	st, err := s.Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("Start reported success although no pid file ever appeared")
	}
	if !strings.Contains(err.Error(), "did not report a running process") {
		t.Errorf("the error does not say what was waited for: %v", err)
	}
	if st.Reason == "" {
		t.Error("no reason was recorded for the panel to show")
	}
	// It really did wait rather than checking once.
	if got := f.Recorder.Slept; got < 3*time.Millisecond {
		t.Errorf("the supervisor slept for %s over 3 tries; it is not waiting between checks", got)
	}
}

// TestStartStopsWhenTheContextIsCancelledWhileWaiting covers the Sleep error
// path in awaitPID. A cancelled start must abandon the wait rather than run
// out its full StartTries.
func TestStartStopsWhenTheContextIsCancelledWhileWaiting(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "hostapd") {
			return Result{}, nil // no pid file
		}
		return base(r, name, args)
	}
	f.failSleep = func() error { return context.Canceled }

	s := faultySupervisor(f)
	s.StartTries = 50

	_, err := s.Start(context.Background(), testPlan(t))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Start returned %v, want context.Canceled", err)
	}
}

// --- the radio state machine ------------------------------------------------

// TestRfkillMissingIsNotFatal covers the branch where rfkill cannot be run.
//
// The decision recorded in ensureRadioUnblocked is that this is NOT a failure:
// on a machine where the radio is not blocked the hotspot starts anyway, and
// if it is blocked, hostapd's own failure is explained by explainFailure. So
// the test asserts the hotspot still comes up.
func TestRfkillMissingIsNotFatal(t *testing.T) {
	f := newFaultySystem()
	f.failRun = func(name string, _ []string) error {
		if strings.HasSuffix(name, "rfkill") {
			return errMachine
		}
		return nil
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("a missing rfkill stopped the hotspot from starting: %v", err)
	}
	if !st.Running {
		t.Errorf("the hotspot did not start although only rfkill was missing: %+v", st)
	}
	if !strings.Contains(st.Radio.Detail, "could not check") {
		t.Errorf("the status does not say the radio state is unknown: %q", st.Radio.Detail)
	}
}

// TestRfkillExitingNonZeroIsAlsoNotFatal covers the exit-code branch of
// rfkillList, which is a different path from the Run error above.
func TestRfkillExitingNonZeroIsAlsoNotFatal(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "list" {
			return Result{ExitCode: 2, Stderr: "rfkill: cannot open /dev/rfkill"}, nil
		}
		return base(r, name, args)
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("rfkill exiting non-zero stopped the hotspot from starting: %v", err)
	}
	if !strings.Contains(st.Radio.Detail, "could not check") {
		t.Errorf("the status does not say the radio state is unknown: %q", st.Radio.Detail)
	}
}

// TestAMachineWithNoWirelessAdapterIsSaidSo covers the empty-wireless-list
// branch, which is the Mode B case from the design: an adapter was expected
// and there is not one.
func TestAMachineWithNoWirelessAdapterIsSaidSo(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "list" {
			// A Bluetooth radio and nothing else.
			return Result{Stdout: "0: hci0: Bluetooth\n\tSoft blocked: no\n\tHard blocked: no\n"}, nil
		}
		return base(r, name, args)
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st.Radio.Present {
		t.Error("a machine with only a Bluetooth radio reported a wireless adapter present")
	}
	if !strings.Contains(st.Radio.Detail, "no wireless adapter") {
		t.Errorf("the status does not say there is no adapter: %q", st.Radio.Detail)
	}
	// docs/2026-08-29-design.md section 5.2: plain words, no jargon.
	for _, jargon := range []string{"rfkill", "phy", "wlan"} {
		if strings.Contains(st.Radio.Detail, jargon) {
			t.Errorf("the message contains jargon %q: %q", jargon, st.Radio.Detail)
		}
	}
}

// TestUnblockingThatCannotBeRunIsReported covers the branch where the radio is
// soft blocked and the unblock command itself fails.
func TestUnblockingThatCannotBeRunIsReported(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "list" {
			return Result{Stdout: SoftBlockedRfkillList}, nil
		}
		return base(r, name, args)
	}
	f.failRun = func(name string, args []string) error {
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "unblock" {
			return errMachine
		}
		return nil
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("Start reported success although the radio is off and could not be switched on")
	}
	if !strings.Contains(st.Radio.Detail, "could not switch it back on") {
		t.Errorf("the status does not say what failed: %q", st.Radio.Detail)
	}
	if st.Reason != st.Radio.Detail {
		t.Errorf("the top-level reason (%q) does not carry the radio detail (%q)", st.Reason, st.Radio.Detail)
	}
}

// TestUnblockingThatCannotBeConfirmedIsReported covers the branch where the
// unblock succeeded and the read-back failed.
//
// The read-back exists because an unblock that returns success and changes
// nothing is the failure worth catching. When the read-back itself cannot run,
// the honest answer is that the state is unknown, and the hotspot is allowed
// to continue.
func TestUnblockingThatCannotBeConfirmedIsReported(t *testing.T) {
	f := newFaultySystem()
	listCalls := 0
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "list" {
			listCalls++
			return Result{Stdout: SoftBlockedRfkillList}, nil
		}
		return base(r, name, args)
	}
	f.failRun = func(name string, args []string) error {
		// The unblock works. The SECOND list, the read-back, does not.
		if strings.HasSuffix(name, "rfkill") && len(args) > 0 && args[0] == "list" && listCalls >= 1 {
			return errMachine
		}
		return nil
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("an unconfirmable unblock stopped the start: %v", err)
	}
	if !strings.Contains(st.Radio.Detail, "could not confirm") {
		t.Errorf("the status does not say the unblock could not be confirmed: %q", st.Radio.Detail)
	}
}

// TestARadioStillBlockedAfterUnblockingIsAFailure covers the branch the
// read-back exists for: rfkill reported success and the radio is still off.
func TestARadioStillBlockedAfterUnblockingIsAFailure(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") {
			if len(args) > 0 && args[0] == "unblock" {
				return Result{}, nil // claims success
			}
			return Result{Stdout: SoftBlockedRfkillList}, nil // and changes nothing
		}
		return base(r, name, args)
	}

	st, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("an unblock that changed nothing was reported as success; " +
			"this is exactly the failure the read-back exists to catch")
	}
	if !strings.Contains(err.Error(), "still blocked") {
		t.Errorf("the error does not say the radio is still blocked: %v", err)
	}
	if st.Reason == "" {
		t.Error("no reason was recorded for the panel to show")
	}
}

// --- Status -----------------------------------------------------------------

// TestStatusExplainsEachWayTheHotspotIsNotWorking covers the four Reason
// branches of Status that nothing reached.
//
// Status changes nothing, so its whole output IS the Reason string. These four
// are the difference between a panel that says what to do and one that says
// "not working".
func TestStatusExplainsEachWayTheHotspotIsNotWorking(t *testing.T) {
	paths := testPaths()

	t.Run("the radio is hard blocked", func(t *testing.T) {
		rec := NewRecorder()
		base := DefaultResponder
		rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
			if strings.HasSuffix(name, "rfkill") {
				return Result{Stdout: HardBlockedRfkillList}, nil
			}
			return base(r, name, args)
		}
		st, err := newTestSupervisor(rec).Status(context.Background(), "wlan0")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.Running {
			t.Error("Status reports a hard-blocked radio as running")
		}
		if !strings.Contains(st.Reason, "switch on the machine itself") {
			t.Errorf("the reason does not tell the user what to go and do: %q", st.Reason)
		}
	})

	t.Run("the radio is soft blocked", func(t *testing.T) {
		rec := NewRecorder()
		base := DefaultResponder
		rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
			if strings.HasSuffix(name, "rfkill") {
				return Result{Stdout: SoftBlockedRfkillList}, nil
			}
			return base(r, name, args)
		}
		st, err := newTestSupervisor(rec).Status(context.Background(), "wlan0")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if !strings.Contains(st.Reason, "switched off in software") {
			t.Errorf("the reason does not describe a software block: %q", st.Reason)
		}
	})

	t.Run("hostapd runs but is not beaconing", func(t *testing.T) {
		rec := NewRecorder()
		base := DefaultResponder
		rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
			if strings.HasSuffix(name, "hostapd_cli") {
				// Running, and the access point is not enabled.
				return Result{Stdout: "state=DISABLED\n"}, nil
			}
			return base(r, name, args)
		}
		// Both daemons are already up, with live pid files.
		rec.SetFile(paths.HostapdPID, "4001\n")
		rec.SetAlive(4001, true)
		rec.SetFile(paths.DnsmasqPID, "4002\n")
		rec.SetAlive(4002, true)

		st, err := newTestSupervisor(rec).Status(context.Background(), "wlan0")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.Running {
			t.Error("a hostapd that is running and beaconing nothing was reported as running")
		}
		if !strings.Contains(st.Reason, "not being broadcast") {
			t.Errorf("the reason does not say the network is not being broadcast: %q", st.Reason)
		}
	})

	t.Run("the DHCP server is not running", func(t *testing.T) {
		rec := NewRecorder()
		// hostapd is up and beaconing; dnsmasq is not there at all.
		rec.SetFile(paths.HostapdPID, "4003\n")
		rec.SetAlive(4003, true)

		st, err := newTestSupervisor(rec).Status(context.Background(), "wlan0")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.Running {
			t.Error("a hotspot with no DHCP server was reported as running")
		}
		if !strings.Contains(st.Reason, "cannot get an address") {
			t.Errorf("the reason does not describe the symptom the user sees: %q", st.Reason)
		}
	})
}

// TestApBeaconingIsFalseWhenTheControlInterfaceFails covers the two failure
// returns of apBeaconing, and the short-circuit when no hostapd_cli is
// configured.
func TestApBeaconingIsFalseWhenTheControlInterfaceFails(t *testing.T) {
	t.Run("hostapd_cli cannot be run", func(t *testing.T) {
		f := newFaultySystem()
		f.failRun = func(name string, _ []string) error {
			if strings.HasSuffix(name, "hostapd_cli") {
				return errMachine
			}
			return nil
		}
		if got := faultySupervisor(f).apBeaconing(context.Background(), "wlan0"); got {
			t.Error("apBeaconing reported true although hostapd_cli could not be run")
		}
	})

	t.Run("hostapd_cli exits non-zero", func(t *testing.T) {
		rec := NewRecorder()
		base := DefaultResponder
		rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
			if strings.HasSuffix(name, "hostapd_cli") {
				return Result{ExitCode: 1, Stderr: "Failed to connect to hostapd"}, nil
			}
			return base(r, name, args)
		}
		if got := newTestSupervisor(rec).apBeaconing(context.Background(), "wlan0"); got {
			t.Error("apBeaconing reported true although hostapd_cli exited non-zero")
		}
	})

	t.Run("no hostapd_cli is configured", func(t *testing.T) {
		// With no control binary there is nothing to ask, and the supervisor
		// assumes the access point is up rather than reporting a hotspot that
		// works as broken.
		paths := testPaths()
		paths.HostapdCLIBinary = ""
		s := NewSupervisor(NewRecorder(), paths)
		if got := s.apBeaconing(context.Background(), "wlan0"); !got {
			t.Error("apBeaconing reported false when there is no way to ask")
		}
	})
}

// --- Stop -------------------------------------------------------------------

// TestStopCollectsEveryFailureRatherThanReturningTheFirst covers the two error
// accumulation branches in Stop.
//
// Stop uses errors.Join rather than returning early, and that is the behaviour
// being pinned: a hostapd that will not stop must not prevent the attempt to
// stop dnsmasq. Returning early here would leave the second daemon holding
// port 53, and the next start would fail for a reason that has nothing to do
// with the real problem.
func TestStopCollectsEveryFailureRatherThanReturningTheFirst(t *testing.T) {
	paths := testPaths()

	f := newFaultySystem()
	// Both daemons are up.
	f.Recorder.SetFile(paths.HostapdPID, "5001\n")
	f.Recorder.SetAlive(5001, true)
	f.Recorder.SetFile(paths.DnsmasqPID, "5002\n")
	f.Recorder.SetAlive(5002, true)
	// Neither can be signalled.
	f.failSignal = func(int) error { return errMachine }

	err := faultySupervisor(f).Stop(context.Background())
	if err == nil {
		t.Fatal("Stop reported success although neither process could be signalled")
	}
	msg := err.Error()
	if !strings.Contains(msg, unitAP) {
		t.Errorf("the error does not mention the hotspot: %v", err)
	}
	if !strings.Contains(msg, unitDHCP) {
		t.Errorf("the error does not mention the DHCP and DNS server; Stop returned early "+
			"instead of trying both: %v", err)
	}
}

// TestStopReportsAFailureToClearAStrayProcess covers the stopStrays error
// branch, which is the second of the two things Stop does per daemon.
func TestStopReportsAFailureToClearAStrayProcess(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "pgrep") {
			// A stray process was found.
			return Result{Stdout: "6001\n"}, nil
		}
		return base(r, name, args)
	}
	f.Recorder.SetAlive(6001, true)
	f.failSignal = func(pid int) error {
		if pid == 6001 {
			return errMachine
		}
		return nil
	}

	err := faultySupervisor(f).Stop(context.Background())
	if err == nil {
		t.Fatal("Stop reported success although a stray process could not be cleared")
	}
	if !strings.Contains(err.Error(), "leftover") {
		t.Errorf("the error does not say a leftover process was the problem: %v", err)
	}
}

// TestStrayProcessListIsFilteredBeforeAnythingIsSignalled covers the
// skip-this-line branch in stopStrays.
//
// This is a safety property, not a parsing nicety. stopStrays terminates
// whatever pgrep names, so a line that is not a plausible pid must be skipped
// rather than coerced. pid 1 in particular is init.
func TestStrayProcessListIsFilteredBeforeAnythingIsSignalled(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "pgrep") {
			// A real pid, init, a zero, a negative and a word.
			return Result{Stdout: "7001\n1\n0\n-5\nnotapid\n"}, nil
		}
		return base(r, name, args)
	}
	f.Recorder.SetAlive(7001, true)

	if err := faultySupervisor(f).Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	for _, sig := range f.Recorder.Signals {
		if sig.PID <= 1 {
			t.Errorf("the supervisor signalled pid %d; pid 1 is init and pid 0 is the whole process group", sig.PID)
		}
	}
	signalled := false
	for _, sig := range f.Recorder.Signals {
		if sig.PID == 7001 {
			signalled = true
		}
	}
	if !signalled {
		t.Error("the one real stray process was not stopped")
	}
}

// TestStopWithNoPgrepConfiguredSkipsTheStraySearch covers the early return in
// stopStrays. The pid file path has already run by then, so this is a
// degradation and not a failure.
func TestStopWithNoPgrepConfiguredSkipsTheStraySearch(t *testing.T) {
	paths := testPaths()
	paths.PgrepBinary = ""
	rec := NewRecorder()
	s := NewSupervisor(rec, paths)

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := rec.CountCalls("/usr/bin/pgrep"); got != 0 {
		t.Errorf("pgrep ran %d times although no pgrep binary is configured", got)
	}
}

// TestTerminateReportsAFailureToCheckLiveness covers the ProcessAlive error
// path inside terminate.
func TestTerminateReportsAFailureToCheckLiveness(t *testing.T) {
	paths := testPaths()
	f := newFaultySystem()
	f.Recorder.SetFile(paths.HostapdPID, "8001\n")
	f.Recorder.SetAlive(8001, true)

	calls := 0
	f.failAlive = func(int) error {
		calls++
		// The first check, from livePID, succeeds so that terminate is
		// reached. The second, inside terminate, fails.
		if calls > 1 {
			return errMachine
		}
		return nil
	}

	err := faultySupervisor(f).Stop(context.Background())
	if err == nil {
		t.Fatal("Stop reported success although liveness could not be checked")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the liveness failure was not propagated: %v", err)
	}
}

// TestTerminateEscalatesAndReportsAFailedKill covers the SIGKILL error branch.
//
// Stopping has to be reliable: a hostapd that ignored SIGTERM still holds the
// radio, and leaving it there makes the next start fail for a reason that
// looks like broken hardware. So a failed kill is reported rather than
// treated as done.
func TestTerminateEscalatesAndReportsAFailedKill(t *testing.T) {
	paths := testPaths()
	f := newFaultySystem()
	f.Recorder.SetFile(paths.HostapdPID, "8002\n")
	f.Recorder.SetAlive(8002, true)
	f.ignoresSignals = true // the process will not die, whatever it is sent

	s := faultySupervisor(f)
	s.StopGrace = 3 * time.Millisecond
	s.StopPoll = time.Millisecond

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Both signals were sent, in order: TERM first, then KILL.
	var sent []Signal
	for _, sig := range f.Recorder.Signals {
		if sig.PID == 8002 {
			sent = append(sent, sig.Signal)
		}
	}
	if len(sent) < 2 {
		t.Fatalf("only %d signals were sent to a process that would not stop: %v", len(sent), sent)
	}
	if sent[0] != SignalTerm {
		t.Errorf("the first signal was %v, want TERM", sent[0])
	}
	if sent[len(sent)-1] != SignalKill {
		t.Errorf("the last signal was %v, want KILL; a process that ignores TERM still holds the radio", sent[len(sent)-1])
	}
}

// TestStopReportsAFailureToSleepWhileWaiting covers the Sleep error path in
// terminate, which is how a cancelled context aborts a stop.
func TestStopReportsAFailureToSleepWhileWaiting(t *testing.T) {
	paths := testPaths()
	f := newFaultySystem()
	f.Recorder.SetFile(paths.HostapdPID, "8003\n")
	f.Recorder.SetAlive(8003, true)
	// The signal is delivered and the process stays alive, so terminate waits.
	f.ignoresSignals = true
	f.failSleep = func() error { return context.Canceled }

	err := faultySupervisor(f).Stop(context.Background())
	if err == nil {
		t.Fatal("Stop reported success although the wait was cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the cancellation was not propagated: %v", err)
	}
}

// TestStopReportsAPidFileItCannotRemove covers the Remove error in
// stopByPIDFile, which runs whether or not the process was live. A pid file
// left behind for a dead process is what makes the next start read a reused
// pid.
func TestStopReportsAPidFileItCannotRemove(t *testing.T) {
	f := newFaultySystem()
	f.failRemove = onlyPath(testPaths().HostapdPID)

	err := faultySupervisor(f).Stop(context.Background())
	if err == nil {
		t.Fatal("Stop reported success although the pid file could not be removed")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the remove failure was not propagated: %v", err)
	}
}

// TestStopReportsAPidFileItCannotRead covers the livePID error in
// stopByPIDFile.
func TestStopReportsAPidFileItCannotRead(t *testing.T) {
	f := newFaultySystem()
	f.failRead = onlyPath(testPaths().DnsmasqPID)

	err := faultySupervisor(f).Stop(context.Background())
	if err == nil {
		t.Fatal("Stop reported success although a pid file could not be read")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the read failure was not propagated: %v", err)
	}
}

// --- restart when the configuration changed ---------------------------------

// TestRestartReportsAFailureToStopTheOldProcess covers the terminate and
// Remove error branches on the confChanged path in startProcess.
func TestRestartReportsAFailureToStopTheOldProcess(t *testing.T) {
	paths := testPaths()

	t.Run("the old process cannot be stopped", func(t *testing.T) {
		f := newFaultySystem()
		// Running the WRONG configuration, so it has to be restarted.
		f.Recorder.SetFile(paths.HostapdConf, "old configuration")
		f.Recorder.SetFile(paths.HostapdPID, "9001\n")
		f.Recorder.SetAlive(9001, true)
		f.failSignal = func(int) error { return errMachine }

		_, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
		if err == nil {
			t.Fatal("Start reported success although the old process could not be stopped")
		}
		if !errors.Is(err, errMachine) {
			t.Errorf("the signal failure was not propagated: %v", err)
		}
	})

	t.Run("the old pid file cannot be removed", func(t *testing.T) {
		f := newFaultySystem()
		f.Recorder.SetFile(paths.HostapdConf, "old configuration")
		f.Recorder.SetFile(paths.HostapdPID, "9002\n")
		f.Recorder.SetAlive(9002, true)
		f.failRemove = onlyPath(paths.HostapdPID)

		_, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
		if err == nil {
			t.Fatal("Start reported success although the old pid file could not be removed")
		}
		if !errors.Is(err, errMachine) {
			t.Errorf("the remove failure was not propagated: %v", err)
		}
	})
}

// TestStartReportsAFailureToClearAStrayBeforeStarting covers the stopStrays
// error branch inside startProcess, which runs before a fresh start because a
// previous run may have left a process holding the radio with no pid file.
func TestStartReportsAFailureToClearAStrayBeforeStarting(t *testing.T) {
	f := newFaultySystem()
	base := DefaultResponder
	f.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "pgrep") {
			return Result{Stdout: "9003\n"}, nil
		}
		return base(r, name, args)
	}
	f.Recorder.SetAlive(9003, true)
	f.failSignal = func(pid int) error {
		if pid == 9003 {
			return errMachine
		}
		return nil
	}

	_, err := faultySupervisor(f).Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("Start reported success although a leftover process could not be cleared")
	}
	if !errors.Is(err, errMachine) {
		t.Errorf("the signal failure was not propagated: %v", err)
	}
}

// --- the last of the reachable branches -------------------------------------

// TestLivePIDTreatsAnUnreadablePidFileAsNotRunning covers the Atoi guard.
//
// A pid file holding garbage, which is what a truncated write or a power cut
// mid-write leaves, must read as "nothing is running" rather than as a pid.
// The alternative is a supervisor that signals whatever number it managed to
// parse.
//
// A note on what this can and cannot guard. The `pid <= 0` half of livePID's
// check is REDUNDANT with both System implementations: execSystem.ProcessAlive
// returns false for any pid <= 0, and the Recorder's map lookup returns false
// for a pid it was never told about. So deleting that half changes no
// observable behaviour and no test can fail on it. Verified by mutation on
// 2026-08-30, where exactly that mutant survived. This test guards the
// CONTRACT, which is what callers depend on, and the redundancy is recorded
// here so nobody reads the clause as protected by it.
func TestLivePIDTreatsAnUnreadablePidFileAsNotRunning(t *testing.T) {
	paths := testPaths()
	for _, content := range []string{"", "   ", "not-a-pid", "0\n", "-17\n", "12 34"} {
		rec := NewRecorder()
		rec.SetFile(paths.HostapdPID, content)
		s := newTestSupervisor(rec)

		pid, err := s.livePID(paths.HostapdPID)
		if err != nil {
			t.Errorf("a pid file containing %q was an error: %v", content, err)
		}
		if pid != 0 {
			t.Errorf("a pid file containing %q was read as pid %d", content, pid)
		}
	}
}

// TestLivePIDReportsAFailedLivenessCheck covers the ProcessAlive error inside
// livePID, which is a different call site from the one inside terminate.
func TestLivePIDReportsAFailedLivenessCheck(t *testing.T) {
	paths := testPaths()
	f := newFaultySystem()
	f.Recorder.SetFile(paths.HostapdPID, "4242\n")
	f.failAlive = func(int) error { return errMachine }

	if _, err := faultySupervisor(f).livePID(paths.HostapdPID); !errors.Is(err, errMachine) {
		t.Errorf("livePID returned %v, want the liveness failure", err)
	}
}

// TestAwaitPIDReportsAFailedRead covers the livePID error path inside
// awaitPID, which is the polling loop rather than the single check.
func TestAwaitPIDReportsAFailedRead(t *testing.T) {
	paths := testPaths()
	f := newFaultySystem()
	f.Recorder.SetFile(paths.HostapdPID, "4243\n")
	f.failAlive = func(int) error { return errMachine }

	s := faultySupervisor(f)
	s.StartTries = 2
	s.StartSettle = time.Millisecond

	if _, err := s.awaitPID(context.Background(), paths.HostapdPID); !errors.Is(err, errMachine) {
		t.Errorf("awaitPID returned %v, want the liveness failure", err)
	}
}

// TestWriteIfChangedReportsAnUnreadableFile covers the branch that separates
// "the file is not there", which is normal, from "the file cannot be read",
// which is not.
//
// Reading the failure as "not there" would rewrite the configuration and
// restart the daemon on every single Start, disconnecting every joined device
// each time, which is precisely what the idempotence in this function exists
// to prevent.
func TestWriteIfChangedReportsAnUnreadableFile(t *testing.T) {
	f := newFaultySystem()
	f.failRead = onlyPath(testPaths().HostapdConf)

	changed, err := faultySupervisor(f).writeIfChanged(testPaths().HostapdConf, "content", 0o600)
	if !errors.Is(err, errMachine) {
		t.Errorf("writeIfChanged returned %v, want the read failure", err)
	}
	if changed {
		t.Error("writeIfChanged reported a change after failing to read the old file")
	}
}

// TestTerminateReportsAFailedLivenessCheckAfterTheGracePeriod covers the
// ProcessAlive error on the path AFTER the wait loop, and the not-alive return
// beside it. They are a different pair from the in-loop checks.
func TestTerminateReportsAFailedLivenessCheckAfterTheGracePeriod(t *testing.T) {
	f := newFaultySystem()
	f.ignoresSignals = true
	f.Recorder.SetAlive(4244, true)

	s := faultySupervisor(f)
	// A zero grace period skips the loop entirely and goes straight to the
	// check after it.
	s.StopGrace = 0

	calls := 0
	f.failAlive = func(int) error {
		calls++
		return errMachine
	}
	if err := s.terminate(context.Background(), 4244); !errors.Is(err, errMachine) {
		t.Errorf("terminate returned %v, want the liveness failure", err)
	}
	if calls == 0 {
		t.Error("terminate never checked whether the process was still alive")
	}
}

// TestTerminateStopsWhenTheProcessDiedDuringTheGracePeriod covers the
// not-alive return after the loop: the process went away on its own and no
// SIGKILL is sent.
func TestTerminateStopsWhenTheProcessDiedDuringTheGracePeriod(t *testing.T) {
	f := newFaultySystem()
	f.Recorder.SetAlive(4245, true)

	s := faultySupervisor(f)
	s.StopGrace = 0 // straight to the post-loop check

	if err := s.terminate(context.Background(), 4245); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	// TERM was sent, the Recorder marked it dead, and no KILL followed.
	for _, sig := range f.Recorder.Signals {
		if sig.PID == 4245 && sig.Signal == SignalKill {
			t.Error("a process that stopped on TERM was also sent KILL")
		}
	}
}

// TestRadioStateWithNoDevicesSaysSo covers the !Present detail branch of
// radioStateFrom, which is what an empty rfkill list produces.
func TestRadioStateWithNoDevicesSaysSo(t *testing.T) {
	st := radioStateFrom(nil, false)
	if st.Present {
		t.Error("an empty device list reported a radio present")
	}
	if !strings.Contains(st.Detail, "no wireless adapter") {
		t.Errorf("the detail does not say there is no adapter: %q", st.Detail)
	}
}

// TestParseHostapdStatusSkipsLinesWithNoValue covers the continue in
// parseHostapdStatus.
//
// It matters because hostapd_cli prints a "Selected interface 'wlan0'" banner
// before the key=value block on some versions. A parser that treated that as a
// malformed record would report a working access point as not beaconing.
func TestParseHostapdStatusSkipsLinesWithNoValue(t *testing.T) {
	out := "Selected interface 'wlan0'\n" +
		"state=ENABLED\n" +
		"a line with no equals sign\n" +
		"\n" +
		"channel=10\n"
	m := parseHostapdStatus(out)

	if got := m["state"]; got != "ENABLED" {
		t.Errorf("state = %q, want ENABLED; the banner line broke the parse", got)
	}
	if got := m["channel"]; got != "10" {
		t.Errorf("channel = %q, want 10", got)
	}
	if len(m) != 2 {
		t.Errorf("the parse produced %d keys, want 2: %v", len(m), m)
	}
}

// TestLeaseFileIsRequired covers the empty-path branch of validAbsPath,
// reached through DNSConfig.Validate.
func TestLeaseFileIsRequired(t *testing.T) {
	c := testDNS()
	c.LeaseFile = ""
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted a configuration with no lease file")
	}
	if !strings.Contains(err.Error(), "no lease file was given") {
		t.Errorf("the message does not name the missing field: %v", err)
	}
}

// TestEventRenderingCoversEveryKind covers Event.String, the rendering the
// ordering assertions use in their failure messages.
//
// A test double whose failure message is wrong costs debugging time on an
// already failing test, which is when it is least affordable.
func TestEventRenderingCoversEveryKind(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{Event{Kind: EventRun, Name: "/usr/sbin/hostapd", Args: []string{"-B", "-P", "/run/x.pid"}},
			"run /usr/sbin/hostapd -B -P /run/x.pid"},
		{Event{Kind: EventRun, Name: "/usr/bin/pgrep"}, "run /usr/bin/pgrep"},
		{Event{Kind: EventWrite, Path: "/run/caspian/hostapd.conf", Perm: 0o600},
			"write /run/caspian/hostapd.conf (0600)"},
		{Event{Kind: EventRemove, Path: "/run/caspian/hostapd.pid"},
			"remove /run/caspian/hostapd.pid"},
		{Event{Kind: EventSignal, PID: 1234, Signal: SignalKill}, "signal KILL to 1234"},
		{Event{Kind: EventSleep, Duration: 100 * time.Millisecond}, "sleep 100ms"},
		{Event{Kind: EventKind("something new")}, "something new"},
	}
	for _, tc := range tests {
		if got := tc.event.String(); got != tc.want {
			t.Errorf("Event.String() = %q, want %q", got, tc.want)
		}
	}
}

// TestEventTrailHelpers covers FirstEvent's not-found return and TrailString.
func TestEventTrailHelpers(t *testing.T) {
	rec := NewRecorder()
	if err := rec.WriteFile("/run/caspian/hostapd.conf", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := rec.Run(context.Background(), "/usr/bin/pgrep", "-f", "/run/x.conf"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Not found is -1, which is what makes an ordering comparison safe to
	// write as two indexes.
	if got := rec.FirstEvent(func(e Event) bool { return e.Kind == EventSignal }); got != -1 {
		t.Errorf("FirstEvent for an event that did not happen = %d, want -1", got)
	}
	if got := rec.FirstEvent(func(e Event) bool { return e.Kind == EventWrite }); got != 0 {
		t.Errorf("FirstEvent for the write = %d, want 0", got)
	}

	trail := rec.TrailString()
	if !strings.Contains(trail, "write /run/caspian/hostapd.conf") {
		t.Errorf("the trail does not show the write:\n%s", trail)
	}
	if !strings.Contains(trail, "run /usr/bin/pgrep") {
		t.Errorf("the trail does not show the run:\n%s", trail)
	}
	if rec.TrailString() == "" {
		t.Error("TrailString produced nothing for a recorder with two events")
	}
}

// TestTerminateReportsAFailedKill covers the SIGKILL error return, which is
// the last thing terminate can do and the one whose failure means the radio is
// still held by a process nothing can stop.
func TestTerminateReportsAFailedKill(t *testing.T) {
	f := newFaultySystem()
	f.ignoresSignals = true
	f.Recorder.SetAlive(4246, true)

	signals := 0
	f.failSignal = func(int) error {
		signals++
		if signals > 1 { // the TERM lands, the KILL does not
			return errMachine
		}
		return nil
	}

	s := faultySupervisor(f)
	s.StopGrace = 2 * time.Millisecond
	s.StopPoll = time.Millisecond

	if err := s.terminate(context.Background(), 4246); !errors.Is(err, errMachine) {
		t.Errorf("terminate returned %v, want the failure from the kill", err)
	}
	if signals < 2 {
		t.Errorf("only %d signals were attempted; the escalation to KILL never happened", signals)
	}
}

// TestStopStraysTreatsAMissingPgrepAsNothingToDo covers the Run error branch
// in stopStrays.
//
// pgrep not being installed must not fail a stop: the pid file path has
// already run by then, so the stray search is a second chance rather than the
// mechanism.
func TestStopStraysTreatsAMissingPgrepAsNothingToDo(t *testing.T) {
	f := newFaultySystem()
	f.failRun = func(name string, _ []string) error {
		if strings.HasSuffix(name, "pgrep") {
			return errMachine
		}
		return nil
	}

	if err := faultySupervisor(f).Stop(context.Background()); err != nil {
		t.Errorf("a missing pgrep failed the whole stop: %v", err)
	}
}
