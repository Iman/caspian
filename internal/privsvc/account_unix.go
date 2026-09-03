// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build unix

package privsvc

// lookupAccountSID has no meaning on unix: accounts are uids, resolved by
// AllowedFor through os/user.
func lookupAccountSID(string) (string, bool) { return "", false }
