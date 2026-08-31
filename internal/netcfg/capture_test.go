// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// The tests in this file run against bytes captured from the target Raspberry
// Pi 5 on 2026-08-30. Their counterparts elsewhere run against authored
// scenarios. Both sets are kept because each catches what the other cannot:
// the captured bytes found a frequency parser that could not read "2412.0 MHz"
// and a sysctl read that returned nothing, and the authored bytes are the only
// coverage of a USB adapter, an IPv6 default route and a link that is down.
//
// See testdata/PROVENANCE.md for which file is which.

// iw 6.9 prints frequencies with one decimal place. The authored fixtures used
// the older integer form, so a parser that did strconv.Atoi on the field read
// every frequency as unparseable and dropped it. Phy.UsableChannels then
// returned nothing, and mode B on a radio with seventeen usable channels would
// have reported none.
func TestCaptured_ParseIwList_DecimalFrequencies(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(phys) != 1 || phys[0].Name != "phy0" {
		t.Fatalf("phys = %+v", phys)
	}
	p := phys[0]

	if len(p.Bands) != 2 {
		t.Fatalf("bands = %d", len(p.Bands))
	}
	// Assert the length before indexing. A test that panics takes the whole
	// binary with it and every test after it silently does not run, which is
	// a worse failure than the one it was trying to report: the frequency
	// parser regressing is exactly the case that empties this slice.
	if len(p.Bands[0].Frequencies) == 0 {
		t.Fatalf("band 1 parsed no frequencies at all")
	}
	first := p.Bands[0].Frequencies[0]
	if first.MHz != 2412 || first.Channel != 1 || first.MaxDBm != 20.0 {
		t.Errorf("first frequency = %+v, want 2412 MHz channel 1 at 20 dBm", first)
	}

	// 2.4 GHz channels 1 to 13 are usable and 14 is disabled; in 5 GHz only
	// 36, 40, 44 and 48 are free of "no IR" and "radar detection".
	usable := p.UsableChannels()
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 36, 40, 44, 48}
	if len(usable) != len(want) {
		t.Fatalf("usable channels = %v (%d), want %v (%d)", usable, len(usable), want, len(want))
	}
	for i := range want {
		if usable[i] != want[i] {
			t.Fatalf("usable channels = %v, want %v", usable, want)
		}
	}
}

// A radio that supports DFS still must not have an access point started on a
// channel needing radar detection: the AP would wait, then possibly move.
func TestCaptured_RadarAndNoIRChannelsAreExcluded(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var radar, disabled int
	for _, b := range phys[0].Bands {
		for _, f := range b.Frequencies {
			if f.Radar {
				radar++
				if f.Usable() {
					t.Errorf("channel %d needs radar detection but is reported usable", f.Channel)
				}
			}
			if f.Disabled {
				disabled++
			}
		}
	}
	if radar == 0 || disabled == 0 {
		t.Fatalf("fixture no longer exercises the flags: radar=%d disabled=%d", radar, disabled)
	}
}

// The captured radio reports "#{ managed } <= 2" in its first combination and
// the AP-capable pair in its second. The design records the second; this reads
// both from the radio rather than trusting the record.
func TestCaptured_InterfaceCombinations(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	p := phys[0]
	if len(p.Combinations) != 2 {
		t.Fatalf("combinations = %+v", p.Combinations)
	}
	if p.Combinations[0].Allows("managed", "AP") {
		t.Errorf("combination %q must not permit managed+AP", p.Combinations[0].Raw)
	}
	ok, combo := p.APWithStation()
	if !ok {
		t.Fatal("the radio must permit an access point beside the station")
	}
	if combo.Channels != 1 || combo.Total != 4 {
		t.Errorf("#channels = %d total = %d, want 1 and 4", combo.Channels, combo.Total)
	}
	if !p.SupportsAP() {
		t.Errorf("AP not among modes %v", p.Modes)
	}
}

// "iw dev" on this radio prints an "Unnamed/non-netdev interface" stanza for
// the P2P device, with the same fields at the same depth as a real interface,
// including a "type" line. It happens to come first here.
func TestCaptured_ParseIwDev_IgnoresTheUnnamedStanza(t *testing.T) {
	ifaces, err := ParseIwDev(read(t, "capture-pi5-iw-dev.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("interfaces = %+v, want only wlan0", ifaces)
	}
	w := ifaces[0]
	if w.Name != "wlan0" || w.Phy != "phy0" || w.Type != "managed" {
		t.Errorf("got %+v, want wlan0 on phy0 in managed mode", w)
	}
	if w.Channel != 10 || w.FreqMHz != 2457 {
		t.Errorf("channel/freq = %d/%d, want 10/2457", w.Channel, w.FreqMHz)
	}
	if w.SSID != "HomeNet" {
		t.Errorf("ssid = %q", w.SSID)
	}
}

// The same stanza placed AFTER the interface, which the captured ordering does
// not exercise. Before the fix the parser had no reset, so the stanza's fields
// landed on whichever interface was parsed last and wlan0 reported itself as a
// P2P-device with no channel. The channel is what pins the access point, so
// the consequence was a hotspot planned on the wrong channel.
func TestParseIwDev_UnnamedStanzaAfterAnInterfaceDoesNotPollute(t *testing.T) {
	ifaces, err := ParseIwDev(read(t, "scenario-modea-iw-dev.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("interfaces = %+v, want only wlan0", ifaces)
	}
	w := ifaces[0]
	if w.Type != "managed" {
		t.Errorf("type = %q, want managed: the unnamed stanza's type leaked onto wlan0", w.Type)
	}
	if w.Channel != 10 {
		t.Errorf("channel = %d, want 10: the unnamed stanza cleared it", w.Channel)
	}
}

// The whole pipeline against the captured machine.
func TestCaptured_PlanEndToEnd(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	if p.Mode != ModeWiredUplink {
		t.Errorf("mode = %v, want a wired uplink", p.Mode)
	}
	if p.Uplink != "eth0" || p.UplinkGateway != netip.MustParseAddr("10.0.0.1") {
		t.Errorf("uplink = %s via %v, want eth0 via 10.0.0.1", p.Uplink, p.UplinkGateway)
	}
	// One radio, already associated on channel 10, and its combinations allow
	// an access point beside the station but only on one channel.
	if !p.HotspotIsVirtual || p.Hotspot != "ap0" || p.HotspotParent != "wlan0" {
		t.Errorf("hotspot = %q virtual=%v parent=%q, want ap0 created beside wlan0",
			p.Hotspot, p.HotspotIsVirtual, p.HotspotParent)
	}
	if !p.ChannelPinned || p.Channel != 10 {
		t.Errorf("channel = %d pinned=%v, want 10 pinned", p.Channel, p.ChannelPinned)
	}
	// The box is on 10.0.0.0/24, which none of the pool collides with.
	if p.HotspotSubnet != netip.MustParsePrefix("10.83.51.0/24") {
		t.Errorf("hotspot subnet = %v", p.HotspotSubnet)
	}
	// No IPv6 default route on this box, and an IPv4 server, so nothing is
	// unpinnable.
	if len(p.UnpinnableServers) != 0 {
		t.Errorf("UnpinnableServers = %v", p.UnpinnableServers)
	}

	wantSequence(t, "pre-engine steps", stepKeys(p.PreEngineSteps(f.Sysctl)), []string{
		"nft -f -",
		"sysctl -w net.ipv4.ip_forward=1",
		"sysctl -w net.ipv4.conf.all.rp_filter=2",
		"sysctl -w net.ipv4.conf.default.rp_filter=2",
		"sysctl -w net.ipv6.conf.all.forwarding=0",
		"iw phy phy0 interface add ap0 type __ap",
		"nmcli device set ap0 managed no",
		"ip link set dev ap0 up",
		"ip address add 10.83.51.1/24 dev ap0",
		"ip route add 203.0.113.10/32 via 10.0.0.1 dev eth0 proto static metric 5",
	})
}

// Every sysctl change on the captured machine has an inverse built from a
// measured value. This is the end-to-end form of the "-n" defect: with the
// flag in place the map came back empty, every inverse was dropped, and
// uninstall left ip_forward and rp_filter changed on a box it promised to
// return to how it was found.
func TestCaptured_TeardownRestoresTheMeasuredSysctlValues(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())

	// The captured box has rp_filter 0 on conf.all and 2 on conf.default, so
	// the inverses must differ from each other. A single hardcoded value
	// would pass a weaker test.
	want := map[string]string{
		"sysctl -w net.ipv4.ip_forward=1":             "sysctl -w net.ipv4.ip_forward=0",
		"sysctl -w net.ipv4.conf.all.rp_filter=2":     "sysctl -w net.ipv4.conf.all.rp_filter=0",
		"sysctl -w net.ipv4.conf.default.rp_filter=2": "sysctl -w net.ipv4.conf.default.rp_filter=2",
	}
	// The uplink's own rp_filter must not be touched at all. The box reports
	// conf.eth0.rp_filter = 2; a fixture once guessed 0 from conf.all, and the
	// generated teardown would have written that 0 back, turning reverse-path
	// filtering OFF on the uplink of a machine that had it on. An uninstall
	// leaving the box weaker than it found it is the worst outcome available
	// to a teardown, and not writing the knob is what makes it impossible.
	for _, st := range p.AllSteps(f.Sysctl) {
		if strings.Contains(RunnerKey(st.Do), ".eth0.") || strings.Contains(RunnerKey(st.Undo), ".eth0.") {
			t.Errorf("a step touches the uplink's own sysctl: do=%q undo=%q", RunnerKey(st.Do), RunnerKey(st.Undo))
		}
	}
	got := map[string]string{}
	for _, s := range p.AllSteps(f.Sysctl) {
		if s.Do.Path != BinSysctl {
			continue
		}
		if s.Undo.IsZero() {
			t.Errorf("%s has no inverse", RunnerKey(s.Do))
			continue
		}
		got[RunnerKey(s.Do)] = RunnerKey(s.Undo)
	}
	for do, undo := range want {
		if got[do] != undo {
			t.Errorf("inverse of %q\n got: %q\nwant: %q", do, got[do], undo)
		}
	}
}

// A hand-set subnet that collides with the network the captured box is
// actually on is reported rather than silently corrected.
func TestCaptured_OverriddenSubnetThatCollidesIsReported(t *testing.T) {
	f := pi5Captured().facts(t, BaseSysctlKnobs())
	o := DefaultOptions()
	o.HotspotSubnet = netip.MustParsePrefix("10.0.0.0/24") // the network both interfaces are on
	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err != nil {
		t.Fatal(err)
	}
	contains(t, strings.Join(p.Notes, "\n"), "overlaps 10.0.0.0/24")
}

// Detection against the captured bytes, including that the sysctl read now
// returns every knob it asked for.
func TestCaptured_Detect(t *testing.T) {
	s := pi5Captured()
	r := s.runner(t)
	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Links) != 3 || len(f.Wireless) != 1 || len(f.Phys) != 1 {
		t.Fatalf("links=%d wireless=%d phys=%d", len(f.Links), len(f.Wireless), len(f.Phys))
	}
	// Two IPv4 defaults and no IPv6 default: the file is empty because the
	// box has none.
	if len(f.Routes) != 2 {
		t.Fatalf("routes = %+v, want the two IPv4 defaults only", f.Routes)
	}
	if _, ok := f.DefaultV6(); ok {
		t.Error("the captured box has no IPv6 default route")
	}
	for _, knob := range BaseSysctlKnobs() {
		if _, ok := f.Sysctl[knob]; !ok {
			t.Errorf("%s came back with no value", knob)
		}
	}
	if f.Sysctl["net.ipv4.conf.default.rp_filter"] != "2" {
		t.Errorf("conf.default.rp_filter = %q, want 2", f.Sysctl["net.ipv4.conf.default.rp_filter"])
	}
}

// The capture that decided the shape of this package's sysctl handling.
//
// Eight knobs were asked for on the target and five came back: the three
// naming ap0 and xray0 are absent because those interfaces did not exist when
// the read ran. "sysctl -e" skips what it cannot read, which is what let the
// read succeed at all, and it is also what leaves a knob with no measured
// value and therefore no inverse.
//
// This fixture is kept although no code path makes that read any more. It is
// the evidence for why none does.
func TestCaptured_KnobsForAbsentInterfacesComeBackMissing(t *testing.T) {
	got := ParseSysctl(read(t, "capture-pi5-sysctl-absent-interfaces.txt"))

	// The interfaces that existed when the read ran.
	if got["net.ipv4.conf.eth0.rp_filter"] != "2" {
		t.Errorf("conf.eth0.rp_filter = %q, want 2 as measured on the box", got["net.ipv4.conf.eth0.rp_filter"])
	}
	if got["net.ipv4.conf.all.rp_filter"] != "0" {
		t.Errorf("conf.all.rp_filter = %q, want 0", got["net.ipv4.conf.all.rp_filter"])
	}
	// The uplink held 2 while conf.all held 0. Guessing the interface value
	// from the global one, which a fixture once did, gets it exactly wrong.
	if got["net.ipv4.conf.eth0.rp_filter"] == got["net.ipv4.conf.all.rp_filter"] {
		t.Error("the fixture no longer shows an interface value differing from conf.all, " +
			"which is the whole reason a guessed inverse was unsafe")
	}
	// The interfaces that did not exist.
	for _, absent := range []string{
		"net.ipv4.conf.ap0.rp_filter",
		"net.ipv4.conf.xray0.rp_filter",
		"net.ipv6.conf.ap0.disable_ipv6",
	} {
		if v, ok := got[absent]; ok {
			t.Errorf("%s came back as %q; the fixture no longer models a knob whose interface is absent", absent, v)
		}
	}
}

// No sysctl step may name any interface, on any machine this package models.
func TestCaptured_NoPerInterfaceSysctlIsEverWritten(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			f, p := mustPlan(t, sc, DefaultOptions())
			for _, st := range p.AllSteps(f.Sysctl) {
				if st.Do.Path != BinSysctl {
					continue
				}
				for _, dev := range []string{p.Uplink, p.Hotspot, p.Tun, p.HotspotParent} {
					if dev != "" && strings.Contains(RunnerKey(st.Do), "."+dev+".") {
						t.Errorf("sysctl step names interface %s: %s", dev, RunnerKey(st.Do))
					}
				}
			}
		})
	}
}
