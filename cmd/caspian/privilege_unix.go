// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build unix

package main

import (
	"os"
	"strconv"
)

// runningPrivileged reports whether this process may run the privileged role,
// and who it is running as, in words for a refusal.
func runningPrivileged() (bool, string) {
	uid := os.Geteuid()
	if uid == 0 {
		return true, "root"
	}
	return false, "user id " + strconv.Itoa(uid)
}
