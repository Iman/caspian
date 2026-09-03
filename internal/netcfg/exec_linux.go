// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package netcfg

import "os/exec"

// linuxSearchPath is where the tools are looked for. It is fixed rather than
// inherited so that a PATH from the environment cannot decide which binary
// called "ip" gets root.
var linuxSearchPath = []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin", "/usr/local/sbin", "/usr/local/bin"}

// NewSystemRunner returns the runner that actually changes the machine.
func NewSystemRunner() Runner {
	return &systemRunner{platform: PlatformLinux, searchPath: linuxSearchPath, lookPath: exec.LookPath}
}

// SystemBackend is the backend for the machine this binary runs on.
func SystemBackend() Backend { return BackendFor(PlatformLinux) }
