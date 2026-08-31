// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd keeps this suite honest
// about its own coverage.
//
// internal/link's supportedSchemes map is the list of things a user can paste
// and have accepted. A scheme added there without a row in protocolCases would
// leave a protocol accepted by the product and proven by nothing, which is the
// exact gap this package was written to close.
//
// The list is READ OUT OF THE SOURCE rather than restated here. A restated copy
// would go stale the moment somebody edited the parser, and it would go stale
// silently: the guard would keep checking a list that no longer describes
// anything, and report covered forever. That is the same failure the sentinel
// registry in test/goldenscan carries DeclaredIn to prevent.
func TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd(t *testing.T) {
	accepted := schemesInternalLinkAccepts(t)
	if len(accepted) == 0 {
		t.Fatal("internal/link accepts no scheme at all, which cannot be right; " +
			"this guard would pass vacuously")
	}

	covered := map[string]bool{}
	for _, p := range protocolCases() {
		covered[p.scheme] = true
	}
	for _, scheme := range accepted {
		if !covered[scheme] {
			t.Errorf("internal/link accepts %q and no row in this suite drives it end to end. "+
				"Add a protocolCase for it, or this product accepts a share link nothing has ever "+
				"pushed a byte through", scheme)
		}
	}

	// The other direction. A row here for a scheme the parser does not accept
	// would be a test that cannot be reached from the product.
	acceptedSet := map[string]bool{}
	for _, s := range accepted {
		acceptedSet[s] = true
	}
	for _, p := range protocolCases() {
		if !acceptedSet[p.scheme] {
			t.Errorf("this suite drives the scheme %q, which internal/link does not accept", p.scheme)
		}
	}

	sort.Strings(accepted)
	t.Logf("internal/link accepts %v, and every one of them has a carriage row", accepted)
}

// schemesInternalLinkAccepts returns the keys of internal/link's
// supportedSchemes map, read by parsing the file.
//
// Parsing rather than grepping, for the reason test/smoke/registry_test.go
// gives for the same choice: a grep matches the name in a comment, in a string
// and in code that has been commented out, and each of those would let this
// guard pass while the thing it names does not exist.
func schemesInternalLinkAccepts(t *testing.T) []string {
	t.Helper()
	const declaredIn = "internal/link/link.go"
	const declaredAs = "supportedSchemes"

	path := filepath.Join(moduleRoot(t), filepath.FromSlash(declaredIn))
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", declaredIn, err)
	}

	var out []string
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name != declaredAs || i >= len(spec.Values) {
				continue
			}
			lit, ok := spec.Values[i].(*ast.CompositeLit)
			if !ok {
				t.Fatalf("%s in %s is no longer a composite literal, so this guard cannot read it",
					declaredAs, declaredIn)
			}
			found = true
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					t.Fatalf("%s in %s has an element that is not a key/value pair", declaredAs, declaredIn)
				}
				key, ok := kv.Key.(*ast.BasicLit)
				if !ok || key.Kind != token.STRING {
					t.Fatalf("%s in %s has a non-string key", declaredAs, declaredIn)
				}
				scheme, uerr := strconv.Unquote(key.Value)
				if uerr != nil {
					t.Fatalf("unquoting a key of %s: %v", declaredAs, uerr)
				}
				out = append(out, scheme)
			}
		}
		return true
	})

	if !found {
		t.Fatalf("%s no longer declares %s, so this guard is covering nothing. "+
			"Point it at whatever replaced it rather than deleting it", declaredIn, declaredAs)
	}
	return out
}

// moduleRoot walks up from the working directory to the directory holding
// go.mod. Same helper, same reason, as test/smoke/registry_test.go: a test that
// reads another package's source has to find the module root rather than assume
// how deep it is.
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
