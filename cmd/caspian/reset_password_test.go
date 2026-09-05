package main

import (
	"caspianbyoc.org/caspian/internal/state"
	"os"
	"strings"
	"testing"
)

func TestResetPasswordPreservesSettingsAndExactSpaces(t *testing.T) {
	prev := stateDir
	stateDir = t.TempDir()
	t.Cleanup(func() { stateDir = prev })
	if err := os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := state.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetHotspot("preserved-network", "preserved-passphrase"); err != nil {
		t.Fatal(err)
	}
	before := s.Hotspot()
	var out, errOut strings.Builder
	pass := " password with spaces "
	if code := resetPanelPassword(nil, strings.NewReader(pass+"\n"+pass+"\n"), &out, &errOut); code != 0 {
		t.Fatal(errOut.String())
	}
	s, err = state.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.VerifyPanelPassword(pass); err != nil || !ok {
		t.Fatal("password changed during reset")
	}
	if s.Hotspot() != before {
		t.Fatal("reset changed hotspot")
	}
	for _, input := range []string{"short\nshort\n", "long-enough\n", "long-enough\ndifferent-value\n", strings.Repeat("x", 9000)} {
		if resetPanelPassword(nil, strings.NewReader(input), &out, &errOut) == 0 {
			t.Fatal("accepted invalid reset input")
		}
	}
	if strings.Contains(out.String(), pass) || strings.Contains(errOut.String(), pass) {
		t.Fatal("password leaked in command output")
	}
}
