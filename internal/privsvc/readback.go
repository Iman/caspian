// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"net/netip"

	"caspianbyoc.org/caspian/internal/netcfg"
)

// Reading the machine back, because starting something is not evidence it
// worked.
//
// # What happened without these
//
// MEASURED on the target on 2026-08-30. The service logged
// "running config=dc7370fd uplink=eth0 hotspot=wlan0 tunnel=xray0 channel=1".
// At that moment "iw dev wlan0 info" reported type managed, ssid HomeNet,
// channel 10: a station on the house network, not an access point. A phone in
// the room listed eleven networks and ours was not one of them. hostapd was a
// live process whose control socket did not answer. dnsmasq had bound to
// wlan0 while wlan0 still carried its station address, and the journal holds a
// "DHCPNAK(wlan0) ... wrong server-ID" for an address on that network, which is
// this appliance refusing a real device's lease renewal on a network it does
// not own. The MAC address in that line is a stranger's device and is left out
// here on purpose.
//
// # The check has a history, and this package dropped it
//
// The shell implementation this project replaces already read the type back
// from the kernel: 004-hotspot/xray-hotspot-fixed.sh:426 runs
// "iw dev wlan0 info | grep -q 'type AP'" and prints "WiFi not in AP mode"
// when it does not match. It is the single check that would have caught the
// failure above, it was written months earlier, and the Go rewrite lost it. It
// is worth knowing that this is a recovered check rather than a new idea.
//
// What that script did NOT do is read the name back: the line below its check
// prints a hardcoded SSID string. So the name readback here has no precedent to
// lean on and rests instead on a measurement taken for it, which is recorded in
// Service.assertHotspotIsAccessPoint.
//
// # Where the effects live
//
// internal/netcfg owns interfaces and provides the two primitives. This
// package owns the start sequence, so it decides WHEN they are called and what
// happens when they fail. internal/hotspot is not the caller: it is documented
// as the half that does not detect interfaces and does not query the radio,
// and giving it a netcfg.Runner to do both would undo that split for a check
// that has to happen between two of this package's steps anyway.

// assertHotspotInterfaceReleased proves the hotspot interface is free before
// anything is allowed to bind to it.
//
// A failure here is fatal to the start. The caller's rollback replays the
// netcfg journal, which is what puts the user's WiFi back: every step of the
// release carries its inverse, so a box that could not host a hotspot is still
// a box that rejoins the network it was on.
func (s *Service) assertHotspotInterfaceReleased(ctx context.Context, plan *netcfg.Plan) error {
	err := netcfg.AssertHotspotInterfaceReleased(ctx, s.cfg.Runner, plan)
	if err == nil {
		return nil
	}
	// internal/netcfg's sentence names the interface, the network it is still
	// joined to and the address it still carries. That is the whole diagnosis,
	// and it is the thing that was missing when this happened for real, so it
	// goes to the log and to the advanced view rather than being reduced to a
	// fault word.
	s.reportReadback("the hotspot interface could not be proved free, so nothing was allowed to serve on it",
		plan, err)
	return fail("hotspot interface", faultOf(err), err)
}

// assertHotspotIsAccessPoint proves the interface is an access point and that
// it is broadcasting the name it was given. Both halves are required.
//
// # The name is checked because it was MEASURED to be readable, not assumed
//
// This function shipped for part of a day requiring the name only when the
// kernel gave one, and treating an empty name as a name that could not be
// read. That was the honest shape while the answer was unknown: this tree held
// no capture of "iw dev" listing an interface in AP mode, so requiring the name
// risked refusing a working box and skipping it would have been a check that
// passes for the wrong reason.
//
// The answer is now known. MEASURED on the target on 2026-08-30 by the
// coordinator, kernel 6.18.34, brcmfmac, with a real access point running:
//
//	Interface wlan0
//		ssid Caspian-Probe
//		type AP
//		channel 6 (2437 MHz), width: 20 MHz, center1: 2437 MHz
//
// So this kernel and this driver do report the name for an interface in AP
// mode, and the tolerant branch is gone. An access point with no name, on this
// hardware, is not an access point that could not be read: it is one that is
// not broadcasting, which is the failure the whole readback exists to catch.
//
// THE MEASUREMENT IS OF ONE DRIVER. The next adapter somebody plugs in may not
// report it, and the symptom would be a box that refuses to start with
// "broadcasts \"\", not <name>" while the network is plainly on the air. That
// is a fail-closed symptom with an obvious message, which is why requiring it
// is the right side to be wrong on, and it is written down here so the next
// person recognises it in one reading instead of rediscovering the question.
func (s *Service) assertHotspotIsAccessPoint(ctx context.Context, plan *netcfg.Plan, ssid string) error {
	if err := netcfg.AssertHotspotIsAccessPoint(ctx, s.cfg.Runner, plan, ssid); err != nil {
		s.reportReadback("the access point could not be read back from the kernel", plan, err)
		return fail("hotspot readback", faultOf(err), err)
	}
	return nil
}

// reportReadback puts a failed readback in the log and in the advanced view.
//
// The server address is taken out for the same reason it is everywhere else in
// this package: internal/netcfg puts it in the argument vector of the pinned
// host route, and docs/LAYOUT.md says the user's config is never printed or
// logged. Nothing else in these messages is a credential: an interface name,
// the name of the network the radio is joined to and an address on this
// machine are facts about the box its owner is standing in front of.
func (s *Service) reportReadback(what string, plan *netcfg.Plan, err error) {
	var servers []netip.Addr
	if plan != nil {
		servers = plan.ServerAddr
	}
	detail := redactedText(err.Error(), servers)
	s.cfg.Logger.Error(what, "detail", detail)
	s.diag.add(what + ": " + detail)
}
