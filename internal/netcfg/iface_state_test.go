// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// The three states measured on the target, and what each predicate must say
// about them.
//
// The third row is the defect. An interface this package has just released,
// stripped and typed itself reports the channel its STATION link was using,
// because the driver keeps reporting it. A predicate that read a channel as
// evidence of use called that interface busy, and the readback proving the
// release worked refused it every time, so the appliance would never have
// started.
func TestWirelessIface_ThreeMeasuredStates(t *testing.T) {
	cases := []struct {
		name        string
		w           WirelessIface
		isAP        bool
		stationLink bool
		inUse       bool
	}{
		{
			name:        "station joined to a network",
			w:           WirelessIface{Name: "wlan0", Type: "managed", SSID: "HomeNet", Channel: 10},
			isAP:        false,
			stationLink: true,
			inUse:       true,
		},
		{
			name:        "access point with hostapd serving",
			w:           WirelessIface{Name: "wlan0", Type: "AP", SSID: "Caspian-Probe", Channel: 6},
			isAP:        true,
			stationLink: false,
			inUse:       true,
		},
		{
			// Released, stripped, typed, nothing serving. Channel 10 is the
			// station's old channel and is stale.
			name:        "access point with nothing serving it",
			w:           WirelessIface{Name: "wlan0", Type: "AP", SSID: "", Channel: 10},
			isAP:        true,
			stationLink: false,
			inUse:       false,
		},
		{
			// THE FOURTH MEASURED STATE, and the one that broke channel
			// selection. wlan0 after a previous hotspot: typed back to
			// managed, joined to nothing, still reporting the channel it last
			// used. This case asserted the OPPOSITE until 2026-08-30, and
			// that assertion is what pinned a 5GHz channel onto a box whose
			// user had asked for 2.4GHz.
			//
			// A channel is not an association, whether the link was probed or
			// not.
			name:        "station reporting a stale channel, link not probed",
			w:           WirelessIface{Name: "wlan0", Type: "managed", SSID: "", Channel: 36},
			isAP:        false,
			stationLink: false,
			inUse:       false,
		},
		{
			// The same interface with the probe done, which is what Detect
			// now produces. The answer must not depend on the channel either
			// way.
			name:        "station measured not connected, stale channel",
			w:           WirelessIface{Name: "wlan0", Type: "managed", SSID: "", Channel: 36, LinkKnown: true, Associated: false},
			isAP:        false,
			stationLink: false,
			inUse:       false,
		},
		{
			// And the direction that must still say yes: measured joined.
			name:        "station measured connected",
			w:           WirelessIface{Name: "wlan1", Type: "managed", SSID: "HomeNet", Channel: 44, LinkKnown: true, Associated: true},
			isAP:        false,
			stationLink: true,
			inUse:       true,
		},
		{
			// The measurement OUTRANKS a leftover SSID, not just a leftover
			// channel.
			name:        "station measured not connected but still naming a network",
			w:           WirelessIface{Name: "wlan1", Type: "managed", SSID: "HomeNet", Channel: 44, LinkKnown: true, Associated: false},
			isAP:        false,
			stationLink: false,
			inUse:       false,
		},
		{
			name:        "station holding nothing",
			w:           WirelessIface{Name: "wlan1", Type: "managed"},
			isAP:        false,
			stationLink: false,
			inUse:       false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.w.IsAccessPoint(); got != c.isAP {
				t.Errorf("IsAccessPoint = %v, want %v", got, c.isAP)
			}
			if got := c.w.StationLink(); got != c.stationLink {
				t.Errorf("StationLink = %v, want %v", got, c.stationLink)
			}
			if got := c.w.InUse(); got != c.inUse {
				t.Errorf("InUse = %v, want %v", got, c.inUse)
			}
		})
	}
}

// The same thing through the parser, from the bytes the box produced.
func TestCaptured_FreedAndTypedInterfaceReadsAsFree(t *testing.T) {
	ifaces, err := ParseIwDev(read(t, "capture-pi5-iw-dev-freed-ap.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("interfaces = %+v, want only wlan0", ifaces)
	}
	w := ifaces[0]
	if !w.IsAccessPoint() {
		t.Fatalf("type = %q, want AP", w.Type)
	}
	if w.SSID != "" {
		t.Fatalf("ssid = %q, want empty: nothing is serving this interface", w.SSID)
	}
	// The stale channel that caused the defect must still be in the fixture,
	// or this test is guarding nothing.
	if w.Channel != 10 {
		t.Fatalf("channel = %d, want the stale 10 the station link was using", w.Channel)
	}
	if w.InUse() {
		t.Error("a released, typed, unserved interface reads as in use; the appliance would never start")
	}
	if w.StationLink() {
		t.Error("an interface in AP mode is not a station link")
	}
}

// The two captures of the freed state, in the two formats iw prints, must
// agree. They were taken from the same interface in the same state, so a
// difference between them is a mistake in one of them.
func TestCaptured_BothViewsOfTheFreedStateAgree(t *testing.T) {
	info := read(t, "capture-pi5-iw-info-freed-ap.txt")
	dev := read(t, "capture-pi5-iw-dev-freed-ap.txt")

	for _, line := range []string{
		"type AP",
		"channel 10 (2457 MHz), width: 20 MHz, center1: 2457 MHz",
	} {
		contains(t, info, line)
		contains(t, dev, line)
	}
	if strings.Contains(info, "ssid") || strings.Contains(dev, "ssid") {
		t.Error("the freed state is defined by having no ssid; a capture showing one is the wrong state")
	}
}

// Releasing the interface from NetworkManager takes its P2P device with it.
//
// MEASURED: "iw dev" in the managed state lists wlan0 AND p2p-dev-wlan0, and
// in the released state lists wlan0 alone. nmcli reported the sibling as
// "unavailable" at the same moment.
//
// This is why the fixture had to be captured rather than derived. A file built
// by hand from the managed-state output carries a P2P device that is not there
// in the released state, and anything walking the device list walks one device
// too many. The derivation this replaced did exactly that, and the note beside
// it claimed nothing in it was invented.
func TestCaptured_ReleasingTheInterfaceRemovesItsP2PDevice(t *testing.T) {
	managed := read(t, "capture-pi5-iw-dev.txt")
	freed := read(t, "capture-pi5-iw-dev-freed-ap.txt")

	if !strings.Contains(managed, "Unnamed/non-netdev interface") {
		t.Fatal("the managed capture no longer shows the P2P stanza, so this test guards nothing")
	}
	if strings.Contains(freed, "Unnamed/non-netdev") || strings.Contains(freed, "P2P") {
		t.Error("the released capture shows a P2P device; releasing the interface removes it")
	}

	// And nmcli agrees, from the same moment.
	owners := ParseNmcliDeviceStatus(read(t, "capture-pi5-nmcli-after-release.txt"))
	if owners["wlan0"] != ManagedByNothing {
		t.Errorf("wlan0 = %q, want none after release", owners["wlan0"])
	}
	if _, ok := owners["p2p-dev-wlan0"]; !ok {
		t.Error("the post-release nmcli capture no longer lists the P2P sibling")
	}
}

// The serving state, from the box, is what the access point check must accept.
func TestCaptured_ServingAccessPointReportsItsSSID(t *testing.T) {
	body := read(t, "capture-pi5-iw-info-ap-serving.txt")
	contains(t, body, "ssid Caspian-Probe")
	contains(t, body, "type AP")
	// This is the fact that makes AssertHotspotIsAccessPoint possible at all:
	// iw reports the SSID for an access point on this driver.
	w := WirelessIface{Type: "AP", SSID: "Caspian-Probe", Channel: 6}
	if !w.InUse() {
		t.Error("an access point that is broadcasting is in use")
	}
}

// End to end: the readback that proves the release worked must PASS for the
// state a correct release actually leaves behind.
func TestAssertHotspotInterfaceReleased_PassesForAFreedAndTypedInterface(t *testing.T) {
	ctx := context.Background()
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q := *p
	q.Hotspot = "wlan0"
	q.HotspotSubnet = netip.MustParsePrefix("10.83.51.0/24")

	r := NewRecordingRunner()
	r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
	r.SetOutput("iw dev", read(t, "capture-pi5-iw-dev-freed-ap.txt"))
	r.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.83.51.1/24 fe80::2ecf:67ff:fe72:51f7/64 \n")

	if err := AssertHotspotInterfaceReleased(ctx, r, &q); err != nil {
		t.Fatalf("a released, stripped, typed interface must pass the readback: %v", err)
	}

	// And an access point somebody else is already serving must not. It is
	// not associated either, so "link" alone cannot see it; the SSID is what
	// gives it away.
	other := NewRecordingRunner()
	other.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
	other.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\tssid SomebodyElse\n\t\ttype AP\n")
	other.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.83.51.1/24 \n")
	err := AssertHotspotInterfaceReleased(ctx, other, &q)
	if !errors.Is(err, ErrHotspotNotReleased) {
		t.Fatalf("err = %v, want a refusal: something is already broadcasting on it", err)
	}
	contains(t, err.Error(), "already broadcasting")
}

// The stale channel must not pin a hotspot. On a box where a previous run left
// the interface typed, reading its channel would put the new access point on
// the old network's channel.
func TestPlan_StaleChannelOnATypedInterfaceDoesNotPinTheHotspot(t *testing.T) {
	s := pi5Captured()
	s.iwdev = "capture-pi5-iw-dev-freed-ap.txt"
	f := s.facts(t, BaseSysctlKnobs())

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.ChannelPinned {
		t.Errorf("the hotspot is pinned to channel %d, taken from an interface in AP mode "+
			"whose channel is the old station link's", p.Channel)
	}
	if p.Channel == 10 {
		t.Errorf("channel = 10, the stale one; a free radio should get a channel of its own")
	}
	// The radio has no station link, so the access point owns it outright.
	if p.HotspotIsVirtual {
		t.Errorf("hotspot = %q created beside %q, but nothing is using the radio",
			p.Hotspot, p.HotspotParent)
	}
}
