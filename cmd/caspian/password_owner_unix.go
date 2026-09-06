// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package main

import (
	"io/fs"
	"os"
	"syscall"
)

func restoreStateOwner(path string, fi fs.FileInfo) error {
	st := fi.Sys().(*syscall.Stat_t)
	return os.Chown(path, int(st.Uid), int(st.Gid))
}
