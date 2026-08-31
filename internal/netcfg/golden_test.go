// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update rewrites the golden files. Run with:
//
//	go test ./internal/netcfg -run Golden -update
//
// The golden files exist so that a change to the generated ruleset or to the
// generated command sequence shows up as a reviewable diff rather than as a
// green test. A firewall nobody reads is a firewall nobody checks.
var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create it)", err)
	}
	if got != string(want) {
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(string(want), "\n")
		for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
			g, w := "", ""
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Fatalf("%s differs at line %d\n got: %s\nwant: %s\n(run with -update if the change is intended)", name, i+1, g, w)
			}
		}
	}
}

// The ruleset for the machine that was actually measured.
//
// Checked on the target on 2026-08-30: "sudo nft -c -f" against this file
// returned exit 0 with no output, on nftables v1.1.3, kernel 6.18.34, with the
// live ruleset verified empty before and after. That is a measurement, not a
// pending action. See testdata/PROVENANCE.md, and note there that this file
// and the mode A golden are byte-identical, so the check covered two distinct
// rulesets and not three.
func TestGolden_RulesetCaptured(t *testing.T) {
	_, p := mustPlan(t, pi5Captured(), DefaultOptions())
	checkGolden(t, "golden-ruleset-captured.nft", p.Ruleset())
}

func TestGolden_CommandSequenceCaptured(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	var b strings.Builder
	b.WriteString("# Generated command sequence for the CAPTURED target.\n")
	b.WriteString("# Left column is applied in order; right column is the inverse recorded\n")
	b.WriteString("# in the journal before the change is made.\n\n")
	b.WriteString("## before the engine starts\n")
	writeSteps(&b, p.PreEngineSteps(f.Sysctl))
	b.WriteString("\n## after the engine has created the tunnel device\n")
	writeSteps(&b, p.PostEngineSteps(f.Sysctl))
	checkGolden(t, "golden-commands-captured.txt", b.String())
}

// The ruleset the target will actually install.
//
// The captured golden above is the plan the capability table implies; the
// driver refuses it, so on this hardware the box falls back to taking over
// wlan0 and installs THIS. It is a different file from the mode A and mode B
// goldens, because the leak block and every interface match name wlan0 rather
// than ap0, so it needs a check of its own.
//
// WHETHER IT HAS HAD ONE IS DELIBERATELY NOT STATED HERE. A sentence here used
// to say it had not, which was true when written and stopped being true on
// 2026-08-30 when the check was run. Nothing failed, because prose beside a
// record is not checked against it. The status lives in nftCheckedDigests,
// keyed by the sha256 of the bytes, and nowhere else;
// TestProvenance_NoDocumentClaimsAVerifiedRulesetIsUnchecked now fails if this
// comment, or testdata/PROVENANCE.md, goes back to restating it.
func TestGolden_RulesetTakeover(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-ruleset-takeover.nft", q.Ruleset())
}

// The command sequence the target will actually run. This is the fallback
// path, and it is the one that matters on that hardware.
func TestGolden_CommandSequenceTakeover(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("# Generated command sequence for the CAPTURED target, TAKEOVER path.\n")
	b.WriteString("# The radio refuses to create a second interface, so the access point\n")
	b.WriteString("# takes over wlan0. Left column is applied in order; right column is the\n")
	b.WriteString("# inverse recorded in the journal before the change is made.\n\n")
	b.WriteString("## before the engine starts\n")
	writeSteps(&b, q.PreEngineSteps(f.Sysctl))
	b.WriteString("\n## after the engine has created the tunnel device\n")
	writeSteps(&b, q.PostEngineSteps(f.Sysctl))
	checkGolden(t, "golden-commands-takeover.txt", b.String())
}

// The cut ruleset, so the difference between forwarding and not is a
// reviewable diff rather than a claim.
func TestGolden_RulesetTakeoverCut(t *testing.T) {
	f, p := mustPlan(t, pi5Captured(), DefaultOptions())
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-ruleset-takeover-cut.nft", q.RulesetFor(ForwardCut))
}

// And the way back for a user on a network nobody thought about.
func TestGolden_RulesetTakeoverEgressOpen(t *testing.T) {
	o := DefaultOptions()
	o.Egress = EgressOpen
	f, p := mustPlan(t, pi5Captured(), o)
	q, err := p.HotspotTakeover(f)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-ruleset-takeover-egress-open.nft", q.Ruleset())
}

func TestGolden_RulesetModeA(t *testing.T) {
	_, p := mustPlan(t, modeAScenario(), DefaultOptions())
	checkGolden(t, "golden-ruleset-mode-a.nft", p.Ruleset())
}

func TestGolden_RulesetModeB(t *testing.T) {
	_, p := mustPlan(t, modeBScenario(), DefaultOptions())
	checkGolden(t, "golden-ruleset-mode-b.nft", p.Ruleset())
}

func TestGolden_CommandSequenceModeA(t *testing.T) {
	f, p := mustPlan(t, modeAScenario(), DefaultOptions())
	var b strings.Builder
	b.WriteString("# Generated command sequence, mode A (wired uplink).\n")
	b.WriteString("# Left column is applied in order; right column is the inverse recorded\n")
	b.WriteString("# in the journal before the change is made.\n\n")
	b.WriteString("## before the engine starts\n")
	writeSteps(&b, p.PreEngineSteps(f.Sysctl))
	b.WriteString("\n## after the engine has created the tunnel device\n")
	writeSteps(&b, p.PostEngineSteps(f.Sysctl))
	checkGolden(t, "golden-commands-mode-a.txt", b.String())
}

func writeSteps(b *strings.Builder, steps []Step) {
	for _, s := range steps {
		b.WriteString("do   " + render(s.Do) + "\n")
		if s.Undo.IsZero() {
			b.WriteString("undo (none)\n")
		} else {
			b.WriteString("undo " + render(s.Undo) + "\n")
		}
	}
}

// render distinguishes the two nft steps, which are the same argument vector
// carrying opposite documents on standard input.
func render(c Command) string {
	// CommandLine plus a description of what is on stdin, rather than
	// RunnerKey: the identity digest would put a ruleset hash in the golden
	// and churn it on every firewall change, which the ruleset goldens
	// already pin.
	k := CommandLine(c)
	if c.Stdin == "" {
		return k
	}
	first := strings.SplitN(strings.TrimSpace(c.Stdin), "\n", 2)[0]
	verb := "load"
	if strings.Contains(c.Stdin, "firewall teardown") {
		verb = "remove"
	}
	return fmt.Sprintf("%s  <stdin: %d bytes, %s the generated table; first line %q>", k, len(c.Stdin), verb, first)
}

// Every fixture must be named in PROVENANCE.md.
//
// That file is the only thing separating "a green test proves something about
// the target" from "a green test proves something about bytes somebody wrote".
// An undocumented fixture is one whose class nobody can tell, so it silently
// becomes evidence it is not. The check runs one way on purpose: the file also
// names retired fixtures, and that history is worth more than the symmetry.
func TestProvenance_DocumentsEveryFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "PROVENANCE.md"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "PROVENANCE.md" {
			continue
		}
		checked++
		if !strings.Contains(string(body), name) {
			t.Errorf("testdata/%s is not mentioned in PROVENANCE.md, so nothing records whether it "+
				"was captured from the target or written by hand", name)
		}
	}
	if checked == 0 {
		t.Fatal("no fixtures found to check")
	}
}

// How many INDEPENDENT rulesets the on-target nft check actually covered.
//
// "sudo nft -c -f" returned exit 0 on all three ruleset goldens on 2026-08-30,
// which reads as three confirmations and is two: the captured golden and the
// mode A golden are byte-identical. Plan.Ruleset is a function of four plan
// fields and six option fields, the two machines agree on all ten, and the
// things they differ in (gateway, knob values) reach the command sequence and
// never the ruleset.
//
// This test exists so that fact cannot quietly stop being true. If a fixture
// change makes the two diverge, the failure names which input moved, and the
// paragraph in testdata/PROVENANCE.md that claims two samples has to be
// rewritten rather than left as a wrong count.
func TestGolden_CapturedAndModeARulesetsAreOneSample(t *testing.T) {
	_, cap := mustPlan(t, pi5Captured(), DefaultOptions())
	_, a := mustPlan(t, modeAScenario(), DefaultOptions())
	_, b := mustPlan(t, modeBScenario(), DefaultOptions())

	// Every input Plan.Ruleset actually reads.
	inputs := func(p *Plan) []string {
		return []string{
			"uplink=" + p.Uplink,
			"hotspot=" + p.Hotspot,
			"tun=" + p.Tun,
			"subnet=" + p.HotspotSubnet.String(),
			fmt.Sprintf("dnsPort=%d", p.Opts.DNSPort),
			fmt.Sprintf("panelPort=%d", p.Opts.PanelPort),
			fmt.Sprintf("ipv6=%v", p.Opts.IPv6),
			fmt.Sprintf("isolation=%v", p.Opts.ClientIsolation),
			fmt.Sprintf("masq=%v", p.Opts.MasqueradeToTunnel),
		}
	}
	ci, ai := inputs(cap), inputs(a)
	for i := range ci {
		if ci[i] != ai[i] {
			t.Errorf("captured and mode A now differ in a ruleset input: %q vs %q.\n"+
				"They are no longer one sample; update the sample-count note in testdata/PROVENANCE.md.",
				ci[i], ai[i])
		}
	}

	if cap.Ruleset() != a.Ruleset() {
		t.Error("captured and mode A rulesets are no longer byte-identical, so the nft check " +
			"on the target covered more samples than PROVENANCE.md claims")
	}
	// Mode B must remain a real second sample, or the check covered one.
	if b.Ruleset() == cap.Ruleset() {
		t.Error("mode B now generates the same ruleset as the captured machine, so the nft " +
			"check covered ONE ruleset, not two")
	}
}

// nftCheckedDigests records, per sha256, the moment a real nft parsed exactly
// those bytes. It is package scope rather than a local so that a second test
// can read it: TestProvenance_NoDocumentClaimsAVerifiedRulesetIsUnchecked
// checks the prose in this file and in testdata/PROVENANCE.md against it,
// which is the difference between a status sentence and a status.
//
// Digest -> the date "sudo nft -c -f" returned exit 0 against exactly these
// bytes, on nftables v1.1.3, kernel 6.18.34, live ruleset verified empty
// before and after.
var nftCheckedDigests = map[string]string{
	// Withdrawn. Each was verified on the target on the date given and
	// then superseded, so no current golden is covered by any of them.
	// They stay, labelled, so that a golden reverting to an older shape
	// reports as verified-but-withdrawn rather than as never checked.
	"b1fc7570efe591169ee5025498bb009bb53f3392cddd6af9e9fb05306c8e89a2": "2026-08-30, WITHDRAWN (captured and mode A, input policy drop)",
	"106cf8e5e9e171a200adf34c6014e736fa18ed6ff3068278c3694f1ac5216763": "2026-08-30, WITHDRAWN (mode B, input policy drop)",
	"61a6306c570b0c537eedbaaf5c7ba24835ce4cecc03fa72f0a7781141f2a9937": "2026-08-30, WITHDRAWN (captured and mode A, output policy accept)",
	"558933430ad45b92f143ca50b1be53b6b355aa6eecf9ea39aa38ccf25e719d59": "2026-08-30, WITHDRAWN (mode B, output policy accept)",
	"1168bb33a0801367516edc7ae706acdf45c8496d1aea136c499f40e7528a8c27": "2026-08-30, WITHDRAWN (takeover, output policy accept)",

	// Current, and verified. Checked on the target on 2026-08-30: all six
	// files copied to the Pi, each parsed with "sudo nft -c -f", every one exit
	// 0; the sha256 of each read back ON THE PI rather than taken from the
	// developer machine, so the bytes nft parsed are provably the bytes in
	// this tree; and "nft list ruleset | wc -l" returned 0 immediately before
	// and immediately after, so nothing in the result came from a rule
	// already loaded and the check left the box as it found it. Captured and
	// mode A are byte-identical, so five rulesets were parsed, not six.
	//
	// How these entries came to be trusted, kept because the gate is better
	// for it. They were first added asserting that check, while a comment
	// elsewhere in this file said no such check had happened. Both could not
	// be true and nobody reading the repository could tell which was, so they
	// were relabelled NOT CONFIRMED and the gate was taught to refuse an
	// entry whose record does not assert a completed check. Knowing a digest
	// is not knowing that nft read it. The check was then run for real and
	// the vantage lines below are its result.
	//
	// The NOT CONFIRMED handling stays in the gate below. Nothing carries
	// that label today; it is there for the next time an entry is written
	// ahead of the check it describes.
	"5af03565cc1ae0db79f04ef7ec1bd31fa40bda5948741d7b45aa076e8605176a": "2026-08-30, nft -c -f on the target, nftables 1.1.3, kernel 6.18.34, sha256 read back ON the Pi, live ruleset empty before and after (captured and mode A, output policy drop)",
	"be040408c4f0b43c29c5d75f90b1f452455c88be5f5f273fb07e1f4c88b435e7": "2026-08-30, nft -c -f on the target, nftables 1.1.3, kernel 6.18.34, sha256 read back ON the Pi, live ruleset empty before and after (mode B, output policy drop)",
	"1dcbacb0fc7752e68b55bf3da8663fdd2c84b964d09eac949aedee9c46fdde7e": "2026-08-30, nft -c -f on the target, nftables 1.1.3, kernel 6.18.34, sha256 read back ON the Pi, live ruleset empty before and after (takeover, output policy drop)",
	"a096849105f4b07716ebb8d3e3e15daf14ff6c6baa8b27565c1ef52c74c48bd0": "2026-08-30, nft -c -f on the target, nftables 1.1.3, kernel 6.18.34, sha256 read back ON the Pi, live ruleset empty before and after (takeover, client traffic cut)",
	"b99aae8bb6f1299758fcd9557a760ed6da1ef646143ae16b98b74da325905cd9": "2026-08-30, nft -c -f on the target, nftables 1.1.3, kernel 6.18.34, sha256 read back ON the Pi, live ruleset empty before and after (takeover, EgressOpen)",
}

// The bytes that passed "nft -c -f" on the target must still be the bytes in
// the repository.
//
// Without this, the record in testdata/PROVENANCE.md decays into a claim about
// a file that no longer exists: the generator changes, the golden is rewritten
// with -update, and the "exit 0 on 2026-08-30" line silently starts describing
// different content.
//
// A failure here is NOT a defect in the ruleset. It means the ruleset changed
// and no longer has a verification behind it, which is a release gate rather
// than a bug: shipping a firewall no nft has ever parsed risks the whole
// ruleset failing to load, which leaves the box with no firewall at all.
func TestGolden_CheckedRulesetDigestsAreStillCurrent(t *testing.T) {
	// Digest -> the date "sudo nft -c -f" returned exit 0 against exactly
	// these bytes, on nftables v1.1.3, kernel 6.18.34, live ruleset verified
	// empty before and after.
	//
	// The three entries below are the input-policy-drop rulesets checked on
	// 2026-08-30. That policy was removed the same day, after it was measured
	// closing every new inbound connection to the box, so no CURRENT golden
	// is covered by them. They stay so the failure names what changed rather
	// than reporting an empty map.
	checked := nftCheckedDigests

	names, err := filepath.Glob(filepath.Join("testdata", "golden-ruleset-*.nft"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no ruleset goldens found")
	}
	var unverified []string
	for _, path := range names {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(body))
		if when, ok := checked[digest]; ok {
			// A digest whose record does not assert a completed check is not
			// a verified digest. Knowing the number is not knowing that nft
			// read it. Nothing carries this label today; the handling is kept
			// for the next entry written ahead of its check.
			if strings.Contains(when, "NOT CONFIRMED") {
				unverified = append(unverified, fmt.Sprintf("  %s  %s  [%s]", digest, filepath.Base(path), when))
				continue
			}
			t.Logf("%s: checked %s", filepath.Base(path), when)
			continue
		}
		unverified = append(unverified, fmt.Sprintf("  %s  %s", digest, filepath.Base(path)))
	}
	if len(unverified) > 0 {
		t.Errorf("%d ruleset golden(s) have never been parsed by nft:\n%s\n\n"+
			"These are shipped firewalls with no verification behind them. On the target:\n"+
			"    for f in testdata/golden-ruleset-*.nft; do sudo nft -c -f \"$f\" && echo \"OK $f\"; done\n"+
			"then add each digest to the map in this test and record the result in testdata/PROVENANCE.md.",
			len(unverified), strings.Join(unverified, "\n"))
	}
}
