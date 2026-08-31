// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package hotspot

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These exercise the real System, not the Recorder. They run on any unix, so
// the plumbing the appliance depends on is tested during development on darwin
// as well as on the Pi.

func TestExecSystemRunSeparatesFailureFromInabilityToRun(t *testing.T) {
	sys := NewExecSystem()
	ctx := context.Background()

	res, err := sys.Run(ctx, "/bin/echo", "hello")
	if err != nil {
		t.Fatalf("running /bin/echo: %v", err)
	}
	if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) != "hello" {
		t.Errorf("result = %+v", res)
	}

	// A command that ran and failed is a Result with a non-zero code and a nil
	// error. The caller needs the code and the output to explain the failure,
	// and wrapping it in an error loses both. pgrep exiting 1 for "no match"
	// is exactly this case.
	res, err = sys.Run(ctx, "/bin/sh", "-c", "echo out; echo err >&2; exit 3")
	if err != nil {
		t.Fatalf("a command that exited non-zero returned an error: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", res.ExitCode)
	}
	if strings.TrimSpace(res.Stdout) != "out" || strings.TrimSpace(res.Stderr) != "err" {
		t.Errorf("result = %+v", res)
	}

	// A command that could not be executed at all is an error.
	if _, err := sys.Run(ctx, "/nonexistent/caspian-test-binary"); err == nil {
		t.Error("running a missing binary did not return an error")
	}
}

func TestExecSystemFilePermissions(t *testing.T) {
	sys := NewExecSystem()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "hostapd.conf")

	// The hostapd configuration carries the WPA2 passphrase and must not be
	// world readable, not even for the moment between creation and chmod.
	if err := sys.WriteFile(path, []byte("wpa_passphrase=secret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the temporary file was left behind")
	}

	got, err := sys.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "wpa_passphrase=secret\n" {
		t.Errorf("read back %q", got)
	}

	// A missing file must satisfy errors.Is(err, fs.ErrNotExist): the
	// supervisor uses that to tell "no pid file" from "cannot read pid file".
	_, err = sys.ReadFile(filepath.Join(dir, "absent"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing file gave %v, want something satisfying fs.ErrNotExist", err)
	}

	if err := sys.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// Removing what is not there is not an error.
	if err := sys.Remove(path); err != nil {
		t.Errorf("removing an absent file returned %v", err)
	}
}

func TestExecSystemProcessAlive(t *testing.T) {
	sys := NewExecSystem()

	alive, err := sys.ProcessAlive(os.Getpid())
	if err != nil {
		t.Fatalf("ProcessAlive: %v", err)
	}
	if !alive {
		t.Error("this test process was reported as not running")
	}

	// pid 0 and negative pids are not processes; on unix they address process
	// groups, and signalling one by accident would be severe.
	for _, pid := range []int{0, -1} {
		if alive, _ := sys.ProcessAlive(pid); alive {
			t.Errorf("pid %d was reported alive", pid)
		}
		if err := sys.SignalProcess(pid, SignalTerm); err == nil {
			t.Errorf("signalling pid %d was allowed", pid)
		}
	}
}

func TestExecSystemSleepRespectsCancellation(t *testing.T) {
	sys := NewExecSystem()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if err := sys.Sleep(ctx, 10*time.Second); err == nil {
		t.Error("Sleep ignored a cancelled context")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Sleep waited %s on a cancelled context", elapsed)
	}
}
