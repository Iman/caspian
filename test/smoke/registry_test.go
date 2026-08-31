// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Package smoke holds the guards on smoke.list.
//
// The list itself is data. What makes it trustworthy is that every name in it
// is checked against the source on every gate run, because the failure mode of
// a -run pattern is silent:
//
//	go test ./internal/panel -run '^TestThatWasRenamed$'
//	ok      caspianbyoc.org/caspian/internal/panel   0.2s
//	exit 0
//
// Nothing executed. The exit code says success. That is the same shape as the
// three false greens recorded in the header of scripts/gate.sh, and it is why
// scripts/smoke.sh also counts the PASS lines it actually saw rather than
// trusting its own exit code.
package smoke

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// entry is one line of smoke.list.
type entry struct {
	Pkg   string
	Regex string
	Line  int
	// Names are the test names the regex alternates over, with the anchors
	// stripped. The list is written as an explicit alternation of anchored
	// names precisely so this is possible.
	Names []string
}

func readList(t *testing.T) []entry {
	t.Helper()
	root := moduleRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "test", "smoke", "smoke.list"))
	if err != nil {
		t.Fatalf("reading smoke.list: %v", err)
	}
	var out []entry
	for i, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) != 2 {
			t.Fatalf("smoke.list line %d is not <package> <regex>: %q", i+1, line)
		}
		e := entry{Pkg: fields[0], Regex: fields[1], Line: i + 1}
		for _, alt := range strings.Split(fields[1], "|") {
			name := strings.TrimSuffix(strings.TrimPrefix(alt, "^"), "$")
			if name == "" {
				t.Fatalf("smoke.list line %d has an empty alternative in %q", i+1, fields[1])
			}
			e.Names = append(e.Names, name)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		t.Fatal("smoke.list has no entries, so a smoke run would execute nothing and exit 0")
	}
	return out
}

// testFuncsIn returns every top-level test function declared in a package
// directory, by parsing the source rather than by grepping it.
//
// Parsing rather than grepping is not fussiness. A grep for "func TestFoo("
// matches the name inside a comment, inside a string, and inside a function
// that has been commented out, and each of those would let this guard pass
// while the test does not exist.
func testFuncsIn(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				if strings.HasPrefix(fn.Name.Name, "Test") {
					out[fn.Name.Name] = true
				}
			}
		}
	}
	return out
}

// TestEverySmokeTestNamedInTheListStillExists is the guard that stops the smoke
// subset from silently becoming empty.
func TestEverySmokeTestNamedInTheListStillExists(t *testing.T) {
	root := moduleRoot(t)
	checked := 0
	for _, e := range readList(t) {
		dir := filepath.Join(root, filepath.FromSlash(e.Pkg))
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("smoke.list line %d names package %s, which does not exist: %v", e.Line, e.Pkg, err)
			continue
		}
		have := testFuncsIn(t, dir)
		for _, name := range e.Names {
			checked++
			if !have[name] {
				t.Errorf("smoke.list line %d names %s in %s and no such test function is declared "+
					"there. 'go test -run' would match nothing, execute nothing, and exit 0, which "+
					"is a smoke run that tested no code.", e.Line, name, e.Pkg)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no test names were checked, so this guard passed vacuously")
	}
	t.Logf("checked %d smoke test names against the source", checked)
}

// smokeExempt are packages with tests that the smoke subset deliberately does
// not reach, each with the reason.
//
// An exemption list rather than silence: without it, a new package with tests
// gets no smoke coverage and nothing says so.
var smokeExempt = map[string]string{
	"internal/panel/qr": "the QR encoder is exercised through internal/panel's own smoke entries, " +
		"which render a join code on every pinned page",
	"third_party/libxray-share": "vendored. Its tests run in the gate; a change to it is a vendor " +
		"update, which is never the thing a smoke run is protecting against",
	"test/bdd": "the behaviour suite. It is slower than the whole smoke budget and it is what the " +
		"gate exists for",
	"test/smoke":  "this package. Its own guards run in the gate",
	"bdd/harness": "a helper binary for the Cucumber suites, owned elsewhere",
}

// TestEveryPackageWithTestsIsEitherInSmokeOrExempt.
//
// The failure this catches is a new package arriving with no smoke coverage and
// nothing noticing. The fix is one line in smoke.list or one line here with a
// reason, and either is a decision somebody made rather than an omission
// nobody saw.
func TestEveryPackageWithTestsIsEitherInSmokeOrExempt(t *testing.T) {
	root := moduleRoot(t)
	inList := map[string]bool{}
	for _, e := range readList(t) {
		inList[e.Pkg] = true
	}

	var missing []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		switch info.Name() {
		case ".git", "node_modules", "testdata", ".codegraph", ".idea", "local":
			return filepath.SkipDir
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		hasTests := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
				hasTests = true
				break
			}
		}
		if !hasTests {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if inList[rel] {
			return nil
		}
		if _, ok := smokeExempt[rel]; ok {
			return nil
		}
		missing = append(missing, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range missing {
		t.Errorf("package %s has tests and is neither in test/smoke/smoke.list nor in the "+
			"smokeExempt map. Add a fast test to the list, or add an exemption with a reason.", m)
	}
}

// TestSmokeExemptionsAreAllStillRealPackages stops the exemption map rotting.
//
// An exemption naming a package that no longer exists is an exemption that
// covers nothing, and the next package to take that path inherits it.
func TestSmokeExemptionsAreAllStillRealPackages(t *testing.T) {
	root := moduleRoot(t)
	for pkg, reason := range smokeExempt {
		if len(reason) < 30 {
			t.Errorf("the exemption for %s has a %d character reason, which is too short to be one",
				pkg, len(reason))
		}
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("smokeExempt names %s, which does not exist. Delete the entry: an exemption "+
				"for a package that is gone will be inherited by the next one at that path.", pkg)
		}
	}
}
