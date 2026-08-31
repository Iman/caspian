// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

func TestUplinkChanged_DistinguishesTheThreeCases(t *testing.T) {
	eth := UplinkState{Interface: "eth0", Gateway: netip.MustParseAddr("192.168.1.1")}
	ethNewGw := UplinkState{Interface: "eth0", Gateway: netip.MustParseAddr("192.168.1.254")}
	wlan := UplinkState{Interface: "wlan0", Gateway: netip.MustParseAddr("10.0.0.1")}

	if changed, _ := UplinkChanged(eth, eth); changed {
		t.Error("an unchanged uplink must not report a change")
	}

	// A DHCP renewal that hands out a different gateway. Nothing fails
	// loudly: the pinned route still exists and still points somewhere.
	changed, reason := UplinkChanged(eth, ethNewGw)
	if !changed {
		t.Fatal("a new gateway on the same interface is a change")
	}
	contains(t, reason, "gateway on eth0 changed")

	changed, reason = UplinkChanged(eth, wlan)
	if !changed {
		t.Fatal("a different interface is a change")
	}
	contains(t, reason, "moved from eth0 to wlan0")

	changed, reason = UplinkChanged(eth, UplinkState{})
	if !changed {
		t.Fatal("a lost uplink is a change")
	}
	contains(t, reason, "lost")
}

func TestReadUplinkState(t *testing.T) {
	r := NewRecordingRunner()
	r.SetOutput("ip route show default", read(t, "scenario-modea-ip-route-default.txt"))
	got, err := ReadUplinkState(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	want := UplinkState{Interface: "eth0", Gateway: netip.MustParseAddr("192.168.1.1")}
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReadUplinkState_NoDefaultRouteIsNotAnError(t *testing.T) {
	r := NewRecordingRunner()
	r.SetOutput("ip route show default", "")
	got, err := ReadUplinkState(context.Background(), r)
	if err != nil {
		t.Fatalf("an unplugged cable is a state, not an error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want the zero state", got)
	}
}

// The pinned host route is what a gateway change silently invalidates, so
// re-derivation removes it through the old gateway and installs it through the
// new one, in that order.
func TestRederiveForUplink_MovesThePinnedRoute(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	newUplink := UplinkState{Interface: "eth0", Gateway: netip.MustParseAddr("192.168.1.254")}

	undo, redo, firewall, np := p.RederiveForUplink(newUplink)

	wantSequence(t, "remove old pins", stepKeys(undo), []string{
		"ip route del 203.0.113.10/32 via 192.168.1.1 dev eth0 metric 5",
	})
	wantSequence(t, "install new pins", stepKeys(redo), []string{
		"ip route add 203.0.113.10/32 via 192.168.1.254 dev eth0 proto static metric 5",
	})
	if np.UplinkGateway != netip.MustParseAddr("192.168.1.254") {
		t.Errorf("rebound plan gateway = %v", np.UplinkGateway)
	}
	// The original plan is not mutated: the caller may still need it to undo
	// what it applied.
	if p.UplinkGateway != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("the original plan was mutated: %v", p.UplinkGateway)
	}
	if firewall.Op != OpNft {
		t.Fatalf("expected a regenerated firewall step, got %q", firewall.Op)
	}
}

// The leak block names the uplink interface. A ruleset still naming the old
// interface stops blocking the moment traffic starts leaving by the new one,
// so moving to another interface has to regenerate the firewall too.
func TestRederiveForUplink_RegeneratesTheFirewallForANewInterface(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	_, _, firewall, np := p.RederiveForUplink(UplinkState{
		Interface: "usb0", Gateway: netip.MustParseAddr("192.168.42.1"),
	})
	contains(t, firewall.Do.Stdin, `iifname "ap0" oifname "usb0" drop`)
	notContains(t, firewall.Do.Stdin, `oifname "eth0"`)
	if np.Uplink != "usb0" {
		t.Errorf("rebound plan uplink = %q", np.Uplink)
	}
}

func TestRederiveForUplink_OnLinkUplink(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	_, redo, _, _ := p.RederiveForUplink(UplinkState{Interface: "ppp0", OnLink: true})
	got := strings.Join(stepKeys(redo), "\n")
	contains(t, got, "ip route add 203.0.113.10/32 dev ppp0 proto static metric 5")
	notContains(t, got, "via")
}
