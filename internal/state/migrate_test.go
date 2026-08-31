// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStateFile drops raw JSON into a fresh directory at the required modes,
// so a test can start from a file this build did not write.
func writeStateFile(t *testing.T, body string) string {
	t.Helper()
	dir := tempStateDir(t)
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), fileMode); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return dir
}

// TestLoadRefusesAFutureVersion. A newer release may have added fields this
// build knows nothing about; loading such a file would work and the next Save
// would drop them silently. Refusing turns data loss into a message.
func TestLoadRefusesAFutureVersion(t *testing.T) {
	tests := []struct {
		name  string
		found int
	}{
		{"one version ahead", CurrentVersion + 1},
		{"far ahead", CurrentVersion + 99},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeStateFile(t, `{"version":`+itoa(tc.found)+`,
				"proxy":{"raw":"`+fakeProxyLink+`"},
				"advanced":{"dns_mode":"tunnel","on_tunnel_down":"block"}}`)

			_, err := Load(dir)
			if err == nil {
				t.Fatal("Load accepted a state file from a newer release")
			}

			// The panel needs to tell this apart from a generic failure so it
			// can show "update this box" rather than "something went wrong".
			var fv *ErrFutureVersion
			if !errors.As(err, &fv) {
				t.Fatalf("error is not an *ErrFutureVersion: %v", err)
			}
			if fv.Found != tc.found || fv.Supports != CurrentVersion {
				t.Errorf("ErrFutureVersion{Found:%d, Supports:%d}, want {%d, %d}",
					fv.Found, fv.Supports, tc.found, CurrentVersion)
			}
			for _, want := range []string{"install a newer Caspian release", "will be lost"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the message does not say %q:\n%v", want, err)
				}
			}
			if strings.Contains(err.Error(), fakeProxyLink) {
				t.Error("the refusal error contains the stored config")
			}
		})
	}
}

// TestMigrateFromVersionZero is the required migration test. A version-0 file
// is one written before the version field existed, so its version key is
// absent and decodes as 0.
func TestMigrateFromVersionZero(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantDNSMode      string
		wantOnTunnelDown string
	}{
		{
			name: "absent policy fields are filled with the fail-closed defaults",
			body: `{
				"proxy":{"raw":"` + fakeProxyLink + `","scheme":"vless","label":"old node"},
				"hotspot":{"ssid":"` + fakeSSID + `","passphrase":"` + fakePassphrase + `"},
				"advanced":{"internet_interface":"eth0","hotspot_interface":"wlan0","channel":10}
			}`,
			wantDNSMode:      DNSModeTunnel,
			wantOnTunnelDown: OnTunnelDownBlock,
		},
		{
			name: "an explicit version 0 is treated the same as an absent one",
			body: `{
				"version":0,
				"proxy":{"raw":"` + fakeProxyLink + `","scheme":"vless","label":"old node"},
				"hotspot":{"ssid":"` + fakeSSID + `","passphrase":"` + fakePassphrase + `"},
				"advanced":{"internet_interface":"eth0","hotspot_interface":"wlan0","channel":10}
			}`,
			wantDNSMode:      DNSModeTunnel,
			wantOnTunnelDown: OnTunnelDownBlock,
		},
		{
			name: "a policy value already present is a user choice and is kept",
			body: `{
				"proxy":{"raw":"` + fakeProxyLink + `","scheme":"vless","label":"old node"},
				"hotspot":{"ssid":"` + fakeSSID + `","passphrase":"` + fakePassphrase + `"},
				"advanced":{"internet_interface":"eth0","hotspot_interface":"wlan0","channel":10,
					"dns_mode":"chosen-by-the-user","on_tunnel_down":"also-chosen"}
			}`,
			wantDNSMode:      "chosen-by-the-user",
			wantOnTunnelDown: "also-chosen",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeStateFile(t, tc.body)

			st, err := Load(dir)
			if err != nil {
				t.Fatalf("Load of a version-0 file must migrate, not fail: %v", err)
			}
			if st.FirstRun() {
				t.Error("FirstRun() = true, but a state file existed")
			}

			got := st.Snapshot()
			if got.Version != CurrentVersion {
				t.Errorf("Version = %d after migration, want %d", got.Version, CurrentVersion)
			}
			if got.Advanced.DNSMode != tc.wantDNSMode {
				t.Errorf("DNSMode = %q, want %q", got.Advanced.DNSMode, tc.wantDNSMode)
			}
			if got.Advanced.OnTunnelDown != tc.wantOnTunnelDown {
				t.Errorf("OnTunnelDown = %q, want %q", got.Advanced.OnTunnelDown, tc.wantOnTunnelDown)
			}

			// Everything the old file did carry must survive the migration.
			if got.Proxy.Raw.Reveal() != fakeProxyLink {
				t.Error("the migration lost the proxy config")
			}
			if got.Proxy.Scheme != "vless" || got.Proxy.Label != "old node" {
				t.Error("the migration lost the proxy metadata")
			}
			if got.Hotspot.SSID != fakeSSID || got.Hotspot.Passphrase.Reveal() != fakePassphrase {
				t.Error("the migration lost the hotspot settings")
			}
			if got.Advanced.InternetInterface != "eth0" ||
				got.Advanced.HotspotInterface != "wlan0" ||
				got.Advanced.Channel != 10 {
				t.Error("the migration lost the advanced overrides")
			}

			// The migration is in memory until something writes. After a Save
			// the file must be at the current version and must reload clean.
			if err := st.Save(); err != nil {
				t.Fatalf("Save after migration: %v", err)
			}
			raw, err := os.ReadFile(st.Path())
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			var probe struct {
				Version  int `json:"version"`
				Advanced struct {
					DNSMode      string `json:"dns_mode"`
					OnTunnelDown string `json:"on_tunnel_down"`
				} `json:"advanced"`
			}
			if err := json.Unmarshal(raw, &probe); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if probe.Version != CurrentVersion {
				t.Errorf("on-disk version after Save = %d, want %d", probe.Version, CurrentVersion)
			}
			if probe.Advanced.DNSMode != tc.wantDNSMode || probe.Advanced.OnTunnelDown != tc.wantOnTunnelDown {
				t.Error("the migrated policy fields were not written back")
			}

			again, err := Load(dir)
			if err != nil {
				t.Fatalf("reloading a migrated file: %v", err)
			}
			if again.Snapshot().Version != CurrentVersion {
				t.Error("the reloaded file is not at the current version")
			}
		})
	}
}

// TestMigrationChainIsComplete fails if CurrentVersion is raised without adding
// the migration that goes with it, which is the failure this whole mechanism
// exists to prevent.
func TestMigrationChainIsComplete(t *testing.T) {
	for v := 0; v < CurrentVersion; v++ {
		if _, ok := migrations[v]; !ok {
			t.Errorf("no migration from schema version %d to %d, so a v%d file cannot be loaded", v, v+1, v)
		}
	}
	for v := range migrations {
		if v >= CurrentVersion {
			t.Errorf("migrations has an entry for version %d, which is not below CurrentVersion %d", v, CurrentVersion)
		}
	}
}

// TestMigrateRejectsANegativeVersion covers a corrupted or hand-edited file.
func TestMigrateRejectsANegativeVersion(t *testing.T) {
	dir := writeStateFile(t, `{"version":-1}`)
	if _, err := Load(dir); err == nil {
		t.Error("Load accepted a negative schema version")
	}
}

// TestMigrationMustAdvanceTheVersion guards the loop in migrate against a
// future migration that edits fields and forgets the version, which would spin
// forever rather than fail.
func TestMigrationMustAdvanceTheVersion(t *testing.T) {
	original := migrations[0]
	t.Cleanup(func() { migrations[0] = original })
	migrations[0] = func(*State) error { return nil } // forgets to set Version

	st := State{Version: 0}
	err := migrate(&st, "test")
	if err == nil {
		t.Fatal("migrate accepted a migration that did not advance the version")
	}
	if !strings.Contains(err.Error(), "left the version at") {
		t.Errorf("unexpected error: %v", err)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
