// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Supervisor starts, stops and health checks the two processes that make up
// the hotspot: hostapd for the access point, dnsmasq for DHCP and DNS.
//
// Every effect goes through System, so the whole of this type is testable
// without root and without either program installed.
type Supervisor struct {
	sys   System
	paths Paths
	now   func() time.Time

	// mu guards unblocked. Start and Stop are serialised by the privileged
	// service's operation lock, but this is a public type and nothing in its
	// contract says so, and Status is deliberately called from another
	// goroutine while a start runs.
	mu sync.Mutex

	// unblocked is the wireless devices THIS program soft-unblocked, so that
	// Stop can put them back. It is empty unless a start actually cleared a
	// block, which is the difference between undoing our own change and
	// switching off a radio the user had switched on. See reblockRadio.
	unblocked []RfkillDevice

	// StopGrace is how long a process gets after SIGTERM before SIGKILL.
	StopGrace time.Duration
	// StopPoll is how often liveness is rechecked while waiting.
	StopPoll time.Duration
	// StartSettle is how long to wait between checks for the pid file a
	// freshly started daemon writes.
	StartSettle time.Duration
	// StartTries is how many of those checks to make.
	StartTries int
}

// NewSupervisor returns a Supervisor over sys.
func NewSupervisor(sys System, paths Paths) *Supervisor {
	return &Supervisor{
		sys:         sys,
		paths:       paths,
		now:         time.Now,
		StopGrace:   3 * time.Second,
		StopPoll:    100 * time.Millisecond,
		StartSettle: 100 * time.Millisecond,
		StartTries:  20,
	}
}

// SetClock replaces the clock, for tests that assert on lease expiry.
func (s *Supervisor) SetClock(now func() time.Time) { s.now = now }

// unit names, used in the messages the user reads.
const (
	unitAP   = "hotspot"
	unitDHCP = "DHCP and DNS server"
)

// Start brings the hotspot up, and is safe to call when it is already up.
//
// Idempotence is not a convenience here. The panel's switch, a reconnect after
// the tunnel drops and a health check that decides to repair all reach this
// function, and restarting a working access point disconnects every device on
// it. So Start does nothing at all when the configuration on disk already
// matches the plan and both processes are alive; it restarts only what has
// actually changed or actually died.
func (s *Supervisor) Start(ctx context.Context, plan Plan) (Status, error) {
	st := Status{}

	// 1. The radio, before anything else. On this platform the wireless
	// device is frequently soft blocked and hostapd's own failure in that
	// state is not readable.
	radio, err := s.ensureRadioUnblocked(ctx)
	st.Radio = radio
	if err != nil {
		st.Reason = radio.Detail
		return st, err
	}

	// 2. Write the configuration, and notice whether it changed.
	apChanged, err := s.writeIfChanged(s.paths.HostapdConf, plan.HostapdConf, 0o600)
	if err != nil {
		st.Reason = "Caspian could not save the hotspot settings on this machine."
		return st, err
	}
	// 0600, not 0644: docs/LAYOUT.md mandates 0600 root for
	// /run/caspian/dnsmasq.conf. dnsmasq parses this file while it is still
	// root, before it drops to the service user, so nothing needs to read it
	// afterwards. This file was 0644 until the layout was checked against it.
	dnsChanged, err := s.writeIfChanged(s.paths.DnsmasqConf, plan.DnsmasqConf, 0o600)
	if err != nil {
		st.Reason = "Caspian could not save the DHCP and DNS settings on this machine."
		return st, err
	}

	// 3. Start each process only if it is not already running the current
	// configuration.
	apState, err := s.startProcess(ctx, procHostapd, s.paths.HostapdPID, apChanged, plan)
	st.AccessPoint = apState
	if err != nil {
		st.Reason = apState.Detail
		return st, err
	}
	dhcpState, err := s.startProcess(ctx, procDnsmasq, s.paths.DnsmasqPID, dnsChanged, plan)
	st.DHCP = dhcpState
	if err != nil {
		st.Reason = dhcpState.Detail
		return st, err
	}

	// 4. Ask hostapd whether the access point is actually beaconing, rather
	// than merely running.
	st.AccessPoint.Beaconing = s.apBeaconing(ctx, plan.AP.Interface)
	if !st.AccessPoint.Beaconing {
		st.Reason = "The hotspot software started but the network is not being broadcast yet."
		return st, nil
	}

	st.Running = st.AccessPoint.Running && st.DHCP.Running
	st.Devices, st.MalformedLeaseLines = s.devices()
	return st, nil
}

// Stop takes the hotspot down, and is safe to call when it is already down.
//
// Two paths, because a pid file is not a guarantee. The normal path reads our
// pid file and terminates that process. The second path exists because a
// previous run that was killed, or a machine that lost power mid-write, leaves
// a hostapd holding the radio with no usable pid file, and the next Start then
// fails with "could not configure driver mode" that nobody can interpret. So
// Stop also looks for any process whose command line names OUR configuration
// file and stops that too.
//
// The search pattern is our own path from Paths, never anything a user typed:
// a privileged helper that will terminate a process matching a caller-supplied
// pattern is not a boundary (docs/2026-08-29-design.md section 5.5).
func (s *Supervisor) Stop(ctx context.Context) error {
	var errs []error

	for _, p := range []struct {
		pidFile string
		conf    string
		unit    string
	}{
		{s.paths.HostapdPID, s.paths.HostapdConf, unitAP},
		{s.paths.DnsmasqPID, s.paths.DnsmasqConf, unitDHCP},
	} {
		if err := s.stopByPIDFile(ctx, p.pidFile); err != nil {
			errs = append(errs, fmt.Errorf("stopping the %s: %w", p.unit, err))
		}
		if err := s.stopStrays(ctx, p.conf); err != nil {
			errs = append(errs, fmt.Errorf("clearing a leftover %s process: %w", p.unit, err))
		}
		// The generated configuration, last for this unit and never before
		// the process that reads it is gone.
		//
		// docs/DEFECTS.md D2b: these survived a stop, and the hostapd one
		// carries the WPA2 passphrase. The cost is bounded, by /run being a
		// tmpfs cleared at boot and by the file being 0600 root, and it is
		// not nothing: the credential outlived the thing that needed it. The
		// removal is here rather than anywhere cleverer because this is
		// already the function that removes what a start created, and "the
		// appliance removes what it wrote" is easier to keep true than to
		// re-establish.
		//
		// It is safe for the next start rather than merely harmless:
		// writeIfChanged compares the file with the plan to decide whether to
		// restart a daemon, and a file that is gone reads as changed, which
		// is the correct answer after a stop. The stray search above matches
		// this path as a pgrep pattern and never reads the file, so removing
		// it afterwards takes nothing away from it.
		if err := s.sys.Remove(p.conf); err != nil {
			errs = append(errs, fmt.Errorf("removing the generated %s configuration: %w", p.unit, err))
		}
	}

	// The radio last, once nothing is using it. Switching it off under a
	// hostapd that is still running would be a change made to a device in use.
	if err := s.reblockRadio(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Status reports what is running now, without changing anything.
func (s *Supervisor) Status(ctx context.Context, iface string) (Status, error) {
	st := Status{}

	devs, err := s.rfkillList(ctx)
	if err == nil {
		st.Radio = radioStateFrom(devs, false)
	}

	apPID, _ := s.livePID(s.paths.HostapdPID)
	dhcpPID, _ := s.livePID(s.paths.DnsmasqPID)
	st.AccessPoint = ProcState{Running: apPID > 0, PID: apPID}
	st.DHCP = ProcState{Running: dhcpPID > 0, PID: dhcpPID}

	if st.AccessPoint.Running {
		st.AccessPoint.Beaconing = s.apBeaconing(ctx, iface)
	}
	// The lease file is the only source for this count, and it OUTLIVES the
	// hotspot: dnsmasq's own teardown says so in a comment, and leases.go
	// filters on expiry alone. So a box that has been switched off goes on
	// reporting the phone that was joined to it until that lease runs out.
	//
	// MEASURED on 2026-08-30 through the real panel, engine stopped and hotspot
	// down, with one unexpired lease on disk:
	//
	//	status-word   Off
	//	device-count  1
	//	device-line   1 device connected
	//
	// A lease on disk is not a joined device. It is a record that something
	// once asked for an address. Nothing can be connected to a hotspot that is
	// not running, so the count is only read when one is.
	if st.AccessPoint.Running && st.DHCP.Running {
		st.Devices, st.MalformedLeaseLines = s.devices()
	}

	switch {
	case st.Radio.HardBlocked:
		st.Reason = st.Radio.Detail
	case st.Radio.SoftBlocked:
		st.Reason = st.Radio.Detail
	case !st.AccessPoint.Running:
		st.Reason = "The hotspot is not running."
	case !st.AccessPoint.Beaconing:
		st.Reason = "The hotspot software is running but the network is not being broadcast."
	case !st.DHCP.Running:
		st.Reason = "The hotspot is broadcasting but devices cannot get an address, " +
			"because the DHCP and DNS server is not running."
	default:
		st.Running = true
	}
	return st, nil
}

// --- processes -------------------------------------------------------------

type procKind int

const (
	procHostapd procKind = iota
	procDnsmasq
)

func (s *Supervisor) startProcess(ctx context.Context, kind procKind, pidFile string, confChanged bool, plan Plan) (ProcState, error) {
	pid, err := s.livePID(pidFile)
	if err != nil {
		return ProcState{}, err
	}

	if pid > 0 && !confChanged {
		// Already running exactly this configuration. Doing nothing is the
		// whole point: a restart here would drop every joined device.
		return ProcState{Running: true, PID: pid, Detail: "already running"}, nil
	}
	if pid > 0 && confChanged {
		// Running the wrong configuration. It has to go before the new one
		// can have the radio.
		if err := s.terminate(ctx, pid); err != nil {
			return ProcState{}, err
		}
		if err := s.sys.Remove(pidFile); err != nil {
			return ProcState{}, err
		}
	}
	if pid == 0 {
		// Either nothing ran, or something ran and died leaving a pid file
		// behind. A stale pid file makes the next start read a pid that is
		// either dead or, worse, has been reused by an unrelated process.
		if err := s.sys.Remove(pidFile); err != nil {
			return ProcState{}, err
		}
		// A previous run may also have left a live process with no usable pid
		// file. It holds the radio or port 53 and has to go first.
		conf := s.paths.HostapdConf
		if kind == procDnsmasq {
			conf = s.paths.DnsmasqConf
		}
		if err := s.stopStrays(ctx, conf); err != nil {
			return ProcState{}, err
		}
	}

	name, args, unit := s.spawnCommand(kind, plan)
	res, err := s.sys.Run(ctx, name, args...)
	if err != nil {
		return ProcState{Detail: "The software Caspian needs for the " + unit +
			" could not be started on this machine."}, err
	}
	if res.ExitCode != 0 {
		detail := explainFailure(unit, res.ExitCode, res.Stderr, res.Stdout)
		return ProcState{Detail: detail}, fmt.Errorf("hotspot: %s exited with code %d", name, res.ExitCode)
	}

	newPID, err := s.awaitPID(ctx, pidFile)
	if err != nil {
		// The process started and then either died or never recorded itself.
		// For dnsmasq the likeliest cause on this appliance is specific
		// enough to name: it drops to the service user (see the user= line in
		// the generated configuration) and then cannot create its pid file,
		// because docs/LAYOUT.md puts /run/caspian at mode 0750 root:caspian,
		// which gives the group no write bit. Saying so beats "stopped with
		// no explanation", which is what this branch used to return.
		detail := explainFailure(unit, 0, res.Stderr, res.Stdout)
		if res.Stderr == "" && res.Stdout == "" {
			detail = "The " + unit + " started and then stopped without recording itself. " +
				"The usual cause is that it is not allowed to write to " + pidFile + "."
		}
		return ProcState{Detail: detail}, err
	}
	return ProcState{Running: true, PID: newPID, Detail: "started"}, nil
}

// spawnCommand builds the argument vector for one daemon.
//
// Both are asked to daemonize and to write a pid file, so this program does
// not have to stay the parent of either: the panel restarting must not take
// the hotspot down with it.
func (s *Supervisor) spawnCommand(kind procKind, plan Plan) (name string, args []string, unit string) {
	switch kind {
	case procHostapd:
		return s.paths.HostapdBinary,
			[]string{"-B", "-P", s.paths.HostapdPID, s.paths.HostapdConf},
			unitAP
	default:
		return s.paths.DnsmasqBinary,
			[]string{
				"--conf-file=" + s.paths.DnsmasqConf,
				"--pid-file=" + s.paths.DnsmasqPID,
				// dnsmasq reads /etc/dnsmasq.d by default even with an
				// explicit conf-file, and a package-supplied fragment there
				// could add a public resolver or turn query logging back on,
				// undoing two of the guarantees the generated file makes.
				"--conf-dir=",
			},
			unitDHCP
	}
}

// awaitPID waits for a freshly started daemon to write its pid file.
//
// Both daemons fork before the parent exits, so a zero exit code means the
// parent returned, not that the child came up. The pid file appearing and
// naming a live process is the first honest evidence that it did.
func (s *Supervisor) awaitPID(ctx context.Context, pidFile string) (int, error) {
	for i := 0; i < s.StartTries; i++ {
		pid, err := s.livePID(pidFile)
		if err != nil {
			return 0, err
		}
		if pid > 0 {
			return pid, nil
		}
		if err := s.sys.Sleep(ctx, s.StartSettle); err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("hotspot: %s did not report a running process", pidFile)
}

// stopByPIDFile terminates the process named in pidFile, then removes it.
func (s *Supervisor) stopByPIDFile(ctx context.Context, pidFile string) error {
	pid, err := s.livePID(pidFile)
	if err != nil {
		return err
	}
	if pid > 0 {
		if err := s.terminate(ctx, pid); err != nil {
			return err
		}
	}
	// Removed whether or not it was live: a pid file left behind for a dead
	// process is the thing that makes the next start read a reused pid.
	return s.sys.Remove(pidFile)
}

// terminate asks a process to stop, waits, and then takes it away.
func (s *Supervisor) terminate(ctx context.Context, pid int) error {
	if err := s.sys.SignalProcess(pid, SignalTerm); err != nil {
		return err
	}
	deadline := s.StopGrace
	for waited := time.Duration(0); waited < deadline; waited += s.StopPoll {
		alive, err := s.sys.ProcessAlive(pid)
		if err != nil {
			return err
		}
		if !alive {
			return nil
		}
		if err := s.sys.Sleep(ctx, s.StopPoll); err != nil {
			return err
		}
	}
	alive, err := s.sys.ProcessAlive(pid)
	if err != nil {
		return err
	}
	if !alive {
		return nil
	}
	// Stopping has to be reliable. A hostapd that ignored SIGTERM still holds
	// the radio, and leaving it there means the next start fails for a reason
	// that looks like broken hardware.
	if err := s.sys.SignalProcess(pid, SignalKill); err != nil {
		return err
	}
	return nil
}

// stopStrays finds and stops any process whose command line contains confPath.
//
// confPath is one of our own generated paths. pgrep exits 1 when nothing
// matches, which is the normal case and not a failure.
func (s *Supervisor) stopStrays(ctx context.Context, confPath string) error {
	if s.paths.PgrepBinary == "" {
		return nil
	}
	res, err := s.sys.Run(ctx, s.paths.PgrepBinary, "-f", confPath)
	if err != nil {
		// pgrep missing is not a reason to fail a stop. The pid file path has
		// already run.
		return nil
	}
	if res.ExitCode != 0 {
		return nil
	}
	for _, line := range strings.Fields(res.Stdout) {
		pid, convErr := strconv.Atoi(line)
		if convErr != nil || pid <= 1 {
			continue
		}
		if err := s.terminate(ctx, pid); err != nil {
			return err
		}
	}
	return nil
}

// livePID reads a pid file and returns the pid only if that process exists.
// A missing pid file, an unreadable one and a dead process are all 0.
func (s *Supervisor) livePID(pidFile string) (int, error) {
	b, err := s.sys.ReadFile(pidFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, nil
	}
	alive, err := s.sys.ProcessAlive(pid)
	if err != nil {
		return 0, err
	}
	if !alive {
		return 0, nil
	}
	return pid, nil
}

// writeIfChanged writes content only when it differs from what is on disk.
//
// The comparison is what makes Start idempotent: an unchanged file means the
// running process is running this configuration and must not be restarted.
func (s *Supervisor) writeIfChanged(path, content string, perm fs.FileMode) (changed bool, err error) {
	old, err := s.sys.ReadFile(path)
	if err == nil && bytes.Equal(old, []byte(content)) {
		return false, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := s.sys.WriteFile(path, []byte(content), perm); err != nil {
		return false, err
	}
	return true, nil
}

// --- radio -----------------------------------------------------------------

// ensureRadioUnblocked clears a soft block if there is one.
//
// The wireless device on this platform is frequently soft blocked, and it is
// the reference implementation's own conclusion too: it ran "rfkill unblock
// wifi" before every start (004-hotspot/install.sh:293 and
// 004-hotspot/xray-hotspot-fixed.sh:351). Doing it blindly, as those did, hides
// the case that matters: a HARD block cannot be cleared in software, so a blind
// unblock reports success and hostapd then fails with a message about the
// driver. So this reads the state, acts, reads it back, and says which of the
// two it was.
func (s *Supervisor) ensureRadioUnblocked(ctx context.Context) (RadioState, error) {
	devs, err := s.rfkillList(ctx)
	if err != nil {
		// rfkill missing is not fatal. On a machine where the radio is not
		// blocked the hotspot starts anyway, and if it is blocked hostapd's
		// own failure is explained by explainFailure.
		return RadioState{Detail: "Caspian could not check whether the wireless adapter is switched on."}, nil
	}

	wireless := wirelessDevices(devs)
	if len(wireless) == 0 {
		return RadioState{Present: false,
			Detail: "This machine reports no wireless adapter."}, nil
	}

	st := radioStateFrom(wireless, false)
	if st.HardBlocked {
		return st, errors.New("hotspot: the wireless adapter is hard blocked")
	}
	if !st.SoftBlocked {
		return st, nil
	}

	// Which devices were blocked BEFORE this ran, so that Stop puts back
	// exactly those and nothing else. Recorded before the unblock, because
	// afterwards the machine no longer says.
	var wereBlocked []RfkillDevice
	for _, d := range wireless {
		if d.SoftBlocked && !d.HardBlocked {
			wereBlocked = append(wereBlocked, d)
		}
	}

	if _, err := s.sys.Run(ctx, s.paths.RfkillBinary, "unblock", "wifi"); err != nil {
		st.Detail = "The wireless adapter is switched off and Caspian could not switch it back on."
		return st, err
	}

	// Read it back. An unblock that returned success and changed nothing is
	// the failure worth catching, and only a second read catches it.
	devs, err = s.rfkillList(ctx)
	if err != nil {
		st.Detail = "Caspian switched the wireless adapter on but could not confirm it."
		return st, nil
	}
	st = radioStateFrom(wirelessDevices(devs), true)
	if st.SoftBlocked || st.HardBlocked {
		return st, errors.New("hotspot: the wireless adapter is still blocked after unblocking it")
	}

	// Only now, with the readback showing it worked, is this a change this
	// program made and therefore one it owes an inverse for.
	s.mu.Lock()
	s.unblocked = wereBlocked
	s.mu.Unlock()
	return st, nil
}

// reblockRadio puts back a soft block this program cleared.
//
// # Why this exists
//
// docs/DEFECTS.md D2a: a box found with its radio switched off was left with it
// switched on. That is a change to somebody's machine that this appliance made
// and did not undo, and "the appliance removes what it wrote" is easier to keep
// true than to re-establish.
//
// # The three conditions, and none of them is assumed
//
//   - It only ever re-blocks devices recorded in Start, so a radio the user had
//     switched ON is never switched off by stopping this appliance.
//   - Those devices were recorded from a reading of the machine taken BEFORE
//     the unblock, not inferred afterwards.
//   - The current state is read again here, so a device somebody else has
//     already blocked, or that has since become hard blocked, is left alone.
//
// # What it cannot establish
//
// Whether the user unblocked the radio themselves while the appliance was
// running. That state is identical to the one this program created, so it is
// re-blocked. The cost is one rfkill command the user can reverse; the cost of
// the other choice is a machine quietly left in a state we changed.
//
// # It does not survive this process
//
// The record is in memory. A service that is killed and restarted has no way to
// know it unblocked anything, so it will not re-block, and netcfg.Recover does
// not cover this because it is not in the journal. It cannot be: rfkill is not
// on internal/netcfg's binary allowlist (netcfg/command.go, allowedBinaries),
// so the journal cannot carry the command. Closing that half needs a decision
// from the owner of that package, and it is written down in the report
// accompanying this change rather than worked around here.
//
// # THE ONE COMMAND HERE THAT HAS NOT BEEN RUN ON THE TARGET
//
// "rfkill block <index>". The unblock this reverses is "rfkill unblock wifi",
// which is in the shipped code and has run on the Pi. The per-device form is
// used instead of "block wifi" on purpose: on a machine with a second radio,
// blocking by type would switch off an adapter that was never blocked, which is
// the same class of defect this function exists to remove. If the identifier
// form turns out to be unsupported, the command fails, the failure is returned
// and logged, and the radio is left on, which is exactly today's behaviour plus
// a loud message. Check on the box with:
//
//	rfkill list ; sudo rfkill block 0 ; rfkill list ; sudo rfkill unblock 0
func (s *Supervisor) reblockRadio(ctx context.Context) error {
	s.mu.Lock()
	ours := s.unblocked
	s.unblocked = nil
	s.mu.Unlock()

	if len(ours) == 0 {
		return nil
	}

	// Read the machine rather than acting on what it looked like at start.
	devs, err := s.rfkillList(ctx)
	if err != nil {
		return fmt.Errorf("hotspot: reading the radio state before switching it back off: %w", err)
	}
	now := map[int]RfkillDevice{}
	for _, d := range wirelessDevices(devs) {
		now[d.Index] = d
	}

	var errs []error
	for _, d := range ours {
		cur, present := now[d.Index]
		switch {
		case !present:
			// The adapter is gone. There is nothing to put back, and saying
			// so is not a failure.
			continue
		case cur.SoftBlocked, cur.HardBlocked:
			// Already blocked, by somebody else or by hardware.
			continue
		}
		if _, err := s.sys.Run(ctx, s.paths.RfkillBinary, "block", strconv.Itoa(d.Index)); err != nil {
			errs = append(errs, fmt.Errorf("hotspot: switching the wireless adapter back off: %w", err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	// Read it back. A block that returned success and changed nothing is the
	// same failure the unblock guards against, and only a second read catches
	// it.
	devs, err = s.rfkillList(ctx)
	if err != nil {
		return fmt.Errorf("hotspot: confirming the radio was switched back off: %w", err)
	}
	after := map[int]RfkillDevice{}
	for _, d := range wirelessDevices(devs) {
		after[d.Index] = d
	}
	for _, d := range ours {
		if cur, present := after[d.Index]; present && !cur.SoftBlocked && !cur.HardBlocked {
			errs = append(errs, fmt.Errorf(
				"hotspot: %s was switched back off and still reports itself on, so this box has left the "+
					"radio in a state it did not find it in", d.Name))
		}
	}
	return errors.Join(errs...)
}

func (s *Supervisor) rfkillList(ctx context.Context) ([]RfkillDevice, error) {
	res, err := s.sys.Run(ctx, s.paths.RfkillBinary, "list")
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("hotspot: rfkill list exited with code %d", res.ExitCode)
	}
	return parseRfkillList(res.Stdout), nil
}

// radioStateFrom folds the per-device rfkill state into one answer. Any
// wireless device still blocked blocks the hotspot.
func radioStateFrom(devs []RfkillDevice, unblockedByUs bool) RadioState {
	st := RadioState{Present: len(devs) > 0, Unblocked: unblockedByUs}
	for _, d := range devs {
		if d.SoftBlocked {
			st.SoftBlocked = true
		}
		if d.HardBlocked {
			st.HardBlocked = true
		}
	}
	switch {
	case !st.Present:
		st.Detail = "This machine reports no wireless adapter."
	case st.HardBlocked:
		// Deliberately not "hard blocked". A hard block is a physical switch
		// or a firmware setting, and the only useful thing to say is what to
		// go and do.
		st.Detail = "The wireless adapter is switched off by a switch on the machine itself, " +
			"which Caspian cannot change. Turn the wireless switch on and try again."
	case st.SoftBlocked:
		st.Detail = "The wireless adapter is switched off in software and Caspian could not switch it back on."
	case unblockedByUs:
		st.Detail = "The wireless adapter was switched off. Caspian switched it back on."
	}
	return st
}

// --- health ----------------------------------------------------------------

// apBeaconing asks hostapd whether the access point is enabled.
//
// A running hostapd is not a working access point: it stays up after the
// interface fails to come up, and a process check reports a hotspot that is
// broadcasting nothing. hostapd_cli's status reports state=ENABLED only once
// the AP is actually beaconing.
func (s *Supervisor) apBeaconing(ctx context.Context, iface string) bool {
	if s.paths.HostapdCLIBinary == "" {
		return true
	}
	res, err := s.sys.Run(ctx, s.paths.HostapdCLIBinary,
		"-p", s.paths.HostapdControlDir, "-i", iface, "status")
	if err != nil || res.ExitCode != 0 {
		return false
	}
	return parseHostapdStatus(res.Stdout)["state"] == "ENABLED"
}

// parseHostapdStatus reads hostapd_cli's key=value output.
func parseHostapdStatus(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return m
}

// devices reads the lease file and keeps the live leases.
func (s *Supervisor) devices() ([]Lease, int) {
	b, err := s.sys.ReadFile(s.paths.LeaseFile)
	if err != nil {
		// No lease file is no devices, not a fault: dnsmasq creates it when
		// it grants the first lease.
		return nil, 0
	}
	leases, malformed, err := ParseLeases(bytes.NewReader(b))
	if err != nil {
		return nil, malformed
	}
	return ActiveLeases(leases, s.now()), malformed
}
