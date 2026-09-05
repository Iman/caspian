// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// InternetSharing is the macOS AccessPoint: Apple's Internet Sharing, driven
// through the preferences file it has always been configured by, with the
// operating system doing the rest (the radio in hotspot mode on ap1, the
// bridge, bootpd for DHCP, mDNSResponder's proxy for DNS).
//
// Why this and not our own access point: no USB Wi-Fi adapter has an
// AP-capable driver on Apple Silicon, DriverKit has no Wi-Fi family, and
// macOS exposes no API that puts a radio into access point mode. Internet
// Sharing is the only access point a Mac can host. It is also undocumented:
// Apple's own 2007 man page called the preferences file "a contract between
// the Sharing preferences pane and InternetSharing ... subject to change".
// Every key written below was read out of real dumps of that file and out of
// the strings of the configd plugin that consumes it (MobileInternetSharing);
// none was guessed. The research is under local/port-research on the
// development machine and is summarised in the port notes.
//
// How it is switched on. The plugin (InternetSharingPreference.bundle, inside
// configd) watches the file through SCPreferences and acts on the commit and
// apply notifications, NOT on the file changing. So the file is written and
// then re-saved through "scutil --prefs", which posts exactly those two
// notifications. If the network does not appear the daemon behind the plugin
// (launchd job com.apple.NetworkSharing) is kickstarted once. Both are
// UNVERIFIED on macOS 26 until measured with root on a real Mac, and the
// Status readback is what decides, never the exit code of either.
//
// What it does not do: it cannot read the SSID back from any tool that ships
// with macOS, so the beaconing check is the kernel's evidence (the bridge is
// up and carries the gateway) plus the preferences it wrote. And it does not
// touch the keychain: whether Tahoe reads the WPA passphrase from the file or
// from a System keychain item is one of the measurements still to make.
type InternetSharing struct {
	sys   System
	paths InternetSharingPaths
	now   func() time.Time

	// StartSettle is how long to wait between checks for the bridge to
	// appear, and StartTries how many of those checks to make.
	StartSettle time.Duration
	StartTries  int

	mu sync.Mutex
	// plan is the last plan Start was given, so that Stop can write the same
	// preferences back with sharing off rather than a different file.
	plan Plan
	have bool
	// poweredOn is the radio this driver switched on, so Stop can switch it
	// back off: undoing our own change, not switching off a radio the user
	// had on. Same rule as the Supervisor's rfkill handling.
	poweredOn string
}

// InternetSharingPaths is where Apple keeps the pieces, and where the tools are.
// Absolute paths, for the reason DefaultPaths gives: a daemon's PATH is not
// the user's, and the account that owns /usr/local/bin here is not root.
type InternetSharingPaths struct {
	// NATPrefs is the Internet Sharing preferences file the Sharing pane
	// writes and the configd plugin reads.
	NATPrefs string
	// NetworkPrefs is the SystemConfiguration preferences file, read for the
	// service UUID of the uplink interface: the plugin wants the service, not
	// the BSD name.
	NetworkPrefs string
	// LeaseFile is bootpd's lease database.
	LeaseFile string
	// Bridge is the interface Internet Sharing gives the clients.
	Bridge string

	Networksetup string
	Scutil       string
	Plutil       string
	Ifconfig     string
	Launchctl    string
}

// DefaultInternetSharingPaths are the paths on a stock Mac.
func DefaultInternetSharingPaths() InternetSharingPaths {
	return InternetSharingPaths{
		NATPrefs:     "/Library/Preferences/SystemConfiguration/com.apple.nat.plist",
		NetworkPrefs: "/Library/Preferences/SystemConfiguration/preferences.plist",
		LeaseFile:    "/var/db/dhcpd_leases",
		Bridge:       "bridge100",
		Networksetup: "/usr/sbin/networksetup",
		Scutil:       "/usr/sbin/scutil",
		Plutil:       "/usr/bin/plutil",
		Ifconfig:     "/sbin/ifconfig",
		Launchctl:    "/bin/launchctl",
	}
}

// natPrefsName is how scutil --prefs names the file: relative to the
// SystemConfiguration preferences directory.
const natPrefsName = "com.apple.nat.plist"

// networkSharingJob is the launchd label of /usr/libexec/InternetSharing on
// macOS 26 (measured: there is no com.apple.InternetSharing job any more).
const networkSharingJob = "system/com.apple.NetworkSharing"

// NewInternetSharing returns the macOS access point over sys.
func NewInternetSharing(sys System, paths InternetSharingPaths) *InternetSharing {
	return &InternetSharing{
		sys:         sys,
		paths:       paths,
		now:         time.Now,
		StartSettle: 500 * time.Millisecond,
		StartTries:  30,
	}
}

var _ AccessPoint = (*InternetSharing)(nil)

// SetClock replaces the clock, for tests that assert on lease expiry.
func (s *InternetSharing) SetClock(now func() time.Time) { s.now = now }

// Start brings Internet Sharing up for the plan, and is safe to call when it
// already is: a plan that matches the preferences on disk and a bridge that
// is already up cost nothing and disconnect nobody.
func (s *InternetSharing) Start(ctx context.Context, plan Plan) (Status, error) {
	dev := plan.AP.Interface
	if dev == "" {
		return Status{Reason: "The plan names no Wi-Fi interface."},
			errors.New("hotspot: internet sharing: the plan names no Wi-Fi interface")
	}
	if plan.AP.Uplink == "" {
		return Status{Reason: "The plan names no internet interface to share."},
			errors.New("hotspot: internet sharing: the plan names no uplink interface")
	}

	// 1. The radio. Internet Sharing will not raise a network on a radio that
	//    is switched off, and it does not switch it on itself.
	if err := s.ensureRadioOn(ctx, dev); err != nil {
		return Status{Reason: "The Wi-Fi radio is switched off and could not be switched on."}, err
	}

	// 2. Which network service is the uplink. The plugin's own log strings
	//    say "no external service id" and "no interface for external service
	//    id" when this is wrong, so it is resolved and checked here.
	service, err := s.uplinkService(ctx, plan.AP.Uplink)
	if err != nil {
		return Status{Reason: "The internet interface is not a network service macOS knows."}, err
	}

	// 3. The preferences. Written as a whole file every time, 0644 as the
	//    Sharing pane leaves it: the passphrase in it is the one the pane
	//    itself stores there, in the same form.
	rendered := renderNATPrefs(plan, service, true)
	s.mu.Lock()
	s.plan, s.have = plan, true
	s.mu.Unlock()
	changed, err := s.writeIfChanged(s.paths.NATPrefs, rendered)
	if err != nil {
		return Status{Reason: "The Internet Sharing preferences could not be written."}, err
	}

	// 4. Already up with these preferences: nothing to do, and saying so is
	//    the idempotence the panel's switch and the health check rely on.
	if !changed {
		if up, _ := s.bridgeUp(ctx, plan.DNS.Gateway); up {
			return s.Status(ctx, dev)
		}
	}

	// 5. Tell configd. The commit and apply notifications are what the plugin
	//    reacts to; "scutil --prefs" re-saving the file posts both.
	if err := s.nudge(ctx); err != nil {
		return Status{Reason: "macOS did not accept the request to start Internet Sharing."}, err
	}

	// 6. Read it back. The bridge carrying the gateway is the kernel's word
	//    that the network exists.
	if err := s.awaitBridge(ctx, plan.DNS.Gateway, true); err != nil {
		// One kickstart of the daemon, then one more wait. This is the
		// fallback every published toggle of Internet Sharing ends with.
		if _, kerr := s.sys.Run(ctx, s.paths.Launchctl, "kickstart", "-k", networkSharingJob); kerr == nil {
			err = s.awaitBridge(ctx, plan.DNS.Gateway, true)
		}
		if err != nil {
			return Status{Reason: "Internet Sharing did not bring the network up. The preferences were written and macOS was asked twice."}, err
		}
	}
	return s.Status(ctx, dev)
}

// Stop switches Internet Sharing off and puts back what Start changed.
func (s *InternetSharing) Stop(ctx context.Context) error {
	var errs []error

	s.mu.Lock()
	plan, have := s.plan, s.have
	powered := s.poweredOn
	s.poweredOn = ""
	s.mu.Unlock()

	// Sharing off. From the plan when this process started it; from the file
	// on disk when a previous process did, so that a fresh start after a
	// crash still stops what is running.
	var rendered string
	if have {
		service, err := s.uplinkService(ctx, plan.AP.Uplink)
		if err == nil {
			rendered = renderNATPrefs(plan, service, false)
		}
	}
	if rendered == "" {
		current, err := s.readPrefsXML(ctx)
		if err == nil {
			rendered = disableInNATPrefs(current)
		}
	}
	if rendered != "" {
		if _, err := s.writeIfChanged(s.paths.NATPrefs, rendered); err != nil {
			errs = append(errs, err)
		}
		if err := s.nudge(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := s.awaitBridge(ctx, netip.Addr{}, false); err != nil {
			if _, kerr := s.sys.Run(ctx, s.paths.Launchctl, "kickstart", "-k", networkSharingJob); kerr == nil {
				err = s.awaitBridge(ctx, netip.Addr{}, false)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if powered != "" {
		if res, err := s.sys.Run(ctx, s.paths.Networksetup, "-setairportpower", powered, "off"); err != nil || res.ExitCode != 0 {
			errs = append(errs, fmt.Errorf("hotspot: internet sharing: switching the radio back off: %v %s", err, strings.TrimSpace(res.Stderr)))
		}
	}
	return errors.Join(errs...)
}

// Status reports without changing anything.
func (s *InternetSharing) Status(ctx context.Context, iface string) (Status, error) {
	var st Status
	s.mu.Lock()
	plan, have := s.plan, s.have
	s.mu.Unlock()

	var gw netip.Addr
	if have {
		gw = plan.DNS.Gateway
	}
	up, addr := s.bridgeUp(ctx, gw)
	st.AccessPoint = ProcState{Running: up, Beaconing: up}
	// bootpd is socket-activated by launchd on the bridge the moment Internet
	// Sharing creates it, and the daemon writes its configuration first, so
	// the bridge existing is the evidence there is for DHCP.
	st.DHCP = ProcState{Running: up}
	st.Running = up

	if power, known := s.radioPower(ctx, iface); known {
		st.Radio = RadioState{HardBlocked: false, SoftBlocked: !power}
	}

	if up {
		if data, err := s.sys.ReadFile(s.paths.LeaseFile); err == nil {
			leases, malformed := ParseDHCPDLeases(string(data))
			if have {
				kept := leases[:0]
				for _, l := range leases {
					if plan.DNS.Subnet.Contains(l.IP) {
						kept = append(kept, l)
					}
				}
				leases = kept
			}
			st.Devices = ActiveLeases(leases, s.now())
			st.MalformedLeaseLines = malformed
		}
		return st, nil
	}

	switch {
	case !have:
		st.Reason = "Internet Sharing is not running."
	case addr.IsValid():
		st.Reason = fmt.Sprintf("Internet Sharing is serving %s, not the network this appliance configured.", addr)
	default:
		st.Reason = "Internet Sharing has not brought the network up."
	}
	return st, nil
}

// ---------------------------------------------------------------------------
// The pieces.
// ---------------------------------------------------------------------------

func (s *InternetSharing) ensureRadioOn(ctx context.Context, dev string) error {
	on, known := s.radioPower(ctx, dev)
	if known && on {
		return nil
	}
	res, err := s.sys.Run(ctx, s.paths.Networksetup, "-setairportpower", dev, "on")
	if err != nil {
		return fmt.Errorf("hotspot: internet sharing: switching the radio on: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("hotspot: internet sharing: networksetup exited %d switching the radio on: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	s.mu.Lock()
	s.poweredOn = dev
	s.mu.Unlock()
	return nil
}

// radioPower reads "networksetup -getairportpower <dev>": "Wi-Fi Power (en0): On".
func (s *InternetSharing) radioPower(ctx context.Context, dev string) (on bool, known bool) {
	if dev == "" {
		return false, false
	}
	res, err := s.sys.Run(ctx, s.paths.Networksetup, "-getairportpower", dev)
	if err != nil || res.ExitCode != 0 {
		return false, false
	}
	return ParseAirportPower(res.Stdout)
}

// ParseAirportPower reads the one line networksetup prints.
func ParseAirportPower(out string) (on bool, known bool) {
	line := strings.TrimSpace(out)
	switch {
	case strings.HasSuffix(line, ": On"):
		return true, true
	case strings.HasSuffix(line, ": Off"):
		return false, true
	}
	return false, false
}

// uplinkService finds the SystemConfiguration service UUID whose interface is
// the uplink. preferences.plist is binary on a modern Mac, so it is read
// through plutil as JSON rather than parsed here.
func (s *InternetSharing) uplinkService(ctx context.Context, uplink string) (string, error) {
	res, err := s.sys.Run(ctx, s.paths.Plutil, "-convert", "json", "-o", "-", s.paths.NetworkPrefs)
	if err != nil {
		return "", fmt.Errorf("hotspot: internet sharing: reading the network services: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("hotspot: internet sharing: plutil exited %d reading the network services: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	id, err := ServiceForInterface(res.Stdout, uplink)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ServiceForInterface finds the network service for a BSD interface name in
// the JSON form of preferences.plist.
func ServiceForInterface(prefsJSON, iface string) (string, error) {
	var doc struct {
		NetworkServices map[string]struct {
			Interface struct {
				DeviceName string `json:"DeviceName"`
			} `json:"Interface"`
		} `json:"NetworkServices"`
	}
	if err := json.Unmarshal([]byte(prefsJSON), &doc); err != nil {
		return "", fmt.Errorf("hotspot: internet sharing: the network services could not be read: %w", err)
	}
	var found []string
	for id, svc := range doc.NetworkServices {
		if svc.Interface.DeviceName == iface {
			found = append(found, id)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("hotspot: internet sharing: no network service uses %s, so macOS cannot share from it", iface)
	case 1:
		return found[0], nil
	default:
		// Deterministic: the same interface configured twice is unusual but
		// legal, and the plugin wants one.
		best := found[0]
		for _, id := range found[1:] {
			if id < best {
				best = id
			}
		}
		return best, nil
	}
}

// nudge re-saves the preferences through scutil, which posts the commit and
// apply notifications the configd plugin acts on.
func (s *InternetSharing) nudge(ctx context.Context) error {
	script := "get NAT\nset NAT\ncommit\napply\nquit\n"
	res, err := s.sys.RunInput(ctx, script, s.paths.Scutil, "--prefs", natPrefsName)
	if err != nil {
		return fmt.Errorf("hotspot: internet sharing: telling configd about the preferences: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("hotspot: internet sharing: scutil exited %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// awaitBridge polls for the bridge carrying gw (want true) or for the bridge
// being gone (want false).
func (s *InternetSharing) awaitBridge(ctx context.Context, gw netip.Addr, want bool) error {
	tries := s.StartTries
	if tries <= 0 {
		tries = 1
	}
	for i := 0; i < tries; i++ {
		// A cancelled caller is answered with its own error, never with "the
		// bridge is gone": a command that could not run says nothing about
		// the bridge.
		if err := ctx.Err(); err != nil {
			return err
		}
		up, _ := s.bridgeUp(ctx, gw)
		if up == want {
			return nil
		}
		if err := s.sys.Sleep(ctx, s.StartSettle); err != nil {
			return err
		}
	}
	if want {
		return fmt.Errorf("hotspot: internet sharing: %s did not come up carrying %s", s.paths.Bridge, gw)
	}
	return fmt.Errorf("hotspot: internet sharing: %s is still up", s.paths.Bridge)
}

// bridgeUp reports whether the bridge exists, is up and carries gw (any
// address when gw is zero), and the first IPv4 address it does carry.
func (s *InternetSharing) bridgeUp(ctx context.Context, gw netip.Addr) (bool, netip.Addr) {
	res, err := s.sys.Run(ctx, s.paths.Ifconfig, s.paths.Bridge)
	if err != nil || res.ExitCode != 0 {
		return false, netip.Addr{}
	}
	up, addrs := ParseIfconfigBrief(res.Stdout)
	var first netip.Addr
	for _, a := range addrs {
		if a.Is4() && !first.IsValid() {
			first = a
		}
		if gw.IsValid() && a == gw {
			return up, a
		}
	}
	if !gw.IsValid() {
		return up && first.IsValid(), first
	}
	return false, first
}

var ifconfigFlags = regexp.MustCompile(`flags=[0-9a-fA-F]+<([A-Z0-9_,]*)>`)

// ParseIfconfigBrief reads one interface's ifconfig block: whether UP is among
// the flags, and its inet addresses.
func ParseIfconfigBrief(out string) (up bool, addrs []netip.Addr) {
	if m := ifconfigFlags.FindStringSubmatch(out); m != nil {
		for _, fl := range strings.Split(m[1], ",") {
			if fl == "UP" {
				up = true
			}
		}
	}
	for _, raw := range strings.Split(out, "\n") {
		f := strings.Fields(raw)
		if len(f) >= 2 && f[0] == "inet" {
			if a, err := netip.ParseAddr(f[1]); err == nil {
				addrs = append(addrs, a)
			}
		}
	}
	return up, addrs
}

func (s *InternetSharing) writeIfChanged(path, content string) (bool, error) {
	if cur, err := s.sys.ReadFile(path); err == nil && string(cur) == content {
		return false, nil
	}
	if err := s.sys.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("hotspot: internet sharing: writing %s: %w", path, err)
	}
	return true, nil
}

// readPrefsXML reads the preferences as XML whatever format they are in on
// disk: configd re-saves them in binary form.
func (s *InternetSharing) readPrefsXML(ctx context.Context) (string, error) {
	res, err := s.sys.Run(ctx, s.paths.Plutil, "-convert", "xml1", "-o", "-", s.paths.NATPrefs)
	if err != nil || res.ExitCode != 0 {
		return "", fmt.Errorf("hotspot: internet sharing: reading %s: %v %s", s.paths.NATPrefs, err, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

// disableInNATPrefs sets the NAT dictionary's Enabled to 0 in XML text,
// leaving nested AirPort and PrimaryInterface keys and whitespace unchanged.
func disableInNATPrefs(xml string) string {
	// Match only the Enabled key directly inside NAT. PrimaryInterface and
	// AirPort also have Enabled keys, and either can be ordered last.
	start := strings.Index(xml, "<key>NAT</key>")
	if start < 0 {
		return xml
	}
	start += len("<key>NAT</key>")
	depth := 0
	for _, loc := range natDictTokens.FindAllStringIndex(xml[start:], -1) {
		i, j := start+loc[0], start+loc[1]
		switch xml[i:j] {
		case "<dict>":
			depth++
		case "</dict>":
			depth--
			if depth == 0 {
				return xml
			}
		default:
			if depth == 1 {
				value := natEnabledValue.FindStringIndex(xml[j:])
				if value != nil {
					return xml[:j] + strings.Replace(xml[j:j+value[1]], "<integer>1</integer>", "<integer>0</integer>", 1) + xml[j+value[1]:]
				}
				return xml
			}
		}
	}
	return xml
}

var natDictTokens = regexp.MustCompile(`</?dict>|<key>Enabled</key>`)
var natEnabledValue = regexp.MustCompile(`^\s*<integer>[01]</integer>`)

// renderNATPrefs writes the preferences file the Sharing pane would write for
// this plan. Key names and types are the ones read in real dumps of the file
// and in the plugin's strings: Enabled, PrimaryService, SharingDevices,
// SharingNetworkMask, SharingNetworkNumberStart, SharingNetworkNumberEnd,
// and the AirPort dictionary's NetworkName, NetworkPassword, Channel, Enabled
// and 40BitEncrypt.
//
// NetworkPassword is data, not a string, and the bytes are the passphrase in
// UTF-16LE: that is what a 2012 dump decodes to and what one 2026 tool that
// drives the same file writes. It is marked LIKELY in the research, and is
// the first thing to compare against a file the Sharing pane wrote.
func renderNATPrefs(plan Plan, service string, enabled bool) string {
	on := 0
	if enabled {
		on = 1
	}
	net := plan.DNS.Subnet.Masked().Addr().String()
	end := subnetEnd(plan.DNS.Subnet).String()
	mask := prefixMask(plan.DNS.Subnet)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n\t<key>NAT</key>\n\t<dict>\n")
	b.WriteString("\t\t<key>AirPort</key>\n\t\t<dict>\n")
	fmt.Fprintf(&b, "\t\t\t<key>40BitEncrypt</key>\n\t\t\t<integer>0</integer>\n")
	fmt.Fprintf(&b, "\t\t\t<key>Channel</key>\n\t\t\t<integer>%d</integer>\n", plan.AP.Channel)
	fmt.Fprintf(&b, "\t\t\t<key>Enabled</key>\n\t\t\t<integer>%d</integer>\n", on)
	fmt.Fprintf(&b, "\t\t\t<key>NetworkName</key>\n\t\t\t<string>%s</string>\n", xmlEscape(plan.AP.SSID))
	fmt.Fprintf(&b, "\t\t\t<key>NetworkPassword</key>\n\t\t\t<data>%s</data>\n", utf16LEBase64(plan.AP.Passphrase))
	b.WriteString("\t\t</dict>\n")
	fmt.Fprintf(&b, "\t\t<key>Enabled</key>\n\t\t<integer>%d</integer>\n", on)
	b.WriteString("\t\t<key>NatPortMapDisabled</key>\n\t\t<false/>\n")
	// NetworkSharing uses this source-interface dictionary together with the
	// service UUID. Older macOS releases tolerated its absence, but current
	// releases otherwise record "no external interface" and turn sharing off.
	b.WriteString("\t\t<key>PrimaryInterface</key>\n\t\t<dict>\n")
	fmt.Fprintf(&b, "\t\t\t<key>Device</key>\n\t\t\t<string>%s</string>\n", xmlEscape(plan.AP.Uplink))
	b.WriteString("\t\t\t<key>Enabled</key>\n\t\t\t<integer>0</integer>\n")
	b.WriteString("\t\t\t<key>HardwareKey</key>\n\t\t\t<string></string>\n")
	fmt.Fprintf(&b, "\t\t\t<key>PrimaryUserReadable</key>\n\t\t\t<string>%s</string>\n", xmlEscape(plan.AP.Uplink))
	b.WriteString("\t\t</dict>\n")
	fmt.Fprintf(&b, "\t\t<key>PrimaryService</key>\n\t\t<string>%s</string>\n", xmlEscape(service))
	fmt.Fprintf(&b, "\t\t<key>SharingDevices</key>\n\t\t<array>\n\t\t\t<string>%s</string>\n\t\t</array>\n", xmlEscape(plan.AP.Interface))
	fmt.Fprintf(&b, "\t\t<key>SharingNetworkMask</key>\n\t\t<string>%s</string>\n", mask)
	fmt.Fprintf(&b, "\t\t<key>SharingNetworkNumberEnd</key>\n\t\t<string>%s</string>\n", end)
	fmt.Fprintf(&b, "\t\t<key>SharingNetworkNumberStart</key>\n\t\t<string>%s</string>\n", net)
	b.WriteString("\t</dict>\n</dict>\n</plist>\n")
	return b.String()
}

// subnetEnd is the last address in the IPv4 subnet. Internet Sharing uses the
// network address for its start marker and the subnet's end marker separately;
// equal values are accepted by plist tooling but cause NetworkSharing to
// reject the AP with "no external interface" on current macOS.
func subnetEnd(p netip.Prefix) netip.Addr {
	p = p.Masked()
	a := p.Addr()
	if !a.Is4() || p.Bits() < 0 || p.Bits() > 32 {
		return a
	}
	a4 := a.As4()
	n := binary.BigEndian.Uint32(a4[:]) | (^uint32(0) >> uint(p.Bits()))
	return netip.AddrFrom4([4]byte{
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	}).WithZone("")
}

func prefixMask(p netip.Prefix) string {
	bits := p.Bits()
	if bits < 0 || bits > 32 {
		return "255.255.255.0"
	}
	var m uint32
	if bits > 0 {
		m = ^uint32(0) << (32 - bits)
	}
	return fmt.Sprintf("%d.%d.%d.%d", m>>24, (m>>16)&0xff, (m>>8)&0xff, m&0xff)
}

func utf16LEBase64(s string) string {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 2*len(units))
	for i, u := range units {
		buf[2*i] = byte(u)
		buf[2*i+1] = byte(u >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseDHCPDLeases reads bootpd's /var/db/dhcpd_leases:
//
//	{
//		name=phone
//		ip_address=192.0.2.7
//		hw_address=1,02:00:5e:00:00:07
//		identifier=1,02:00:5e:00:00:07
//		lease=0x68b8f2a1
//	}
//
// The lease is the expiry as a hexadecimal Unix time. The hardware address
// carries a leading type byte ("1," is Ethernet) which is stripped. A record
// with no readable address is counted as malformed, the same contract as
// ParseLeases so the panel can say the count may be short.
func ParseDHCPDLeases(data string) (leases []Lease, malformed int) {
	var cur map[string]string
	flush := func() {
		if cur == nil {
			return
		}
		defer func() { cur = nil }()
		ip, err := netip.ParseAddr(cur["ip_address"])
		if err != nil {
			malformed++
			return
		}
		l := Lease{IP: ip, Hostname: cur["name"], ClientID: cur["identifier"]}
		if hw := cur["hw_address"]; hw != "" {
			if _, rest, ok := strings.Cut(hw, ","); ok {
				hw = rest
			}
			l.MAC = strings.ToLower(hw)
		}
		if lease := cur["lease"]; lease != "" {
			if secs, err := strconv.ParseInt(strings.TrimPrefix(lease, "0x"), 16, 64); err == nil {
				l.Expiry = time.Unix(secs, 0).UTC()
			}
		}
		leases = append(leases, l)
	}
	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "{":
			flush()
			cur = map[string]string{}
		case line == "}":
			flush()
		case cur != nil:
			if k, v, ok := strings.Cut(line, "="); ok {
				cur[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	flush()
	return leases, malformed
}
