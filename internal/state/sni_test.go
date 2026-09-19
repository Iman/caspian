// SPDX-License-Identifier: AGPL-3.0-or-later
package state

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// An encoding failure must leave the last usable configuration and SNI choice
// intact both on disk and in memory, with no partial temporary file.
func TestSpoofSNIUpdateRollsBackWhenStateCannotBeEncoded(t *testing.T) {
	dir := tempStateDir(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSpoofSNI("cover.example.invalid"); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	diskBefore, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(st *State) error {
		st.Proxy.SpoofSNI = "replacement.example.invalid"
		st.Proxy.AddedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "encoding state") {
		t.Fatalf("want an encoding error, got %v", err)
	}
	if !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("failed encoding published a new state")
	}
	diskAfter, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(diskBefore, diskAfter) {
		t.Fatal("failed encoding changed the persisted state")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state-") {
			t.Fatal("failed encoding left a temporary state file")
		}
	}
}

func TestSpoofSNIIsOptionalPersistentAndIndependent(t *testing.T) {
	dir := tempStateDir(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.SetSpoofSNI("cover.example.invalid") == nil {
		t.Fatal("accepted without config")
	}
	if err = s.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatal(err)
	}
	if err = s.SetSpoofSNI(" COVER.Example.Invalid. "); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Proxy().SpoofSNI.Reveal() != "cover.example.invalid" || reloaded.Proxy().Raw.Reveal() != fakeProxyLink {
		t.Fatal("round trip changed settings")
	}
	before := s.Snapshot()
	if s.SetSpoofSNI("https://bad.invalid") == nil {
		t.Fatal("accepted URL")
	}
	if !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("failed update mutated state")
	}
	if err = s.Update(func(st *State) error { st.Proxy.SpoofSNI = Secret("bad/name"); return nil }); err == nil {
		t.Fatal("Update bypassed validation")
	}
	if err = s.SetSpoofSNI(""); err != nil {
		t.Fatal(err)
	}
	if s.Proxy().SpoofSNI.Reveal() != "" {
		t.Fatal("did not disable")
	}
	s.SetSpoofSNI("cover.example.invalid")
	s.SetProxyConfig(fakeProxyLink, fakeProxyScheme, "replacement")
	if s.Proxy().SpoofSNI.Reveal() != "" {
		t.Fatal("replacement retained old spoof setting")
	}
}

func TestV3MigrationPreservesConfigWithSpoofingOff(t *testing.T) {
	before := fullState(t)
	before.Version = 3
	before.Proxy.SpoofSNI = ""
	raw, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(writeStateFile(t, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	after := s.Snapshot()
	if after.Version != CurrentVersion || after.Proxy.Raw != before.Proxy.Raw || after.Proxy.Selected != before.Proxy.Selected || after.Proxy.SpoofSNI != "" {
		t.Fatal("migration changed config or enabled spoofing")
	}
}

func TestDPISettingsRemainIndependentAcrossReloadAndReplacement(t *testing.T) {
	dir := tempStateDir(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetDPISettings("", true, false); err == nil {
		t.Fatal("accepted splitting without config")
	}
	if err = s.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatal(err)
	}
	for mask := 0; mask < 8; mask++ {
		name := ""
		if mask&1 != 0 {
			name = "cover.example.invalid"
		}
		if err = s.SetDPISettings(name, mask&2 != 0, mask&4 != 0); err != nil {
			t.Fatal(err)
		}
		again, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		got := again.Proxy()
		if got.SpoofSNI.Reveal() != name || got.TCPSplit != (mask&2 != 0) || got.TLSRecordSplit != (mask&4 != 0) || got.Raw.Reveal() != fakeProxyLink {
			t.Fatal("settings did not survive independently")
		}
	}
	before := s.Snapshot()
	if err = s.SetDPISettings("bad/name", false, false); err == nil || !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("invalid settings changed state")
	}
	if err = s.SetProxyConfig(fakeProxyLink, fakeProxyScheme, "replacement"); err != nil {
		t.Fatal(err)
	}
	if p := s.Proxy(); p.TCPSplit || p.TLSRecordSplit || !p.SpoofSNI.IsZero() {
		t.Fatal("replacement retained DPI settings")
	}
}
func TestV4MigrationPreservesFakeNameAndDisablesSplitting(t *testing.T) {
	before := fullState(t)
	before.Version = 4
	raw, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(writeStateFile(t, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Proxy()
	if got.SpoofSNI != before.Proxy.SpoofSNI || got.Raw != before.Proxy.Raw || got.TCPSplit || got.TLSRecordSplit {
		t.Fatal("migration lost SNI or enabled splitting")
	}
}
