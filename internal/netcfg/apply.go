// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// StepResult is what happened to one step.
type StepResult struct {
	Step   Step
	Err    error
	Stderr string

	// Skipped reports that the step made no change because the change was
	// already in place. Reason says which of the three ways it was found to
	// be. A skipped step journals nothing, so teardown will not undo it.
	Skipped bool
	Reason  string
}

// Report is the outcome of an apply or a teardown.
type Report struct {
	Results []StepResult
	// Failed counts results that carry an error.
	Failed int
	// Skipped counts steps that made no change because the change was
	// already in place.
	Skipped int
}

func (r *Report) add(res StepResult) {
	r.Results = append(r.Results, res)
	if res.Err != nil {
		r.Failed++
	}
	if res.Skipped {
		r.Skipped++
	}
}

// Err returns a single error summarising the failures, or nil.
func (r Report) Err() error {
	if r.Failed == 0 {
		return nil
	}
	var msgs []string
	for _, res := range r.Results {
		if res.Err != nil {
			msgs = append(msgs, fmt.Sprintf("%s: %v", res.Step.Do, res.Err))
		}
	}
	return fmt.Errorf("netcfg: %d of %d steps failed: %s", r.Failed, len(r.Results), strings.Join(msgs, "; "))
}

// FailedStep returns the step that failed, if any. Apply stops at the first
// failure, so there is at most one.
func (r Report) FailedStep() (Step, bool) {
	for _, res := range r.Results {
		if res.Err != nil {
			return res.Step, true
		}
	}
	return Step{}, false
}

// Lines renders the report one step per line, for a log.
func (r Report) Lines() []string {
	out := make([]string, 0, len(r.Results))
	for _, res := range r.Results {
		status := "ok"
		switch {
		case res.Err != nil:
			status = "FAILED: " + res.Err.Error()
		case res.Skipped:
			status = "skipped: " + res.Reason
		}
		out = append(out, fmt.Sprintf("%s %s [%s]", res.Step.Op, res.Step.Do, status))
	}
	return out
}

// Applier runs steps and records how to undo them.
//
// The contract is that the inverse reaches the disk before the change reaches
// the kernel. Everything else about crash recovery follows from that one
// ordering: whatever state the machine is left in, the journal names at least
// as much as was actually changed, and replaying inverses that were never
// needed is harmless because every inverse in this package succeeds when there
// is nothing to undo.
type Applier struct {
	r Runner
	j *Journal
}

// NewApplier opens the journal at path and returns an applier writing to it.
func NewApplier(r Runner, path string) (*Applier, error) {
	if r == nil {
		return nil, errors.New("netcfg: applier needs a runner")
	}
	j, err := OpenJournal(path)
	if err != nil {
		return nil, err
	}
	return &Applier{r: r, j: j}, nil
}

// Journal exposes the underlying journal, mostly so a caller can report its
// path in a log line.
func (a *Applier) Journal() *Journal { return a.j }

// Close closes the journal without undoing anything.
func (a *Applier) Close() error { return a.j.Close() }

// Apply runs steps in order and stops at the first failure.
//
// It stops rather than continuing because the steps are ordered by dependency:
// a failed address makes every route that uses it fail too, and a run of
// twenty errors tells a reader less than the first one does. The journal
// already holds the inverse of everything attempted, including the step that
// failed, so the caller's response to an error is Teardown.
func (a *Applier) Apply(ctx context.Context, steps []Step) (Report, error) {
	// What a previous apply, still in force, already did. Matching by the
	// exact command is deliberate: it means "this identical change is ours
	// and is recorded", which is the only claim the journal can support.
	state, err := a.currentState()
	if err != nil {
		return Report{}, err
	}

	var rep Report
	for _, s := range steps {
		if s.Do.IsZero() {
			continue
		}

		// 1. Ours already, from an apply that has not been torn down. Leave
		//    the original entry alone: its inverse was derived when the
		//    machine still held the value this package later overwrote, and
		//    that is the only honest one.
		if state[CommandLine(s.Do)] == RunnerKey(s.Do) {
			rep.add(StepResult{Step: s, Skipped: true,
				Reason: "this exact change is the one already in force, recorded in the journal"})
			continue
		}

		// 2. Present on the machine, and not ours. Nothing is written to the
		//    journal, so teardown cannot remove it.
		if s.Pre != nil {
			// The query's error is handed to Satisfied rather than aborting:
			// the clearest existence checks fail when the object is absent,
			// and refusing to proceed because a check said "no" would be the
			// opposite of the intended answer.
			res, qerr := a.r.Run(ctx, s.Pre.Query)
			if s.Pre.Satisfied(res, qerr) {
				rep.add(StepResult{Step: s, Skipped: true,
					Reason: "already present on this machine and not recorded as ours, so it is left alone and no inverse is journalled"})
				continue
			}
		}

		seq, err := a.j.Begin(s)
		if err != nil {
			return rep, err
		}
		res, runErr := a.r.Run(ctx, s.Do)

		// 3. It ran and changed nothing, because the object was already
		//    there. Retract the inverse Begin wrote, and carry on: a second
		//    connect is not a failure.
		// A REMOVAL whose object is already gone has achieved what it wanted.
		// This is the same rule the undo path has always had, and its absence
		// here was measured breaking the appliance on hardware: the release
		// sequence deletes the addresses the interface was carrying, and
		// releasing an interface from NetworkManager can take NM's own
		// address with it. The delete then failed, Apply stopped at it, and
		// the interface was left released but never typed. The readback that
		// runs next then refused it, and the message named the SSID rather
		// than the step that had not run.
		if runErr != nil && commandRemoves(s.Do) && IsNotFound(res, runErr) {
			if jerr := a.j.Preexisting(seq, runErr); jerr != nil {
				return rep, jerr
			}
			rep.add(StepResult{Step: s, Stderr: res.Stderr, Skipped: true,
				Reason: "already gone, so nothing was changed and the inverse was retracted"})
			continue
		}

		if runErr != nil && IsAlreadyExists(res, runErr) {
			if jerr := a.j.Preexisting(seq, runErr); jerr != nil {
				return rep, jerr
			}
			rep.add(StepResult{Step: s, Stderr: res.Stderr, Skipped: true,
				Reason: "already existed, so nothing was changed and the inverse was retracted"})
			continue
		}

		if runErr != nil {
			rep.add(StepResult{Step: s, Err: runErr, Stderr: res.Stderr})
			if jerr := a.j.Failed(seq, runErr); jerr != nil {
				return rep, jerr
			}
			return rep, fmt.Errorf("netcfg: step %q failed (%s): %w", s.Op, s.Do, runErr)
		}

		rep.add(StepResult{Step: s, Stderr: res.Stderr})
		if err := a.j.Done(seq); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// currentState returns, for each resource this journal has touched, the
// identity of the LAST command that touched it and completed.
//
// The map is keyed by CommandLine and valued by RunnerKey, and that shape is
// the whole point. The nftables table is one mutable resource that several
// different commands write: the firewall load, the cut, the restore and the
// teardown are all "nft -f -" and differ only on standard input. Asking "was
// this exact command ever applied" is the wrong question for a resource like
// that, because loading the ordinary ruleset, then the cut, then the ordinary
// one again is three operations and the middle one invalidates the record of
// the first.
//
// MEASURED consequence of asking the wrong question: the cut was skipped as
// already applied and the box went on forwarding. Fixing only the key would
// have left the restore skipped for the same reason one step later, because
// the ordinary ruleset genuinely had been applied before the cut.
//
// For every command that is not a state transition on a shared resource the
// two keys coincide, so this reduces to the previous behaviour: applying the
// same plan twice still converges.
func (a *Applier) currentState() (map[string]string, error) {
	entries, err := a.j.Entries()
	if err != nil {
		return nil, err
	}
	// Entries come back in apply order, so a later one overwrites an earlier
	// one for the same resource.
	out := map[string]string{}
	for _, e := range entries {
		if e.Do.IsZero() {
			continue
		}
		switch e.Phase {
		case PhaseDone:
			// Only PhaseDone is evidence the change landed. A Begin with no
			// result means a process died mid-command and the state is
			// unknown, so the step is re-run.
			out[CommandLine(e.Do)] = RunnerKey(e.Do)
		case PhaseUndone:
			// Reversed, so it no longer describes the resource.
			delete(out, CommandLine(e.Do))
		}
	}
	return out, nil
}

// Teardown replays the inverse of every journalled step, newest first, and
// keeps going past failures.
//
// Continuing past a failure is the point. A single inverse can fail for
// reasons that say nothing about the rest: the route was already gone, the
// interface has disappeared, the address was never added because the step
// before it failed. Stopping there would leave every earlier change in place,
// which is the opposite of what a teardown is for.
//
// Anything that could not be undone is written back to the journal so the next
// start retries it. Only a teardown that undid everything removes the file.
func (a *Applier) Teardown(ctx context.Context) (Report, error) {
	entries, err := a.j.Entries()
	if err != nil {
		return Report{}, err
	}
	rep, remaining := replay(ctx, a.r, entries, a.j)
	if err := a.j.Close(); err != nil {
		return rep, err
	}
	if err := RewriteJournal(a.j.path, remaining); err != nil {
		return rep, err
	}
	return rep, nil
}

// Recover replays a journal left behind by a process that was killed, which is
// the case the on-disk record exists for. It is safe to call at every start:
// with no journal it does nothing and reports nothing.
//
// It is deliberately a package function rather than a method, because the
// caller has no Applier yet at the point it needs this: recovery happens
// before the new plan is applied, not after.
func Recover(ctx context.Context, r Runner, path string) (Report, error) {
	entries, err := LoadJournal(path)
	if err != nil {
		return Report{}, err
	}
	if len(entries) == 0 {
		return Report{}, nil
	}
	rep, remaining := replay(ctx, r, entries, nil)
	if err := RewriteJournal(path, remaining); err != nil {
		return rep, err
	}
	return rep, nil
}

// replay runs the inverses newest first and returns what could not be undone.
// j may be nil, in which case progress is recorded only in the returned
// remainder, which the caller writes back.
//
// # The firewall goes last, and only on a clean sweep
//
// Two passes, not one, and the second pass is the fail-closed guarantee.
//
// The ruleset was applied first, so replaying newest first already reached its
// inverse last. That made the ordering TRUE BY ACCIDENT, in two ways that both
// broke. It only held while the ruleset was the oldest entry, which the
// hotspot fallback breaks by opening a second journal over leftovers from the
// first; and it ran the inverse whatever had happened to the entries before
// it, which is the part that leaks.
//
// MEASURED on 2026-08-30 by the audit, with the daemons unkillable and every
// "ip" and "sysctl" inverse refusing: the table was deleted, correctly last,
// and the end state was an access point still beaconing, ip_forward still 1,
// the hotspot address still on the interface, the policy rule and the tunnel
// routes still installed, and NO generated table. Those client packets fall
// through the emptied tunnel table to main and out of the uplink in the clear.
// That is the only path in this design known to produce a leak.
//
// So the second pass runs only when the first undid everything. Anything left
// stays in the journal, the block stays in force, and the next start retries
// both. It converges: whatever failed is replayed again, and the moment that
// sweep is clean the table comes out.
//
// TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed is the guard.
func replay(ctx context.Context, r Runner, entries []Entry, j *Journal) (Report, []Entry) {
	var rep Report
	var remaining []Entry

	// Pass one: everything that is not the firewall.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.NeedsUndo() || e.Op == OpNft {
			continue
		}
		undoOne(ctx, r, e, j, &rep, &remaining)
	}
	// Pass two: the firewall, only if pass one left nothing behind.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.NeedsUndo() || e.Op != OpNft {
			continue
		}
		if rep.Failed > 0 {
			// Held, not failed: it was never attempted. The distinction
			// matters to whoever reads the report, because "the block is
			// still in force on purpose" and "the block would not come out"
			// call for opposite responses.
			rep.Results = append(rep.Results, StepResult{
				Step:    Step{Op: e.Op, Why: e.Why, Do: e.Undo, Undo: e.Do},
				Skipped: true,
				Reason: "held: an earlier inverse failed, so the fail-closed ruleset stays " +
					"loaded rather than being taken away from a machine that is still " +
					"carrying routes, addresses or forwarding. It stays in the journal and " +
					"the next start retries it",
			})
			rep.Skipped++
			remaining = append(remaining, e)
			continue
		}
		undoOne(ctx, r, e, j, &rep, &remaining)
	}

	// Keep what is left in apply order so a later retry runs it newest first
	// again.
	for i, k := 0, len(remaining)-1; i < k; i, k = i+1, k-1 {
		remaining[i], remaining[k] = remaining[k], remaining[i]
	}
	return rep, remaining
}

// undoOne replays a single inverse and records what happened. It is shared by
// both passes of replay so that the firewall's inverse is handled by exactly
// the same code as every other one; the only thing the second pass changes is
// WHEN it is allowed to run.
func undoOne(ctx context.Context, r Runner, e Entry, j *Journal, rep *Report, remaining *[]Entry) {
	step := Step{Op: e.Op, Why: e.Why, Do: e.Undo, Undo: e.Do}
	res, err := r.Run(ctx, e.Undo)

	// An inverse whose object is already gone has achieved what it wanted.
	// This is the mirror of the EEXIST rule on the apply side, and without it an
	// entry for a step that never took effect is retried on every start for
	// ever: the journal never empties and every start reports a failure nothing
	// can fix.
	//
	// commandRemoves as well as IsNotFound, because an inverse is not always a
	// removal. Restoring a station address is "ip address add", and "Cannot
	// find device" there means the interface is gone and the address was never
	// put back. Reading that as nothing-to-undo drops the entry and loses the
	// restoration silently, which is the opposite of what a teardown promises.
	if err != nil && commandRemoves(e.Undo) && IsNotFound(res, err) {
		rep.Results = append(rep.Results, StepResult{Step: step, Stderr: res.Stderr, Skipped: true,
			Reason: "nothing to undo: the object was already gone"})
		rep.Skipped++
		if j != nil {
			_ = j.Undone(e.Seq)
		}
		return
	}

	rep.Results = append(rep.Results, StepResult{Step: step, Err: err, Stderr: res.Stderr})
	if err != nil {
		rep.Failed++
		*remaining = append(*remaining, e)
		return
	}
	if j != nil {
		// A failure to journal the undo is not worth aborting a teardown for:
		// the change is already reversed, and the worst case of a missing record
		// is that the next start reverses it again, which every inverse here
		// tolerates.
		_ = j.Undone(e.Seq)
	}
}
