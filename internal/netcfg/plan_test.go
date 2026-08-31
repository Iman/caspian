// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// Mode A of the design: the internet arrives on the wired port and the hotspot
// runs on the built-in radio.
func TestPlan_ModeA_WiredUplink(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())

	if p.Mode != ModeWiredUplink {
		t.Errorf("mode = %v, want %v", p.Mode, ModeWiredUplink)
	}
	if p.Uplink != "eth0" {
		t.Errorf("uplink = %q, want eth0 (it carries the lowest-metric default route)", p.Uplink)
	}
	if p.UplinkGateway != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("gateway = %v", p.UplinkGateway)
	}
	if p.HotspotPhy != "phy0" {
		t.Errorf("hotspot radio = %q, want phy0", p.HotspotPhy)
	}
}

// The radio on the target is already associated on channel 10 and reports
// "#channels <= 1". An access point on it therefore has to be a second
// interface on the same radio, pinned to that channel. Neither half of that is
// assumed: both are read from iw output.
func TestPlan_ModeA_APIsASecondInterfacePinnedToTheStationChannel(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())

	if !p.HotspotIsVirtual {
		t.Fatalf("hotspot %q should be an interface created on the radio, not the associated one", p.Hotspot)
	}
	if p.Hotspot != "ap0" || p.HotspotParent != "wlan0" {
		t.Errorf("hotspot/parent = %q/%q, want ap0/wlan0", p.Hotspot, p.HotspotParent)
	}
	if !p.ChannelPinned || p.Channel != 10 {
		t.Errorf("channel = %d pinned=%v, want 10 pinned (the radio allows one channel and wlan0 is on 10)",
			p.Channel, p.ChannelPinned)
	}
	joined := ""
	for _, n := range p.Notes {
		joined += n + "\n"
	}
	contains(t, joined, "#channels <= 1")
	contains(t, joined, "pinned to channel 10")
	contains(t, joined, "roams")
}

// Mode B: the internet arrives over the built-in radio and the hotspot runs on
// a USB adapter. The adapter's radio has no station link, so the access point
// owns it and picks its own channel.
func TestPlan_ModeB_USBAdapter(t *testing.T) {
	_, p := mustPlan(t, modeBScenario(), DefaultOptions())

	if p.Mode != ModeWirelessUplink {
		t.Errorf("mode = %v, want %v", p.Mode, ModeWirelessUplink)
	}
	if p.Uplink != "wlan0" {
		t.Errorf("uplink = %q, want wlan0", p.Uplink)
	}
	if p.Hotspot != "wlan1" || p.HotspotPhy != "phy1" {
		t.Errorf("hotspot = %q on %q, want wlan1 on phy1", p.Hotspot, p.HotspotPhy)
	}
	if p.HotspotIsVirtual {
		t.Error("the USB radio has no station link, so no extra interface is needed")
	}
	if p.ChannelPinned {
		t.Error("a radio with no station link does not pin the channel")
	}
	if p.Channel != 1 {
		t.Errorf("channel = %d, want the first usable channel 1", p.Channel)
	}
}

// The USB adapter is preferred over the built-in radio in mode B because it is
// free, and the bus is only ever used to break a tie.
func TestPlan_PrefersTheRadioWithoutAStationLink(t *testing.T) {
	_, p := mustPlan(t, modeBScenario(), DefaultOptions())
	if p.HotspotPhy == "phy0" {
		t.Errorf("chose the radio carrying the uplink (%s) over the free USB radio", p.HotspotPhy)
	}
}

func TestPlan_RefusesWithNoUplink(t *testing.T) {
	s := modeAScenario()
	s.route = "scenario-ip-route-default-none.txt"
	s.route6 = ""
	f := s.facts(t, BaseSysctlKnobs())

	_, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if !errors.Is(err, ErrNoUplink) {
		t.Fatalf("err = %v, want ErrNoUplink", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("refusal must carry wording for the panel")
	}
	contains(t, pe.UserMessage(), "no internet connection")
	notContains(t, pe.UserMessage(), "default route")
}

// "No AP-capable phy" is not an error message a person can act on. The design
// asks for "No adapter on this machine can create a hotspot. Plug in a USB
// WiFi adapter."
func TestPlan_RefusesWithNoAPCapableRadio(t *testing.T) {
	s := modeAScenario()
	s.iwlist = "scenario-iw-list-noap-only.txt"
	f := s.facts(t, BaseSysctlKnobs())

	_, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if !errors.Is(err, ErrNoAPCapableInterface) {
		t.Fatalf("err = %v, want ErrNoAPCapableInterface", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("refusal must carry wording for the panel")
	}
	if pe.UserMessage() != "No adapter on this machine can create a hotspot. Plug in a USB WiFi adapter." {
		t.Errorf("user message = %q", pe.UserMessage())
	}
	notContains(t, pe.UserMessage(), "phy")
	notContains(t, pe.UserMessage(), "AP")
}

// A radio that supports AP but never at the same time as a station, while it
// is carrying the uplink, is a different situation with a different remedy.
func TestPlan_RefusesWhenTheOnlyRadioCannotDoBoth(t *testing.T) {
	s := modeAScenario()
	s.route = "scenario-modeb-ip-route-default.txt"
	s.iwlist = "scenario-iw-list-no-concurrency.txt"
	f := s.facts(t, BaseSysctlKnobs())

	_, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if !errors.Is(err, ErrAPConflictsWithUplink) {
		t.Fatalf("err = %v, want ErrAPConflictsWithUplink", err)
	}
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatal("refusal must carry wording for the panel")
	}
	contains(t, pe.UserMessage(), "USB WiFi adapter")
	contains(t, pe.UserMessage(), "cable")
}

func TestPlan_RefusesWithNoServerAddress(t *testing.T) {
	f := modeAScenario().facts(t, BaseSysctlKnobs())
	_, err := PlanNetwork(f, nil, DefaultOptions())
	if !errors.Is(err, ErrNoServerAddress) {
		t.Fatalf("err = %v, want ErrNoServerAddress", err)
	}
}

// Every automatic choice must be overridable, because detection that cannot be
// overridden fails silently on the one machine that does not match.
func TestPlan_Overrides(t *testing.T) {
	f := modeAScenario().facts(t, BaseSysctlKnobs())
	o := DefaultOptions()
	o.UplinkOverride = "wlan0"
	o.HotspotSubnet = netip.MustParsePrefix("10.99.99.0/24")
	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Uplink != "wlan0" {
		t.Errorf("uplink override ignored: %q", p.Uplink)
	}
	if p.HotspotSubnet != netip.MustParsePrefix("10.99.99.0/24") {
		t.Errorf("subnet override ignored: %v", p.HotspotSubnet)
	}
	if p.HotspotGateway != netip.MustParseAddr("10.99.99.1") {
		t.Errorf("gateway = %v, want 10.99.99.1", p.HotspotGateway)
	}
}

// An override that collides is honoured and reported, not silently corrected.
// The user asked for it; the note is how they find out it was a bad idea.
func TestPlan_OverriddenSubnetThatCollidesIsReported(t *testing.T) {
	f := modeAScenario().facts(t, BaseSysctlKnobs())
	o := DefaultOptions()
	o.HotspotSubnet = netip.MustParsePrefix("192.168.1.0/24") // the network eth0 is on
	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Notes) == 0 {
		t.Fatal("a colliding hand-set subnet must be reported in the notes")
	}
	joined := ""
	for _, n := range p.Notes {
		joined += n + "\n"
	}
	contains(t, joined, "overlaps 192.168.1.0/24")
}

func TestPlan_RefusesAnUnusableTunnelName(t *testing.T) {
	f := modeAScenario().facts(t, BaseSysctlKnobs())
	o := DefaultOptions()
	o.TunName = "a name with spaces"
	if _, err := PlanNetwork(f, []netip.Addr{testServer}, o); err == nil {
		t.Fatal("an implausible tunnel device name must be refused before it reaches a command")
	}
}

func TestPlan_Explain(t *testing.T) {
	_, a := mustPlan(t, modeAScenario(), DefaultOptions())
	got := a.Explain()
	// The channel is fixed by wlan0's existing association, and wlan0 is not
	// the uplink here, so the sentence must not call it the internet
	// connection.
	want := "Internet: wired connection on eth0. Hotspot: WiFi on ap0, a second connection on the same radio as wlan0 (phy0), channel 10, fixed by the radio to match the WiFi connection already on wlan0." +
		" Devices on the hotspot can reach only the services this box offers them; how you reach this machine from eth0 is not changed." +
		" While the hotspot is on, programs on this box cannot reach the internet directly, so updating its software will not work until you switch it off."
	if got != want {
		t.Errorf("Explain()\n got: %s\nwant: %s", got, want)
	}

	_, b := mustPlan(t, modeBScenario(), DefaultOptions())
	got = b.Explain()
	want = "Internet: WiFi on wlan0. Hotspot: WiFi on wlan1 (phy1), channel 1." +
		" Devices on the hotspot can reach only the services this box offers them; how you reach this machine from wlan0 is not changed." +
		" While the hotspot is on, programs on this box cannot reach the internet directly, so updating its software will not work until you switch it off."
	if got != want {
		t.Errorf("Explain()\n got: %s\nwant: %s", got, want)
	}
}

// Invariants that must hold on every machine this package models, captured or
// authored. A suite whose cross-cutting checks run against one machine passes
// on that machine and misleads everywhere else, which is exactly what the
// captured bytes exposed.
func TestPlan_InvariantsHoldOnEveryModelledMachine(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			f, p := mustPlan(t, sc, DefaultOptions())

			if p.Uplink == "" || !ValidInterfaceName(p.Uplink) {
				t.Errorf("uplink = %q", p.Uplink)
			}
			if p.Hotspot == "" || !ValidInterfaceName(p.Hotspot) {
				t.Errorf("hotspot = %q", p.Hotspot)
			}
			if p.Hotspot == p.Uplink {
				t.Errorf("the hotspot and the uplink are the same interface %q", p.Hotspot)
			}
			if p.Mode == ModeUnset {
				t.Error("mode was never decided")
			}
			if p.Channel <= 0 {
				t.Errorf("channel = %d, want one the radio reported as usable", p.Channel)
			}

			// The chosen subnets must collide with nothing the box is on, and
			// not with each other.
			for _, taken := range f.OccupiedPrefixes() {
				if Overlaps(p.HotspotSubnet, taken) {
					t.Errorf("hotspot subnet %v overlaps %v, which this machine is already on", p.HotspotSubnet, taken)
				}
				if Overlaps(p.TunSubnet, taken) {
					t.Errorf("tunnel subnet %v overlaps %v", p.TunSubnet, taken)
				}
			}
			if Overlaps(p.HotspotSubnet, p.TunSubnet) {
				t.Errorf("hotspot %v overlaps tunnel %v", p.HotspotSubnet, p.TunSubnet)
			}

			// Every generated command must pass the allowlist, and every
			// change that is not explicitly inverse-free must have an inverse.
			steps := p.AllSteps(f.Sysctl)
			if len(steps) == 0 {
				t.Fatal("no steps generated")
			}
			for _, st := range steps {
				if err := ValidateCommand(st.Do); err != nil {
					t.Errorf("step %s: %v", st.Op, err)
				}
				if st.Undo.IsZero() {
					// Two sanctioned inverse-free steps, both named exactly
					// rather than matched by substring, and both explained in
					// their own comment where they are generated:
					//
					//   - bringing the hotspot link up, because taking a radio
					//     back down on teardown is worse than leaving it up;
					//   - releasing a CREATED interface from NetworkManager,
					//     because the interface is removed by the inverse of
					//     the step that made it and a reboot destroys it, so
					//     there is nothing left to give management back to.
					//
					// The takeover's nmcli release is a different step on a
					// device the user owns, and it is NOT sanctioned here: it
					// must keep its inverse or the box never rejoins their
					// WiFi.
					sanctioned := map[string]bool{
						"ip link set dev " + p.Hotspot + " up": true,
					}
					if p.HotspotIsVirtual {
						sanctioned["nmcli device set "+p.Hotspot+" managed no"] = true
					}
					if !sanctioned[RunnerKey(st.Do)] {
						t.Errorf("step %s has no inverse: %s", st.Op, RunnerKey(st.Do))
					}
					continue
				}
				if err := ValidateCommand(st.Undo); err != nil {
					t.Errorf("inverse of %s: %v", st.Op, err)
				}
			}

			// The one line the panel shows must name the interfaces it chose.
			e := p.Explain()
			contains(t, e, p.Uplink)
			contains(t, e, p.Hotspot)
			if !strings.HasSuffix(e, ".") {
				t.Errorf("Explain() is not a sentence: %q", e)
			}
		})
	}
}
