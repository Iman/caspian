// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// The release sequence, asserted exactly, because the order is the whole point
// and every one of these was missing when the takeover was reported as done.
func TestHotspotReleaseSteps_ExactSequence(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}

	wantSequence(t, "release steps", stepKeys(q.HotspotReleaseSteps()), []string{
		"nmcli device set wlan0 managed no",
		"ip address del 10.0.0.222/24 dev wlan0",
		"ip link set dev wlan0 down",
		"iw dev wlan0 set type __ap",
	})

	// Every one has an inverse. The nmcli one especially: a user whose Pi
	// permanently stopped joining their WiFi has lost more than a hotspot.
	wantSequence(t, "release inverses", undoKeys(q.HotspotReleaseSteps()), []string{
		"nmcli device set wlan0 managed yes",
		"ip address add 10.0.0.222/24 dev wlan0",
		"ip link set dev wlan0 up",
		"iw dev wlan0 set type managed",
	})
}

// The station address must come off. Leaving it is what gave the DHCP server a
// path onto the house network, where it answered a real device with DHCPNAK.
func TestHotspotTakeover_StripsTheStationAddress(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.HotspotStationPrefixes) != 1 {
		t.Fatalf("station prefixes = %v, want the one address wlan0 holds", q.HotspotStationPrefixes)
	}
	if q.HotspotStationPrefixes[0] != netip.MustParsePrefix("10.0.0.222/24") {
		t.Errorf("station prefix = %v", q.HotspotStationPrefixes[0])
	}
	// Link-local is left alone: the kernel regenerates it and it reaches
	// nobody.
	for _, pfx := range q.HotspotStationPrefixes {
		if pfx.Addr().IsLinkLocalUnicast() {
			t.Errorf("link-local %v should not be removed", pfx)
		}
	}

	// The removal must be ordered before the hotspot address is added.
	keys := stepKeys(q.AllSteps(f.Sysctl))
	delAt, addAt := -1, -1
	for i, k := range keys {
		if k == "ip address del 10.0.0.222/24 dev wlan0" {
			delAt = i
		}
		if k == "ip address add 10.83.51.1/24 dev wlan0" {
			addAt = i
		}
	}
	if delAt < 0 || addAt < 0 || delAt > addAt {
		t.Fatalf("station address removed at %d, hotspot address added at %d, in:\n  %s",
			delAt, addAt, strings.Join(keys, "\n  "))
	}
}

// Not knowing what holds an interface is the state that put a DHCP server on
// somebody's home network, so it is a refusal.
func TestHotspotTakeover_RefusesWhenTheOwnerIsUnknown(t *testing.T) {
	s := pi5Captured()
	s.nmcli = "" // nmcli absent or failing: NetworkManager cannot be asked
	f := s.facts(t, BaseSysctlKnobs())

	w, ok := f.WirelessByName("wlan0")
	if !ok {
		t.Fatal("wlan0 missing from facts")
	}
	if w.Manager != ManagedByUnknown {
		t.Fatalf("manager = %q, want unknown when nmcli cannot answer", w.Manager)
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.HotspotTakeover(f)
	if !errors.Is(err, ErrInterfaceOwnerUnknown) {
		t.Fatalf("err = %v, want ErrInterfaceOwnerUnknown", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("the refusal must carry wording for the panel")
	}
	contains(t, pe.UserMessage(), "disrupt that network")
	notContains(t, pe.UserMessage(), "nmcli")
}

// An interface that is joined to a network with no manager claiming it is held
// by something this package cannot release, so it is left alone.
func TestHotspotTakeover_RefusesWhenAssociatedButUnmanaged(t *testing.T) {
	s := pi5Captured()
	s.nmcli = "scenario-nmcli-wlan0-unmanaged.txt" // NM says it is not ours
	f := s.facts(t, BaseSysctlKnobs())

	w, _ := f.WirelessByName("wlan0")
	if w.Manager != ManagedByNothing {
		t.Fatalf("manager = %q, want none", w.Manager)
	}
	if !w.InUse() {
		t.Fatalf("wlan0 must still be associated for this test to mean anything: ssid=%q channel=%d", w.SSID, w.Channel)
	}

	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.HotspotTakeover(f)
	if !errors.Is(err, ErrInterfaceOwnerUnknown) {
		t.Fatalf("err = %v, want a refusal: something holds wlan0 that this package cannot name", err)
	}
	var pe *PlanError
	if errors.As(err, &pe) {
		contains(t, pe.UserMessage(), "Disconnect that WiFi network first")
	}
}

// A released, idle interface needs no nmcli step.
func TestHotspotReleaseSteps_NoManagerMeansNoReleaseCommand(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	q.HotspotManager = ManagedByNothing
	for _, s := range q.HotspotReleaseSteps() {
		if s.Do.Path == BinNmcli {
			t.Errorf("nothing manages the interface, so nothing should be asked to release it: %s", RunnerKey(s.Do))
		}
	}
	// The address strip and the type change still have to happen.
	contains(t, strings.Join(stepKeys(q.HotspotReleaseSteps()), "\n"), "iw dev wlan0 set type __ap")
}

// The readback. A process being alive is the same class of evidence as a
// connect code: necessary, not sufficient.
func TestAssertHotspotInterfaceReleased(t *testing.T) {
	ctx := context.Background()
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q := *p
	q.Hotspot = "wlan0"
	q.HotspotSubnet = netip.MustParsePrefix("10.83.51.0/24")

	// The state the box was actually in: still a station on the house
	// network, still holding its address.
	// Every runner below states the link state explicitly, from the real
	// captures. It used to be left unregistered, which the double answered
	// with empty output, which the parser read as "free": the still-joined
	// case was then refused for a DIFFERENT reason than the one it claims to
	// test. Unrecognised link output is now an error, which is what surfaced
	// that.
	stillJoined := NewRecordingRunner()
	stillJoined.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-connected.txt"))
	stillJoined.SetOutput("iw dev", read(t, "capture-pi5-iw-dev.txt"))
	stillJoined.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.0.0.222/24 fe80::2ecf:67ff:fe72:51f7/64 \n")
	err := AssertHotspotInterfaceReleased(ctx, stillJoined, &q)
	if !errors.Is(err, ErrHotspotNotReleased) {
		t.Fatalf("err = %v, want ErrHotspotNotReleased: this is exactly the state the box was in", err)
	}
	contains(t, err.Error(), "HomeNet")
	contains(t, err.Error(), "still joined to the network")

	// Released and stripped.
	freed := NewRecordingRunner()
	freed.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
	freed.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\tifindex 3\n\t\ttype AP\n")
	freed.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.83.51.1/24 fe80::2ecf:67ff:fe72:51f7/64 \n")
	if err := AssertHotspotInterfaceReleased(ctx, freed, &q); err != nil {
		t.Errorf("a released, stripped interface must pass: %v", err)
	}

	// Released but still carrying an address from the other network. This is
	// the half that let dnsmasq reach the house LAN.
	halfDone := NewRecordingRunner()
	halfDone.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
	halfDone.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\tifindex 3\n\t\ttype AP\n")
	halfDone.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.83.51.1/24 10.0.0.222/24 \n")
	err = AssertHotspotInterfaceReleased(ctx, halfDone, &q)
	if !errors.Is(err, ErrHotspotNotReleased) {
		t.Fatalf("err = %v, want a refusal: an address from another network is a path onto it", err)
	}
	contains(t, err.Error(), "10.0.0.222/24")
}

func TestAssertHotspotIsAccessPoint(t *testing.T) {
	ctx := context.Background()
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q := *p
	q.Hotspot = "wlan0"

	// What the box actually reported while claiming to be running.
	managed := NewRecordingRunner()
	managed.SetOutput("iw dev", read(t, "capture-pi5-iw-dev.txt"))
	err := AssertHotspotIsAccessPoint(ctx, managed, &q, "Caspian-Wifi")
	if !errors.Is(err, ErrNotAccessPoint) {
		t.Fatalf("err = %v, want ErrNotAccessPoint", err)
	}
	contains(t, err.Error(), `reports type "managed"`)

	// An access point, but broadcasting the wrong name.
	wrongSSID := NewRecordingRunner()
	wrongSSID.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\tssid HomeNet\n\t\ttype AP\n")
	err = AssertHotspotIsAccessPoint(ctx, wrongSSID, &q, "Caspian-Wifi")
	if !errors.Is(err, ErrNotAccessPoint) {
		t.Fatalf("err = %v, want a refusal on the SSID", err)
	}
	contains(t, err.Error(), "Caspian-Wifi")

	// Right mode, right name.
	good := NewRecordingRunner()
	good.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\tssid Caspian-Wifi\n\t\ttype AP\n")
	if err := AssertHotspotIsAccessPoint(ctx, good, &q, "Caspian-Wifi"); err != nil {
		t.Errorf("a real access point must pass: %v", err)
	}

	// Gone entirely.
	absent := NewRecordingRunner()
	absent.SetOutput("iw dev", "")
	if err := AssertHotspotIsAccessPoint(ctx, absent, &q, ""); !errors.Is(err, ErrNotAccessPoint) {
		t.Errorf("err = %v, want a refusal when the interface is not listed at all", err)
	}
}

// End to end: the release actually happens against a kernel that models it,
// and teardown gives the interface back.
func TestApply_TakeoverReleasesTheInterfaceAndGivesItBack(t *testing.T) {
	ctx := context.Background()
	sc := pi5Captured()
	f, p := mustPlan(t, sc, DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}

	k := capturedKernel(t)
	k.Reads = sc.runner(t)
	k.Preload("addr", "wlan0", "10.0.0.222/24")
	before := k.Snapshot()

	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ctx, q.AllSteps(f.Sysctl)); err != nil {
		t.Fatalf("apply: %v", err)
	}

	after := strings.Join(k.Snapshot(), "\n")
	contains(t, after, "unmanaged wlan0")
	contains(t, after, "iftype wlan0=__ap")
	contains(t, after, "addr wlan0 10.83.51.1/24")
	notContains(t, after, "addr wlan0 10.0.0.222/24")

	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	// Everything back: managed by NetworkManager again, a station again,
	// with its own address again.
	wantSequence(t, "machine state after teardown", k.Snapshot(), before)
}

// The next thing that can fail on this hardware, and what happens when it does.
//
// The driver refused to CREATE an interface with "Input/output error (-5)"
// while wlan0 was associated. That refusal has since been measured to be
// conditional on the association rather than a property of the driver, and
// "set type __ap" on the existing interface was measured to SUCCEED, so this
// models an adapter that behaves differently rather than this one. If it refuses, the release has already happened: the
// interface is unmanaged and stripped of the address it was using. Leaving it
// there would be worse than never trying, because the box would have taken the
// user's WiFi away and given nothing back.
func TestApply_TakeoverThatFailsMidReleaseGivesTheInterfaceBack(t *testing.T) {
	ctx := context.Background()
	sc := pi5Captured()
	f, p := mustPlan(t, sc, DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}

	k := capturedKernel(t)
	k.Reads = sc.runner(t)
	k.Preload("addr", "wlan0", "10.0.0.222/24")
	k.RefuseSetType = "command failed: Input/output error (-5)"
	before := k.Snapshot()

	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := a.Apply(ctx, q.AllSteps(f.Sysctl))
	if err == nil {
		t.Fatal("the driver refuses the type change, so the apply must fail rather than carry on")
	}
	failed, ok := rep.FailedStep()
	if !ok || !strings.Contains(RunnerKey(failed.Do), "set type __ap") {
		t.Fatalf("failed step = %v", failed)
	}
	// It must NOT have gone on to address or serve on the interface.
	mid := strings.Join(k.Snapshot(), "\n")
	notContains(t, mid, "addr wlan0 10.83.51.1/24")

	if _, err := a.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	// The user's WiFi has to come back: managed again, address restored.
	wantSequence(t, "machine state after a failed takeover", k.Snapshot(), before)
}

// The real nmcli bytes, and the two shapes an authored guess did not have.
func TestCaptured_ParseNmcliDeviceStatus(t *testing.T) {
	got := ParseNmcliDeviceStatus(read(t, "capture-pi5-nmcli-device-status.txt"))

	want := map[string]InterfaceManager{
		"eth0":          ManagedByNetworkManager,
		"wlan0":         ManagedByNetworkManager,
		"lo":            ManagedByNetworkManager, // "connected (externally)"
		"xray0":         ManagedByNetworkManager, // "connected (externally)"
		"p2p-dev-wlan0": ManagedByNetworkManager, // "disconnected"
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d devices, want %d: %v", len(got), len(want), got)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %q, want %q", name, got[name], w)
		}
	}
}

// A state can carry a parenthetical. Anything that compared the whole field
// against "connected" would answer differently for lo and xray0 than for eth0.
func TestParseNmcliDeviceStatus_StateWithAParenthetical(t *testing.T) {
	got := ParseNmcliDeviceStatus(
		"a0:connected (externally)\n" +
			"a1:unmanaged\n" +
			"a2:unmanaged (externally)\n" +
			"a3:disconnected\n" +
			"a4:\n")

	if got["a0"] != ManagedByNetworkManager {
		t.Errorf(`a0 "connected (externally)" = %q, want managed`, got["a0"])
	}
	if got["a1"] != ManagedByNothing {
		t.Errorf(`a1 "unmanaged" = %q, want none`, got["a1"])
	}
	// The parenthetical must not stop "unmanaged" being recognised either.
	if got["a2"] != ManagedByNothing {
		t.Errorf(`a2 "unmanaged (externally)" = %q, want none`, got["a2"])
	}
	if got["a3"] != ManagedByNetworkManager {
		t.Errorf(`a3 "disconnected" = %q, want managed: NetworkManager still holds it`, got["a3"])
	}
	// A device with no state says nothing about ownership, and silence must
	// not be read as "nobody owns it".
	if _, ok := got["a4"]; ok {
		t.Errorf(`a4 with an empty state was classified as %q`, got["a4"])
	}
}

// The radio presents a second device whose name CONTAINS the real one. A
// prefix or substring match here would let the P2P device decide what is true
// of wlan0.
func TestCaptured_P2PDeviceDoesNotAnswerForTheRadio(t *testing.T) {
	// Released: wlan0 unmanaged, its P2P sibling unavailable.
	got := ParseNmcliDeviceStatus(read(t, "capture-pi5-nmcli-after-release.txt"))
	if got["wlan0"] != ManagedByNothing {
		t.Errorf("wlan0 = %q, want none after release", got["wlan0"])
	}
	if got["p2p-dev-wlan0"] != ManagedByNetworkManager {
		t.Errorf("p2p-dev-wlan0 = %q; it is a different device with its own state", got["p2p-dev-wlan0"])
	}
	// Neither may be reachable under the other's name.
	if _, ok := got["wlan"]; ok {
		t.Error("a partial name resolved to a device")
	}

	// End to end, and this is the case that separates an exact lookup from a
	// substring one: in the post-release listing the two devices have
	// DIFFERENT managers. wlan0 is unmanaged and p2p-dev-wlan0 is not, so a
	// substring match returns whichever the map happened to yield, which is
	// both wrong and non-deterministic.
	sc := pi5Captured()
	sc.nmcli = "capture-pi5-nmcli-after-release.txt"
	f := sc.facts(t, BaseSysctlKnobs())
	w, ok := f.WirelessByName("wlan0")
	if !ok {
		t.Fatal("wlan0 missing")
	}
	if w.Manager != ManagedByNothing {
		t.Errorf("wlan0 manager = %q, want none: it is listed as unmanaged and its P2P sibling is not", w.Manager)
	}
	for _, other := range f.Wireless {
		if strings.HasPrefix(other.Name, "p2p-") {
			t.Errorf("a P2P device leaked into the wireless interface list: %+v", other)
		}
	}

	// And the listing before release, where wlan0 is genuinely managed.
	sc2 := pi5Captured()
	f2 := sc2.facts(t, BaseSysctlKnobs())
	w2, _ := f2.WirelessByName("wlan0")
	if w2.Manager != ManagedByNetworkManager {
		t.Errorf("wlan0 manager = %q, want NetworkManager before release", w2.Manager)
	}
}

// The live refusal, reproduced.
//
// Reported from the box: two starts in a row refused with "the hotspot
// interface is not free: wlan0 is still joined to \"\" on channel 10", with no
// journal file, and the interface left released from NetworkManager but never
// typed. Removing one leftover address made the next start succeed.
//
// The mechanism is not the address itself. It is that releasing an interface
// from NetworkManager can take NM's own address with it, so the delete this
// package then issues finds nothing. Apply treated that as fatal and stopped,
// and the step that types the interface never ran. The readback that runs next
// then refused the interface and blamed an SSID.
//
// A removal whose object is already gone has achieved its goal, which is the
// rule the undo path always had.
func TestApply_ARemovalThatFindsNothingDoesNotStopTheRelease(t *testing.T) {
	ctx := context.Background()
	sc := pi5Captured()
	sc.addr = "scenario-leftover-hotspot-addr.txt"
	f, p := mustPlan(t, sc, DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	// Both addresses are scheduled for removal, which is what the leftover
	// one changes.
	if len(q.HotspotStationPrefixes) != 2 {
		t.Fatalf("station prefixes = %v, want both the real address and the leftover", q.HotspotStationPrefixes)
	}

	k := capturedKernel(t)
	k.Reads = sc.runner(t)
	// The leftover is there; NM's own address is NOT, because releasing the
	// interface took it. This is the state the delete meets on the box.
	k.Preload("addr", "wlan0", "10.83.51.1/24")

	a, err := NewApplier(k, tmpJournal(t))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := a.Apply(ctx, q.AllSteps(f.Sysctl))
	if err != nil {
		t.Fatalf("a delete that finds nothing must not stop the release: %v", err)
	}
	if rep.Failed != 0 {
		t.Errorf("failures: %v", rep.Err())
	}

	// The step that was never reached before must have run.
	snap := strings.Join(k.Snapshot(), " | ")
	contains(t, snap, "iftype wlan0=__ap")
	notContains(t, snap, "addr wlan0 10.83.51.1/24")

	// And nothing may be journalled for the address that was already gone:
	// this package did not remove it, and giving it back is owned by the
	// inverse of the NetworkManager release.
	entries, err := LoadJournal(a.Journal().Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.NeedsUndo() && strings.Contains(CommandLine(e.Undo), "address add 10.0.0.222/24") {
			t.Error("journalled a restore for an address this run never removed")
		}
	}

	// The readback that refused on the box must now pass.
	r := NewRecordingRunner()
	r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
	r.SetOutput("iw dev", read(t, "capture-pi5-iw-dev-freed-ap.txt"))
	r.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.174.29.1/24 \n")
	q2 := *q
	if err := AssertHotspotInterfaceReleased(ctx, r, &q2); err != nil {
		t.Errorf("the readback still refuses a correctly released interface: %v", err)
	}
}

// A channel is NOT an association, and this is the third time that has cost
// this package.
//
// This test used to assert the opposite. It required a managed interface with
// a channel and no SSID to be refused as "still a station", and that
// requirement is what stopped the appliance starting: an access point
// interface the plan had just created inherits the parent radio's channel, so
// the channel is always there, so the readback always failed.
//
// MEASURED on the target 2026-08-30, on a freshly created AP vif:
//
//	iw dev captest info   ->  type AP
//	iw dev captest link   ->  Not connected.
//
// The question is now asked of "link", which answers it, rather than inferred
// from a channel, which does not.
func TestAssertHotspotInterfaceReleased_AChannelIsNotAnAssociation(t *testing.T) {
	ctx := context.Background()
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q := *p
	q.Hotspot = "wlan0"
	q.HotspotSubnet = netip.MustParsePrefix("10.83.51.0/24")

	r := NewRecordingRunner()
	// The exact shape that used to be refused: a channel, no SSID, and not
	// connected to anything.
	r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-not-connected.txt"))
	r.SetOutput("iw dev", "phy#0\n\tInterface wlan0\n\t\ttype managed\n\t\tchannel 36 (5180 MHz)\n")
	r.SetOutput("ip -br addr show dev wlan0", "wlan0  UP  10.83.51.1/24 \n")

	if err := AssertHotspotInterfaceReleased(ctx, r, &q); err != nil {
		t.Fatalf("an interface that reports a channel and is not connected to anything is free: %v", err)
	}
}

// The four measured answers to "is this interface associated", each driven by
// the bytes the target actually printed.
//
// This is the test that would have caught the defect the readback shipped
// with. Case B is a freshly created access point interface: the old code read
// its inherited channel and called it a station, so every start refused, so
// the appliance never came up. Its bytes are IDENTICAL to case A's, which is
// why no parser reading THIS command could have made that mistake.
func TestParseIwLink_TheFourMeasuredCases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture string
		want    linkState
	}{
		{
			name:    "A: managed, up, not joined to anything",
			fixture: "capture-pi5-iw-link-not-connected.txt",
			want:    linkState{},
		},
		{
			name:    "B: a freshly created AP vif, which iw dev info calls type AP",
			fixture: "capture-pi5-iw-link-not-connected.txt",
			want:    linkState{},
		},
		{
			name:    "C: a real station association",
			fixture: "capture-pi5-iw-link-connected.txt",
			want:    linkState{Connected: true, SSID: "HomeNet"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIwLink(read(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("parseIwLink = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Cases A and B are the same file on purpose. If somebody ever "fixes" that by
// giving them different bytes, this fails and says why.
func TestCaptured_AFreshAccessPointVifAndAnIdleStationAnswerIdentically(t *testing.T) {
	b := read(t, "capture-pi5-iw-link-not-connected.txt")
	if strings.TrimSpace(b) != "Not connected." {
		t.Fatalf("capture = %q, want exactly \"Not connected.\"", b)
	}
	// The whole readback rests on this: the two states are indistinguishable
	// here, and both mean free.
	st, err := parseIwLink(b)
	if err != nil {
		t.Fatal(err)
	}
	if st.Connected || st.SSID != "" {
		t.Fatalf("parseIwLink(%q) = %+v, want the zero value", b, st)
	}
}

// Output with no verdict in it is a REFUSAL, not "free". This is the direction
// that matters: a parser that returned the zero value for anything it did not
// understand would report an interface of unknown state as released, and a
// test double that simply forgot to answer would look like a pass.
func TestParseIwLink_UnrecognisedOutputIsAnError(t *testing.T) {
	for _, out := range []string{"", "\n\n", "some future wording nobody has seen", "\tSSID: HomeNet\n"} {
		if _, err := parseIwLink(out); !errors.Is(err, ErrLinkStateUnrecognised) {
			t.Fatalf("parseIwLink(%q) err = %v, want ErrLinkStateUnrecognised", out, err)
		}
	}
}

// The SSID is on the SSID line, not the first line. The first line carries the
// BSSID, and a parser that took the name from it would report a MAC address as
// a network name.
func TestParseIwLink_TakesTheNameFromTheSSIDLineNotTheFirstLine(t *testing.T) {
	out := read(t, "capture-pi5-iw-link-connected.txt")
	first := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(first, "02:00:5e:00:00:01") {
		t.Fatalf("fixture changed: first line %q no longer carries the BSSID", first)
	}
	st, err := parseIwLink(out)
	if err != nil {
		t.Fatal(err)
	}
	if st.SSID != "HomeNet" {
		t.Fatalf("SSID = %q, want HomeNet", st.SSID)
	}
	// The indent on the SSID line is a TAB in the real output.
	if !strings.Contains(out, "\n\tSSID: HomeNet") {
		t.Fatal("fixture changed: the SSID line is no longer TAB-indented, which is what the target printed")
	}
}

// Case D. A hotspot interface that does not exist is a NAMED refusal, not
// "free". Reporting it free would send the caller on to configure an interface
// that is not there.
func TestAssertHotspotInterfaceReleased_AMissingInterfaceIsItsOwnRefusal(t *testing.T) {
	ctx := context.Background()
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q := *p
	q.Hotspot = "nosuchdev"

	r := NewRecordingRunner()
	r.SetFailure("iw dev nosuchdev link", 237, read(t, "capture-pi5-iw-link-nosuchdev-stderr.txt"))

	err := AssertHotspotInterfaceReleased(ctx, r, &q)
	if !errors.Is(err, ErrHotspotInterfaceMissing) {
		t.Fatalf("err = %v, want ErrHotspotInterfaceMissing", err)
	}
	if errors.Is(err, ErrHotspotNotReleased) {
		t.Fatal("a missing interface must not be reported as an interface somebody else is using")
	}
	contains(t, err.Error(), "nosuchdev")
}

// The address readback, and the DOWN state that must not be read as a failure.
//
// MEASURED on the target 2026-08-30, after the NetworkManager release was
// added by hand:
//
//	ip -br addr show ap0test   ->  ap0test    DOWN    10.83.51.1/24
//	python UDP bind 10.83.51.1:67  ->  BIND OK
//
// An access point interface has no carrier until hostapd starts, so it reads
// DOWN while holding a bindable address. A readback that required UP would
// refuse a machine that is working, which is the same class of mistake as
// reading a channel as an association.
func TestAssertHotspotAddressPresent(t *testing.T) {
	ctx := context.Background()
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q := *p
	q.Hotspot = "ap0"
	q.HotspotSubnet = netip.MustParsePrefix("10.83.51.0/24")
	q.HotspotGateway = netip.MustParseAddr("10.83.51.1")

	down := NewRecordingRunner()
	down.SetOutput("ip -br addr show dev ap0", "ap0    DOWN    10.83.51.1/24 \n")
	if err := AssertHotspotAddressPresent(ctx, down, &q); err != nil {
		t.Fatalf("an interface that is DOWN and holds the address passes: %v", err)
	}

	// The measured failure: the address was flushed and nothing noticed until
	// dnsmasq exited 2.
	flushed := NewRecordingRunner()
	flushed.SetOutput("ip -br addr show dev ap0", "ap0    UP    fe80::1c9e:5eff:fe2b:9a1/64 \n")
	err := AssertHotspotAddressPresent(ctx, flushed, &q)
	if !errors.Is(err, ErrHotspotAddressMissing) {
		t.Fatalf("err = %v, want ErrHotspotAddressMissing", err)
	}
	contains(t, err.Error(), "10.83.51.1")
	// UP is not what makes it pass, and the message must not suggest it is.
	notContains(t, err.Error(), "down")

	// Nothing at all on the interface, which is the state the box was left in.
	empty := NewRecordingRunner()
	empty.SetOutput("ip -br addr show dev ap0", "ap0    DOWN    \n")
	err = AssertHotspotAddressPresent(ctx, empty, &q)
	if !errors.Is(err, ErrHotspotAddressMissing) {
		t.Fatalf("err = %v, want ErrHotspotAddressMissing", err)
	}
	contains(t, err.Error(), "nothing")

	// A different address is not the address.
	other := NewRecordingRunner()
	other.SetOutput("ip -br addr show dev ap0", "ap0    UP    10.83.51.9/24 \n")
	if err := AssertHotspotAddressPresent(ctx, other, &q); !errors.Is(err, ErrHotspotAddressMissing) {
		t.Fatalf("err = %v, want ErrHotspotAddressMissing: an address in the subnet is not the box's own address", err)
	}
}
