// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testPaths() Paths {
	p := DefaultPaths()
	return p
}

func testPlan(t *testing.T) Plan {
	t.Helper()
	plan, err := NewPlan(testAP(), testDNS(), RadioConstraint{
		SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10,
	})
	if err != nil {
		t.Fatalf("NewPlan: %v", err)
	}
	return plan
}

func newTestSupervisor(rec *Recorder) *Supervisor {
	s := NewSupervisor(rec, testPaths())
	s.SetClock(func() time.Time { return leaseNow })
	return s
}

func TestStartBringsBothProcessesUp(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)

	st, err := s.Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !st.Running {
		t.Fatalf("Start did not report a running hotspot: %+v", st)
	}
	if !st.AccessPoint.Running || !st.AccessPoint.Beaconing {
		t.Errorf("access point state = %+v", st.AccessPoint)
	}
	if !st.DHCP.Running {
		t.Errorf("dhcp state = %+v", st.DHCP)
	}
	if st.Reason != "" {
		t.Errorf("a running hotspot has a failure reason: %q", st.Reason)
	}

	// The configuration actually reached the machine, and the hostapd file,
	// which holds the WPA2 passphrase, went out with restrictive permissions.
	if got := string(rec.Files[testPaths().HostapdConf]); !strings.Contains(got, "ssid=Caspian") {
		t.Error("the hostapd configuration was not written")
	}
	if got := string(rec.Files[testPaths().DnsmasqConf]); !strings.Contains(got, "dhcp-range=") {
		t.Error("the dnsmasq configuration was not written")
	}

	lines := rec.CommandLines()
	wantSubstrings := []string{
		"/usr/sbin/hostapd -B -P /run/caspian/hostapd.pid /run/caspian/hostapd.conf",
		"/usr/sbin/dnsmasq --conf-file=/run/caspian/dnsmasq.conf",
	}
	for _, want := range wantSubstrings {
		if !anyContains(lines, want) {
			t.Errorf("expected a command containing %q, got:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

// TestStartIsIdempotent is the case that decides whether pressing the panel's
// switch twice, or a health check deciding to repair, disconnects every device
// on the hotspot.
func TestStartIsIdempotent(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	plan := testPlan(t)

	if _, err := s.Start(context.Background(), plan); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	firstHostapd := rec.CountCalls(testPaths().HostapdBinary)
	firstDnsmasq := rec.CountCalls(testPaths().DnsmasqBinary)
	if firstHostapd != 1 || firstDnsmasq != 1 {
		t.Fatalf("first Start ran hostapd %d times and dnsmasq %d times, want 1 and 1",
			firstHostapd, firstDnsmasq)
	}
	signalsBefore := len(rec.Signals)

	st, err := s.Start(context.Background(), plan)
	if err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if !st.Running {
		t.Fatalf("the second Start reported the hotspot down: %+v", st)
	}
	if got := rec.CountCalls(testPaths().HostapdBinary); got != firstHostapd {
		t.Errorf("the second Start started hostapd again (%d calls, want %d); "+
			"that would disconnect every joined device", got, firstHostapd)
	}
	if got := rec.CountCalls(testPaths().DnsmasqBinary); got != firstDnsmasq {
		t.Errorf("the second Start started dnsmasq again (%d calls, want %d)", got, firstDnsmasq)
	}
	if len(rec.Signals) != signalsBefore {
		t.Errorf("the second Start signalled a running process: %+v", rec.Signals[signalsBefore:])
	}
	if st.AccessPoint.Detail != "already running" {
		t.Errorf("access point detail = %q, want \"already running\"", st.AccessPoint.Detail)
	}
}

// TestStartRestartsWhenTheConfigurationChanged is the other half of
// idempotence: doing nothing is right only while the running process is
// running the configuration we want.
func TestStartRestartsWhenTheConfigurationChanged(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)

	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	oldPID := 0
	if b, err := rec.ReadFile(testPaths().HostapdPID); err == nil {
		oldPID, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	if oldPID == 0 {
		t.Fatal("no pid file after the first Start")
	}

	ap := testAP()
	ap.SSID = "Caspian-Renamed"
	changed, err := NewPlan(ap, testDNS(), RadioConstraint{
		SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10,
	})
	if err != nil {
		t.Fatalf("NewPlan: %v", err)
	}

	st, err := s.Start(context.Background(), changed)
	if err != nil {
		t.Fatalf("Start after a change: %v", err)
	}
	if !st.Running {
		t.Fatalf("the hotspot is down after a configuration change: %+v", st)
	}
	if got := rec.CountCalls(testPaths().HostapdBinary); got != 2 {
		t.Errorf("hostapd was started %d times, want 2", got)
	}
	if !signalled(rec, oldPID, SignalTerm) {
		t.Errorf("the old hostapd (pid %d) was not asked to stop: %+v", oldPID, rec.Signals)
	}
	// dnsmasq's configuration did not change, so it must not have been
	// restarted: a DHCP restart is not free either.
	if got := rec.CountCalls(testPaths().DnsmasqBinary); got != 1 {
		t.Errorf("dnsmasq was restarted %d times for a change that did not affect it", got)
	}
}

// TestStartClearsAStaleProcessLeftByAPreviousRun covers the machine state that
// makes the next start fail with a driver message nobody can interpret: a
// hostapd from a previous run still holding the radio, with no usable pid file.
func TestStartClearsAStaleProcessLeftByAPreviousRun(t *testing.T) {
	const strayPID = 4242

	rec := NewRecorder()
	rec.SetAlive(strayPID, true)
	// No pid file: this is the run that was killed, not the one that stopped.
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "pgrep") {
			// pgrep finds the stray by our own configuration path, once. After
			// it has been stopped it is no longer there.
			alive, _ := r.ProcessAlive(strayPID)
			if alive {
				return Result{Stdout: strconv.Itoa(strayPID) + "\n"}, nil
			}
			return Result{ExitCode: 1}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	st, err := s.Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !st.Running {
		t.Fatalf("Start did not bring the hotspot up: %+v", st)
	}
	if !signalled(rec, strayPID, SignalTerm) {
		t.Errorf("the leftover process %d was not stopped: %+v", strayPID, rec.Signals)
	}
	if alive, _ := rec.ProcessAlive(strayPID); alive {
		t.Errorf("the leftover process %d is still running", strayPID)
	}
	// The search was made with our own path, not with anything a user typed.
	if !anyContains(rec.CommandLines(), "pgrep -f /run/caspian/hostapd.conf") {
		t.Errorf("the stale search did not use our own configuration path:\n%s",
			strings.Join(rec.CommandLines(), "\n"))
	}
}

// TestStartWithAStalePIDFileForADeadProcess covers the other leftover: a pid
// file naming a process that no longer exists. Reusing that pid would signal
// an unrelated process.
func TestStartWithAStalePIDFileForADeadProcess(t *testing.T) {
	rec := NewRecorder()
	rec.SetFile(testPaths().HostapdPID, "31337\n")
	rec.SetAlive(31337, false)

	s := newTestSupervisor(rec)
	st, err := s.Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !st.Running {
		t.Fatalf("Start did not bring the hotspot up: %+v", st)
	}
	if signalled(rec, 31337, SignalTerm) || signalled(rec, 31337, SignalKill) {
		t.Errorf("a dead pid from a stale pid file was signalled: %+v", rec.Signals)
	}
	if !contains(rec.Removed, testPaths().HostapdPID) {
		t.Errorf("the stale pid file was not removed: %v", rec.Removed)
	}
	if st.AccessPoint.PID == 31337 {
		t.Error("the supervisor adopted a dead pid")
	}
}

func TestStopTerminatesBothAndClearsPIDFiles(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)

	st, err := s.Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	apPID, dhcpPID := st.AccessPoint.PID, st.DHCP.PID

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	for _, pid := range []int{apPID, dhcpPID} {
		if !signalled(rec, pid, SignalTerm) {
			t.Errorf("process %d was not asked to stop: %+v", pid, rec.Signals)
		}
		if alive, _ := rec.ProcessAlive(pid); alive {
			t.Errorf("process %d is still running after Stop", pid)
		}
	}
	for _, p := range []string{testPaths().HostapdPID, testPaths().DnsmasqPID} {
		if !contains(rec.Removed, p) {
			t.Errorf("%s was not removed", p)
		}
	}
}

// TestStopEscalatesToKill: stopping has to be reliable. A hostapd that ignores
// SIGTERM still holds the radio, and leaving it there makes the next start
// look like broken hardware.
func TestStopEscalatesToKill(t *testing.T) {
	const stubborn = 777

	rec := NewRecorder()
	rec.SetFile(testPaths().HostapdPID, strconv.Itoa(stubborn)+"\n")
	rec.SetAlive(stubborn, true)

	// A process that ignores SIGTERM and only dies on SIGKILL.
	ignoreTerm := &stubbornSystem{Recorder: rec, pid: stubborn}
	s := NewSupervisor(ignoreTerm, testPaths())
	s.StopGrace = 300 * time.Millisecond
	s.StopPoll = 50 * time.Millisecond

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !signalled(rec, stubborn, SignalTerm) {
		t.Error("the process was never asked to stop politely")
	}
	if !signalled(rec, stubborn, SignalKill) {
		t.Errorf("a process that ignored SIGTERM was left running: %+v", rec.Signals)
	}
}

// stubbornSystem is a Recorder whose pid survives SIGTERM.
type stubbornSystem struct {
	*Recorder
	pid int
}

func (s *stubbornSystem) SignalProcess(pid int, sig Signal) error {
	s.Recorder.mu.Lock()
	s.Recorder.noteSignal(pid, sig)
	if pid != s.pid || sig == SignalKill {
		s.Recorder.Alive[pid] = false
	}
	s.Recorder.mu.Unlock()
	return nil
}

func TestStopOnAMachineWhereNothingIsRunning(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	// Stopping something that is already stopped is success, not an error.
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop on an idle machine: %v", err)
	}
	if len(rec.Signals) != 0 {
		t.Errorf("Stop signalled something on an idle machine: %+v", rec.Signals)
	}
}

// TestSoftBlockedRadioIsUnblockedAndSaidSo covers the state this platform is
// frequently in.
func TestSoftBlockedRadioIsUnblockedAndSaidSo(t *testing.T) {
	rec := NewRecorder()
	unblocked := false
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") {
			if len(args) > 0 && args[0] == "unblock" {
				unblocked = true
				return Result{}, nil
			}
			if unblocked {
				return Result{Stdout: unblockedRfkillList}, nil
			}
			return Result{Stdout: SoftBlockedRfkillList}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	st, err := s.Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !unblocked {
		t.Fatal("the soft-blocked radio was never unblocked")
	}
	if !st.Running {
		t.Fatalf("the hotspot did not start after the radio was unblocked: %+v", st)
	}
	if !st.Radio.Unblocked {
		t.Error("the status does not record that the radio had to be switched on")
	}
	if st.Radio.SoftBlocked {
		t.Error("the status still reports the radio as blocked")
	}
	if !strings.Contains(st.Radio.Detail, "switched it back on") {
		t.Errorf("the status does not say what happened: %q", st.Radio.Detail)
	}
	// The state was read back after unblocking, rather than assumed: an
	// unblock that returns success and changes nothing is the failure worth
	// catching.
	listCalls := 0
	for _, c := range rec.Calls {
		if strings.HasSuffix(c.Name, "rfkill") && len(c.Args) > 0 && c.Args[0] == "list" {
			listCalls++
		}
	}
	if listCalls < 2 {
		t.Errorf("rfkill list ran %d times; the state was not read back after unblocking", listCalls)
	}
}

// TestHardBlockedRadioFailsWithSomethingTheUserCanDo: software cannot clear a
// hard block, so blindly unblocking and starting hostapd (which is what the
// reference implementation did) produces a driver error instead of an
// instruction.
func TestHardBlockedRadioFailsWithSomethingTheUserCanDo(t *testing.T) {
	rec := NewRecorder()
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "rfkill") {
			return Result{Stdout: HardBlockedRfkillList}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	st, err := s.Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("a hard-blocked radio started the hotspot")
	}
	if st.Running {
		t.Error("the status reports a running hotspot")
	}
	if !st.Radio.HardBlocked {
		t.Error("the status does not record the hard block")
	}
	if !strings.Contains(st.Reason, "switch") {
		t.Errorf("the reason does not tell the user what to do: %q", st.Reason)
	}
	for _, jargon := range []string{"rfkill", "errno", "EPERM", "-1"} {
		if strings.Contains(st.Reason, jargon) {
			t.Errorf("the reason contains jargon %q: %q", jargon, st.Reason)
		}
	}
	// hostapd was never started: there was no point.
	if rec.CountCalls(testPaths().HostapdBinary) != 0 {
		t.Error("hostapd was started against a hard-blocked radio")
	}
}

// TestHostapdRunningButNotBeaconing is the failure a process check cannot see.
func TestHostapdRunningButNotBeaconing(t *testing.T) {
	rec := NewRecorder()
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "hostapd_cli") {
			return Result{Stdout: "state=DISABLED\n"}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	st, err := s.Start(context.Background(), testPlan(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st.Running {
		t.Error("a hotspot that is broadcasting nothing was reported as running")
	}
	if st.AccessPoint.Beaconing {
		t.Error("Beaconing is true while hostapd reports state=DISABLED")
	}
	if !strings.Contains(st.Reason, "not being broadcast") {
		t.Errorf("the reason does not describe the failure: %q", st.Reason)
	}
}

func TestStartFailureIsExplainedInPlainWords(t *testing.T) {
	rec := NewRecorder()
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "/hostapd") {
			return Result{
				ExitCode: 1,
				Stderr:   "nl80211: Could not configure driver mode\nnl80211 driver initialization failed.\n",
			}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	st, err := s.Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("a failing hostapd was reported as a success")
	}
	if st.Running {
		t.Error("the status reports a running hotspot")
	}
	if !strings.Contains(st.Reason, "USB WiFi adapter") {
		t.Errorf("the reason does not tell the user what to do: %q", st.Reason)
	}
	if strings.Contains(st.Reason, "nl80211") {
		t.Errorf("the reason leaks driver jargon: %q", st.Reason)
	}
}

func TestStatusReportsDevicesFromTheLeaseFile(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// No lease file yet: nobody has joined. That is zero devices, not a fault.
	st, err := s.Status(context.Background(), "wlan0")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Running {
		t.Fatalf("Status reports the hotspot down: %+v", st)
	}
	if st.DeviceCount() != 0 {
		t.Errorf("device count = %d with no lease file, want 0", st.DeviceCount())
	}

	rec.SetFile(testPaths().LeaseFile,
		"1788051600 02:00:5e:02:00:01 192.168.66.51 iPhone 01:02:00:5e:02:00:01\n"+
			"1788044400 02:00:5e:02:00:04 192.168.66.54 pixel-8 *\n"+ // expired
			"0 02:00:5e:02:00:05 192.168.66.60 kitchen-printer *\n")

	st, err = s.Status(context.Background(), "wlan0")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.DeviceCount() != 2 {
		t.Errorf("device count = %d, want 2 (the third lease expired): %+v", st.DeviceCount(), st.Devices)
	}
	names := []string{st.Devices[0].DisplayName(), st.Devices[1].DisplayName()}
	if names[0] != "iPhone" || names[1] != "kitchen-printer" {
		t.Errorf("device names = %v", names)
	}
}

func TestStatusOnAStoppedHotspot(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)

	st, err := s.Status(context.Background(), "wlan0")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Running {
		t.Error("Status reports a running hotspot on an idle machine")
	}
	if st.Reason == "" {
		t.Error("Status gives no reason for a hotspot that is not running")
	}
}

func TestDnsmasqIsToldToIgnoreTheSystemConfigDirectory(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// A fragment in /etc/dnsmasq.d could add a public resolver or turn query
	// logging back on, undoing two of the guarantees the generated file makes.
	if !anyContains(rec.CommandLines(), "--conf-dir=") {
		t.Errorf("dnsmasq was not told to ignore /etc/dnsmasq.d:\n%s",
			strings.Join(rec.CommandLines(), "\n"))
	}
}

func TestExplainFailure(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		exit   int
		want   string
	}{
		{"missing binary", "", 127, "not installed"},
		{"no such file", "hostapd: no such file or directory", 127, "not installed"},
		{"channel refused", "Could not set channel for kernel driver", 1, "channel"},
		{"no ap support", "nl80211: Could not configure driver mode", 1, "USB WiFi adapter"},
		{"interface gone", "Could not read interface wlan1 flags: No such device", 1, "not there any more"},
		{"port in use", "dnsmasq: failed to create listening socket: Address already in use", 2, "already answering"},
		{"not root", "dnsmasq: setting capabilities failed: Operation not permitted", 5, "administrator rights"},
		{"unknown option", "dnsmasq: bad option at line 12", 1, "did not understand its settings"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := explainFailure(unitAP, tc.exit, tc.stderr, "")
			if !strings.Contains(got, tc.want) {
				t.Errorf("explainFailure(%q) = %q, want it to mention %q", tc.stderr, got, tc.want)
			}
			// Whatever it says, it must not be an errno or a bare code.
			if strings.HasPrefix(got, "exit status") {
				t.Errorf("explainFailure returned a raw status: %q", got)
			}
		})
	}
}

// TestExplainFailureDoesNotInventACause: when the message is not one this
// table knows, saying so is better than a plausible guess that sends the user
// to fix the wrong thing.
func TestExplainFailureDoesNotInventACause(t *testing.T) {
	got := explainFailure(unitAP, 3, "something nobody has seen before", "")
	if !strings.Contains(got, "does not recognise the reason") {
		t.Errorf("an unrecognised failure was given a confident explanation: %q", got)
	}
	if !strings.Contains(got, "something nobody has seen before") {
		t.Errorf("the original text was dropped: %q", got)
	}
}

func TestParseHostapdStatus(t *testing.T) {
	m := parseHostapdStatus("state=ENABLED\nphy=phy0\nfreq=2457\n\nchannel=10\n")
	if m["state"] != "ENABLED" || m["channel"] != "10" {
		t.Errorf("parseHostapdStatus = %v", m)
	}
	if len(parseHostapdStatus("")) != 0 {
		t.Error("empty output parsed to a non-empty map")
	}
}

func TestNewPlanRefusesAChannelTheRadioCannotUse(t *testing.T) {
	ap := testAP()
	ap.Channel = 6
	_, err := NewPlan(ap, testDNS(), RadioConstraint{
		SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10,
	})
	if err == nil {
		t.Fatal("a plan was made for a channel the radio cannot use")
	}
	if !strings.Contains(err.Error(), "channel 10") {
		t.Errorf("the error does not name the channel that must be used: %v", err)
	}
}

func TestNewPlanDefaultsTheDNSInterfaceToTheAPInterface(t *testing.T) {
	dns := testDNS()
	dns.Interface = ""
	plan, err := NewPlan(testAP(), dns, RadioConstraint{
		SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10,
	})
	if err != nil {
		t.Fatalf("NewPlan: %v", err)
	}
	if plan.DNS.Interface != "wlan0" {
		t.Errorf("dns interface = %q, want wlan0", plan.DNS.Interface)
	}
}

func TestNewPlanReportsAGeneratedPassphrase(t *testing.T) {
	ap := testAP()
	ap.Passphrase = ""
	plan, err := NewPlan(ap, testDNS(), RadioConstraint{
		SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10,
	})
	if err != nil {
		t.Fatalf("NewPlan: %v", err)
	}
	if !plan.PassphraseGenerated {
		t.Error("the plan does not report that the passphrase was generated")
	}
	// The panel has to be able to show it, so it must be on the plan.
	if plan.AP.Passphrase == "" {
		t.Fatal("the generated passphrase is not on the plan")
	}
	if !strings.Contains(plan.HostapdConf, "wpa_passphrase="+plan.AP.Passphrase) {
		t.Error("the rendered configuration does not carry the generated passphrase")
	}
}

// --- helpers ---------------------------------------------------------------

func anyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func signalled(rec *Recorder, pid int, sig Signal) bool {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, s := range rec.Signals {
		if s.PID == pid && s.Signal == sig {
			return true
		}
	}
	return false
}

// TestGeneratedFilesUseTheModesTheLayoutFixes checks the two generated files
// against docs/LAYOUT.md, which fixes both at 0600 root.
//
// This caught a real divergence: the dnsmasq configuration was being written
// 0644 while the layout said 0600. The layout is the authority ("Everything in
// the project agrees with this file"), and nothing needs to read that file
// after dnsmasq parses it as root.
func TestGeneratedFilesUseTheModesTheLayoutFixes(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for _, tc := range []struct {
		path string
		want os.FileMode
		why  string
	}{
		{testPaths().HostapdConf, 0o600, "it carries the WPA2 passphrase"},
		{testPaths().DnsmasqConf, 0o600, "docs/LAYOUT.md fixes it at 0600 root"},
	} {
		got, ok := rec.Perms[tc.path]
		if !ok {
			t.Errorf("%s was never written", tc.path)
			continue
		}
		if got != tc.want {
			t.Errorf("%s was written %04o, want %04o: %s", tc.path, got, tc.want, tc.why)
		}
	}
}

// TestMissingPIDFileSaysWhatToLookAt covers the branch that used to return
// "stopped with no explanation". A daemon that exits zero and records no pid
// is most often one that was not allowed to write where it was told to.
func TestMissingPIDFileSaysWhatToLookAt(t *testing.T) {
	rec := NewRecorder()
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "/dnsmasq") {
			// Started, said nothing, wrote no pid file.
			return Result{}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	s.StartTries = 2
	st, err := s.Start(context.Background(), testPlan(t))
	if err == nil {
		t.Fatal("a dnsmasq that never recorded itself was reported as a success")
	}
	if st.Running {
		t.Error("the status reports a running hotspot")
	}
	if !strings.Contains(st.Reason, testPaths().DnsmasqPID) {
		t.Errorf("the reason does not name the file that could not be written: %q", st.Reason)
	}
	if strings.Contains(st.Reason, "no explanation") {
		t.Errorf("the reason is still the unhelpful fallback: %q", st.Reason)
	}
}

// --- ordering, which the double could not express until it kept one trail ---

func ranProgram(name string) func(Event) bool {
	return func(e Event) bool { return e.Kind == EventRun && e.Name == name }
}

func signalledPID(pid int) func(Event) bool {
	return func(e Event) bool { return e.Kind == EventSignal && e.PID == pid }
}

func wrotePath(path string) func(Event) bool {
	return func(e Event) bool { return e.Kind == EventWrite && e.Path == path }
}

func removedPath(path string) func(Event) bool {
	return func(e Event) bool { return e.Kind == EventRemove && e.Path == path }
}

// TestConfigurationIsWrittenBeforeTheProcessThatReadsIt.
//
// Reversing these two is not a crash: the process starts on the PREVIOUS run's
// settings, so a user who changes the hotspot name sees the old one and tries
// again. Nothing in the suite could have caught it before the recorder kept an
// ordered trail.
func TestConfigurationIsWrittenBeforeTheProcessThatReadsIt(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for _, tc := range []struct {
		conf   string
		binary string
	}{
		{testPaths().HostapdConf, testPaths().HostapdBinary},
		{testPaths().DnsmasqConf, testPaths().DnsmasqBinary},
	} {
		wrote := rec.FirstEvent(wrotePath(tc.conf))
		ran := rec.FirstEvent(ranProgram(tc.binary))
		if wrote < 0 || ran < 0 {
			t.Fatalf("expected both a write of %s and a run of %s:\n%s", tc.conf, tc.binary, rec.TrailString())
		}
		if wrote > ran {
			t.Errorf("%s started before %s was written, so it would read the previous run's settings:\n%s",
				tc.binary, tc.conf, rec.TrailString())
		}
	}
}

// TestStrayIsStoppedBeforeTheReplacementStarts.
//
// The reverse order is not a crash either: hostapd starts while the leftover
// process still holds the radio, fails with a driver message, and the user is
// told their adapter cannot create a hotspot when in fact it can.
func TestStrayIsStoppedBeforeTheReplacementStarts(t *testing.T) {
	const strayPID = 4242

	rec := NewRecorder()
	rec.SetAlive(strayPID, true)
	base := DefaultResponder
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "pgrep") {
			if alive, _ := r.ProcessAlive(strayPID); alive {
				return Result{Stdout: strconv.Itoa(strayPID) + "\n"}, nil
			}
			return Result{ExitCode: 1}, nil
		}
		return base(r, name, args)
	}

	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}

	stopped := rec.FirstEvent(signalledPID(strayPID))
	started := rec.FirstEvent(ranProgram(testPaths().HostapdBinary))
	if stopped < 0 || started < 0 {
		t.Fatalf("expected the stray to be signalled and hostapd to run:\n%s", rec.TrailString())
	}
	if stopped > started {
		t.Errorf("hostapd was started before the leftover process %d was stopped, so it would "+
			"fail with the radio still held:\n%s", strayPID, rec.TrailString())
	}
}

// TestStalePIDFileIsRemovedBeforeTheReplacementStarts. Leaving it until after
// means a window in which the supervisor reads a pid belonging to whatever
// else has since been given that number.
func TestStalePIDFileIsRemovedBeforeTheReplacementStarts(t *testing.T) {
	rec := NewRecorder()
	rec.SetFile(testPaths().HostapdPID, "31337\n")
	rec.SetAlive(31337, false)

	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	removed := rec.FirstEvent(removedPath(testPaths().HostapdPID))
	started := rec.FirstEvent(ranProgram(testPaths().HostapdBinary))
	if removed < 0 || started < 0 {
		t.Fatalf("expected a removal and a start:\n%s", rec.TrailString())
	}
	if removed > started {
		t.Errorf("the stale pid file was removed after hostapd started:\n%s", rec.TrailString())
	}
}

// TestStopPollsRatherThanSleepingOutTheGrace. The total sleep alone cannot
// tell these apart, which is why the recorder now keeps each wait: a single
// sleep for the whole grace period means Stop takes the full three seconds
// even when the process exited immediately.
func TestStopPollsRatherThanSleepingOutTheGrace(t *testing.T) {
	const stubborn = 777

	rec := NewRecorder()
	rec.SetFile(testPaths().HostapdPID, strconv.Itoa(stubborn)+"\n")
	rec.SetAlive(stubborn, true)

	s := NewSupervisor(&stubbornSystem{Recorder: rec, pid: stubborn}, testPaths())
	s.StopGrace = 300 * time.Millisecond
	s.StopPoll = 50 * time.Millisecond

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(rec.Sleeps) < 2 {
		t.Fatalf("Stop made %d waits; it is sleeping out the grace period rather than polling: %v",
			len(rec.Sleeps), rec.Sleeps)
	}
	for i, d := range rec.Sleeps {
		if d > s.StopPoll {
			t.Errorf("wait %d was %s, longer than the %s poll interval", i, d, s.StopPoll)
		}
	}
	if rec.Slept > s.StopGrace {
		t.Errorf("Stop waited %s in total, longer than the %s grace period", rec.Slept, s.StopGrace)
	}
}

// TestDnsmasqPIDLivesInADirectoryOfItsOwn.
//
// docs/LAYOUT.md fixes /run/caspian at 0750 root:caspian, which gives the
// group no write bit, and /run/caspian/dnsmasq at 0700 caspian. dnsmasq drops
// to that account, so its pid file has to be in the second directory, not the
// first. Whether dnsmasq writes the pid before or after dropping privileges is
// a fact about dnsmasq nobody here has measured; putting the file in a
// directory dnsmasq owns makes the answer irrelevant instead of load-bearing.
//
// Without this test, tidying the path back alongside hostapd.pid looks like a
// simplification and breaks the hotspot on a box nobody has tested yet.
func TestDnsmasqPIDLivesInADirectoryOfItsOwn(t *testing.T) {
	p := DefaultPaths()

	dnsmasqDir := path.Dir(p.DnsmasqPID)
	hostapdDir := path.Dir(p.HostapdPID)
	if dnsmasqDir == hostapdDir {
		t.Fatalf("the dnsmasq pid file is in %s, the same directory as hostapd's. That directory "+
			"is 0750 root:caspian per docs/LAYOUT.md and dnsmasq runs as caspian, so it may not "+
			"be able to write there", dnsmasqDir)
	}
	if dnsmasqDir != "/run/caspian/dnsmasq" {
		t.Errorf("the dnsmasq pid directory is %s; docs/LAYOUT.md fixes it at /run/caspian/dnsmasq "+
			"(0700, caspian)", dnsmasqDir)
	}
	// And it must still be under /run, which is a tmpfs: a pid file that
	// survives a reboot can name a process id that has since been reused, and
	// the supervisor would then decide dnsmasq is already running and never
	// start it. That is why this did not go in /var/lib/caspian, which is the
	// other directory the service user owns.
	if !strings.HasPrefix(p.DnsmasqPID, "/run/") {
		t.Errorf("the dnsmasq pid file is at %s, outside /run; a pid file that survives a reboot "+
			"can name a reused process id", p.DnsmasqPID)
	}
}

// TestDnsmasqIsToldToWriteItsPIDWhereWeExpect closes the loop: the path above
// is only worth fixing if it is the one actually passed to dnsmasq.
func TestDnsmasqIsToldToWriteItsPIDWhereWeExpect(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)
	if _, err := s.Start(context.Background(), testPlan(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := "--pid-file=" + DefaultPaths().DnsmasqPID
	if !anyContains(rec.CommandLines(), want) {
		t.Errorf("dnsmasq was not told %q:\n%s", want, strings.Join(rec.CommandLines(), "\n"))
	}
}

// A lease file outlives the hotspot, so a box that has been switched off must
// not go on claiming the phone that was joined to it.
//
// MEASURED on 2026-08-30 through the real panel: engine stopped, hotspot down,
// one unexpired lease still on disk, and the dashboard said "Off" beside
// "1 device connected". dnsmasq's own teardown comment says the lease file
// remains, and the lease reader filters on expiry alone, so nothing in that
// chain knows the hotspot has gone. A lease is a record that something once
// asked for an address; it is not evidence that anything is connected now.
//
// This is the unit-level guard. The browser suite has the same case as a
// scenario, and the panel pins the rendered page as a golden.
func TestAStoppedHotspotClaimsNoDevicesHoweverManyLeasesRemain(t *testing.T) {
	rec := NewRecorder()
	s := newTestSupervisor(rec)

	// Exactly the leases the running case counts as two devices, on a machine
	// where nothing is serving. The file is the same; only the hotspot differs.
	rec.SetFile(testPaths().LeaseFile,
		"1788051600 02:00:5e:02:00:01 192.168.66.51 iPhone 01:02:00:5e:02:00:01\n"+
			"0 02:00:5e:02:00:05 192.168.66.60 kitchen-printer *\n")

	st, err := s.Status(context.Background(), "wlan0")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Running {
		t.Fatalf("this machine has no hotspot running, so the rest of this test proves nothing: %+v", st)
	}
	if got := st.DeviceCount(); got != 0 {
		t.Errorf("a hotspot that is not running reports %d device(s) connected to it, want 0. "+
			"The count comes from the lease file, which survives the hotspot, so the box goes on "+
			"naming a phone that left when the network it was on stopped existing.", got)
	}
}
