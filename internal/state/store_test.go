// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Every credential in these tests is fabricated. The proxy links are
// syntactically plausible and point at documentation-reserved addresses
// (RFC 5737 192.0.2.0/24, RFC 6761 example.com); no value here works against
// anything.
const (
	fakeProxyLink = "vless://11111111-2222-3333-4444-555555555555@192.0.2.10:443" +
		"?type=tcp&security=reality&pbk=FAKEPUBLICKEYNOTREALFAKEPUBLICKEYNOTREAL0&sid=0123abcd" +
		"&fp=chrome&sni=example.com&spx=%2F#fake-test-node"
	fakeProxyScheme = "vless"
	fakeProxyLabel  = "test node"
	fakePassphrase  = "hotspot-passphrase-not-real"
	fakeSSID        = "Caspian-Test"
	fakePanelPass   = "panel-password-not-real"
)

// fullState returns a State with every single field set to a non-zero value, so
// that a round trip proves each one survives rather than proving that the
// fields somebody remembered survive.
func fullState(t *testing.T) State {
	t.Helper()
	hash, err := hashPassword(fakePanelPass)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	return State{
		Version: CurrentVersion,
		Proxy: ProxyConfig{
			Raw:     Secret(fakeProxyLink),
			Scheme:  fakeProxyScheme,
			Label:   fakeProxyLabel,
			AddedAt: time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
		},
		Hotspot: HotspotConfig{
			SSID:       fakeSSID,
			Passphrase: Secret(fakePassphrase),
		},
		Panel: PanelAuth{PasswordHash: Secret(hash)},
		Advanced: Advanced{
			InternetInterface: "eth0",
			HotspotInterface:  "wlan0",
			Channel:           10,
			Band:              "2.4",
			Country:           "GB",
			Subnet:            "10.42.0.0/24",
			DNSMode:           DNSModeTunnel,
			OnTunnelDown:      OnTunnelDownBlock,
			ClientIPv6:        ClientIPv6Block,
			EngineLogLevel:    "warning",
			PanelOnLAN:        true,
		},
		UpdatedAt: time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
	}
}

// assertNoZeroFields walks a value and fails on any field still at its zero
// value. fullState uses it so that adding a field to State without adding it
// here breaks this test instead of silently going untested.
func assertNoZeroFields(t *testing.T, v reflect.Value, path string) {
	t.Helper()
	if v.Kind() == reflect.Struct && v.Type() != reflect.TypeOf(time.Time{}) {
		for i := 0; i < v.NumField(); i++ {
			assertNoZeroFields(t, v.Field(i), path+"."+v.Type().Field(i).Name)
		}
		return
	}
	if v.IsZero() {
		t.Errorf("%s is at its zero value; the round-trip test is not covering it", path)
	}
}

func TestFullStateCoversEveryField(t *testing.T) {
	assertNoZeroFields(t, reflect.ValueOf(fullState(t)), "State")
}

// ---------------------------------------------------------------- first run

func TestLoadFirstRun(t *testing.T) {
	tests := []struct {
		name string
		// prepare returns the directory to load from.
		prepare func(t *testing.T) string
	}{
		{
			name: "directory does not exist",
			prepare: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "never-created")
			},
		},
		{
			name: "directory exists but is empty",
			prepare: func(t *testing.T) string {
				dir := filepath.Join(t.TempDir(), "empty")
				if err := os.Mkdir(dir, dirMode); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
				return dir
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.prepare(t)

			st, err := Load(dir)
			if err != nil {
				t.Fatalf("Load on a first run must not error, got: %v", err)
			}
			if !st.FirstRun() {
				t.Error("FirstRun() = false; the panel would show a broken screen instead of setup")
			}
			if !st.NeedsSetup() {
				t.Error("NeedsSetup() = false with no password and no config")
			}

			// The zero state must be usable, which means the fail-closed policy
			// fields are already populated, not empty.
			adv := st.Advanced()
			if adv.DNSMode != DNSModeTunnel {
				t.Errorf("DNSMode = %q, want %q", adv.DNSMode, DNSModeTunnel)
			}
			if adv.OnTunnelDown != OnTunnelDownBlock {
				t.Errorf("OnTunnelDown = %q, want %q", adv.OnTunnelDown, OnTunnelDownBlock)
			}
			if got := st.Snapshot().Version; got != CurrentVersion {
				t.Errorf("Version = %d, want %d", got, CurrentVersion)
			}
			if st.Proxy().IsConfigured() {
				t.Error("Proxy().IsConfigured() = true on a first run")
			}

			// Load must not have written anything.
			if _, err := os.Stat(st.Path()); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("Load created %s; it must not write", st.Path())
			}

			// And the first write must clear the first-run flag and create the
			// directory at the required mode.
			if err := st.Save(); err != nil {
				t.Fatalf("Save: %v", err)
			}
			if st.FirstRun() {
				t.Error("FirstRun() still true after a successful Save")
			}
			assertMode(t, dir, dirMode)
			assertMode(t, st.Path(), fileMode)
		})
	}
}

// ---------------------------------------------------------------- round trip

func TestRoundTripEveryField(t *testing.T) {
	dir := tempStateDir(t)
	want := fullState(t)

	writer, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := writer.Update(func(st *State) error {
		*st = want
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	reader, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after write: %v", err)
	}
	if reader.FirstRun() {
		t.Error("FirstRun() = true after a state file was written")
	}
	got := reader.Snapshot()

	// Update stamps UpdatedAt, so that one field is compared separately.
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not stamped by Update")
	}
	want.UpdatedAt = got.UpdatedAt

	if !reflect.DeepEqual(got, want) {
		// Printed through the redacting String methods, so a failure here does
		// not put the credentials in the test log.
		t.Errorf("round trip changed the state\n got: %v\nwant: %v", got, want)
		// Report the differing field names without their values.
		gv, wv := reflect.ValueOf(got), reflect.ValueOf(want)
		for i := 0; i < gv.NumField(); i++ {
			if !reflect.DeepEqual(gv.Field(i).Interface(), wv.Field(i).Interface()) {
				t.Errorf("field %s differs", gv.Type().Field(i).Name)
			}
		}
	}

	// The credentials specifically must come back byte for byte, which is the
	// part a Secret's String method could plausibly have broken.
	if got.Proxy.Raw.Reveal() != fakeProxyLink {
		t.Error("proxy config did not survive the round trip intact")
	}
	if got.Hotspot.Passphrase.Reveal() != fakePassphrase {
		t.Error("hotspot passphrase did not survive the round trip intact")
	}
	ok, err := reader.VerifyPanelPassword(fakePanelPass)
	if err != nil || !ok {
		t.Errorf("panel password did not survive the round trip: ok=%t err=%v", ok, err)
	}
}

func TestTypedSettersRoundTrip(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	if err := st.SetHotspot(fakeSSID, fakePassphrase); err != nil {
		t.Fatalf("SetHotspot: %v", err)
	}
	if err := st.SetPanelPassword(fakePanelPass); err != nil {
		t.Fatalf("SetPanelPassword: %v", err)
	}
	if st.NeedsSetup() {
		t.Error("NeedsSetup() = true after a password and a config were set")
	}

	again, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := again.Proxy(); got.Raw.Reveal() != fakeProxyLink || got.Scheme != fakeProxyScheme || got.Label != fakeProxyLabel {
		t.Error("SetProxyConfig did not persist all three fields")
	}
	if got := again.Hotspot(); got.SSID != fakeSSID || got.Passphrase.Reveal() != fakePassphrase {
		t.Error("SetHotspot did not persist both fields")
	}
	if err := st.SetProxyConfig("", "", ""); err == nil {
		t.Error("SetProxyConfig accepted an empty config")
	}
}

// ------------------------------------------------------------- atomic writes

func TestWriteIsAtomic(t *testing.T) {
	injected := errors.New("injected failure just before the rename")

	tests := []struct {
		name string
		// hook stands in for a failure at the last possible moment. inspect
		// receives the temporary file path so the test can assert on it.
		hook func(t *testing.T, dir string) func(string) error
	}{
		{
			name: "temporary file is in the target directory, at 0600, and the target is untouched until the rename",
			hook: func(t *testing.T, dir string) func(string) error {
				return func(tmpPath string) error {
					// Same directory, or the rename would be a cross-device
					// copy and would not be atomic.
					if got := filepath.Dir(tmpPath); got != dir {
						t.Errorf("temporary file is in %s, want %s", got, dir)
					}
					if !strings.HasPrefix(filepath.Base(tmpPath), ".state-") ||
						!strings.HasSuffix(tmpPath, ".tmp") {
						t.Errorf("temporary file %s does not use the expected name shape", tmpPath)
					}
					// The credential is already on disk at this point, so the
					// mode has to be right on the temporary file too, not only
					// after the rename.
					assertMode(t, tmpPath, fileMode)
					return injected
				}
			},
		},
		{
			name: "failure after the temporary file is synced",
			hook: func(t *testing.T, dir string) func(string) error {
				return func(string) error { return injected }
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempStateDir(t)

			// Establish a good file first: that is what must survive.
			st, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, "original"); err != nil {
				t.Fatalf("SetProxyConfig: %v", err)
			}
			before, err := os.ReadFile(st.Path())
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			// Now make the next write fail at the last moment.
			st.hookAfterTempWrite = tc.hook(t, dir)
			err = st.SetProxyConfig("vless://replacement@192.0.2.99:443#replacement", "vless", "replacement")
			if !errors.Is(err, injected) {
				t.Fatalf("Update error = %v, want the injected failure", err)
			}

			// 1. The original file is byte for byte intact.
			after, err := os.ReadFile(st.Path())
			if err != nil {
				t.Fatalf("ReadFile after the failed write: %v", err)
			}
			if string(after) != string(before) {
				t.Error("a failed write changed the state file; the previous state must survive intact")
			}

			// 2. No temporary file was left behind.
			assertNoTempFiles(t, dir)

			// 3. The in-memory state did not move either, so a reader that
			//    raced the failed write still sees the committed value.
			if got := st.Proxy().Label; got != "original" {
				t.Errorf("in-memory label = %q after a failed write, want %q", got, "original")
			}

			// 4. And the file still parses, which is the property a truncated
			//    write would have destroyed.
			st.hookAfterTempWrite = nil
			reloaded, err := Load(dir)
			if err != nil {
				t.Fatalf("the state file did not survive the failed write: %v", err)
			}
			if got := reloaded.Proxy().Label; got != "original" {
				t.Errorf("reloaded label = %q, want %q", got, "original")
			}
		})
	}
}

func TestWriteFailsCleanlyWhenTheDirectoryIsNotWritable(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, "original"); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	before, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// 0500 keeps the mode check happy (nothing is granted to group or other)
	// while making the temporary file impossible to create.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, dirMode) })

	if err := st.SetHotspot(fakeSSID, fakePassphrase); err == nil {
		t.Fatal("a write into a read-only directory succeeded")
	}

	if err := os.Chmod(dir, dirMode); err != nil {
		t.Fatalf("Chmod back: %v", err)
	}
	after, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatalf("ReadFile after the failed write: %v", err)
	}
	if string(after) != string(before) {
		t.Error("a failed write changed the state file")
	}
	assertNoTempFiles(t, dir)
}

// --------------------------------------------------------------- file modes

func TestLoadRefusesLoosePermissions(t *testing.T) {
	if !permChecksEnforced {
		t.Skip("permission checks are not enforced on this platform; see perm_other.go")
	}

	tests := []struct {
		name       string
		dirMode    fs.FileMode
		fileMode   fs.FileMode
		wantRefuse bool
		wantInErr  string
	}{
		{name: "0600 file in a 0700 directory is accepted", dirMode: 0o700, fileMode: 0o600},
		{name: "0400 file is accepted, it is tighter", dirMode: 0o700, fileMode: 0o400},
		{name: "world-readable file is refused", dirMode: 0o700, fileMode: 0o604, wantRefuse: true, wantInErr: "chmod 0600 "},
		{name: "0644 file is refused", dirMode: 0o700, fileMode: 0o644, wantRefuse: true, wantInErr: "chmod 0600 "},
		{name: "0666 file is refused", dirMode: 0o700, fileMode: 0o666, wantRefuse: true, wantInErr: "chmod 0600 "},
		{name: "group-readable file is refused", dirMode: 0o700, fileMode: 0o640, wantRefuse: true, wantInErr: "chmod 0600 "},
		{name: "world-readable directory is refused", dirMode: 0o755, fileMode: 0o600, wantRefuse: true, wantInErr: "chmod 0700 "},
		{name: "group-readable directory is refused", dirMode: 0o750, fileMode: 0o600, wantRefuse: true, wantInErr: "chmod 0700 "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			st, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
				t.Fatalf("SetProxyConfig: %v", err)
			}

			if err := os.Chmod(st.Path(), tc.fileMode); err != nil {
				t.Fatalf("Chmod file: %v", err)
			}
			if err := os.Chmod(dir, tc.dirMode); err != nil {
				t.Fatalf("Chmod dir: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

			_, err = Load(dir)
			if !tc.wantRefuse {
				if err != nil {
					t.Fatalf("Load refused an acceptable mode: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Load accepted a state file other users on the box can read")
			}
			// The error has to say what to fix, not merely that something is
			// wrong: the person reading it is not a developer.
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("error does not tell the user what to run.\n got: %v\nwant it to contain: %q", err, tc.wantInErr)
			}
			if strings.Contains(err.Error(), fakeProxyLink) || strings.Contains(err.Error(), fakePassphrase) {
				t.Error("the refusal error contains a credential")
			}
		})
	}
}

// TestSaveRefusesALooseDirectory covers the write side of the mode rule, which
// is a separate check from the one Load makes: a directory widened after the
// Store was created must not be written into either.
func TestSaveRefusesALooseDirectory(t *testing.T) {
	if !permChecksEnforced {
		t.Skip("permission checks are not enforced on this platform; see perm_other.go")
	}
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, "original"); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}

	// Somebody widens the directory while the service is running.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, dirMode) })

	err = st.SetHotspot(fakeSSID, fakePassphrase)
	if err == nil {
		t.Fatal("Save wrote a credential into a world-readable directory")
	}
	if !strings.Contains(err.Error(), "chmod 0700 ") {
		t.Errorf("the error does not tell the user what to run: %v", err)
	}
	assertNoTempFiles(t, dir)
}

// TestSaveCreatesAMissingDirectoryAt0700 is the first-run write path: nothing
// exists, so state has to create the directory itself and must not inherit a
// loose mode from the umask.
func TestSaveCreatesAMissingDirectoryAt0700(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "caspian")
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetPanelPassword(fakePanelPass); err != nil {
		t.Fatalf("SetPanelPassword: %v", err)
	}
	assertMode(t, dir, dirMode)
	assertMode(t, st.Path(), fileMode)

	// And it must load back cleanly, which it only does if both modes are right.
	if _, err := Load(dir); err != nil {
		t.Fatalf("Load of the directory state just created: %v", err)
	}
}

// ------------------------------------------------------------------ corrupt

func TestLoadRefusesCorruptFileRatherThanResetting(t *testing.T) {
	dir := tempStateDir(t)
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("{this is not json"), fileMode); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load accepted a corrupt state file")
	}
	// The user's config must still be on disk: resetting silently would throw
	// away the one thing they cannot easily recreate.
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("Load removed or replaced the corrupt file: %v", statErr)
	}
	if !strings.Contains(err.Error(), "has not been touched") {
		t.Errorf("error does not tell the user the file was left alone: %v", err)
	}
}

// ---------------------------------------------------------------- redaction

// TestNothingRendersASecret is the guard behind requirement 4. It renders the
// state every way a careless caller might and asserts that no secret substring
// appears in any of them.
func TestNothingRendersASecret(t *testing.T) {
	st := fullState(t)

	secrets := map[string]string{
		"proxy config":       fakeProxyLink,
		"hotspot passphrase": fakePassphrase,
		"panel password":     fakePanelPass,
		"password hash":      st.Panel.PasswordHash.Reveal(),
	}
	// A fragment of the link too, in case a renderer truncates rather than
	// redacts: the UUID on its own is enough to authenticate.
	secrets["proxy uuid"] = "11111111-2222-3333-4444-555555555555"
	secrets["reality public key"] = "FAKEPUBLICKEYNOTREALFAKEPUBLICKEYNOTREAL0"

	renderings := map[string]string{
		"State.Redacted()":        st.Redacted(),
		"State.String()":          st.String(),
		"fmt %v on State":         fmt.Sprintf("%v", st),
		"fmt %s on State":         fmt.Sprintf("%s", st),
		"fmt %q on State":         fmt.Sprintf("%q", st),
		"fmt %#v on State":        fmt.Sprintf("%#v", st),
		"fmt %v on a pointer":     fmt.Sprintf("%v", &st),
		"fmt %v on Secret":        fmt.Sprintf("%v", st.Proxy.Raw),
		"fmt %s on Secret":        fmt.Sprintf("%s", st.Hotspot.Passphrase),
		"fmt %q on Secret":        fmt.Sprintf("%q", st.Panel.PasswordHash),
		"fmt %#v on Secret":       fmt.Sprintf("%#v", st.Proxy.Raw),
		"fmt %v on ProxyConfig":   fmt.Sprintf("%v", st.Proxy),
		"fmt %v on HotspotConfig": fmt.Sprintf("%v", st.Hotspot),
		"fmt %v on PanelAuth":     fmt.Sprintf("%v", st.Panel),
		"fmt %v in a slice":       fmt.Sprintf("%v", []State{st}),
		"fmt %v in a map":         fmt.Sprintf("%v", map[string]State{"s": st}),
		"errors.New wrapping":     fmt.Errorf("some failure: %v", st).Error(),
	}

	for what, rendered := range renderings {
		for name, secret := range secrets {
			if strings.Contains(rendered, secret) {
				t.Errorf("%s leaked the %s", what, name)
			}
		}
	}

	// The redaction still has to be useful, or people will print the struct
	// instead.
	r := st.Redacted()
	for _, want := range []string{
		"proxy.configured=true",
		"proxy.scheme=" + fakeProxyScheme,
		"proxy.fingerprint=" + st.Proxy.Fingerprint(),
		"panel.password_set=true",
		"argon2id(m=",
		fakeSSID, // the SSID is public: it is broadcast
		DNSModeTunnel,
		OnTunnelDownBlock,
	} {
		if !strings.Contains(r, want) {
			t.Errorf("redacted rendering is missing %q, which diagnostics need:\n%s", want, r)
		}
	}
}

func TestFingerprintIdentifiesWithoutDisclosing(t *testing.T) {
	a := ProxyConfig{Raw: Secret(fakeProxyLink)}
	b := ProxyConfig{Raw: Secret(fakeProxyLink)}
	c := ProxyConfig{Raw: Secret(fakeProxyLink + "x")}

	if a.Fingerprint() != b.Fingerprint() {
		t.Error("the same config produced two fingerprints")
	}
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("two different configs produced the same fingerprint")
	}
	if (ProxyConfig{}).Fingerprint() != "" {
		t.Error("an unset config produced a fingerprint")
	}
	if strings.Contains(a.Fingerprint(), "192.0.2.10") {
		t.Error("the fingerprint contains part of the config")
	}
}

// TestSecretIsNotWrittenToTheLogByJSONEither checks the other direction: the
// on-disk file DOES carry the real values, because the engine has to replay
// them. The protection there is the file mode, not the encoding.
func TestOnDiskFileCarriesTheRealCredential(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	raw, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), fakeProxyLink) {
		t.Error("the state file does not contain the config; Secret's String method has broken persistence")
	}
	if strings.Contains(string(raw), redacted) {
		t.Error("the state file contains the redaction placeholder instead of the value")
	}
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("the state file is not valid JSON: %v", err)
	}
	if probe.Version != CurrentVersion {
		t.Errorf("on-disk version = %d, want %d", probe.Version, CurrentVersion)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("the state file does not end with a newline")
	}
}

// -------------------------------------------------------------- concurrency

// TestConcurrentReadsAndWrites is meaningful under -race, which the reported
// test run uses. It also checks the property that makes the lock-free read
// safe: a snapshot handed to a caller cannot be mutated by a later write.
func TestConcurrentReadsAndWrites(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetHotspot(fakeSSID, fakePassphrase); err != nil {
		t.Fatalf("SetHotspot: %v", err)
	}

	const readers, writers, iterations = 8, 4, 50
	var wg sync.WaitGroup

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				snap := st.Snapshot()
				// Every snapshot must be internally consistent: it is a whole
				// state, never a half-applied write.
				if snap.Version != CurrentVersion {
					t.Errorf("reader saw version %d", snap.Version)
					return
				}
				if snap.Hotspot.Passphrase.Reveal() != fakePassphrase {
					t.Error("reader saw a torn hotspot passphrase")
					return
				}
				if snap.Advanced.OnTunnelDown == "" {
					t.Error("reader saw an empty fail-closed policy")
					return
				}
				_ = st.Proxy()
				_ = st.Advanced()
				_ = st.NeedsSetup()
				_ = snap.Redacted()
			}
		}()
	}

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if err := st.Update(func(s *State) error {
					s.Advanced.Channel = id*100 + j
					s.Proxy.Label = fmt.Sprintf("writer-%d-%d", id, j)
					return nil
				}); err != nil {
					t.Errorf("Update: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// A snapshot taken before a write must not have been changed by it.
	snap := st.Snapshot()
	before := snap.Advanced.Channel
	if err := st.Update(func(s *State) error { s.Advanced.Channel = 9999; return nil }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if snap.Advanced.Channel != before {
		t.Error("a write mutated a snapshot already handed to a reader")
	}

	// And the file is still one valid state after all that concurrency.
	if _, err := Load(dir); err != nil {
		t.Fatalf("the state file did not survive concurrent writes: %v", err)
	}
	assertNoTempFiles(t, dir)
}

// TestUpdateDoesNotPublishAFailedEdit checks that a failing edit function
// leaves both the file and the in-memory state alone.
func TestUpdateDoesNotPublishAFailedEdit(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetHotspot(fakeSSID, fakePassphrase); err != nil {
		t.Fatalf("SetHotspot: %v", err)
	}

	sentinel := errors.New("edit refused")
	err = st.Update(func(s *State) error {
		s.Hotspot.SSID = "half-applied"
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Update error = %v, want the sentinel", err)
	}
	if got := st.Hotspot().SSID; got != fakeSSID {
		t.Errorf("SSID = %q after a refused edit, want %q", got, fakeSSID)
	}
}

// ---------------------------------------------------------------- invariants

func TestSaveRefusesAnEmptyFailClosedPolicy(t *testing.T) {
	tests := []struct {
		name string
		edit func(*State)
	}{
		{"empty DNS mode", func(s *State) { s.Advanced.DNSMode = "" }},
		{"empty tunnel-down policy", func(s *State) { s.Advanced.OnTunnelDown = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempStateDir(t)
			st, err := Load(dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			err = st.Update(func(s *State) error { tc.edit(s); return nil })
			if err == nil {
				t.Fatal("Update persisted an empty fail-closed policy field")
			}
			if !strings.Contains(err.Error(), "let client traffic out") {
				t.Errorf("the error does not explain why this matters: %v", err)
			}
			if _, statErr := os.Stat(st.Path()); !errors.Is(statErr, fs.ErrNotExist) {
				t.Error("the refused state was written to disk anyway")
			}
		})
	}
}

func TestLoadRejectsAnEmptyDirectoryArgument(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Error("Load(\"\") did not error")
	}
}

// ------------------------------------------------------------------ helpers

// tempStateDir returns a directory a Store can legitimately use.
//
// t.TempDir on its own is not one. Measured on this machine (Go 1.27.0,
// darwin/arm64, umask 022) it returns a directory at 0755, which Load correctly
// refuses. Tightening it here is test setup rather than a workaround: 0700 is
// what the installer is expected to create, and TestLoadRefusesLoosePermissions
// is the test that covers the loose case deliberately.
func tempStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, dirMode); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	return dir
}

func assertMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	if !permChecksEnforced {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat %s: %v", path, err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("%s has mode %#o, want %#o", path, got, want)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("a temporary file was left behind: %s", e.Name())
		}
	}
}

// TestTwoIndependentStoresNeverCorruptTheFile models the split in design
// section 5.5: the unprivileged panel and the privileged network service are
// separate processes, so they do NOT share the Store mutex. Two Stores over one
// directory is that situation exactly.
//
// What this proves: the file is never torn. Each writer creates its own
// uniquely named temporary file and renames it over the target, so every
// observer sees one complete state.
//
// What this does NOT prove, and what is a real limitation of this package: the
// last writer wins. A read-modify-write in one process can silently discard a
// change another process made in between, because there is no cross-process
// lock. That is a lost update, not corruption. It is acceptable only while one
// process writes; if both ever write, this package needs an advisory file lock.
// The test asserts the property that actually holds rather than the one that
// would be nice.
func TestTwoIndependentStoresNeverCorruptTheFile(t *testing.T) {
	dir := tempStateDir(t)

	seed, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := seed.SetHotspot(fakeSSID, fakePassphrase); err != nil {
		t.Fatalf("SetHotspot: %v", err)
	}

	const writers, iterations = 4, 25
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// A Store of its own, with its own mutex: no shared lock, which is
			// what a second process looks like.
			own, err := Load(dir)
			if err != nil {
				t.Errorf("Load: %v", err)
				return
			}
			for j := 0; j < iterations; j++ {
				if err := own.Update(func(s *State) error {
					s.Proxy.Label = fmt.Sprintf("writer-%d-%d", id, j)
					s.Advanced.Channel = id + 1
					return nil
				}); err != nil {
					t.Errorf("Update: %v", err)
					return
				}
				// Every read, at any moment, must yield a complete state.
				check, err := Load(dir)
				if err != nil {
					t.Errorf("the state file was unreadable mid-flight: %v", err)
					return
				}
				got := check.Snapshot()
				if got.Version != CurrentVersion ||
					got.Hotspot.Passphrase.Reveal() != fakePassphrase ||
					got.Advanced.OnTunnelDown == "" {
					t.Error("a reader saw a torn or partial state")
					return
				}
			}
		}(i)
	}
	wg.Wait()

	final, err := Load(dir)
	if err != nil {
		t.Fatalf("the state file did not survive independent writers: %v", err)
	}
	if final.Snapshot().Hotspot.Passphrase.Reveal() != fakePassphrase {
		t.Error("a field no writer touched was lost")
	}
	assertNoTempFiles(t, dir)
}
