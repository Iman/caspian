// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// twoRadioScenario is the box with a USB dongle plugged in: phy0 is the
// built-in brcmfmac radio, phy1 the TP-Link RTL8192EU. Captured 2026-08-30.
func twoRadioScenario(route string) scenario {
	s := pi5Captured()
	s.name = "two radios"
	s.route = route
	s.route6 = ""
	s.addr = "capture-pi5-2radio-ip-br-addr.txt"
	s.dlink = ""
	s.iwdev = "capture-pi5-2radio-iw-dev.txt"
	s.iwlist = "capture-pi5-2radio-iw-list.txt"
	s.nmcli = "capture-pi5-2radio-nmcli.txt"
	return s
}

// dongleOnlyScenario models a machine whose ONLY radio is the dongle, using it
// for the internet. The phy1 block is a verbatim excerpt of the two-radio
// capture; the machine it describes is authored.
func dongleOnlyScenario() scenario {
	s := pi5Captured()
	s.name = "dongle only, dongle is the uplink"
	s.route = "scenario-dongle-only-ip-route.txt"
	s.route6 = ""
	s.addr = "scenario-dongle-only-ip-br-addr.txt"
	s.dlink = ""
	s.iwdev = "scenario-dongle-only-iw-dev.txt"
	s.iwlist = "scenario-dongle-only-iw-list.txt"
	s.nmcli = "scenario-dongle-only-nmcli.txt"
	return s
}

// What the two radios declare. The dongle declares AP among its modes and
// declares no interface combinations at all.
func TestCaptured_TwoRadiosDeclareDifferentThings(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-2radio-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Phy{}
	for _, p := range phys {
		byName[p.Name] = p
	}
	if len(byName) != 2 {
		t.Fatalf("phys = %d, want 2", len(byName))
	}

	dongle := byName["phy1"]
	if !dongle.SupportsAP() {
		t.Errorf("phy1 modes = %v, want AP among them", dongle.Modes)
	}
	if len(dongle.Combinations) != 0 {
		t.Errorf("phy1 declares %d combinations, want none: the absence is the point",
			len(dongle.Combinations))
	}
	if ok, _ := dongle.APWithStation(); ok {
		t.Error("phy1 declares nothing, so it cannot be said to permit AP beside a station")
	}
	if len(dongle.UsableChannels()) == 0 {
		t.Error("phy1 reports no usable channel")
	}

	builtin := byName["phy0"]
	if len(builtin.Combinations) != 2 {
		t.Errorf("phy0 declares %d combinations, want 2", len(builtin.Combinations))
	}
	if ok, _ := builtin.APWithStation(); !ok {
		t.Error("phy0 declares a managed+AP combination")
	}
}

// PARKED AND KNOWN BROKEN: choosing the dongle produces a plan that fails.
//
// The planner no longer refuses a radio that declares no combinations, and
// that much is right: an absent declaration is a statement about COEXISTING,
// and it says nothing about a radio whose station is going to be ended.
//
// But the plan it produces does not work on this hardware, and this test
// records that rather than pretending otherwise. MEASURED on the target
// 2026-08-30:
//
//	iw phy phy1 interface add captest type __ap   rc=0, the interface EXISTS
//	ip link set dev captest up                    refused
//
// The create succeeds, so the fallback, which triggers on the create failing,
// never runs, and the start dies at the link-up. The fix is designed and
// parked; see testdata/PROVENANCE.md, "OPEN DEFECT: the second-interface
// attempt cannot work on either radio".
//
// WHEN THAT IS FIXED this test must change. It asserts today's broken shape so
// that the shape cannot drift unnoticed while the fix waits.
func TestTwoRadio_ChoosingTheDonglePlansATakeover(t *testing.T) {
	s := twoRadioScenario("capture-pi5-ip-route-default.txt")
	f := s.facts(t, BaseSysctlKnobs())
	o := DefaultOptions()
	o.HotspotOverride = "wlan1"

	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err != nil {
		t.Fatalf("choosing the dongle: %v", err)
	}

	// The shape that MATCHES the radio: no second interface, the station on
	// it ended first, the access point on the interface already there.
	if p.Hotspot != "wlan1" {
		t.Fatalf("hotspot = %q, want wlan1, the interface the dongle already has", p.Hotspot)
	}
	if p.HotspotIsVirtual {
		t.Error("a second interface is still being created on a radio that declares it cannot hold one beside a station")
	}
	if !p.HotspotTakenOver {
		t.Error("the plan does not take the interface over, so nothing ends the station link first")
	}
	if p.HotspotPhy != "phy1" {
		t.Errorf("hotspot radio = %q, want phy1", p.HotspotPhy)
	}
	if p.ChannelPinned {
		t.Errorf("channel pinned to %d; with the station ended the access point owns the radio", p.Channel)
	}

	// The release sequence, which is the whole point of doing this at plan
	// time, and the order proved by hand on the box.
	// BOTH addresses come off, and both are real: the capture shows wlan1
	// holding 10.90.1.1/24, a hotspot address left by an earlier run, next to
	// 10.0.0.160/24 from the house network. An interface serving DHCP while
	// holding either of those has a path onto a network that is not ours.
	wantSequence(t, "release steps", stepKeys(p.HotspotReleaseSteps()), []string{
		"nmcli device set wlan1 managed no",
		"ip address del 10.90.1.1/24 dev wlan1",
		"ip address del 10.0.0.160/24 dev wlan1",
		"ip link set dev wlan1 down",
		"iw dev wlan1 set type __ap",
	})
	// And every one of them comes back, because the adapter is the user's.
	wantSequence(t, "release inverses", undoKeys(p.HotspotReleaseSteps()), []string{
		"nmcli device set wlan1 managed yes",
		"ip address add 10.90.1.1/24 dev wlan1",
		"ip address add 10.0.0.160/24 dev wlan1",
		"ip link set dev wlan1 up",
		"iw dev wlan1 set type managed",
	})

	// The cost is stated, and it names the network that stops.
	notes := strings.Join(p.Notes, "\n")
	contains(t, notes, "the connection to")
	contains(t, notes, "HomeNet")
	contains(t, notes, "restored when the hotspot is switched off")
	notContains(t, notes, "will fail to start")
}

// With the internet on the built-in radio, the hotspot goes to the dongle.
// This is the arrangement the user asked for and the reason the dongle was
// bought.
//
// The mechanism changed on 2026-08-30 and the outcome did not: it used to be a
// second interface beside the dongle's station, which the radio cannot hold and
// which failed at the link-up; it is now a takeover that ends that station
// first. This test asserted the outcome before the takeover existed, then
// asserted a refusal while it did not, and now asserts the outcome again.
func TestTwoRadio_BuiltInAsUplinkPutsTheHotspotOnTheDongle(t *testing.T) {
	s := twoRadioScenario("scenario-2radio-ip-route-wlan0.txt")
	f := s.facts(t, BaseSysctlKnobs())
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.Uplink != "wlan0" {
		t.Fatalf("uplink = %q, want wlan0", p.Uplink)
	}
	if p.HotspotPhy != "phy1" {
		t.Errorf("hotspot radio = %q, want the dongle phy1", p.HotspotPhy)
	}
	if p.Hotspot == p.Uplink {
		t.Errorf("hotspot and uplink are both %q", p.Hotspot)
	}
	if !p.HotspotTakenOver || p.HotspotIsVirtual {
		t.Errorf("takenOver=%v virtual=%v, want a takeover on a radio that declares no combination",
			p.HotspotTakenOver, p.HotspotIsVirtual)
	}
	// The internet connection is not the thing being ended.
	for _, st := range p.HotspotReleaseSteps() {
		if strings.Contains(RunnerKey(st.Do), p.Uplink) {
			t.Fatalf("the release touches the uplink: %s", RunnerKey(st.Do))
		}
	}
}

// The coordinator's explicit question, as a test rather than a reading of the
// code: with the uplink on one radio and the hotspot on another there is no
// shared radio, so nothing pins the channel.
//
// The machine is built here rather than taken from the two-radio capture,
// because that capture's dongle is joined to a network of its own and is
// therefore refused outright now. What this test is about is a FREE second
// radio, which is mode B, the arrangement this product tells people to buy.
func TestTwoRadio_NothingIsPinnedWhenTheRadiosAreDifferent(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-2radio-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f := Facts{
		Phys: phys,
		Wireless: []WirelessIface{
			// The uplink, joined to the house network, measured as joined.
			{Name: "wlan0", Phy: "phy0", Type: "managed", SSID: "HomeNet", Channel: 10,
				LinkKnown: true, Associated: true, Manager: ManagedByNetworkManager},
			// The dongle, free.
			{Name: "wlan1", Phy: "phy1", Type: "managed", LinkKnown: true, Associated: false,
				Manager: ManagedByNothing},
		},
		Routes: []DefaultRoute{{
			Family: 4, Dev: "wlan0", Metric: 600,
			Gateway: netip.MustParseAddr("10.0.0.1"), Src: netip.MustParseAddr("10.0.0.222"),
		}},
		Links: []Link{
			{Name: "wlan0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.222/24")}},
			{Name: "wlan1", State: "DOWN", Bus: "usb"},
		},
		Sysctl: map[string]string{},
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	uplinkWireless, ok := f.WirelessByName(p.Uplink)
	if !ok {
		t.Fatal("this test needs a wireless uplink to mean anything")
	}
	if uplinkWireless.Phy == p.HotspotPhy {
		t.Fatalf("uplink and hotspot are on the same radio %q, so this test guards nothing", p.HotspotPhy)
	}
	if p.ChannelPinned {
		t.Errorf("channel pinned to %d across two different radios; nothing constrains them together", p.Channel)
	}
	// The channel must come from the hotspot radio's own usable list, not be
	// read off the uplink.
	hotspotPhy, ok := f.PhyByName(p.HotspotPhy)
	if !ok {
		t.Fatalf("the plan names a radio %q that is not in the facts", p.HotspotPhy)
	}
	inOwnList := false
	for _, ch := range hotspotPhy.UsableChannels() {
		if ch == p.Channel {
			inOwnList = true
		}
	}
	if !inOwnList {
		t.Errorf("channel %d is not one %s reported it can use", p.Channel, p.HotspotPhy)
	}
}

// A radio that must KEEP its station still needs the declaration, because the
// access point has to coexist with it. Taking it over would end the internet
// connection the box is sharing.
func TestDongleOnly_ARadioCarryingTheUplinkStillNeedsTheCombination(t *testing.T) {
	f := dongleOnlyScenario().facts(t, BaseSysctlKnobs())
	if len(f.Phys) != 1 {
		t.Fatalf("phys = %d, want only the dongle", len(f.Phys))
	}

	_, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if !errors.Is(err, ErrAPConflictsWithUplink) {
		t.Fatalf("err = %v, want ErrAPConflictsWithUplink: the only radio is the internet connection", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("the refusal must carry wording for the panel")
	}
	contains(t, pe.UserMessage(), "cable")
	contains(t, pe.UserMessage(), "USB WiFi adapter")
	notContains(t, pe.UserMessage(), "phy")
	notContains(t, pe.UserMessage(), "combination")
}

// THE TAKEOVER MUST NEVER TAKE THE UPLINK, including when the user names it by
// hand.
//
// chooseHotspot never offers the uplink's radio as a takeover candidate, but
// the HotspotOverride path calls acceptHotspot DIRECTLY and skips that
// classification entirely. So the guard has to be in both places, and this
// test is the one that fails if the second copy is removed: a mutation that
// dropped it from acceptHotspot passed every other test in the package.
//
// What it would cost is the whole product: the access point takes over the
// interface carrying the internet connection, so the box has nothing left to
// share, and the user who chose that interface gets a hotspot that reaches
// nowhere.
func TestTakeover_NeverTakesTheUplinkEvenWhenNamedByHand(t *testing.T) {
	f := dongleOnlyScenario().facts(t, BaseSysctlKnobs())
	if len(f.Phys) != 1 {
		t.Fatalf("phys = %d, want only the dongle", len(f.Phys))
	}
	if f.Phys[0].DeclaresAPWithStation() {
		t.Fatal("this test needs a radio that declares no combination, or it proves nothing")
	}

	// The interface this machine actually has, and the radio it is on. Both
	// are ways a user can name the same thing in advanced settings.
	for _, override := range []string{"wlan1", f.Phys[0].Name} {
		t.Run(override, func(t *testing.T) {
			o := DefaultOptions()
			o.HotspotOverride = override
			p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
			if err == nil {
				t.Fatalf("planned hotspot=%s takenOver=%v on the interface carrying the internet connection %s",
					p.Hotspot, p.HotspotTakenOver, p.Uplink)
			}
			if !errors.Is(err, ErrAPConflictsWithUplink) {
				t.Fatalf("err = %v, want ErrAPConflictsWithUplink", err)
			}
		})
	}
}

// The default arrangement is untouched: cable in, hotspot on the built-in
// radio, whether or not a dongle is plugged in.
func TestTwoRadio_TheDefaultArrangementIsUnchanged(t *testing.T) {
	s := twoRadioScenario("capture-pi5-ip-route-default.txt")
	f := s.facts(t, BaseSysctlKnobs())
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.Uplink != "eth0" {
		t.Errorf("uplink = %q, want eth0", p.Uplink)
	}
	if p.HotspotPhy != "phy0" || p.Hotspot != "wlan0" {
		t.Errorf("hotspot = %q on %q, want wlan0 on the built-in phy0: plugging a dongle in must not "+
			"move the hotspot off the radio the default arrangement uses", p.Hotspot, p.HotspotPhy)
	}
	if p.HotspotIsVirtual {
		t.Error("the built-in radio has no station link here, so the access point owns it outright")
	}
}

// The hotspot is never the interface carrying the internet connection.
//
// MEASURED as a gap: planning against the two-radio capture with the default
// route on the built-in radio produced hotspot == uplink == wlan0, because the
// interface was type AP and so read as having no station link to preserve. The
// uplink must count as a link that cannot be disturbed whatever type it
// reports.
func TestPlan_TheHotspotIsNeverTheUplink(t *testing.T) {
	cases := []struct {
		name string
		s    scenario
	}{
		{"captured pi5", pi5Captured()},
		{"authored mode A", modeAScenario()},
		{"authored mode B", modeBScenario()},
		{"two radios, wired uplink", twoRadioScenario("capture-pi5-ip-route-default.txt")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := c.s.facts(t, BaseSysctlKnobs())
			p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if p.Hotspot == p.Uplink {
				t.Errorf("hotspot and uplink are both %q; the access point would end the internet "+
					"connection the box exists to share", p.Hotspot)
			}
			if p.HotspotParent == p.Uplink && !p.HotspotIsVirtual {
				t.Errorf("the hotspot takes over %q, which carries the uplink", p.HotspotParent)
			}
		})
	}

	// The two-radio machine with the internet on the built-in radio used to be
	// a sixth case here. It is now a REFUSAL rather than a plan, so it cannot
	// assert anything about the hotspot not being the uplink, and asserting the
	// refusal is TestTwoRadio_BuiltInAsUplinkRefusesRatherThanChoosingEitherRadio's
	// job. What must not happen is that machine quietly starting to plan again
	// with the hotspot on the uplink's own radio, so that is checked here.
	t.Run("two radios, built-in uplink, refused rather than planned onto the uplink", func(t *testing.T) {
		f := twoRadioScenario("scenario-2radio-ip-route-wlan0.txt").facts(t, BaseSysctlKnobs())
		p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
		if err == nil && p.Hotspot == p.Uplink {
			t.Fatalf("hotspot and uplink are both %q", p.Hotspot)
		}
		if err == nil {
			uplinkWireless, ok := f.WirelessByName(p.Uplink)
			if ok && uplinkWireless.Phy == p.HotspotPhy {
				t.Fatalf("the hotspot was planned onto %s, the radio carrying the internet connection",
					p.HotspotPhy)
			}
		}
	})
}

// Which of two usable radios gets the access point must be a decision, not a
// consequence of the order "iw list" happened to print them in.
//
// Facts are built directly here rather than parsed, because the thing under
// test is the selection and the only way to vary the order is to vary it. The
// radios themselves are the captured ones: phy0 declares the managed+AP
// combination, phy1 declares none.
func TestTwoRadio_TheRadioCarryingTheUplinkIsTheLastResort(t *testing.T) {
	phys, err := ParseIwList(read(t, "capture-pi5-2radio-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var builtin, dongle Phy
	for _, p := range phys {
		switch p.Name {
		case "phy0":
			builtin = p
		case "phy1":
			dongle = p
		}
	}
	if len(builtin.Combinations) == 0 || len(dongle.Combinations) != 0 {
		t.Fatal("the captured radios are not what this test assumes")
	}

	// Both radios carry a station. wlan0's IS the internet connection.
	wireless := []WirelessIface{
		{Name: "wlan0", Phy: "phy0", Type: "managed", SSID: "HomeNet", Channel: 10, Manager: ManagedByNetworkManager},
		{Name: "wlan1", Phy: "phy1", Type: "managed", SSID: "HomeNet", Channel: 10, Manager: ManagedByNetworkManager},
	}
	routes := []DefaultRoute{{
		Family: 4, Dev: "wlan0", Metric: 600,
		Gateway: netip.MustParseAddr("10.0.0.1"), Src: netip.MustParseAddr("10.0.0.222"),
	}}
	links := []Link{
		{Name: "wlan0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.222/24")}},
		{Name: "wlan1", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.160/24")}},
	}

	// Both orders must give the same answer, and it must be the dongle.
	//
	// The reasoning is in the comment above and has not changed. What changed
	// on 2026-08-30 is HOW the dongle is used: a takeover that ends its
	// station, rather than a second interface the radio cannot hold. Ending a
	// connection that is not the internet beats putting the access point
	// beside the internet connection, where it is pinned to that connection's
	// channel, roams with it, and has no fallback.
	for _, order := range [][]Phy{{builtin, dongle}, {dongle, builtin}} {
		names := order[0].Name + " then " + order[1].Name
		t.Run(names, func(t *testing.T) {
			f := Facts{Links: links, Routes: routes, Wireless: wireless, Phys: order, Sysctl: map[string]string{}}
			p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if p.Uplink != "wlan0" {
				t.Fatalf("uplink = %q", p.Uplink)
			}
			if p.HotspotPhy != "phy1" {
				t.Errorf("hotspot radio = %q, want phy1.\n"+
					"phy0 carries the internet connection: an access point there is pinned to the uplink's "+
					"channel, follows it on a roam, and has no fallback because taking that interface over "+
					"would end the connection being shared.", p.HotspotPhy)
			}
			if !p.HotspotTakenOver {
				t.Errorf("the dongle declares no combination, so the only way to use it is to end its station first")
			}
			if p.ChannelPinned {
				t.Errorf("channel pinned to %d; the chosen radio does not carry the uplink", p.Channel)
			}
		})
	}
}

// THE PHY NUMBERS SWAP ACROSS A REBOOT. Measured on the target 2026-08-30: the
// built-in radio and the USB dongle came back with their wiphy indices
// exchanged, so "phy1" named a different piece of hardware than it had before
// the power cycle.
//
// Nothing may therefore be decided from a REMEMBERED index. This test proves
// the radio is resolved from the interface within one detection run: rename
// the phys in the facts, change nothing else, and the same interface must be
// chosen and the create command must follow it to whatever the radio is now
// called.
//
// It would fail if any part of the planner matched a phy by name or number
// against something carried over from an earlier run.
func TestPhyNumbersMaySwapAndTheDecisionFollowsTheInterface(t *testing.T) {
	sc := twoRadioScenario("capture-pi5-ip-route-default.txt")
	f, err := Detect(context.Background(), sc.runner(t), BaseSysctlKnobs())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	swapped := f
	swapped.Phys = append([]Phy{}, f.Phys...)
	swapped.Wireless = append([]WirelessIface{}, f.Wireless...)
	swap := func(n string) string {
		switch n {
		case "phy0":
			return "phy1"
		case "phy1":
			return "phy0"
		}
		return n
	}
	for i := range swapped.Phys {
		swapped.Phys[i].Name = swap(swapped.Phys[i].Name)
	}
	for i := range swapped.Wireless {
		swapped.Wireless[i].Phy = swap(swapped.Wireless[i].Phy)
	}

	before, errBefore := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	after, errAfter := PlanNetwork(swapped, []netip.Addr{testServer}, DefaultOptions())

	if (errBefore == nil) != (errAfter == nil) {
		t.Fatalf("renumbering the radios changed whether the plan succeeds: before=%v after=%v", errBefore, errAfter)
	}
	if errBefore != nil {
		// Both refused. The refusal must be the same one, for the same
		// interface, and it must not name a number as the reason.
		if errBefore.Error() != errAfter.Error() {
			t.Fatalf("refusals differ after a renumber:\n before: %v\n  after: %v", errBefore, errAfter)
		}
		return
	}

	if before.Hotspot != after.Hotspot || before.HotspotParent != after.HotspotParent ||
		before.HotspotIsVirtual != after.HotspotIsVirtual {
		t.Fatalf("the renumber moved the hotspot: before %s (parent %q, virtual %v), after %s (parent %q, virtual %v)",
			before.Hotspot, before.HotspotParent, before.HotspotIsVirtual,
			after.Hotspot, after.HotspotParent, after.HotspotIsVirtual)
	}
	if before.Uplink != after.Uplink {
		t.Fatalf("the renumber moved the uplink: %s then %s", before.Uplink, after.Uplink)
	}

	// The phy the plan uses must be the one THIS run's facts give for the
	// interface the access point is built on, not the number it had before.
	wantPhy := func(f Facts, p *Plan) string {
		name := p.Hotspot
		if p.HotspotIsVirtual {
			name = p.HotspotParent
		}
		for _, w := range f.Wireless {
			if w.Name == name {
				return w.Phy
			}
		}
		return ""
	}
	if got, want := before.HotspotPhy, wantPhy(f, before); got != want {
		t.Fatalf("HotspotPhy = %q, want %q, the radio the chosen interface is on", got, want)
	}
	if got, want := after.HotspotPhy, wantPhy(swapped, after); got != want {
		t.Fatalf("after the renumber HotspotPhy = %q, want %q", got, want)
	}
	if before.HotspotPhy == after.HotspotPhy {
		t.Fatalf("both plans say %q: the swap did not reach the decision, so this test guards nothing",
			before.HotspotPhy)
	}
}

// THE PHY NUMBERS MOVED AGAIN, and they are not contiguous.
//
// MEASURED on the target across one day, 2026-08-30: phy0 was brcmfmac in the
// morning, phy0 was rtl8xxxu a few hours later, and by the evening the radios
// were phy1 (brcmfmac) and phy2 (rtl8xxxu) with no phy0 at all. Interface
// names were stable throughout.
//
// So nothing may assume a phy index is stable, zero-based, contiguous, or
// bound to a particular driver. This test renames the radios to the numbers
// the box actually reported, changes nothing else, and requires the same
// decision.
func TestPhyNumbersAreNotContiguousAndTheDecisionIsUnchanged(t *testing.T) {
	base := func(t *testing.T) Facts {
		phys, err := ParseIwList(read(t, "capture-pi5-2radio-iw-list.txt"))
		if err != nil {
			t.Fatal(err)
		}
		return Facts{
			Phys: phys,
			Wireless: []WirelessIface{
				{Name: "wlan0", Phy: "phy0", Type: "managed", SSID: "HomeNet", Channel: 10,
					LinkKnown: true, Associated: true, Manager: ManagedByNetworkManager},
				{Name: "wlan1", Phy: "phy1", Type: "managed", SSID: "Guest", Channel: 6,
					LinkKnown: true, Associated: true, Manager: ManagedByNetworkManager},
			},
			Routes: []DefaultRoute{{
				Family: 4, Dev: "eth0", Metric: 100,
				Gateway: netip.MustParseAddr("192.168.1.1"), Src: netip.MustParseAddr("192.168.1.50"),
			}},
			Links: []Link{
				{Name: "eth0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("192.168.1.50/24")}},
				{Name: "wlan0", State: "UP"},
				{Name: "wlan1", State: "UP", Bus: "usb"},
			},
			Sysctl: map[string]string{},
		}
	}

	before := base(t)
	pb, err := PlanNetwork(before, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatalf("plan before the renumber: %v", err)
	}

	// The reboot: phy0 and phy1 become phy1 and phy2, and the hardware behind
	// each name is unchanged.
	after := base(t)
	rename := map[string]string{"phy0": "phy1", "phy1": "phy2"}
	for i := range after.Phys {
		after.Phys[i].Name = rename[after.Phys[i].Name]
	}
	for i := range after.Wireless {
		after.Wireless[i].Phy = rename[after.Wireless[i].Phy]
	}
	pa, err := PlanNetwork(after, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatalf("plan after the renumber: %v", err)
	}

	if pb.Hotspot != pa.Hotspot || pb.HotspotTakenOver != pa.HotspotTakenOver ||
		pb.HotspotIsVirtual != pa.HotspotIsVirtual || pb.Channel != pa.Channel {
		t.Fatalf("the renumber changed the decision:\n before: hotspot=%s takenOver=%v virtual=%v channel=%d\n  after: hotspot=%s takenOver=%v virtual=%v channel=%d",
			pb.Hotspot, pb.HotspotTakenOver, pb.HotspotIsVirtual, pb.Channel,
			pa.Hotspot, pa.HotspotTakenOver, pa.HotspotIsVirtual, pa.Channel)
	}
	if pb.HotspotPhy == pa.HotspotPhy {
		t.Fatalf("both plans name radio %q, so the renumber did not reach the decision and this test guards nothing",
			pb.HotspotPhy)
	}
	if pa.HotspotPhy != rename[pb.HotspotPhy] {
		t.Fatalf("after the renumber the plan names %q; the radio the chosen interface is on is now %q",
			pa.HotspotPhy, rename[pb.HotspotPhy])
	}
}

// SHARING BEATS TAKING OVER. Both radios carry a connection that is not the
// internet; one can hold an access point beside it and one cannot. The one
// that can wins, because the other costs somebody their connection.
//
// No fixture had both shapes at once, so a mutation that reversed this
// preference passed every test in the package. This machine is built by hand
// from the two captured radios for that reason.
func TestTakeover_SharingIsPreferredOverEndingAConnection(t *testing.T) {
	twoRadio, err := ParseIwList(read(t, "capture-pi5-2radio-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var sharer, taker Phy
	for _, p := range twoRadio {
		if p.DeclaresAPWithStation() {
			sharer = p
		} else {
			taker = p
		}
	}
	if sharer.Name == "" || taker.Name == "" {
		t.Fatal("the captured radios no longer differ in this way, so this test proves nothing")
	}

	f := Facts{
		Phys: []Phy{sharer, taker},
		Wireless: []WirelessIface{
			{Name: "wlanA", Phy: sharer.Name, Type: "managed", SSID: "Guest", Channel: 6,
				LinkKnown: true, Associated: true, Manager: ManagedByNetworkManager},
			{Name: "wlanB", Phy: taker.Name, Type: "managed", SSID: "HomeNet", Channel: 10,
				LinkKnown: true, Associated: true, Manager: ManagedByNetworkManager},
		},
		Routes: []DefaultRoute{{
			Family: 4, Dev: "eth0", Metric: 100,
			Gateway: netip.MustParseAddr("192.168.1.1"), Src: netip.MustParseAddr("192.168.1.50"),
		}},
		Links: []Link{
			{Name: "eth0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("192.168.1.50/24")}},
			{Name: "wlanA", State: "UP"},
			// The USB bus is on the radio that would have to be taken over, so
			// the mode B preference for a USB adapter cannot be what decides
			// this: only sharing versus ending a connection can.
			{Name: "wlanB", State: "UP", Bus: "usb"},
		},
		Sysctl: map[string]string{},
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.HotspotTakenOver {
		t.Fatalf("the plan ended the connection on %s while %s could have held an access point beside its own",
			p.Hotspot, sharer.Name)
	}
	if p.HotspotPhy != sharer.Name {
		t.Fatalf("hotspot radio = %q, want %q, the one that declares it can do both", p.HotspotPhy, sharer.Name)
	}
	if !p.HotspotIsVirtual || p.HotspotParent != "wlanA" {
		t.Fatalf("virtual=%v parent=%q, want a second interface beside wlanA", p.HotspotIsVirtual, p.HotspotParent)
	}
	// And nothing in the plan touches the other radio's connection.
	for _, st := range p.AllSteps(f.Sysctl) {
		if strings.Contains(RunnerKey(st.Do), "wlanB") {
			t.Fatalf("the plan touches wlanB, whose connection it has no reason to disturb: %s", RunnerKey(st.Do))
		}
	}
}
