// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package privsvc

import "golang.org/x/sys/windows"

// lookupAccountSID resolves an account name (for example the virtual service
// account "NT SERVICE\caspian-panel") to its SID string.
func lookupAccountSID(name string) (string, bool) {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return "", false
	}
	return sid.String(), true
}
