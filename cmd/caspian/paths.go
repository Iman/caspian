// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/state"
)

// ---------------------------------------------------------------------------
// Everything docs/LAYOUT.md fixes, in one place.
//
// LAYOUT.md, "Ports": "cmd/caspian reads these and passes them in. No package
// hardcodes a value it does not own." This file is that reading. Every value
// below is either taken from the package that owns it, or, where LAYOUT.md
// fixes it and no package owns it, written here once and passed in.
//
// TestLayoutValuesMatchTheDocument checks each one against docs/LAYOUT.md, so a
// change to the document that is not made here is a failing test rather than a
// box that comes up on the wrong port.
// ---------------------------------------------------------------------------

const (
	// serviceAccount is the system account the panel runs as, and one of the
	// two accounts permitted to drive the privileged service.
	// LAYOUT.md, "Names": "Service user and group | caspian".
	serviceAccount = "caspian"
	serviceGroup   = "caspian"

	// socketPath is the panel-to-privileged socket. LAYOUT.md, "Paths":
	// /run/caspian/priv.sock, 0660, root:caspian.
	socketPath = "/run/caspian/priv.sock"

	// firstRunPasswordPath is where the installer leaves the plaintext panel
	// password for the panel to consume. docs/INSTALL.md, "The handoff".
	firstRunPasswordPath = "/var/lib/caspian/first-run-password"
)

// stateDir is the persistent state directory. It is internal/state's constant
// rather than a copy, because that package owns the file in it.
const stateDir = state.DefaultDir

// journalPath is the teardown journal. It is internal/netcfg's constant rather
// than a copy: LAYOUT.md and that package agreed on the name on 2026-08-30, and
// a second spelling here is how they would come to disagree again.
const journalPath = netcfg.DefaultJournalPath

// The ports, from docs/LAYOUT.md, "Ports".
const (
	// dnsPort is dnsmasq on the hotspot interface: DHCP and DNS for joined
	// devices.
	dnsPort = 53

	// localDNSPort is the engine's local DNS listener on 127.0.0.1, and
	// dnsmasq's only permitted upstream.
	//
	// THIS IS THE PAIRING THAT BREAKS QUIETLY. If the two ends drift, DNS stops
	// resolving for every joined device while the hotspot and the tunnel both
	// look healthy. It is passed to internal/privsvc as ONE value, which gives
	// it to internal/xcfg to listen on and to internal/hotspot to forward to.
	localDNSPort = 5354

	// panelPort is the web panel.
	//
	// It is 8088, which is what docs/LAYOUT.md fixes and what
	// internal/netcfg/plan.go opens on the hotspot interface. It is NOT
	// internal/panel's DefaultPort, which was 8080 until 2026-08-30 and is now
	// 8088, matching docs/LAYOUT.md; see the report accompanying
	// this command. LAYOUT.md is binding, so the value is passed in from here
	// and that constant is never used.
	panelPort = 8088

	// socksPort is the loopback diagnostics inbound, used for the exit-IP
	// proof.
	socksPort = 10808
)

// hotspotPaths are where hostapd and dnsmasq keep their files.
//
// internal/hotspot's DefaultPaths already matches docs/LAYOUT.md, including the
// reason dnsmasq gets a directory of its own, so they are taken from there
// rather than restated.
func hotspotPaths() hotspot.Paths { return hotspot.DefaultPaths() }
