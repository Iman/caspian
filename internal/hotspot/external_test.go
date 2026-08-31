// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Checking the generated configuration against the program that consumes it.
//
// A golden file is a change detector: it proves the bytes are the same bytes
// somebody approved. It says nothing about whether dnsmasq still accepts them,
// and those are different questions, because a dnsmasq upgrade can invalidate
// a directive without a line of this package changing. This file closes that
// gap the same way internal/xcfg does with TestGoldenFilesAreAcceptedByTheEngine:
// take the committed artefact and put it through the real consumer.
//
// THE MEASUREMENT THIS IS BUILT ON. On 2026-08-30, on the target Raspberry Pi
// (Raspberry Pi OS, kernel 6.18.34, dnsmasq 2.91):
//
//	sudo /usr/sbin/dnsmasq --test --conf-file=<testdata/dnsmasq.golden>
//	dnsmasq: syntax check OK
//	exit 0
//
// A syntax check is a statement about ONE version of one program, which is why
// the version is recorded here AND read back at runtime by the tests below:
// the comment says what was true on the box that was measured, and the log
// line says what was true on the box that actually ran. That record was itself
// corrected once: this file first said 2.90, which was wrong. A wrong version
// in the record is worse than no version, because it reads as a measurement.
//
// THE CHECK DISCRIMINATES, measured on the same box in both directions:
//
//	a good config:      "dnsmasq: syntax check OK"        exit 0
//	an unknown option:  "dnsmasq: bad option at line 2"   exit 1
//
// Both directions matter. A checker that returns success for everything
// passes this whole file vacuously, and until those two runs the only
// evidence it discriminated came from stub binaries written alongside these
// tests, which prove the harness and say nothing about dnsmasq.
//
// THE EXIT CODE IS THE SIGNAL, AND A PIPE DESTROYS IT. assertDnsmasqAccepts
// runs dnsmasq directly through exec.Command and reads the process's own exit
// status. It must stay that way. In a shell, "dnsmasq --test ... | head"
// reports head's status, not dnsmasq's, so a failing check reads back as
// exit 0 and the config looks accepted when it was rejected. That is not
// hypothetical: the first attempt at the measurement above was piped into
// head and came back "exit=0" while dnsmasq had actually failed, and it took
// a second, unpiped run to get the real number. Never pipe the command whose
// exit code you are about to believe.
//
// THERE IS NO EQUIVALENT FOR hostapd. See the note above
// TestRenderHostapdGolden. The asymmetry is a property of the two programs,
// not a gap in this file.

// dnsmasqCandidates are where dnsmasq is found, ABSOLUTE PATH FIRST.
//
// The order is not cosmetic. On the target Pi neither rfkill nor pgrep was on
// the PATH of a non-interactive shell, and dnsmasq lives in the same /usr/sbin
// that was missing from it, so a bare-name lookup can fail on the very machine
// this check exists to run on. Trying /usr/sbin/dnsmasq first means the test
// runs there rather than skipping with "not installed", which would be a false
// clean: a skip that looks like a pass is exactly the failure this file is
// meant to remove. The remaining entries let a developer who has installed it
// locally get the check too.
var dnsmasqCandidates = []string{
	"/usr/sbin/dnsmasq",
	"dnsmasq",
	"/usr/local/sbin/dnsmasq",
	"/opt/homebrew/sbin/dnsmasq",
}

// findDnsmasq returns the path to a dnsmasq binary, or "" when there is none.
func findDnsmasq() string {
	for _, c := range dnsmasqCandidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// dnsmasqVersionLine returns the first line of "dnsmasq --version".
//
// Read at runtime rather than trusted from the comment above, because the
// whole value of this check is that it is a statement about a specific
// version, and the version that matters is the one on the machine running the
// test, not the one somebody wrote down.
func dnsmasqVersionLine(bin string) string {
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil && len(out) == 0 {
		return "unknown version"
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return line
}

// requireDnsmasq skips the test when dnsmasq is not installed.
//
// Skipping is right rather than failing: this package is developed on darwin,
// where there is no dnsmasq and no hotspot, and a test that cannot run is not
// a test that failed. It is logged loudly enough that nobody mistakes a skip
// for a pass.
func requireDnsmasq(t *testing.T) string {
	t.Helper()
	bin := findDnsmasq()
	if bin == "" {
		t.Skipf("dnsmasq is not installed on this machine, so the generated configuration "+
			"was NOT checked against it (looked for %s). This check runs on the appliance.",
			strings.Join(dnsmasqCandidates, ", "))
	}
	t.Logf("checking against %s (%s), running as uid %d", bin, dnsmasqVersionLine(bin), os.Getuid())
	return bin
}

// assertDnsmasqAccepts runs the real dnsmasq over a configuration file.
func assertDnsmasqAccepts(t *testing.T, bin, confPath, what string) {
	t.Helper()

	// The exact form that was measured on the Pi. The supervisor additionally
	// passes --pid-file and --conf-dir= when it really starts dnsmasq; those
	// are not part of a syntax check and were not part of the measurement.
	cmd := exec.Command(bin, "--test", "--conf-file="+confPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}

	// Two things this failure could be, and the message has to let a reader
	// tell them apart. A real syntax rejection is a defect in this package. A
	// refusal because the service account named by user= does not exist on
	// THIS machine, or because the check needs privileges it was not given, is
	// a property of the machine. The control case in
	// TestGeneratedDnsmasqVariantsAreAcceptedByDnsmasq exists to separate
	// them: it names an account that is present everywhere.
	t.Errorf("%s: dnsmasq rejected the configuration\n"+
		"  command: %s --test --conf-file=%s\n"+
		"  exit:    %v\n"+
		"  uid:     %d (the recorded measurement was made with sudo)\n"+
		"  output:  %s",
		what, bin, confPath, err, os.Getuid(), strings.TrimSpace(string(out)))
}

// TestGoldenDnsmasqConfigIsAcceptedByDnsmasq keeps the frozen bytes honest.
//
// It reads the COMMITTED file rather than re-rendering, for the reason
// internal/xcfg gives: a golden records what this package produced when
// somebody last ran -update, and an upgrade of the consuming program can
// invalidate that file without this package changing at all. Then the golden
// test stays green and the hotspot does not come up.
func TestGoldenDnsmasqConfigIsAcceptedByDnsmasq(t *testing.T) {
	bin := requireDnsmasq(t)

	path := filepath.Join("testdata", "dnsmasq.golden")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the golden file is missing, so this check would pass vacuously: %v", err)
	}
	assertDnsmasqAccepts(t, bin, path, "testdata/dnsmasq.golden")
}

// TestGeneratedDnsmasqVariantsAreAcceptedByDnsmasq covers what one golden
// cannot.
//
// The golden is a single configuration. The renderer has branches the golden
// never takes, and one of them is load-bearing: filter-AAAA is emitted only
// when asked for, and the reason it is off by default is that it needs dnsmasq
// 2.81 or newer while an older dnsmasq treats an unknown option as fatal.
//
// filter-AAAA was measured accepted on the target Pi on 2026-08-30 (dnsmasq
// 2.91, "syntax check OK", exit 0). That confirms the option exists on THAT
// box; it does not weaken the case for leaving it off by default, because the
// default has to be right on a box whose dnsmasq version nobody has looked at,
// and on an older one an unknown option is fatal rather than ignored. The
// measurement turns this variant from an assumption into a check that will
// confirm the same thing on every future run there.
func TestGeneratedDnsmasqVariantsAreAcceptedByDnsmasq(t *testing.T) {
	bin := requireDnsmasq(t)
	dir := t.TempDir()

	variants := []struct {
		name   string
		mutate func(*DNSConfig)
	}{
		// A control that names an account present on every unix. If this one
		// passes and the others fail, the failure is the caspian account
		// missing from this machine, not the configuration.
		{"control, root as the service account", func(c *DNSConfig) {
			c.ServiceUser, c.ServiceGroup = "root", "root"
		}},
		{"as generated", func(c *DNSConfig) {}},
		{"filter-AAAA on", func(c *DNSConfig) { c.FilterAAAA = true }},
		{"cache disabled", func(c *DNSConfig) { c.CacheSize = 0 }},
		{"lease time in whole minutes", func(c *DNSConfig) { c.LeaseTime = 90 * time.Minute }},
		{"lease time in seconds", func(c *DNSConfig) { c.LeaseTime = 150 * time.Second }},
		{"IPv6 loopback resolver", func(c *DNSConfig) {
			c.Upstream = netip.MustParseAddrPort("[::1]:5354")
		}},
		{"smaller subnet", func(c *DNSConfig) {
			c.Subnet = netip.MustParsePrefix("192.168.66.0/25")
			c.RangeEnd = netip.MustParseAddr("192.168.66.120")
		}},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			cfg := testDNS()
			v.mutate(&cfg)

			conf, err := RenderDnsmasq(cfg)
			if err != nil {
				t.Fatalf("RenderDnsmasq: %v", err)
			}
			path := filepath.Join(dir, safeFileName(v.name)+".conf")
			if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
				t.Fatalf("writing the variant: %v", err)
			}
			assertDnsmasqAccepts(t, bin, path, v.name)
		})
	}
}

// safeFileName turns a variant's prose name into a plain filename.
//
// The name is written by hand above and reaches a command line, so it is
// reduced to letters, digits and underscores rather than trusted: a comma or a
// space in a path is the kind of thing that fails on one machine and not
// another, and the point of this file is to remove doubt, not add a new source
// of it.
func safeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
