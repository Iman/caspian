// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package main

import "golang.org/x/sys/windows"

// runningPrivileged reports whether this process holds an elevated token.
//
// os.Geteuid is -1 on Windows, so the unix check would refuse every start. A
// Windows service running as LocalSystem and an administrator's elevated
// console both carry an elevated token; a standard user's token, and an
// administrator's un-elevated one, do not.
func runningPrivileged() (bool, string) {
	tok := windows.GetCurrentProcessToken()
	if tok.IsElevated() {
		return true, "an elevated account"
	}
	return false, "an account without elevation"
}
