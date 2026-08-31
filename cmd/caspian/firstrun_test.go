// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

// The password the installer generates: twenty characters from a thirty-two
// character alphabet with no 0, O, 1, l or I, printed as five-character groups.
// docs/INSTALL.md, "First run".
const installerPassword = "nvbqd-3kx7m-rjhta-92wpe"

func newStore(t *testing.T) (*state.Store, string) {
	t.Helper()
	dir := t.TempDir()
	// 0700, because internal/state refuses to read a state directory any other
	// user on the box could look into, and t.TempDir gives 0755. The installer
	// creates it at 0700; docs/LAYOUT.md fixes that.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	store, err := state.Load(dir)
	if err != nil {
		t.Fatalf("loading state: %v", err)
	}
	return store, dir
}

func writeSeed(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, "first-run-password")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing the first-run password: %v", err)
	}
	return path
}

func testLogger(w *strings.Builder) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestTheFirstRunPasswordIsConsumedAndTheFileDeleted is the handoff
// docs/INSTALL.md describes, and the one thing that makes the password the
// installer prints work at all. Until this ran, "the printed password does not
// work" was a known open item in that document.
func TestTheFirstRunPasswordIsConsumedAndTheFileDeleted(t *testing.T) {
	store, dir := newStore(t)
	path := writeSeed(t, dir, installerPassword)

	var logs strings.Builder
	if err := consumeFirstRunPassword(store, path, testLogger(&logs)); err != nil {
		t.Fatalf("the handoff failed: %v", err)
	}

	ok, err := store.VerifyPanelPassword(installerPassword)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if !ok {
		t.Fatalf("the password the installer printed does not open the panel")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the plaintext password is still on disk at %s", path)
	}

	// The password is a credential and must not have reached a log line, nor
	// the redacted rendering of the state.
	if strings.Contains(logs.String(), installerPassword) {
		t.Fatalf("the password reached a log line:\n%s", logs.String())
	}
	if strings.Contains(store.Snapshot().Redacted(), installerPassword) {
		t.Fatalf("the password appears in the state's own diagnostic rendering")
	}
	// And what IS stored is a verifier, not the password.
	if hash := store.Snapshot().Panel.PasswordHash.Reveal(); strings.Contains(hash, installerPassword) {
		t.Fatalf("the stored verifier contains the plaintext")
	} else if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("the stored verifier is not an argon2id hash: %q", hash)
	}
}

// TestATrailingNewlineInTheFileIsNotPartOfThePassword.
//
// install.sh writes the file with printf and no newline, but a file made by
// hand, or by a later installer using echo, carries one, and a password with an
// invisible character on the end is one nobody can type.
func TestATrailingNewlineInTheFileIsNotPartOfThePassword(t *testing.T) {
	store, dir := newStore(t)
	path := writeSeed(t, dir, installerPassword+"\n")

	var logs strings.Builder
	if err := consumeFirstRunPassword(store, path, testLogger(&logs)); err != nil {
		t.Fatalf("the handoff failed: %v", err)
	}
	ok, err := store.VerifyPanelPassword(installerPassword)
	if err != nil || !ok {
		t.Fatalf("the password did not survive a trailing newline: ok=%t err=%v", ok, err)
	}
}

// TestNoFileAndNoPasswordShowsSetup.
//
// docs/INSTALL.md names state.ErrNoPanelPassword as the signal for the panel to
// show its setup screen, which is the right fallback when the file is absent.
// The handoff must therefore do nothing at all rather than fail.
func TestNoFileAndNoPasswordShowsSetup(t *testing.T) {
	store, dir := newStore(t)

	var logs strings.Builder
	if err := consumeFirstRunPassword(store, filepath.Join(dir, "first-run-password"), testLogger(&logs)); err != nil {
		t.Fatalf("an absent file was treated as a failure: %v", err)
	}
	if store.Snapshot().Panel.IsSet() {
		t.Fatalf("a password was set from a file that does not exist")
	}
	if _, err := store.VerifyPanelPassword("anything"); !errors.Is(err, state.ErrNoPanelPassword) {
		t.Fatalf("the store does not report ErrNoPanelPassword, which is what tells the panel to show setup: %v", err)
	}
	if !strings.Contains(logs.String(), "no panel password is set") {
		t.Fatalf("nothing was said about the box having no password:\n%s", logs.String())
	}
}

// TestALeftoverFileIsRemovedWithoutBeingUsed.
//
// The password in it was never the one in use, so it opens nothing. Keeping a
// plaintext credential on disk that nothing will ever consume is a cost with no
// purpose.
func TestALeftoverFileIsRemovedWithoutBeingUsed(t *testing.T) {
	store, dir := newStore(t)
	if err := store.SetPanelPassword("the-one-the-user-chose"); err != nil {
		t.Fatalf("setting a password: %v", err)
	}
	path := writeSeed(t, dir, installerPassword)

	var logs strings.Builder
	if err := consumeFirstRunPassword(store, path, testLogger(&logs)); err != nil {
		t.Fatalf("the handoff failed: %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a leftover plaintext password was left on disk")
	}
	ok, err := store.VerifyPanelPassword("the-one-the-user-chose")
	if err != nil || !ok {
		t.Fatalf("the password already in use was replaced by a leftover file: ok=%t err=%v", ok, err)
	}
	if ok, _ := store.VerifyPanelPassword(installerPassword); ok {
		t.Fatalf("the leftover password now opens the panel")
	}
	if !strings.Contains(logs.String(), "left behind") {
		t.Fatalf("nothing was said about removing a leftover credential:\n%s", logs.String())
	}
}

// TestAnEmptyFileIsRefusedRatherThanSettingAnEmptyPassword.
func TestAnEmptyFileIsRefusedRatherThanSettingAnEmptyPassword(t *testing.T) {
	store, dir := newStore(t)
	path := writeSeed(t, dir, "   \n")

	err := consumeFirstRunPassword(store, path, testLogger(&strings.Builder{}))
	if err == nil {
		t.Fatalf("an empty first-run password file was accepted")
	}
	if store.Snapshot().Panel.IsSet() {
		t.Fatalf("a password was set from an empty file")
	}
	if !strings.Contains(err.Error(), "ask you to choose one") {
		t.Fatalf("the message does not say what happens next: %v", err)
	}
}

// TestAnUnreadableFileIsReportedAndDoesNotStopThePanel.
func TestAnUnreadableFileIsReportedAndDoesNotStopThePanel(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a file with no permission bits, so this case cannot be built here")
	}
	store, dir := newStore(t)
	path := writeSeed(t, dir, installerPassword)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	err := consumeFirstRunPassword(store, path, testLogger(&strings.Builder{}))
	if err == nil {
		t.Fatalf("a file that could not be read was treated as success")
	}
	if !strings.Contains(err.Error(), "will not work") {
		t.Fatalf("the message does not say the printed password will not work: %v", err)
	}
	if store.Snapshot().Panel.IsSet() {
		t.Fatalf("a password was set from a file that could not be read")
	}
}
