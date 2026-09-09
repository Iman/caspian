//go:build !windows

// SPDX-License-Identifier: AGPL-3.0-or-later
package state

import "os"

func replaceStateFile(from, to string) error { return os.Rename(from, to) }

func readStateFile(path string) ([]byte, error) { return os.ReadFile(path) }
