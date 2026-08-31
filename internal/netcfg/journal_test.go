// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmpJournal(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "netcfg.journal")
}

func step(op, arg string) Step {
	return Step{
		Op:   op,
		Why:  "test step " + arg,
		Do:   Command{Path: BinIP, Args: []string{op, "add", arg}},
		Undo: Command{Path: BinIP, Args: []string{op, "del", arg}},
	}
}

func TestJournal_RecordsInverseBeforeTheChange(t *testing.T) {
	path := tmpJournal(t)
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := j.Begin(step(OpRoute, "a"))
	if err != nil {
		t.Fatal(err)
	}

	// The record is on disk before the change is made, which is what lets a
	// process killed mid-apply be undone by the next start.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"phase":"begin"`) {
		t.Fatalf("begin record not flushed before the change:\n%s", data)
	}
	if !strings.Contains(string(data), `"undo"`) {
		t.Fatalf("begin record does not carry the inverse:\n%s", data)
	}

	if err := j.Done(seq); err != nil {
		t.Fatal(err)
	}
	entries, err := j.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Phase != PhaseDone {
		t.Fatalf("entries = %+v", entries)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
}

// A step that began and never completed still needs its inverse: a command
// killed halfway can have landed part of its effect.
func TestJournal_PendingEntryStillNeedsUndo(t *testing.T) {
	path := tmpJournal(t)
	j, _ := OpenJournal(path)
	if _, err := j.Begin(step(OpRoute, "a")); err != nil {
		t.Fatal(err)
	}
	j.Close()

	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Phase != PhaseBegin || !entries[0].NeedsUndo() {
		t.Errorf("a pending entry must still be undone: %+v", entries[0])
	}
}

// A process killed mid-write truncates the last line. Every complete record
// before it must still be readable, because refusing the file would throw away
// everything that has to be undone.
func TestLoadJournal_ToleratesATruncatedTail(t *testing.T) {
	path := tmpJournal(t)
	j, _ := OpenJournal(path)
	s1, _ := j.Begin(step(OpRoute, "a"))
	j.Done(s1)
	j.Begin(step(OpRule, "b"))
	j.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Chop the file mid-record, as a kill during a write would.
	if err := os.WriteFile(path, data[:len(data)-25], 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatalf("a truncated journal must still load: %v", err)
	}
	if len(entries) < 1 {
		t.Fatalf("entries = %+v, want at least the first complete record", entries)
	}
	if entries[0].Op != OpRoute {
		t.Errorf("first entry = %+v", entries[0])
	}
}

func TestLoadJournal_MissingFileIsNotAnError(t *testing.T) {
	entries, err := LoadJournal(filepath.Join(t.TempDir(), "nothing-here"))
	if err != nil {
		t.Fatalf("a missing journal means nothing to undo, not an error: %v", err)
	}
	if entries != nil {
		t.Errorf("entries = %+v", entries)
	}
}

// A second run must not reuse sequence numbers from a journal it inherited, or
// two different steps collide on one number and one inverse is lost.
func TestOpenJournal_ContinuesTheSequence(t *testing.T) {
	path := tmpJournal(t)
	j, _ := OpenJournal(path)
	j.Begin(step(OpRoute, "a"))
	j.Begin(step(OpRoute, "b"))
	j.Close()

	j2, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := j2.Begin(step(OpRoute, "c"))
	if err != nil {
		t.Fatal(err)
	}
	if seq != 3 {
		t.Errorf("second run started at seq %d, want 3", seq)
	}
	j2.Close()

	entries, _ := LoadJournal(path)
	if len(entries) != 3 {
		t.Fatalf("entries = %+v, want 3 distinct steps", entries)
	}
}

func TestRewriteJournal_EmptyRemovesTheFile(t *testing.T) {
	path := tmpJournal(t)
	j, _ := OpenJournal(path)
	j.Begin(step(OpRoute, "a"))
	j.Close()

	if err := RewriteJournal(path, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("journal should be gone, stat err = %v", err)
	}
}

func TestRewriteJournal_KeepsWhatIsLeft(t *testing.T) {
	path := tmpJournal(t)
	entries := []Entry{
		{Seq: 4, Op: OpRoute, Why: "kept", Do: Command{Path: BinIP, Args: []string{"route", "add", "x"}}, Undo: Command{Path: BinIP, Args: []string{"route", "del", "x"}}},
	}
	if err := RewriteJournal(path, entries); err != nil {
		t.Fatal(err)
	}
	got, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Seq != 4 || !got[0].NeedsUndo() {
		t.Fatalf("got %+v", got)
	}
	if RunnerKey(got[0].Undo) != "ip route del x" {
		t.Errorf("inverse survived as %q", RunnerKey(got[0].Undo))
	}
}

func TestJournal_FileIsNotWorldReadable(t *testing.T) {
	path := tmpJournal(t)
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	j.Begin(step(OpRoute, "a"))
	j.Close()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		t.Errorf("journal mode = %v, want no group or other access", fi.Mode().Perm())
	}
}
