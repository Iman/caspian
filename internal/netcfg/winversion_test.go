// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"strings"
	"testing"
)

// The build numbers below are the real ones, so a reader can check them
// against Microsoft's list rather than against this file.
const (
	build1507 uint32 = 10240 // the first Windows 10, no Mobile Hotspot
	build1511 uint32 = 10586
	build1607 uint32 = 14393 // Mobile Hotspot, and the adapter overload
	build1809 uint32 = 17763 // an LTSC still in support and still too old
	build1909 uint32 = 18363 // the last build before the band and DNS calls
	build2004 uint32 = 19041 // band, idle timeout, SetInterfaceDnsSettings
	build22H2 uint32 = 19045
	build11   uint32 = 22621
)

// TestWindowsCapabilityForEveryBuildThatMatters walks the versions a person
// might actually be running and states what each can do. It is a table rather
// than a set of thresholds so that a change to one boundary shows up as a
// change to the builds around it.
func TestWindowsCapabilityForEveryBuildThatMatters(t *testing.T) {
	cases := []struct {
		name                                      string
		build                                     uint32
		hotspot, adapter, band, idle, dns, usable bool
	}{
		{"1507, before Mobile Hotspot existed", build1507, false, false, false, false, false, false},
		{"1511, still before it", build1511, false, false, false, false, false, false},
		{"1607, the hotspot and the adapter choice arrive", build1607, true, true, false, false, false, false},
		{"1809, an LTSC in support and still without the DNS call", build1809, true, true, false, false, false, false},
		{"1909, the last build without it", build1909, true, true, false, false, false, false},
		{"2004, everything Caspian calls", build2004, true, true, true, true, true, true},
		{"22H2", build22H2, true, true, true, true, true, true},
		{"Windows 11", build11, true, true, true, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := WindowsCapabilityFor(tc.build)
			if c.Build != tc.build {
				t.Errorf("Build = %d, want %d", c.Build, tc.build)
			}
			for _, got := range []struct {
				what       string
				have, want bool
			}{
				{"MobileHotspot", c.MobileHotspot, tc.hotspot},
				{"AdapterChoice", c.AdapterChoice, tc.adapter},
				{"BandChoice", c.BandChoice, tc.band},
				{"IdleTimeoutControl", c.IdleTimeoutControl, tc.idle},
				{"DNSPinning", c.DNSPinning, tc.dns},
				{"Usable", c.Usable(), tc.usable},
			} {
				if got.have != got.want {
					t.Errorf("%s = %v, want %v", got.what, got.have, got.want)
				}
			}
		})
	}
}

// TestTheBoundariesAreExact pins each threshold at the build itself and at the
// build below it, because an off-by-one here either refuses a machine that
// works or admits one that cannot resolve a name.
func TestTheBoundariesAreExact(t *testing.T) {
	if WindowsCapabilityFor(BuildMobileHotspot - 1).MobileHotspot {
		t.Error("the build below 14393 is reported as having Mobile Hotspot")
	}
	if !WindowsCapabilityFor(BuildMobileHotspot).MobileHotspot {
		t.Error("14393 is reported as not having Mobile Hotspot")
	}
	if WindowsCapabilityFor(BuildModernTethering - 1).DNSPinning {
		t.Error("the build below 19041 is reported as able to pin DNS")
	}
	if !WindowsCapabilityFor(BuildModernTethering).DNSPinning {
		t.Error("19041 is reported as unable to pin DNS")
	}
	if BuildMobileHotspot >= BuildModernTethering {
		t.Fatal("the thresholds are the wrong way round")
	}
}

// TestAnUnreadableBuildIsTreatedAsCapable states the deliberate choice in the
// unhappy path: a version call that fails must not take the box off the air.
// The calls it guards still fail on their own if the guess was wrong, and
// they fail with the reason rather than with silence.
func TestAnUnreadableBuildIsTreatedAsCapable(t *testing.T) {
	c := WindowsCapabilityFor(0)
	if !c.Usable() || !c.BandChoice || !c.IdleTimeoutControl || !c.AdapterChoice {
		t.Errorf("an unknown build is refused: %+v", c)
	}
	if c.Build != 0 {
		t.Errorf("Build = %d, want the zero that was passed in", c.Build)
	}
}

// TestTheRefusalNamesTheVersionAndNotTheMachine guards the wording. The
// sentence a person eventually reads comes from the panel catalogue, but the
// error this package returns is what a maintainer reads in a log, and it has
// to say which version is wanted rather than that something is unsupported.
func TestTheRefusalNamesTheVersionAndNotTheMachine(t *testing.T) {
	msg := ErrWindowsTooOld.Error()
	for _, want := range []string{"2004", "DNS"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not mention %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "unsupported") {
		t.Errorf("the refusal calls the platform unsupported, which sends the person to the wrong remedy: %s", msg)
	}
}
