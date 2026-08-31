// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package state

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// File modes are part of the design here, not housekeeping. The state file
// holds the pasted proxy config and the hotspot passphrase in the clear,
// because the engine has to be able to replay them. The only thing standing
// between those and another account on the box is the mode, so the mode is
// checked on every load rather than assumed to be whatever Save last set.
//
// Group bits are refused alongside other bits. "World-readable" was the stated
// requirement, but a credential readable by the group is the same disclosure
// with a smaller audience, and on a Raspberry Pi OS install the default user is
// in a long list of groups.

// permChecksEnforced tells the tests whether to expect a refusal. See
// perm_other.go for the platform where it is false.
const permChecksEnforced = true

// checkPerms refuses a path whose permission bits are looser than want, or
// whose owner is neither the calling user nor root.
//
// The owner rule allows root because the design splits the program in two
// (section 5.5): an unprivileged panel and a privileged network service. The
// privileged side running as root legitimately reads a file owned by the
// service user. Anything else owning it means the file is not the one this
// service wrote, which is worth stopping on rather than reading.
func checkPerms(path string, fi fs.FileInfo, want fs.FileMode, kind string) error {
	if got := fi.Mode().Perm(); got&^want != 0 {
		return fmt.Errorf(
			"state: %s %s has permissions %#o, which lets other users on this box read the proxy config and hotspot passphrase it holds; "+
				"fix it with: chmod %#o %s",
			kind, path, got, want, path)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// Nothing to check against. Not an error: the mode check above already
		// ran, and inventing a failure here would break on a filesystem that
		// does not report a Stat_t.
		return nil
	}
	euid := os.Geteuid()
	if int(st.Uid) != euid && euid != 0 {
		return fmt.Errorf(
			"state: %s %s is owned by uid %d, but this process is running as uid %d; "+
				"it is not the file this service wrote. Fix it with: chown %d %s",
			kind, path, st.Uid, euid, euid, path)
	}
	return nil
}
