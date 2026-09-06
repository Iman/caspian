// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"caspianbyoc.org/caspian/internal/state"
)

func runResetPassword(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "Usage: caspian reset-password")
		return exitUsage
	}
	if ok, _ := runningPrivileged(); !ok {
		fmt.Fprintln(stderr, "Run caspian reset-password as an administrator (sudo on Linux and macOS).")
		return exitError
	}
	err := generatePanelPassword(stateDir, stdout, func(start bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return changePanelService(ctx, start)
	})
	if err != nil {
		fmt.Fprintf(stderr, "caspian: %v\n", err)
		return exitError
	}
	return exitOK
}

// Stop the sole state writer before loading its state. Restarting the panel
// also invalidates existing browser sessions. The hotspot service stays up.
func generatePanelPassword(dir string, out io.Writer, service func(bool) error) (err error) {
	path := filepath.Join(dir, state.FileName)
	fi, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot read the installed panel state: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return errors.New("the panel state must be a regular file")
	}
	if err = service(false); err != nil {
		return fmt.Errorf("could not stop the panel; password unchanged: %w", err)
	}
	defer func() {
		if startErr := service(true); startErr != nil {
			err = errors.Join(err, fmt.Errorf("could not restart the panel: %w", startErr))
		}
	}()
	store, err := state.Load(dir)
	if err != nil {
		return err
	}
	var random [18]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	password := base64.RawURLEncoding.EncodeToString(random[:])
	if err := store.SetPanelPassword(password); err != nil {
		return err
	}
	// Atomic state writes create a new inode. Restore its service ownership
	// before restarting the unprivileged panel.
	if err := restoreStateOwner(path, fi); err != nil {
		return fmt.Errorf("password changed but state ownership could not be restored: %w", err)
	}
	_, err = fmt.Fprintf(out, "New Caspian panel password: %s\nProxy and Wi-Fi settings are unchanged.\n", password)
	return err
}

func changePanelService(ctx context.Context, start bool) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "linux":
		verb := "stop"
		if start {
			verb = "start"
		}
		name, args = "systemctl", []string{verb, "caspian-panel.service"}
	case "darwin":
		name = "/bin/launchctl"
		if !start {
			return stopDarwinPanel(ctx)
		}
		args = []string{"bootstrap", "system", "/Library/LaunchDaemons/org.caspianbyoc.caspian-panel.plist"}
	case "windows":
		verb := "Stop-Service"
		if start {
			verb = "Start-Service"
		}
		name = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		args = []string{"-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; " + verb + " -Name caspian-panel"}
	default:
		return errors.New("password recovery is not supported on this operating system")
	}
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, output)
	}
	return nil
}

func stopDarwinPanel(ctx context.Context) error {
	const job = "system/org.caspianbyoc.caspian-panel"
	output, err := exec.CommandContext(ctx, "/bin/launchctl", "print", job).CombinedOutput()
	if err != nil {
		// An unloaded panel is already stopped. Still reject an expired request.
		return ctx.Err()
	}
	pid := ""
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "pid" && fields[1] == "=" {
			if n, err := strconv.Atoi(fields[2]); err == nil && n > 0 {
				pid = fields[2]
			}
		}
	}
	if output, err := exec.CommandContext(ctx, "/bin/launchctl", "bootout", job).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootout: %w: %s", err, output)
	}
	for pid != "" {
		if exec.CommandContext(ctx, "/bin/kill", "-0", pid).Run() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}
