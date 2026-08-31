// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// THE DEFAULT ARRANGEMENT. Cable in, internet on eth0, hotspot on the built-in
// radio. It is proven end to end on hardware, repeatedly, and it is the
// arrangement every other change in this package has to leave alone.
//
// READ THIS BEFORE UPDATING ANYTHING BELOW. If one of these assertions fails,
// the behaviour of the default arrangement has changed. That is a finding, not
// a test to bring into line, and it needs the coordinator's decision before
// anything is edited here.
//
// The two states are both real. The radio may be joined to the house network,
// which is the captured machine, or joined to nothing, which is the state a
// previous hotspot leaves behind. They take different paths through the
// planner deliberately, and both are pinned.
func TestDefaultArrangement_WiredUplinkBuiltInRadio(t *testing.T) {
	t.Run("radio joined to a network: a second interface beside it", func(t *testing.T) {
		// The captured machine: eth0 carries the default route, wlan0 is on
		// HomeNet, phy0 declares managed+AP with #channels <= 1.
		sc := pi5Captured()
		r := sc.runner(t)
		r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-connected.txt"))
		f, err := Detect(context.Background(), r, BaseSysctlKnobs())
		if err != nil {
			t.Fatal(err)
		}
		p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if p.Uplink != "eth0" {
			t.Fatalf("uplink = %q, want eth0", p.Uplink)
		}
		if p.Hotspot != "ap0" || !p.HotspotIsVirtual || p.HotspotParent != "wlan0" {
			t.Fatalf("hotspot = %q virtual=%v parent=%q, want ap0 created beside wlan0",
				p.Hotspot, p.HotspotIsVirtual, p.HotspotParent)
		}
		if p.HotspotTakenOver {
			t.Fatal("the default arrangement now ENDS the box's own WiFi connection, which it has never done")
		}
		if !p.ChannelPinned || p.Channel != 10 {
			t.Fatalf("channel = %d pinned=%v, want 10 pinned to the station", p.Channel, p.ChannelPinned)
		}
		if p.HotspotFallback != "wlan0" {
			t.Fatalf("fallback = %q, want wlan0: the driver refuses the create while the station is up",
				p.HotspotFallback)
		}
		wantSequence(t, "hotspot steps", hotspotSteps(p, f), []string{
			"iw phy phy0 interface add ap0 type __ap",
			"nmcli device set ap0 managed no",
			"ip link set dev ap0 up",
			"ip address add 10.83.51.1/24 dev ap0",
		})
	})

	t.Run("radio joined to nothing: the interface is used as it is", func(t *testing.T) {
		// The state the box is in after a hotspot has run: wlan0 typed back to
		// managed, associated with nothing, still reporting a stale channel.
		sc := pi5Captured()
		r := sc.runner(t)
		r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
		f, err := Detect(context.Background(), r, BaseSysctlKnobs())
		if err != nil {
			t.Fatal(err)
		}
		p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if p.Uplink != "eth0" {
			t.Fatalf("uplink = %q, want eth0", p.Uplink)
		}
		if p.Hotspot != "wlan0" || p.HotspotIsVirtual || p.HotspotTakenOver {
			t.Fatalf("hotspot = %q virtual=%v takenOver=%v, want wlan0 used directly",
				p.Hotspot, p.HotspotIsVirtual, p.HotspotTakenOver)
		}
		if p.ChannelPinned {
			t.Fatalf("channel pinned to %d against a connection that does not exist", p.Channel)
		}
		if p.Channel != 1 {
			t.Fatalf("channel = %d, want 1", p.Channel)
		}
		// The address strip is there because the CAPTURED machine's wlan0
		// still holds 10.0.0.222/24 from the house network while being joined
		// to nothing. Serving DHCP on an interface holding that address is
		// the fault that answered a real device on somebody else's LAN.
		wantSequence(t, "hotspot steps", hotspotSteps(p, f), []string{
			"nmcli device set wlan0 managed no",
			"ip address del 10.0.0.222/24 dev wlan0",
			"ip link set dev wlan0 up",
			"ip address add 10.83.51.1/24 dev wlan0",
		})
	})
}

// hotspotSteps is the commands of a plan that touch the hotspot interface, in
// order, which is what the two subtests above compare.
func hotspotSteps(p *Plan, f Facts) []string {
	var out []string
	for _, st := range p.PreEngineSteps(f.Sysctl) {
		k := RunnerKey(st.Do)
		if strings.Contains(k, p.Hotspot) || strings.Contains(k, "interface add") {
			out = append(out, k)
		}
	}
	return out
}
