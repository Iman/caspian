// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !unix

package main

import "io/fs"

// Windows state files inherit their service ACL from the state directory.
func restoreStateOwner(string, fs.FileInfo) error { return nil }
