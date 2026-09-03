// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"strings"
)

// Applying a plan twice must converge, not fail.
//
// A person pressing a button twice is not an error, and a panel reporting
// failure on a working box is worse than one that quietly does nothing. But
// convergence cannot be bought by making every command succeed, because the
// journal has to stay honest: an inverse recorded for a change that was never
// made is an instruction to delete something that was there before this
// program ran.
//
// So the rule is narrower than "ignore errors". It is:
//
//	A step journals an inverse if and only if it changed something.
//
// Three mechanisms enforce it, in the order Apply tries them:
//
//  1. If the journal already holds a completed entry for the identical
//     command, the change is ours and is still in force. Skip it, and leave
//     the FIRST entry's inverse alone. This is what keeps sysctl honest: on a
//     second apply the machine now reports the value this package wrote, so a
//     freshly derived inverse would restore that instead of the original.
//  2. If the step carries a precondition and it reports the change already
//     present, skip it and journal nothing. This exists for creating the
//     access point's interface, where iw's refusal is the one error wording
//     in this package that has not been measured, so the code asks rather
//     than depending on matching it.
//  3. If the command runs and fails because the object already exists, then
//     nothing was changed, so the inverse is retracted from the journal and
//     the apply carries on rather than stopping.
//
// What none of that does is overwrite. Where a route or an address is already
// present and this package did not put it there, it is left exactly as it is
// and reported, rather than replaced. "ip route replace" would converge just
// as well and would quietly destroy a route somebody else depends on, with a
// journalled delete to finish the job on teardown.

// AlreadyApplied is a read-only query that answers whether the change a step
// makes is already in place, for the commands whose failure the kernel does
// not report.
//
// It is consulted BEFORE the journal entry is written, so a step it skips
// leaves no record at all and there is no window in which a crash could make
// teardown undo something this package never did.
type AlreadyApplied struct {
	// Query must not change anything.
	Query Command

	// Satisfied reports whether the change the step would make is already
	// present. It receives the query's error as well as its output, because
	// the clearest existence check available is a command that FAILS when the
	// object is absent, and a query that cannot answer must report "not
	// present" rather than abort the apply: skipping on uncertainty risks not
	// creating something later steps need.
	Satisfied func(Result, error) bool
}

// alreadyExistsMarkers are the strings the tools print when the object a
// command would create is already there.
//
// This is a string match on tool output, which is the weakest kind of signal
// and is used because the Runner interface carries an exit code and a message
// rather than an errno.
//
// MEASURED on the target, kernel 6.18.34, iproute2 6.15.0, 2026-08-30:
//
//	ip route add   (existing route)  RTNETLINK answers: File exists   exit 2
//	ip rule add    (identical rule,
//	                same priority)   RTNETLINK answers: File exists   exit 2
//
// OBSERVED for iw, and deliberately absent from this list, both of them:
//
//	interface add, name already taken   command failed: Invalid exchange (-52)
//	interface add, driver refuses        command failed: Input/output error (-5)
//
// The second one is why neither is here. "-5" is the brcmfmac driver refusing
// to create a second interface at all, measured on the target on 2026-08-30.
// It means the OPPOSITE of "already exists": nothing was created and nothing
// is there. Treating it as an already-exists condition would skip the creation
// step, report success, and leave the plan addressing an interface that does
// not exist. A marker list is only safe while every string in it means the
// state is already as desired, and that one does not.
//
// "-52" does mean the name is taken, and it is still left out: interface
// creation asks first with a precondition, so nothing depends on matching it.
// See ifacePresent, and Plan.HotspotTakeover for what happens on "-5".
//
// So the list is authoritative for ip and is a secondary tolerance elsewhere.
// A string that fails to match never produces a wrong inverse; it produces a
// visible hard stop, which is the behaviour this package had before.
var alreadyExistsMarkers = []string{
	"file exists",
	"object exists",
}

// IsAlreadyExists reports whether a failed command failed only because what it
// would have created is already present.
func IsAlreadyExists(res Result, err error) bool {
	if err == nil {
		return false
	}
	hay := strings.ToLower(res.Stderr + " " + err.Error())
	for _, m := range alreadyExistsMarkers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}

// ifacePresent builds the precondition for creating the access point's
// interface.
//
// It exists because iw's refusal is the one answer in this package that is not
// measured. Adding an interface whose name is taken produced "Invalid exchange
// (-52)" on an adjacent probe, not the "File exists" that IsAlreadyExists
// matches, so a second apply that reached the command could stop hard. Asking
// first removes the dependency on knowing the wording.
//
// The query is "ip -br link show dev <name>". MEASURED on the target,
// 2026-08-30, iproute2 6.15.0:
//
//	absent:   exit 1, stdout empty, STDERR `Device "nope0" does not exist.`
//	present:  exit 0, the interface line on stdout
//
// Note the absent case is NOT silent. It prints, on stderr. Any predicate that
// asked "is the output empty?" against a combined stream would read that
// message and conclude the device exists, which is the inverse of the truth.
// Result keeps the two streams in separate fields and systemRunner fills them
// from separate buffers, so the stdout check below never sees it; the status
// is what distinguishes the two cases.
func ifacePresent(name string) *AlreadyApplied {
	return &AlreadyApplied{
		Query: Command{
			Path: BinIP, Args: []string{"-br", "link", "show", "dev", name},
			Why: "iw's refusal when the interface name is taken is not a wording this package has measured, so it asks instead of relying on the error",
		},
		Satisfied: func(res Result, err error) bool {
			// Two clauses, load-bearing in opposite directions. Simplifying
			// away either one has been tried and breaks a test.
			//
			// The status clause is the answer to the real kernel: an absent
			// device is a non-zero exit. It is FIRST so that a runner which
			// merged stderr into stdout could never reach the output check
			// holding "Device ... does not exist." and read it as presence.
			//
			// The stdout clause is not a second opinion about absence. It is
			// an evidence requirement against a runner that reports success
			// without answering: RecordingRunner returns an empty successful
			// Result for any command it has no response registered for, and a
			// status-only predicate would read that as "the interface is
			// already there" and skip creating it. No evidence is treated as
			// "not present", which is the safe direction: the step then runs
			// and either succeeds or fails visibly.
			return err == nil && strings.TrimSpace(res.Stdout) != ""
		},
	}
}

// notFoundMarkers are the strings the tools print when the object a command
// would REMOVE is already gone.
//
// They matter on the undo path only, and they are the mirror of
// alreadyExistsMarkers: if the thing is not there, the inverse has already
// achieved what it wanted, so the entry is done rather than pending.
//
// Without this, an inverse for a step that never took effect is retried on
// every start, for ever. Reproduced in this package: a journal entry left at
// "begin" for an interface creation that failed leaves "iw dev ap0 del",
// which fails because ap0 was never created, so the replay records it as
// outstanding, RewriteJournal keeps it, and the next start does the same. The
// journal never empties and every start reports a failure nothing can fix.
//
// INFERRED, not measured, with ONE exception. These are the errno wordings
// iproute2 and iw use for "no such object". The first, "no such device", IS
// measured: "iw dev nosuchdev link" on the target answered "command failed: No
// such device (-19)" on stderr with a non-zero exit, which is
// capture-pi5-iw-link-nosuchdev-stderr.txt and is what makes a missing hotspot
// interface a named refusal rather than "free". The rest were not captured. The two failure
// directions are not symmetric, which is what makes that acceptable:
//
//   - A string that does not match degrades to the previous behaviour, an
//     entry retried on the next start. Safe, and merely noisy.
//   - A string that matches wrongly marks an entry undone while its object
//     still exists, leaving that object behind. Every entry below is an errno
//     that can only mean absence, which is what bounds that risk.
var notFoundMarkers = []string{
	"no such device",                  // iw -19, and ip for a missing interface
	"no such file or directory",       // ip rule del with no match
	"no such process",                 // ip route del with no match
	"cannot find device",              // ip, named device absent
	"cannot assign requested address", // ip addr del, address not on the device
	"not in table",                    // route(8) on macOS: delete of a route that is not there
}

// commandRemoves reports whether a command's job is to take something away.
//
// It exists because IsNotFound is direction-blind and "no such object" means
// opposite things in the two directions. For a removal it means the goal is
// already achieved. For an addition it means the thing being added TO is
// missing, which is a real failure: "ip address add 10.0.0.1/24 dev nope0"
// answers "Cannot find device", and tolerating that would report success for
// an address nobody holds.
//
// Every command this package generates is written here, so the check is a
// closed set rather than a guess about a general command line.
func commandRemoves(c Command) bool {
	for _, a := range c.Args {
		switch a {
		case "del", "delete":
			return true
		case "add", "replace", "set":
			// The verb comes before its object, so the first of these decides.
			return false
		}
	}
	return false
}

// IsNotFound reports whether a failed command failed only because what it
// would have removed is already gone.
//
// Callers must check commandRemoves first. This answers "the object is not
// there", and only a removal is entitled to read that as success.
func IsNotFound(res Result, err error) bool {
	if err == nil {
		return false
	}
	hay := strings.ToLower(res.Stderr + " " + err.Error())
	for _, m := range notFoundMarkers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}
