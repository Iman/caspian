// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPackageLogsNothing enforces the rule in the package doc: nothing in this
// package writes to a stream.
//
// It parses the package's own non-test source rather than grepping it. A text
// scan was tried first and was wrong in both directions: it matched the JSON
// struct tag `json:"log"` in build.go and the phrase "fmt.Print" inside a
// comment in doc.go, while it would have missed a dot-imported logger or an
// aliased one. The AST knows the difference between a comment, a struct tag
// and a call.
//
// Why the rule exists: the document this package builds carries the user's key
// material. Anything here that writes to stdout, stderr or a log file writes a
// credential to whatever is catching it, and on this appliance that is a
// systemd journal a support request would be asked to paste.
func TestPackageLogsNothing(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return filepath.Ext(fi.Name()) == ".go" &&
			!strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing the package: %v", err)
	}
	pkg, ok := pkgs["xcfg"]
	if !ok || len(pkg.Files) == 0 {
		t.Fatal("no non-test source files parsed; the scan would pass vacuously")
	}

	bannedImports := map[string]bool{
		"log":      true,
		"log/slog": true,
		"os/exec":  true,
	}
	// Package-qualified writers. The key is the package identifier as
	// imported, the value the set of selectors that write.
	bannedSelectors := map[string]map[string]bool{
		"fmt": {"Print": true, "Printf": true, "Println": true,
			"Fprint": true, "Fprintf": true, "Fprintln": true},
		"os": {"Stdout": true, "Stderr": true},
	}

	scanned := 0
	for name, file := range pkg.Files {
		scanned++
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquoting import: %v", name, err)
			}
			if bannedImports[path] {
				t.Errorf("%s imports %q; this package must not write to any stream or run a command", name, path)
			}
			if imp.Name != nil && imp.Name.Name == "." {
				// A dot import would make every check below blind, because
				// the writer would appear as a bare identifier.
				t.Errorf("%s uses a dot import of %q, which defeats this scan", name, path)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.SelectorExpr:
				id, ok := v.X.(*ast.Ident)
				if !ok {
					return true
				}
				if sels, ok := bannedSelectors[id.Name]; ok && sels[v.Sel.Name] {
					t.Errorf("%s uses %s.%s, which writes to a stream", name, id.Name, v.Sel.Name)
				}
			case *ast.CallExpr:
				// The builtins, which have no package qualifier.
				if id, ok := v.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
					t.Errorf("%s calls the builtin %s, which writes to stderr", name, id.Name)
				}
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("every file was skipped; the scan would pass vacuously")
	}
	t.Logf("parsed %d non-test source files", scanned)
}
