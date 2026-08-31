// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// Lease is one device that holds a DHCP lease on the hotspot.
type Lease struct {
	// Expiry is when the lease runs out. Zero means the lease never expires:
	// dnsmasq writes 0 in the first column for an infinite lease.
	Expiry time.Time

	// MAC is the client's hardware address, lower case, as written by
	// dnsmasq. For an IPv6 lease this column holds the IAID instead, so it is
	// kept as text rather than parsed into a hardware address.
	MAC string

	// IP is the leased address.
	IP netip.Addr

	// Hostname is the name the client asked to be known by, or empty when it
	// sent none. dnsmasq writes "*" in that case.
	Hostname string

	// ClientID is the DHCP client identifier, or empty when there is none.
	ClientID string
}

// Expired reports whether the lease has run out at now. A lease with no expiry
// never has.
func (l Lease) Expired(now time.Time) bool {
	if l.Expiry.IsZero() {
		return false
	}
	return now.After(l.Expiry)
}

// DisplayName is what the panel shows for this device. A device that sent no
// hostname is shown by address, because "unknown device" tells the user less
// than a number they can compare against their phone's WiFi screen.
func (l Lease) DisplayName() string {
	if l.Hostname != "" {
		return l.Hostname
	}
	if l.IP.IsValid() {
		return l.IP.String()
	}
	return "unknown device"
}

// ParseLeases reads a dnsmasq lease file.
//
// The format dnsmasq writes is one lease per line, five space-separated fields:
//
//	<expiry seconds since the epoch> <mac> <address> <hostname> <client id>
//
// with "*" standing in for an absent hostname or client id, and 0 in the first
// field meaning a lease that does not expire. When dnsmasq is also serving
// IPv6 the file starts with a "duid <duid>" line, which is not a lease.
//
// malformed counts lines that could not be read. They are skipped rather than
// failing the parse: a single corrupt line, which a lease file being appended
// to while it is read can produce, must not make the panel report zero devices
// when four are connected. The count is returned so the caller can say "one
// line unreadable" instead of quietly showing fewer devices than there are.
//
// err is returned only for a read failure, never for file content.
func ParseLeases(r io.Reader) (leases []Lease, malformed int, err error) {
	sc := bufio.NewScanner(r)
	// A lease line is short. The default 64 KiB token limit is ample, but a
	// binary file fed here by mistake could produce one enormous "line", so
	// the limit is stated rather than inherited.
	sc.Buffer(make([]byte, 0, 4096), 64*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// The IPv6 DUID header line is not a lease and not malformed.
		if strings.HasPrefix(line, "duid ") {
			continue
		}
		l, ok := parseLeaseLine(line)
		if !ok {
			malformed++
			continue
		}
		leases = append(leases, l)
	}
	if err := sc.Err(); err != nil {
		return leases, malformed, fmt.Errorf("hotspot: could not read the lease file: %w", err)
	}
	return leases, malformed, nil
}

func parseLeaseLine(line string) (Lease, bool) {
	f := strings.Fields(line)
	// Exactly five fields. Fewer is a truncated write; more would mean a
	// hostname with a space in it, which dnsmasq does not produce and which
	// cannot be split back apart unambiguously, so it is refused rather than
	// guessed at.
	if len(f) != 5 {
		return Lease{}, false
	}

	secs, err := strconv.ParseInt(f[0], 10, 64)
	if err != nil || secs < 0 {
		return Lease{}, false
	}
	addr, err := netip.ParseAddr(f[2])
	if err != nil {
		return Lease{}, false
	}

	l := Lease{
		MAC:      strings.ToLower(f[1]),
		IP:       addr,
		Hostname: f[3],
		ClientID: f[4],
	}
	if secs != 0 {
		l.Expiry = time.Unix(secs, 0).UTC()
	}
	if l.Hostname == "*" {
		l.Hostname = ""
	}
	if l.ClientID == "*" {
		l.ClientID = ""
	}
	return l, true
}

// ReadLeaseFile parses the lease file at path.
//
// A missing file is zero devices and no error. That is the normal state, not a
// fault: dnsmasq creates the file when it grants its first lease, so between
// the hotspot starting and the first device joining there is no file, and the
// panel must show "0 devices connected" rather than an error.
func ReadLeaseFile(path string) (leases []Lease, malformed int, err error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("hotspot: could not open the lease file: %w", err)
	}
	defer f.Close()
	return ParseLeases(f)
}

// ActiveLeases keeps the leases that have not expired at now.
//
// dnsmasq usually removes an expired lease from the file, but not instantly,
// and the file is also read after a restart before dnsmasq has tidied it. The
// count the panel shows is of devices with a live lease.
func ActiveLeases(leases []Lease, now time.Time) []Lease {
	out := make([]Lease, 0, len(leases))
	for _, l := range leases {
		if !l.Expired(now) {
			out = append(out, l)
		}
	}
	return out
}
