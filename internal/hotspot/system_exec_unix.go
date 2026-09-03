// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package hotspot

import (
	"errors"
	"fmt"
	"syscall"
)

func (execSystem) ProcessAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	// Signal 0 performs the permission and existence check without sending
	// anything. ESRCH means no such process; EPERM means it exists and
	// belongs to someone else, which for our purposes is alive.
	err := syscall.Kill(pid, 0)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	case errors.Is(err, syscall.EPERM):
		return true, nil
	default:
		return false, fmt.Errorf("hotspot: could not check process %d: %w", pid, err)
	}
}

func (execSystem) SignalProcess(pid int, sig Signal) error {
	if pid <= 0 {
		return fmt.Errorf("hotspot: refusing to signal process id %d", pid)
	}
	var s syscall.Signal
	switch sig {
	case SignalTerm:
		s = syscall.SIGTERM
	case SignalKill:
		s = syscall.SIGKILL
	default:
		return fmt.Errorf("hotspot: unknown signal %d", int(sig))
	}
	if err := syscall.Kill(pid, s); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			// Already gone. Stopping something that has stopped is success.
			return nil
		}
		return fmt.Errorf("hotspot: could not send %s to process %d: %w", sig, pid, err)
	}
	return nil
}
