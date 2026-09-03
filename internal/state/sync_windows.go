// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package state

// syncDir is a no-op on Windows.
//
// Go opens a directory with GENERIC_READ and FILE_FLAG_BACKUP_SEMANTICS, and
// FlushFileBuffers on such a handle fails with ERROR_ACCESS_DENIED (it needs
// GENERIC_WRITE). Running the unix version here would report every save as
// failed after the rename had already succeeded, which leaves the store's
// memory behind its disk. NTFS journals directory metadata itself, so the
// rename's durability does not depend on this call the way ext4's does.
func syncDir(string) error { return nil }
