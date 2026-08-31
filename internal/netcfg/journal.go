// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultJournalPath is where the record of applied changes lives on the
// appliance. It is under /var/lib rather than /run on purpose: /run is cleared
// at boot, and a journal that does not survive a reboot cannot undo a change
// that did.
const DefaultJournalPath = "/var/lib/caspian/netcfg.journal"

// Phase is which half of a step a journal line records.
type Phase string

const (
	// PhaseBegin is written, and flushed, BEFORE the change is made. That
	// ordering is the whole point: a process killed while the change is in
	// flight leaves a begin with no done, and the next start still knows the
	// inverse to run.
	PhaseBegin Phase = "begin"
	// PhaseDone is written after the change succeeded.
	PhaseDone Phase = "done"
	// PhaseFailed is written when the change itself failed. The inverse is
	// still replayed on teardown, because a failed command can have landed
	// part of its effect.
	PhaseFailed Phase = "failed"
	// PhaseUndone is written when the inverse has been replayed successfully,
	// so that a second teardown does not try again.
	PhaseUndone Phase = "undone"
	// PhasePreexisting is written when the command ran and CHANGED NOTHING,
	// in either direction: what it would have created was already there, or
	// what it would have removed was already gone. The inverse recorded by
	// the Begin record must NOT be replayed. For a create, replaying it would
	// delete an address, route or rule that existed before this program ran.
	// For a removal, replaying it would restore something this program never
	// took away, and whatever did take it away owns putting it back.
	//
	// The wire value stays "preexisting" although the meaning is now broader,
	// because journals written by an earlier build are on disk and must still
	// read correctly.
	PhasePreexisting Phase = "preexisting"
)

// Record is one line of the journal. The file is JSON lines rather than one
// JSON document so that a process killed mid-write truncates one record
// instead of destroying the file.
type Record struct {
	Seq   int       `json:"seq"`
	Phase Phase     `json:"phase"`
	Time  time.Time `json:"t"`
	Op    string    `json:"op,omitempty"`
	Why   string    `json:"why,omitempty"`
	Do    Command   `json:"do,omitempty"`
	Undo  Command   `json:"undo,omitempty"`
	Err   string    `json:"err,omitempty"`
}

// Entry is a step as reconstructed from the journal.
type Entry struct {
	Seq   int
	Op    string
	Why   string
	Do    Command
	Undo  Command
	Phase Phase
}

// NeedsUndo reports whether the inverse of this entry should still be
// replayed. A step that began and never completed needs it as much as one that
// completed: a command killed halfway can have landed part of its effect, and
// the inverses in this package are written to succeed when there is nothing to
// undo.
func (e Entry) NeedsUndo() bool {
	switch e.Phase {
	case PhaseUndone:
		// Already reversed.
		return false
	case PhasePreexisting:
		// Ran, changed nothing. Undoing it would remove something this
		// program did not add.
		return false
	}
	return !e.Undo.IsZero()
}

// Journal is the on-disk record of what has been applied.
type Journal struct {
	path string
	f    *os.File
	seq  int
}

// OpenJournal opens or creates the journal at path, appending to whatever is
// already there. An existing journal is not truncated: it belongs to a
// previous run that may not have been undone, and Recover is what deals with
// it.
func OpenJournal(path string) (*Journal, error) {
	if path == "" {
		return nil, errors.New("netcfg: journal path is empty")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("netcfg: create journal directory: %w", err)
		}
	}
	existing, err := LoadJournal(path)
	if err != nil {
		return nil, err
	}
	next := 0
	for _, e := range existing {
		if e.Seq > next {
			next = e.Seq
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("netcfg: open journal: %w", err)
	}
	return &Journal{path: path, f: f, seq: next}, nil
}

// Path returns the journal's location.
func (j *Journal) Path() string { return j.path }

// write appends one record and flushes it to the file system. Sync costs a
// round trip per step and there are a few dozen steps in a whole apply, which
// is a price worth paying for a record that survives a power cut and not only
// a kill.
func (j *Journal) write(r Record) error {
	r.Time = time.Now().UTC()
	line, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("netcfg: encode journal record: %w", err)
	}
	if _, err := j.f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("netcfg: write journal record: %w", err)
	}
	return j.f.Sync()
}

// Begin records a step and its inverse before the step is run, and returns the
// sequence number to pass to Done or Failed.
func (j *Journal) Begin(s Step) (int, error) {
	j.seq++
	err := j.write(Record{Seq: j.seq, Phase: PhaseBegin, Op: s.Op, Why: s.Why, Do: s.Do, Undo: s.Undo})
	return j.seq, err
}

// Done records that a step succeeded.
func (j *Journal) Done(seq int) error {
	return j.write(Record{Seq: seq, Phase: PhaseDone})
}

// Failed records that a step failed. The inverse stays live.
func (j *Journal) Failed(seq int, cause error) error {
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	return j.write(Record{Seq: seq, Phase: PhaseFailed, Err: msg})
}

// Preexisting records that a step ran and changed nothing, because what it
// would have created was already present. It retracts the inverse written by
// Begin.
func (j *Journal) Preexisting(seq int, cause error) error {
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	return j.write(Record{Seq: seq, Phase: PhasePreexisting, Err: msg})
}

// Undone records that a step's inverse has been replayed.
func (j *Journal) Undone(seq int) error {
	return j.write(Record{Seq: seq, Phase: PhaseUndone})
}

// Entries reconstructs the journal in apply order.
func (j *Journal) Entries() ([]Entry, error) { return LoadJournal(j.path) }

// Close closes the underlying file.
func (j *Journal) Close() error {
	if j.f == nil {
		return nil
	}
	err := j.f.Close()
	j.f = nil
	return err
}

// Discard closes the journal and removes the file. It is called only once
// every inverse has been replayed successfully; anything still outstanding is
// kept by Rewrite instead.
func (j *Journal) Discard() error {
	if err := j.Close(); err != nil {
		return err
	}
	if err := os.Remove(j.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("netcfg: remove journal: %w", err)
	}
	return nil
}

// LoadJournal reads a journal from disk and reconstructs its entries in apply
// order. A missing file is not an error: it means there is nothing to undo.
//
// A truncated final line is dropped rather than failing the load. That case is
// exactly the one the journal exists for, a process killed mid-write, and
// refusing to read the file would throw away every complete record before it.
func LoadJournal(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("netcfg: open journal: %w", err)
	}
	defer f.Close()

	bySeq := map[int]*Entry{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			// Truncated or corrupt tail. Everything before it is still good.
			continue
		}
		e, ok := bySeq[r.Seq]
		if !ok {
			e = &Entry{Seq: r.Seq}
			bySeq[r.Seq] = e
		}
		if r.Phase == PhaseBegin {
			e.Op, e.Why, e.Do, e.Undo = r.Op, r.Why, r.Do, r.Undo
		}
		e.Phase = r.Phase
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("netcfg: read journal: %w", err)
	}

	out := make([]Entry, 0, len(bySeq))
	for _, e := range bySeq {
		// A record whose begin line was lost has no inverse to replay and
		// nothing useful to say, so it is dropped rather than reported.
		if e.Do.IsZero() && e.Undo.IsZero() {
			continue
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Seq < out[k].Seq })
	return out, nil
}

// RewriteJournal replaces the journal with the given entries, which is how a
// teardown that could not undo everything leaves the rest for the next start
// to retry. The write goes to a temporary file in the same directory and is
// renamed over the original, so a crash during the rewrite leaves either the
// old journal or the new one and never a half of each.
func RewriteJournal(path string, entries []Entry) error {
	if len(entries) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("netcfg: remove journal: %w", err)
		}
		return nil
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".netcfg.journal.*")
	if err != nil {
		return fmt.Errorf("netcfg: create replacement journal: %w", err)
	}
	defer os.Remove(tmp.Name())

	for _, e := range entries {
		rec := Record{Seq: e.Seq, Phase: PhaseBegin, Op: e.Op, Why: e.Why, Do: e.Do, Undo: e.Undo, Time: time.Now().UTC()}
		line, err := json.Marshal(rec)
		if err != nil {
			tmp.Close()
			return fmt.Errorf("netcfg: encode journal record: %w", err)
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			tmp.Close()
			return fmt.Errorf("netcfg: write replacement journal: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("netcfg: sync replacement journal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("netcfg: close replacement journal: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return fmt.Errorf("netcfg: chmod replacement journal: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("netcfg: replace journal: %w", err)
	}
	return nil
}
