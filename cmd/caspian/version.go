// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
)

// Build information. version and commit are set at link time:
//
//	go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git rev-parse HEAD)"
//
// They are variables rather than constants because that is the only way
// -ldflags -X can reach them. An unset version reads "dev", which is the honest
// answer for a binary built without the release pipeline and is not the same
// thing as a release with an empty version string.
var (
	version = "dev"
	commit  = ""
)

// writeVersion prints what this binary is.
//
// The revision and the dirty flag come from the build info the toolchain
// records, so a binary built without -ldflags still says which commit it came
// from and whether the tree was clean. docs/2026-08-29-design.md makes the
// engine a linked dependency, so its version is part of what this binary is and
// is printed with it: an engine security fix is a rebuild of this binary, and
// somebody has to be able to see which engine they are carrying.
func writeVersion(w io.Writer) {
	fmt.Fprintf(w, "caspian %s\n", version)

	rev, dirty, engineVersion := buildFacts()
	shown := commit
	if shown == "" {
		shown = rev
	}
	if shown != "" {
		fmt.Fprintf(w, "commit:   %s\n", shown)
		if dirty {
			// Only alongside a revision. "modified since that commit" with no
			// commit named is a sentence with nothing in it, and a build from
			// a tree with no commits at all reaches exactly that.
			fmt.Fprintf(w, "tree:     modified since that commit\n")
		}
	}
	if engineVersion != "" {
		fmt.Fprintf(w, "engine:   %s\n", engineVersion)
	}
	fmt.Fprintf(w, "go:       %s\n", runtime.Version())
	fmt.Fprintf(w, "platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// buildFacts reads what the toolchain embedded.
func buildFacts() (revision string, dirty bool, engineVersion string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false, ""
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	for _, d := range info.Deps {
		if d.Path == enginePath {
			engineVersion = d.Path + " " + d.Version
		}
	}
	return revision, dirty, engineVersion
}

// enginePath is the module the tunnel engine comes from. It is written out
// rather than derived so that a change of engine is a change to this line and
// therefore visible in a diff.
const enginePath = "github.com/xtls/xray-core"
