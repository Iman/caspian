// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

// An explicit band is honoured or REFUSED. It is never replaced.
//
// The failure this exists for, measured on 2026-08-30: the plan produced a
// channel, internal/privsvc took the band from the user's request and the
// channel from the plan without checking that they agree, and hostapd was
// given hw_mode=g with channel 36. The start failed as "the hotspot failed",
// with nothing pointing at the band. The other direction is worse and also
// measured: a hotspot that came up on 5GHz channel 36 was invisible to the
// user's handset, whose scan returns 2412 to 2462 MHz only, while the panel
// correctly said it was up and broadcasting.
func TestBand_AnExplicitBandIsHonouredOnARadioThatHasIt(t *testing.T) {
	for _, tc := range []struct {
		band     RadioBand
		wantMin  int
		wantMax  int
		bandName string
	}{
		{Band2GHz, 1, 14, "2.4GHz"},
		{Band5GHz, 32, 177, "5GHz"},
	} {
		t.Run(string(tc.band), func(t *testing.T) {
			f := freeBuiltInRadio(t)
			o := DefaultOptions()
			o.HotspotBand = tc.band
			p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if p.Channel < tc.wantMin || p.Channel > tc.wantMax {
				t.Fatalf("channel = %d, which is not in %s", p.Channel, tc.bandName)
			}
			// And the radio itself must agree, rather than the test's own
			// idea of which numbers belong to which band.
			phy, ok := f.PhyByName(p.HotspotPhy)
			if !ok {
				t.Fatalf("the plan names a radio %q that is not in the facts", p.HotspotPhy)
			}
			if got := phy.BandOfChannel(p.Channel); got != tc.band {
				t.Fatalf("the radio says channel %d is %s, not %s", p.Channel, got, tc.band)
			}
		})
	}
}

// A radio with no channel in the band the user asked for is a refusal that
// names the band, not a channel from the other band.
func TestBand_ARadioWithoutTheBandIsARefusal(t *testing.T) {
	f := freeBuiltInRadio(t)
	// Strip the 5GHz frequencies from the radio, leaving a 2.4GHz-only
	// adapter, which is what most cheap USB adapters are.
	for i := range f.Phys {
		var kept []Band
		for _, b := range f.Phys[i].Bands {
			var freqs []Frequency
			for _, fr := range b.Frequencies {
				if BandOf(fr.MHz) == Band2GHz {
					freqs = append(freqs, fr)
				}
			}
			if len(freqs) > 0 {
				b.Frequencies = freqs
				kept = append(kept, b)
			}
		}
		f.Phys[i].Bands = kept
	}

	o := DefaultOptions()
	o.HotspotBand = Band5GHz
	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err == nil {
		t.Fatalf("planned channel %d on a 2.4GHz-only radio for a 5GHz request", p.Channel)
	}
	if !errors.Is(err, ErrBandUnavailable) {
		t.Fatalf("err = %v, want ErrBandUnavailable", err)
	}
	contains(t, err.Error(), "5GHz")

	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %T, want a PlanError carrying a sentence for the user", err)
	}
	contains(t, pe.User, "5GHz")
	notContains(t, pe.User, "phy")
}

// A pinned channel outside the requested band is the case that produced the
// contradiction. The pin cannot move and the band cannot be replaced, so the
// only honest answer is a refusal that names both.
func TestBand_APinOutsideTheRequestedBandIsARefusal(t *testing.T) {
	sc := pi5Captured()
	r := sc.runner(t)
	r.SetOutput("iw dev wlan0 link", read(t, "capture-pi5-iw-link-connected.txt"))
	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}

	// The station is on channel 10, which is 2.4GHz, and the radio reports
	// "#channels <= 1", so the access point is pinned there.
	o := DefaultOptions()
	o.HotspotBand = Band5GHz
	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err == nil {
		t.Fatalf("planned channel %d for a 5GHz request while pinned to the station", p.Channel)
	}
	if !errors.Is(err, ErrBandUnavailable) {
		t.Fatalf("err = %v, want ErrBandUnavailable", err)
	}
	// The refusal names the band asked for, the band it is stuck on, and the
	// network responsible, because those are the three things the user needs
	// to decide what to do.
	var pe *PlanError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %T, want a PlanError", err)
	}
	contains(t, pe.User, "5GHz")
	contains(t, pe.User, "2.4GHz")
	contains(t, pe.User, "HomeNet")

	// And the same machine with the band it IS on plans normally.
	o.HotspotBand = Band2GHz
	if p, err := PlanNetwork(f, []netip.Addr{testServer}, o); err != nil {
		t.Fatalf("2.4GHz was refused on a radio pinned to a 2.4GHz channel: %v", err)
	} else if p.Channel != 10 {
		t.Fatalf("channel = %d, want the pinned 10", p.Channel)
	}
}

// With no band asked for, 2.4GHz wins when the radio has both.
//
// REACH, not speed. This is a deliberate default and not a consequence of
// sorting channel numbers, which is what it used to be. A 5GHz hotspot is
// faster and is invisible to a handset that only scans 2.4GHz, and the box
// cannot tell the difference: it reports a working access point either way.
func TestBand_AutoPrefers24ForReach(t *testing.T) {
	f := freeBuiltInRadio(t)
	p, err := PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	phy, _ := f.PhyByName(p.HotspotPhy)
	if len(phy.UsableChannelsIn(Band5GHz)) == 0 {
		t.Fatal("this radio has no 5GHz channels, so the preference is not being tested")
	}
	if got := phy.BandOfChannel(p.Channel); got != Band2GHz {
		t.Fatalf("channel %d is %s; with no band asked for, reach beats speed", p.Channel, got)
	}
}

// The band is classified from the FREQUENCY the radio reported, not from the
// channel number, because the numbering has seams.
func TestBand_ClassifiedByFrequencyNotChannelNumber(t *testing.T) {
	cases := []struct {
		mhz  int
		want RadioBand
	}{
		{2412, Band2GHz},
		{2484, Band2GHz}, // channel 14, outside a naive 1-13 range
		{5180, Band5GHz}, // channel 36
		{5825, Band5GHz},
		{5955, BandAuto}, // 6GHz, where channel numbering restarts at 1
		{900, BandAuto},
	}
	for _, c := range cases {
		if got := BandOf(c.mhz); got != c.want {
			t.Errorf("BandOf(%d) = %q, want %q", c.mhz, got, c.want)
		}
	}
}

// freeBuiltInRadio is the captured Pi 5 radio with nothing on it, which is the
// machine the band decisions are made against.
func freeBuiltInRadio(t *testing.T) Facts {
	t.Helper()
	phys, err := ParseIwList(read(t, "capture-pi5-iw-list.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return Facts{
		Phys: phys,
		Wireless: []WirelessIface{
			{Name: "wlan0", Phy: phys[0].Name, Type: "managed", LinkKnown: true, Associated: false},
		},
		Routes: []DefaultRoute{{
			Family: 4, Dev: "eth0", Metric: 100,
			Gateway: netip.MustParseAddr("192.168.1.1"), Src: netip.MustParseAddr("192.168.1.50"),
		}},
		Links: []Link{
			{Name: "eth0", State: "UP", Prefixes: []netip.Prefix{netip.MustParsePrefix("192.168.1.50/24")}},
			{Name: "wlan0", State: "DOWN"},
		},
		Sysctl: map[string]string{},
	}
}
