// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"caspianbyoc.org/caspian/internal/state"
)

// consumeFirstRunPassword performs the installer handoff described in
// docs/INSTALL.md, "The handoff, which needs the panel to cooperate":
//
//	On its first start, the panel reads that file, passes the contents to
//	state.Store.SetPanelPassword, which hashes it with argon2id, and then
//	deletes the file.
//
// A file rather than an argument or an environment variable, because both of
// those are readable from /proc by anything on the box.
//
// The four cases, and what each one does:
//
//   - no password set, file present: the password is set from it and the file
//     is deleted. This is the fresh install.
//   - no password set, file absent: nothing happens, and internal/panel shows
//     its setup screen. docs/INSTALL.md names state.ErrNoPanelPassword as the
//     signal for exactly that.
//   - password already set, file absent: nothing happens. This is every start
//     after the first, and every upgrade, because install.sh generates a
//     password on a fresh install only.
//   - password already set, file present: the file is a leftover. It is
//     DELETED without being used, and that is said out loud. The password in it
//     was never the one in use, so it opens nothing; keeping a plaintext
//     credential on disk that nothing will ever consume is a cost with no
//     purpose.
//
// It never logs the password, and it never returns it.
func consumeFirstRunPassword(store *state.Store, path string, log *slog.Logger) error {
	alreadySet := store.Snapshot().Panel.IsSet()

	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if !alreadySet {
			log.Warn("no panel password is set and the installer left none, so the panel will ask for one",
				"expected_file", path)
		}
		return nil
	case err != nil:
		// Readable by the caspian account inside a 0700 directory it owns, so
		// a failure here is a permissions problem on the box rather than a
		// normal state. It is reported and does not stop the panel: the setup
		// screen is still a way in.
		return fmt.Errorf("the first-run password at %s could not be read, so the password the installer "+
			"printed will not work. The panel will ask you to choose one instead: %w", path, err)
	}

	if alreadySet {
		log.Warn("a first-run password file was left behind after a password had already been set; "+
			"removing it without using it, because the password printed by that install was never the one in use",
			"file", path)
		return removeSecretFile(path, log)
	}

	// Trimmed because a file written by hand, or by an installer using echo
	// rather than printf, carries a trailing newline, and a password with an
	// invisible character on the end is one nobody can type. The generated
	// alphabet is lower-case letters, digits and hyphens (docs/INSTALL.md,
	// "First run"), so nothing legitimate is removed.
	password := strings.TrimSpace(string(raw))
	if password == "" {
		return fmt.Errorf("the first-run password at %s is empty, so the password the installer printed "+
			"will not work. The panel will ask you to choose one instead", path)
	}

	if err := store.SetPanelPassword(password); err != nil {
		// internal/state hashes with argon2id and never keeps the plaintext.
		// The error it returns names no value.
		return fmt.Errorf("the first-run password could not be saved: %w", err)
	}
	log.Info("the panel password was set from the file the installer left", "file", path)

	return removeSecretFile(path, log)
}

// removeSecretFile deletes a file holding a credential and is loud when it
// cannot.
//
// A failure here is not fatal to the panel, and it is serious: the plaintext of
// the panel password is still on the disk. Saying so at Error level is the only
// thing this program can do about it.
func removeSecretFile(path string, log *slog.Logger) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Error("the first-run password file could not be deleted and still holds the password in plain text; "+
			"remove it by hand", "file", path, "error", err.Error())
		return nil
	}
	return nil
}
