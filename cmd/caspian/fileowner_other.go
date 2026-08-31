// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build !unix

package main

import "io/fs"

// ownerOf reports that this platform does not carry the ownership docs/LAYOUT.md
// fixes. It says so rather than inventing a plausible answer.
func ownerOf(fs.FileInfo) string { return "not reported on this platform" }
