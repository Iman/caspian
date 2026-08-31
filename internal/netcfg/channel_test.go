// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// The third instance of one shape: A CHANNEL IS NOT AN ASSOCIATION.
//
// MEASURED on the target on 2026-08-30. wlan0 had hosted the hotspot, was put
// back to managed, and was joined to nothing. All three sources agreed:
//
//	iw dev wlan0 link         Not connected.
//	iw dev wlan0 info         type managed, channel 36, no ssid line
//	nmcli -t -f DEVICE,STATE  wlan0:disconnected
//
// The selector read the leftover channel as a live connection and pinned the
// access point to channel 36. Channel 36 is 5GHz; the user had asked for
// 2.4GHz in advanced settings; hostapd was handed a band and a channel that
// contradict each other and the start failed with fault=hotspot-failed. When
// the same pin had succeeded earlier the hotspot came up on 5GHz, which the
// test handset cannot see at all: its scan returns 2412 to 2462 MHz only. The
// panel said the hotspot was up, the phone could not find it, and both were
// true.
//
// This test drives the real bytes on both sides: the captured "iw dev" listing
// with wlan0 named and on a channel, and the captured "Not connected." that
// the box printed for it.
func TestChannel_AMeasuredNonAssociationBeatsAStaleChannelAndAStaleName(t *testing.T) {
	sc := pi5Captured()
	r := sc.runner(t)
	// The whole difference. Everything else is the machine as captured, where
	// wlan0 is listed with ssid HomeNet on channel 10.
	r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))

	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}
	w, ok := f.WirelessByName("wlan0")
	if !ok {
		t.Fatal("wlan0 is not in the facts")
	}
	if !w.LinkKnown {
		t.Fatal("the link was not probed, so this test is not testing what it says")
	}
	if w.Channel == 0 {
		t.Fatal("the capture no longer reports a channel for wlan0, so the stale-channel case is not modelled")
	}
	if w.StationLink() {
		t.Fatalf("wlan0 reads as a station on channel %d while the box says Not connected", w.Channel)
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	// The radio is free, so the access point uses the interface already there
	// rather than adding one beside a connection that does not exist.
	if p.Hotspot != "wlan0" || p.HotspotIsVirtual {
		t.Errorf("hotspot = %q virtual=%v, want wlan0 used directly", p.Hotspot, p.HotspotIsVirtual)
	}
	if p.ChannelPinned {
		t.Errorf("channel pinned to %d against a connection that does not exist", p.Channel)
	}
	// And the channel comes from the radio's own list, which starts at 1.
	if p.Channel != 1 {
		t.Errorf("channel = %d, want 1, the first channel this radio reported it can use", p.Channel)
	}
	if b := bandOf(p.Channel); b != "2.4GHz" {
		t.Errorf("channel %d is %s; the pin used to hand hostapd a 5GHz channel while the user had asked for 2.4GHz",
			p.Channel, b)
	}

	// And nothing may TELL the operator there is a connection.
	notes := strings.Join(p.Notes, "\n")
	notContains(t, notes, "existing WiFi connection")
	notContains(t, notes, "pinned to channel")
}

// The other direction, which must keep working: a measured association DOES
// pin, and the sentence names the network rather than asserting one in the
// abstract.
func TestChannel_AMeasuredAssociationStillPinsAndTheNoteNamesTheNetwork(t *testing.T) {
	sc := pi5Captured()
	r := sc.runner(t)
	r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-connected.txt"))

	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}
	w, _ := f.WirelessByName("wlan0")
	if !w.StationLink() {
		t.Fatal("a measured association must be a station link")
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !p.ChannelPinned || p.Channel != 10 {
		t.Fatalf("channel = %d pinned=%v, want the station's channel 10 pinned", p.Channel, p.ChannelPinned)
	}
	notes := strings.Join(p.Notes, "\n")
	contains(t, notes, "pinned to channel 10")
	// The network is NAMED. The sentence used to say "an existing WiFi
	// connection" whether or not one existed and whatever it was called.
	contains(t, notes, `"HomeNet"`)
}

// A note that asserts a connection may only appear when one was measured.
//
// This is the guard for the sentence itself rather than for the pin, because
// the sentence is what an operator reads and acts on. On 2026-08-30 it said
// the hotspot was pinned to "the channel an existing WiFi connection is using
// on wlan0" while wlan0 was joined to nothing, and it explained away the wrong
// channel so convincingly that the real cause was not looked for.
func TestChannel_NoNoteClaimsAConnectionThatWasNotMeasured(t *testing.T) {
	for _, tc := range []struct {
		name    string
		link    string
		wantAny bool
	}{
		{"not connected", "capture-pi5-iw-link-not-connected.txt", false},
		{"connected", "capture-pi5-iw-link-connected.txt", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := pi5Captured()
			r := sc.runner(t)
			r.SetOutput("iw dev wlan0 link", read(t, tc.link))
			f, err := Detect(context.Background(), r, BaseSysctlKnobs())
			if err != nil {
				t.Fatal(err)
			}
			p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			claims := false
			for _, n := range p.Notes {
				if strings.Contains(n, "WiFi connection") || strings.Contains(n, "internet connection") {
					claims = true
				}
			}
			if claims != tc.wantAny {
				t.Fatalf("a note claiming a connection: got %v, want %v.\nNotes: %v", claims, tc.wantAny, p.Notes)
			}
		})
	}
}

// bandOf is the test's own reading of a channel number, so that the assertion
// about 2.4GHz is made here and not borrowed from the package under test.
func bandOf(ch int) string {
	if ch >= 1 && ch <= 14 {
		return "2.4GHz"
	}
	return "5GHz"
}

// An access point is not asked whether it is a station.
//
// "Is this interface joined to a network" is a question about a STATION.
// Asking it of an access point is the category error this package has made
// three times in other forms, and what the command prints for an access point
// with hostapd running has never been measured here. So it is not asked, and
// nothing depends on the answer.
func TestChannel_TheLinkProbeSkipsAccessPointInterfaces(t *testing.T) {
	// The two-radio capture has wlan0 in AP mode and wlan1 a managed station.
	sc := twoRadioScenario("capture-pi5-ip-route-default.txt")
	r := sc.runner(t)
	if _, err := Detect(context.Background(), r, BaseSysctlKnobs()); err != nil {
		t.Fatal(err)
	}

	var probed []string
	for _, line := range r.Lines() {
		if strings.HasPrefix(line, "iw dev ") && strings.HasSuffix(line, " link") {
			probed = append(probed, strings.TrimSuffix(strings.TrimPrefix(line, "iw dev "), " link"))
		}
	}
	if len(probed) == 0 {
		t.Fatal("nothing was probed at all, so this test guards nothing")
	}
	for _, name := range probed {
		if name == "wlan0" {
			t.Errorf("probed %s, which the capture lists as type AP", name)
		}
	}
	found := false
	for _, name := range probed {
		if name == "wlan1" {
			found = true
		}
	}
	if !found {
		t.Errorf("the managed interface wlan1 was not probed; probed %v", probed)
	}
}

// The uplink is put in the candidate's station slot whatever its type, because
// it is a link that must not be disturbed. That is a different question from
// whether it is JOINED to anything, and only the second one may pin a channel.
//
// The machine here is measured in its parts: an interface of type AP carrying
// the default route is what the two-radio capture shows, and a radio reporting
// "#channels <= 1" is the built-in one.
func TestChannel_AnUplinkThatIsNotAStationDoesNotPin(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if ok, combo := phys[0].APWithStation(); !ok || combo.Channels != 1 {
		t.Fatalf("this test needs a radio declaring managed+AP with #channels <= 1; got ok=%v combo=%+v", ok, combo)
	}
	f := Facts{
		Phys: phys,
		Wireless: []WirelessIface{
			// Carrying the default route, typed AP, joined to nothing, and
			// still reporting a channel.
			{Name: "wlan0", Phy: phys[0].Name, Type: "AP", Channel: 36, LinkKnown: true, Associated: false},
		},
		Routes: []DefaultRoute{{
			Family: 4, Dev: "wlan0", Metric: 600,
			Gateway: netip.MustParseAddr("10.0.0.1"), Src: netip.MustParseAddr("10.0.0.222"),
		}},
		Links: []Link{
			{Name: "wlan0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.222/24")}},
		},
		Sysctl: map[string]string{},
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.Hotspot == p.Uplink {
		t.Fatalf("the hotspot took the uplink interface %q", p.Hotspot)
	}
	if p.ChannelPinned {
		t.Errorf("pinned to channel %d, read off an interface that is joined to nothing", p.Channel)
	}
	if p.Channel == 36 {
		t.Errorf("channel 36 came from the uplink's stale report, not from the radio's own list")
	}
}

// A blocked radio must not poison a machine that has a workable one.
//
// The refusal added on 2026-08-30 is for the case where the only route to an
// access point needs a capability this package does not have. It must not fire
// when something else can host the hotspot: mode B is a free USB adapter, and
// a second radio being busy says nothing about it.
func TestChannel_ABlockedRadioDoesNotRefuseAMachineThatHasAFreeOne(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-2radio-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f := Facts{
		Phys: phys,
		Wireless: []WirelessIface{
			// phy1 declares no combinations and is joined to a network: blocked.
			{Name: "wlan1", Phy: "phy1", Type: "managed", SSID: "HomeNet", Channel: 10,
				LinkKnown: true, Associated: true, Manager: ManagedByNetworkManager},
			// phy0 is free.
			{Name: "wlan0", Phy: "phy0", Type: "managed", LinkKnown: true, Associated: false,
				Manager: ManagedByNothing},
		},
		Routes: []DefaultRoute{{
			Family: 4, Dev: "eth0", Metric: 100,
			Gateway: netip.MustParseAddr("192.168.1.1"), Src: netip.MustParseAddr("192.168.1.50"),
		}},
		Links: []Link{
			{Name: "eth0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("192.168.1.50/24")}},
			{Name: "wlan0", State: "DOWN"},
			{Name: "wlan1", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.160/24")}},
		},
		Sysctl: map[string]string{},
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatalf("a busy second radio refused a machine whose first radio is free: %v", err)
	}
	if p.Hotspot != "wlan0" || p.HotspotPhy != "phy0" {
		t.Fatalf("hotspot = %q on %q, want wlan0 on the free radio phy0", p.Hotspot, p.HotspotPhy)
	}
	if p.ChannelPinned {
		t.Errorf("channel pinned to %d on a radio that carries no connection", p.Channel)
	}
}
