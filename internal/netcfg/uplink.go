// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"fmt"
	"net/netip"
	"time"
)

// UplinkState is the small set of facts the pinned host route depends on.
//
// The hazard this type exists for is in the design's list of known hazards:
// the pinned host route to the user's server is silently wrong after a DHCP
// renewal that hands out a different gateway, after a cable is unplugged, or
// after the box is moved to another network. Nothing fails loudly. The route
// still exists, it still points at an address, and that address is no longer a
// gateway, so the engine cannot reach the server and the panel reports a
// configuration problem that is not one.
type UplinkState struct {
	Interface string
	Gateway   netip.Addr
	OnLink    bool
}

// Equal reports whether two states describe the same uplink.
func (u UplinkState) Equal(o UplinkState) bool {
	return u.Interface == o.Interface && u.Gateway == o.Gateway && u.OnLink == o.OnLink
}

// IsZero reports whether the state names no uplink at all.
func (u UplinkState) IsZero() bool {
	return u.Interface == "" && !u.Gateway.IsValid()
}

// String renders the state for logs.
func (u UplinkState) String() string {
	if u.IsZero() {
		return "no uplink"
	}
	if u.OnLink || !u.Gateway.IsValid() {
		return fmt.Sprintf("%s (no gateway)", u.Interface)
	}
	return fmt.Sprintf("%s via %s", u.Interface, u.Gateway)
}

// UplinkChanged compares two states and returns a reason fit for a log line.
// The reason distinguishes the three cases because they need different
// responses: a lost uplink means stop, a new interface means replan, and a new
// gateway on the same interface means re-derive the pinned routes only.
func UplinkChanged(old, now UplinkState) (bool, string) {
	switch {
	case old.Equal(now):
		return false, ""
	case now.IsZero():
		return true, fmt.Sprintf("uplink lost (was %s)", old)
	case old.IsZero():
		return true, fmt.Sprintf("uplink appeared: %s", now)
	case old.Interface != now.Interface:
		return true, fmt.Sprintf("uplink moved from %s to %s", old.Interface, now.Interface)
	default:
		return true, fmt.Sprintf("gateway on %s changed from %s to %s", now.Interface, old.Gateway, now.Gateway)
	}
}

// ReadUplinkState asks the machine what the uplink is now. It is the cheap
// half of detection: one command, no radio enumeration.
func ReadUplinkState(ctx context.Context, r Runner) (UplinkState, error) {
	res, err := r.Run(ctx, Command{
		Path: BinIP, Args: []string{"route", "show", "default"},
		Why: "the uplink is whichever interface carries the default route",
	})
	if err != nil {
		return UplinkState{}, fmt.Errorf("netcfg: read default route: %w", err)
	}
	routes, err := ParseDefaultRoutes(res.Stdout)
	if err != nil {
		return UplinkState{}, err
	}
	def, ok := Facts{Routes: routes}.PrimaryDefault()
	if !ok {
		return UplinkState{}, nil
	}
	return UplinkState{Interface: def.Dev, Gateway: def.Gateway, OnLink: def.OnLink}, nil
}

// WithUplink returns a copy of the plan rebound to a new uplink. Nothing else
// in the plan changes: the hotspot, its subnet and the tunnel addresses are
// independent of which way the internet arrives.
func (p *Plan) WithUplink(u UplinkState) *Plan {
	q := *p
	q.Uplink = u.Interface
	q.UplinkGateway = u.Gateway
	q.UplinkOnLink = u.OnLink
	return &q
}

// RederiveForUplink returns the steps that move the pinned host routes from
// the old uplink to a new one: first the inverses of the routes as they were
// pinned, then the same routes pinned through the new gateway.
//
// # Nothing in the appliance calls this
//
// This function and [WatchUplink] are a capability, not a shipped behaviour.
// No code outside this package's own tests calls either, which
// TestNothingInTheApplianceWatchesTheUplink pins. A moved uplink today means
// the tunnel stops and stays stopped until somebody presses connect, and
// docs/BEHAVIOUR.md, "a change of uplink leaves the box blocked and waiting for
// a reconnect", is the statement of that. Wiring a watcher in is a feature and
// needs its own measurement and its own decision, so it must not be done as a
// side effect of making a sentence true.
//
// # Why the firewall is returned, and what that is NOT for
//
// The regenerated ruleset is returned as the third value so that a caller which
// moves the routes also replaces the text naming the old interface, and the
// generated header keeps describing the machine it is loaded on.
//
// It is NOT load bearing for the block. This comment used to say the leak block
// names the uplink, so "a ruleset still naming the old interface stops blocking
// the moment traffic starts leaving by the new one". That is the inverse of what
// the generator builds. The forward chain's policy is drop and every accept in
// it names the TUNNEL, so traffic leaving by an interface the ruleset does not
// mention is dropped by the policy. The explicit leak block is a named,
// readable statement of an outcome the policy already guarantees.
// TestRulesetStillBlocksWhenTheUplinkIsRenamed is the proof.
func (p *Plan) RederiveForUplink(u UplinkState) (undo []Step, redo []Step, firewall Step, newPlan *Plan) {
	old := p.ServerRouteSteps()
	undo = make([]Step, 0, len(old))
	for i := len(old) - 1; i >= 0; i-- {
		s := old[i]
		if s.Undo.IsZero() {
			continue
		}
		undo = append(undo, Step{
			Op:   s.Op,
			Why:  "remove the host route pinned through the previous gateway, which no longer reaches the server",
			Do:   s.Undo,
			Undo: s.Do,
		})
	}
	newPlan = p.WithUplink(u)
	return undo, newPlan.ServerRouteSteps(), newPlan.FirewallStep(), newPlan
}

// WatchUplink polls for uplink changes and calls onChange for each one. It
// polls rather than subscribing to netlink because the whole impure surface of
// this package is one Runner method, which keeps every path above it testable
// with a recorder.
//
// It returns when ctx is done, or when the runner fails twice in a row: a
// single failure is treated as transient because "ip" can lose a race with an
// interface disappearing.
func WatchUplink(ctx context.Context, r Runner, every time.Duration, initial UplinkState, onChange func(UplinkState, string)) error {
	if every <= 0 {
		every = 5 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	last := initial
	consecutiveErrors := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			now, err := ReadUplinkState(ctx, r)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= 2 {
					return err
				}
				continue
			}
			consecutiveErrors = 0
			if changed, reason := UplinkChanged(last, now); changed {
				last = now
				if onChange != nil {
					onChange(now, reason)
				}
			}
		}
	}
}
