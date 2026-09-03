// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build darwin

package main

// platformLayout is the macOS table. Apple's conventions rather than the
// FHS: daemon state under /Library/Application Support, the runtime directory
// under /var/run (cleared at boot like /run), a hidden system account with a
// leading underscore, and launchd labels in reverse-DNS form of the module.
//
// docs/LAYOUT.md gains a macOS table with the port; until then this is the
// one statement of these values.
func platformLayout() layoutTable {
	const stateDir = "/Library/Application Support/Caspian"
	return layoutTable{
		ServiceAccount:        "_caspian",
		ServiceGroup:          "_caspian",
		PrivEndpoint:          "/var/run/caspian/priv.sock",
		RuntimeDir:            "/var/run/caspian",
		StateDir:              stateDir,
		FirstRunPasswordPath:  stateDir + "/first-run-password",
		JournalPath:           stateDir + "/netcfg.journal",
		BinaryPath:            "/usr/local/bin/caspian",
		ServiceManager:        "launchd",
		StartPrivilegedAdvice: "sudo launchctl kickstart -k system/org.caspianbyoc.caspian",
		StopPrivilegedAdvice:  "sudo launchctl bootout system/org.caspianbyoc.caspian",
	}
}
