// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uncheckedClaimPhrases are the sentence shapes that assert a specific artefact
// has not been through nft.
//
// Deliberately narrow. The digest gate's own failure message says "have never
// been parsed by nft" and its doc comment says "no nft has ever parsed", both
// about the general case with no basename beside them; neither is a claim about
// a named file and neither belongs here.
//
// Package scope so the guard and its positive control read the SAME slice.
var uncheckedClaimPhrases = []string{
	"not nft-checked",
	"has not been checked",
	"have not been checked",
	"had not been checked",
	"not been checked with",
}

// How far from the sentence a basename still counts as what the sentence is
// about. Asymmetric on purpose: in Go source the claim sits in a doc comment
// and the filename appears in the function BELOW it, 21 lines away in the one
// real instance, so a symmetric window of 12 silently missed it. Over-reaching
// costs a loud failure naming a line somebody can read; under-reaching costs
// the whole guard, which is the failure that already happened once.
const (
	claimWindowBack    = 8
	claimWindowForward = 40
)

// verifiedRulesetGoldens returns the basename of every ruleset golden whose
// CURRENT bytes are covered by a record in nftCheckedDigests.
//
// "Covered" means the digest is present and its record is not labelled NOT
// CONFIRMED, which is the same reading TestGolden_CheckedRulesetDigestsAreStillCurrent
// applies. A WITHDRAWN record still counts: it says nft parsed those bytes on a
// date and was then superseded, which is a completed check and not an absent
// one.
func verifiedRulesetGoldens(t *testing.T) map[string]string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join("testdata", "golden-ruleset-*.nft"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no ruleset goldens found, so this test checked nothing")
	}
	out := map[string]string{}
	for _, path := range names {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		record, ok := nftCheckedDigests[fmt.Sprintf("%x", sha256.Sum256(body))]
		if !ok || strings.Contains(record, "NOT CONFIRMED") {
			continue
		}
		out[filepath.Base(path)] = record
	}
	return out
}

// No document may say a ruleset golden is unchecked while the digest record
// says a real nft parsed it.
//
// # Why this test exists rather than a corrected sentence
//
// Two places said golden-ruleset-takeover.nft had never been through
// "nft -c -f": this file's own comment above TestGolden_RulesetTakeover, and a
// section heading in testdata/PROVENANCE.md. Both were true when written and
// both stopped being true on 2026-08-30, when the check was run and the digest
// 1dcbacb0fc7752e68b55bf3da8663fdd2c84b964d09eac949aedee9c46fdde7e was recorded.
// Nothing failed, because prose beside a record is not checked against it.
//
// That is the worse half of the failure. A reader who believes a shipped
// firewall has never been parsed concludes there is work outstanding and either
// does it again or discounts the whole verification record; a reader who
// believes the inverse ships an unparsed ruleset. Correcting the words would
// have fixed today's instance and left the mechanism that produced it.
//
// # Why only this direction is guarded here
//
// The opposite error, prose claiming a check that never happened, is already
// mechanical: TestGolden_CheckedRulesetDigestsAreStillCurrent fails when a
// golden's current digest is absent from the record, whatever any document
// says. The direction nothing covered is the stale "not checked" claim, because
// running a check makes a document wrong without touching it.
func TestProvenance_NoDocumentClaimsAVerifiedRulesetIsUnchecked(t *testing.T) {
	verified := verifiedRulesetGoldens(t)
	if len(verified) == 0 {
		t.Fatal("no golden is currently verified, so this test could not fail")
	}

	claims := uncheckedClaimPhrases

	docs := []string{
		filepath.Join("testdata", "PROVENANCE.md"),
		"golden_test.go",
	}
	checkedAnything := false
	for _, doc := range docs {
		body, err := os.ReadFile(doc)
		if err != nil {
			t.Fatalf("reading %s: %v", doc, err)
		}
		lines := strings.Split(string(body), "\n")
		checkedAnything = true
		for i, line := range lines {
			lower := strings.ToLower(line)
			matched := ""
			for _, c := range claims {
				if strings.Contains(lower, c) {
					matched = c
					break
				}
			}
			if matched == "" {
				continue
			}
			lo, hi := max(0, i-claimWindowBack), min(len(lines), i+claimWindowForward+1)
			near := strings.Join(lines[lo:hi], "\n")
			for name, record := range verified {
				if !strings.Contains(near, name) {
					continue
				}
				t.Errorf("%s:%d says %q about %s, but that file's current bytes ARE covered:\n"+
					"    %s\n"+
					"  line: %s\n"+
					"Correct the sentence, or withdraw the digest if the record is the thing that is wrong.",
					doc, i+1, matched, name, record, strings.TrimSpace(line))
			}
		}
	}
	if !checkedAnything {
		t.Fatal("no document was read, so this test checked nothing")
	}
}

// The guard above can only work if the phrases it looks for are the phrases
// this repository actually uses, so this is its positive control: a sentence in
// the banned shape, beside a verified basename, must be detected.
//
// Without it, a typo in the claims list would make the guard silently pass on
// every document forever, which is the same class of failure it was written to
// catch.
func TestProvenance_TheUncheckedClaimGuardCanActuallyFail(t *testing.T) {
	verified := verifiedRulesetGoldens(t)
	if len(verified) == 0 {
		t.Fatal("no golden is verified, so the control proves nothing")
	}
	var name string
	for n := range verified {
		name = n
		break
	}

	// The SAME slice the guard reads, never a copy. A copy is how a control
	// comes to prove nothing: the list under test drifts, the copy still
	// matches the sample, and the control stays green while the guard has
	// stopped being able to fire. Measured: with a duplicated list here,
	// misspelling every phrase in the real one left this test passing.
	for _, sample := range []string{
		"## " + name + " is NOT nft-checked",
		"// installs THIS. It has not been checked with \"nft -c -f\" on the box.",
	} {
		lower := strings.ToLower(sample)
		hit := ""
		for _, c := range uncheckedClaimPhrases {
			if strings.Contains(lower, c) {
				hit = c
				break
			}
		}
		if hit == "" {
			t.Errorf("no phrase in uncheckedClaimPhrases matches %q, so "+
				"TestProvenance_NoDocumentClaimsAVerifiedRulesetIsUnchecked cannot fire on it", sample)
		}
	}

	// And the window has to reach from a doc comment to the basename in the
	// function it documents. Measured on this file: the comment above
	// TestGolden_RulesetTakeover opens 21 lines above its checkGolden call, and
	// an earlier version of this guard used a window of 12 and missed it.
	if claimWindowForward < 30 {
		t.Errorf("claimWindowForward is %d, which is too short to reach from a doc comment "+
			"to the basename in the function below it", claimWindowForward)
	}
}
