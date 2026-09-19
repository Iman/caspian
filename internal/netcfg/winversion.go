// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"errors"
	"fmt"
)

// This file holds what each Windows 10 build can and cannot do, as a plain
// function of the build number, so that the decision is readable and testable
// on any machine. The one line that asks Windows for its build number is in
// winversion_windows.go and does nothing else.
//
// # Why a build number decides anything
//
// Mobile Hotspot is not one API that has always existed. It arrived in 1607
// and grew afterwards, and Caspian calls three things that came later than
// the feature itself. Until this file existed, a box on an older Windows
// installed happily, failed at the first call, and was told to restart, which
// is the one action that cannot help: the build number does not change.
//
// The measured facts, from learn.microsoft.com on 2026-09-09:
//
//	1607 / 14393  Mobile Hotspot arrives. NetworkOperatorTetheringManager
//	              gains CreateFromConnectionProfile(profile, adapter), which
//	              is how a box with two radios says which one hosts.
//	2004 / 19041  TetheringWiFiBand and the Band property arrive, so a band
//	              can be chosen and reported. IsNoConnectionsTimeoutEnabled
//	              and DisableNoConnectionsTimeoutAsync arrive, so the hotspot
//	              can be kept up with nobody joined. SetInterfaceDnsSettings
//	              arrives in the IP Helper API, which is how the tunnel
//	              adapter gets a resolver.
//
// The first three are conveniences: without them Windows picks the radio, the
// band is whatever Windows chose, and the hotspot stops five minutes after
// the last device leaves. The fourth is not a convenience, and it is why the
// working floor is 2004 rather than 1607. See DNSPinning below.
const (
	// BuildMobileHotspot is Windows 10 version 1607, the first build with
	// Mobile Hotspot at all. Below this there is no hotspot to drive.
	BuildMobileHotspot uint32 = 14393

	// BuildModernTethering is Windows 10 version 2004, the first build with
	// the band control, the idle-timeout control and the DNS call.
	BuildModernTethering uint32 = 19041
)

// ErrWindowsTooOld is returned when the box is running a Windows that cannot
// direct DNS through the tunnel.
//
// It is deliberately not "unsupported platform": the platform is supported,
// this copy of it is too old, and the person can act on that by updating
// Windows. Nothing else in the tree returns it.
var ErrWindowsTooOld = errors.New("netcfg: this Windows is older than version 2004, which is the first with the call that points DNS at the tunnel")

// WindowsCapability is what one build of Windows can do.
//
// Every field is a thing Caspian would otherwise call unconditionally, and
// every false is a call that would throw at run time. It is a plain struct of
// bools rather than a build number carried around, because a caller asking
// "can I choose the band" should not have to know which build introduced it.
type WindowsCapability struct {
	// Build is the number this was decided from, kept so a message can name
	// it and a log line can record what was seen.
	Build uint32

	// MobileHotspot is whether Windows has the feature at all.
	MobileHotspot bool

	// AdapterChoice is whether the hotspot can be pinned to a named radio.
	// Without it Windows picks, which is wrong on a box with two.
	AdapterChoice bool

	// BandChoice is whether the 2.4 and 5 GHz bands can be chosen and
	// reported. Without it the band control is a switch with nothing behind
	// it, so the panel must not offer one.
	BandChoice bool

	// IdleTimeoutControl is whether the hotspot can be kept up with nobody
	// joined. Without it Windows stops it five minutes after the last device
	// leaves, and the person finds the network gone with no explanation.
	IdleTimeoutControl bool

	// DNSPinning is whether the tunnel adapter can be given a resolver.
	//
	// This is the one that decides whether Caspian runs at all. The Windows
	// Filtering Platform rules block every DNS query that does not leave
	// through the tunnel adapter, which is what stops a leak, and those
	// filters work on every build back to Vista. But with no resolver ON the
	// tunnel adapter, every query is blocked and none is answered: the box
	// would be fail-closed and useless rather than fail-closed and working.
	//
	// There is no second way to do it in this tree. The IP Helper call is the
	// only DNS mechanism the Windows backend has; netsh is not on the binary
	// allowlist and adding it would be a different change with its own
	// evidence. So a build without this is refused rather than started.
	DNSPinning bool
}

// Usable reports whether Caspian can run on this build at all.
func (c WindowsCapability) Usable() bool { return c.MobileHotspot && c.DNSPinning }

// CheckSupport reports an actionable refusal without changing the machine.
func (c WindowsCapability) CheckSupport() error {
	if !c.Usable() {
		return fmt.Errorf("%w (build %d)", ErrWindowsTooOld, c.Build)
	}
	return nil
}

// WindowsCapabilityFor answers what a build can do.
//
// A build of zero means the number could not be read, and is treated as
// capable: refusing to start because a version call failed would turn a
// diagnostic problem into an outage, and the calls themselves still fail
// honestly if the guess was wrong.
func WindowsCapabilityFor(build uint32) WindowsCapability {
	if build == 0 {
		return WindowsCapability{
			Build: 0, MobileHotspot: true, AdapterChoice: true,
			BandChoice: true, IdleTimeoutControl: true, DNSPinning: true,
		}
	}
	c := WindowsCapability{Build: build}
	c.MobileHotspot = build >= BuildMobileHotspot
	c.AdapterChoice = build >= BuildMobileHotspot
	c.BandChoice = build >= BuildModernTethering
	c.IdleTimeoutControl = build >= BuildModernTethering
	c.DNSPinning = build >= BuildModernTethering
	return c
}
