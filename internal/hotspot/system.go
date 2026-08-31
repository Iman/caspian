// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"io/fs"
	"time"
)

// Signal is the small set of signals this package sends. It is not the whole
// of syscall.Signal on purpose: the privileged side of this appliance accepts
// a short list of named actions, never an arbitrary one built from its
// client's input (docs/2026-08-29-design.md section 5.5).
type Signal int

const (
	// SignalTerm asks a process to stop.
	SignalTerm Signal = iota
	// SignalKill takes it away.
	SignalKill
)

func (s Signal) String() string {
	switch s {
	case SignalTerm:
		return "TERM"
	case SignalKill:
		return "KILL"
	default:
		return "unknown"
	}
}

// Result is the outcome of running a command.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// System is every effect this package can have on the machine.
//
// It exists so the supervisor can be tested without root, without a radio and
// without hostapd installed: tests substitute Recorder and assert on the exact
// argument vectors. It is also the list of capabilities the privileged helper
// has to expose, and keeping it short keeps that list short.
type System interface {
	// Run executes a command to completion and returns its result. err is
	// returned when the command could not be executed at all (not found, not
	// permitted, context cancelled); a command that ran and failed comes back
	// as a Result with a non-zero ExitCode and a nil error, because the
	// difference matters when explaining a failure to the user.
	Run(ctx context.Context, name string, args ...string) (Result, error)

	// WriteFile writes a file, creating parent directories as needed.
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// ReadFile reads a file. A missing file must come back as an error
	// satisfying errors.Is(err, fs.ErrNotExist).
	ReadFile(path string) ([]byte, error)

	// Remove deletes a file. Removing something that is not there is not an
	// error.
	Remove(path string) error

	// ProcessAlive reports whether a process with this pid exists.
	ProcessAlive(pid int) (bool, error)

	// SignalProcess sends sig to pid.
	SignalProcess(pid int, sig Signal) error

	// Sleep waits, or returns early if ctx is done. It is on the interface so
	// that the supervisor's waits cost nothing in tests.
	Sleep(ctx context.Context, d time.Duration) error
}

// Paths is where the pieces live on the appliance.
//
// Nothing here is derived from anything a user typed. The supervisor searches
// for a stale process by matching our own configuration path, and that only
// stays safe while these values stay ours.
type Paths struct {
	HostapdBinary     string
	HostapdCLIBinary  string
	DnsmasqBinary     string
	RfkillBinary      string
	PgrepBinary       string
	HostapdConf       string
	DnsmasqConf       string
	HostapdPID        string
	DnsmasqPID        string
	HostapdControlDir string
	LeaseFile         string
	StateDir          string
}

// DefaultPaths are the Raspberry Pi OS locations, matching docs/LAYOUT.md.
//
// Generated configuration goes under /run, not /etc: it is rewritten on every
// start, it contains the WPA2 passphrase, and /run is a tmpfs so it does not
// survive a power cut into a file nobody knows is there. The lease file has to
// outlive a dnsmasq restart, so it lives under /var/lib.
//
// WHY DnsmasqPID IS IN A SUBDIRECTORY OF ITS OWN.
//
// dnsmasq drops to the service user (see the user= line in the generated
// configuration), and docs/LAYOUT.md fixes /run/caspian at 0750 root:caspian,
// which gives the group no write bit. So whether dnsmasq could write a pid
// file directly in /run/caspian depends on whether it writes that file before
// or after it drops privileges. That is a fact about dnsmasq which this
// project has not measured.
//
// A design that only works if an unmeasured fact goes one way is a design
// waiting on a fact. Giving dnsmasq a directory it owns,
// /run/caspian/dnsmasq at 0700 caspian (docs/LAYOUT.md), makes the answer
// irrelevant in both directions, and costs one directory in the installer
// rather than a probe on every box.
//
// DO NOT instead relax /run/caspian to 0770. Permission to create and delete
// inside a directory comes from the directory, not the file, so a
// group-writable /run/caspian would let the unprivileged panel account delete
// /run/caspian/hostapd.conf and write its own, which the privileged side then
// hands to hostapd running as root. That turns a pid-file inconvenience into
// local privilege escalation. This is recorded as a standing warning in
// docs/LAYOUT.md; it is repeated here because this is the struct someone edits
// when the pid file will not write.
//
// hostapd is unaffected by any of it. It stays root, and everything it is
// pointed at (its configuration, its pid file, its control directory) is
// root-owned; dnsmasq changing identity does not change any file's mode.
//
// BINARY LOCATIONS, and which of them have actually been looked at.
//
// MEASURED on the target Raspberry Pi on 2026-08-30: /usr/sbin/hostapd and
// /usr/sbin/hostapd_cli exist (hostapd installed as 2:2.10-24), and
// /usr/sbin/dnsmasq exists and ran (dnsmasq 2.91). All three are also
// reachable as /sbin/... through the usr-merge symlink, so either spelling
// works; the /usr/sbin spelling is kept because it is the real path.
//
// ALSO MEASURED on 2026-08-30, closing the two that were open: rfkill is at
// /usr/sbin/rfkill and pgrep at /usr/bin/pgrep, both as this struct says, and
// both also reachable at /sbin/ and /bin/ through usr-merge. Every path in
// this struct has now been looked at on the target hardware.
//
// WHY THESE ARE ABSOLUTE PATHS AND NOT BARE NAMES. Neither rfkill nor pgrep
// was on the PATH of a non-interactive shell on that box: "command -v rfkill"
// found nothing, and an exec.LookPath("rfkill") would have failed the same
// way. A privileged service started by systemd gets a minimal environment, so
// resolving a program by name is a lookup that works when a person types it
// and fails when the appliance runs it. Naming the full path removes the
// environment from the question, and it is also the safer choice for a process
// running as root, since a PATH lookup is a place an attacker can put a
// different program.
//
// hostapd's Debian packaging masks its systemd unit and symlinks it to
// /dev/null. That suits this design exactly: this package spawns and
// supervises hostapd itself, and a systemd unit racing it to claim the radio
// would be a bug. It is why nothing here calls "systemctl unmask hostapd",
// which the implementation this replaces had to do
// (004-hotspot/install.sh:550) because it drove hostapd through systemd.
func DefaultPaths() Paths {
	return Paths{
		HostapdBinary:     "/usr/sbin/hostapd",
		HostapdCLIBinary:  "/usr/sbin/hostapd_cli",
		DnsmasqBinary:     "/usr/sbin/dnsmasq",
		RfkillBinary:      "/usr/sbin/rfkill",
		PgrepBinary:       "/usr/bin/pgrep",
		HostapdConf:       "/run/caspian/hostapd.conf",
		DnsmasqConf:       "/run/caspian/dnsmasq.conf",
		HostapdPID:        "/run/caspian/hostapd.pid",
		DnsmasqPID:        "/run/caspian/dnsmasq/dnsmasq.pid",
		HostapdControlDir: "/run/hostapd",
		LeaseFile:         "/var/lib/caspian/dnsmasq.leases",
		StateDir:          "/run/caspian",
	}
}
