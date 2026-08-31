// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

// read loads a fixture. See testdata/PROVENANCE.md before treating a green
// test here as evidence about the target hardware.
func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(b)
}

func TestParseDefaultRoutes_Dual(t *testing.T) {
	routes, err := ParseDefaultRoutes(read(t, "scenario-modea-ip-route-default.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("want 2 routes, got %d: %+v", len(routes), routes)
	}
	got := routes[0]
	want := DefaultRoute{
		Family:  4,
		Gateway: netip.MustParseAddr("192.168.1.1"),
		Dev:     "eth0",
		Src:     netip.MustParseAddr("192.168.1.42"),
		Proto:   "dhcp",
		Metric:  100,
	}
	if got != want {
		t.Errorf("route 0:\n got %+v\nwant %+v", got, want)
	}
	if routes[1].Dev != "wlan0" || routes[1].Metric != 600 {
		t.Errorf("route 1: got %+v", routes[1])
	}
}

// The uplink is whichever interface carries the default route the kernel will
// actually use. On the target both the wired port and the built-in radio hold
// DHCP leases, so "there is only one default route" is not a safe assumption
// and the metric decides.
func TestPrimaryDefault_PicksLowestMetric(t *testing.T) {
	routes, err := ParseDefaultRoutes(read(t, "scenario-modea-ip-route-default.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f := Facts{Routes: routes}
	d, ok := f.PrimaryDefault()
	if !ok {
		t.Fatal("no primary default found")
	}
	if d.Dev != "eth0" {
		t.Errorf("primary default dev = %q, want eth0 (metric 100 beats 600)", d.Dev)
	}
}

// A cable pulled out leaves the route in the table with the linkdown flag. If
// that route is chosen the pinned host route to the user's server is installed
// through a dead gateway and the tunnel never comes up.
func TestPrimaryDefault_SkipsLinkDown(t *testing.T) {
	routes, err := ParseDefaultRoutes(read(t, "scenario-ip-route-default-linkdown.txt"))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := Facts{Routes: routes}.PrimaryDefault()
	if !ok {
		t.Fatal("no primary default found")
	}
	if d.Dev != "wlan0" {
		t.Errorf("primary default dev = %q, want wlan0 (eth0 is linkdown)", d.Dev)
	}
}

func TestParseDefaultRoutes_None(t *testing.T) {
	routes, err := ParseDefaultRoutes(read(t, "scenario-ip-route-default-none.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("want no routes, got %+v", routes)
	}
	if _, ok := (Facts{Routes: routes}).PrimaryDefault(); ok {
		t.Error("PrimaryDefault reported a route when there is none")
	}
}

// A point-to-point uplink has no gateway. The pinned host route then has to be
// written as "dev X" with no "via", so the parser must record the difference
// rather than leaving an invalid gateway that later formats as a bad argument.
func TestParseDefaultRoutes_OnLink(t *testing.T) {
	routes, err := ParseDefaultRoutes(read(t, "scenario-ip-route-default-onlink.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("want 1 route, got %+v", routes)
	}
	if !routes[0].OnLink || routes[0].Gateway.IsValid() || routes[0].Dev != "ppp0" {
		t.Errorf("got %+v, want OnLink route on ppp0 with no gateway", routes[0])
	}
}

func TestParseDefaultRoutes_V6(t *testing.T) {
	routes, err := ParseDefaultRoutes(read(t, "scenario-modea-ip-route6-default.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("want 1 route, got %+v", routes)
	}
	if routes[0].Family != 6 || routes[0].Metric != 1024 || routes[0].Dev != "eth0" {
		t.Errorf("got %+v, want family 6 metric 1024 on eth0", routes[0])
	}
}

func TestParseDefaultRoutes_RejectsGarbage(t *testing.T) {
	for _, in := range []string{
		"default via not-an-address dev eth0",
		"default via 192.168.1.1 proto dhcp",
		"default via 192.168.1.1 dev eth0 metric x",
		"default via 192.168.1.1 dev this-name-is-far-too-long-for-a-device",
	} {
		if _, err := ParseDefaultRoutes(in); err == nil {
			t.Errorf("ParseDefaultRoutes(%q) accepted a malformed line", in)
		}
	}
}

func TestParseBriefAddr(t *testing.T) {
	links, err := ParseBriefAddr(read(t, "scenario-modea-ip-br-addr.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("want 3 links, got %d: %+v", len(links), links)
	}
	if !links[0].IsLoopback() {
		t.Error("first link should be recognised as loopback")
	}
	eth := links[1]
	if eth.Name != "eth0" || eth.State != "UP" || len(eth.Prefixes) != 2 {
		t.Fatalf("eth0: got %+v", eth)
	}
	if eth.Prefixes[0] != netip.MustParsePrefix("192.168.1.42/24") {
		t.Errorf("eth0 first prefix = %v", eth.Prefixes[0])
	}
}

func TestParseBriefAddr_NoAddresses(t *testing.T) {
	links, err := ParseBriefAddr(read(t, "scenario-modeb-ip-br-addr.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	byName := map[string]Link{}
	for _, l := range links {
		byName[l.Name] = l
	}
	if l, ok := byName["wlan1"]; !ok || len(l.Prefixes) != 0 || l.State != "DOWN" {
		t.Errorf("wlan1: got %+v ok=%v, want a DOWN link with no prefixes", byName["wlan1"], ok)
	}
	if l := byName["wlan0"]; len(l.Prefixes) != 2 {
		t.Errorf("wlan0 prefixes = %v", l.Prefixes)
	}
}

// Bus detection is best effort: it is used to prefer a USB adapter for the
// access point in mode B and never to require one.
func TestParseLinkDetail_Bus(t *testing.T) {
	buses := ParseLinkDetail(read(t, "scenario-modeb-ip-d-link.txt"))
	want := map[string]string{"eth0": "platform", "wlan0": "sdio", "wlan1": "usb"}
	for name, w := range want {
		if buses[name] != w {
			t.Errorf("%s bus = %q, want %q", name, buses[name], w)
		}
	}
	if _, ok := buses["lo"]; ok {
		t.Errorf("lo reported a parent bus: %q", buses["lo"])
	}
}

// The same parse against the captured bytes. iproute2 6.15 prints parentbus at
// the end of the link/ether line rather than on a line of its own, and the
// built-in radio is on the SDIO bus, not platform: assuming "the built-in one
// is platform" would have misread the only radio the target has.
func TestParseLinkDetail_Bus_Captured(t *testing.T) {
	buses := ParseLinkDetail(read(t, "capture-pi5-ip-d-link.txt"))
	if buses["eth0"] != "platform" {
		t.Errorf("eth0 bus = %q, want platform", buses["eth0"])
	}
	if buses["wlan0"] != "sdio" {
		t.Errorf("wlan0 bus = %q, want sdio", buses["wlan0"])
	}
	if _, ok := buses["lo"]; ok {
		t.Errorf("lo reported a parent bus: %q", buses["lo"])
	}
}

func TestParseIwDev(t *testing.T) {
	ifaces, err := ParseIwDev(read(t, "scenario-modea-iw-dev.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("want 1 interface, got %+v", ifaces)
	}
	w := ifaces[0]
	if w.Name != "wlan0" || w.Phy != "phy0" || w.Type != "managed" {
		t.Errorf("got %+v", w)
	}
	// Channel matters: a radio whose combination says "#channels <= 1" pins
	// any access point to the channel the station link is already using.
	if w.Channel != 10 || w.FreqMHz != 2457 {
		t.Errorf("channel/freq = %d/%d, want 10/2457", w.Channel, w.FreqMHz)
	}
	if w.SSID != "HomeNet" {
		t.Errorf("ssid = %q", w.SSID)
	}
}

func TestParseIwDev_MultiplePhys(t *testing.T) {
	ifaces, err := ParseIwDev(read(t, "scenario-modeb-iw-dev.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("want 2 interfaces, got %+v", ifaces)
	}
	got := map[string]string{}
	for _, w := range ifaces {
		got[w.Name] = w.Phy
	}
	if got["wlan0"] != "phy0" || got["wlan1"] != "phy1" {
		t.Errorf("phy mapping = %v", got)
	}
}

func TestParseIwList_BuiltIn(t *testing.T) {
	phys, err := ParseIwList(read(t, "scenario-modea-iw-list.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(phys) != 1 {
		t.Fatalf("want 1 phy, got %d", len(phys))
	}
	p := phys[0]
	if p.Name != "phy0" || p.Index != 0 {
		t.Errorf("name/index = %q/%d", p.Name, p.Index)
	}
	if !p.SupportsAP() {
		t.Fatalf("AP support not detected; modes = %v", p.Modes)
	}
	wantModes := []string{"IBSS", "managed", "AP", "P2P-client", "P2P-GO", "P2P-device"}
	if len(p.Modes) != len(wantModes) {
		t.Fatalf("modes = %v, want %v", p.Modes, wantModes)
	}
	for i := range wantModes {
		if p.Modes[i] != wantModes[i] {
			t.Errorf("mode %d = %q, want %q", i, p.Modes[i], wantModes[i])
		}
	}
	if len(p.Combinations) != 2 {
		t.Fatalf("combinations = %d: %+v", len(p.Combinations), p.Combinations)
	}
}

// This is the hardware fact the design records for the target radio, read from
// the radio rather than assumed: an access point may run beside the station,
// and #channels <= 1 pins it to the station's channel.
func TestParseIwList_APBesideStationIsPinnedToOneChannel(t *testing.T) {
	phys, err := ParseIwList(read(t, "scenario-modea-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	ok, combo := phys[0].APWithStation()
	if !ok {
		t.Fatalf("managed+AP not permitted; combinations = %+v", phys[0].Combinations)
	}
	if combo.Channels != 1 {
		t.Errorf("#channels = %d, want 1", combo.Channels)
	}
	if combo.Total != 4 {
		t.Errorf("total = %d, want 4", combo.Total)
	}
	apLimit := -1
	for _, l := range combo.Limits {
		if l.Has("AP") {
			apLimit = l.Max
		}
	}
	if apLimit != 1 {
		t.Errorf("#{ AP } <= %d, want 1", apLimit)
	}
}

// The first combination on this radio lists managed but never AP. A naive
// "does any combination mention AP" test would pass on it; Allows must not.
func TestCombination_AllowsIsAnAssignmentNotAMembershipTest(t *testing.T) {
	phys, err := ParseIwList(read(t, "scenario-modea-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if phys[0].Combinations[0].Allows("managed", "AP") {
		t.Errorf("combination %q must not permit managed+AP", phys[0].Combinations[0].Raw)
	}
	if !phys[0].Combinations[1].Allows("managed", "AP") {
		t.Errorf("combination %q must permit managed+AP", phys[0].Combinations[1].Raw)
	}
}

// "#{ AP, mesh point } <= 8" allows eight access points and no station at all.
// Grouped types share one budget, so membership in the same group is not
// permission to run both.
func TestCombination_SharedGroupBudget(t *testing.T) {
	c, err := ParseCombination("#{ AP, mesh point } <= 8, total <= 8, #channels <= 1")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Allows("AP") {
		t.Error("one AP must be allowed")
	}
	if !c.Allows("AP", "AP") {
		t.Error("two APs must be allowed by <= 8")
	}
	if c.Allows("managed", "AP") {
		t.Error("managed is not in any group, so managed+AP must be refused")
	}

	single, err := ParseCombination("#{ AP, mesh point } <= 1, total <= 1, #channels <= 1")
	if err != nil {
		t.Fatal(err)
	}
	if single.Allows("AP", "mesh point") {
		t.Error("a shared group with max 1 cannot hold two interfaces")
	}
}

func TestCombination_TotalCaps(t *testing.T) {
	c, err := ParseCombination("#{ managed } <= 2, #{ AP } <= 2, total <= 1, #channels <= 1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Allows("managed", "AP") {
		t.Error("total <= 1 must refuse two interfaces however generous the groups are")
	}
}

func TestParseCombination_KeepsNotes(t *testing.T) {
	c, err := ParseCombination("#{ managed } <= 1, #{ AP } <= 1, total <= 4, #channels <= 1, STA/AP BI must match")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Notes) != 1 || c.Notes[0] != "STA/AP BI must match" {
		t.Errorf("notes = %v, want the STA/AP note preserved", c.Notes)
	}
}

func TestParseIwList_Frequencies(t *testing.T) {
	phys, err := ParseIwList(read(t, "scenario-modea-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(phys) == 0 {
		t.Fatal("no radios parsed")
	}
	p := phys[0]
	if len(p.Bands) != 2 {
		t.Fatalf("bands = %d: %+v", len(p.Bands), p.Bands)
	}
	if len(p.Bands[0].Frequencies) != 4 {
		t.Fatalf("band 1 frequencies = %+v", p.Bands[0].Frequencies)
	}
	first := p.Bands[0].Frequencies[0]
	if first.MHz != 2412 || first.Channel != 1 || first.MaxDBm != 20.0 {
		t.Errorf("band 1 first frequency = %+v", first)
	}
	// A channel needing radar detection, one flagged no-IR and one disabled
	// must all be excluded: an access point cannot be started on any of them.
	usable := p.UsableChannels()
	want := []int{1, 6, 10, 13, 36}
	if len(usable) != len(want) {
		t.Fatalf("usable channels = %v, want %v", usable, want)
	}
	for i := range want {
		if usable[i] != want[i] {
			t.Fatalf("usable channels = %v, want %v", usable, want)
		}
	}
}

func TestParseIwList_TwoPhys(t *testing.T) {
	phys, err := ParseIwList(read(t, "scenario-modeb-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(phys) != 2 {
		t.Fatalf("want 2 phys, got %d", len(phys))
	}
	byName := map[string]Phy{}
	for _, p := range phys {
		byName[p.Name] = p
	}
	if !byName["phy1"].SupportsAP() {
		t.Error("phy1 must report AP support")
	}
	if !byName["phy0"].SupportsAP() {
		t.Error("phy0 must report AP support")
	}
}

func TestParseIwList_NoAP(t *testing.T) {
	phys, err := ParseIwList(read(t, "scenario-iw-list-usb-noap.txt"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Phy{}
	for _, p := range phys {
		byName[p.Name] = p
	}
	if byName["phy1"].SupportsAP() {
		t.Errorf("phy1 modes %v must not be read as AP capable", byName["phy1"].Modes)
	}
}

func TestParseSysctl(t *testing.T) {
	got := ParseSysctl("net.ipv4.ip_forward = 0\nnet.ipv4.conf.all.rp_filter = 1\n\n")
	if got["net.ipv4.ip_forward"] != "0" || got["net.ipv4.conf.all.rp_filter"] != "1" {
		t.Errorf("got %v", got)
	}
}
