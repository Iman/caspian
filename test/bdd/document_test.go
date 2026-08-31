// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestBehaviourDocumentDoesNotClaimShippedCodeIsAbsent.
//
// docs/BEHAVIOUR.md carries a list of what this suite does NOT prove, and that
// list is the most dangerous prose in the repository: every entry is a reason
// not to go and check something. An entry that has quietly become false is a
// piece of work suppressed indefinitely, and nothing about reading the document
// reveals it.
//
// One had. Until 2026-08-30 the document said "`cmd/caspian` is an empty
// directory, and `internal/privsvc` ... does not compile yet and has no `Start`
// method". `019fba6` added both, and the sentence survived a commit five hours
// later whose subject line was correcting stale documents. A reader had every
// reason to believe there was no orchestrator to test.
//
// So this is the guard rather than a better sentence. For every Go package path
// the document mentions in backticks, if that package exists on disk with Go
// files in it, the document may not describe it as absent, empty, or not
// building. It is deliberately narrow: it checks the one class of claim that
// can be settled by looking at the filesystem, and makes no attempt to judge
// prose it cannot check.
func TestBehaviourDocumentDoesNotClaimShippedCodeIsAbsent(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "docs", "BEHAVIOUR.md"))
	if err != nil {
		t.Fatalf("read docs/BEHAVIOUR.md: %v", err)
	}
	doc := string(raw)

	// Claims that can be checked against the filesystem, and the phrasings
	// actually used in this repository's documents rather than an invented
	// vocabulary.
	absence := []string{
		"is an empty directory",
		"is empty",
		"does not exist",
		"does not compile",
		"has no Start method",
		"does not compile yet",
	}

	// A backticked token that looks like a package path in this module.
	pathRe := regexp.MustCompile("`((?:cmd|internal|test|third_party)/[A-Za-z0-9_/.-]+)`")

	// Sentences, so a claim is only attributed to a path it actually sits with.
	for _, sentence := range splitSentences(doc) {
		paths := pathRe.FindAllStringSubmatch(sentence, -1)
		if len(paths) == 0 {
			continue
		}
		var claimed string
		for _, phrase := range absence {
			if strings.Contains(sentence, phrase) {
				claimed = phrase
				break
			}
		}
		if claimed == "" {
			continue
		}
		for _, m := range paths {
			rel := m[1]
			if !hasGoFiles(filepath.Join(root, rel)) {
				continue
			}
			t.Errorf(
				"docs/BEHAVIOUR.md says %q of %s, and %s exists on disk with Go files in it.\n"+
					"  sentence: %s\n"+
					"This document's \"does not prove\" list is a list of reasons not to check things. An "+
					"entry that has become false suppresses the work that would have caught whatever it is "+
					"now hiding. Correct the entry in the same change that made it false.",
				claimed, rel, rel, strings.TrimSpace(sentence))
		}
	}
}

// hasGoFiles reports whether dir exists and contains at least one .go file.
func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// splitSentences is deliberately crude: it splits on a full stop followed by a
// space or a newline, which is enough to keep a claim with the path it is made
// about and no more than that. A sentence splitter that tried to be clever
// about "e.g." would be a second thing to maintain for no gain here.
func splitSentences(doc string) []string {
	var out []string
	var b strings.Builder
	runes := []rune(doc)
	for i, r := range runes {
		b.WriteRune(r)
		if r != '.' {
			continue
		}
		if i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
