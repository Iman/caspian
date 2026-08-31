// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package goldenscan

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The gate
// ---------------------------------------------------------------------------

// TestRepositoryCarriesNobodysAddress is the privacy gate.
//
// It walks the WHOLE module, not only its testdata directories, and fails on
// any MAC address, routable IPv4 address or removed value that is neither
// covered by an in-code exception naming the value nor by an allowlist entry
// naming the file and the reason.
//
// The scope is the point. When the values this scan exists to keep out were
// removed on 2026-08-31 they were in test assertions, package doc comments, a
// hardware harness's shell comments and a provenance file, and four of about
// forty sites were under a testdata directory. A testdata-only walk reports
// CLEAN over that.
func TestRepositoryCarriesNobodysAddress(t *testing.T) {
	root := moduleRoot(t)

	findings, err := ScanRepo(root, PrivacySentinels())
	if err != nil {
		t.Fatalf("scanning the repository: %v", err)
	}
	if len(findings) == 0 {
		t.Log("privacy scan clean")
	}

	if len(findings) > 0 {
		var b strings.Builder
		for _, f := range findings {
			b.WriteString("  " + f.String() + "\n")
			b.WriteString("      " + PrivacyClassWhy(f.Class) + "\n")
		}
		t.Errorf("%d value(s) in this repository identify a person, a home or a device:\n%s\n"+
			"This repository is published. A router BSSID alone places a home within metres,\n"+
			"because public WiFi geolocation services index them; a network name beside one\n"+
			"removes the last ambiguity.\n\n"+
			"Substitute the value. The table of substitutes, and what each stands for, is in\n"+
			"internal/netcfg/testdata/PROVENANCE.md; use the values in it rather than inventing\n"+
			"new ones, because several tests depend on two interfaces differing.\n\n"+
			"If the literal is genuinely not an address (an RFC section number, a version\n"+
			"string, the base of a routing prefix), add an entry to allowlist() in registry.go\n"+
			"that says which literal it is and why. A privacy-sentinel hit cannot be\n"+
			"allowlisted: it can only be present if a substitution was reverted.",
			len(findings), b.String())
	}
}

// TestScanRepoWalksMoreThanTestdata stops the gate above passing because it
// looked at nothing.
//
// A walk that silently excluded, say, internal/ would report clean forever.
// This asserts the walk reaches the four kinds of file the removed values were
// actually found in.
func TestScanRepoWalksMoreThanTestdata(t *testing.T) {
	root := moduleRoot(t)
	want := []string{
		"internal/netcfg/release_test.go",                 // a test assertion
		"internal/privsvc/readback.go",                    // a package doc comment
		"test/hardware/lib/phone.sh",                      // a shell comment
		"internal/netcfg/testdata/PROVENANCE.md",          // a provenance file
		"internal/netcfg/testdata/capture-pi5-iw-dev.txt", // a captured fixture
	}
	seen := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if ok, _ := PrivacyRoots(rel); !ok {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("the privacy walk does not reach %s. The values removed on 2026-08-31 were "+
				"in files of exactly this kind, so a walk that misses it reports CLEAN over the "+
				"thing the scan exists to find.", w)
		}
	}
}

// ---------------------------------------------------------------------------
// PROVING THE GUARD CAN FAIL
//
// Same discipline as TestScannerCatchesAPlantedSecretOfEveryClass: a scanner
// that reports CLEAN proves nothing until it has been watched catching
// something, and the proof runs on every gate rather than being remembered.
// ---------------------------------------------------------------------------

// plantedSentinel is a value invented for this test alone.
//
// The registry cannot be used to prove itself. Its entries pin digests
// PRECISELY so that the values are not in this repository, so a test that
// planted a real one would put it back into the source tree, which is the thing
// the registry exists to prevent. This proves the MECHANISM instead: a value of
// the same shape, hashed the same way, found by the same walk.
//
// The real digests were watched firing once, by hand, on 2026-08-31, against
// the pre-substitution content taken from git rather than from any file in the
// working tree. That is the strongest proof available without re-adding a value
// and it is not repeatable from source, which is why the automated half is this
// one.
const plantedSentinel = "PlantedPrivacyValue4rk9"

func plantedSentinelEntry() PrivacySentinel {
	sum := sha256.Sum256([]byte(strings.ToLower(plantedSentinel)))
	return PrivacySentinel{
		Name:   "planted test value",
		SHA256: hex.EncodeToString(sum[:]),
		Len:    len(plantedSentinel),
		Why:    "invented for TestPrivacyScannerCatchesAPlantOfEveryClass and used nowhere else",
	}
}

// TestPrivacyScannerCatchesAPlantOfEveryClass is the proof.
func TestPrivacyScannerCatchesAPlantOfEveryClass(t *testing.T) {
	cases := []struct {
		class string
		body  string
		why   string
	}{
		{
			class: ClassNonLocalMAC,
			body:  "    link/ether b8:27:eb:11:22:33 brd ff:ff:ff:ff:ff:ff\n",
			why:   "a manufacturer-assigned address: b8:27:eb is the Raspberry Pi Foundation",
		},
		{
			class: ClassUnregisteredMAC,
			body:  "Connected to ae:11:22:33:44:55 (on wlan0)\n",
			why: "locally administered, so non-local-mac is silent, and outside the substitute " +
				"block. This is the shape of the home router BSSID that was removed",
		},
		{
			class: ClassPublicIPv4,
			body:  "server = 198.51.101.7\n",
			why:   "routable and in no documentation range",
		},
		{
			class: ClassPrivacySentinel,
			body:  "\t\tssid " + plantedSentinel + "\n",
			why:   "a value whose digest is registered, in the position a capture puts an SSID",
		},
	}

	// Every declared class must be planted. A class added to privacy.go with no
	// planted case here is a class nobody has watched fire.
	planted := map[string]bool{}
	for _, c := range cases {
		planted[c.class] = true
	}
	for _, name := range PrivacyClassNames() {
		if !planted[name] {
			t.Errorf("privacy class %q has no planted case, so nothing has ever watched it fire", name)
		}
	}

	sentinels := append(PrivacySentinels(), plantedSentinelEntry())

	for _, c := range cases {
		t.Run(c.class, func(t *testing.T) {
			root := t.TempDir()
			// internal/planted/, NOT a testdata directory: the whole claim
			// being proved is that this scan reaches past testdata.
			dir := filepath.Join(root, "internal", "planted")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "planted.txt")
			if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			findings, err := ScanRepo(root, sentinels)
			if err != nil {
				t.Fatalf("scanning: %v", err)
			}
			var got []string
			for _, f := range findings {
				got = append(got, f.Class)
			}
			if !contains(got, c.class) {
				t.Fatalf("planted a %s (%s) and the scanner did not report it. Classes reported: "+
					"%v.\nA scanner that misses a planted value is a scanner whose clean result "+
					"means nothing.", c.class, c.why, got)
			}
			t.Logf("planted a %s (%s), scanner reported: %v", c.class, c.why, got)

			// And the plant removed again leaves the tree clean, so the hit
			// above was the plant and not something the temporary tree carries
			// on its own.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			after, err := ScanRepo(root, sentinels)
			if err != nil {
				t.Fatal(err)
			}
			if len(after) != 0 {
				t.Fatalf("with the plant removed the scan still reports %v, so the hit above did "+
					"not prove the plant was what was found", after)
			}
		})
	}
}

// TestPrivacyScannerCatchesAValueInAFileName is the other half, for the same
// reason scanName exists on the credential side: a capture named after the
// network it was taken on carries the network name into a path, where no body
// scan will look at it.
func TestPrivacyScannerCatchesAValueInAFileName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "planted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "capture-" + plantedSentinel + "-iw-dev.txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("nothing in the body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := ScanRepo(root, append(PrivacySentinels(), plantedSentinelEntry()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Line == 0 && f.Class == ClassPrivacySentinel {
			t.Logf("filename hit reported as expected: %s", f)
			return
		}
	}
	t.Fatalf("a removed value in a FILE NAME was not reported; findings were %v", findings)
}

// TestAPlantedPrivacySentinelCannotBeAllowlisted holds the rule that has no
// exceptions, for the privacy half.
//
// A shape class cannot tell a fixture from a real reading, so it needs an
// allowlist. A privacy sentinel can: it is a specific value that was removed
// from this repository, and it can only be back because a substitution was
// reverted. This plants one in the most permissive allowlisted path there is
// and asserts it is still reported.
func TestAPlantedPrivacySentinelCannotBeAllowlisted(t *testing.T) {
	root := t.TempDir()
	// internal/xcfg/private.go is allowlisted for public-ipv4.
	dir := filepath.Join(root, "internal", "xcfg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "// RFC 1122 section 3.2.1.3, ssid " + plantedSentinel + "\n"
	if err := os.WriteFile(filepath.Join(dir, "private.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := ScanRepo(root, append(PrivacySentinels(), plantedSentinelEntry()))
	if err != nil {
		t.Fatal(err)
	}
	sentinelHit, ipHit := false, false
	for _, f := range findings {
		switch f.Class {
		case ClassPrivacySentinel:
			sentinelHit = true
		case ClassPublicIPv4:
			ipHit = true
		}
	}
	if !sentinelHit {
		t.Error("a privacy sentinel planted in an allowlisted path was suppressed. No allowlist " +
			"entry may suppress one: it is present only if a removed value came back")
	}
	if ipHit {
		t.Error("the allowlisted public-ipv4 literal in that path was reported, so the allowlist " +
			"is not being applied at all and the assertion above proved nothing")
	}
}

// TestDocumentationAddressesAreNotReported is the negative control.
//
// A scan that fires on every address is switched off within a week, so the
// ranges that fill the hotspot and netcfg fixtures have to be silent for a hit
// to mean anything. The loud half asserts the inverse: the ranges a real server
// or a household address lives in must NOT be quiet.
func TestDocumentationAddressesAreNotReported(t *testing.T) {
	quiet := []string{
		"0.0.0.0", "10.0.0.221", "10.83.51.1", "127.0.0.1", "169.254.1.1",
		"172.16.0.1", "192.168.66.50", "192.0.2.1", "198.51.100.10",
		"203.0.113.20", "198.18.51.1", "100.64.0.1", "224.0.0.251",
		"255.255.255.255", "999.999.999.999",
	}
	for _, a := range quiet {
		if !documentationIPv4(a) {
			t.Errorf("%s is reported as identifying; it is not, and the noise would make the "+
				"scan unusable", a)
		}
	}
	// The two substitutes above are the ones the removed addresses became.
	// They have to be quiet or the substitution could not have been made.
	loud := []string{"143.20.69.40", "34.117.59.81", "93.184.216.34", "198.51.101.7", "172.66.157.237"}
	for _, a := range loud {
		if documentationIPv4(a) {
			t.Errorf("%s is treated as a documentation address, so a real server address in that "+
				"range would pass the scan silently", a)
		}
	}
}

// TestLocallyAdministeredIsTheSecondHexDigit pins the bit test, because the
// whole non-local-mac class is that one bit and an inverted test would report
// clean over every manufacturer address there is.
func TestLocallyAdministeredIsTheSecondHexDigit(t *testing.T) {
	local := []string{
		"02:00:5e:00:00:01", "06:11:22:33:44:55", "0a:11:22:33:44:55",
		"0e:11:22:33:44:55", "12:34:56:78:9a:bc", "aa:bb:cc:dd:ee:ff",
		"fe:11:22:33:44:55",
	}
	for _, m := range local {
		if !locallyAdministered(m) {
			t.Errorf("%s has the locally-administered bit SET and was read as manufacturer "+
				"assigned", m)
		}
	}
	global := []string{
		"00:11:22:33:44:55", "b8:27:eb:11:22:33", "dc:a6:32:11:22:33",
		"a4:83:e7:11:22:33", "fc:11:22:33:44:55", "01:11:22:33:44:55",
	}
	for _, m := range global {
		if locallyAdministered(m) {
			t.Errorf("%s has the locally-administered bit CLEAR and was read as invented, so a "+
				"manufacturer address would pass the scan silently", m)
		}
	}
}

// TestAClientIdentifierIsReadAsItsTrailingAddress.
//
// A DHCP client identifier is 01:<six octets> and a DHCPv6 DUID-LLT is
// 00:01:<four octets of time>:<six octets of hardware address>. A six-octet
// pattern anchored with word boundaries reports the FIRST six octets of a DUID,
// which are a timestamp, and says nothing about the hardware address in the
// last six. That is a loud false positive that hides a real one, and it is the
// failure this test pins.
func TestAClientIdentifierIsReadAsItsTrailingAddress(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"a bare address", "addr 02:00:5e:00:00:11", "02:00:5e:00:00:11"},
		{"a dhcp client identifier", "lease 01:b8:27:eb:11:22:33 host", "b8:27:eb:11:22:33"},
		{"a dhcpv6 duid", "duid 00:01:00:01:2d:9f:3c:11:b8:27:eb:11:22:33", "b8:27:eb:11:22:33"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := macsIn(tc.line)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("macsIn(%q) = %v, want exactly [%s]", tc.line, got, tc.want)
			}
		})
	}

	// And the negative: fewer than six octets is not an address.
	for _, line := range []string{
		"captured 2026-08-30 00:48 BST",
		"took 00:01:02 wall clock",
		"ports 80:443",
	} {
		if got := macsIn(line); len(got) != 0 {
			t.Errorf("macsIn(%q) = %v; a run shorter than six octets is a clock time or a port "+
				"list, and reporting it would make the class noise", line, got)
		}
	}

	// A DUID whose trailing address is invented must be silent, or every IPv6
	// lease fixture becomes a permanent finding.
	quiet := "duid 00:01:00:01:2d:9f:3c:11:02:00:5e:02:00:05"
	got := macsIn(quiet)
	if len(got) != 1 || locallyAdministered(got[0]) == false {
		t.Errorf("macsIn(%q) = %v; the trailing address is in the substitute block and must be "+
			"read as such", quiet, got)
	}
}

// ---------------------------------------------------------------------------
// Guards on the privacy registry itself
// ---------------------------------------------------------------------------

// TestPrivacySentinelsAreDigestsAndExplainThemselves.
//
// The registry is only useful if every row can be acted on without being told
// the value, which means every row needs a Why that says what the value WAS and
// what to put in its place. And every row must be a digest: a row holding a
// plaintext value would put the value back into the published tree, which is
// the failure this whole file exists to prevent.
func TestPrivacySentinelsAreDigestsAndExplainThemselves(t *testing.T) {
	if len(PrivacySentinels()) == 0 {
		t.Fatal("the privacy sentinel registry is empty, so a revert of any removed value would " +
			"be caught only by the structural classes")
	}
	hexDigest := regexp.MustCompile(`^[0-9a-f]{64}$`)
	seen := map[string]string{}
	for _, s := range PrivacySentinels() {
		if !hexDigest.MatchString(s.SHA256) {
			t.Errorf("%s has SHA256=%q, which is not 64 lowercase hex characters. A row that is "+
				"not a digest is a row holding the value itself", s.Name, s.SHA256)
		}
		if s.Len < 4 {
			t.Errorf("%s has Len=%d; a window that short produces ambiguous hits", s.Name, s.Len)
		}
		if len(s.Why) < 40 {
			t.Errorf("%s has a %d character Why. A failure names the row and never the value, so "+
				"the Why is the only thing a reader has to act on", s.Name, len(s.Why))
		}
		if prev, ok := seen[s.SHA256]; ok {
			t.Errorf("%s and %s pin the same digest, so a hit cannot say which one came back",
				prev, s.Name)
		}
		seen[s.SHA256] = s.Name
	}
}

// TestNoPrivacySentinelIsAMACOrAnIPv4.
//
// This pins the decision recorded on the PrivacySentinel doc comment. A digest
// of a low-entropy value is confirmable by anyone who can guess it, so a digest
// is only worth storing where no structural class already covers the value.
// unregistered-mac covers every MAC and public-ipv4 covers every routable
// address, so a row for either would add nothing to the guard while making the
// value confirmable: an IPv4 exhaustively in seconds, and a router BSSID
// against a public WiFi-geolocation dump, which is the one value in this
// incident that places a house.
//
// The test cannot read the values, so it checks the property it can: no row has
// the LENGTH of a MAC or of an IPv4 literal written in full.
func TestNoPrivacySentinelIsAMACOrAnIPv4(t *testing.T) {
	for _, s := range PrivacySentinels() {
		if s.Len == len("02:00:5e:00:00:01") {
			t.Errorf("%s is %d bytes, the length of a MAC address. unregistered-mac already "+
				"catches ANY address outside the substitute block, which is strictly better "+
				"coverage than a digest of one known address, and the digest would make a router "+
				"BSSID confirmable. See the PrivacySentinel doc comment.", s.Name, s.Len)
		}
		if s.Len >= len("0.0.0.0") && s.Len <= len("255.255.255.255") {
			// Not conclusive on its own: several removed strings are in this
			// length range. Only flag it alongside the note, so this reads as
			// a prompt rather than a false accusation.
			t.Logf("%s is %d bytes, which is also an IPv4 literal's range. If it IS an address, "+
				"remove the row: public-ipv4 already covers it and 2^32 is exhaustible in "+
				"seconds.", s.Name, s.Len)
		}
	}
}

// TestEveryPrivacyExceptionSaysWhy covers the two in-code maps and the walk
// exclusions with the same rule the allowlist has: an exception nobody can
// check is a permission.
func TestEveryPrivacyExceptionSaysWhy(t *testing.T) {
	for value, why := range KnownPublicAddresses() {
		if len(why) < 20 {
			t.Errorf("knownPublicAddresses[%s] has a %d character reason. Every entry has to say "+
				"which service the address is and where this repository chose it, because the "+
				"test that separates a public resolver from a user's server is that the resolver "+
				"is the same on every box", value, len(why))
		}
	}
	for value, why := range RegisteredMACs() {
		if len(why) < 20 {
			t.Errorf("registeredMACs[%s] has a %d character reason", value, len(why))
		}
	}
	for dir, why := range PrivacySkipReasons() {
		if len(why) < 20 {
			t.Errorf("the privacy walk skips %s with a %d character reason. An exclusion without "+
				"one reads as an oversight, and the next person widens it", dir, len(why))
		}
	}
}

// TestNoKnownPublicAddressIsPrivate stops the exception map being used as a
// second, unreviewed allowlist.
//
// Every entry in it must actually be a routable address, because an entry for a
// private one would be a row nobody notices is dead.
func TestNoKnownPublicAddressIsPrivate(t *testing.T) {
	for value := range KnownPublicAddresses() {
		if documentationIPv4(value) {
			t.Errorf("knownPublicAddresses names %s, which documentationIPv4 already treats as "+
				"non-identifying. The row is dead: delete it", value)
		}
	}
}

// TestEveryPrivacyAllowlistEntryIsUsed is TestAllowlistEntriesAreAllUsed for
// the privacy classes, whose findings come from the repository walk rather than
// from the testdata walk.
//
// Split into its own test rather than folded into that one so that a failure
// names which scan the dead entry belonged to.
func TestEveryPrivacyAllowlistEntryIsUsed(t *testing.T) {
	root := moduleRoot(t)
	raw, err := rawPrivacyFindings(root)
	if err != nil {
		t.Fatal(err)
	}
	privacyClass := map[string]bool{}
	for _, c := range PrivacyClassNames() {
		privacyClass[c] = true
	}
	for _, a := range Allowlist() {
		if !privacyClass[a.Class] {
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
			t.Errorf("privacy allowlist entry %s / %s matches nothing any more. Delete it: an "+
				"entry that matches nothing is a permission nobody will notice becoming wrong.\n"+
				"  reason was: %s", a.PathGlob, a.Class, a.Reason)
		}
	}
}

// rawPrivacyFindings is the repository walk with the allowlist NOT applied.
func rawPrivacyFindings(root string) ([]Finding, error) {
	var raw []Finding
	sentinels := PrivacySentinels()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		walk, shapeExempt, _ := privacyScope(rel)
		if !walk {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		opt := scanOpts{applyLiteralAllow: false, sentinelsOnly: shapeExempt}
		raw = append(raw, scanPrivacyName(rel, sentinels, opt)...)
		if info.IsDir() || skipExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		raw = append(raw, scanPrivacyBody(rel, string(b), sentinels, opt)...)
		return nil
	})
	return raw, err
}

// TestTheSubstituteTableIsRecordedWhereTheFixturesAre.
//
// The fixtures under internal/netcfg/testdata are REAL captures with
// identifying values substituted. That is a third state, and it is not the one
// the class table at the top of that file describes: a reader who takes
// capture-pi5-* at its word believes every byte is what the kernel printed. The
// substitution has to be recorded there, with the table, or the next capture is
// taken and pasted in with a different set of invented values and the fixtures
// stop agreeing with each other.
//
// This checks the record exists and names the block, rather than checking its
// prose, which would be a test of wording.
func TestTheSubstituteTableIsRecordedWhereTheFixturesAre(t *testing.T) {
	root := moduleRoot(t)
	path := filepath.Join(root, "internal", "netcfg", "testdata", "PROVENANCE.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		substituteMACPrefix, // the block every substitute MAC is drawn from
		"HomeNet",           // the substitute SSID
		"caspian-box",       // the substitute hostname
		"198.51.100.10",     // the substitute for the proxy server address
		"203.0.113.20",      // the substitute for the household's public address
		"PHONE-SERIAL",      // the substitute handset serial
	} {
		if !strings.Contains(text, want) {
			t.Errorf("internal/netcfg/testdata/PROVENANCE.md does not mention %q. The substitute "+
				"table is the only place that says which invented value stands for which real "+
				"one; without it the next person substitutes differently and the fixtures stop "+
				"being consistent with each other, which several tests depend on.", want)
		}
	}
	// And the record that these files are not what their class prefix alone
	// implies.
	for _, want := range []string{"substitut", "PRIVACY"} {
		if !strings.Contains(text, want) {
			t.Errorf("internal/netcfg/testdata/PROVENANCE.md does not contain %q. A reader who is "+
				"not told believes capture-pi5-* is byte-for-byte what the kernel printed, and "+
				"the whole value of a captured fixture is that claim.", want)
		}
	}
}

// TestEveryPublicIPv4EntryNamesItsLiterals is the guard the Literals field's own
// doc comment names.
//
// It exists because of a measured failure and not a preference. The eight
// public-ipv4 entries were first written path-scoped, and running the scan over
// the PRE-SUBSTITUTION content of the same files reported public-ipv4 zero
// times: the entry for a pinned ipinfo.io address was also covering the
// household's public address in the same file. A path plus a class is the wrong
// grain for a class whose findings are values, so every entry has to name them.
func TestEveryPublicIPv4EntryNamesItsLiterals(t *testing.T) {
	found := 0
	for _, a := range Allowlist() {
		if a.Class != ClassPublicIPv4 {
			continue
		}
		found++
		if len(a.Literals) == 0 {
			t.Errorf("allowlist entry %s / %s permits the whole class in that file. It must name "+
				"the exact literals instead, or it also permits the next address somebody pastes "+
				"into the same file, which is what this class exists to catch.", a.PathGlob, a.Class)
		}
		for _, lit := range a.Literals {
			if documentationIPv4(lit) {
				t.Errorf("allowlist entry %s names %s, which documentationIPv4 already treats as "+
					"non-identifying. The literal is dead: delete it", a.PathGlob, lit)
			}
			if _, ok := KnownPublicAddresses()[lit]; ok {
				t.Errorf("allowlist entry %s names %s, which knownPublicAddresses already covers "+
					"everywhere. The literal is dead: delete it", a.PathGlob, lit)
			}
		}
	}
	if found == 0 {
		t.Fatal("no public-ipv4 allowlist entry exists, so this guard checked nothing")
	}
}

// TestLiteralsIsOnlyUsedWhereItIsApplied.
//
// allowedLiteral is called from exactly one place, the public-ipv4 arm of
// privacyFindings. An entry that set Literals for any other class would be read
// by nobody and would permit nothing, while LOOKING in the registry like a
// granted exception. That is worse than a missing entry: the next reader
// believes a hit is already covered.
func TestLiteralsIsOnlyUsedWhereItIsApplied(t *testing.T) {
	for _, a := range Allowlist() {
		if len(a.Literals) > 0 && a.Class != ClassPublicIPv4 {
			t.Errorf("allowlist entry %s / %s sets Literals, but only the public-ipv4 arm of "+
				"privacyFindings consults allowedLiteral. This entry permits nothing and reads "+
				"like it does.", a.PathGlob, a.Class)
		}
	}
}

// TestTheScannersOwnDirectoryIsExemptFromShapesAndNotFromSentinels.
//
// The exemption for test/goldenscan was wholesale for about an hour on
// 2026-08-31, and in that hour a comment in privacy.go illustrating three token
// positions illustrated them with two of the values being removed. A grep found
// it; this scan did not. The exemption is now shape-only, and this pins both
// halves of that: a planted MAC in the scanner's own directory is silent,
// because the scanner has to be able to plant one, and a planted sentinel is
// NOT, because the registry holds digests rather than values and so has no
// reason to hold one.
func TestTheScannersOwnDirectoryIsExemptFromShapesAndNotFromSentinels(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "test", "goldenscan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "// a planted mac b8:27:eb:11:22:33 and a planted address 198.51.101.7\n" +
		"// and a value that must never be here: ssid " + plantedSentinel + "\n"
	if err := os.WriteFile(filepath.Join(dir, "privacy.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := ScanRepo(root, append(PrivacySentinels(), plantedSentinelEntry()))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := 0
	var shapes []string
	for _, f := range findings {
		if f.Class == ClassPrivacySentinel {
			sentinel++
			continue
		}
		shapes = append(shapes, f.Class)
	}
	if sentinel == 0 {
		t.Error("a privacy sentinel in the scanner's own directory was not reported. That is the " +
			"exact miss the wholesale exclusion produced: the registry holds digests, not " +
			"values, so the scanner has no reason to carry one and a hit there is real")
	}
	if len(shapes) > 0 {
		t.Errorf("the MAC and IPv4 classes fired inside the scanner's own directory: %v. That "+
			"directory has to be able to plant one of each class, so this reports the guard as "+
			"the leak", shapes)
	}
}
