// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package hotspot

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// ProcessAlive asks the kernel for the process and its exit code. Windows
// has no kill(pid, 0); a handle that opens and a process that has not
// exited is the same fact.
func (execSystem) ProcessAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("hotspot: refusing to query pid %d", pid)
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil // no such process
		}
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return true, nil // alive, and somebody else's
		}
		return false, err
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false, err
	}
	const stillActive = 259 // STILL_ACTIVE
	return code == stillActive, nil
}

// SignalProcess: Windows has no SIGTERM for a process without a console, so
// both signals end the process outright. The graceful half of the
// Supervisor's TERM, poll, KILL sequence does not exist here; the access
// point drivers on this platform do not spawn daemons, so nothing reaches
// this in practice.
func (execSystem) SignalProcess(pid int, _ Signal) error {
	if pid <= 0 {
		return fmt.Errorf("hotspot: refusing to signal pid %d", pid)
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil // already gone
		}
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}
