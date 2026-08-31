// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Copyright (C) 2026 Iman Samizadeh
//
// This file is part of Caspian-BYOC.

// Package bdd holds the behaviour suite: the tests that describe what the
// product does, as opposed to what each package does.
//
// # Why it exists
//
// Every package under internal/ is unit tested in isolation and well covered.
// None of those tests says "a person pastes a config and the devices on the
// hotspot reach the internet through the tunnel", and none says "when the
// tunnel drops, nothing leaks". Those are the two claims the product makes.
// They are claims about the COMPOSITION of the packages, so no test that lives
// inside one package can make them.
//
// # What it drives
//
// Everything runs in memory. There is no root, no radio, no /dev/net/tun and
// no network. The substitutions are the ones the packages already ship for
// their own use, at the system boundary and nowhere above it:
//
//	internal/netcfg   RecordingRunner in place of the "ip"/"iw"/"nft" executor
//	internal/hotspot  Recorder in place of the System interface
//	internal/engine   the real engine, in process, with the TUN inbound off
//	internal/state    a real Store over a temporary directory
//	internal/link     the real parser
//	internal/xcfg     the real config composer
//
// The engine is real, not faked. It is the same xray-core build the appliance
// links, loading the same document through the same loader, so "the engine
// accepted this config" means what it says. What it does not do here is create
// a tunnel device, because the TUN inbound is switched off; see the comment on
// appliance.connect.
//
// # What it cannot prove, stated once here rather than implied everywhere
//
// No scenario in this package captures an exit IP, and none can. The design's
// rule is that nothing is called working without an exit IP captured from real
// traffic (docs/2026-08-29-design.md section 6). Everything here is one layer
// below that: it proves the box is CONFIGURED to carry traffic only through
// the tunnel, and that the block survives the tunnel going away. It does not
// prove a packet went anywhere. Read docs/BEHAVIOUR.md for the list of
// behaviours that are proven and the list of behaviours that are not.
package bdd
