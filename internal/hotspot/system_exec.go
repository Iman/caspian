// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package hotspot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// execSystem is the real System: it runs commands and touches real files.
//
// Built on unix so that it compiles, vets and can be exercised on darwin
// during development. Only NewSystemRunner is gated to Linux, because the
// appliance is Linux and the DefaultPaths are Linux paths.
type execSystem struct{}

// NewExecSystem returns a System backed by real processes and real files.
//
// Available on any unix so a developer on darwin can run the exec plumbing
// tests. Use NewSystemRunner for the appliance.
func NewExecSystem() System { return execSystem{} }

func (execSystem) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}

	var ee *exec.ExitError
	switch {
	case err == nil:
		res.ExitCode = 0
		return res, nil
	case errors.As(err, &ee):
		// The command ran and failed. That is a Result, not an error: the
		// caller needs the exit code and stderr to explain the failure in
		// words, and wrapping it in an error loses both.
		res.ExitCode = ee.ExitCode()
		return res, nil
	default:
		// Could not execute at all: not found, not permitted, cancelled.
		return res, fmt.Errorf("hotspot: could not run %s: %w", name, err)
	}
}

func (execSystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("hotspot: could not create %s: %w", dir, err)
		}
	}
	// Written through a temporary file and renamed, so a config is never half
	// written when hostapd reads it. The temporary carries the final
	// permissions from the start: the hostapd configuration holds the WPA2
	// passphrase and must not exist, even briefly, as a world-readable file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("hotspot: could not write %s: %w", path, err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("hotspot: could not set permissions on %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("hotspot: could not replace %s: %w", path, err)
	}
	return nil
}

func (execSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (execSystem) Remove(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("hotspot: could not remove %s: %w", path, err)
	}
	return nil
}

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

func (execSystem) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
