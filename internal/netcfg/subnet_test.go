// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"errors"
	"net/netip"
	"testing"
)

func TestOverlaps(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"192.168.1.0/24", "192.168.1.0/24", true},
		{"192.168.1.0/24", "192.168.1.128/25", true},
		{"192.168.0.0/16", "192.168.1.0/24", true},
		{"192.168.1.0/24", "192.168.2.0/24", false},
		{"10.0.0.0/8", "10.83.51.0/24", true},
		{"10.83.51.0/24", "10.83.52.0/24", false},
		// An unmasked prefix must compare as the network it belongs to. This
		// matters because "ip -br addr" prints host addresses with a prefix
		// length, never network addresses.
		{"192.168.1.42/24", "192.168.1.0/24", true},
		// Different families never overlap.
		{"192.168.1.0/24", "fe80::/64", false},
	}
	for _, c := range cases {
		got := Overlaps(netip.MustParsePrefix(c.a), netip.MustParsePrefix(c.b))
		if got != c.want {
			t.Errorf("Overlaps(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestChooseSubnet_AvoidsWhatTheBoxIsOn(t *testing.T) {
	pool := DefaultHotspotPool()
	taken := []netip.Prefix{
		netip.MustParsePrefix("192.168.1.42/24"),
		netip.MustParsePrefix("192.168.1.57/24"),
	}
	got, err := ChooseSubnet(pool, taken)
	if err != nil {
		t.Fatal(err)
	}
	if got != netip.MustParsePrefix("10.83.51.0/24") {
		t.Errorf("chose %v, want the first free candidate 10.83.51.0/24", got)
	}
}

func TestChooseSubnet_SkipsEachCollisionInTurn(t *testing.T) {
	pool := DefaultHotspotPool()
	taken := []netip.Prefix{
		netip.MustParsePrefix("10.83.51.7/24"),
		netip.MustParsePrefix("10.174.29.0/24"),
	}
	got, err := ChooseSubnet(pool, taken)
	if err != nil {
		t.Fatal(err)
	}
	if got != netip.MustParsePrefix("172.28.113.0/24") {
		t.Errorf("chose %v, want 172.28.113.0/24", got)
	}
}

// A box behind a 10.0.0.0/8 supernet collides with every 10/8 candidate, and
// the chooser must walk past all of them rather than returning the first
// entry.
func TestChooseSubnet_WalksPastASupernet(t *testing.T) {
	got, err := ChooseSubnet(DefaultHotspotPool(), []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Addr().String()[:3] == "10." {
		t.Errorf("chose %v, which is inside the 10.0.0.0/8 the box is already on", got)
	}
	if got != netip.MustParsePrefix("172.28.113.0/24") {
		t.Errorf("chose %v, want 172.28.113.0/24", got)
	}
}

func TestChooseSubnet_Exhausted(t *testing.T) {
	taken := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	_, err := ChooseSubnet(DefaultHotspotPool(), taken)
	if !errors.Is(err, ErrNoFreeSubnet) {
		t.Fatalf("err = %v, want ErrNoFreeSubnet", err)
	}
	var um interface{ UserMessage() string }
	if !errors.As(err, &um) {
		t.Fatal("refusal must carry wording for the panel")
	}
	contains(t, um.UserMessage(), "advanced settings")
}

// The tunnel pool is outside RFC 1918 on purpose, so it cannot collide with a
// home network or with a client's own VPN.
func TestTunnelPool_IsNotPrivateSpace(t *testing.T) {
	private := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	for _, cand := range DefaultTunnelPool() {
		for _, p := range private {
			if Overlaps(cand, p) {
				t.Errorf("tunnel candidate %v is inside private space %v", cand, p)
			}
		}
	}
}

// The pool deliberately avoids the ranges a domestic router or a VPN tutorial
// hands out, because a collision there is invisible until a client cannot
// reach anything.
func TestHotspotPool_AvoidsTheCommonRanges(t *testing.T) {
	avoid := []netip.Prefix{
		netip.MustParsePrefix("192.168.0.0/24"),
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("172.17.0.0/16"),
		netip.MustParsePrefix("10.8.0.0/24"),
	}
	for _, cand := range DefaultHotspotPool() {
		for _, bad := range avoid {
			if Overlaps(cand, bad) {
				t.Errorf("hotspot candidate %v collides with the commonly used %v", cand, bad)
			}
		}
	}
}

func TestGatewayAndPeerAddr(t *testing.T) {
	gw, err := GatewayAddr(netip.MustParsePrefix("10.83.51.0/24"))
	if err != nil || gw != netip.MustParseAddr("10.83.51.1") {
		t.Fatalf("gateway = %v, %v", gw, err)
	}
	peer, err := PeerAddr(netip.MustParsePrefix("198.18.51.0/30"))
	if err != nil || peer != netip.MustParseAddr("198.18.51.2") {
		t.Fatalf("peer = %v, %v", peer, err)
	}
	if _, err := GatewayAddr(netip.MustParsePrefix("10.0.0.1/32")); err == nil {
		t.Error("a /32 has no room for a gateway address and must be refused")
	}
}

// The chosen tunnel subnet must not collide with the chosen hotspot subnet
// either, which is why the hotspot choice is passed into the tunnel choice.
func TestPlan_TunnelAndHotspotSubnetsDoNotCollide(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	if Overlaps(p.TunSubnet, p.HotspotSubnet) {
		t.Errorf("tunnel %v overlaps hotspot %v", p.TunSubnet, p.HotspotSubnet)
	}
	if p.HotspotSubnet != netip.MustParsePrefix("10.83.51.0/24") {
		t.Errorf("hotspot subnet = %v", p.HotspotSubnet)
	}
	if p.TunSubnet != netip.MustParsePrefix("198.18.51.0/30") {
		t.Errorf("tunnel subnet = %v", p.TunSubnet)
	}
	if p.TunAddr != netip.MustParseAddr("198.18.51.1") {
		t.Errorf("tunnel address = %v", p.TunAddr)
	}
}
