// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build darwin

package netcfg

import "os/exec"

// darwinSearchPath holds the four directories Apple ships the tools in:
// /sbin has route, ifconfig and pfctl; /usr/sbin has sysctl and networksetup.
// Homebrew's prefixes are deliberately absent: nothing this backend runs is
// installed by a package manager, and a directory the login user can write to
// must not be a place a root process looks for a binary.
var darwinSearchPath = []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin"}

// NewSystemRunner returns the runner that actually changes the Mac.
func NewSystemRunner() Runner {
	return &systemRunner{platform: PlatformDarwin, searchPath: darwinSearchPath, lookPath: exec.LookPath}
}

// SystemBackend is the backend for the machine this binary runs on.
func SystemBackend() Backend { return BackendFor(PlatformDarwin) }
