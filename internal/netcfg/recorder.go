// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
)

// RecordingRunner is the substitute for the real executor. It records every
// command in order and returns canned output, so the whole apply and teardown
// path is exercised on a machine that has no "ip" and no "nft".
//
// It is not a test-only type: it is also how a dry run is implemented.
type RecordingRunner struct {
	mu   sync.Mutex
	cmds []Command

	// Responses maps a key built by RunnerKey to the result to return. A
	// command with no entry gets an empty successful result.
	Responses map[string]Result

	// Errors maps the same key to an error to return instead of running. Use
	// it to prove that teardown continues past a failing inverse.
	Errors map[string]error

	// Fallback answers any command with no entry in Responses or Errors. It
	// exists for commands whose arguments are computed, such as the sysctl
	// read whose knob list depends on which interfaces the plan chose.
	Fallback func(Command) (Result, error)
}

// NewRecordingRunner returns an empty recorder.
func NewRecordingRunner() *RecordingRunner {
	return &RecordingRunner{
		Responses: map[string]Result{},
		Errors:    map[string]error{},
	}
}

// RunnerKey is the IDENTITY of a command: two commands share a key if and only
// if running one is the same as running the other.
//
// It includes a digest of standard input, and that is the whole point. Every
// nftables load in this package is "nft -f -", so the entire difference
// between the ordinary ruleset, the cut one and the teardown travels on stdin
// and nowhere else. A key built from the path and the argument vector alone
// makes all of them the same command.
//
// MEASURED consequence, on hardware: the client-traffic cut was applied
// through Applier.Apply after the firewall step was already in the journal.
// The applier found a completed entry with the same key, skipped the step,
// loaded zero rulesets and reported success, and the box went on forwarding.
// A button that reports done and does nothing is the same class of false green
// as a started process taken for a working access point.
//
// Use CommandLine for anything a person reads. This is for deciding whether
// two commands are the same one.
func RunnerKey(c Command) string {
	k := CommandLine(c)
	if c.Stdin == "" {
		return k
	}
	sum := sha256.Sum256([]byte(c.Stdin))
	return fmt.Sprintf("%s <stdin:%x>", k, sum[:8])
}

// CommandLine renders a command for a human and for assertions about which
// commands ran in what order. It deliberately omits standard input, so a
// sequence assertion does not churn every time a ruleset changes.
//
// It is NOT an identity: two "nft -f -" invocations carrying different
// rulesets produce the same line. Anything deciding whether two commands are
// the same must use RunnerKey.
func CommandLine(c Command) string {
	if len(c.Args) == 0 {
		return c.Path
	}
	return c.Path + " " + strings.Join(c.Args, " ")
}

// Run implements Runner.
func (r *RecordingRunner) Run(_ context.Context, c Command) (Result, error) {
	if err := ValidateCommand(c); err != nil {
		return Result{}, err
	}
	r.mu.Lock()
	r.cmds = append(r.cmds, c)
	r.mu.Unlock()

	k := RunnerKey(c)
	if err, ok := r.Errors[k]; ok && err != nil {
		// A failing command can still print. nft lists what it managed to
		// read before erroring, and ip prints usage on a bad argument. A
		// double that swallowed the output whenever it returned an error
		// could not express that, and a caller reading only stdout would
		// look correct against it while being wrong against the real thing.
		res := Result{ExitCode: 2, Stderr: err.Error()}
		if canned, ok := r.Responses[k]; ok {
			res.Stdout = canned.Stdout
			if canned.ExitCode != 0 {
				res.ExitCode = canned.ExitCode
			}
			if canned.Stderr != "" {
				res.Stderr = canned.Stderr
			}
		}
		return res, err
	}
	if res, ok := r.Responses[k]; ok {
		return res, nil
	}
	if r.Fallback != nil {
		return r.Fallback(c)
	}
	return Result{}, nil
}

// Commands returns the commands run so far, in order.
func (r *RecordingRunner) Commands() []Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Command, len(r.cmds))
	copy(out, r.cmds)
	return out
}

// Lines returns the commands so far rendered one per line, which is what
// tests assert on.
func (r *RecordingRunner) Lines() []string {
	cmds := r.Commands()
	out := make([]string, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, CommandLine(c))
	}
	return out
}

// Reset drops the recorded commands, keeping the canned responses.
func (r *RecordingRunner) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cmds = nil
}

// SetOutput registers canned stdout for a command, keyed the same way
// RunnerKey keys it.
func (r *RecordingRunner) SetOutput(key, stdout string) {
	if r.Responses == nil {
		r.Responses = map[string]Result{}
	}
	r.Responses[key] = Result{Stdout: stdout}
}

// SetFailure registers a command that fails the way the real tool fails: a
// non-zero exit code and a message on stderr, with nothing on stdout.
//
// It exists because callers such as readLinkState decide what a failure MEANS
// by reading stderr (IsNotFound), so a double that only carries a Go error
// cannot express the difference between "no such device" and any other
// refusal.
func (r *RecordingRunner) SetFailure(key string, code int, stderr string) {
	if r.Responses == nil {
		r.Responses = map[string]Result{}
	}
	r.Responses[key] = Result{ExitCode: code, Stderr: stderr}
	r.SetError(key, fmt.Errorf("exit status %d", code))
}

// SetError registers a failure for a command.
func (r *RecordingRunner) SetError(key string, err error) {
	if r.Errors == nil {
		r.Errors = map[string]error{}
	}
	r.Errors[key] = err
}

// FailingRunner is a Runner that fails every command with the same error. It
// stands in for the platforms where nothing can be applied.
type FailingRunner struct{ Err error }

// Run implements Runner.
func (f FailingRunner) Run(_ context.Context, c Command) (Result, error) {
	return Result{}, fmt.Errorf("%w (refused %s)", f.Err, c.Path)
}
