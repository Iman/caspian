// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// Cutting client traffic without switching the appliance off.
//
// # Why it exists
//
// Switching the appliance off takes the hotspot down with it, and that
// disconnects every joined device including the phone the user is holding to
// press the button. So the one control they most need in a hurry is the one
// that destroys its own way back. Cutting keeps the hotspot, DHCP, DNS and the
// panel up, so the device stays joined and the page stays reachable, and drops
// only forwarded client traffic.
//
// # What this package decides, and what it does not
//
// internal/netcfg owns the ruleset. RulesetFor takes a ForwardState, the two
// states differ by exactly the forward accepts plus an explicit drop rule so
// an operator reading "nft list ruleset" sees a reason rather than an absence,
// and CutStep and RestoreStep load them. This package owns WHEN, which is the
// state machine below.
//
// # It is runtime state and it is never written down
//
// A cut dies on restart, and that is a feature, not an omission: somebody who
// cannot work out why their internet has stopped gets it back by pulling the
// plug. netcfg regenerates and reloads the ruleset on every start, so the
// property holds as long as this package stores the flag nowhere but memory.
// It is a field on Service and it reaches no file. docs/LAYOUT.md already puts
// state.json under the panel process, and the privileged side "reads no state
// file"; this adds nothing to that. TestACutIsNeverWrittenDown walks every
// file this service can write and TestACutDoesNotSurviveARestart proves the
// property that matters through a second Service over the same directory.
//
// # Where it is reset, and why one place is enough
//
// stopLocked, and nowhere else. An earlier draft also cleared it where a start
// publishes its state, which reads as belt and braces and is dead code: the
// flag is only ever set while running is true, and every path from there back
// to a fresh apply passes through stopLocked. Start reassert when the
// fingerprint matches, stops first when it does not, and rollbackLocked stops
// too. A mutation removing the second assignment changed no test, which is how
// it was found; removing it means the one place that clears the flag is the
// one place that removes the ruleset carrying it.

// Cut drops forwarded client traffic, leaving the hotspot, DHCP, DNS and the
// panel working.
//
// It is idempotent: cutting an already cut box succeeds and loads nothing,
// because two browser tabs pressing the same button must not turn the second
// press into an error.
func (s *Service) Cut(ctx context.Context) error {
	return s.setForward(ctx, netcfg.ForwardCut)
}

// Restore puts forwarding back.
func (s *Service) Restore(ctx context.Context) error {
	return s.setForward(ctx, netcfg.ForwardNormal)
}

// ClientTrafficCut reports whether forwarded client traffic is being dropped
// at the user's request. It is what the panel's control shows its state from.
func (s *Service) ClientTrafficCut() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.forward == netcfg.ForwardCut
}

// errNotRunning is the refusal for cutting or restoring a box that is off.
//
// # Why this is a refusal and not a no-op
//
// The state a cut would have to produce on a box that is not running does not
// exist to be produced. There is no plan, so there is no ruleset to build one
// from; this service would have to detect and plan purely to load a table
// naming a hotspot interface that has not been created and a tunnel device
// that does not exist. That table would then be a change to a machine whose
// whole invariant while off is that it was left as it was found, and something
// would have to tear it down.
//
// And it would achieve less than the state it replaced. A box that is off
// forwards nothing already: no hotspot, no routes, no table. Cut is strictly
// weaker than off, so cutting an off box is more work for a smaller result.
//
// The other shape, accepting the request and REMEMBERING it for the next
// start, is worse still: that is a cut surviving an off and on cycle, which is
// the persistence this design exists to avoid. Accepting it and forgetting it
// is a control that reports success and does nothing.
//
// So: refuse, and say so. The panel can keep the control out of reach while
// the box is off, and this is what answers the stale tab and the scripted
// client that get there anyway.
var errNotRunning = errors.New("privsvc: the appliance is not running, so there is no client traffic to cut")

func (s *Service) setForward(ctx context.Context, want netcfg.ForwardState) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.RLock()
	plan, running, have := s.plan, s.running, s.forward
	s.mu.RUnlock()

	if !running || plan == nil {
		return fail("cut", panel.FaultNotRunning, errNotRunning)
	}
	if have == want {
		return nil
	}

	step := plan.RestoreStep()
	if want == netcfg.ForwardCut {
		step = plan.CutStep()
	}

	// Run through the Runner rather than the Applier, and this is not a
	// preference. MEASURED on 2026-08-30 by writing it the other way: routing
	// a cut through Applier.Apply loads NOTHING AT ALL and reports success.
	//
	// Applier.Apply skips a step whose command it finds already recorded in
	// the journal, and it matches by netcfg.RunnerKey, which is the path plus
	// the argument vector. Every nft load in this project is "nft -f -" with
	// the ruleset on STDIN, and RunnerKey does not include stdin. So the cut
	// step's key is identical to the firewall step's key from the start, the
	// applier answers "already applied by this package and recorded in the
	// journal", and the box carries on forwarding while the panel says the
	// traffic is cut. That is the exact class of false green this appliance
	// keeps being bitten by, one layer down.
	//
	// internal/netcfg's own comment on CutStep says "a caller that journals a
	// cut is still correct". That is true of the INVERSE, which is what it is
	// reasoning about: both steps carry the same undo as the firewall step, so
	// teardown is right either way. It is not true of the forward direction,
	// and this is the note that says so. The guard is not this note:
	// TestCuttingClientTrafficLeavesTheWayBack requires a cut to load exactly
	// one ruleset, and that is the assertion that caught it.
	//
	// There is a second reason that would stand on its own: the inverse is
	// already in the journal from the start, so journalling a toggle appends
	// entries whose undo duplicates one already recorded, and twenty presses
	// would leave twenty entries whose teardown runs the identical command
	// twenty times. Nothing about a cut needs its own undo, because Stop
	// removes the whole table.
	//
	// The Step is still what is run, so the ruleset and the reason are
	// netcfg's and this package composes no nft input of its own.
	if _, err := s.cfg.Runner.Run(ctx, step.Do); err != nil {
		s.recordFailure("client traffic could not be cut or restored", "", err)
		return fail("cut", faultOf(err), err)
	}

	s.mu.Lock()
	s.forward = want
	s.mu.Unlock()

	if want == netcfg.ForwardCut {
		s.cfg.Logger.Warn("client traffic cut by the user",
			"hotspot", plan.Hotspot, "note", "the hotspot stays up and this does not survive a restart")
	} else {
		s.cfg.Logger.Info("client traffic restored", "hotspot", plan.Hotspot)
	}
	return nil
}
