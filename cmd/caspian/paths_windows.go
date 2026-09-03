// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// platformLayout is the Windows table. State lives under ProgramData, the
// panel-to-privileged endpoint is a named pipe (Windows AF_UNIX sockets carry
// no peer credentials, so a pipe with a security descriptor takes their
// place), and the panel runs under a virtual service account that exists
// without being created.
//
// ProgramData is read from the environment at start because its location is
// not fixed by Windows; the fallback is the default on every supported
// installation.
func platformLayout() layoutTable {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	stateDir := filepath.Join(base, "Caspian")
	programs := os.Getenv("ProgramFiles")
	if programs == "" {
		programs = `C:\Program Files`
	}
	return layoutTable{
		ServiceAccount:        `NT SERVICE\caspian-panel`,
		ServiceGroup:          `NT SERVICE\caspian-panel`,
		PrivEndpoint:          `\\.\pipe\caspian-priv`,
		RuntimeDir:            "",
		StateDir:              stateDir,
		FirstRunPasswordPath:  filepath.Join(stateDir, "first-run-password"),
		JournalPath:           filepath.Join(stateDir, "netcfg.journal"),
		BinaryPath:            filepath.Join(programs, "Caspian", "caspian.exe"),
		ServiceManager:        "the Service Control Manager",
		StartPrivilegedAdvice: "sc start caspian",
		StopPrivilegedAdvice:  "sc stop caspian",
	}
}
