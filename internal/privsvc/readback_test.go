// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// These tests are the ones the target earned on 2026-08-30. The service logged
// "running config=dc7370fd uplink=eth0 hotspot=wlan0 tunnel=xray0 channel=1"
// while wlan0 was type managed, ssid HomeNet, channel 10, a phone in the room
// could not see the network, and dnsmasq was answering DHCP on the house LAN.
// Every command this package ran had returned success.

// aKernelThatIgnoredTheRelease is the box as it actually was: the release
// commands returned success and the kernel did not change.
//
// It is modelled by pinning the answer to "iw dev", which is what the box
// itself did: thirty seconds after the takeover reported success, that command
// still described a station on the house network.
func aKernelThatIgnoredTheRelease(w *world) {
	w.runner.SetOutput("iw dev", fixtureIWDev)
	w.runner.SetOutput("ip -br addr show dev wlan0",
		"wlan0            UP             192.168.1.57/24 10.83.51.1/24 \n")
}

// hostapdDoes replaces what the fake hostapd does to the modelled machine, so
// a test can describe an access point process that came up and left the radio
// in some other state. The process still starts and still writes its pid file:
// that is the point, because a live process is the evidence that was mistaken
// for a working hotspot.
func hostapdDoes(w *world, effect func(m *machine)) {
	base := hotspot.DefaultResponder
	bin := w.cfg.HotspotPaths.HostapdBinary
	w.sys.Responder = func(rec *hotspot.Recorder, name string, args []string) (hotspot.Result, error) {
		res, err := base(rec, name, args)
		if name == bin && err == nil && effect != nil {
			effect(w.runner)
		}
		return res, err
	}
}

// TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree.
//
// This is the whole event in one test. dnsmasq bound to wlan0 while wlan0 was
// still joined to the house network and still carried its station address, and
// answered a real device's lease renewal with DHCPNAK on a LAN this appliance
// does not own.
func TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	aKernelThatIgnoredTheRelease(w)

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("the start reported success on a box whose hotspot interface was still on another network"+
			"\ntimeline:%s", w.tl)
	}
	if !errors.Is(err, netcfg.ErrHotspotNotReleased) {
		t.Fatalf("err = %v, want netcfg.ErrHotspotNotReleased", err)
	}

	// Neither server may have been started. This is the assertion that would
	// have caught the DHCPNAK.
	for _, bin := range []string{w.cfg.HotspotPaths.DnsmasqBinary, w.cfg.HotspotPaths.HostapdBinary} {
		if n := w.sys.CountCalls(bin); n != 0 {
			t.Errorf("%s was started %d times on an interface that was never proved free\ntrail:\n%s",
				bin, n, w.sys.TrailString())
		}
	}
	if w.svc.isRunning() {
		t.Error("the service reports itself running")
	}
	if strings.Contains(w.logs.String(), "msg=running") {
		t.Errorf("the service logged that it is running:\n%s", w.logs.String())
	}
}

// TestABoxThatCannotProveTheInterfaceIsItsOwnStillGivesTheWiFiBack.
//
// A user whose Pi cannot host a hotspot has lost a feature. A user whose Pi
// cannot host a hotspot AND never rejoins their WiFi again has lost the
// machine. Every step of the release carries its inverse, and the failure path
// has to replay them.
func TestABoxThatCannotProveTheInterfaceIsItsOwnStillGivesTheWiFiBack(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	aKernelThatIgnoredTheRelease(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err == nil {
		t.Fatal("the start reported success")
	}

	ran := map[string]int{}
	for _, c := range w.runner.Commands() {
		ran[netcfg.RunnerKey(c)]++
	}
	// The three inverses that give the user their network back, and the
	// hotspot address that has to come off the interface either way.
	for _, undo := range []string{
		"nmcli device set wlan0 managed yes",
		"iw dev wlan0 set type managed",
		"ip address add 192.168.1.57/24 dev wlan0",
		"ip address del 10.83.51.1/24 dev wlan0",
	} {
		if ran[undo] == 0 {
			t.Errorf("the inverse %q never ran, so the box keeps a change it made\ntimeline:%s", undo, w.tl)
		}
	}
	if _, statErr := os.Stat(w.cfg.JournalPath); !errors.Is(statErr, os.ErrNotExist) {
		left, _ := netcfg.LoadJournal(w.cfg.JournalPath)
		t.Errorf("the journal survived with %d entries, so the box still carries changes", len(left))
	}
}

// TestAReleaseThatCouldNotBeProvedSaysSomethingTrueAndUseful.
//
// The fault word and the sentence are two different things and both have to be
// right. There is no word in the panel's closed vocabulary for "the adapter is
// busy holding somebody's network", so the fault is unclassified on purpose,
// and the sentence that IS true has to reach the advanced view.
func TestAReleaseThatCouldNotBeProvedSaysSomethingTrueAndUseful(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	aKernelThatIgnoredTheRelease(w)

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatal("the start reported success")
	}
	if got := panel.FaultOf(err); got != panel.FaultHotspotInterfaceBusy {
		t.Errorf("fault = %q, want %q: FaultHotspotFailed tells the user to restart, which cannot help, "+
			"and FaultNoAPAdapter says the adapter cannot do it, which is false",
			got, panel.FaultHotspotInterfaceBusy)
	}

	shown := advancedView(t, w)
	for _, where := range []struct{ name, text string }{
		{"the advanced view", shown},
		{"the log", w.logs.String()},
	} {
		if !strings.Contains(where.text, "wlan0") {
			t.Errorf("%s does not name the interface:\n%s", where.name, where.text)
		}
		// internal/netcfg's refusal names the network the interface is still
		// joined to, which is the fact that tells the user what to go and do.
		if !strings.Contains(where.text, "HomeNet") {
			t.Errorf("%s does not name the network the adapter is still on:\n%s", where.name, where.text)
		}
		if !strings.Contains(where.text, "not allowed to serve") &&
			!strings.Contains(where.text, "could not be proved free") {
			t.Errorf("%s does not say what was refused:\n%s", where.name, where.text)
		}
	}
}

// TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne.
//
// hostapd starts, writes a pid file, stays alive, and its control interface
// reports state=ENABLED. On the target all of that was true and the radio was
// a station on the house network. A process being alive is the same class of
// evidence as an exit code.
func TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	// The release works. hostapd then comes up and the radio ends up a
	// station on somebody's network anyway.
	hostapdDoes(w, func(m *machine) { m.rejoin("wlan0", "HomeNet", 10, "") })

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("the service reported success with a live hostapd and no access point\ntimeline:%s", w.tl)
	}
	if !errors.Is(err, netcfg.ErrNotAccessPoint) {
		t.Fatalf("err = %v, want netcfg.ErrNotAccessPoint", err)
	}
	if got := panel.FaultOf(err); got != panel.FaultHotspotFailed {
		t.Errorf("fault = %q, want %q", got, panel.FaultHotspotFailed)
	}
	if w.svc.isRunning() {
		t.Error("the service reports itself running")
	}
	if strings.Contains(w.logs.String(), "msg=running") {
		t.Errorf("the service logged that it is running:\n%s", w.logs.String())
	}
	// hostapd really did start and really did stay alive, or this test is
	// proving something else.
	if w.sys.CountCalls(w.cfg.HotspotPaths.HostapdBinary) == 0 {
		t.Fatal("hostapd was never started, so this is not the failure this test is named for")
	}
	assertMachineRestored(t, w)
}

// TestAnAccessPointBroadcastingAnotherNameIsNotOurs.
//
// The interface is in access point mode and the name on the air is not the one
// the user was given, so nothing they were told to look for exists.
func TestAnAccessPointBroadcastingAnotherNameIsNotOurs(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	hostapdDoes(w, func(m *machine) { m.startAccessPoint("wlan0", "Somebody-Elses-Network", 1) })

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("the service reported success while broadcasting another name\ntimeline:%s", w.tl)
	}
	if !errors.Is(err, netcfg.ErrNotAccessPoint) {
		t.Fatalf("err = %v, want netcfg.ErrNotAccessPoint", err)
	}
	if w.svc.isRunning() {
		t.Error("the service reports itself running")
	}
}

// TestAnAccessPointBroadcastingNoNameAtAllIsNotOurs.
//
// This test was the opposite of itself for part of a day. It used to assert
// that an access point the kernel reported no name for was ACCEPTED, with the
// gap said out loud, because no capture in this tree established whether this
// kernel reports the name for an interface in AP mode, and refusing on an
// unknown would have risked refusing a working box.
//
// MEASURED on the target on 2026-08-30, kernel 6.18.34, brcmfmac, with a real
// access point running: "iw dev" printed "ssid Caspian-Probe" beside "type AP".
// The kernel does report it. So on this hardware an access point with no name
// is not one that could not be read; it is one that is not broadcasting, and
// broadcasting nothing while reporting itself up is the entire failure this
// readback exists to catch. The tolerance is gone and the assertion is
// inverted.
func TestAnAccessPointBroadcastingNoNameAtAllIsNotOurs(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	// hostapd starts, stays alive, and the interface ends in AP mode with no
	// name on the air. Its channel is the stale one the release left behind.
	hostapdDoes(w, func(m *machine) { m.startAccessPoint("wlan0", "", 10) })

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("the service reported success with an access point broadcasting nothing\ntimeline:%s", w.tl)
	}
	if !errors.Is(err, netcfg.ErrNotAccessPoint) {
		t.Fatalf("err = %v, want netcfg.ErrNotAccessPoint", err)
	}
	if got := panel.FaultOf(err); got != panel.FaultHotspotFailed {
		t.Errorf("fault = %q, want %q", got, panel.FaultHotspotFailed)
	}
	if w.svc.isRunning() {
		t.Error("the service reports itself running")
	}
	if strings.Contains(w.logs.String(), "msg=running") {
		t.Errorf("the service logged that it is running:\n%s", w.logs.String())
	}
	// hostapd really did start and stay alive, or this is not the failure the
	// test is named for: a live process broadcasting nothing.
	if w.sys.CountCalls(w.cfg.HotspotPaths.HostapdBinary) == 0 {
		t.Fatal("hostapd was never started")
	}
	assertMachineRestored(t, w)
}

// TestARepeatPressDoesNotStartAServerOnAnInterfaceSomethingElseTookBack.
//
// Start is reached a second time with the same request, which repairs the two
// daemons without touching the network configuration. That is the SECOND path
// in this package that binds a server to the hotspot interface, and the rule
// is about every path in, not the one that was checked.
func TestARepeatPressDoesNotStartAServerOnAnInterfaceSomethingElseTookBack(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	req := startRequest(t)

	if err := w.svc.Start(context.Background(), req); err != nil {
		t.Fatalf("the first start failed: %v\ntimeline:%s", err, w.tl)
	}
	before := w.sys.CountCalls(w.cfg.HotspotPaths.DnsmasqBinary)

	// Both daemons die. This is what makes the repeat press a repair rather
	// than a no-op: with them alive internal/hotspot starts nothing, and the
	// test would pass on a service that checks nothing.
	killDaemon(t, w, w.cfg.HotspotPaths.HostapdPID)
	killDaemon(t, w, w.cfg.HotspotPaths.DnsmasqPID)

	// Something outside this appliance takes the radio back and puts it on
	// another network, carrying that network's address.
	w.runner.rejoin("wlan0", "HomeNet", 10, "192.168.1.57/24")

	err := w.svc.Start(context.Background(), req)
	if err == nil {
		t.Fatalf("the repeat press reported success on an interface that is now somebody else's"+
			"\ntimeline:%s", w.tl)
	}
	if !errors.Is(err, netcfg.ErrHotspotNotReleased) {
		t.Fatalf("err = %v, want netcfg.ErrHotspotNotReleased", err)
	}
	if after := w.sys.CountCalls(w.cfg.HotspotPaths.DnsmasqBinary); after != before {
		t.Errorf("the DHCP and DNS server was started again (%d then %d) on an interface that is not ours",
			before, after)
	}

	// A refused repair applies nothing, so it undoes nothing: the box is
	// exactly as it was before the switch was pressed, with its journal still
	// on disk. What has to remain true is that switching off still gives the
	// user their network back, because that is the only move left to somebody
	// looking at a box that will not come up.
	if err := w.svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop after a refused repair: %v", err)
	}
	ran := map[string]int{}
	for _, c := range w.runner.Commands() {
		ran[netcfg.RunnerKey(c)]++
	}
	for _, undo := range []string{
		"nmcli device set wlan0 managed yes",
		"iw dev wlan0 set type managed",
	} {
		if ran[undo] == 0 {
			t.Errorf("switching off after a refused repair did not run %q\ntimeline:%s", undo, w.tl)
		}
	}
}

// TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease.
//
// The note the panel shows says wlan0 has to be released and stripped before
// anything binds to it. This is the assertion that the commands which make
// that true are actually run, and run in the right order. It exists because
// the note used to claim an effect no code produced, and a test on the wording
// alone is what let that stand.
func TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	// The sequence, in order, and every one of them before either server is
	// asked to serve on the interface.
	want := []string{
		"net: nmcli device set wlan0 managed no",
		"net: ip address del 192.168.1.57/24 dev wlan0",
		"net: ip link set dev wlan0 down",
		"net: iw dev wlan0 set type __ap",
	}
	last := -1
	for _, cmd := range want {
		at := w.tl.indexOf(cmd)
		if at < 0 {
			t.Fatalf("%q never ran, so the interface was not released\ntimeline:%s", cmd, w.tl)
		}
		if at < last {
			t.Fatalf("%q ran out of order\ntimeline:%s", cmd, w.tl)
		}
		last = at
	}
	for _, bin := range []string{w.cfg.HotspotPaths.HostapdBinary, w.cfg.HotspotPaths.DnsmasqBinary} {
		mustBefore(t, w.tl, want[len(want)-1], "hotspot: "+bin,
			"nothing may bind to the interface before it has been released")
	}
}

// TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter.
//
// Two readbacks, two moments. The first has to come after the steps it
// verifies and before either server starts. The second has to come after
// hostapd, because until hostapd runs there is no access point to read.
func TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	// The readback reads the interface's addresses, and that command is run
	// nowhere else in a start, so it marks the moment.
	readback := "net: ip -br addr show dev wlan0"
	if w.tl.indexOf(readback) < 0 {
		t.Fatalf("the hotspot interface was never read back\ntimeline:%s", w.tl)
	}
	mustBefore(t, w.tl, "net: iw dev wlan0 set type __ap", readback,
		"the readback has to come after the steps it verifies")
	mustBefore(t, w.tl, readback, "hotspot: "+w.cfg.HotspotPaths.HostapdBinary,
		"nothing may bind to the interface until the release has been read back")

	// And the access point is read back after hostapd started.
	apRead := w.tl.lastIndexOf("net: iw dev")
	apStart := w.tl.lastIndexOf("hotspot: " + w.cfg.HotspotPaths.HostapdBinary)
	if apRead < apStart {
		t.Fatalf("the last reading of the wireless interfaces is at %d and hostapd started at %d, "+
			"so the access point was never read back after it was started\ntimeline:%s", apRead, apStart, w.tl)
	}
}

// TestTheModelledMachineStartsAsTheFixtureDescribesIt.
//
// The double renders its own answers, so it can drift from the fixture bytes
// it was seeded with and make every test that reads it prove something about a
// machine nobody described. This is the round trip: seed from the fixture,
// render, parse, and compare the fields the production code reads.
func TestTheModelledMachineStartsAsTheFixtureDescribesIt(t *testing.T) {
	m := newRecordedMachine(fixtureIWList, fixtureIWRegGet)
	ctx := context.Background()

	res, err := m.Run(ctx, netcfg.Command{Path: netcfg.BinIw, Args: []string{"dev"}, Why: "test"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := netcfg.ParseIwDev(res.Stdout)
	if err != nil {
		t.Fatalf("the rendered iw dev does not parse: %v\n%s", err, res.Stdout)
	}
	want, err := netcfg.ParseIwDev(fixtureIWDev)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("rendered %d wireless interfaces, the fixture has %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Phy != want[i].Phy || got[i].Type != want[i].Type ||
			got[i].SSID != want[i].SSID || got[i].Channel != want[i].Channel {
			t.Errorf("interface %d rendered as %+v, the fixture says %+v", i, got[i], want[i])
		}
	}

	res, err = m.Run(ctx, netcfg.Command{Path: netcfg.BinIP, Args: []string{"-br", "addr"}, Why: "test"})
	if err != nil {
		t.Fatal(err)
	}
	gotLinks, err := netcfg.ParseBriefAddr(res.Stdout)
	if err != nil {
		t.Fatalf("the rendered ip -br addr does not parse: %v\n%s", err, res.Stdout)
	}
	wantLinks, err := netcfg.ParseBriefAddr(fixtureIPBriefAddr)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotLinks) != len(wantLinks) {
		t.Fatalf("rendered %d links, the fixture has %d", len(gotLinks), len(wantLinks))
	}
	for i := range wantLinks {
		if gotLinks[i].Name != wantLinks[i].Name || gotLinks[i].State != wantLinks[i].State ||
			len(gotLinks[i].Prefixes) != len(wantLinks[i].Prefixes) {
			t.Errorf("link %d rendered as %+v, the fixture says %+v", i, gotLinks[i], wantLinks[i])
		}
	}
}

// TestTheModelledMachineReportsAccessPointModeTheWayTheKernelDoes.
//
// "iw dev wlan0 set type __ap" is answered by the kernel reporting "type AP".
// MEASURED on the target, recorded in internal/netcfg/testdata/PROVENANCE.md.
// A double that echoed "__ap" back would fail every readback for a reason the
// real box does not have, and the readback would then be tuned to the double.
func TestTheModelledMachineReportsAccessPointModeTheWayTheKernelDoes(t *testing.T) {
	m := newRecordedMachine(fixtureIWList, fixtureIWRegGet)
	ctx := context.Background()

	if _, err := m.Run(ctx, netcfg.Command{
		Path: netcfg.BinIw, Args: []string{"dev", "wlan0", "set", "type", "__ap"}, Why: "test",
	}); err != nil {
		t.Fatal(err)
	}
	w, ok := m.wirelessState("wlan0")
	if !ok {
		t.Fatal("wlan0 is gone")
	}
	if w.Type != "AP" {
		t.Errorf("type = %q, want AP: the kernel reports AP, not the __ap it was asked for", w.Type)
	}
	// The NAME goes and the CHANNEL stays. Both halves are measured, and the
	// second half is the one that is easy to get wrong in a double: the driver
	// keeps reporting the channel the station link was on, so a double that
	// cleared it would model a machine this hardware never is.
	if w.SSID != "" {
		t.Errorf("the interface still reports ssid %q after becoming an access point", w.SSID)
	}
	if w.Channel != 10 {
		t.Errorf("channel = %d, want 10: a freed and typed interface keeps reporting the channel the "+
			"station link was on, measured on the target on 2026-08-30, kernel 6.18.34, brcmfmac", w.Channel)
	}
}

// TestAFreedInterfaceStillReportingItsOldChannelIsNotJoinedToAnything.
//
// MEASURED on the target on 2026-08-30 by the coordinator, kernel 6.18.34,
// brcmfmac. After the release sequence, with no hostapd running, "iw dev"
// prints for wlan0 "type AP" and "channel 10 (2457 MHz)", and no ssid line at
// all. The channel is the one the STATION link was on before the release and
// the driver keeps reporting it.
//
// The predicate internal/netcfg used to answer this with was
// WirelessIface.Associated, "an SSID is set OR a channel is reported", and it
// returned true for an interface this appliance had just freed and typed
// itself. Every start on this hardware would have refused here, with the
// refusal reading "still joined to \"\" on channel 10": joined to a network
// with no name. That predicate has been retired for IsAccessPoint, StationLink
// and InUse (internal/netcfg/facts.go), and this test is what holds the ground
// from THIS side of the boundary: the start sequence in this package is the
// thing that breaks if it comes back.
//
// It asks internal/netcfg directly rather than through a whole start, so that
// a failure names the predicate and not the eleven tests downstream of it.
func TestAFreedInterfaceStillReportingItsOldChannelIsNotJoinedToAnything(t *testing.T) {
	ctx := context.Background()
	plan := &netcfg.Plan{
		Hotspot:       "wlan0",
		HotspotSubnet: netip.MustParsePrefix("10.83.51.0/24"),
	}

	// The measured state: freed, typed, nothing serving, stale channel.
	freed := netcfg.NewRecordingRunner()
	freed.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\ttype AP\n"+
		"\t\tchannel 10 (2457 MHz), width: 20 MHz, center1: 2457 MHz\n")
	// The question netcfg now asks, and the answer this exact state gives.
	//
	// MEASURED on the box 2026-08-30: an access point interface with nothing
	// serving on it prints "Not connected." and exits 0, byte-identical to
	// what an unassociated station prints. The stale channel above is still
	// reported alongside it, which is the whole point of this test: the
	// channel says nothing, and this line is what the verdict comes from.
	freed.SetOutput("iw dev wlan0 link", "Not connected.\n")
	freed.SetOutput("ip -br addr show dev wlan0", "wlan0            UP             10.83.51.1/24 \n")
	if err := netcfg.AssertHotspotInterfaceReleased(ctx, freed, plan); err != nil {
		t.Fatalf("an interface this appliance has just released, stripped and typed itself was reported as "+
			"still somebody else's: %v\n\n"+
			"MEASURED on the target 2026-08-30, kernel 6.18.34, brcmfmac: a freed and typed interface reports "+
			"no ssid and keeps the channel the station link was on, so a predicate that reads a channel as "+
			"evidence of use refuses every correct release on this hardware.", err)
	}

	// And the check still bites, or the assertion above would pass on a
	// function that had been reduced to returning nil. Both states this has to
	// refuse, and both are measured shapes.
	// iwLink carries the measured shapes of "iw dev <if> link", which is now
	// where the association verdict comes from. The two refusals below are
	// refused for DIFFERENT reasons and it matters which: the station is
	// refused because it is joined to something, and the access point is
	// refused because it is already broadcasting a name. An access point is
	// never "connected" to anything, so its link output is the same "Not
	// connected." a free interface gives, and the SSID check is what has to
	// catch it. If that check were dropped, this case would pass.
	for _, tc := range []struct {
		name   string
		iwDev  string
		iwLink string
		brAddr string
	}{
		{
			name: "a station still joined to the house network",
			iwDev: "phy#0\n\tInterface wlan0\n\t\tssid HomeNet\n\t\ttype managed\n" +
				"\t\tchannel 10 (2457 MHz), width: 20 MHz\n",
			iwLink: "Connected to 02:00:5e:00:00:01 (on wlan0)\n\tSSID: HomeNet\n\tfreq: 2457.0\n",
			brAddr: "wlan0            UP             192.168.1.57/24 \n",
		},
		{
			name:   "an access point already broadcasting somebody else's network",
			iwDev:  "phy#0\n\tInterface wlan0\n\t\tssid Somebody-Else\n\t\ttype AP\n\t\tchannel 6 (2437 MHz)\n",
			iwLink: "Not connected.\n",
			brAddr: "wlan0            UP             10.83.51.1/24 \n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := netcfg.NewRecordingRunner()
			r.SetOutput("iw dev", tc.iwDev)
			r.SetOutput("iw dev wlan0 link", tc.iwLink)
			r.SetOutput("ip -br addr show dev wlan0", tc.brAddr)
			if err := netcfg.AssertHotspotInterfaceReleased(ctx, r, plan); !errors.Is(err, netcfg.ErrHotspotNotReleased) {
				t.Fatalf("err = %v, want ErrHotspotNotReleased: this interface is in use", err)
			}
		})
	}

	// The address half of the check bites too: a freed interface still
	// carrying an address from the network it was on is the half that let
	// dnsmasq reach the house LAN.
	carrying := netcfg.NewRecordingRunner()
	carrying.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\ttype AP\n\t\tchannel 10 (2457 MHz)\n")
	carrying.SetOutput("iw dev wlan0 link", "Not connected.\n")
	carrying.SetOutput("ip -br addr show dev wlan0", "wlan0            UP             10.83.51.1/24 192.168.1.57/24 \n")
	if err := netcfg.AssertHotspotInterfaceReleased(ctx, carrying, plan); !errors.Is(err, netcfg.ErrHotspotNotReleased) {
		t.Fatalf("err = %v, want ErrHotspotNotReleased: an address from another network is a path onto it", err)
	}
}

// killDaemon makes the process named in a pid file stop existing, which is
// what a hostapd or a dnsmasq that died leaves behind: a pid file naming
// nothing.
func killDaemon(t *testing.T, w *world, pidFile string) {
	t.Helper()
	raw, ok := w.sys.Files[pidFile]
	if !ok {
		t.Fatalf("no pid file at %s, so nothing was running to kill", pidFile)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("pid file %s holds %q", pidFile, raw)
	}
	w.sys.SetAlive(pid, false)
}

// TestStatusReadsTheRadioBackAndSpendsNoProcessDoingIt.
//
// The steady-state half of the readback. On the target on 2026-08-30 the panel
// showed connected while the radio was a station on the house network, and
// hostapd_cli was the only thing being asked. This poll now asks the kernel
// too, out of the detection it already holds.
//
// The second assertion is the one that keeps it affordable. It pins the cost
// at zero extra commands, so an implementation that reached for "iw dev" per
// poll would fail here rather than quietly tripling the steady-state process
// count of a box that polls every five seconds per open tab.
func TestStatusReadsTheRadioBackAndSpendsNoProcessDoingIt(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()

	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}
	st, err := w.svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Hotspot.Running {
		t.Fatalf("the hotspot is not reported running on a box that just started: %+v", st.Hotspot)
	}

	// A second poll inside the detection TTL must not run a single netcfg
	// command: the reading it needs was already taken.
	before := len(w.runner.Commands())
	if _, err := w.svc.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if spent := len(w.runner.Commands()) - before; spent != 0 {
		t.Errorf("a steady-state poll ran %d netcfg commands, want 0; it reads the interface out of the "+
			"detection that already ran, and the browser polls every 5000 ms per open tab", spent)
	}

	// Something takes the radio back and puts it on another network. hostapd
	// is still alive and its control socket still answers state=ENABLED, which
	// is the whole of what this used to believe.
	w.runner.rejoin("wlan0", "HomeNet", 10, "")
	if _, err := w.svc.Detect(ctx); err != nil {
		t.Fatal(err)
	}

	st, err = w.svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Hotspot.Running {
		t.Errorf("the panel reports the hotspot running while the adapter is a station on another network: %+v",
			st.Hotspot)
	}
	if st.Hotspot.Fault == panel.FaultNone {
		t.Error("no fault is reported, so the panel shows a hotspot that is down and no reason for it")
	}
	if st.Connected() {
		t.Error("the panel reports the box connected while nothing is on the air")
	}
}

// TestStatusDoesNotInventAFaultWhenDetectionCannotSeeTheInterface.
//
// Absence is not evidence. A detection that has nothing for the hotspot
// interface says nothing about it, and reading that silence as "not
// broadcasting" would put a fault on the panel of a working box every time a
// detection came back thin.
func TestStatusDoesNotInventAFaultWhenDetectionCannotSeeTheInterface(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	ctx := context.Background()

	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	// A detection in which the wireless list is empty, which is what a machine
	// with no "iw" or a failing enumeration produces.
	w.runner.SetOutput("iw dev", "")
	if _, err := w.svc.Detect(ctx); err != nil {
		t.Fatal(err)
	}

	st, err := w.svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Hotspot.Running {
		t.Errorf("a detection that could not see the interface was read as the interface being off the air: %+v",
			st.Hotspot)
	}
}
