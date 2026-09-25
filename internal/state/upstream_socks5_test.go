// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package state

import (
	"os"
	"strings"
	"testing"
)

func TestUpstreamSOCKS5StateRoundTripAndRedaction(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, dirMode); err != nil {
		t.Fatal(err)
	}
	st, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Update(func(s *State) error {
		s.Advanced.UpstreamSOCKS5 = UpstreamSOCKS5{
			Enabled:  true,
			Address:  "127.0.0.1",
			Port:     1080,
			Username: Secret("user"),
			Password: Secret("secret"),
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	got := st.Snapshot().Advanced.UpstreamSOCKS5
	if !got.Enabled || got.Address != "127.0.0.1" || got.Port != 1080 {
		t.Fatalf("upstream did not round-trip: %+v", got)
	}
	if got.Username.Reveal() != "user" || got.Password.Reveal() != "secret" {
		t.Fatal("upstream credentials did not round-trip")
	}
	red := st.Snapshot().Redacted()
	if strings.Contains(red, "secret") || strings.Contains(red, "user") {
		t.Fatalf("upstream credential leaked in redacted state: %s", red)
	}
}

func TestUpstreamSOCKS5StateRejectsIncompleteCredentials(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, dirMode); err != nil {
		t.Fatal(err)
	}
	st, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = st.Update(func(s *State) error {
		s.Advanced.UpstreamSOCKS5 = UpstreamSOCKS5{
			Enabled:  true,
			Address:  "127.0.0.1",
			Port:     1080,
			Username: Secret("user"),
		}
		return nil
	})
	if err == nil {
		t.Fatal("state accepted incomplete upstream SOCKS5 credentials")
	}
}
