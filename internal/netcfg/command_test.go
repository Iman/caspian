// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"testing"
)

// The privileged side accepts a short list of named actions and never a
// command built from anything the user typed. The allowlist is where that
// stops being a convention.
func TestValidateCommand_Allowlist(t *testing.T) {
	for _, ok := range []string{BinIP, BinIw, BinNft, BinSysctl} {
		if err := ValidateCommand(Command{Path: ok, Args: []string{"x"}}); err != nil {
			t.Errorf("%s should be allowed: %v", ok, err)
		}
	}
	for _, bad := range []string{"sh", "/bin/sh", "bash", "systemctl", "iptables", "curl", "../../bin/sh", "ip "} {
		err := ValidateCommand(Command{Path: bad, Args: []string{"x"}})
		if !errors.Is(err, ErrDisallowedBinary) {
			t.Errorf("ValidateCommand(%q) = %v, want ErrDisallowedBinary", bad, err)
		}
	}
	if err := ValidateCommand(Command{}); err == nil {
		t.Error("an empty command must be refused")
	}
	if err := ValidateCommand(Command{Path: BinIP, Args: []string{"a\x00b"}}); err == nil {
		t.Error("an argument containing a NUL must be refused")
	}
}

func TestValidInterfaceName(t *testing.T) {
	for _, ok := range []string{"eth0", "wlan0", "ap0", "xray0", "end0", "enxdca632112233", "br-lan", "eth0.100"} {
		if !ValidInterfaceName(ok) {
			t.Errorf("%q should be a valid interface name", ok)
		}
	}
	for _, bad := range []string{
		"", "eth0 up", "eth0;reboot", "../etc", "a/b", "\"eth0\"",
		"aninterfacenamethatistoolong", "eth0\n",
	} {
		if ValidInterfaceName(bad) {
			t.Errorf("%q must not be accepted as an interface name", bad)
		}
	}
}

// Every command this package generates must pass its own allowlist. A step
// that cannot be executed is a step that fails at the worst moment.
func TestGeneratedSteps_AllPassValidation(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	for _, s := range p.AllSteps(f.Sysctl) {
		if err := ValidateCommand(s.Do); err != nil {
			t.Errorf("step %s produced an invalid command: %v", s.Op, err)
		}
		if s.Undo.IsZero() {
			continue
		}
		if err := ValidateCommand(s.Undo); err != nil {
			t.Errorf("inverse of %s is an invalid command: %v", s.Op, err)
		}
	}
}

// On a platform with no runner the runner refuses rather than quietly doing
// nothing. A no-op runner would make an apply on a development machine report
// success, which is a false green. On Linux and macOS there IS a runner, and
// it attempts a command from its own allowlist and refuses one from another
// platform's.
func TestNewSystemRunner_RefusesOffLinux(t *testing.T) {
	r := NewSystemRunner()
	_, err := r.Run(context.Background(), Command{Path: BinIP, Args: []string{"link"}})
	if !hasSystemRunner {
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("err = %v, want ErrUnsupportedPlatform", err)
		}
		return
	}
	if errors.Is(err, ErrUnsupportedPlatform) {
		t.Error("on a platform with a runner it must attempt or refuse the command by allowlist, not by platform")
	}
	if isLinux {
		return
	}
	// macOS: "ip" is Linux's and is not on this runner's list.
	if !errors.Is(err, ErrDisallowedBinary) {
		t.Errorf("err = %v, want ErrDisallowedBinary for a Linux binary on a non-Linux runner", err)
	}
	if _, err := r.Run(context.Background(), Command{Path: "ifconfig", Args: []string{"lo0"}}); err != nil {
		t.Errorf("ifconfig lo0 on the macOS runner: %v", err)
	}
}

func TestCommandString_IsNotAShellCommand(t *testing.T) {
	c := Command{Path: BinIP, Args: []string{"route", "add", "a b"}}
	got := c.String()
	want := `ip route add "a b"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
