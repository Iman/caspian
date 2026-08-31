// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package goldenscan

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// moduleRoot walks up from the working directory to the directory holding
// go.mod, the same way scripts/gate.sh finds it.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod above the working directory")
		}
		dir = parent
	}
}

// ---------------------------------------------------------------------------
// The scan itself
// ---------------------------------------------------------------------------

// TestNoCommittedFixtureCarriesACredential is the gate.
//
// It walks every testdata directory in the module and fails on any hit that is
// neither allowlisted nor in the open register. A sentinel hit fails
// unconditionally: no allowlist entry can suppress one, because a sentinel can
// only be present if something wrote a credential into a fixture.
func TestNoCommittedFixtureCarriesACredential(t *testing.T) {
	root := moduleRoot(t)
	dirs, err := Roots(root)
	if err != nil {
		t.Fatalf("finding testdata directories: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("no testdata directories found, so this scan would pass vacuously")
	}
	for _, d := range dirs {
		rel, _ := filepath.Rel(root, d)
		t.Logf("scanning %s", filepath.ToSlash(rel))
	}

	findings, err := Scan(dirs, root)
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}

	var fail []Finding
	open := 0
	for _, f := range findings {
		if o, ok := IsOpenFinding(f); ok {
			open++
			t.Logf("OPEN FINDING (not failing this scan): %s\n"+
				"    recorded: %s\n"+
				"    owner:    %s\n"+
				"    detail:   %s", f, o.Recorded, o.Owner, o.Detail)
			continue
		}
		fail = append(fail, f)
	}

	if len(fail) > 0 {
		var b strings.Builder
		for _, f := range fail {
			b.WriteString("  " + f.String() + "\n")
			b.WriteString("      " + ClassWhy(f.Class) + "\n")
		}
		t.Errorf("%d credential-shaped value(s) in committed fixtures:\n%s\n"+
			"A committed golden is permanent: once a credential reaches git history, deleting\n"+
			"it from the working tree deletes nothing.\n\n"+
			"If a hit is an invented fixture, add an entry to allowlist() in registry.go that\n"+
			"names the file that defines the value, so the claim can be checked.\n"+
			"If it is real, or you cannot tell, it does not belong in a commit.",
			len(fail), b.String())
	}
	t.Logf("scan complete: %d file(s) of findings failed, %d open finding(s) reported", len(fail), open)
}

// ---------------------------------------------------------------------------
// PROVING THE GUARD CAN FAIL
//
// A scanner that reports CLEAN proves nothing until it has been watched
// catching something. These tests plant one credential of every class into a
// temporary tree and assert each is found. They run on every gate, so the proof
// is repeated rather than remembered.
// ---------------------------------------------------------------------------

// TestScannerCatchesAPlantedSecretOfEveryClass is the proof.
func TestScannerCatchesAPlantedSecretOfEveryClass(t *testing.T) {
	cases := []struct {
		class string
		body  string
	}{
		{"uuid", `{"id": "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"}`},
		{"base64-32byte-key", `{"publicKey": "Q0FTUElBTi1QTEFOVEVELVJFQUxJVFktS0VZLTMyYnQ"}`},
		{"wpa-passphrase", "interface=wlan0\nwpa_passphrase=planted-wifi-key-here\n"},
		{"share-link", "vless://aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee@198.51.100.4:443#planted"},
		{"pem-private-key", "-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----"},
		{"argon2-hash", "password_hash: $argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"},
		{"routable-ipv4", "server = 198.51.101.7\n"},
	}

	// Every declared class must be planted. A class added to scan.go without a
	// planted case here is a class nobody has watched fire.
	planted := map[string]bool{}
	for _, c := range cases {
		planted[c.class] = true
	}
	for _, name := range ShapeClassNames() {
		if !planted[name] {
			t.Errorf("class %q has no planted case, so nothing has ever watched it fire", name)
		}
	}

	for _, c := range cases {
		t.Run(c.class, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "internal", "planted", "testdata")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			// A .txt file, so no allowlist entry can match: every entry above
			// names a specific path.
			path := filepath.Join(dir, "planted.txt")
			if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			findings, err := Scan([]string{dir}, root)
			if err != nil {
				t.Fatalf("scanning: %v", err)
			}
			var got []string
			for _, f := range findings {
				got = append(got, f.Class)
			}
			if !contains(got, c.class) {
				t.Fatalf("planted a %s and the scanner did not report it. Classes reported: %v.\n"+
					"A scanner that misses a planted secret is a scanner whose clean result "+
					"means nothing.", c.class, got)
			}
			t.Logf("planted a %s, scanner reported: %v", c.class, got)
		})
	}
}

// TestScannerCatchesAPlantedSentinel is the same proof for the strong check.
func TestScannerCatchesAPlantedSentinel(t *testing.T) {
	for _, s := range Sentinels() {
		t.Run(s.Name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "internal", "planted", "testdata")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			body := "a line that should never exist: " + s.Value + "\n"
			if err := os.WriteFile(filepath.Join(dir, "planted.txt"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			findings, err := Scan([]string{dir}, root)
			if err != nil {
				t.Fatalf("scanning: %v", err)
			}
			hit := false
			for _, f := range findings {
				if f.Class == "sentinel" && f.Name == s.Name {
					hit = true
				}
			}
			if !hit {
				t.Fatalf("planted the %s and the scanner did not report it", s.Name)
			}
		})
	}
}

// TestAPlantedSentinelCannotBeAllowlisted holds the one rule that has no
// exceptions.
//
// The allowlist exists because shape classes cannot tell a fixture from a
// credential. Sentinels can, so allowing one would be allowing a known
// credential into a commit. This plants a sentinel in a path that every
// allowlist entry matches and asserts it is still reported.
func TestAPlantedSentinelCannotBeAllowlisted(t *testing.T) {
	s := Sentinels()[0]
	root := t.TempDir()
	// The most permissive path in the allowlist: an xcfg golden, which is
	// allowed to carry both a uuid and a 32 byte key.
	dir := filepath.Join(root, "internal", "xcfg", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"password": "` + s.Value + `", "id": "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"}`
	if err := os.WriteFile(filepath.Join(dir, "planted.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := Scan([]string{dir}, root)
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	sentinelHit, uuidHit := false, false
	for _, f := range findings {
		if f.Class == "sentinel" {
			sentinelHit = true
		}
		if f.Class == "uuid" {
			uuidHit = true
		}
	}
	if !sentinelHit {
		t.Error("a sentinel planted in an allowlisted path was suppressed. No allowlist entry " +
			"may ever suppress a sentinel: a sentinel can only be present if a credential was " +
			"written into a fixture.")
	}
	if uuidHit {
		t.Error("the uuid in an allowlisted path was reported, so the allowlist is not being " +
			"applied at all and the test above proved nothing")
	}
}

// TestScannerCatchesASecretInAFileName is the half hw_leakscan added after the
// body check, because a fixture named after a config label carries an address
// into a path where no body scan will ever look at it.
func TestScannerCatchesASecretInAFileName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "planted", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "capture-198.51.101.7-run.txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("nothing in the body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := Scan([]string{dir}, root)
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	for _, f := range findings {
		if f.Line == 0 && f.Class == "routable-ipv4" {
			t.Logf("filename hit reported as expected: %s", f)
			return
		}
	}
	t.Fatalf("an address in a FILE NAME was not reported; findings were %v", findings)
}

// TestNonRoutableAddressesAreNotReported is the negative control.
//
// A scanner that fires on every address reports everything and is therefore
// switched off within a week. This asserts the ranges that fill the hotspot and
// netcfg fixtures are silent, so that a hit means something.
func TestNonRoutableAddressesAreNotReported(t *testing.T) {
	quiet := []string{
		"0.0.0.0", "10.62.0.1", "127.0.0.1", "169.254.1.1", "172.16.0.1",
		"192.168.66.50", "192.0.2.1", "198.51.100.4", "203.0.113.10",
		"198.18.51.1", "100.64.0.1", "224.0.0.251", "255.255.255.0",
		"999.999.999.999",
	}
	for _, a := range quiet {
		if !nonRoutable(a) {
			t.Errorf("%s is reported as routable; it is not, and the noise would make the "+
				"scan unusable", a)
		}
	}
	loud := []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "198.51.101.7", "143.20.69.40"}
	for _, a := range loud {
		if nonRoutable(a) {
			t.Errorf("%s is treated as non-routable, so a real server address in that range "+
				"would pass the scan silently", a)
		}
	}
}

// ---------------------------------------------------------------------------
// Guards on the registry itself
// ---------------------------------------------------------------------------

// TestEverySentinelIsStillDeclaredWhereItSaysItIs closes the failure that would
// otherwise make this whole package report CLEAN while covering nothing.
//
// The sentinel values live in two places: the test that feeds them to the
// product, and this registry. If the first renames its constant and this one is
// not updated, the scan keeps looking for a string that no longer exists and
// reports clean forever. That is the same shape as a suite reporting success
// having executed zero tests, and it is caught by reading the declaring file.
func TestEverySentinelIsStillDeclaredWhereItSaysItIs(t *testing.T) {
	root := moduleRoot(t)
	if len(Sentinels()) == 0 {
		t.Fatal("the sentinel registry is empty, so the strong half of the scan covers nothing")
	}
	for _, s := range Sentinels() {
		path := filepath.Join(root, filepath.FromSlash(s.DeclaredIn))
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s says it is declared in %s, which cannot be read: %v", s.Name, s.DeclaredIn, err)
			continue
		}
		if !strings.Contains(string(body), s.Value) {
			t.Errorf("%s is not present in %s any more. The scan is looking for a value nothing "+
				"produces, so it would report CLEAN while covering nothing. Update the registry "+
				"in test/goldenscan/registry.go to match the constant in that file.",
				s.Name, s.DeclaredIn)
		}
	}
}

// TestSentinelsAreUniqueAndDistinctive.
//
// A sentinel that is short, or that is a substring of another, cannot give an
// unambiguous hit, which is the only property that makes the strong check
// strong.
func TestSentinelsAreUniqueAndDistinctive(t *testing.T) {
	seen := map[string]string{}
	for _, s := range Sentinels() {
		if len(s.Value) < 12 {
			t.Errorf("%s is only %d characters; a short sentinel produces ambiguous hits",
				s.Name, len(s.Value))
		}
		if prev, ok := seen[s.Value]; ok {
			t.Errorf("%s and %s are the same value, so a hit cannot say which one leaked",
				prev, s.Name)
		}
		seen[s.Value] = s.Name
		for _, other := range Sentinels() {
			if other.Value == s.Value {
				continue
			}
			if strings.Contains(other.Value, s.Value) {
				t.Errorf("%s is a substring of %s, so a hit on the longer one also reports the "+
					"shorter and the two cannot be told apart", s.Name, other.Name)
			}
		}
	}
}

// TestAllowlistEntriesAreAllUsed stops the list rotting into a blanket
// permission.
//
// An entry that matches nothing is an entry nobody will notice becoming wrong.
// The failure names the entry, so the fix is to delete it rather than to widen
// it.
func TestAllowlistEntriesAreAllUsed(t *testing.T) {
	root := moduleRoot(t)
	dirs, err := Roots(root)
	if err != nil {
		t.Fatal(err)
	}
	// Scan with the allowlist NOT applied, so every raw hit is visible.
	var raw []Finding
	for _, d := range dirs {
		err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			raw = append(raw, scanName(rel)...)
			if info.IsDir() || skipExt[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			raw = append(raw, scanBody(rel, string(b))...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	privacyClass := map[string]bool{}
	for _, c := range PrivacyClassNames() {
		privacyClass[c] = true
	}
	for _, a := range Allowlist() {
		if a.Class == "sentinel-not-applicable" {
			// A documentation-only entry. It records a deliberate exclusion and
			// is not expected to match a finding.
			continue
		}
		if privacyClass[a.Class] {
			// A privacy-class entry is used by the REPOSITORY walk, not by this
			// testdata walk, so checking it here would report every one of them
			// as dead. TestEveryPrivacyAllowlistEntryIsUsed does the same job
			// against the right scan, and splitting them means a failure names
			// which scan the dead entry belonged to.
			continue
		}
		used := false
		for _, f := range raw {
			if f.Class == a.Class && match(a.PathGlob, f.Path) {
				used = true
				break
			}
		}
		if !used {
			t.Errorf("allowlist entry %s / %s matches nothing any more. Delete it: an entry that "+
				"matches nothing is a permission nobody will notice becoming wrong.\n  reason was: %s",
				a.PathGlob, a.Class, a.Reason)
		}
	}
}

// TestAllowlistEntriesExplainThemselves refuses an entry whose reason is not a
// reason.
//
// "known good", "fixture", "ok" and their relatives are how an allowlist stops
// being reviewable. Every entry has to say what the value is and where it is
// defined, so a reader can check the claim instead of taking it.
func TestAllowlistEntriesExplainThemselves(t *testing.T) {
	shrug := regexp.MustCompile(`(?i)^\s*(ok|fine|known good|fixture|test data|not a secret)\s*\.?\s*$`)
	for _, a := range Allowlist() {
		if len(a.Reason) < 40 {
			t.Errorf("allowlist entry %s / %s has a %d character reason, which is too short to "+
				"say what the value is and where it is defined", a.PathGlob, a.Class, len(a.Reason))
		}
		if shrug.MatchString(a.Reason) {
			t.Errorf("allowlist entry %s / %s has a reason that says nothing: %q",
				a.PathGlob, a.Class, a.Reason)
		}
		if a.Class == "sentinel" {
			t.Errorf("allowlist entry %s allows the sentinel class. That can never be right: a "+
				"sentinel is present only if a credential was written into a fixture", a.PathGlob)
		}
	}
}

// TestOpenFindingsAreDatedAndOwned refuses an entry that cannot be actioned.
//
// An open finding is a decision somebody has to make. Without a date it cannot
// age, and without an owner it belongs to nobody and stays open forever, which
// turns the register into the allowlist it exists not to be.
func TestOpenFindingsAreDatedAndOwned(t *testing.T) {
	date := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	for _, o := range OpenFindings() {
		if !date.MatchString(o.Recorded) {
			t.Errorf("open finding %s / %s has Recorded=%q, which is not a YYYY-MM-DD date",
				o.PathGlob, o.Class, o.Recorded)
		}
		if len(strings.TrimSpace(o.Owner)) < 8 {
			t.Errorf("open finding %s / %s has no owner, so it belongs to nobody",
				o.PathGlob, o.Class)
		}
		if len(o.Detail) < 80 {
			t.Errorf("open finding %s / %s has a %d character detail, which cannot say what the "+
				"finding is and what acting on it would mean", o.PathGlob, o.Class, len(o.Detail))
		}
	}
}

// TestEveryTestdataDirectoryHasAProvenanceFile.
//
// internal/netcfg established the rule and internal/panel/qr follows it: a
// fixture nobody can trace is a fixture whose class nobody can tell, so it
// silently becomes evidence it is not. This applies it to every testdata
// directory in the module at once, so a new one arrives with the record rather
// than without it.
func TestEveryTestdataDirectoryHasAProvenanceFile(t *testing.T) {
	root := moduleRoot(t)
	dirs, err := Roots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no testdata directories found, so this guard passed vacuously")
	}
	for _, d := range dirs {
		rel, _ := filepath.Rel(root, d)
		if _, err := os.Stat(filepath.Join(d, "PROVENANCE.md")); err != nil {
			t.Errorf("%s has no PROVENANCE.md. Every fixture directory records where its "+
				"contents came from and what a diff in them means; without it, a green test "+
				"proves something about bytes somebody wrote rather than about the product.",
				filepath.ToSlash(rel))
		}
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
