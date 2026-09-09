// SPDX-License-Identifier: AGPL-3.0-or-later
package state

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSpoofSNIIsOptionalPersistentAndIndependent(t *testing.T) {
	dir := t.TempDir()
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
	if after.Version != 4 || after.Proxy.Raw != before.Proxy.Raw || after.Proxy.Selected != before.Proxy.Selected || after.Proxy.SpoofSNI != "" {
		t.Fatal("migration changed config or enabled spoofing")
	}
}
