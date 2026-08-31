// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !unix

package state

import "io/fs"

// This file exists so the package still compiles if the Windows phase in design
// section 2 is ever started. It is not a port.
//
// The unix build checks POSIX permission bits and the owning uid. Windows has
// neither: os.FileInfo.Mode().Perm() reports a synthesised 0666 or 0777 there,
// so running the unix check unchanged would refuse every state file that exists.
// The equivalent protection on Windows is an ACL, and applying and verifying one
// is real work with no test coverage on this project's hardware.
//
// So on a non-unix platform the credential protection this package promises is
// NOT in force. That is stated here rather than hidden behind a nil return,
// and permChecksEnforced lets the tests assert the difference rather than
// silently pass.
const permChecksEnforced = false

func checkPerms(path string, fi fs.FileInfo, want fs.FileMode, kind string) error {
	return nil
}
