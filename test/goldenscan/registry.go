// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package goldenscan

import "strings"

// ---------------------------------------------------------------------------
// The sentinels
// ---------------------------------------------------------------------------

// Sentinel is one value a golden layer feeds to the product as a credential.
//
// Every one of them occurs nowhere else in this repository, which is the whole
// design: a hit is unambiguous, needs no allowlist, and cannot be a false
// positive. Nothing here is, or has ever been, a working credential.
type Sentinel struct {
	// Name is what a failure prints. The VALUE is never printed.
	Name string

	// Value is the literal.
	Value string

	// DeclaredIn is the file that declares this constant. Checked by
	// TestEverySentinelIsStillDeclaredWhereItSaysItIs, so that a rename in the
	// owning package makes this registry fail rather than silently stop
	// covering anything.
	DeclaredIn string
}

// Sentinels is the registry.
//
// THE FAILURE THIS SHAPE PREVENTS. The obvious design is to let each package
// keep its own sentinel and let this scan know nothing about them. Then a
// package renames its constant, this scan keeps looking for a string that no
// longer exists, and it reports CLEAN forever while covering nothing. That is
// the exact shape of a test suite reporting success while executing zero tests.
// DeclaredIn plus the guard that checks it is what closes it.
func Sentinels() []Sentinel {
	return []Sentinel{
		{
			Name:       "panel golden hotspot passphrase",
			Value:      "sentinelwpa-kzmqvrxt7",
			DeclaredIn: "internal/panel/golden_test.go",
		},
		{
			Name:       "panel golden panel password",
			Value:      "sentinelpanelpw-4bq8",
			DeclaredIn: "internal/panel/golden_test.go",
		},
		{
			Name:       "hotspot variant WPA passphrase",
			Value:      "sentinelwpa-hotspot-9d4",
			DeclaredIn: "internal/hotspot/golden_variants_test.go",
		},
	}
}

// ---------------------------------------------------------------------------
// The allowlist
// ---------------------------------------------------------------------------

// Allow is one shape hit that is known not to be a secret.
//
// A sentinel hit can NEVER be allowlisted. Apply is written so that the class
// is checked before the path, and "sentinel" matches no entry.
type Allow struct {
	// PathGlob matches the file, relative to the module root, slash separated.
	PathGlob string

	// Class is the shape class this entry permits in that file.
	Class string

	// Reason must say WHY the hit is not a secret, and must point at where the
	// value is defined so a reader can check the claim rather than take it.
	// TestAllowlistEntriesExplainThemselves enforces that it is not empty and
	// is not a shrug.
	Reason string

	// Literals narrows the entry from "this class is permitted in this file" to
	// "these exact values are permitted in this file". Empty means the whole
	// class, which is the older behaviour and is still right for a class whose
	// matched text is a shape rather than a value.
	//
	// WHY THIS FIELD EXISTS, because it was added to close a measured hole and
	// not for tidiness. On 2026-08-31 the eight public-ipv4 entries below were
	// first written path-scoped. Running the scan over the PRE-SUBSTITUTION
	// content of the same files, taken from git, reported the two MAC classes
	// and all seven privacy sentinels and reported public-ipv4 ZERO TIMES: the
	// entry that exists for a pinned ipinfo.io address was also covering the
	// household's public address in the same file, and the entry for two
	// measured DNS answers was also covering a proxy server address. Both
	// values are exactly what the class is for.
	//
	// A path plus a class is the wrong grain for a class whose findings are
	// VALUES. TestEveryPublicIPv4EntryNamesItsLiterals requires this field on
	// every public-ipv4 entry so the hole cannot be reopened by writing the
	// next entry the way the first eight were written.
	Literals []string
}

// allowlist is every known, reviewed, non-secret shape hit.
//
// Read the reasons. Each one is a claim that a value which LOOKS like a
// credential is an invented fixture, and each names the file that defines it so
// the claim is checkable. An entry whose reason cannot be checked should not be
// here.
func allowlist() []Allow {
	return []Allow{
		// --- internal/xcfg -------------------------------------------------
		//
		// The engine configuration document IS the credential-carrying
		// document: that is what it is for. Its inputs are the invented share
		// links in internal/xcfg/fixtures_test.go, whose own header says every
		// value there is built from its parts so a reader can see at a glance
		// that they are fake.
		{
			PathGlob: "internal/xcfg/testdata/*.json",
			Class:    "uuid",
			Reason: "the vless and vmess user id from fakeUUID in internal/xcfg/fixtures_test.go, " +
				"which is 11111111-2222-4333-8444-555555555555 and is invented",
		},
		{
			PathGlob: "internal/xcfg/testdata/*.json",
			Class:    "base64-32byte-key",
			Reason: "the REALITY public key from fakePublicKey in internal/xcfg/fixtures_test.go, " +
				"which is base64 of the ASCII text CASPIAN-FAKE-REALITY-PUBKEY-3232",
		},
		// --- internal/hotspot ----------------------------------------------
		//
		// A hostapd configuration cannot exist without a wpa_passphrase line;
		// that line is the file's purpose. What matters is that the value in it
		// is a fixture and that it never appears in the dnsmasq file, which
		// TestGoldenVariants_NoGoldenCarriesTheSentinelPassphrase asserts
		// separately.
		{
			// Named exactly, NOT hostapd*.golden. A glob would let every future
			// hostapd golden inherit the exception, and the variants added on
			// 2026-08-30 redact their passphrase to a digest precisely so that
			// they do not need it. This one file predates that and is the file
			// the external-evidence record in external_test.go is written
			// against, so rewriting it would invalidate that record.
			PathGlob: "internal/hotspot/testdata/hostapd.golden",
			Class:    "wpa-passphrase",
			Reason: "testAP()'s passphrase, correct-horse-battery, declared in " +
				"internal/hotspot/golden_test.go and also used as panel_test.go's testPassword. " +
				"Invented, never a working credential. The newer hostapd variant goldens redact " +
				"theirs to a sha256 digest; this file is exempt only because the on-target " +
				"evidence recorded in external_test.go names these exact bytes",
		},

		{
			// The provenance file names the fixture values so a reader can
			// check the claim that they are invented rather than take it. That
			// is the point of the file, and it means the shapes appear in
			// prose. The share-link shape is NOT allowlisted here: those two
			// lines were rewritten in words instead, because a provenance file
			// that reproduces the shape it is explaining trains people to
			// allowlist the class.
			PathGlob: "internal/xcfg/testdata/PROVENANCE.md",
			Class:    "uuid",
			Reason: "the file quotes fakeUUID from internal/xcfg/fixtures_test.go, " +
				"11111111-2222-4333-8444-555555555555, in the sentence explaining that it " +
				"is syntactically valid and nobody's account",
		},
		{
			PathGlob: "internal/xcfg/testdata/PROVENANCE.md",
			Class:    "routable-ipv4",
			Reason: "the file names the public resolvers from internal/xcfg/resolvers.go so a " +
				"reader can check that the addresses in the goldens are those and not a " +
				"user's server",
		},

		// --- public resolver addresses -------------------------------------
		//
		// Both the generated firewall and the generated engine configuration
		// name public DNS resolvers by address. They are the same on every box,
		// they are chosen by this repository rather than by a user, and they
		// identify nobody: the whole point of a public resolver is that
		// everybody uses it.
		{
			PathGlob: "internal/xcfg/testdata/*.json",
			Class:    "routable-ipv4",
			Reason: "the public resolvers internal/xcfg writes into the dns section, from " +
				"resolvers.go: Quad9 9.9.9.9 and 9.9.9.11, Cloudflare 1.1.1.3 and " +
				"CleanBrowsing 185.228.168.9. Identical on every box and chosen by this " +
				"repository, not by a user",
		},

		// --- internal/netcfg -----------------------------------------------
		//
		// No entry. The netcfg fixtures are captures from the target and
		// hand-written scenarios, and every address in them is a hotspot
		// subnet, a DHCP range or a gateway, which nonRoutable already
		// excludes. The one routable address in that directory is an open
		// finding, not an allowlisted one: see openFindings below. Entries
		// were written here for the ruleset goldens and for the provenance
		// file's long tokens, and TestAllowlistEntriesAreAllUsed reported both
		// as matching nothing, so they were deleted rather than kept as
		// permissions nobody would notice becoming wrong.

		{
			// Converted from an open finding on 2026-08-31. The finding said a
			// routable address labelled "the proxy server" was in this table.
			// It is not: the same 2026-08-30 change that recorded the finding
			// had already replaced it with the words <the proxy server>, and
			// the record kept describing a state the file was no longer in.
			// What is left is four addresses that are not secrets, so this is
			// an allowlist entry, which says "these are not secrets", and no
			// longer an open finding, which says "somebody has to decide".
			PathGlob: "internal/netcfg/testdata/PROVENANCE.md",
			Class:    "routable-ipv4",
			Reason: "the measured permit-and-refuse table in that file: 1.1.1.1 as a public resolver " +
				"and 93.184.216.34 as IANA's example host, both used there as addresses the policy " +
				"must REFUSE, plus the 0.debian.pool.ntp.org member and the example.org address the " +
				"NTP and DNS provocations resolved to. All four are records of what a command " +
				"printed, and rewriting a measured reading to a prettier number would falsify the " +
				"evidence the section exists to carry",
		},

		// --- the scanner's own source --------------------------------------
		//
		// scan.go and registry.go contain every sentinel and every pattern, so
		// they match themselves. They are excluded by not being under a
		// testdata directory, and this entry is here only to record that the
		// exclusion is deliberate.
		{
			PathGlob: "test/goldenscan/*.go",
			Class:    "sentinel-not-applicable",
			Reason: "the scanner's own source declares every sentinel and every pattern. It is " +
				"outside every testdata directory and so is never scanned; this entry records " +
				"that as a decision rather than leaving it as an accident of the walk",
		},

		// -------------------------------------------------------------------
		// PRIVACY CLASSES. See privacy.go for what each one is.
		//
		// There is deliberately NO entry for non-local-mac, unregistered-mac or
		// privacy-sentinel, in any file. Every MAC address in this repository
		// is drawn from the 02:00:5e substitute block documented in
		// internal/netcfg/testdata/PROVENANCE.md, so no exception is needed,
		// and an exception naming a lease fixture or a capture would also
		// permit the day somebody pasted a real reading into that same file.
		// The two in-code exception maps in privacy.go, notAnAddress and
		// registeredMACs, hold the only MAC values that are not in the block,
		// and both are claims about a VALUE rather than about a file.
		//
		// Every entry below is therefore public-ipv4, and every one of them
		// says which literal in that file is not an address, or is an address
		// this repository chose rather than a user.
		// -------------------------------------------------------------------
		{
			PathGlob: "internal/xcfg/private.go",
			Class:    ClassPublicIPv4,
			Literals: []string{"3.2.1.3"},
			Reason: "3.2.1.3 in that file is a SECTION NUMBER: the comment on the 0.0.0.0/8 entry " +
				"cites RFC 1122 section 3.2.1.3 for the meaning of \"this network\". It is not an " +
				"address, and rewording a citation to please a regular expression would make the " +
				"claim uncheckable",
		},
		{
			PathGlob: "internal/netcfg/route.go",
			Class:    ClassPublicIPv4,
			Literals: []string{"128.0.0.0"},
			Reason: "128.0.0.0 there is the base of 128.0.0.0/1, the upper half of the address " +
				"space. Together with 0.0.0.0/1 it is how the tunnel default route is written so " +
				"that a more specific route to the server still wins; see the comment above " +
				"defaultRouteHalves in the same file",
		},
		{
			PathGlob: "internal/netcfg/route_test.go",
			Class:    ClassPublicIPv4,
			Literals: []string{"128.0.0.0"},
			Reason: "the same 128.0.0.0/1 half, asserted as a command argument. The test names the " +
				"prefix route.go emits, so it holds the same literal for the same reason",
		},
		{
			PathGlob: "test/hardware/lib/exitip.sh",
			Class:    ClassPublicIPv4,
			Literals: []string{"34.117.59.81"},
			Reason: "34.117.59.81 is EI_ECHO_B_ADDR, the ipinfo.io address the exit-IP harness is " +
				"PINNED to. The long comment above it records why the pin exists: the resolver on " +
				"the test LAN sinkholes every other echo service, so a name-based endpoint would " +
				"make a box that changed nothing but its DNS look like a box that tunnelled. It " +
				"is an anycast address of a public service, identical for every operator",
		},
		{
			PathGlob: "test/hardware/lib/phone.sh",
			Class:    ClassPublicIPv4,
			Literals: []string{"34.117.59.81"},
			Reason: "the same pinned ipinfo.io address, quoted in the comment above ph_http_get to " +
				"record that the request must still carry Host: ipinfo.io because the endpoint is " +
				"a name-based virtual host. Defined as EI_ECHO_B_ADDR in test/hardware/lib/exitip.sh",
		},
		{
			PathGlob: "docs/HARDWARE-TEST.md",
			Class:    ClassPublicIPv4,
			Literals: []string{"34.117.59.81"},
			Reason: "the same pinned ipinfo.io address, in the table of the two independent exit-IP " +
				"sources and in the pre-flight checklist. Defined as EI_ECHO_B_ADDR in " +
				"test/hardware/lib/exitip.sh",
		},
		{
			PathGlob: "docs/HARDWARE-TEST.fa.md",
			Class:    ClassPublicIPv4,
			Literals: []string{"34.117.59.81"},
			Reason: "the Persian edition of docs/HARDWARE-TEST.md, which carries the same pinned " +
				"ipinfo.io address in the same two places for the same reason. A translation that " +
				"dropped or altered the address would stop describing the harness, so it is " +
				"reproduced verbatim. Defined as EI_ECHO_B_ADDR in test/hardware/lib/exitip.sh",
		},
		{
			PathGlob: "test/hardware/selftest/run.sh",
			Class:    ClassPublicIPv4,
			Literals: []string{"148.0.0.0"},
			Reason: "148.0.0.0 there is not an address: it is the version in the Chrome " +
				"User-Agent string the selftest feeds its parsers, Chrome/148.0.0.0 Mobile Safari. " +
				"The handset's real Chrome build is named in the header of test/hardware/lib/phone.sh",
		},
		{
			PathGlob: "internal/netcfg/testdata/PROVENANCE.md",
			Class:    ClassPublicIPv4,
			Literals: []string{"143.20.69.40", "172.66.157.237"},
			Reason: "the two addresses the 2026-08-30 provocations resolved to and recorded: an " +
				"0.debian.pool.ntp.org member reached by systemd-timesyncd, and the address " +
				"example.org returned. Both are measured readings in an evidence table. The same " +
				"file's routable-ipv4 entry above says why they are not rewritten",
		},
	}
}

// ---------------------------------------------------------------------------
// Open findings
// ---------------------------------------------------------------------------

// OpenFinding is a hit that is, or may be, a real credential in a file this
// package does not own.
//
// It is NOT an allowlist entry. An allowlist entry says "this is not a secret".
// This says "this looks like one, nobody here can fix it, and here is the
// record". Open findings do not fail the scan. They are printed on every run
// with their owner and their date, so they cannot go quiet, and
// TestOpenFindingsAreDatedAndOwned refuses one that does not say both.
type OpenFinding struct {
	PathGlob string
	Class    string
	// Recorded is the date the finding was made, as YYYY-MM-DD.
	Recorded string
	// Owner is who has to decide what happens, in words, not a name a reader
	// has to look up.
	Owner string
	// What it is, and what acting on it would mean.
	Detail string
}

// openFindings is the register.
//
// EMPTY IS A STATE, NOT A GAP. The one entry this register held was closed on
// 2026-08-31: it recorded a proxy server address in
// internal/netcfg/testdata/PROVENANCE.md that had already been replaced by the
// words <the proxy server> before the finding was written, so the record
// described a state the file was not in. It is now an allowlist entry naming
// the four public addresses that remain, which is a claim a reader can check.
// The privacy classes added on the same date have no open findings: every value
// they cover was substituted rather than recorded, because this repository is
// being published and a recorded finding leaves the value in the tree.
//
// THE LIMIT OF ALL OF THIS, stated here because it is the thing a reader is
// most likely to assume away. Every scan in this package reads the WORKING
// TREE. A value that reached a commit is permanent, and none of these classes
// can reach it. Substituting removes a value from the working tree and from
// every future clone's checkout, and it removes nothing from history. Deciding
// whether to rewrite history is a disclosure question and not a test question.
func openFindings() []OpenFinding {
	return []OpenFinding{}
}

// ---------------------------------------------------------------------------
// Applying the lists
// ---------------------------------------------------------------------------

// filterAllowed removes allowlisted findings. A sentinel is never removed.
func filterAllowed(in []Finding) []Finding {
	var out []Finding
	for _, f := range in {
		if f.Class == "sentinel" {
			out = append(out, f)
			continue
		}
		if allowed(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func allowed(f Finding) bool {
	for _, a := range allowlist() {
		if a.Class != f.Class {
			continue
		}
		if len(a.Literals) > 0 {
			// A value-scoped entry is applied where the value is still in hand,
			// in allowedLiteral below, and never here: a Finding does not carry
			// the matched text, so applying it here could only mean permitting
			// the whole file, which is the grain this field exists to leave.
			continue
		}
		if match(a.PathGlob, f.Path) {
			return true
		}
	}
	return false
}

// allowedLiteral reports whether one exact value is permitted in one file.
//
// Called at DETECTION time, from privacy.go, because that is the only moment
// the value exists. It never reaches a Finding: printing it is what this whole
// package refuses to do, and for the public-ipv4 class the matched text is
// precisely the thing that might identify somebody.
func allowedLiteral(path, class, value string) bool {
	for _, a := range allowlist() {
		if a.Class != class || len(a.Literals) == 0 {
			continue
		}
		if !match(a.PathGlob, path) {
			continue
		}
		for _, lit := range a.Literals {
			if lit == value {
				return true
			}
		}
	}
	return false
}

// IsOpenFinding reports whether a finding is in the open register, and returns
// the entry so a caller can print it.
func IsOpenFinding(f Finding) (OpenFinding, bool) {
	if f.Class == "sentinel" {
		return OpenFinding{}, false
	}
	for _, o := range openFindings() {
		if o.Class == f.Class && match(o.PathGlob, f.Path) {
			return o, true
		}
	}
	return OpenFinding{}, false
}

// OpenFindings exposes the register for the guards and for printing.
func OpenFindings() []OpenFinding { return openFindings() }

// Allowlist exposes the list for the guards.
func Allowlist() []Allow { return allowlist() }

// match is a slash-separated glob over a whole path. It supports a single "*"
// per segment, which is all the entries above need, and it is written out
// rather than using path.Match so that a pattern with no wildcard is an exact
// comparison and cannot match a longer path by accident.
func match(pattern, path string) bool {
	pp := strings.Split(pattern, "/")
	sp := strings.Split(path, "/")
	if len(pp) != len(sp) {
		return false
	}
	for i := range pp {
		if !segmentMatch(pp[i], sp[i]) {
			return false
		}
	}
	return true
}

func segmentMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	star := strings.IndexByte(pattern, '*')
	if star < 0 {
		return pattern == s
	}
	prefix, suffix := pattern[:star], pattern[star+1:]
	if strings.IndexByte(suffix, '*') >= 0 {
		// More than one wildcard in a segment is not supported, and silently
		// matching nothing would be worse than saying so.
		panic("goldenscan: allowlist pattern has more than one wildcard in a segment: " + pattern)
	}
	return len(s) >= len(prefix)+len(suffix) &&
		strings.HasPrefix(s, prefix) && strings.HasSuffix(s, suffix)
}
