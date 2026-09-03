// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// Start brings the tunnel and the hotspot up.
//
// Calling it when the same thing is already running does NOT re-apply the
// network configuration. What it does instead is re-assert the two supervised
// processes, which is how a hostapd that died is brought back without
// disturbing one that did not.
//
// internal/netcfg's Applier.Apply became idempotent on 2026-08-30 and now
// converges rather than failing on a second apply, so this is no longer what
// stops a repeat press from breaking the box. It is kept because it is still
// the right answer: a repeat press should not re-derive a plan, re-run twenty
// commands and re-read the machine to arrive back where it already was.
//
// Calling it with a DIFFERENT request while something is running is a
// configuration change: the running one is stopped, completely and with its
// journal replayed, before the new one is applied.
func (s *Service) Start(ctx context.Context, req panel.StartRequest) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	fp := requestFingerprint(req)

	if s.isRunning() {
		if s.Applied() == fp {
			s.cfg.Logger.Info("already running this configuration", "config", shortFingerprint(fp))
			return s.reassertLocked(ctx)
		}
		s.cfg.Logger.Info("configuration changed, stopping the running one first",
			"was", shortFingerprint(s.Applied()), "now", shortFingerprint(fp))
		if err := s.stopLocked(ctx); err != nil {
			return err
		}
	}

	if err := s.applyLocked(ctx, req, fp); err != nil {
		// Everything that was applied is undone. A box left half configured is
		// worse than one left alone: the user cannot see it, and the next
		// start plans against a machine it no longer describes.
		s.rollbackLocked(ctx, "start failed")
		return err
	}

	// From here the box is fully configured and fail-closed. The probe below
	// asks the third of the design's three questions and DOES NOT roll back if
	// the answer is no.
	//
	// The reason is worth stating, because "any failure tears down" is the rule
	// everywhere above this line. A server that does not answer is not a
	// half-applied box: every change succeeded, the firewall is in force, and
	// forwarded client traffic is blocked because the tunnel carries nothing.
	// Tearing down would take away the hotspot the user just watched appear and
	// leave them with no way to see the message. So the fault is reported and
	// the box keeps trying.
	return s.probeServer(ctx)
}

func (s *Service) applyLocked(ctx context.Context, req panel.StartRequest, fp string) error {
	// -----------------------------------------------------------------------
	// 1. The clock, before validation and before anything is attempted.
	//
	// docs/2026-08-29-design.md section 9: the clock check runs before
	// validation, not only before connecting, because WHICH CONFIGS THE ENGINE
	// ACCEPTS DEPENDS ON THE DATE. A box whose clock is wrong accepts a config
	// the same binary rejects once the clock is corrected.
	// -----------------------------------------------------------------------
	if !clockPlausible(s.cfg.Now(), s.cfg.ClockFloor) {
		return fail("clock", panel.FaultClockImplausible,
			errors.New("this machine's clock is earlier than the date this software was built"))
	}

	// -----------------------------------------------------------------------
	// 2. Read what arrived, and refuse what cannot work, before touching the
	//    machine. A box that reconfigured its firewall and then said "I could
	//    not read that" has changed the network for nothing.
	// -----------------------------------------------------------------------
	l, err := configFromRequest(req.ConfigJSON)
	if err != nil {
		return err
	}
	if err := s.validateStatic(req); err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// 3. Clean up after a previous run that was killed. Nothing is applied at
	//    this point, so the machine can be put back to the state the detection
	//    below is about to measure.
	// -----------------------------------------------------------------------
	if _, err := s.recoverLocked(ctx); err != nil {
		return fail("recover", faultOf(err), err)
	}

	// -----------------------------------------------------------------------
	// 4. Look at the machine, and check the request against what it says.
	// -----------------------------------------------------------------------
	facts, err := s.cfg.Backend.Detect(ctx, s.cfg.Runner, s.cfg.Backend.BaseSysctlKnobs())
	if err != nil {
		return fail("detect", faultOf(err), err)
	}
	if err := s.validateAgainstFacts(req, facts); err != nil {
		return err
	}
	country := req.Hotspot.Country
	if country == "" {
		country = s.regulatoryDomain(ctx)
	}

	// -----------------------------------------------------------------------
	// 5. Resolve the server. The pinned host route to it is part of the plan,
	//    so the address has to be known before the plan is made, which means
	//    before the tunnel exists. See the Resolver documentation for what
	//    that discloses.
	// -----------------------------------------------------------------------
	servers, err := s.cfg.Resolver.Resolve(ctx, l.Address)
	if err != nil {
		return fail("resolve", panel.FaultServerNoAnswer, err)
	}

	// -----------------------------------------------------------------------
	// 6. Decide.
	// -----------------------------------------------------------------------
	netOpts, err := s.netOptionsFor(req)
	if err != nil {
		return err
	}
	plan, err := netcfg.PlanNetwork(facts, servers, netOpts)
	if err != nil {
		return fail("plan", faultOf(err), err)
	}
	if err := s.validateAgainstPlan(req, plan); err != nil {
		return err
	}
	for _, n := range plan.Notes {
		s.note(n)
	}

	// The kernel knobs are read again ONLY if the plan needs one the first read
	// did not fetch, which is the shape internal/netcfg's own DetectAndPlan
	// uses and the reasoning is its: a knob with no measured value gets no
	// inverse, and teardown then cannot put it back.
	//
	// Today it never re-reads. Every knob a plan changes is global
	// (internal/netcfg/route.go, SysctlKnobs: "Every knob here is global. Not
	// one names an interface"), so the first read already has all of them. The
	// branch is kept rather than deleted because the day a plan needs a knob
	// that names an interface, this is where it is noticed instead of the knob
	// being changed with no value to restore.
	if missingKnobs(facts, plan.SysctlKnobs()) {
		facts, err = s.cfg.Backend.Detect(ctx, s.cfg.Runner, plan.SysctlKnobs())
		if err != nil {
			return fail("detect", faultOf(err), err)
		}
	}

	// -----------------------------------------------------------------------
	// 7. Compose the engine's configuration and ask the engine whether it will
	//    take it, before touching the machine. This is the second of the three
	//    failure states (design section 8, step 11) and it has to be reachable
	//    without having reconfigured anything.
	// -----------------------------------------------------------------------
	doc, err := s.engineDocument(l, req, netOpts)
	if err != nil {
		return err
	}
	// engine.Validate decodes the document and builds it, opening no socket
	// and dialing nothing. Its error is already redacted: internal/engine's
	// Error type has no Unwrap, so the unredacted cause cannot be recovered
	// downstream even by this package.
	if err := engine.Validate(doc); err != nil {
		s.recordFailure("the engine would not accept the configuration", "", err)
		return fail("engine configuration", panel.FaultEngineRejectedConfig, err)
	}

	// The access point is rendered here too, still before anything is applied,
	// so that a passphrase hostapd would refuse or a channel the radio will not
	// take is a refusal rather than a half-configured box.
	hp, err := s.hotspotPlanFor(plan, facts, req, country)
	if err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// 8. Apply the pre-engine steps.
	//
	// The ORDER INSIDE this list is internal/netcfg's, not this package's, and
	// that is the point of the split: the firewall is first so there is never
	// a moment when forwarding is enabled and the block is not, and the pinned
	// host route is last of the routing work and still before the engine, so
	// the engine's very first connection to the server is already outside the
	// tunnel. This package must not reorder them and must not merge the two
	// lists.
	// -----------------------------------------------------------------------
	// The radio may refuse to create the access point's interface even though
	// its own combination table says it can, so this returns the plan that was
	// actually applied, which may be the takeover fallback, together with the
	// facts that plan was made from.
	plan, hp, facts, err = s.applyPreEngine(ctx, plan, hp, facts, req, country)
	if err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// 9. Read the hotspot interface back from the kernel, BEFORE anything is
	//    allowed to bind to it.
	//
	// This is the readback of the steps step 8 just applied. Applying a step
	// is not the same as the kernel having done it, and the difference is not
	// theoretical: MEASURED on the target on 2026-08-30, the service reported
	// running with hotspot=wlan0 while wlan0 was type managed, still joined to
	// the house network on the station's channel, and still holding its
	// station address. dnsmasq bound to it anyway and answered a real device
	// on that LAN with "DHCPNAK(wlan0) ... wrong server-ID", which is this
	// appliance disrupting a network it does not own.
	//
	// It is here, next to the steps it verifies, rather than immediately
	// before the access point starts, for two reasons. It is the readback OF
	// those steps and belongs with them. And a box that cannot prove the
	// interface is its own must stop before the engine dials the user's
	// server, not after.
	//
	// What runs between this line and the access point starting, checked
	// rather than assumed: the engine, whose two inbounds internal/xcfg pins
	// to loopback IP literals (checkLoopbackListen) and whose TUN inbound
	// creates the tunnel device; and PostEngineSteps, which adds a route
	// through the hotspot interface into the tunnel table. A route through an
	// interface is neither a server bound to it nor a release of it, so
	// nothing in that window changes the answer this check just read or acts
	// on it.
	//
	// It also cannot be moved LATER. Once hostapd is beaconing, "iw dev"
	// reports an SSID for the interface, netcfg's WirelessIface.InUse is true
	// for an access point that is broadcasting one, and this check would then
	// refuse every working box. The guard is
	// TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter, which
	// pins both ends: after the steps, before either server.
	// -----------------------------------------------------------------------
	if err := s.assertHotspotInterfaceReleased(ctx, plan); err != nil {
		return err
	}

	s.mu.Lock()
	s.plan = plan
	s.facts = facts
	s.hotspotPlan = hp
	s.engineDoc = doc
	s.mu.Unlock()

	// -----------------------------------------------------------------------
	// 10. The engine. This is the moment the tunnel device is created.
	// -----------------------------------------------------------------------
	if err := s.cfg.Engine.Start(ctx, doc); err != nil {
		s.recordFailure("the engine would not start", "", err)
		return fail("engine", panel.FaultEngineRejectedConfig, err)
	}

	// -----------------------------------------------------------------------
	// 11. The steps that name the tunnel device. Every command in this list
	//     names it, so every one of them fails if it is run before step 10.
	// -----------------------------------------------------------------------
	rep, err := s.currentApplier().Apply(ctx, plan.PostEngineSteps(facts.Sysctl))
	if err != nil {
		s.recordStepFailure("applying the tunnel routing", plan.ServerAddr, rep, err)
		return fail("apply", faultOf(err), err)
	}

	// -----------------------------------------------------------------------
	// 12. The access point and its DHCP and DNS server, last, because a client
	//     that joins before the tunnel exists is a client with a working
	//     network connection and nowhere to go.
	// -----------------------------------------------------------------------
	st, err := s.sup.Start(ctx, hp)
	if err != nil {
		s.recordFailure("the access point did not come up", st.Reason, err)
		return fail("hotspot", hotspotFault(unitAP, st.Reason, err), err)
	}
	if !st.Running {
		s.recordFailure("the access point did not come up", st.Reason, nil)
		return fail("hotspot", hotspotFault(unitAP, st.Reason, nil), errors.New(st.Reason))
	}

	// -----------------------------------------------------------------------
	// 13. Read the access point back, before this service says it is running.
	//
	// hostapd being alive is the same class of evidence as an exit code:
	// necessary, not sufficient. On the target hostapd was a live process, its
	// control socket did not answer, and a phone in the room listed eleven
	// networks with ours not among them.
	// -----------------------------------------------------------------------
	if err := s.assertHotspotIsAccessPoint(ctx, plan, hp.AP.SSID); err != nil {
		return err
	}

	s.mu.Lock()
	s.running = true
	s.fingerprint = fp
	s.serverPortValue = l.Port
	// The cached detection describes a machine that has just changed: the
	// hotspot interface now exists and holds an address. Invalidating it means
	// the next status poll reports the box as it is now rather than as it was
	// before the switch was pressed.
	s.lastDetectAt = time.Time{}
	s.country = country
	s.mu.Unlock()

	s.cfg.Logger.Info("running",
		"config", shortFingerprint(fp),
		"uplink", plan.Uplink,
		"hotspot", plan.Hotspot,
		"tunnel", plan.Tun,
		"channel", plan.Channel)
	return nil
}

// reassertLocked re-runs the two supervised processes for a configuration that
// is already applied, and touches the network configuration not at all.
//
// internal/hotspot's Start is idempotent by comparing the configuration on disk
// with the one in the plan, and internal/engine's Start returns immediately
// when an instance is already running. So this repairs a daemon that died and
// disturbs one that did not.
//
// It is the SECOND path in this package that can bind a server to the hotspot
// interface, and the rule that nothing may bind to that interface until it has
// been proved ours is a rule about every path in, not about the one that was
// checked. A hostapd that died is exactly the state in which something else
// can have taken the interface back, so this repairs nothing until it knows
// what it is repairing on.
func (s *Service) reassertLocked(ctx context.Context) error {
	s.mu.RLock()
	hp := s.hotspotPlan
	doc := s.engineDoc
	plan := s.plan
	s.mu.RUnlock()

	// Two states are safe to start a DHCP server on, and this is the whole
	// list. Either the interface is still the access point this service
	// started, or it is free. Anything else is somebody's network.
	if plan != nil {
		if apErr := s.assertHotspotIsAccessPoint(ctx, plan, hp.AP.SSID); apErr != nil {
			if err := s.assertHotspotInterfaceReleased(ctx, plan); err != nil {
				return err
			}
		}
	}

	if err := s.cfg.Engine.Start(ctx, doc); err != nil {
		return fail("engine", panel.FaultEngineRejectedConfig, err)
	}
	st, err := s.sup.Start(ctx, hp)
	if err != nil {
		s.recordFailure("the access point did not come back up", st.Reason, err)
		return fail("hotspot", hotspotFault(unitAP, st.Reason, err), err)
	}
	if !st.Running {
		s.recordFailure("the access point did not come back up", st.Reason, nil)
		return fail("hotspot", hotspotFault(unitAP, st.Reason, nil), errors.New(st.Reason))
	}
	if plan != nil {
		if err := s.assertHotspotIsAccessPoint(ctx, plan, hp.AP.SSID); err != nil {
			return err
		}
	}
	// A repair changed the machine: the access point that was down is up. The
	// cached detection describes the box before that, and Status now reads the
	// interface back out of it, so a stale reading would report a repaired
	// hotspot as not broadcasting for as long as the cache lives.
	s.mu.Lock()
	s.lastDetectAt = time.Time{}
	s.mu.Unlock()
	return nil
}

// probeServer asks whether the user's server answers. See Reachability for what
// a success here does and does not prove.
func (s *Service) probeServer(ctx context.Context) error {
	s.mu.RLock()
	plan := s.plan
	s.mu.RUnlock()
	if plan == nil || len(plan.ServerAddr) == 0 {
		return nil
	}
	port := s.serverPort()
	if port == 0 {
		return nil
	}
	var last error
	for _, a := range plan.ServerAddr {
		if err := s.cfg.Reach.Probe(ctx, a, port); err == nil {
			return nil
		} else {
			last = err
		}
	}
	s.cfg.Logger.Warn("the proxy server did not answer", "addresses", len(plan.ServerAddr))
	return fail("server", panel.FaultServerNoAnswer, last)
}

// Stop takes the tunnel and the hotspot down and returns the machine to how it
// was, by replaying the teardown journal.
//
// The order is engine, hotspot, then the journal, and the last one is what
// makes it safe. The journal replays newest first, and the firewall was the
// FIRST thing applied, so its inverse is the LAST thing run: the fail-closed
// ruleset stays in force while every route, rule and address is removed, and
// only then is it taken away. Nothing here has to remember that; it falls out
// of the journal being a stack.
// Recover is the way out of a stuck box without a reboot and without a
// terminal.
//
// It stops whatever is running, replays the teardown journal so that every
// change this appliance made to the machine is put back, and then starts again
// from req. The journal replay is the part that matters and the part a plain
// stop does not give you: a start that failed part way through can leave an
// entry behind, and the next start then trips over it.
//
// MEASURED on 2026-08-30, which is why this exists. The appliance reached
// several states that only somebody with an SSH session could clear: an
// interface created by a failed start and never removed, an address flushed out
// from under it, a journal reporting "some changes could not be undone and stay
// in the journal". Each was recoverable from what was already written down, and
// none of it was reachable from the panel, which is the only thing a person
// with a phone has.
//
// It deliberately does not reboot and does not restart the two systemd units.
// The panel and any SSH session stay up throughout. A control that takes away
// the page you pressed it on is not a recovery control for somebody holding a
// phone, and that person is the one who needs it.
//
// The stop and the replay are done under the operation lock together, so
// nothing can start between them. Start takes the lock itself, so it is called
// after the lock is released rather than from inside.
func (s *Service) Recover(ctx context.Context, req panel.StartRequest) error {
	if err := s.recoverToCleanMachine(ctx); err != nil {
		return err
	}
	return s.Start(ctx, req)
}

// recoverToCleanMachine is the first half of Recover: everything down, and the
// machine put back the way it was found. Split out so the lock is held across
// both steps and released before Start takes it again.
func (s *Service) recoverToCleanMachine(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// A stop that fails is not a reason to skip the replay. The replay is the
	// part that clears what a failed start left behind, so it runs either way
	// and the stop error is reported only if the replay also fails.
	stopErr := s.stopLocked(ctx)

	rep, err := s.recoverLocked(ctx)
	if err != nil {
		if stopErr != nil {
			return fmt.Errorf("recover: stopping failed (%v) and replaying the journal failed: %w", stopErr, err)
		}
		return fmt.Errorf("recover: replaying the journal failed: %w", err)
	}
	s.cfg.Logger.Info("recovered the machine to how it was found",
		"inverses_run", len(rep.Results), "inverses_failed", rep.Failed)
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.stopLocked(ctx)
}

func (s *Service) stopLocked(ctx context.Context) error {
	var errs []error

	if err := s.cfg.Engine.Stop(); err != nil {
		// internal/engine's Stop leaves the phase stopped whether or not
		// closing complained, so this is "it stopped and closing these
		// features complained", never "it is still running".
		s.cfg.Logger.Warn("the engine complained while stopping", "error", err.Error())
		errs = append(errs, err)
	}
	if err := s.sup.Stop(ctx); err != nil {
		s.cfg.Logger.Warn("the hotspot complained while stopping", "error", err.Error())
		errs = append(errs, err)
	}

	s.mu.Lock()
	ap := s.applier
	s.applier = nil
	s.plan = nil
	s.running = false
	s.fingerprint = ""
	s.forward = netcfg.ForwardNormal
	s.engineDoc = nil
	s.lastDetectAt = time.Time{}
	s.mu.Unlock()

	if ap != nil {
		rep, err := ap.Teardown(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		if rep.Failed > 0 {
			// A teardown that could not undo everything leaves the rest in the
			// journal for the next start to retry, so this is a warning and
			// not a failure. Saying how many is what lets somebody tell "one
			// route was already gone" from "nothing was undone".
			s.cfg.Logger.Warn("some changes could not be undone and stay in the journal",
				"undone", len(rep.Results)-rep.Failed, "left", rep.Failed)
		}
	}
	if len(errs) > 0 {
		err := errors.Join(errs...)
		return fail("stop", faultOf(err), err)
	}
	return nil
}

// rollbackGrace bounds the undo of a start that failed.
const rollbackGrace = 30 * time.Second

// rollbackLocked undoes a start that failed part way through.
//
// It keeps going past every failure, for the same reason internal/netcfg's
// Teardown does: a single step failing says nothing about the rest, and
// stopping would leave every earlier change in place, which is the opposite of
// what a rollback is for. The original error is what the caller sees; anything
// that went wrong here is logged.
//
// # Why the caller's context is deliberately not used
//
// One of the ways a start fails is that its own deadline ran out, and the
// caller's context is then already cancelled. Undoing on that context would run
// every inverse against a dead context and every one of them would fail
// immediately, so the box would be left carrying exactly the changes this
// function exists to remove, while the log said a rollback had been attempted.
//
// context.WithoutCancel keeps the caller's values and drops its cancellation,
// and a fresh bound is put on top so that a wedged teardown cannot hold the
// service's operation lock for ever.
func (s *Service) rollbackLocked(ctx context.Context, why string) {
	s.cfg.Logger.Warn("undoing a part-applied start", "reason", why)

	undoCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackGrace)
	defer cancel()

	if err := s.stopLocked(undoCtx); err != nil {
		s.cfg.Logger.Error("the box could not be returned to how it was found", "error", err.Error())
	}
}

// serverPort is the port the engine will dial. It is read back out of the
// applied plan's link rather than kept, so there is one source for it.
func (s *Service) serverPort() uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverPortValue
}

// requestFingerprint identifies a start request without holding any part of it.
//
// It covers every field that decides what gets applied, so that "the same
// request" means the same configuration AND the same hotspot AND the same
// network policy. A change in any of them has to stop and restart, because
// internal/netcfg's steps are not idempotent.
//
// It is a SHA-256 and only its first eight hex digits are ever logged, which is
// the same identifier and the same length internal/state uses for a stored
// config (ProxyConfig.Fingerprint): enough to tell two configurations apart in
// a log, and no part of either.
func requestFingerprint(req panel.StartRequest) string {
	h := sha256.New()
	write := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		h.Write(n[:])
		h.Write([]byte(s))
	}
	write(string(req.ConfigJSON))
	write(req.Hotspot.SSID)
	write(req.Hotspot.Passphrase)
	write(req.Hotspot.Interface)
	write(fmt.Sprintf("%d", req.Hotspot.Channel))
	write(req.Hotspot.Band)
	write(req.Hotspot.Country)
	write(req.Hotspot.Subnet)
	write(req.Network.InternetInterface)
	write(req.Network.DNSMode)
	write(req.Network.OnTunnelDown)
	write(req.Network.ClientIPv6)
	write(req.EngineLogLevel)
	return hex.EncodeToString(h.Sum(nil))
}

// applyPreEngine applies the pre-engine steps, falling back to a hotspot
// takeover when the radio refuses to create the access point's own interface.
//
// # Why this is a runtime fallback and not a planner rule
//
// A radio's interface-combination table states what the hardware could do in
// principle. It is not proof that creating the interface succeeds. MEASURED on
// the target on 2026-08-30: phy0 advertises a combination allowing an access
// point beside the station link, and "iw phy phy0 interface add ap0 type __ap"
// returns "Input/output error (-5)" while wlan0 is associated. The planner
// reads the table correctly and the driver refuses anyway, so nothing short of
// trying settles it. internal/netcfg/doc.go, "When the radio refuses what it
// advertises", records the same finding from its side.
//
// Creating a second interface stays the FIRST choice, because when it works it
// costs the user nothing: the WiFi connection that interface's radio already
// holds keeps running. The fallback is only reached after the first choice has
// actually been tried and refused.
//
// # Why the teardown between the two is not optional
//
// Both plans touch the same firewall, the same kernel knobs and the same
// hotspot address, and the second names a different interface. Applying the
// fallback on top of a half-applied first plan would leave the journal
// describing a machine that never existed, and a later teardown would then try
// to undo an address on an interface that never held it.
//
// # What the fallback costs, and why it is said out loud
//
// Taking over an interface ends whatever WiFi connection it currently holds.
// internal/netcfg puts that sentence in the plan's notes and this passes it to
// Service.note, which puts it in the log and in the advanced view. Somebody
// whose other WiFi connection drops is otherwise left guessing.
//
// It is never the uplink: internal/netcfg refuses in words a person can act on
// when the only candidate is the interface carrying the internet, and that
// refusal is passed through rather than reinterpreted here.
// # Why the takeover decision is made from a fresh reading of the machine
//
// internal/netcfg's Plan.HotspotTakeover refuses unless it can prove the
// interface is takeable, and it asks that of a netcfg.Facts. The facts this
// function was given describe the machine as it was BEFORE the first plan was
// applied and undone, which is the wrong moment: what has to be true is that
// the interface is takeable NOW. The two can differ without anything in this
// package doing it, because NetworkManager is still running: the radio can
// roam to another network, or pick up a different address, between the two
// points.
//
// The cost is one detection, five read-only commands, on a path that is
// already the slow one. The reading is taken after the first plan's teardown
// has succeeded, which is what makes the kernel knobs in it the values the
// first plan started from rather than the values it left behind.
//
// "Has succeeded" is now enforced rather than assumed. Until 2026-08-30 this
// paragraph ended "a teardown that failed returns above and never reaches
// here", and the guard it named tested only Teardown's error, which is never
// set by a failed inverse. So a teardown that undid nothing carried straight
// on. The blast radius was bounded, because the journal is a stack and the
// older entry holding the ORIGINAL value is replayed last, but "the wrong
// value gets corrected later, provided a replay eventually succeeds" is not a
// sentence this package should rest on.
func (s *Service) applyPreEngine(
	ctx context.Context,
	plan *netcfg.Plan,
	hp hotspot.Plan,
	facts netcfg.Facts,
	req panel.StartRequest,
	country string,
) (*netcfg.Plan, hotspot.Plan, netcfg.Facts, error) {

	ap, err := netcfg.NewApplier(s.cfg.Runner, s.cfg.JournalPath)
	if err != nil {
		return nil, hp, facts, fail("journal", faultOf(err), err)
	}
	s.setApplier(ap)

	rep, applyErr := ap.Apply(ctx, plan.PreEngineSteps(facts.Sysctl))
	if applyErr == nil {
		return plan, hp, facts, nil
	}
	s.recordStepFailure("applying the network configuration", plan.ServerAddr, rep, applyErr)

	step, ok := rep.FailedStep()
	if !ok || step.Op != netcfg.OpCreateIface {
		return nil, hp, facts, fail("apply", faultOf(applyErr), applyErr)
	}

	// Undo the first plan completely before trying the second.
	//
	// BOTH halves of the answer are checked, and the second is the one that was
	// missing until 2026-08-30. internal/netcfg's Applier.Teardown returns an
	// error only when the journal FILE cannot be closed or rewritten; a replay in
	// which every inverse failed returns no error at all, and the count lives in
	// the report. This code read the error alone under a comment asserting that
	// "a teardown that failed returns above and never reaches here", which was
	// the inverse of what that function does, and the fallback was applied on top
	// of a machine still carrying the first plan.
	//
	// stopLocked already knew to look at both. This makes the two paths agree.
	// TestTeardownReportsFailedInversesInItsReportAndNotInItsError pins the
	// contract on netcfg's side; the guard on this side is
	// TestTheFallbackIsRefusedWhenTheFirstPlanCouldNotBeUndone.
	tRep, tErr := ap.Teardown(ctx)
	if tErr != nil || tRep.Failed > 0 {
		why := "could not undo the first plan, so the fallback was not attempted"
		if tErr != nil {
			s.cfg.Logger.Error(why, "error", redactedText(tErr.Error(), plan.ServerAddr))
		} else {
			s.cfg.Logger.Error(why, "inverses_failed", tRep.Failed,
				"undone", len(tRep.Results)-tRep.Failed)
		}
		s.diag.add("could not undo the first plan, so the hotspot fallback was not attempted")
		return nil, hp, facts, fail("apply", faultOf(applyErr), applyErr)
	}
	s.setApplier(nil)

	fresh, err := s.cfg.Backend.Detect(ctx, s.cfg.Runner, s.cfg.Backend.BaseSysctlKnobs())
	if err != nil {
		return nil, hp, facts, fail("detect", faultOf(err), err)
	}

	fallback, fErr := plan.HotspotTakeover(fresh)
	if fErr != nil {
		// internal/netcfg refuses with wording written for this audience.
		// Passed through as its own fault and its own sentence rather than
		// being reinterpreted here.
		var pe *netcfg.PlanError
		if errors.As(fErr, &pe) {
			s.diag.add(pe.UserMessage())
		}
		return nil, hp, fresh, fail("hotspot takeover", faultOf(fErr), fErr)
	}

	// The fallback is a different interface on a channel that is no longer
	// pinned, so both checks that depend on the chosen radio run again.
	if err := s.validateAgainstPlan(req, fallback); err != nil {
		return nil, hp, fresh, err
	}
	fallbackHotspot, hErr := s.hotspotPlanFor(fallback, fresh, req, country)
	if hErr != nil {
		return nil, hp, fresh, hErr
	}
	for _, n := range fallback.Notes {
		s.note(n)
	}
	s.note(fallback.Explain())

	ap2, err := netcfg.NewApplier(s.cfg.Runner, s.cfg.JournalPath)
	if err != nil {
		return nil, hp, fresh, fail("journal", faultOf(err), err)
	}
	s.setApplier(ap2)

	rep2, applyErr2 := ap2.Apply(ctx, fallback.PreEngineSteps(fresh.Sysctl))
	if applyErr2 != nil {
		s.recordStepFailure("applying the fallback network configuration", fallback.ServerAddr, rep2, applyErr2)
		return nil, fallbackHotspot, fresh, fail("apply", faultOf(applyErr2), applyErr2)
	}
	return fallback, fallbackHotspot, fresh, nil
}

// currentApplier returns the applier in force. It is never nil between
// applyPreEngine returning without an error and stopLocked clearing it.
func (s *Service) currentApplier() *netcfg.Applier {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.applier
}

// setApplier publishes the applier a rollback has to use. It is set before the
// steps it will have to undo are applied, and replaced when the fallback opens
// a second journal, so a failure at any point rolls back the right one.
func (s *Service) setApplier(ap *netcfg.Applier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applier = ap
}

// missingKnobs reports whether any of the knobs a plan will change has no
// measured value in the facts detection returned.
func missingKnobs(f netcfg.Facts, knobs []string) bool {
	for _, k := range knobs {
		if _, ok := f.Sysctl[k]; !ok {
			return true
		}
	}
	return false
}

func shortFingerprint(fp string) string {
	if len(fp) < 8 {
		return fp
	}
	return fp[:8]
}
