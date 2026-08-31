// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// Service is the privileged side of the appliance.
//
// It is safe for concurrent use, which is a requirement and not a courtesy:
// internal/panel/priv.go says "the panel polls Status from one request while
// another is running Start."
//
// Two locks, and the split is the reason a poll is never blocked behind a
// start. opMu serialises Start against Stop and is held for the whole of an
// operation, including the slow parts. mu guards the fields a status poll
// reads, and is only ever held for the length of a struct copy. This is the
// same shape internal/engine uses for the same reason.
type Service struct {
	cfg  Config
	sup  *hotspot.Supervisor
	diag *diagRing
	opMu sync.Mutex

	mu          sync.RWMutex
	applier     *netcfg.Applier
	plan        *netcfg.Plan
	facts       netcfg.Facts
	hotspotPlan hotspot.Plan
	engineDoc   []byte
	running     bool
	fingerprint string

	// forward is whether client traffic is being forwarded, and it lives HERE
	// and nowhere else. It is deliberately not persisted: see cut.go. Its zero
	// value is netcfg.ForwardNormal, so a Service that has never been asked to
	// cut is forwarding, and a Service rebuilt after a restart is too.
	forward      netcfg.ForwardState
	lastDetect   panel.Detection
	lastDetectAt time.Time
	country      string

	// serverPortValue is the port the engine dials, kept so the reachability
	// probe asks about the same endpoint the plan pinned a route to. It is
	// the only part of the user's configuration this struct retains, it is
	// not a credential, and it is never logged beside the address.
	serverPortValue uint16
}

var _ panel.Privileged = (*Service)(nil)

// New returns a Service. It touches nothing: Recover is the first thing that
// does, and the caller runs it deliberately.
func New(cfg Config) (*Service, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}
	cfg = cfg.withDefaults()
	return &Service{
		cfg:     cfg,
		sup:     hotspot.NewSupervisor(cfg.System, cfg.HotspotPaths),
		diag:    newDiagRing(diagCapacity, cfg.Now),
		country: cfg.Country,
	}, nil
}

// Recover replays a teardown journal left behind by a process that was killed,
// and must be called before anything else the service does.
//
// It is not tidying. The plan a later Start applies assumes the machine is in
// the state detection found it in, and a leftover policy rule, half default
// route or loaded ruleset makes that false. internal/netcfg/apply.go says the
// same from its side: "recovery happens before the new plan is applied, not
// after."
//
// A missing journal is not an error: it means there is nothing to undo.
// ReplayJournal puts the machine back the way this appliance found it, by
// running the inverse of every change recorded in the teardown journal. It is
// what the privileged service does on boot, to clean up after a run that was
// killed rather than stopped.
//
// It was called Recover, and the name is now taken by the panel-facing action
// that stops, replays and starts again. Two different things: this one only
// replays, and it is the piece that one is built out of.
func (s *Service) ReplayJournal(ctx context.Context) (netcfg.Report, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.recoverLocked(ctx)
}

func (s *Service) recoverLocked(ctx context.Context) (netcfg.Report, error) {
	rep, err := netcfg.Recover(ctx, s.cfg.Runner, s.cfg.JournalPath)
	if err != nil {
		s.cfg.Logger.Error("could not replay the journal left by a previous run",
			"journal", s.cfg.JournalPath, "error", err.Error())
		return rep, err
	}
	if len(rep.Results) > 0 {
		s.cfg.Logger.Warn("replayed changes left by a previous run that was killed",
			"inverses_run", len(rep.Results), "inverses_failed", rep.Failed)
	}
	return rep, nil
}

// Detect reports what the machine has. It changes nothing.
func (s *Service) Detect(ctx context.Context) (panel.Detection, error) {
	d, err := s.detect(ctx)
	if err != nil {
		return panel.Detection{}, err
	}
	return d, nil
}

// detect runs a full detection and turns it into the panel's vocabulary.
func (s *Service) detect(ctx context.Context) (panel.Detection, error) {
	facts, err := netcfg.Detect(ctx, s.cfg.Runner, netcfg.BaseSysctlKnobs())
	if err != nil {
		return panel.Detection{}, fail("detect", faultOf(err), err)
	}
	country := s.regulatoryDomain(ctx)

	s.mu.RLock()
	live := s.plan
	running := s.running
	s.mu.RUnlock()

	var (
		plan  *netcfg.Plan
		fault = panel.FaultNone
	)
	if running && live != nil {
		plan = live
	} else {
		// PlanNetwork will not plan without a server address, because a plan
		// with none is a plan that loops the engine through its own tunnel.
		// Detection has no config and therefore no server, so a documentation
		// address stands in purely so the planner can report which interface,
		// channel and subnet it WOULD choose.
		//
		// THE RESULTING PLAN MUST NEVER BE APPLIED. It carries a pinned host
		// route to an address nobody owns. That is why this function returns a
		// panel.Detection and not a *netcfg.Plan: there is no value here for a
		// later caller to pick up by mistake.
		p, perr := netcfg.PlanNetwork(facts, []netip.Addr{detectionPlaceholderServer}, s.cfg.netOptions())
		if perr != nil {
			fault = faultOf(perr)
			s.cfg.Logger.Info("this machine has no workable arrangement yet", "fault", string(fault))
		} else {
			plan = p
		}
	}

	d := detectionFrom(facts, plan, country, fault, s.cfg.Now())
	s.mu.Lock()
	s.facts = facts
	s.lastDetect = d
	s.lastDetectAt = s.cfg.Now()
	if country != "" {
		s.country = country
	}
	s.mu.Unlock()
	return d, nil
}

// detectionPlaceholderServer is RFC 5737 TEST-NET-1. It exists so that
// PlanNetwork can run with no config pasted; see detect.
var detectionPlaceholderServer = netip.MustParseAddr("192.0.2.1")

// cachedDetection returns the last detection when it is fresh enough, and runs
// a new one otherwise.
//
// The panel polls status every couple of seconds and a full detection is five
// commands, four of which enumerate radios. Re-running it per poll would spend
// the box's CPU redrawing a picture that changes when a cable moves.
func (s *Service) cachedDetection(ctx context.Context) panel.Detection {
	s.mu.RLock()
	d, at := s.lastDetect, s.lastDetectAt
	s.mu.RUnlock()
	if !at.IsZero() && s.cfg.Now().Sub(at) < s.cfg.DetectTTL {
		return d
	}
	fresh, err := s.detect(ctx)
	if err != nil {
		// A status poll must still answer when detection fails, or the panel
		// cannot draw at the moment it is most needed. The stale picture is
		// returned with the fault attached rather than nothing at all.
		d.Fault = faultOf(err)
		return d
	}
	return fresh
}

// Status reports what is running now. It changes nothing.
func (s *Service) Status(ctx context.Context) (panel.SystemStatus, error) {
	st := panel.SystemStatus{
		Engine:    s.cfg.Engine.State(),
		Detection: s.cachedDetection(ctx),
		At:        s.cfg.Now(),
		// Without this the panel shows a green light over a box that forwards
		// nothing: panel.SystemStatus.Connected reads it, and the control has
		// no other way to know which state it is in.
		ClientTrafficCut: s.ClientTrafficCut(),
	}

	s.mu.RLock()
	plan := s.plan
	hp := s.hotspotPlan
	running := s.running
	s.mu.RUnlock()

	iface := st.Detection.HotspotInterface
	if plan != nil {
		iface = plan.Hotspot
	}
	if iface == "" {
		// Nothing has been planned and nothing detected an access point, so
		// there is no interface to ask about. Saying so beats asking
		// hostapd_cli about an interface called "".
		//
		// The fault is the DETECTION's when detection had one. Reporting
		// "no adapter can create a hotspot" on a machine where detection
		// itself could not run would send the user to buy a USB adapter for a
		// missing program.
		st.Hotspot = panel.HotspotStatus{Fault: st.Detection.Fault}
		if st.Hotspot.Fault == panel.FaultNone {
			st.Hotspot.Fault = panel.FaultNoAPAdapter
		}
		return st, nil
	}

	hs, err := s.sup.Status(ctx, iface)
	if err != nil {
		st.Hotspot = panel.HotspotStatus{Fault: faultOf(err)}
		return st, nil
	}
	st.Hotspot = panel.HotspotStatus{
		Running:              hs.Running,
		Devices:              hs.DeviceCount(),
		UnreadableLeaseLines: hs.MalformedLeaseLines,
	}
	if running {
		st.Hotspot.SSID = hp.AP.SSID

		// The kernel's own view, and it costs nothing. hostapd_cli reporting
		// state=ENABLED is the same class of evidence as a live process: on
		// the target on 2026-08-30 the process was alive, the panel showed
		// connected, and the radio was a station on the house network. The
		// start sequence reads the interface back for exactly that reason and
		// this is the steady-state half of it.
		//
		// MEASURED this session rather than estimated, because the obvious
		// implementation is to run "iw dev" per poll and that is the wrong
		// trade: a steady-state poll costs 0 netcfg commands and 2 subprocess
		// spawns (rfkill, hostapd_cli), the browser polls every 5000 ms
		// (internal/panel/assets/panel.js), and "iw dev" is ALREADY run, as
		// one of the 9 commands a detection runs at most once per DetectTTL.
		// So the bytes are collected already and the gap was that this
		// function did not look at them. Reading them here adds no process and
		// makes the answer at most DetectTTL stale, which is right for a
		// health indicator when the start sequence holds the fresh check.
		if onAir, known := s.hotspotOnTheAir(iface, hp.AP.SSID); known && !onAir {
			hs.Running = false
			st.Hotspot.Running = false
			hs.Reason = "The hotspot software is running and the wireless adapter is not broadcasting this network."
		}
	}
	// A fault ONLY when the box is meant to be on. A hotspot that is down
	// because nobody has pressed the switch is not a failure, and reporting
	// one would put an error on the panel of a box that is working exactly as
	// asked. internal/panel/priv.go: Fault is "why it is not running, when it
	// is not", and "nobody turned it on" is not a why the user can act on.
	if running && !hs.Running {
		st.Hotspot.Fault = hotspotFault(unitAP, hs.Reason, nil)
	}
	return st, nil
}

// hotspotOnTheAir answers, from the detection this poll already holds, whether
// the hotspot interface is an access point broadcasting the expected name.
//
// The second return is whether the question could be answered at all. A
// detection that has nothing for this interface says nothing about it, and
// silence must not be read as "it is not on the air": that would put a fault
// on the panel of a working box every time a detection failed. Only a reading
// that actually names the interface is allowed to settle it.
//
// The predicate is netcfg's IsAccessPoint and not a second definition of what
// an access point is.
func (s *Service) hotspotOnTheAir(iface, ssid string) (onAir bool, known bool) {
	if iface == "" {
		return false, false
	}
	s.mu.RLock()
	facts := s.facts
	s.mu.RUnlock()

	w, ok := facts.WirelessByName(iface)
	if !ok {
		return false, false
	}
	return w.IsAccessPoint() && (ssid == "" || w.SSID == ssid), true
}

// EngineLog returns the recent diagnostic output for the advanced view.
//
// It is TWO rings merged, not one, and the second is the reason this method is
// worth reading. internal/engine's ring holds what the engine said. diagRing
// holds what THIS service said: which step failed, with the command and the
// error, and what a fallback cost. A start that fails before the engine is
// reached produces nothing at all in the first ring, which is exactly the
// event that most needs a line in the advanced view.
//
// Both are redacted on the way in by the ring that holds them, so nothing here
// redacts and nothing here can forget to.
//
// The two are interleaved by time rather than concatenated, because the whole
// value of the merge is reading one sequence: the engine refusing a config and
// this service tearing the box back down are one event, and reading them as
// two lists means working out the order by hand.
func (s *Service) EngineLog(_ context.Context) (panel.EngineLog, error) {
	engineLines := s.cfg.Engine.Logs()
	diagLines := s.diag.entries()

	merged := make([]engine.LogEntry, 0, len(engineLines)+len(diagLines))
	merged = append(merged, engineLines...)
	merged = append(merged, diagLines...)
	sort.SliceStable(merged, func(i, k int) bool { return merged[i].At.Before(merged[k].At) })

	return panel.EngineLog{
		Entries: merged,
		// Summed, because the panel says "you are looking at the last N of M"
		// and a count from one of two rings would understate what was lost.
		Dropped: s.cfg.Engine.LogsDropped() + s.diag.droppedCount(),
	}, nil
}

// note records something an operator should know that did not stop the start.
//
// It goes to the log AND to the advanced view. The log is for whoever is
// helping; the advanced view is for the person at the box, who has no other
// way to see it. internal/netcfg produces these as plan notes, and the one
// that matters most is the sentence explaining that the hotspot took over an
// existing WiFi connection and ended it: without it somebody is left wondering
// why their other connection dropped.
func (s *Service) note(text string) {
	s.cfg.Logger.Warn("network plan note", "note", text)
	s.diag.add(text)
}

// recordFailure reports a failure that is not a network step: the engine
// refusing a configuration, or the access point not coming up.
//
// It exists for the same reason recordStepFailure does, one layer over.
// internal/hotspot has already turned the daemon's own output into a sentence
// (status.go, explainFailure), and this service was using that sentence ONLY
// to pick a Fault and then dropping it, so the one fact that makes a hostapd
// failure diagnosable reached neither the log nor the advanced view. That is
// the same shape as the defect a real start exposed on 2026-08-30, and leaving
// it would mean the next real failure reproduced it exactly.
//
// internal/engine's errors are already redacted by that package. Redact is
// documented idempotent, so running it again here costs nothing and means this
// function does not have to know which path its caller came from.
func (s *Service) recordFailure(what, detail string, err error) {
	if err != nil {
		if detail != "" {
			detail += ": "
		}
		detail += err.Error()
	}
	if detail == "" {
		return
	}
	detail = redactedText(detail, nil)
	s.cfg.Logger.Error(what, "detail", detail)
	s.diag.add(what + ": " + detail)
}

// recordStepFailure reports a step that failed, with the command and the
// error, to the log and to the advanced view.
//
// This is the gap a real start on the target exposed on 2026-08-30: the fault
// reached the panel as "unknown", the advanced view had nothing, and the log
// had two lines that named neither the command nor the error. The command is
// what made the failure diagnosable.
func (s *Service) recordStepFailure(where string, servers []netip.Addr, rep netcfg.Report, err error) {
	step, ok := rep.FailedStep()
	if !ok {
		s.cfg.Logger.Error("a step failed and the report does not say which",
			"where", where, "error", redactedText(err.Error(), servers))
		s.diag.addf("%s failed and the report does not say which step", where)
		return
	}
	cmd := redactedCommand(step.Do, servers)
	cause := redactedText(err.Error(), servers)

	s.cfg.Logger.Error("a network step failed",
		"where", where, "op", step.Op, "command", cmd, "error", cause, "why", step.Why)
	s.diag.addf("%s failed: %s (%s)", step.Op, cmd, cause)
}

// regulatoryDomain reads the country the radio is operating under.
//
// hostapd refuses to beacon without a country_code in most of the world, and
// internal/hotspot refuses to render a configuration without one, so this value
// has to come from somewhere. It comes from the radio through "iw", which is
// already on internal/netcfg's binary allowlist, rather than from a constant.
//
// "00" is the world domain, which is what a radio reports when nothing has set
// a country. It is treated as "not known" rather than as a country, because
// beaconing is not permitted on most channels there and passing it through
// would produce a hotspot that never appears.
func (s *Service) regulatoryDomain(ctx context.Context) string {
	res, err := s.cfg.Runner.Run(ctx, netcfg.Command{
		Path: netcfg.BinIw, Args: []string{"reg", "get"},
		Why: "the regulatory domain, which hostapd needs before it will beacon on any channel",
	})
	if err != nil {
		return s.cfg.Country
	}
	if cc, ok := parseRegDomain(res.Stdout); ok {
		return cc
	}
	return s.cfg.Country
}

// isRunning reports whether a start has been applied and not undone.
func (s *Service) isRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Applied reports the fingerprint of the request currently in force, or the
// empty string. It is here so a caller can log which configuration is running
// without holding any part of it.
func (s *Service) Applied() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fingerprint
}

// Close releases what the service holds without undoing anything on the
// machine. Stop is what undoes things; this is for a process that is exiting
// and wants the journal file handle back.
func (s *Service) Close() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	ap := s.applier
	s.applier = nil
	s.mu.Unlock()
	if ap == nil {
		return nil
	}
	if err := ap.Close(); err != nil {
		return fmt.Errorf("privsvc: closing the journal: %w", err)
	}
	return nil
}
