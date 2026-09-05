// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// The fixtures below are the shapes the tools print on macOS 26.6, with every
// address from the documentation ranges (RFC 5737) and every MAC from the
// locally administered range, per the repository's privacy rule.

const darwinIfconfig = `lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> mtu 16384
	options=1203<RXCSUM,TXCSUM,TXSTATUS,SW_TIMESTAMP>
	inet 127.0.0.1 netmask 0xff000000
	inet6 ::1 prefixlen 128
	inet6 fe80::1%lo0 prefixlen 64 scopeid 0x1
en7: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
	ether 02:00:5e:00:00:07
	inet6 fe80::1c2a:3b4c:5d6e:7f80%en7 prefixlen 64 secured scopeid 0xb
	inet 198.51.100.23 netmask 0xffffff00 broadcast 198.51.100.255
	media: autoselect (1000baseT <full-duplex>)
	status: active
en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
	ether 02:00:5e:00:00:01
	media: autoselect
	status: inactive
ap1: flags=8802<BROADCAST,SIMPLEX,MULTICAST> mtu 1500
	ether 02:00:5e:00:00:02
	media: autoselect (none)
utun100: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST> mtu 1500
	inet 169.254.10.2 --> 169.254.10.1 netmask 0xfffffffc
`

const darwinRouteGet = `   route to: default
destination: default
       mask: default
    gateway: 198.51.100.1
  interface: en7
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
 recvpipe  sendpipe  ssthresh  rtt,msec    rttvar  hopcount      mtu     expire
       0         0         0         0         0         0      1500         0
`

const darwinHardwarePorts = `
Hardware Port: USB 10/100/1000 LAN
Device: en7
Ethernet Address: 02:00:5e:00:00:07

Hardware Port: Wi-Fi
Device: en0
Ethernet Address: 02:00:5e:00:00:01

Hardware Port: Thunderbolt Bridge
Device: bridge0
Ethernet Address: 02:00:5e:00:00:03

VLAN Configurations
===================
`

func darwinRunner(t *testing.T, associated bool) *RecordingRunner {
	t.Helper()
	air := "You are not associated with an AirPort network.\n"
	if associated {
		air = "Current Wi-Fi Network: house\n"
	}
	r := &RecordingRunner{Platform: PlatformDarwin, Responses: map[string]Result{}}
	r.Responses[RunnerKey(Command{Path: BinIfconfig, Args: []string{"-a"}})] = Result{Stdout: darwinIfconfig}
	r.Responses[RunnerKey(Command{Path: BinRoute, Args: []string{"-n", "get", "default"}})] = Result{Stdout: darwinRouteGet}
	r.Responses[RunnerKey(Command{Path: BinNetworksetup, Args: []string{"-listallhardwareports"}})] = Result{Stdout: darwinHardwarePorts}
	r.Responses[RunnerKey(Command{Path: BinNetworksetup, Args: []string{"-getairportnetwork", "en0"}})] = Result{Stdout: air}
	r.Responses[RunnerKey(Command{Path: BinSysctl, Args: []string{"-e", "--", "net.inet.ip.forwarding", "net.inet6.ip6.forwarding"}})] =
		Result{Stdout: "net.inet.ip.forwarding=0\nnet.inet6.ip6.forwarding=0\n"}
	r.Responses[RunnerKey(Command{Path: BinPfctl, Args: []string{"-s", "info"}})] = Result{Stdout: "Status: Disabled\n"}
	return r
}

func TestDarwinDetect_ReadsTheMacIntoTheSharedFacts(t *testing.T) {
	r := darwinRunner(t, false)
	f, err := BackendFor(PlatformDarwin).Detect(context.Background(), r, darwinSysctlKnobs())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	def, ok := f.PrimaryDefault()
	if !ok || def.Dev != "en7" || def.Gateway != netip.MustParseAddr("198.51.100.1") {
		t.Fatalf("default route = %+v, %v", def, ok)
	}
	l, ok := f.LinkByName("en7")
	if !ok || l.Bus != "usb" || len(l.Prefixes) != 2 {
		t.Fatalf("en7 = %+v, %v", l, ok)
	}
	w, ok := f.WirelessByName("en0")
	if !ok || w.Phy != "radio-en0" || !w.LinkKnown || w.Associated {
		t.Fatalf("en0 wireless = %+v, %v", w, ok)
	}
	phy, ok := f.PhyByName("radio-en0")
	if !ok || !phy.SupportsAP() {
		t.Fatalf("radio-en0 = %+v, %v", phy, ok)
	}
	if shared, _ := phy.APWithStation(); shared {
		t.Fatal("the Mac's radio must not claim it can host and join at once")
	}
	if f.Sysctl["net.inet.ip.forwarding"] != "0" || f.Sysctl[darwinPfStatusKnob] != "0" {
		t.Fatalf("knobs = %v", f.Sysctl)
	}
	for _, c := range r.Commands() {
		if err := ValidateCommandOn(PlatformDarwin, c); err != nil {
			t.Errorf("detection ran %s, which the macOS runner refuses: %v", c, err)
		}
		if c.Path == BinSysctl && c.Args[0] == "-w" {
			t.Errorf("detection wrote a knob: %s", c)
		}
	}
}

func TestDarwinPlan_EthernetUplinkHostsOnTheBuiltInRadio(t *testing.T) {
	r := darwinRunner(t, false)
	be := BackendFor(PlatformDarwin)
	f, err := be.Detect(context.Background(), r, be.BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}
	o := DefaultOptions()
	o.Platform = PlatformDarwin
	o.TunName = "utun100"
	p, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if p.Platform != PlatformDarwin || p.Uplink != "en7" || p.Hotspot != "en0" || p.Tun != "utun100" {
		t.Fatalf("plan = uplink %s hotspot %s tun %s platform %s", p.Uplink, p.Hotspot, p.Tun, p.Platform)
	}

	pre := p.PreEngineSteps(f.Sysctl)
	if len(pre) == 0 || pre[0].Do.Path != BinPfctl || pre[0].Do.Args[0] != "-e" {
		t.Fatalf("with pf disabled the first step must enable it, got %v", pre)
	}
	if pre[1].Op != OpPf || pre[1].Do.Args[1] != darwinAnchor || !strings.Contains(pre[1].Do.Stdin, "route-to (utun100 169.254.10.1)") {
		t.Fatalf("second step must load the anchor with the steering rule, got %s", pre[1].Do)
	}
	var sawForward, sawPin bool
	for _, s := range pre {
		if err := ValidateCommandOn(PlatformDarwin, s.Do); err != nil {
			t.Errorf("%s: %v", s.Do, err)
		}
		if !s.Undo.IsZero() {
			if err := ValidateCommandOn(PlatformDarwin, s.Undo); err != nil {
				t.Errorf("%s: %v", s.Undo, err)
			}
		}
		if s.Do.Path == BinSysctl && s.Do.Args[1] == "net.inet.ip.forwarding=1" {
			sawForward = true
			if s.Undo.Args[1] != "net.inet.ip.forwarding=0" {
				t.Errorf("forwarding inverse = %v", s.Undo.Args)
			}
		}
		if s.Do.Path == BinRoute && strings.Join(s.Do.Args, " ") == "-n add -host 203.0.113.10 198.51.100.1" {
			sawPin = true
			if strings.Join(s.Undo.Args, " ") != "-n delete -host 203.0.113.10" {
				t.Errorf("pinned route inverse = %v", s.Undo.Args)
			}
		}
	}
	if !sawForward || !sawPin {
		t.Fatalf("forwarding %t, pinned route %t in %v", sawForward, sawPin, pre)
	}
	if post := p.PostEngineSteps(f.Sysctl); len(post) != 0 {
		t.Fatalf("policy steering needs no post-engine route, got %v", post)
	}
	if strings.Contains(p.CutStep().Do.Stdin, "route-to") {
		t.Fatal("the cut ruleset must withhold the steering rule")
	}
	if !strings.Contains(p.RestoreStep().Do.Stdin, "route-to") {
		t.Fatal("the restore ruleset must carry the steering rule")
	}
	if strings.Contains(p.RestoreStep().Do.Stdin, "block drop out") {
		t.Fatal("no outbound block on the uplink: pf translates before it filters, so such a rule enforces nothing")
	}
}

func TestDarwinPlan_RejectsUSBWiFiAsTheHotspot(t *testing.T) {
	r := darwinRunner(t, false)
	r.Responses[RunnerKey(Command{Path: BinNetworksetup, Args: []string{"-listallhardwareports"}})] = Result{Stdout: darwinHardwarePorts + `
Hardware Port: USB Wi-Fi
Device: en2
`}
	be := BackendFor(PlatformDarwin)
	f, err := be.Detect(context.Background(), r, be.BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}
	phy, ok := f.PhyByName("radio-en2")
	if !ok || phy.SupportsAP() {
		t.Fatalf("USB Wi-Fi must be visible but not AP-capable: %+v, %v", phy, ok)
	}
	o := DefaultOptions()
	o.Platform = PlatformDarwin
	o.TunName = "utun100"
	o.HotspotOverride = "en2"
	if _, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o); err == nil {
		t.Fatal("Ethernet to USB Wi-Fi must be refused on macOS")
	}
}

func TestDarwinPlan_WiFiUplinkCannotFallBackToUSBWiFi(t *testing.T) {
	r := darwinRunner(t, true)
	r.Responses[RunnerKey(Command{Path: BinRoute, Args: []string{"-n", "get", "default"}})] =
		Result{Stdout: strings.Replace(darwinRouteGet, "interface: en7", "interface: en0", 1)}
	r.Responses[RunnerKey(Command{Path: BinNetworksetup, Args: []string{"-listallhardwareports"}})] = Result{Stdout: darwinHardwarePorts + `
Hardware Port: USB Wi-Fi
Device: en2
`}
	be := BackendFor(PlatformDarwin)
	f, err := be.Detect(context.Background(), r, be.BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}
	o := DefaultOptions()
	o.Platform = PlatformDarwin
	o.TunName = "utun100"
	if _, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o); err == nil {
		t.Fatal("Wi-Fi to USB Wi-Fi must be refused when USB Wi-Fi cannot host AP")
	}
}

func TestDarwinPlan_SplitDefaultInstallsTheTwoHalves(t *testing.T) {
	r := darwinRunner(t, false)
	be := BackendFor(PlatformDarwin)
	f, _ := be.Detect(context.Background(), r, be.BaseSysctlKnobs())
	o := DefaultOptions()
	o.Platform = PlatformDarwin
	o.TunName = "utun100"
	o.Strategy = StrategySplitDefault
	p, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o)
	if err != nil {
		t.Fatal(err)
	}
	post := p.PostEngineSteps(f.Sysctl)
	if len(post) != 2 || post[0].Do.Args[3] != "0.0.0.0/1" || post[1].Do.Args[5] != "utun100" {
		t.Fatalf("post = %v", post)
	}
}

func TestDarwinPlan_RefusesWhenTheRadioIsTheUplink(t *testing.T) {
	r := darwinRunner(t, true)
	// The default route now leaves by the radio.
	r.Responses[RunnerKey(Command{Path: BinRoute, Args: []string{"-n", "get", "default"}})] =
		Result{Stdout: strings.Replace(darwinRouteGet, "interface: en7", "interface: en0", 1)}
	be := BackendFor(PlatformDarwin)
	f, _ := be.Detect(context.Background(), r, be.BaseSysctlKnobs())
	o := DefaultOptions()
	o.Platform = PlatformDarwin
	o.TunName = "utun100"
	if _, err := PlanNetwork(f, []netip.Addr{netip.MustParseAddr("203.0.113.10")}, o); err == nil {
		t.Fatal("a Mac whose only internet is its Wi-Fi cannot also host the hotspot on it")
	}
}

func TestDarwinReadbacks_UseTheBridgeNotTheRadio(t *testing.T) {
	p := &Plan{Platform: PlatformDarwin, Hotspot: "en0",
		HotspotSubnet: netip.MustParsePrefix("10.83.51.0/24"), HotspotGateway: netip.MustParseAddr("10.83.51.1")}
	r := &RecordingRunner{Platform: PlatformDarwin, Responses: map[string]Result{}, Errors: map[string]error{}}
	key := RunnerKey(Command{Path: BinIfconfig, Args: []string{"bridge100"}})

	// Absent: released, not an access point.
	r.Errors[key] = errors.New("netcfg: ifconfig exited 1: ifconfig: interface bridge100 does not exist")
	r.Responses[key] = Result{Stderr: "ifconfig: interface bridge100 does not exist", ExitCode: 1}
	if err := AssertHotspotInterfaceReleased(context.Background(), r, p); err != nil {
		t.Fatalf("absent bridge must count as released: %v", err)
	}
	if err := AssertHotspotIsAccessPoint(context.Background(), r, p, "x"); err == nil {
		t.Fatal("absent bridge is not an access point")
	}

	// Carrying somebody else's network: not released.
	delete(r.Errors, key)
	r.Responses[key] = Result{Stdout: "bridge100: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n\tinet 192.168.2.1 netmask 0xffffff00 broadcast 192.168.2.255\n"}
	if err := AssertHotspotInterfaceReleased(context.Background(), r, p); err == nil {
		t.Fatal("a bridge on another subnet is not released")
	}

	// Carrying ours: an access point.
	r.Responses[key] = Result{Stdout: "bridge100: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n\tinet 10.83.51.1 netmask 0xffffff00 broadcast 10.83.51.255\n"}
	if err := AssertHotspotIsAccessPoint(context.Background(), r, p, "x"); err != nil {
		t.Fatalf("bridge with our gateway must read as the access point: %v", err)
	}
	if err := AssertHotspotInterfaceReleased(context.Background(), r, p); err != nil {
		t.Fatalf("our own subnet on the bridge is not a foreign network: %v", err)
	}
}

func TestDarwinParsers(t *testing.T) {
	links := ParseIfconfig(darwinIfconfig)
	if len(links) != 5 || links[1].Name != "en7" || links[1].State != "UP" || links[3].State != "DOWN" {
		t.Fatalf("links = %+v", links)
	}
	if links[1].Prefixes[1] != netip.MustParsePrefix("198.51.100.23/24") {
		t.Fatalf("en7 prefixes = %v", links[1].Prefixes)
	}
	if links[4].Prefixes[0] != netip.MustParsePrefix("169.254.10.2/30") {
		t.Fatalf("utun prefixes = %v", links[4].Prefixes)
	}
	if _, ok := maskBits("0xffff0f00"); ok {
		t.Fatal("a non-contiguous mask must be refused")
	}

	dr, ok := ParseRouteGet(darwinRouteGet)
	if !ok || dr.Dev != "en7" || dr.Gateway.String() != "198.51.100.1" {
		t.Fatalf("route = %+v %v", dr, ok)
	}
	if _, ok := ParseRouteGet("route: writing to routing socket: not in table\n"); ok {
		t.Fatal("no default route must parse as none")
	}
	v6, ok := ParseRouteGet("   route to: default\n    gateway: fe80::1%en7\n  interface: en7\n")
	if !ok || v6.Gateway.String() != "fe80::1" {
		t.Fatalf("v6 gateway = %+v", v6)
	}

	ports := ParseHardwarePorts(darwinHardwarePorts)
	if len(ports) != 3 || !ports[1].IsWiFi() || ports[1].Device != "en0" || ports[0].IsWiFi() {
		t.Fatalf("ports = %+v", ports)
	}

	if ssid, assoc, known := ParseAirportNetwork("Current Wi-Fi Network: house\n"); !known || !assoc || ssid != "house" {
		t.Fatal("associated line misread")
	}
	if _, assoc, known := ParseAirportNetwork("You are not associated with an AirPort network.\n"); !known || assoc {
		t.Fatal("not-associated line misread")
	}
	if _, _, known := ParseAirportNetwork("Wi-Fi power is currently off.\n"); known {
		t.Fatal("a powered-off radio must not read as known")
	}

	if on, ok := ParsePfStatus("Status: Enabled for 0 days 00:03:12           Debug: Urgent\n"); !ok || !on {
		t.Fatal("pf enabled misread")
	}
	if on, ok := ParsePfStatus("No ALTQ support in kernel\nStatus: Disabled\n"); !ok || on {
		t.Fatal("pf disabled misread")
	}
	if cc, ok := ParseRegDomain("global\ncountry GB: DFS-ETSI\n"); !ok || cc != "GB" {
		t.Fatal("reg domain misread")
	}
}

func TestBackendFor_UnknownPlatformRefuses(t *testing.T) {
	be := BackendFor(Platform("plan9"))
	if _, err := be.Detect(context.Background(), &RecordingRunner{}, nil); err == nil {
		t.Fatal("an unknown platform must refuse detection")
	}
	if err := ValidateCommandOn(Platform("plan9"), Command{Path: BinIP, Args: []string{"x"}}); err == nil {
		t.Fatal("an unknown platform allows no binary")
	}
	if err := ValidateCommandOn(PlatformDarwin, Command{Path: BinNft}); err == nil {
		t.Fatal("nft is not a macOS binary")
	}
}
