// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNothingInTheApplianceWatchesTheUplink is a guard on an ABSENCE, which is
// the only kind of test that can hold this particular sentence honest.
//
// internal/netcfg ships WatchUplink, ReadUplinkState and Plan.RederiveForUplink.
// They work, and internal/netcfg's own tests prove they work. Until 2026-08-30
// docs/BEHAVIOUR.md and the comment above RederiveForUplink both said the box
// used them: "The box notices, takes the old route away before adding the new
// one, and reloads the firewall as well." It does not. The only caller was the
// scenario step in this package, which performed the move itself and then
// asserted it had happened, so the suite was green on a promise nothing kept.
//
// This test fails the moment a shipped file calls one of them. That is not a
// prohibition on ever wiring a watcher in: it is a requirement that whoever
// does also updates the promise, because the promise and the code are the same
// claim written twice and only one of them is executable.
func TestNothingInTheApplianceWatchesTheUplink(t *testing.T) {
	// The uplink watcher's entry points. Naming them as strings is deliberate:
	// referring to the symbols would make this file a caller.
	watchers := []string{"WatchUplink(", "ReadUplinkState(", "RederiveForUplink("}

	root := filepath.Join("..", "..")
	// Where they are DEFINED, and the one place a mention is not a call.
	definitions := filepath.Join("internal", "netcfg", "uplink.go")

	var callers []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".idea", ".codegraph", "third_party", "local":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if rel == definitions {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, w := range watchers {
			if strings.Contains(string(body), w) {
				callers = append(callers, rel+" calls "+strings.TrimSuffix(w, "("))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}

	if len(callers) > 0 {
		t.Fatalf("the appliance now watches its uplink:\n  %s\n\n"+
			"That is a change in what the product promises, not only in what it does. Update "+
			"docs/BEHAVIOUR.md, \"a change of uplink leaves the box blocked and waiting for a "+
			"reconnect\", and the scenario behind it, then delete this test. Leaving it red, or "+
			"deleting it without rewriting the promise, puts the document back to asserting "+
			"behaviour nothing implements, which is what it did until 2026-08-30.",
			strings.Join(callers, "\n  "))
	}

	// The test must be able to see a caller at all, or it would pass on a
	// walk that read nothing. This file's own package is not searched, so the
	// check is that the walk reached real source.
	var goFiles int
	_ = filepath.WalkDir(filepath.Join(root, "internal"), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			goFiles++
		}
		return nil
	})
	if goFiles < 20 {
		t.Fatalf("the walk found only %d files under internal/, so a caller could have been "+
			"missed and this green result means nothing", goFiles)
	}
}
