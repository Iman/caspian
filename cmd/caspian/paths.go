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
// The values differ per operating system and the ports do not. The paths, the
// account and the service manager's verbs live in a layoutTable, filled by
// platformLayout in paths_linux.go, paths_darwin.go and paths_windows.go. The
// Linux table is the one docs/LAYOUT.md documents today and the one the tests
// read; TestLayoutPathsMatchTheDocument checks it against the document on
// every platform, so a change to the document that is not made here is a
// failing test rather than a box that comes up on the wrong path.
// ---------------------------------------------------------------------------

// layoutTable is what docs/LAYOUT.md fixes for one operating system.
type layoutTable struct {
	// ServiceAccount is the account the panel runs as, and one of the two
	// accounts permitted to drive the privileged service.
	ServiceAccount string
	ServiceGroup   string

	// PrivEndpoint is the panel-to-privileged endpoint: a unix socket path on
	// Linux and macOS, a named pipe on Windows.
	PrivEndpoint string

	// RuntimeDir holds the endpoint and the generated hotspot files on the
	// platforms that have such a directory; empty where there is none.
	RuntimeDir string

	// StateDir is the persistent state directory. FirstRunPasswordPath and
	// JournalPath are inside it.
	StateDir             string
	FirstRunPasswordPath string
	JournalPath          string

	// BinaryPath is where the installer puts this binary.
	BinaryPath string

	// ServiceManager names what starts the two roles, and the two advice
	// strings are the verbs a person types to start or stop the privileged
	// service by hand.
	ServiceManager        string
	StartPrivilegedAdvice string
	StopPrivilegedAdvice  string
}

// layout is the table for the platform this binary was built for.
var layout = platformLayout()

// The names the rest of this command has always used, now read from the
// table. Keeping them means the code that passes the values in did not change
// when the values gained a second and third platform.
var (
	serviceAccount       = layout.ServiceAccount
	serviceGroup         = layout.ServiceGroup
	socketPath           = layout.PrivEndpoint
	firstRunPasswordPath = layout.FirstRunPasswordPath
	stateDir             = layout.StateDir
	journalPath          = layout.JournalPath
)

// linuxLayout is the table docs/LAYOUT.md documents. It is a function on every
// platform, not only Linux, because the tests compare it with the document
// wherever they run.
func linuxLayout() layoutTable {
	return layoutTable{
		// LAYOUT.md, "Names": "Service user and group | caspian".
		ServiceAccount: "caspian",
		ServiceGroup:   "caspian",
		// LAYOUT.md, "Paths": /run/caspian/priv.sock, 0660, root:caspian.
		PrivEndpoint: "/run/caspian/priv.sock",
		RuntimeDir:   "/run/caspian",
		// internal/state's constant rather than a copy, because that package
		// owns the file in it.
		StateDir: state.DefaultDir,
		// docs/INSTALL.md, "The handoff".
		FirstRunPasswordPath: "/var/lib/caspian/first-run-password",
		// internal/netcfg's constant rather than a copy: LAYOUT.md and that
		// package agreed on the name on 2026-08-30, and a second spelling here
		// is how they would come to disagree again.
		JournalPath:           netcfg.DefaultJournalPath,
		BinaryPath:            "/usr/local/bin/caspian",
		ServiceManager:        "systemd",
		StartPrivilegedAdvice: "systemctl start caspian.service",
		StopPrivilegedAdvice:  "systemctl stop caspian.service",
	}
}

// The ports, from docs/LAYOUT.md, "Ports". The same on every platform.
const (
	// dnsPort is DHCP and DNS for joined devices on the hotspot interface:
	// dnsmasq on Linux, the operating system's own server elsewhere.
	dnsPort = 53

	// localDNSPort is the engine's local DNS listener on 127.0.0.1, and the
	// only upstream client DNS is forwarded or redirected to.
	//
	// THIS IS THE PAIRING THAT BREAKS QUIETLY. If the two ends drift, DNS stops
	// resolving for every joined device while the hotspot and the tunnel both
	// look healthy. It is passed to internal/privsvc as ONE value, which gives
	// it to internal/xcfg to listen on and to the hotspot side to forward to.
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

// hotspotPaths are where hostapd and dnsmasq keep their files on Linux.
//
// internal/hotspot's DefaultPaths already matches docs/LAYOUT.md, including the
// reason dnsmasq gets a directory of its own, so they are taken from there
// rather than restated. On the platforms whose access point is the operating
// system's, these paths are passed in and never used.
func hotspotPaths() hotspot.Paths { return hotspot.DefaultPaths() }
