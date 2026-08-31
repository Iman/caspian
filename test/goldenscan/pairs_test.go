// SPDX-License-Identifier: AGPL-3.0-or-later

package goldenscan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every English document has a Persian edition, and the Persian edition names
// every identifier the English one does.
//
// This project ships two editions of every document because some of the people
// it exists for read Persian and not English. That is only worth doing while
// the two agree. A Persian file that has drifted is worse than no Persian file:
// a reader trusting it is misled, and misled with confidence, which is the
// failure this repository has a rule about.
//
// What this CAN check: that the pair exists, and that no identifier, test name,
// fault code, error name, command or URL appears in the English and not in the
// Persian. Those are the parts that rot silently, because they change when
// somebody edits code and never thinks about a translation.
//
// What this CANNOT check, and no test can: whether the prose still means the
// same thing. A test claiming to would be exactly the confident wrong sentence
// CONTRIBUTING.md warns about. So the Persian editions each say in their own
// first lines that the English is authoritative and is what the tests check,
// and this guard covers the mechanical half only.
func TestEveryEnglishDocumentHasAPersianEditionThatKeptUp(t *testing.T) {
	root := moduleRoot(t)

	// Identifier-shaped things: backticked tokens, and the Go names that appear
	// in prose. A URL counts too, because a link that exists in one edition and
	// not the other sends a reader somewhere the other reader cannot follow.
	backticked := regexp.MustCompile("`([^`\n]{2,80})`")
	goName := regexp.MustCompile(`\b(?:Test|Fault|Err|Assert|Msg)[A-Z][A-Za-z0-9_]{2,}`)
	url := regexp.MustCompile(`https?://[^\s)\]"'>]+`)

	var pairs int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "local" || base == "bdd" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".fa.md") {
			return nil
		}
		rel0, _ := filepath.Rel(root, path)
		if !isPublishedDocument(rel0) {
			return nil
		}
		fa := strings.TrimSuffix(path, ".md") + ".fa.md"
		rel, _ := filepath.Rel(root, path)
		relFa, _ := filepath.Rel(root, fa)

		if _, statErr := os.Stat(fa); statErr != nil {
			t.Errorf("%s has no Persian edition at %s. Every document in this project "+
				"exists in both languages, or it is deleted in both. A document that exists "+
				"only in English tells a Persian-reading contributor the project was not "+
				"written with them in mind.", rel, relFa)
			return nil
		}
		pairs++

		en, e1 := os.ReadFile(path)
		pe, e2 := os.ReadFile(fa)
		if e1 != nil || e2 != nil {
			t.Errorf("reading the pair %s: %v %v", rel, e1, e2)
			return nil
		}
		enText, faText := string(en), string(pe)

		want := map[string]bool{}
		for _, m := range backticked.FindAllStringSubmatch(enText, -1) {
			tok := strings.TrimSpace(m[1])
			// Prose in backticks is not an identifier. Require something that
			// looks like code: a path, a dot, an underscore, or no spaces.
			if strings.ContainsAny(tok, " ") && !strings.ContainsAny(tok, "/=") {
				continue
			}
			want[tok] = true
		}
		for _, m := range goName.FindAllString(enText, -1) {
			want[m] = true
		}
		for _, m := range url.FindAllString(enText, -1) {
			want[strings.TrimRight(m, ".,")] = true
		}

		// An English file links to its own Persian sibling and the Persian one
		// links back. Neither contains the other's name, and that is correct
		// rather than drift.
		delete(want, filepath.Base(fa))
		delete(want, relFa)
		delete(want, filepath.Base(path))

		var missing []string
		for tok := range want {
			if !strings.Contains(faText, tok) {
				missing = append(missing, tok)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			show := missing
			if len(show) > 8 {
				show = show[:8]
			}
			t.Errorf("%s names %d thing(s) that %s does not: %s\n"+
				"An identifier, a command or a link changed in the English edition and the "+
				"Persian one was not told. This does not mean the translation is bad; it means "+
				"it is behind. Update it, or delete both.",
				rel, len(missing), relFa, strings.Join(show, ", "))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if pairs < 8 {
		t.Errorf("only %d document pairs were checked, which is fewer than this repository "+
			"has. The walk is not finding them.", pairs)
	}
	t.Logf("checked %d English and Persian document pairs", pairs)
}

// isPublishedDocument says whether a markdown file is one a reader is meant to
// read, as opposed to a record kept beside the code for whoever maintains it.
//
// The rule that every document exists in both languages is about the published
// set: the README, the policy files, and docs/. It is not about testdata
// provenance files, which exist so that a maintainer can tell where a captured
// fixture came from, or about tooling configuration. Demanding Persian for
// those would produce a guard that fails constantly for no reader's benefit,
// and a guard that cries wolf is one somebody deletes.
func isPublishedDocument(rel string) bool {
	if strings.HasPrefix(rel, "docs/") {
		return true
	}
	if strings.Contains(rel, "/") {
		return false
	}
	switch rel {
	case "README.md", "FAQ.md", "SECURITY.md", "CONTRIBUTING.md",
		"CODE_OF_CONDUCT.md", "TRADEMARK.md", "FUNDING.md", "CLA.md":
		return true
	}
	return false
}
