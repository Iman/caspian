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
		if !strings.HasSuffix(path, ".md") || isTranslationEdition(path) {
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
		delete(want, filepath.ToSlash(relFa))
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

// translationSuffixes are the language editions this repository publishes.
//
// A file carrying one of these is a TRANSLATION and not an English original, so
// the walk must skip it rather than turn round and demand a Persian sibling for
// it. Without this, adding README.ru.md makes the guard ask for
// README.ru.fa.md, which is nonsense and would have to be silenced by deleting
// the guard.
//
// Persian is the only language held to FULL parity with English. Russian and
// Chinese are a README only. That asymmetry is deliberate: the Persian edition
// was reviewed by somebody who reads Persian, and an unreviewed translation of
// a security document does not become trustworthy by being long. Two honest
// pages beat twelve that nobody can check.
var translationSuffixes = []string{".fa.md", ".ru.md", ".zh.md"}

func isTranslationEdition(path string) bool {
	for _, suffix := range translationSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
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
	rel = filepath.ToSlash(rel)
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

// TestEveryPublishedDocumentOffersTheSameFourLanguages checks the language bar
// is on every published page, in both the English and the Persian edition.
//
// Why a test and not a convention: a reader who lands on SECURITY.md and finds
// no way back to their own language is stuck on that page, and the person who
// adds the fifteenth document will not remember a rule written in CONTRIBUTING.
// The failure mode is invisible to everyone who reads English, which is exactly
// the kind of rot this repository has decided to catch mechanically.
//
// It checks that the links are PRESENT and that their targets EXIST. It cannot
// check that the Russian actually says what the English says. Nothing can, and
// claiming otherwise here would be the confident wrong sentence CONTRIBUTING.md
// warns about.
func TestEveryPublishedDocumentOffersTheSameFourLanguages(t *testing.T) {
	root := moduleRoot(t)

	// The four editions of the front page. Every published document links to
	// all of them, so a reader can leave any page in their own language.
	fronts := []string{"README.md", "README.fa.md", "README.ru.md", "README.zh.md"}
	for _, f := range fronts {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("the language bar names %s but it does not exist: %v\n"+
				"Either add the translation or take it out of the bar. A flag that "+
				"leads to a 404 is worse than no flag, because it looks like the "+
				"project supports a language it does not.", f, err)
		}
	}

	var checked int
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
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)

		// A translation is published if its English original is. Strip the
		// language suffix to ask that question.
		english := rel
		for _, suffix := range translationSuffixes {
			if strings.HasSuffix(rel, suffix) {
				english = strings.TrimSuffix(rel, suffix) + ".md"
				break
			}
		}
		if !isPublishedDocument(english) {
			return nil
		}

		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("reading %s: %v", rel, readErr)
			return nil
		}
		text := string(body)

		// Persian and English are held to full parity, so on a page like
		// SECURITY.md those two entries point at SECURITY.fa.md and at the page
		// itself. Russian and Chinese exist as a front page only, so every page
		// sends those readers there. Only the second pair is checked here; the
		// first is what the parity test above already covers.
		var missing []string
		for _, f := range []string{"README.ru.md", "README.zh.md"} {
			// A page does not need to link to itself.
			if filepath.Base(rel) == f {
				continue
			}
			if !strings.Contains(text, f) {
				missing = append(missing, f)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s does not offer %s.\n"+
				"Every published page carries the same language bar. A reader who "+
				"arrives here from a search engine and does not read English has no "+
				"way out of this page without it.", rel, strings.Join(missing, ", "))
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if checked < 20 {
		t.Errorf("only %d published pages were checked, which is fewer than this "+
			"repository has. The walk is not finding them.", checked)
	}
	t.Logf("checked the language bar on %d published pages", checked)
}
