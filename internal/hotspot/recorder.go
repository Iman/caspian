// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EventKind names one kind of effect on the machine.
type EventKind string

const (
	EventRun    EventKind = "run"
	EventWrite  EventKind = "write"
	EventRemove EventKind = "remove"
	EventSignal EventKind = "signal"
	EventSleep  EventKind = "sleep"
)

// Event is one effect, in the order it happened.
//
// WHY A SINGLE ORDERED TRAIL AND NOT FIVE SEPARATE LISTS. The Recorder used to
// keep commands, writes, removals and signals in four lists with no shared
// ordering. Each list on its own can only answer "did this happen", never "did
// this happen first", and for this package the ordering IS the behaviour:
//
//   - a leftover process must be stopped BEFORE its replacement is started, or
//     the replacement fails because the old one still holds the radio;
//   - a stale pid file must be removed BEFORE a new process writes one, or the
//     supervisor reads a pid that belongs to something else;
//   - a configuration must be written BEFORE the process that reads it starts,
//     or the process starts on the previous run's settings.
//
// All three were already implemented correctly and none of them was guarded,
// because the double had thrown the interleaving away. This is the same defect
// class as the file mode it used to discard: the test could not have failed,
// so it proved nothing.
type Event struct {
	Kind     EventKind
	Name     string      // EventRun: the program
	Args     []string    // EventRun
	Path     string      // EventWrite, EventRemove
	Perm     fs.FileMode // EventWrite
	PID      int         // EventSignal
	Signal   Signal      // EventSignal
	Duration time.Duration
}

// String renders an event for a test failure message.
func (e Event) String() string {
	switch e.Kind {
	case EventRun:
		return "run " + Call{Name: e.Name, Args: e.Args}.String()
	case EventWrite:
		return fmt.Sprintf("write %s (%04o)", e.Path, e.Perm)
	case EventRemove:
		return "remove " + e.Path
	case EventSignal:
		return fmt.Sprintf("signal %s to %d", e.Signal, e.PID)
	case EventSleep:
		return "sleep " + e.Duration.String()
	default:
		return string(e.Kind)
	}
}

// Call is one command the Recorder was asked to run.
type Call struct {
	Name string
	Args []string
}

// String renders the call as a command line, for assertions and failure
// messages.
func (c Call) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}

// SignalRecord is one signal the Recorder was asked to send.
type SignalRecord struct {
	PID    int
	Signal Signal
}

// Recorder is an in-memory System for tests.
//
// It records every command, file write, removal and signal, and answers
// commands from Responder. The default responder emulates a machine where
// everything works: the radio is unblocked, hostapd and dnsmasq start and
// write their pid files, and no stale process is found. A test that wants a
// failure replaces or wraps it.
//
// It is in the package rather than in a _test file so that the panel and the
// engine, which drive a Supervisor, can test against the same double.
type Recorder struct {
	mu sync.Mutex

	// Calls is every Run, in order.
	Calls []Call
	// Files is the file system as this recorder sees it.
	Files map[string][]byte
	// Perms is the mode each file was written with. Recorded because the
	// mode is part of the contract: the hostapd configuration carries the
	// WPA2 passphrase, and docs/LAYOUT.md fixes a mode for both generated
	// files. A double that dropped the mode would let a permissions
	// regression pass unnoticed.
	Perms map[string]fs.FileMode
	// Removed is every path passed to Remove, in order.
	Removed []string

	// RemoveErr, when set, makes every Remove fail and leave the file alone.
	RemoveErr error
	// Signals is every signal sent, in order.
	Signals []SignalRecord
	// Alive is which pids exist.
	Alive map[int]bool
	// Slept is the total time the supervisor asked to wait for.
	Slept time.Duration
	// Sleeps is every individual wait. The total alone cannot tell a loop
	// that polls from a single long sleep, and the difference is whether the
	// supervisor notices a process exiting promptly or waits out the whole
	// grace period first.
	Sleeps []time.Duration

	// Events is every effect above, in one ordered trail. See Event.
	Events []Event

	// Responder answers a command. nil means DefaultResponder.
	Responder func(rec *Recorder, name string, args []string) (Result, error)

	// nextPID is the pid handed to the next process the default responder
	// pretends to start.
	nextPID int
}

// NewRecorder returns a Recorder on a machine where nothing has happened yet.
func NewRecorder() *Recorder {
	return &Recorder{
		Files:   map[string][]byte{},
		Perms:   map[string]fs.FileMode{},
		Alive:   map[int]bool{},
		nextPID: 1000,
	}
}

// CommandLines returns every recorded call as a command line.
func (r *Recorder) CommandLines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.Calls))
	for _, c := range r.Calls {
		out = append(out, c.String())
	}
	return out
}

// CountCalls returns how many recorded commands had this program name.
func (r *Recorder) CountCalls(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.Calls {
		if c.Name == name {
			n++
		}
	}
	return n
}

// SetFile seeds a file, for tests that start from a machine where a previous
// run left something behind.
func (r *Recorder) SetFile(path, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Files[path] = []byte(content)
	if _, ok := r.Perms[path]; !ok {
		r.Perms[path] = 0o600
	}
}

// SetAlive marks a pid as existing.
func (r *Recorder) SetAlive(pid int, alive bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Alive[pid] = alive
}

// NextPID reserves and returns the next pid the default responder will use.
func (r *Recorder) NextPID() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextPID++
	return r.nextPID
}

func (r *Recorder) Run(ctx context.Context, name string, args ...string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	r.mu.Lock()
	argsCopy := append([]string(nil), args...)
	r.Calls = append(r.Calls, Call{Name: name, Args: argsCopy})
	r.Events = append(r.Events, Event{Kind: EventRun, Name: name, Args: argsCopy})
	responder := r.Responder
	r.mu.Unlock()

	if responder == nil {
		responder = DefaultResponder
	}
	return responder(r, name, args)
}

func (r *Recorder) WriteFile(path string, data []byte, perm fs.FileMode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Files[path] = append([]byte(nil), data...)
	r.Perms[path] = perm
	r.Events = append(r.Events, Event{Kind: EventWrite, Path: path, Perm: perm})
	return nil
}

func (r *Recorder) ReadFile(path string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.Files[path]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), b...), nil
}

// RemoveErr, when set, is returned by every Remove and the file is left in
// place, which is what a read-only filesystem or a permission problem looks
// like. It exists because a stop that cannot remove the generated hotspot
// configuration has left the WPA passphrase on the machine, and a test has to
// be able to produce that.
func (r *Recorder) Remove(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.RemoveErr != nil {
		return r.RemoveErr
	}
	r.Removed = append(r.Removed, path)
	r.Events = append(r.Events, Event{Kind: EventRemove, Path: path})
	delete(r.Files, path)
	return nil
}

func (r *Recorder) ProcessAlive(pid int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Alive[pid], nil
}

func (r *Recorder) SignalProcess(pid int, sig Signal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.noteSignal(pid, sig)
	// A recorded machine behaves: TERM and KILL both stop the process.
	r.Alive[pid] = false
	return nil
}

// noteSignal records a signal in both the list and the ordered trail. Callers
// must hold r.mu. It exists so that a System that wraps this one to model a
// process ignoring SIGTERM cannot record into one and not the other.
func (r *Recorder) noteSignal(pid int, sig Signal) {
	r.Signals = append(r.Signals, SignalRecord{PID: pid, Signal: sig})
	r.Events = append(r.Events, Event{Kind: EventSignal, PID: pid, Signal: sig})
}

func (r *Recorder) Sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Slept += d
	r.Sleeps = append(r.Sleeps, d)
	r.Events = append(r.Events, Event{Kind: EventSleep, Duration: d})
	return nil
}

// EventTrail returns a copy of every effect, in order.
func (r *Recorder) EventTrail() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.Events...)
}

// FirstEvent returns the index of the first event that matches, or -1.
//
// Two indexes and a comparison is how an ordering property is asserted:
// FirstEvent(stopped the stray) < FirstEvent(started the replacement).
func (r *Recorder) FirstEvent(match func(Event) bool) int {
	for i, e := range r.EventTrail() {
		if match(e) {
			return i
		}
	}
	return -1
}

// TrailString renders the whole trail, for a test failure message that has to
// show what actually happened and in what order.
func (r *Recorder) TrailString() string {
	var b strings.Builder
	for i, e := range r.EventTrail() {
		fmt.Fprintf(&b, "%2d. %s\n", i, e)
	}
	return b.String()
}

// unblockedRfkillList is what `rfkill list` prints on a machine whose radio is
// ready. The layout is the real one, tab-indented under the device line.
const unblockedRfkillList = "0: phy0: Wireless LAN\n\tSoft blocked: no\n\tHard blocked: no\n"

// SoftBlockedRfkillList is what `rfkill list` prints when the radio is soft
// blocked, which on this platform is common enough to be the default state
// after a fresh image boots.
const SoftBlockedRfkillList = "0: phy0: Wireless LAN\n\tSoft blocked: yes\n\tHard blocked: no\n"

// HardBlockedRfkillList is what `rfkill list` prints when software cannot fix
// it.
const HardBlockedRfkillList = "0: phy0: Wireless LAN\n\tSoft blocked: no\n\tHard blocked: yes\n"

// DefaultResponder emulates a machine on which everything works.
//
// It is deliberately specific: it reads the pid file argument out of the
// command line and writes a pid file, because "did start write a usable pid
// file" is exactly the thing the supervisor depends on and a responder that
// merely returned exit 0 would let a broken supervisor pass.
func DefaultResponder(rec *Recorder, name string, args []string) (Result, error) {
	base := name
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	switch base {
	case "rfkill":
		if len(args) > 0 && args[0] == "list" {
			return Result{Stdout: unblockedRfkillList}, nil
		}
		return Result{}, nil

	case "hostapd":
		pid := rec.NextPID()
		if p, ok := flagValue(args, "-P"); ok {
			rec.SetFile(p, strconv.Itoa(pid)+"\n")
		}
		rec.SetAlive(pid, true)
		return Result{}, nil

	case "dnsmasq":
		pid := rec.NextPID()
		if p, ok := optValue(args, "--pid-file="); ok {
			rec.SetFile(p, strconv.Itoa(pid)+"\n")
		}
		rec.SetAlive(pid, true)
		return Result{}, nil

	case "hostapd_cli":
		return Result{Stdout: "state=ENABLED\nchannel=10\n"}, nil

	case "pgrep":
		// No stale process. pgrep exits 1 when nothing matches, which the
		// supervisor must not read as a failure.
		return Result{ExitCode: 1}, nil

	default:
		return Result{}, fmt.Errorf("hotspot: recorder has no answer for %q", name)
	}
}

// flagValue finds "-P /path" style arguments.
func flagValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// optValue finds "--pid-file=/path" style arguments.
func optValue(args []string, prefix string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix), true
		}
	}
	return "", false
}
