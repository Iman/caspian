// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A first run gets the blocking policy, not Go's zero value.
//
// The whole point of the field is that a downstream package reading it can
// never see an empty string and have to decide what that means.
func TestDefaultStateCarriesTheBlockingClientIPv6(t *testing.T) {
	if got := defaultState().Advanced.ClientIPv6; got != ClientIPv6Block {
		t.Errorf("defaultState().Advanced.ClientIPv6 = %q, want %q", got, ClientIPv6Block)
	}
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load of an empty directory: %v", err)
	}
	if !st.FirstRun() {
		t.Fatal("FirstRun() = false on an empty directory")
	}
	if got := st.Snapshot().Advanced.ClientIPv6; got != ClientIPv6Block {
		t.Errorf("a first run reads ClientIPv6 = %q, want %q", got, ClientIPv6Block)
	}
}

// A v1 file, which is what every box in the field is carrying, has no
// client_ipv6 key at all. It must load, and it must land on the blocking
// value rather than on empty.
//
// This is the migration the schema bump exists for. Without it a v1 file would
// decode ClientIPv6 as "", which internal/privsvc refuses, so an existing box
// would stop starting after an update.
func TestAVersion1FileWithoutTheFieldMigratesToTheBlockingValue(t *testing.T) {
	dir := writeStateFile(t, `{
		"version":1,
		"proxy":{"raw":"`+fakeProxyLink+`","scheme":"vless"},
		"hotspot":{"ssid":"`+fakeSSID+`","passphrase":"`+fakePassphrase+`"},
		"advanced":{"internet_interface":"eth0","dns_mode":"tunnel","on_tunnel_down":"block"}
	}`)

	st, err := Load(dir)
	if err != nil {
		t.Fatalf("a v1 file must migrate, not fail: %v", err)
	}
	got := st.Snapshot()
	if got.Version != CurrentVersion {
		t.Errorf("Version = %d after migration, want %d", got.Version, CurrentVersion)
	}
	if got.Advanced.ClientIPv6 != ClientIPv6Block {
		t.Errorf("ClientIPv6 = %q after migrating a v1 file, want %q", got.Advanced.ClientIPv6, ClientIPv6Block)
	}
	// The rest of the file survives. A migration that quietly resets the
	// user's config would pass a test that only looked at the new field.
	if got.Advanced.InternetInterface != "eth0" {
		t.Errorf("InternetInterface = %q, want %q", got.Advanced.InternetInterface, "eth0")
	}
	if got.Proxy.Scheme != "vless" {
		t.Errorf("Proxy.Scheme = %q, want %q", got.Proxy.Scheme, "vless")
	}
	if got.Hotspot.SSID != fakeSSID {
		t.Errorf("Hotspot.SSID = %q, want %q", got.Hotspot.SSID, fakeSSID)
	}
}

// A v0 file, which predates the version key, walks the whole chain and still
// arrives at the blocking value.
func TestAVersion0FileMigratesAllTheWayToTheBlockingValue(t *testing.T) {
	dir := writeStateFile(t, `{
		"proxy":{"raw":"`+fakeProxyLink+`"},
		"hotspot":{"ssid":"`+fakeSSID+`","passphrase":"`+fakePassphrase+`"}
	}`)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("a v0 file must migrate, not fail: %v", err)
	}
	got := st.Snapshot()
	if got.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", got.Version, CurrentVersion)
	}
	if got.Advanced.ClientIPv6 != ClientIPv6Block {
		t.Errorf("ClientIPv6 = %q, want %q", got.Advanced.ClientIPv6, ClientIPv6Block)
	}
}

// A value already in the file is a choice somebody made, and a migration that
// overwrote it would be this package deciding for them.
//
// It is still refused further down the stack: internal/privsvc supports only
// the blocking value and names the refusal. Storing it and refusing it are
// different jobs, and this package does the first.
func TestAClientIPv6AlreadyInTheFileIsKept(t *testing.T) {
	dir := writeStateFile(t, `{
		"version":1,
		"hotspot":{"ssid":"`+fakeSSID+`","passphrase":"`+fakePassphrase+`"},
		"advanced":{"dns_mode":"tunnel","on_tunnel_down":"block","client_ipv6":"chosen-by-the-user"}
	}`)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := st.Snapshot().Advanced.ClientIPv6; got != "chosen-by-the-user" {
		t.Errorf("ClientIPv6 = %q, want the value from the file", got)
	}
}

// Empty is never written, for the same reason as DNSMode and OnTunnelDown: a
// downstream package must never be able to read empty and treat it as "no
// policy configured".
func TestSaveRefusesAnEmptyClientIPv6(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = st.Update(func(s *State) error { s.Advanced.ClientIPv6 = ""; return nil })
	if err == nil {
		t.Fatal("Save accepted an empty client IPv6 policy")
	}
	if !strings.Contains(err.Error(), "client IPv6") {
		t.Errorf("the refusal does not name the field: %v", err)
	}
	// And nothing was written, so a refused Save cannot leave a file that the
	// next Load would have to cope with.
	if _, statErr := os.Stat(filepath.Join(dir, FileName)); statErr == nil {
		t.Error("a refused Save wrote the file anyway")
	}
}

// A file this build writes must load in this build. Obvious, and it is the
// check that catches a version bumped without its migration, or a json tag
// that does not match what the decoder looks for.
func TestAFileWrittenByThisBuildStillLoads(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Update(func(s *State) error {
		s.Advanced.InternetInterface = "eth0"
		s.Hotspot.SSID = fakeSSID
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Version  int `json:"version"`
		Advanced struct {
			ClientIPv6 string `json:"client_ipv6"`
		} `json:"advanced"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Version != CurrentVersion {
		t.Errorf("on-disk version = %d, want %d", probe.Version, CurrentVersion)
	}
	if probe.Advanced.ClientIPv6 != ClientIPv6Block {
		t.Errorf("on-disk client_ipv6 = %q, want %q", probe.Advanced.ClientIPv6, ClientIPv6Block)
	}

	again, err := Load(dir)
	if err != nil {
		t.Fatalf("reloading a file this build wrote: %v", err)
	}
	got := again.Snapshot()
	if got.Advanced.ClientIPv6 != ClientIPv6Block {
		t.Errorf("after a reload ClientIPv6 = %q, want %q", got.Advanced.ClientIPv6, ClientIPv6Block)
	}
	if got.Advanced.InternetInterface != "eth0" {
		t.Errorf("after a reload InternetInterface = %q, want eth0", got.Advanced.InternetInterface)
	}
}

// The redacted rendering is what reaches a log or a support bundle, and it is
// built field by field on purpose, so a field added to Advanced is invisible
// there until somebody adds it deliberately. This is that deliberate step,
// asserted rather than remembered.
func TestTheRedactedRenderingNamesTheClientIPv6Policy(t *testing.T) {
	s := defaultState()
	if got := s.Redacted(); !strings.Contains(got, `adv.client_ipv6="block"`) {
		t.Errorf("Redacted() does not report the client IPv6 policy: %s", got)
	}
}
