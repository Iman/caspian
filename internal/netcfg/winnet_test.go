// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// A Windows 11 laptop: Ethernet uplink, one Wi-Fi adapter, the hotspot's
// virtual adapter not yet created. Documentation addresses throughout.
const winInventory = `{"adapters":[
 {"alias":"Ethernet","index":5,"type":"ethernet","up":true,"usb":true,"prefixes":["198.51.100.23/24","fe80::1/64"],"forwarding":false},
 {"alias":"Wi-Fi","index":7,"type":"wifi","up":true,"prefixes":[],"forwarding":false},
 {"alias":"Loopback Pseudo-Interface 1","index":1,"type":"loopback","up":true,"prefixes":["127.0.0.1/8"],"forwarding":false}
],"defaults":[{"alias":"Ethernet","gateway":"198.51.100.1","metric":25,"family":4,"up":true}]}`

const winInventoryHotspotUp = `{"adapters":[
 {"alias":"Ethernet","index":5,"type":"ethernet","up":true,"prefixes":["198.51.100.23/24"],"forwarding":false},
 {"alias":"Wi-Fi","index":7,"type":"wifi","up":true,"prefixes":[],"forwarding":false},
 {"alias":"Local Area Connection* 1","index":11,"type":"wifi","wifiDirect":true,"up":false,"prefixes":[],"forwarding":false},
 {"alias":"Local Area Connection* 2","index":12,"type":"wifi","wifiDirect":true,"up":true,"prefixes":["192.168.137.1/24"],"forwarding":true}
],"defaults":[{"alias":"Ethernet","gateway":"198.51.100.1","metric":25,"family":4,"up":true}]}`

func winRunner(inv string) *RecordingRunner {
	r := &RecordingRunner{Platform: PlatformWindows, Responses: map[string]Result{}}
	r.Responses[RunnerKey(Command{Path: BinIPHelper, Args: []string{"adapters"}})] = Result{Stdout: inv}
	return r
}

func winPlan(t *testing.T, inv string) (*Plan, Facts) {
	t.Helper()
	be := BackendFor(PlatformWindows)
	f, err := be.Detect(context.Background(), winRunner(inv), be.BaseSysctlKnobs())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	o := DefaultOptions()
	o.Platform = PlatformWindows
	o.HotspotSubnet = windowsBackend{}.FixedHotspotSubnet()
	p, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return p, f
}

func TestWindowsDetect_ReadsTheInventoryIntoTheSharedFacts(t *testing.T) {
	be := BackendFor(PlatformWindows)
	f, err := be.Detect(context.Background(), winRunner(winInventory), nil)
	if err != nil {
		t.Fatal(err)
	}
	def, ok := f.PrimaryDefault()
	if !ok || def.Dev != "Ethernet" || def.Gateway.String() != "198.51.100.1" || def.Metric != 25 {
		t.Fatalf("default = %+v %v", def, ok)
	}
	if l, _ := f.LinkByName("Ethernet"); l.Bus != "usb" || len(l.Prefixes) != 2 || l.State != "UP" {
		t.Fatalf("Ethernet = %+v", l)
	}
	w, ok := f.WirelessByName("Wi-Fi")
	if !ok || w.Phy != "radio-Wi-Fi" {
		t.Fatalf("Wi-Fi = %+v %v", w, ok)
	}
	phy, _ := f.PhyByName(w.Phy)
	if shared, _ := phy.APWithStation(); !shared {
		t.Fatal("Windows hosts the hotspot beside a station link; the radio must say so")
	}
	if f.Sysctl[windowsForwardingKnob("Ethernet")] != "0" {
		t.Fatalf("knobs = %v", f.Sysctl)
	}
	if _, err := ParseWindowsInventory(`{"adapters":[{"alias":"x","surprise":1}]}`); err == nil {
		t.Fatal("an unknown field is a runner and backend that disagree, and must be refused")
	}
}

func TestWindowsPlan_AdapterAliasesWithSpaces(t *testing.T) {
	inv := strings.ReplaceAll(winInventory, `"Ethernet"`, `"Ethernet 2"`)
	inv = strings.ReplaceAll(inv, `"Wi-Fi"`, `"Wi-Fi 3"`)
	p, facts := winPlan(t, inv)
	if p.Hotspot != "Wi-Fi 3" || p.Uplink != "Ethernet 2" {
		t.Fatalf("automatic adapter selection: %+v", p)
	}
	o := DefaultOptions()
	o.Platform = PlatformWindows
	o.HotspotSubnet = WindowsHotspotSubnet
	o.UplinkOverride, o.HotspotOverride = "Ethernet 2", "Wi-Fi 3"
	p, err := PlanNetwork(facts, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o)
	if err != nil || p.Hotspot != "Wi-Fi 3" {
		t.Fatalf("explicit adapter selection: %v, %v", p, err)
	}
}

func TestWindowsInventory_RemovedAdaptersExcludedIdleRadiosKept(t *testing.T) {
	inv := WindowsInventory{Adapters: []WindowsAdapter{
		{Alias: "Wi-Fi", Index: 1, Type: "wifi"},
		{Alias: "Wi-Fi 3", Index: 27, Type: "wifi", Up: false},
	}}
	inv.retainPresentAdapters(map[int]bool{27: true})
	if len(inv.Adapters) != 1 || inv.Adapters[0].Alias != "Wi-Fi 3" {
		t.Fatalf("usable adapters: %+v", inv.Adapters)
	}
}

func TestWindowsPlan_TunnelsTheWholeHostFailClosed(t *testing.T) {
	p, f := winPlan(t, winInventory)
	if p.Hotspot != "Wi-Fi" || p.Uplink != "Ethernet" || p.HotspotSubnet != WindowsHotspotSubnet || p.HotspotGateway.String() != "192.168.137.1" {
		t.Fatalf("plan = hotspot %s uplink %s subnet %s gw %s", p.Hotspot, p.Uplink, p.HotspotSubnet, p.HotspotGateway)
	}
	pre := p.PreEngineSteps(f.Sysctl)
	want := []string{
		"wfp load",
		"iphlpapi route add 203.0.113.10/32 dev Ethernet via 198.51.100.1 metric 1",
	}
	if len(pre) != len(want) {
		t.Fatalf("pre-engine steps = %v", pre)
	}
	for i, s := range pre {
		if got := CommandLine(s.Do); got != want[i] {
			t.Errorf("step %d = %q, want %q", i, got, want[i])
		}
		if err := ValidateCommandOn(PlatformWindows, s.Do); err != nil {
			t.Errorf("%s: %v", s.Do, err)
		}
		if s.Undo.IsZero() && s.Op != OpSysctl {
			t.Errorf("step %d has no inverse", i)
		}
	}
	if !strings.Contains(pre[0].Do.Stdin, `"forward":"cut"`) {
		t.Fatalf("clients must be blocked before the tunnel exists: %s", pre[0].Do.Stdin)
	}

	post := p.PostEngineSteps(f.Sysctl)
	wantPost := []string{
		"wfp load",
		"iphlpapi iface set xray0 forwarding on",
		"iphlpapi addr add " + p.TunAddr.String() + "/30 dev xray0",
		"iphlpapi iface set xray0 metric 0",
		"iphlpapi route add 0.0.0.0/0 dev xray0 metric 0",
	}
	if len(post) != len(wantPost) {
		t.Fatalf("post-engine steps = %v", post)
	}
	if !strings.Contains(post[0].Do.Stdin, `"forward":"normal"`) || post[0].Undo.Stdin != pre[0].Do.Stdin {
		t.Fatal("post-engine permit must roll back to the initial block")
	}
	for i, s := range post {
		if got := CommandLine(s.Do); got != wantPost[i] {
			t.Errorf("post step %d = %q, want %q", i, got, wantPost[i])
		}
	}
	if !strings.Contains(p.CutStep().Do.Stdin, `"forward":"cut"`) || RunnerKey(p.CutStep().Do) == RunnerKey(p.RestoreStep().Do) {
		t.Fatal("cut and restore must load different filter sets and be told apart by RunnerKey")
	}
}

func TestWindowsReadbacks_UseTheHotspotAdapter(t *testing.T) {
	p, _ := winPlan(t, winInventory)
	be := BackendFor(PlatformWindows)
	// Not created yet: released, not an access point.
	r := winRunner(winInventory)
	if err := be.AssertHotspotInterfaceReleased(context.Background(), r, p); err != nil {
		t.Fatalf("absent adapter is released: %v", err)
	}
	if err := be.AssertHotspotIsAccessPoint(context.Background(), r, p, ""); err == nil {
		t.Fatal("absent adapter is not an access point")
	}
	// Up with the ICS gateway: an access point, and released (our subnet).
	r = winRunner(winInventoryHotspotUp)
	if err := be.AssertHotspotIsAccessPoint(context.Background(), r, p, ""); err != nil {
		t.Fatalf("hotspot up: %v", err)
	}
	unlabelled := strings.Replace(winInventoryHotspotUp, `"type":"wifi","wifiDirect":true,"up":true`, `"type":"ethernet","up":true`, 1)
	if err := be.AssertHotspotIsAccessPoint(context.Background(), winRunner(unlabelled), p, ""); err != nil {
		t.Fatalf("hotspot with an unlabelled virtual adapter: %v", err)
	}
	if err := be.AssertHotspotInterfaceReleased(context.Background(), r, p); err != nil {
		t.Fatalf("our own subnet is not foreign: %v", err)
	}
	// Somebody else's network on it.
	foreign := strings.Replace(winInventoryHotspotUp, "192.168.137.1/24", "192.168.2.1/24", 1)
	if err := be.AssertHotspotInterfaceReleased(context.Background(), winRunner(foreign), p); err == nil {
		t.Fatal("a hotspot adapter on another subnet is not released")
	}
}

func TestWindowsMetric(t *testing.T) {
	if n, ok := windowsMetric("25"); !ok || n != 25 {
		t.Fatal("metric")
	}
	if _, ok := windowsMetric("auto"); ok {
		t.Fatal("auto is not a number")
	}
}
