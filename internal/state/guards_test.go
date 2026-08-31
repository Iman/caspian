// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The tests in this file are CHARACTERISATION tests. Every behaviour asserted
// here was already correct on 2026-08-30; what was missing was anything that
// would notice if it stopped. They cover the branches the coverage profile
// showed unreached.
//
// The theme running through most of them: this package's job is to refuse
// rather than to guess. A state file that cannot be read, a directory that is
// not a directory, a schema version this build does not understand. Every one
// of those has a wrong answer that looks like it works, which is to fall back
// to defaults, and that wrong answer silently discards the user's proxy config
// and panel password.

// --- Load refuses what it cannot trust --------------------------------------

// TestLoadRefusesADirectoryThatIsAFile covers the !IsDir branch on the
// directory. It is reachable in practice: /var/lib/caspian is created by the
// installer, and a half-finished install or a stray touch leaves a file there.
func TestLoadRefusesADirectoryThatIsAFile(t *testing.T) {
	parent := t.TempDir()
	notADir := filepath.Join(parent, "caspian")
	if err := os.WriteFile(notADir, []byte("i am a file"), fileMode); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(notADir)
	if err == nil {
		t.Fatal("Load treated a regular file as its state directory")
	}
	// The assertion has to distinguish WHICH branch refused it. Removing this
	// check does not make Load succeed: it makes the stat of the state file
	// inside the non-directory fail instead, and the operating system's own
	// ENOTDIR message also contains the words "not a directory". So a test
	// that only looked for that phrase would pass with the guard deleted.
	// Verified by mutation on 2026-08-30: it did.
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
	if strings.Contains(err.Error(), "examining") {
		t.Errorf("the directory check did not fire; the failure came from a later stat instead: %v", err)
	}
}

// TestLoadRefusesAStateFileThatIsADirectory covers the IsDir branch on the
// file. Silently starting from defaults here would present the user with a
// setup screen and then fail every write.
func TestLoadRefusesAStateFileThatIsADirectory(t *testing.T) {
	dir := tempStateDir(t)
	if err := os.Mkdir(filepath.Join(dir, FileName), dirMode); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load treated a directory as its state file")
	}
	if !strings.Contains(err.Error(), "not a state file") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
}

// TestLoadRefusesAnUnreadableStateFile covers the ReadFile failure.
//
// The file is mode 0200: write only. That passes the permission check, because
// checkPerms refuses bits LOOSER than 0600 and 0200 is tighter, and then the
// read fails. This is the case that separates "cannot read it" from "it is not
// there": the second is a first run, the first is a fault, and answering the
// first with defaults would hand the user a blank box and throw away the
// config that is sitting on the disk.
func TestLoadRefusesAnUnreadableStateFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read a mode 0200 file")
	}
	dir := tempStateDir(t)
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o200); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// The premise: the permission check must NOT be what rejects this, or the
	// test is not reaching the read.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("setup stat: %v", err)
	}
	if err := checkPerms(path, fi, fileMode, "file"); err != nil {
		t.Fatalf("the permission check rejected mode 0200, so this test no longer reaches the read: %v", err)
	}

	store, err := Load(dir)
	if err == nil {
		t.Fatalf("an unreadable state file was loaded as first run = %v", store.FirstRun())
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("the error does not say the read failed: %v", err)
	}
}

// TestLoadReportsAStateFileItCannotExamine covers the Stat failure on the
// state FILE, as distinct from the one on the directory above.
//
// The setup is a directory at mode 0600: readable, not searchable. That is a
// real mis-permission, and note that it PASSES the permission check, because
// checkPerms refuses bits looser than 0700 and 0600 is tighter. So the
// directory looks acceptable and then nothing inside it can be stat'ed. The
// requirement is that this is an error rather than a first run.
func TestLoadReportsAStateFileItCannotExamine(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can search a directory with no execute bit")
	}
	dir := tempStateDir(t)
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"version":1}`), fileMode); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Chmod(dir, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, dirMode) })

	// The premise: the permission check must not be what rejects this.
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("setup stat: %v", err)
	}
	if err := checkPerms(dir, fi, dirMode, "directory"); err != nil {
		t.Fatalf("the permission check rejected mode 0600, so this test no longer reaches the file stat: %v", err)
	}

	store, err := Load(dir)
	if err == nil {
		t.Fatalf("an unsearchable state directory was loaded as first run = %v", store.FirstRun())
	}
	if !strings.Contains(err.Error(), "examining") {
		t.Errorf("the error does not say the examination failed: %v", err)
	}
}

// TestSaveReportsADirectoryItCannotExamine covers ensureDir's Stat failure,
// the write-side twin of the two Load cases above. The directory was fine when
// Load ran and its parent stopped being searchable afterwards.
func TestSaveReportsADirectoryItCannotExamine(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can search a directory with no execute bit")
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "caspian")
	if err := os.Mkdir(dir, dirMode); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The parent loses its execute bit, so the state directory can no longer
	// be stat'ed even though it is still there.
	if err := os.Chmod(parent, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	err = s.Save()
	if err == nil {
		t.Fatal("Save succeeded although the state directory cannot be examined")
	}
	if !strings.Contains(err.Error(), "examining") {
		t.Errorf("the error does not say the examination failed: %v", err)
	}
}

// TestStoreDirIsTheDirectoryItWasGiven covers Dir().
//
// It is a one-line accessor and it is tested because it is one of two
// near-identical accessors, Dir and Path, and returning the wrong one compiles.
// The privileged service uses Dir to set ownership on the state directory.
func TestStoreDirIsTheDirectoryItWasGiven(t *testing.T) {
	dir := tempStateDir(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.Dir(); got != dir {
		t.Errorf("Dir() = %q, want %q", got, dir)
	}
	if s.Dir() == s.Path() {
		t.Error("Dir() and Path() returned the same value; one of them is wrong")
	}
	if want := filepath.Join(dir, FileName); s.Path() != want {
		t.Errorf("Path() = %q, want %q", s.Path(), want)
	}
}

// --- writeAtomic and ensureDir failure paths --------------------------------

// TestSaveReportsAFailedRename covers the rename failure.
//
// It is set up by making the target path a directory AFTER Load has run, which
// is how a real box reaches it: something else created a directory of that
// name while the service was up. The requirement being checked is that the
// failure is reported. A swallowed rename error means Update publishes the new
// state to readers while the disk still holds the old one, and the box then
// behaves one way until it reboots and another way afterwards.
func TestSaveReportsAFailedRename(t *testing.T) {
	dir := tempStateDir(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Nothing on disk yet, so this is a first run. Now take the name.
	if err := os.Mkdir(s.Path(), dirMode); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err = s.SetHotspot(fakeSSID, fakePassphrase)
	if err == nil {
		t.Fatal("Save succeeded although the target path is a directory")
	}
	if !strings.Contains(err.Error(), "replacing") {
		t.Errorf("the error does not say the replace failed: %v", err)
	}

	// The failed write must not have been published to readers.
	if got := s.Hotspot().SSID; got != "" {
		t.Errorf("a failed write was published to readers: SSID = %q", got)
	}

	// And no temporary file was left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".state-") {
			t.Errorf("a temporary file was left behind after a failed write: %s", e.Name())
		}
	}
}

// TestSaveRefusesADirectoryThatBecameAFile covers ensureDir's !IsDir branch.
// Same shape as the rename case: the directory was fine at Load and is not
// fine now.
func TestSaveRefusesADirectoryThatBecameAFile(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "caspian")

	// Missing directory at Load, which is a first run.
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !s.FirstRun() {
		t.Fatal("a missing directory did not read as a first run")
	}

	// Something takes the name as a file before the first write.
	if err := os.WriteFile(dir, []byte("not a directory"), fileMode); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err = s.Save()
	if err == nil {
		t.Fatal("Save wrote into a path that is a regular file")
	}
	// As in TestLoadRefusesADirectoryThatIsAFile: without this guard the write
	// still fails, but at the temporary-file creation, whose ENOTDIR message
	// also reads "not a directory". The distinguishing assertion is the one
	// below. Verified by mutation on 2026-08-30.
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
	if strings.Contains(err.Error(), "creating a temporary file") {
		t.Errorf("the directory check did not fire; the failure came from the temporary file instead: %v", err)
	}
}

// TestLoadReportsADirectoryItCannotExamine covers the Stat failure on the
// state directory, which is the branch BELOW "it is not there".
//
// The distinction is the whole point. A directory that is not there is a first
// run. A directory that cannot be examined is a fault, and answering it with
// first-run defaults would show the user a setup screen while their config sat
// on the disk. Here the parent is a regular file, so the stat fails with
// something that is not fs.ErrNotExist.
func TestLoadReportsADirectoryItCannotExamine(t *testing.T) {
	parent := t.TempDir()
	blocker := filepath.Join(parent, "blocker")
	if err := os.WriteFile(blocker, []byte("in the way"), fileMode); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(filepath.Join(blocker, "caspian"))
	if err == nil {
		t.Fatal("Load treated an unexaminable directory as a first run")
	}
	if !strings.Contains(err.Error(), "examining") {
		t.Errorf("the error does not say the examination failed: %v", err)
	}
}

// TestSaveReportsAFailedDirectoryCreation covers the MkdirAll failure.
//
// The parent is readable and searchable but not writable, so Load can see that
// the state directory is absent, which is a first run, and the first write
// then cannot create it. That is the shape a real box hits when the installer
// has not run or has run as the wrong user.
func TestSaveReportsAFailedDirectoryCreation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can write into a directory with no write bit")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// t.TempDir cleanup needs the write bit back.
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	dir := filepath.Join(parent, "caspian")
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !s.FirstRun() {
		t.Fatal("a missing state directory did not read as a first run")
	}

	err = s.Save()
	if err == nil {
		t.Fatal("Save created a directory inside a parent it cannot write to")
	}
	if !strings.Contains(err.Error(), "creating") {
		t.Errorf("the error does not say the creation failed: %v", err)
	}
}

// --- the schema version guard -----------------------------------------------

// TestValidateRefusesToWriteAForeignSchemaVersion covers the version check in
// State.validate.
//
// Update sets the version itself before calling validate, so this branch
// cannot be reached through the public API. It is tested directly because it
// is the last guard against a code path that forgets to set the version and
// writes a file this build could not load back.
func TestValidateRefusesToWriteAForeignSchemaVersion(t *testing.T) {
	st := fullState(t)

	st.Version = CurrentVersion
	if err := st.validate(); err != nil {
		t.Fatalf("a state at the current version was refused: %v", err)
	}

	for _, v := range []int{0, CurrentVersion + 1, -1, 99} {
		st.Version = v
		err := st.validate()
		if err == nil {
			t.Errorf("validate accepted schema version %d, but this build writes %d", v, CurrentVersion)
			continue
		}
		if !strings.Contains(err.Error(), "refusing to write") {
			t.Errorf("the error for version %d does not say it is refusing: %v", v, err)
		}
	}
}

// --- migrate ----------------------------------------------------------------

// TestMigrateReportsAMissingStepRatherThanLooping covers the missing-migration
// branch.
//
// The failure it prevents is not a wrong answer, it is a hang: the loop in
// migrate runs while Version < CurrentVersion, so a gap in the table with no
// error return spins forever on a box the user cannot log into.
//
// The migrations table is swapped for the duration and restored, so this test
// cannot leak into another.
func TestMigrateReportsAMissingStepRatherThanLooping(t *testing.T) {
	original := migrations
	t.Cleanup(func() { migrations = original })

	migrations = map[int]func(*State) error{} // no step from 0 to 1

	st := defaultState()
	st.Version = 0
	err := migrate(&st, "/var/lib/caspian/state.json")
	if err == nil {
		t.Fatal("migrate accepted a state it has no migration for")
	}
	if !strings.Contains(err.Error(), "no migration from 0 to 1") {
		t.Errorf("the error does not name the missing step: %v", err)
	}
	// The path is named so an administrator knows which file to move aside.
	if !strings.Contains(err.Error(), "/var/lib/caspian/state.json") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// TestMigrateWrapsAFailingStep covers the error-wrapping branch, and pins that
// the wrapping keeps the cause reachable through errors.Is. A migration that
// fails has to say which step failed, because "the state file is broken" sends
// the user to delete a file that is fine.
func TestMigrateWrapsAFailingStep(t *testing.T) {
	original := migrations
	t.Cleanup(func() { migrations = original })

	sentinel := errors.New("the disk said no")
	migrations = map[int]func(*State) error{
		0: func(*State) error { return sentinel },
	}

	st := defaultState()
	st.Version = 0
	err := migrate(&st, "/var/lib/caspian/state.json")
	if err == nil {
		t.Fatal("migrate reported success although the migration failed")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("the cause is not reachable through errors.Is: %v", err)
	}
	if !strings.Contains(err.Error(), "from schema version 0 to 1") {
		t.Errorf("the error does not name the step that failed: %v", err)
	}
}

// --- the PHC parameter fields -----------------------------------------------

// TestDecodeHashRejectsEachUnreadableParameter covers the t= and p= parsing
// branches.
//
// The existing table in password_test.go covers the m= branch twice, through
// "unreadable memory" and through "parameters out of order", and never reaches
// the other two: every one of its malformed cases fails at or before m=. So a
// bug that dropped the t or p check would have been invisible.
//
// It is not a theoretical gap. The bound on each of these exists because a
// corrupted value turns a login attempt into an out-of-memory kill or a
// multi-minute hang, on an appliance whose only interface is the panel.
func TestDecodeHashRejectsEachUnreadableParameter(t *testing.T) {
	tests := []struct {
		name   string
		stored string
	}{
		// m is well formed in every case below, so the failure has to come
		// from the field being named.
		{"unreadable passes", "$argon2id$v=19$m=65536,t=lots,p=4$c2FsdA$a2V5"},
		{"unreadable lanes", "$argon2id$v=19$m=65536,t=3,p=many$c2FsdA$a2V5"},
		{"passes field is not named t", "$argon2id$v=19$m=65536,x=3,p=4$c2FsdA$a2V5"},
		{"lanes field is not named p", "$argon2id$v=19$m=65536,t=3,x=4$c2FsdA$a2V5"},
		{"passes value is empty", "$argon2id$v=19$m=65536,t=,p=4$c2FsdA$a2V5"},
		{"lanes value is empty", "$argon2id$v=19$m=65536,t=3,p=$c2FsdA$a2V5"},
		{"passes value is negative", "$argon2id$v=19$m=65536,t=-3,p=4$c2FsdA$a2V5"},
		{"lanes value overflows uint32", "$argon2id$v=19$m=65536,t=3,p=4294967296$c2FsdA$a2V5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeHash(tc.stored)
			if err == nil {
				t.Fatal("decodeHash accepted a malformed parameter field")
			}
			// The stored hash is a credential verifier. The error may name the
			// parameter and must not print the salt or the digest.
			for _, secret := range []string{"c2FsdA", "a2V5"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("the error quotes hash material: %v", err)
				}
			}

			ok, verr := verifyPassword(tc.stored, fakePanelPass)
			if verr == nil {
				t.Error("verifyPassword accepted a malformed parameter field")
			}
			if ok {
				t.Error("verifyPassword returned true for a malformed stored hash")
			}
		})
	}
}

// TestDescribeHashOnAnUnreadableValue pins the redacted rendering for a hash
// that will not decode. It has to be a fixed word rather than the raw value,
// because this string is what the panel and the logs show.
func TestDescribeHashOnAnUnreadableValue(t *testing.T) {
	if got := describeHash("$argon2id$v=19$m=65536,t=lots,p=4$c2FsdA$a2V5"); got != "unreadable" {
		t.Errorf("describeHash of a broken hash = %q, want \"unreadable\"", got)
	}
	if got := describeHash(""); got != "unreadable" {
		t.Errorf("describeHash of an empty hash = %q, want \"unreadable\"", got)
	}
}

// --- formatTime -------------------------------------------------------------

// TestZeroTimesRenderAsNever covers the zero branch of formatTime.
//
// A zero time.Time renders as "0001-01-01T00:00:00Z", which in the panel reads
// as a real date in the year 1 rather than as "this has not happened".
func TestZeroTimesRenderAsNever(t *testing.T) {
	if got := formatTime(time.Time{}); got != "never" {
		t.Errorf("formatTime of the zero time = %q, want \"never\"", got)
	}

	when := time.Date(2026, 8, 30, 1, 2, 3, 0, time.UTC)
	if got := formatTime(when); got != "2026-08-30T01:02:03Z" {
		t.Errorf("formatTime of a real time = %q", got)
	}

	// And through the rendering the panel actually uses: a first-run state has
	// never been updated and has no config, so both timestamps are zero.
	st := defaultState()
	r := st.Redacted()
	if strings.Contains(r, "0001-01-01") {
		t.Errorf("the redacted rendering shows a zero time as a date in the year 1: %s", r)
	}
	if !strings.Contains(r, "never") {
		t.Errorf("the redacted rendering of a first-run state does not say \"never\": %s", r)
	}
}
