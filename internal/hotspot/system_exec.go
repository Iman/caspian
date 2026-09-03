// SPDX-License-Identifier: AGPL-3.0-or-later

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
	"strings"
	"time"
)

// execSystem is the real System: it runs commands and touches real files.
//
// Untagged: everything here is portable Go. The two process operations that
// differ per operating system, ProcessAlive and SignalProcess, live in
// system_exec_unix.go and system_exec_windows.go. Only NewSystemRunner is
// gated to Linux, because the hostapd and dnsmasq Supervisor is Linux; the
// other platforms build their access points over the same execSystem.
type execSystem struct{}

// NewExecSystem returns a System backed by real processes and real files.
//
// Available on any unix so a developer on darwin can run the exec plumbing
// tests. Use NewSystemRunner for the appliance.
func NewExecSystem() System { return execSystem{} }

func (e execSystem) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return e.RunInput(ctx, "", name, args...)
}

func (execSystem) RunInput(ctx context.Context, stdin string, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
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
