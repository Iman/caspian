// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

func TestResetPasswordPreservesSettingsAndRejectsOldPassword(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetPanelPassword("old-password"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHotspot("test-hotspot", "wifi-password"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProxyConfig("test-config", "vless", "test"); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	var calls []bool
	var out bytes.Buffer
	err = generatePanelPassword(dir, &out, func(start bool) error {
		calls = append(calls, start)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []bool{false, true}) {
		t.Fatal(calls)
	}
	password := strings.TrimPrefix(strings.Split(out.String(), "\n")[0], "New Caspian panel password: ")
	s, err = state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.VerifyPanelPassword(password); !ok || err != nil {
		t.Fatal("new password rejected", err)
	}
	if ok, _ := s.VerifyPanelPassword("old-password"); ok {
		t.Fatal("old password accepted")
	}
	after := s.Snapshot()
	if before.Proxy != after.Proxy || before.Hotspot != after.Hotspot || before.Advanced != after.Advanced {
		t.Fatal("reset changed network settings")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, state.FileName))
	if bytes.Contains(raw, []byte(password)) {
		t.Fatal("plaintext password persisted")
	}
}

func TestResetPasswordFailureRestartsPanelAndDoesNotPrintPassword(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	os.WriteFile(filepath.Join(dir, state.FileName), []byte("corrupt"), 0600)
	var calls []bool
	var out bytes.Buffer
	err := generatePanelPassword(dir, &out, func(start bool) error { calls = append(calls, start); return nil })
	if err == nil || out.Len() != 0 || !reflect.DeepEqual(calls, []bool{false, true}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
	calls = nil
	err = generatePanelPassword(dir, &out, func(start bool) error { calls = append(calls, start); return errors.New("stop failed") })
	if err == nil || !reflect.DeepEqual(calls, []bool{false}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
}
