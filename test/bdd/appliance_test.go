// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/state"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// ---------------------------------------------------------------------------
// The appliance: the order the packages are used in.
//
// READ THIS BEFORE TRUSTING A GREEN RUN. There is no orchestration layer this
// suite can call. Checked on 2026-08-30: cmd/caspian is an empty directory, and
// internal/privsvc, which appeared while this file was being written, does not
// compile and has no Start method ("*Service does not implement
// panel.Privileged (missing method Start)", service.go:47). So the sequence
// below is not a call into shipped code. It is this suite's statement of what
// the sequence has to be, assembled from what each package's documentation says
// about its own place in it.
//
// That makes these tests weaker than they look in one specific way, and
// stronger than they look in another. Weaker: a green run does not prove the
// shipped product does this, because there is no shipped product to do it.
// Stronger: whatever ends up owning this sequence has to match it, and the
// moment internal/privsvc builds and implements Start, this file should shrink
// to a thin call into it and these scenarios should drive that instead. Until
// then this is the specification the privileged service has to satisfy, in
// executable form, with the reason for each ordering constraint written beside
// it and cited to the package that imposes it.
//
// The orderings that are not preferences:
//
//	recovery before anything      netcfg/apply.go Recover: "recovery happens
//	                              before the new plan is applied, not after"
//	firewall before forwarding    netcfg/route.go PreEngineSteps: "so there is
//	                              never a moment when forwarding is enabled and
//	                              the block is not"
//	conf.default before engine    netcfg/doc.go: rp_filter's default is
//	                              inherited at interface creation, and the
//	                              engine creates the tunnel device
//	pinned host route before      netcfg/route.go ServerRouteSteps: without it
//	engine                        the engine's own connection to the server
//	                              matches the default route through the tunnel
//	post-engine steps after       netcfg/route.go PostEngineSteps: "Every
//	engine                        command here names that device"
// ---------------------------------------------------------------------------

// connect is what pressing the switch does.
func (w *World) connect() error {
	// -----------------------------------------------------------------------
	// 0. Clean up after a previous run that was killed.
	//
	// A journal on disk means a process died between writing an inverse and
	// running the change, or after running it and before undoing it. Replaying
	// it first is not tidying: the plan about to be applied assumes the
	// machine is in the state detection found it in, and a leftover policy
	// rule or half default route makes that false.
	// -----------------------------------------------------------------------
	if !w.defs.skipRecovery {
		rep, err := netcfg.Recover(w.ctx, w.tracedNetRunner(), w.journalPath())
		w.recovered = rep
		if err != nil {
			return w.classify(panel.StageNone, w.fail(fmt.Errorf("clean up the previous run: %w", err)))
		}
		if len(rep.Results) > 0 {
			w.event(fmt.Sprintf("recovery: replayed %d inverses from a journal left by a killed process", len(rep.Results)))
			w.note("recovered %d changes left by a previous run", len(rep.Results))
		}
	}

	// A deliberate defect, not a branch of the design: touching the machine
	// before the pasted text has been read at all.
	if w.defs.detectBeforeParse {
		if _, _, err := netcfg.DetectAndPlan(w.ctx, w.tracedNetRunner(),
			[]netip.Addr{netip.MustParseAddr("203.0.113.10")}, netcfg.DefaultOptions()); err != nil {
			return w.fail(err)
		}
	}

	// -----------------------------------------------------------------------
	// 1. Read what the user pasted. Nothing on the machine is touched until
	//    this succeeds: a box that reconfigured its firewall and then said "I
	//    could not read that link" has changed the network for nothing.
	// -----------------------------------------------------------------------
	l, err := link.Parse(w.pasted)
	if err != nil {
		return w.classify(panel.StageParse, w.fail(err))
	}
	w.lnk = l
	w.note("config accepted: %s", l.Redacted())

	// -----------------------------------------------------------------------
	// 2. Persist it. internal/state is the only writer of configuration, and
	//    it holds the credential at 0600 inside a 0700 directory.
	// -----------------------------------------------------------------------
	if err := w.store.SetProxyConfig(w.pasted, l.Protocol, l.Tag); err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	if err := w.store.SetHotspot(hotspotSSID, fakeHotspotPassphrase); err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	w.note("state: %s", w.store.Snapshot().Redacted())

	// -----------------------------------------------------------------------
	// 3. Resolve the server. The pinned host route is part of the plan, so the
	//    address has to be known before the plan is made, which means before
	//    the tunnel exists. See fakeResolver for what that costs.
	// -----------------------------------------------------------------------
	servers, err := w.resolve.Resolve(w.ctx, l.Address)
	if err != nil {
		return w.classify(panel.StageServer, w.fail(err))
	}

	// -----------------------------------------------------------------------
	// 4. Compose the engine's configuration, and ask the engine whether it
	//    will take it BEFORE touching the machine. This is the second of the
	//    design's three failure states (section 8, step 11) and it has to be
	//    reached without having reconfigured anything.
	// -----------------------------------------------------------------------
	opts := xcfg.Defaults()
	opts.Link = l
	// The TUN inbound is off. On a developer machine there is no /dev/net/tun
	// and no root, so a document carrying it would not start. What that costs
	// is stated in doc.go and again at the engine start below: the tunnel
	// device is never really created here, so the ordering assertion around it
	// is against a marker this file emits, not against a device appearing.
	opts.TUN.Disabled = true
	opts.SOCKS.Port = w.socksAt
	// The listener internal/hotspot's dnsmasq forwards to. Enabling it is what
	// makes client DNS resolvable at all, and it is the same field the hotspot
	// plan below takes its forwarding target from.
	opts.LocalDNS.Enabled = true
	opts.LocalDNS.Port = w.localDNSAt

	cfg, err := xcfg.Build(opts)
	if err != nil {
		return w.classify(panel.StageEngine, w.fail(err))
	}
	w.engineCfg = w.defs.mutateEngineConfig(cfg)

	if err := engine.Validate(cfg); err != nil {
		return w.classify(panel.StageEngine, w.fail(err))
	}

	// -----------------------------------------------------------------------
	// 5. Look at the machine and decide. Detection reads the kernel knobs a
	//    second time once the plan has named the interfaces, so every change
	//    has an exact inverse rather than a guessed one.
	// -----------------------------------------------------------------------
	netOpts := netcfg.DefaultOptions()
	if w.defs.collideTheHotspotSubnet {
		// An advanced-mode override that collides. netcfg reports it as a note
		// rather than refusing, which is the right call for an override, and
		// leaves whoever set it to notice.
		netOpts.HotspotSubnet = netip.MustParsePrefix("192.168.1.0/24")
	}
	facts, plan, err := netcfg.DetectAndPlan(w.ctx, w.tracedNetRunner(), servers, netOpts)
	if err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	w.facts, w.plan = facts, plan
	w.note("detected: %s", plan.Explain())
	for _, n := range plan.Notes {
		w.note("note: %s", n)
	}

	w.preSteps = plan.PreEngineSteps(facts.Sysctl)
	w.postSteps = plan.PostEngineSteps(facts.Sysctl)
	if w.defs.firewallAfterForwarding {
		w.preSteps = moveFirewallLast(w.preSteps)
	}
	if w.defs.dropPinnedServerRoute {
		w.preSteps = dropHostRoutes(w.preSteps)
	}
	if w.defs.skipTeardownOfRoutes {
		w.postSteps = nil
		defer w.applyOutsideTheJournal(plan.TunnelRouteSteps())
	}

	// -----------------------------------------------------------------------
	// 6. Apply, journalling the inverse of every change before making it.
	// -----------------------------------------------------------------------
	ap, err := netcfg.NewApplier(w.tracedNetRunner(), w.journalPath())
	if err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	w.applier = ap

	if w.defs.postEngineStepsBeforeEngine {
		if _, err := ap.Apply(w.ctx, w.postSteps); err != nil {
			return w.classify(panel.StageNone, w.fail(err))
		}
		w.postSteps = nil
	}

	if _, err := ap.Apply(w.ctx, w.preSteps); err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	w.rulesetInForce = w.defs.mutateRuleset(plan.Ruleset())

	// -----------------------------------------------------------------------
	// 7. Start the engine. On the appliance this is the moment the tunnel
	//    device is created; here the inbound that would create it is switched
	//    off, so the line below is a MARKER standing for that moment and not
	//    an observation of it. Everything after it in the timeline is a step
	//    that would have failed if run before it.
	// -----------------------------------------------------------------------
	// One Engine for the life of the box, not one per connect. Engine.Start is
	// idempotent and a second Start on a running engine does nothing; building
	// a second Engine here would hide that and leave two instances.
	if w.eng == nil {
		w.eng = engine.NewWithLogCapacity(64)
	}
	if err := w.eng.Start(w.ctx, cfg); err != nil {
		return w.classify(panel.StageEngine, w.fail(err))
	}
	w.event("engine: started; the tunnel device " + plan.Tun + " exists from here on")

	// -----------------------------------------------------------------------
	// 8. The steps that name the tunnel device.
	// -----------------------------------------------------------------------
	if _, err := ap.Apply(w.ctx, w.postSteps); err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}

	// -----------------------------------------------------------------------
	// 9. The access point and its DHCP and DNS server.
	// -----------------------------------------------------------------------
	hp, err := w.hotspotPlanFor(plan, facts)
	if err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	if w.defs.restartHotspotEveryConnect {
		// A configuration that differs on every start is the defect this
		// scenario exists to catch: writeIfChanged sees a change, and the
		// supervisor restarts a working access point, dropping every device.
		hp.HostapdConf += fmt.Sprintf("\n# regenerated at %d\n", time.Now().UnixNano())
	}
	w.hotspotPlan = hp

	if w.supervis == nil {
		w.supervis = hotspot.NewSupervisor(w.tracedHotspotSystem(), w.hotspotPaths())
	}
	st, err := w.supervis.Start(w.ctx, hp)
	w.hotspotStat = st
	if err != nil {
		return w.classify(panel.StageNone, w.fail(err))
	}
	if !st.Running {
		return w.classify(panel.StageNone, w.fail(errors.New(st.Reason)))
	}
	w.event("hotspot: beaconing on " + hp.AP.Interface)

	// -----------------------------------------------------------------------
	// 10. Only now ask whether the server is there. The three failure states
	//     the panel has to tell apart are ordered: the text, the engine, then
	//     the server. Asking the server first would blame a config that never
	//     had a chance to be tried.
	// -----------------------------------------------------------------------
	if err := w.server.Probe(w.ctx); err != nil {
		return w.classify(panel.StageServer, w.fail(err))
	}

	w.detection = w.detectionFor(plan, facts)
	w.status = panel.SystemStatus{
		Engine: w.eng.State(),
		Hotspot: panel.HotspotStatus{
			Running:              st.Running,
			SSID:                 hp.AP.SSID,
			Devices:              st.DeviceCount(),
			UnreadableLeaseLines: st.MalformedLeaseLines,
		},
		Detection: w.detection,
		At:        time.Now(),
	}
	return nil
}

// disconnect is what turning the switch off does: stop what was started, then
// replay the inverse of every change, newest first.
func (w *World) disconnect() error {
	if w.eng != nil {
		if err := w.eng.Stop(); err != nil {
			return w.fail(err)
		}
		w.event("engine: stopped; the tunnel device is gone from here on")
	}
	if w.supervis != nil {
		if err := w.supervis.Stop(w.ctx); err != nil {
			return w.fail(err)
		}
		w.event("hotspot: stopped")
	}
	if w.applier == nil {
		return nil
	}
	rep, err := w.applier.Teardown(w.ctx)
	w.teardown = rep
	if err != nil {
		return w.fail(err)
	}
	return nil
}

// classify turns a failure into the words the panel shows. The stage is the
// distinction the design demands (section 8, step 11): the three states need
// three different actions from the user, and one "it did not work" makes all
// three look like the same problem.
func (w *World) classify(stage panel.ConfigStage, err error) error {
	if err == nil {
		return nil
	}
	if w.defs.classifyEveryFailureAsParse {
		stage = panel.StageParse
	}
	if w.defs.swallowRefusals {
		// The shape of this defect matters: the message is not lost, the TYPE
		// is. A panel that has only a sentence cannot branch on the reason, and
		// a *netcfg.PlanError carries wording written for this audience that a
		// generic wrap throws away.
		err = fmt.Errorf("could not set the box up: %s", err.Error())
	}
	switch stage {
	case panel.StageParse:
		w.problem = panel.ParseProblem(err)
	case panel.StageEngine:
		w.problem = panel.EngineProblem()
	case panel.StageServer:
		w.problem = panel.ServerProblem()
	default:
		w.problem = panel.StartProblem(panel.FaultOf(err))
	}
	w.note("could not connect: %s", w.problem.Text())
	w.connectErr = err
	return err
}

// ---------------------------------------------------------------------------
// Building the hotspot from the network plan
// ---------------------------------------------------------------------------

const hotspotSSID = "Caspian-Living-Room"

func (w *World) hotspotPaths() hotspot.Paths {
	p := hotspot.DefaultPaths()
	p.HostapdConf = filepath.Join(w.dir, "hostapd.conf")
	p.DnsmasqConf = filepath.Join(w.dir, "dnsmasq.conf")
	p.HostapdPID = filepath.Join(w.dir, "hostapd.pid")
	p.DnsmasqPID = filepath.Join(w.dir, "dnsmasq.pid")
	p.LeaseFile = filepath.Join(w.dir, "dnsmasq.leases")
	p.StateDir = w.dir
	return p
}

// hotspotPlanFor turns the network plan into an access point. Nothing here is
// invented: the interface, the channel and the subnet all come from the plan,
// and the radio's limits are read back out of the facts rather than assumed.
func (w *World) hotspotPlanFor(p *netcfg.Plan, f netcfg.Facts) (hotspot.Plan, error) {
	band := hotspot.Band2GHz
	if p.Channel > 14 {
		band = hotspot.Band5GHz
	}

	stored := w.store.Hotspot()
	ap := hotspot.APConfig{
		Interface:   p.Hotspot,
		SSID:        stored.SSID,
		Passphrase:  stored.Passphrase.Reveal(),
		CountryCode: "GB",
		Channel:     p.Channel,
		Band:        band,
		ControlDir:  w.hotspotPaths().HostapdControlDir,
	}

	phy, _ := f.PhyByName(p.HotspotPhy)
	_, combo := phy.APWithStation()
	maxAPs := 0
	for _, lim := range combo.Limits {
		if lim.Has("AP") {
			maxAPs = lim.Max
		}
	}
	rc := hotspot.RadioConstraint{
		SupportsAP:      phy.SupportsAP(),
		MaxAPs:          maxAPs,
		MaxChannels:     combo.Channels,
		AllowedChannels: phy.UsableChannels(),
	}
	if p.ChannelPinned {
		rc.ClientChannel = p.Channel
	}

	start, err := nthAddress(p.HotspotSubnet, 50)
	if err != nil {
		return hotspot.Plan{}, err
	}
	end, err := nthAddress(p.HotspotSubnet, 200)
	if err != nil {
		return hotspot.Plan{}, err
	}
	dns := hotspot.DNSConfig{
		Interface:  p.Hotspot,
		Subnet:     p.HotspotSubnet,
		Gateway:    p.HotspotGateway,
		RangeStart: start,
		RangeEnd:   end,
		LeaseTime:  12 * time.Hour,
		LeaseFile:  w.hotspotPaths().LeaseFile,
		// The resolver on this box. It must be a loopback address: a
		// forwarding target anywhere else is every client's every lookup
		// leaving outside the tunnel.
		//
		// It is w.localDNSAt, the SAME field the engine's listener above is
		// bound to, because that is how internal/privsvc wires it and a suite
		// that gave the two halves separate numbers could not have noticed
		// them drifting apart. It used to be the literal 15353 against a
		// listener that was never enabled.
		Upstream:  netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), w.localDNSAt),
		CacheSize: 150,
	}
	return hotspot.NewPlan(ap, dns, rc)
}

// nthAddress returns the nth host address inside a prefix.
func nthAddress(p netip.Prefix, n int) (netip.Addr, error) {
	a := p.Masked().Addr()
	for i := 0; i < n; i++ {
		a = a.Next()
		if !a.IsValid() || !p.Contains(a) {
			return netip.Addr{}, fmt.Errorf("%v has no address number %d", p, n)
		}
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// What the panel is told
// ---------------------------------------------------------------------------

func (w *World) detectionFor(p *netcfg.Plan, f netcfg.Facts) panel.Detection {
	var ifaces []panel.InterfaceInfo
	seen := map[string]bool{}
	add := func(i panel.InterfaceInfo) {
		if i.Name == "" || seen[i.Name] {
			return
		}
		seen[i.Name] = true
		ifaces = append(ifaces, i)
	}

	for _, l := range f.Links {
		if l.IsLoopback() {
			continue
		}
		info := panel.InterfaceInfo{Name: l.Name, Kind: panel.KindEthernet}
		if wi, ok := f.WirelessByName(l.Name); ok {
			info.Kind = wifiKind(l.Bus)
			if phy, ok := f.PhyByName(wi.Phy); ok {
				info.CanHostAP = phy.SupportsAP()
			}
		}
		if def, ok := f.PrimaryDefault(); ok && def.Dev == l.Name {
			info.HasDefaultRoute = true
		}
		add(info)
	}

	// The access point's own interface does not exist until it is created, so
	// it is not in the link list. It is still the interface the panel has to
	// name, and its kind is the kind of the radio it sits on.
	if p.HotspotIsVirtual {
		kind := panel.KindWiFi
		if parent, ok := f.LinkByName(p.HotspotParent); ok {
			kind = wifiKind(parent.Bus)
		}
		add(panel.InterfaceInfo{Name: p.Hotspot, Kind: kind, CanHostAP: true})
	}

	band := "2.4"
	if p.Channel > 14 {
		band = "5"
	}
	return panel.Detection{
		InternetInterface: p.Uplink,
		HotspotInterface:  p.Hotspot,
		Interfaces:        ifaces,
		Channel:           p.Channel,
		Band:              band,
		Country:           "GB",
		UsableChannels:    p.UsableChannel,
		ChannelPinned:     p.ChannelPinned,
		Subnet:            p.HotspotSubnet.String(),
		HotspotAddress:    p.HotspotGateway.String(),
		// The address the box holds on the network it is attached to. The panel
		// binds to it only when the user has turned that on, and only when it is
		// private; see thePanelIsServedOnTheHotspotAndNeverOnAPublicAddress.
		LocalNetworkAddress: firstIPv4On(f, p.Uplink),
		At:                  time.Now(),
	}
}

// firstIPv4On returns the box's own IPv4 address on an interface, if it has one.
func firstIPv4On(f netcfg.Facts, iface string) string {
	l, ok := f.LinkByName(iface)
	if !ok {
		return ""
	}
	for _, pfx := range l.Prefixes {
		if pfx.Addr().Is4() {
			return pfx.Addr().String()
		}
	}
	return ""
}

// wifiKind reads the parent bus. iproute2 reports it only on some kernels, so
// an empty string means "not reported" and never "not USB"; the fallback is
// the generic word rather than a guess at which radio it is.
func wifiKind(bus string) panel.InterfaceKind {
	switch bus {
	case "usb":
		return panel.KindUSBWiFi
	case "":
		return panel.KindWiFi
	default:
		return panel.KindBuiltinWiFi
	}
}

// ---------------------------------------------------------------------------
// Defect helpers. Each one damages the composition in a way a careless change
// to the real orchestrator would.
// ---------------------------------------------------------------------------

// moveFirewallLast puts the nft load after everything else in the pre-engine
// list, which is the window PreEngineSteps exists to close: forwarding on, and
// the block not yet in force.
func moveFirewallLast(steps []netcfg.Step) []netcfg.Step {
	var fw []netcfg.Step
	var rest []netcfg.Step
	for _, s := range steps {
		if s.Op == netcfg.OpNft {
			fw = append(fw, s)
			continue
		}
		rest = append(rest, s)
	}
	return append(rest, fw...)
}

// dropHostRoutes removes the pinned host route to the server, which is the
// route without which the engine's own connection loops through the tunnel it
// is trying to build.
func dropHostRoutes(steps []netcfg.Step) []netcfg.Step {
	var out []netcfg.Step
	for _, s := range steps {
		if s.Op == netcfg.OpRoute && containsArg(s.Do.Args, "/32") {
			continue
		}
		out = append(out, s)
	}
	return out
}

func containsArg(args []string, suffix string) bool {
	for _, a := range args {
		if len(a) >= len(suffix) && a[len(a)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// applyOutsideTheJournal runs steps straight at the runner, so the machine
// changes and nothing records how to change it back. It is the shape of a
// change somebody adds in a hurry, and the teardown scenario has to catch it.
func (w *World) applyOutsideTheJournal(steps []netcfg.Step) {
	for _, s := range steps {
		if s.Do.IsZero() {
			continue
		}
		_, _ = w.tracedNetRunner().Run(w.ctx, s.Do)
	}
}

// storeFingerprint is the eight hex digits that identify a stored config
// without disclosing any part of it. Used by the secret scan to prove the
// diagnostic identifier is what is logged rather than the config.
func (w *World) storeFingerprint() string {
	var p state.ProxyConfig = w.store.Proxy()
	return p.Fingerprint()
}
