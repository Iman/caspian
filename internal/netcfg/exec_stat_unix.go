// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package netcfg

import "os"

// lstatRegularExecutable reports whether path is a regular file with an
// execute bit. It follows symlinks, because the tools on Debian are frequently
// symlinks into /usr, and it rejects a directory or a device node so that a
// path that happens to exist is not mistaken for a binary.
func lstatRegularExecutable(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !fi.Mode().IsRegular() {
		return false, nil
	}
	return fi.Mode().Perm()&0o111 != 0, nil
}
