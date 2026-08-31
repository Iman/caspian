// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testdata/dnsmasq.leases and its siblings were written by hand in the format
// dnsmasq documents for dhcp-leasefile. They were NOT captured from a running
// Raspberry Pi: no such box was reachable when this package was written, and
// that gap is recorded rather than papered over. What they are is a faithful
// reproduction of the five-column layout, including the "*" placeholders and
// the 0 expiry, which is what the parser has to handle.

// leaseNow is 2026-08-30 00:00:00 UTC, chosen so that four of the five leases
// in the sample are live and one has expired.
var leaseNow = time.Unix(1788048000, 0).UTC()

func TestParseSampleLeaseFile(t *testing.T) {
	leases, malformed, err := ReadLeaseFile(filepath.Join("testdata", "dnsmasq.leases"))
	if err != nil {
		t.Fatalf("ReadLeaseFile: %v", err)
	}
	if malformed != 0 {
		t.Errorf("the sample lease file produced %d malformed lines, want 0", malformed)
	}
	if len(leases) != 5 {
		t.Fatalf("parsed %d leases, want 5", len(leases))
	}

	first := leases[0]
	if first.Hostname != "iPhone" {
		t.Errorf("hostname = %q, want iPhone", first.Hostname)
	}
	if first.IP != netip.MustParseAddr("192.168.66.51") {
		t.Errorf("address = %s, want 192.168.66.51", first.IP)
	}
	if first.MAC != "02:00:5e:02:00:01" {
		t.Errorf("mac = %q", first.MAC)
	}
	if want := time.Unix(1788051600, 0).UTC(); !first.Expiry.Equal(want) {
		t.Errorf("expiry = %s, want %s", first.Expiry, want)
	}

	// A client that sent no hostname is written by dnsmasq as "*", which is
	// an absent name and not a device called "*".
	noName := leases[2]
	if noName.Hostname != "" {
		t.Errorf("the * placeholder was kept as a hostname: %q", noName.Hostname)
	}
	if noName.ClientID != "" {
		t.Errorf("the * placeholder was kept as a client id: %q", noName.ClientID)
	}
	if got := noName.DisplayName(); got != "192.168.66.53" {
		t.Errorf("a device with no hostname displays as %q, want its address", got)
	}

	// Expiry 0 means the lease never expires.
	infinite := leases[4]
	if !infinite.Expiry.IsZero() {
		t.Errorf("a 0 expiry became %s", infinite.Expiry)
	}
	if infinite.Expired(leaseNow.Add(100 * 365 * 24 * time.Hour)) {
		t.Error("a lease with no expiry was reported as expired")
	}
}

func TestActiveLeasesExcludesExpired(t *testing.T) {
	leases, _, err := ReadLeaseFile(filepath.Join("testdata", "dnsmasq.leases"))
	if err != nil {
		t.Fatalf("ReadLeaseFile: %v", err)
	}
	active := ActiveLeases(leases, leaseNow)
	if len(active) != 4 {
		t.Fatalf("%d live leases at %s, want 4", len(active), leaseNow)
	}
	for _, l := range active {
		if l.Hostname == "pixel-8" {
			t.Error("a lease that expired an hour ago is being counted as a connected device")
		}
	}
}

// TestParseMalformedLeaseFile is the case that decides whether a corrupt line
// makes the panel report zero devices when four are connected. It must not.
func TestParseMalformedLeaseFile(t *testing.T) {
	leases, malformed, err := ReadLeaseFile(filepath.Join("testdata", "dnsmasq.leases.malformed"))
	if err != nil {
		t.Fatalf("ReadLeaseFile returned an error for a file with bad lines in it: %v", err)
	}
	if len(leases) != 2 {
		t.Fatalf("parsed %d good leases, want 2: %+v", len(leases), leases)
	}
	if malformed != 4 {
		t.Errorf("counted %d malformed lines, want 4", malformed)
	}
	if leases[0].Hostname != "iPhone" || leases[1].IP != netip.MustParseAddr("192.168.66.53") {
		t.Errorf("the good lines either side of the bad ones were not parsed: %+v", leases)
	}
}

func TestParseEmptyLeaseFile(t *testing.T) {
	leases, malformed, err := ReadLeaseFile(filepath.Join("testdata", "dnsmasq.leases.empty"))
	if err != nil {
		t.Fatalf("an empty lease file is an error: %v", err)
	}
	if len(leases) != 0 || malformed != 0 {
		t.Errorf("an empty lease file produced %d leases and %d malformed lines", len(leases), malformed)
	}
}

// TestMissingLeaseFileIsZeroDevices covers the normal state between the
// hotspot starting and the first device joining. dnsmasq creates the file when
// it grants its first lease, so "no file" must read as "nobody has joined yet".
func TestMissingLeaseFileIsZeroDevices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.leases")
	leases, malformed, err := ReadLeaseFile(path)
	if err != nil {
		t.Fatalf("a missing lease file was reported as an error: %v", err)
	}
	if len(leases) != 0 || malformed != 0 {
		t.Errorf("a missing lease file produced %d leases and %d malformed lines", len(leases), malformed)
	}
}

func TestUnreadableLeaseFileIsAnError(t *testing.T) {
	// A file that exists but cannot be read is a real fault and must not be
	// silently reported as zero devices.
	dir := t.TempDir()
	path := filepath.Join(dir, "leases")
	if err := os.WriteFile(path, []byte("1788051600 02:00:5e:02:00:01 192.168.66.51 x *\n"), 0o000); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read a mode 000 file")
	}
	if _, _, err := ReadLeaseFile(path); err == nil {
		t.Error("an unreadable lease file was reported as zero devices")
	}
}

func TestParseIPv6LeaseFile(t *testing.T) {
	// dnsmasq writes a "duid ..." header line when it is serving IPv6. It is
	// not a lease and must not be counted as a malformed one.
	leases, malformed, err := ReadLeaseFile(filepath.Join("testdata", "dnsmasq.leases.ipv6"))
	if err != nil {
		t.Fatalf("ReadLeaseFile: %v", err)
	}
	if malformed != 0 {
		t.Errorf("the duid header was counted as %d malformed lines", malformed)
	}
	if len(leases) != 2 {
		t.Fatalf("parsed %d leases, want 2", len(leases))
	}
	if !leases[0].IP.Is6() {
		t.Errorf("the IPv6 lease parsed as %s", leases[0].IP)
	}
}

func TestParseLeasesEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		leases    int
		malformed int
	}{
		{"blank lines are ignored", "\n\n\n", 0, 0},
		{"four fields is truncated", "1788051600 02:00:5e:02:00:01 192.168.66.51 host\n", 0, 1},
		{"six fields is refused", "1788051600 02:00:5e:02:00:01 192.168.66.51 host name *\n", 0, 1},
		{"negative expiry", "-5 02:00:5e:02:00:01 192.168.66.51 host *\n", 0, 1},
		{"trailing whitespace", "  1788051600 02:00:5e:02:00:01 192.168.66.51 host *  \n", 1, 0},
		{"no trailing newline", "1788051600 02:00:5e:02:00:01 192.168.66.51 host *", 1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			leases, malformed, err := ParseLeases(strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("ParseLeases: %v", err)
			}
			if len(leases) != tc.leases || malformed != tc.malformed {
				t.Errorf("got %d leases and %d malformed, want %d and %d",
					len(leases), malformed, tc.leases, tc.malformed)
			}
		})
	}
}
