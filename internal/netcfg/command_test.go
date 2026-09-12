// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// The privileged side accepts a short list of named actions and never a
// command built from anything the user typed. The allowlist is where that
// stops being a convention.
func TestValidateCommand_Allowlist(t *testing.T) {
	for _, ok := range []string{BinIP, BinIw, BinNft, BinSysctl, BinNmcli} {
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
	if runtime.GOOS == "windows" {
		return
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

// TestTheUninstallerReplaysEveryBinaryThisPackageMayRun holds uninstall.sh to
// the allowlist above.
//
// The uninstaller replays the network journal, and it refuses a journal that
// names a binary it does not know. That is the right refusal for /bin/sh. It is
// the wrong refusal for nmcli, which this package runs when it takes an
// interface away from NetworkManager, and whose inverse it journals. The two
// lists drifted: this package allowed five binaries, the script's ALLOWED tuple
// named four, and on 2026-09-12 a real uninstall on a Raspberry Pi refused its
// whole journal at entry 6 and restored nothing. The script cannot import this
// package, so this test reads its tuple and compares the two sets.
func TestTheUninstallerReplaysEveryBinaryThisPackageMayRun(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "uninstall.sh"))
	if err != nil {
		t.Fatalf("reading uninstall.sh: %v", err)
	}
	m := regexp.MustCompile(`(?m)^ALLOWED = \((.*)\)$`).FindSubmatch(body)
	if m == nil {
		t.Fatal("uninstall.sh no longer has a line of the form ALLOWED = (...). " +
			"The replay allowlist moved; move this test with it rather than deleting it.")
	}
	script := map[string]bool{}
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(m[1], -1) {
		script[string(q[1])] = true
	}
	for bin := range allowedBinaries {
		if !script[bin] {
			t.Errorf("this package may run %q and journal its inverse, but the uninstaller's "+
				"ALLOWED tuple does not name it, so a journal holding one is refused whole and "+
				"the box's network is not restored", bin)
		}
	}
	for bin := range script {
		if !allowedBinaries[bin] {
			t.Errorf("the uninstaller would replay %q, which this package never runs", bin)
		}
	}
}
