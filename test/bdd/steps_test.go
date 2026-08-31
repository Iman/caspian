// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// ---------------------------------------------------------------------------
// Given: the box, and what the user has.
// ---------------------------------------------------------------------------

// aFreshBox is a machine that has never run this program: no saved config, no
// journal of applied changes, nothing running.
func aFreshBox(w *World) error {
	if !w.store.FirstRun() {
		return fmt.Errorf("the state store does not report a first run, so this box is not fresh")
	}
	if _, err := os.Stat(w.journalPath()); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("a journal of applied changes already exists at %s", w.journalPath())
	}
	if len(w.tl.all()) != 0 {
		return fmt.Errorf("something has already touched this machine: %v", w.tl.all())
	}
	return nil
}

// aBoxWhoseOnlyRadioCannotCreateAHotspot is the machine section 5.2 of the
// design names: the one where the panel has to say what to go and buy.
func aBoxWhoseOnlyRadioCannotCreateAHotspot(w *World) error {
	w.iwList = fixtureIWListNoAP
	w.primeMachine()
	return nil
}

// aBoxWithNoInternetConnectionOfItsOwn has no default route, so there is
// nothing to share.
func aBoxWithNoInternetConnectionOfItsOwn(w *World) error {
	w.runner.SetOutput("ip route show default", "")
	w.runner.SetOutput("ip -6 route show default", "")
	return nil
}

// aBoxLeftHalfConfiguredByAKilledProcess writes the journal a process leaves
// when it dies between recording an inverse and running the change. That is
// the exact case the on-disk record exists for: the inverse reaches the disk
// before the change reaches the kernel, so the entry is present whether or not
// the change ever happened.
func aBoxLeftHalfConfiguredByAKilledProcess(w *World) error {
	j, err := netcfg.OpenJournal(w.journalPath())
	if err != nil {
		return err
	}
	leftover := netcfg.Step{
		Op:  netcfg.OpRule,
		Why: "written by a run that was killed before it could finish",
		Do: netcfg.Command{Path: netcfg.BinIP, Args: []string{
			"rule", "add", "from", "10.83.51.0/24", "lookup", "8410", "priority", "8410"}},
		Undo: netcfg.Command{Path: netcfg.BinIP, Args: []string{
			"rule", "del", "from", "10.83.51.0/24", "lookup", "8410", "priority", "8410"}},
	}
	if _, err := j.Begin(leftover); err != nil {
		return err
	}
	w.leftover = netcfg.RunnerKey(leftover.Undo)
	return j.Close()
}

// aValidRealityLink is the share link the user was given.
func aValidRealityLink(w *World) error {
	w.pasted = realityShareLink()
	return nil
}

// textThatIsNotALinkAtAll is what a user pastes when they copy the message
// around the link instead of the link.
func textThatIsNotALinkAtAll(w *World) error {
	w.pasted = "here you go, this is the one for the living room box"
	return nil
}

// aLinkTheEngineWillNotAccept parses cleanly and is refused by the engine.
// insecure=1 is the common shape for a self-signed server and xray-core
// v1.260327.0 removed the option it maps to.
func aLinkTheEngineWillNotAccept(w *World) error {
	if !time.Now().After(allowInsecureGate) {
		return fmt.Errorf(
			"this machine's clock reads %s, before the %s gate on which the engine starts refusing "+
				"allowInsecure, so this scenario cannot distinguish anything. That clock dependency is "+
				"the trap itself: config validity is not a property of the config alone",
			time.Now().UTC().Format(time.RFC3339), allowInsecureGate.Format("2006-01-02"))
	}
	w.pasted = insecureShareLink()
	return nil
}

// aProxyServerThatNeverAnswers is the third failure state: the text is fine,
// the engine took it, and nothing is listening at the other end.
func aProxyServerThatNeverAnswers(w *World) error {
	w.server = fakeServer{answers: false}
	return nil
}

// aBoxThatIsAlreadyConnected has been through the whole flow once.
func aBoxThatIsAlreadyConnected(w *World) error {
	w.pasted = realityShareLink()
	if err := w.connect(); err != nil {
		return fmt.Errorf("the first connect did not succeed, so nothing after it means anything: %w", err)
	}
	w.engineRunningSince = w.eng.State().Since
	w.hostapdStarts = w.sys.CountCalls(w.hotspotPaths().HostapdBinary)
	w.dnsmasqStarts = w.sys.CountCalls(w.hotspotPaths().DnsmasqBinary)
	return nil
}

// ---------------------------------------------------------------------------
// When: what the person at the box does.
// ---------------------------------------------------------------------------

// theUserPressesConnect runs the whole flow. It does not fail the scenario when
// the flow fails: whether the failure was right is what the Then steps decide.
func theUserPressesConnect(w *World) error {
	_ = w.connect()
	return nil
}

// theUserPressesConnectAgain is the same switch, pressed twice.
func theUserPressesConnectAgain(w *World) error {
	_ = w.connect()
	return nil
}

// theUserPressesDisconnect turns the switch off, which has to return the
// machine to how it was found.
func theUserPressesDisconnect(w *World) error {
	if err := w.disconnect(); err != nil {
		return fmt.Errorf("turning the switch off failed: %w", err)
	}
	return nil
}

// theTunnelDrops is the engine going away without anybody asking it to. The
// kernel withdraws every route through the tunnel device at this moment, which
// is why "the engine stopped" leaks by default rather than stopping.
func theTunnelDrops(w *World) error {
	if w.eng == nil {
		return errors.New("there is no engine to drop")
	}
	before := w.tl.lastIndexOf("net: nft")
	if err := w.eng.Stop(); err != nil {
		return err
	}
	w.event("engine: stopped; the tunnel device is gone from here on")
	if after := w.tl.lastIndexOf("net: nft"); after != before {
		return errors.New("the firewall was reloaded when the tunnel dropped, so the ruleset in force is not the one that was asserted")
	}
	return nil
}

// theProcessIsKilledAndTheBoxRestarts drops everything in memory without a
// teardown, which is what a power cut or a SIGKILL does. The journal on disk is
// all that survives.
func theProcessIsKilledAndTheBoxRestarts(w *World) error {
	if w.applier == nil {
		return errors.New("nothing was applied, so there is nothing to be killed in the middle of")
	}
	if err := w.applier.Close(); err != nil {
		return err
	}
	w.applier = nil
	// The engine runs inside this process, so killing the process takes it, and
	// the tunnel device it made, with it. The hotspot does not go: both daemons
	// were started detached with a pid file precisely so that this program
	// restarting does not take the hotspot down with it.
	if w.eng != nil {
		_ = w.eng.Stop()
		w.event("engine: stopped; the tunnel device is gone from here on")
		w.eng = nil
	}
	w.supervis = nil
	w.event("process: killed; only the journal on disk survives")

	rep, err := netcfg.Recover(w.ctx, w.tracedNetRunner(), w.journalPath())
	w.recovered = rep
	if err != nil {
		return fmt.Errorf("the restarted process could not replay the journal: %w", err)
	}
	return nil
}

// theInternetMovesToADifferentInterface is the hazard the design lists and
// nothing fails loudly for: the cable is unplugged and WiFi takes over, or DHCP
// renews with a different gateway. The pinned host route still exists, still
// points at an address, and that address is no longer a way to the server.
//
// NOTHING IS DONE TO THE BOX HERE, and that is the point of this step.
//
// Until 2026-08-30 this function called Plan.RederiveForUplink and applied the
// result itself, and the assertions then checked that the routes had moved. It
// was the test performing the behaviour it was testing for. internal/netcfg can
// move the routes when something asks it to; nothing in the shipped appliance
// ever asks. netcfg.WatchUplink, netcfg.ReadUplinkState and
// Plan.RederiveForUplink have no caller outside this file, which
// TestNothingInTheApplianceWatchesTheUplink pins.
//
// So this step now does what the world does and no more: the machine's default
// route moves, and the appliance is not told.
func theInternetMovesToADifferentInterface(w *World) error {
	if w.plan == nil || w.applier == nil {
		return errors.New("nothing is set up, so there is no uplink to move")
	}
	was := w.plan.UplinkState()
	now := netcfg.UplinkState{
		Interface: "wlan0",
		Gateway:   netip.MustParseAddr("192.168.1.1"),
	}
	changed, reason := netcfg.UplinkChanged(was, now)
	if !changed {
		return fmt.Errorf("the uplink did not actually move, from %v to %v", was, now)
	}
	w.event("the internet moved: " + reason)
	w.note("the internet moved: %s", reason)
	w.uplinkNow = now
	return nil
}

// ---------------------------------------------------------------------------
// Then: the outcome
// ---------------------------------------------------------------------------

// theBoxDoesNotNoticeTheUplinkMoved is the honest half of this scenario, and it
// is written as an assertion rather than left as a silence so that wiring a
// watcher in later turns this red and makes somebody update the promise.
//
// docs/BEHAVIOUR.md said the opposite until 2026-08-30: "The box notices, takes
// the old route away before adding the new one, and reloads the firewall as
// well." It does not notice. Nothing in the appliance polls the uplink.
func theBoxDoesNotNoticeTheUplinkMoved(w *World) error {
	if !w.uplinkNow.IsZero() && w.plan.Uplink == w.uplinkNow.Interface {
		return fmt.Errorf("the plan followed the uplink to %s on its own, so something now watches it "+
			"and docs/BEHAVIOUR.md has to be rewritten to promise that", w.uplinkNow.Interface)
	}
	if len(w.plan.ServerAddr) == 0 {
		return errors.New("the plan has no server address")
	}
	server := w.plan.ServerAddr[0].String() + "/32"
	moved := w.tl.lastIndexOf("net: " + netcfg.BinIP + " route add " + server + " via 192.168.1.1 dev wlan0")
	if moved >= 0 {
		return fmt.Errorf("the pinned host route was re-pinned through the new uplink at event %d, "+
			"so something acted on the change\n  %s", moved, w.tl)
	}
	return nil
}

// theBlockStillStopsClientTrafficByTheNewUplink is what makes the answer above
// acceptable rather than alarming.
//
// The ruleset still in force names the OLD uplink, so its explicit leak block
// no longer matches anything. That costs nothing, and the sentence that used to
// be here claimed the opposite: "a ruleset still naming the old one stops
// blocking the moment traffic starts leaving by the new one". The forward chain
// is policy drop and its only accepts name the TUNNEL, so a packet from the
// hotspot heading for any interface that is not the tunnel is dropped by the
// policy whatever the uplink is called.
//
// internal/netcfg's TestRulesetStillBlocksWhenTheUplinkIsRenamed proves that
// property against the generator; this proves it about the text actually loaded.
func theBlockStillStopsClientTrafficByTheNewUplink(w *World) error {
	if w.rulesetInForce == "" {
		return errors.New("no ruleset has been loaded at all")
	}
	rs, err := parseNft(w.rulesetInForce)
	if err != nil {
		return err
	}
	fwd, err := rs.chain("forward")
	if err != nil {
		return err
	}
	if fwd.policy != "drop" {
		return fmt.Errorf("the forward policy is %q, so client traffic heading for the interface that "+
			"now carries the internet is not dropped by the policy", fwd.policy)
	}
	for _, r := range fwd.withVerb("accept") {
		if !strings.Contains(r.text, "\""+w.plan.Tun+"\"") {
			return fmt.Errorf("the forward chain accepts something that does not name the tunnel, so it "+
				"can match traffic leaving by the interface that now carries the internet: %s", r.text)
		}
	}
	return nil
}

func theBoxConnects(w *World) error {
	if w.connectErr != nil {
		return fmt.Errorf("connect failed: %w", w.connectErr)
	}
	return nil
}

func theConfigIsSavedOnDisk(w *World) error {
	p := filepath.Join(w.dir, "state.json")
	fi, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("no state file at %s: %w", p, err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		return fmt.Errorf("the state file holds the pasted credential and its mode is %#o, want 0600", got)
	}
	if !w.store.Proxy().IsConfigured() {
		return errors.New("the store reports no config")
	}
	if w.storeFingerprint() == "" {
		return errors.New("the store has no fingerprint for the saved config, so two configs cannot be told apart in a log")
	}
	return nil
}

func aPlanIsProduced(w *World) error {
	if w.plan == nil {
		return errors.New("no plan")
	}
	if w.plan.Uplink == "" || w.plan.Hotspot == "" {
		return fmt.Errorf("the plan names uplink %q and hotspot %q", w.plan.Uplink, w.plan.Hotspot)
	}
	if !w.plan.HotspotSubnet.IsValid() || !w.plan.TunSubnet.IsValid() {
		return fmt.Errorf("the plan has hotspot subnet %v and tunnel subnet %v", w.plan.HotspotSubnet, w.plan.TunSubnet)
	}
	if len(w.plan.ServerAddr) == 0 {
		return errors.New("the plan has no server address, so no host route can be pinned and the engine would loop through its own tunnel")
	}
	return nil
}

func theEngineIsRunning(w *World) error {
	if w.eng == nil {
		return errors.New("no engine was started")
	}
	if got := w.eng.State().Phase; got != engine.PhaseRunning {
		return fmt.Errorf("engine phase is %v, want running (reason: %q)", got, w.eng.State().Reason)
	}
	return nil
}

func theAccessPointIsBeaconing(w *World) error {
	if !w.hotspotStat.AccessPoint.Running {
		return fmt.Errorf("the access point process is not running: %s", w.hotspotStat.Reason)
	}
	if !w.hotspotStat.AccessPoint.Beaconing {
		return fmt.Errorf("the access point is running and broadcasting nothing: %s", w.hotspotStat.Reason)
	}
	if !w.hotspotStat.DHCP.Running {
		return fmt.Errorf("devices would see the network and fail to join it: %s", w.hotspotStat.Reason)
	}
	return nil
}

func thePanelReportsConnected(w *World) error {
	if !w.status.Connected() {
		return fmt.Errorf("the panel would not say connected: engine %v, hotspot running %t",
			w.status.Engine.Phase, w.status.Hotspot.Running)
	}
	return nil
}

// shownDetection is what the panel would be looking at, with any deliberate
// defect applied.
func (w *World) shownDetection() panel.Detection {
	d := w.detection
	if w.defs.detection != nil {
		w.defs.detection(&d)
	}
	return d
}

// thePanelNamesWhatItDetected is design section 5.4: an automatic choice that
// is never displayed is one nobody can tell is wrong.
func thePanelNamesWhatItDetected(w *World) error {
	d := w.shownDetection()
	line := panel.DetectedLine(d)
	for _, want := range []string{"Internet:", "Hotspot:"} {
		if !strings.Contains(line, want) {
			return fmt.Errorf("the detected line %q does not say %q", line, want)
		}
	}
	if strings.Contains(line, "not found") {
		return fmt.Errorf("the detected line names nothing: %q", line)
	}
	// The words have to be words. "eth0" and "ap0" are kernel names, and the
	// audience is somebody who will not open a terminal.
	if strings.Contains(line, d.InternetInterface) || strings.Contains(line, d.HotspotInterface) {
		return fmt.Errorf("the detected line shows a kernel interface name to a non-technical user: %q", line)
	}
	if d.Subnet == "" || d.HotspotAddress == "" {
		return fmt.Errorf("advanced mode has nothing to show: subnet %q, hotspot address %q", d.Subnet, d.HotspotAddress)
	}
	return nil
}

// thePanelIsServedOnTheHotspotAndNeverOnAPublicAddress is design section 5.6:
// the hotspot interface always, the local network only if the user turns that
// on, never the uplink. What the code guarantees, and therefore what is
// asserted here, is narrower than that sentence and is worth stating exactly:
// the hotspot address alone by default, a wildcard never, and a local-network
// address only when it is private.
func thePanelIsServedOnTheHotspotAndNeverOnAPublicAddress(w *World) error {
	d := w.shownDetection()
	const port = 8088

	addrs, err := panel.BindAddrs(d, port, false)
	if err != nil {
		return fmt.Errorf("the panel has nowhere to listen: %w", err)
	}
	want := net.JoinHostPort(d.HotspotAddress, "8088")
	if len(addrs) != 1 || addrs[0] != want {
		return fmt.Errorf(
			"by default the panel must listen on the hotspot address %s and nothing else, and it would listen on %v",
			want, addrs)
	}
	for _, a := range addrs {
		for _, wild := range []string{"0.0.0.0:", "[::]:", ":8088"} {
			if a == wild || strings.HasPrefix(a, wild) {
				return fmt.Errorf("the panel would listen on the wildcard %q, which is reachable from the uplink", a)
			}
		}
	}

	// Turning the local network on adds one private address and nothing else.
	onLAN, err := panel.BindAddrs(d, port, true)
	if err != nil {
		return fmt.Errorf("opening the panel on the local network failed: %w", err)
	}
	if len(onLAN) != 2 {
		return fmt.Errorf("opening the panel on the local network gave %v, want the hotspot address and one more", onLAN)
	}

	// And a box whose attached network address is globally routable is the case
	// the rule exists for: it must be refused rather than quietly bound.
	public := d
	public.LocalNetworkAddress = "198.51.100.7"
	if _, err := panel.BindAddrs(public, port, true); err == nil {
		return errors.New("the panel would bind a globally routable address because the user ticked a box marked local network")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Ordering
// ---------------------------------------------------------------------------

// theFirewallIsLoadedBeforeForwardingIsEnabled closes the window in which the
// box forwards and the block is not yet in force.
func theFirewallIsLoadedBeforeForwardingIsEnabled(w *World) error {
	fw := w.tl.indexOf("net: nft -f -")
	fwd := w.tl.indexOf("net: sysctl -w net.ipv4.ip_forward=1")
	if fw < 0 {
		return fmt.Errorf("the ruleset was never loaded\n  %s", w.tl)
	}
	if fwd < 0 {
		return fmt.Errorf("forwarding was never enabled, so the box cannot share anything\n  %s", w.tl)
	}
	if fw > fwd {
		return fmt.Errorf(
			"forwarding was enabled at event %d and the ruleset was not loaded until event %d, "+
				"so for that window the box forwarded client traffic with no block in force\n  %s", fwd, fw, w.tl)
	}
	return nil
}

// theRPFilterDefaultIsSetBeforeTheEngineStarts.
//
// net.ipv4.conf.default.rp_filter is the value an interface is created with,
// and the tunnel device is created by the engine, so this write has to happen
// first to reach it at all.
//
// What it is FOR changed under this suite during the session it was written,
// and the change is worth recording rather than quietly absorbing. It was the
// mechanism: an earlier internal/netcfg wrote rp_filter per interface on the
// belief that relaxing conf.all alone changed nothing. That was the inverse of
// the kernel's documented behaviour, which takes the MAXIMUM of conf.all and
// the per-interface value while loose (2) is numerically larger than strict
// (1), so conf.all pins every interface on its own. The per-interface writes
// are gone and conf.default is now redundancy, kept in case something else on
// the box lowers conf.all later.
//
// The ordering assertion survives that change unaltered, which is the point of
// asserting the order rather than the reason.
func theRPFilterDefaultIsSetBeforeTheEngineStarts(w *World) error {
	knob := w.tl.indexOf("net: sysctl -w net.ipv4.conf.default.rp_filter=")
	eng := w.tl.indexOf("engine: started")
	if knob < 0 {
		return fmt.Errorf("the inherited reverse-path filter default was never set\n  %s", w.tl)
	}
	if eng < 0 {
		return fmt.Errorf("the engine never started\n  %s", w.tl)
	}
	if knob > eng {
		return fmt.Errorf(
			"the engine started at event %d and the inherited default was not set until event %d, "+
				"so the tunnel device inherited the old value and nothing corrects it\n  %s", eng, knob, w.tl)
	}
	return nil
}

// everyStepThatNamesTheTunnelWaitsForTheEngine. The tunnel device is created by
// the engine, so a command naming it that runs first fails.
//
// HONESTY NOTE: the engine here runs with its TUN inbound switched off, so no
// device is created and this is asserted against the marker the appliance emits
// at the moment the engine starts. It proves the ORDER the appliance uses. It
// does not observe a device appearing, and it cannot.
func everyStepThatNamesTheTunnelWaitsForTheEngine(w *World) error {
	if w.plan == nil {
		return errors.New("no plan")
	}
	eng := w.tl.indexOf("engine: started")
	if eng < 0 {
		return fmt.Errorf("the engine never started\n  %s", w.tl)
	}
	tun := w.plan.Tun
	for i, e := range w.tl.all() {
		line, ok := strings.CutPrefix(e, "net: ")
		if !ok {
			continue
		}
		// The firewall names the tunnel and must NOT wait: it matches by name,
		// which is resolved per packet, so it loads with no tunnel present.
		// That is the property the whole fail-closed design rests on.
		if strings.HasPrefix(line, netcfg.BinNft+" ") {
			continue
		}
		// Only changes. Detection READS a knob for the tunnel device before it
		// exists, and that is correct: "sysctl -e" skips a knob with no file
		// behind it rather than failing the whole read. The consequence is not
		// nothing, and it is not hidden: no previous value is recorded, so the
		// later change to that knob has no inverse. See
		// everyRecordedChangeIsUndone.
		if !isChange(strings.Fields(line)) {
			continue
		}
		if !strings.Contains(e, " "+tun) && !strings.Contains(e, "."+tun+".") {
			continue
		}
		if i < eng {
			return fmt.Errorf(
				"event %d names the tunnel device and ran before the engine started at event %d, "+
					"so on a real box it would have failed with no such device: %s\n  %s", i, eng, e, w.tl)
		}
	}
	return nil
}

// theFirewallDoesNotWaitForTheTunnel is the other half, and it is the one that
// matters most: a ruleset that could not load until the tunnel existed would be
// absent at exactly the moment it has to be in force.
func theFirewallDoesNotWaitForTheTunnel(w *World) error {
	fw := w.tl.indexOf("net: nft -f -")
	eng := w.tl.indexOf("engine: started")
	if fw < 0 || eng < 0 {
		return fmt.Errorf("firewall at %d, engine at %d\n  %s", fw, eng, w.tl)
	}
	if fw > eng {
		return fmt.Errorf(
			"the ruleset was loaded at event %d, after the engine started at event %d. "+
				"It has to load with no tunnel present, because the moment it is needed most is the "+
				"moment the tunnel is gone\n  %s", fw, eng, w.tl)
	}
	return nil
}

// theEngineReachesTheServerOutsideTheTunnel: without a pinned host
// route through the real gateway the engine's own connection to the user's
// server matches the default route through the tunnel it has not built yet.
func theEngineReachesTheServerOutsideTheTunnel(w *World) error {
	if w.plan == nil {
		return errors.New("no plan")
	}
	if len(w.plan.ServerAddr) == 0 {
		return errors.New("the plan has no server address")
	}
	eng := w.tl.indexOf("engine: started")
	for _, s := range w.plan.ServerAddr {
		want := "net: ip route add " + s.String() + "/32"
		at := w.tl.indexOf(want)
		if at < 0 {
			return fmt.Errorf(
				"no host route was pinned to the server at %s, so the engine's own connection to it "+
					"matches the default route through the tunnel and loops through itself\n  %s", s, w.tl)
		}
		if !strings.Contains(w.tl.all()[at], "via "+w.plan.UplinkGateway.String()) {
			return fmt.Errorf("the pinned route to %s does not go via the real gateway %s: %s",
				s, w.plan.UplinkGateway, w.tl.all()[at])
		}
		if eng >= 0 && at > eng {
			return fmt.Errorf(
				"the host route to %s was pinned at event %d, after the engine started at event %d, "+
					"so the engine's very first connection was not protected\n  %s", s, at, eng, w.tl)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Fail closed
// ---------------------------------------------------------------------------

// clientTrafficLeavesOnlyThroughTheTunnel: in the forward chain, every rule
// that permits anything names the tunnel device.
func clientTrafficLeavesOnlyThroughTheTunnel(w *World) error {
	rs, fwd, err := w.forwardChain()
	if err != nil {
		return err
	}
	_ = rs
	if fwd.policy != "drop" {
		return fmt.Errorf("the forward policy is %q, so anything no rule mentions is carried", fwd.policy)
	}
	accepts := fwd.withVerb("accept")
	if len(accepts) == 0 {
		return errors.New("no rule in the forward chain permits anything, so no client can reach the internet at all")
	}
	for _, r := range accepts {
		if !strings.Contains(r.text, `"`+w.plan.Tun+`"`) {
			return fmt.Errorf(
				"this forward rule permits traffic without naming the tunnel device %q, so it keeps "+
					"matching when the tunnel is gone: %s", w.plan.Tun, r.text)
		}
	}
	// And the way out is by the tunnel, not by the uplink: the tunnel table's
	// default route is the tunnel device.
	if w.tl.indexOf("net: ip route add default dev "+w.plan.Tun+" proto static table") < 0 {
		return fmt.Errorf("no default route through %s was installed in the tunnel table\n  %s", w.plan.Tun, w.tl)
	}
	return nil
}

// withTheTunnelGoneNoRuleLetsClientTrafficOut is the fail-closed claim itself.
// Every rule naming the tunnel is removed, which models what the kernel does
// when the device disappears, and what is left must still block.
func withTheTunnelGoneNoRuleLetsClientTrafficOut(w *World) error {
	text, err := w.ruleset()
	if err != nil {
		return err
	}
	stripped := withoutInterface(text, w.plan.Tun)
	rs, err := parseNft(stripped)
	if err != nil {
		return fmt.Errorf("the ruleset with the tunnel rules removed could not be read: %w", err)
	}
	fwd, err := rs.chain("forward")
	if err != nil {
		return err
	}
	if fwd.policy != "drop" {
		return fmt.Errorf("with the tunnel gone the forward policy is %q, so client traffic falls back to the uplink", fwd.policy)
	}
	if got := fwd.withVerb("accept"); len(got) > 0 {
		return fmt.Errorf(
			"with the tunnel gone %d forward rule(s) still permit traffic, and the tunnel is gone, "+
				"so what they permit leaves by the uplink: %v", len(got), ruleTexts(got))
	}
	// The block itself must survive: it names only the hotspot and the uplink.
	want := fmt.Sprintf("iifname %q oifname %q drop", w.plan.Hotspot, w.plan.Uplink)
	found := false
	for _, r := range fwd.withVerb("drop") {
		if r.text == want {
			found = true
		}
	}
	if !found {
		return fmt.Errorf(
			"the explicit leak block %q did not survive the tunnel going away, so the guarantee "+
				"rests on the chain policy alone: %v", want, ruleTexts(fwd.rules))
	}
	return nil
}

// theBoxNeverMasqueradesOntoTheUplink. Source NAT towards the uplink is the
// single line that would quietly turn the appliance into an ordinary router.
func theBoxNeverMasqueradesOntoTheUplink(w *World) error {
	text, err := w.ruleset()
	if err != nil {
		return err
	}
	rs, err := parseNft(text)
	if err != nil {
		return err
	}
	post, err := rs.chain("postrouting")
	if err != nil {
		return err
	}
	for _, r := range post.withVerb("masquerade") {
		if strings.Contains(r.text, `"`+w.plan.Uplink+`"`) {
			return fmt.Errorf("the ruleset masquerades onto the uplink: %s", r.text)
		}
	}
	for name, c := range rs.chains {
		for _, r := range c.rules {
			if r.verb == "masquerade" && strings.Contains(r.text, `"`+w.plan.Uplink+`"`) {
				return fmt.Errorf("chain %s masquerades onto the uplink: %s", name, r.text)
			}
		}
	}
	return nil
}

// noClientIPv6IsOffered. There is no IPv6 tunnel, and a client with a working
// IPv6 path prefers it over IPv4 and bypasses the tunnel completely.
//
// Three mechanisms are checked, because one is not enough for a leak of this
// shape and because each covers a different way a client could end up with a
// working v6 path: the box does not forward v6 at all, the firewall drops
// forwarded v6 on the hotspot in both directions, and the box never advertises
// a prefix, so a client cannot autoconfigure an address in the first place.
//
// Deliberately NOT checked: net.ipv6.conf.<hotspot>.disable_ipv6. It was in
// the plan earlier in this session and internal/netcfg removed it on purpose
// (see the comment in SysctlSteps): it is a per-interface knob on an interface
// this program creates, so there is no prior value to read, the change would
// get no inverse, and on a hotspot interface that already existed an uninstall
// would leave the user's adapter with IPv6 switched off.
func noClientIPv6IsOffered(w *World) error {
	text, err := w.ruleset()
	if err != nil {
		return err
	}
	rs, err := parseNft(text)
	if err != nil {
		return err
	}
	fwd, err := rs.chain("forward")
	if err != nil {
		return err
	}
	hot := w.plan.Hotspot
	var inbound, outbound bool
	for _, r := range fwd.withVerb("drop") {
		if !strings.Contains(r.text, "nfproto ipv6") {
			continue
		}
		if strings.Contains(r.text, `iifname "`+hot+`"`) {
			inbound = true
		}
		if strings.Contains(r.text, `oifname "`+hot+`"`) {
			outbound = true
		}
	}
	if !inbound || !outbound {
		return fmt.Errorf(
			"the forward chain does not drop client IPv6 in both directions (from the hotspot %t, to the hotspot %t), "+
				"so a client that gets an IPv6 path uses it instead of the tunnel", inbound, outbound)
	}

	out, err := rs.chain("output")
	if err != nil {
		return err
	}
	advertised := true
	for _, r := range out.withVerb("drop") {
		if strings.Contains(r.text, "nd-router-advert") && strings.Contains(r.text, `"`+hot+`"`) {
			advertised = false
		}
	}
	if advertised {
		return errors.New("the box may still advertise an IPv6 prefix to clients, and an autoconfigured client would prefer v6")
	}

	if w.tl.indexOf("net: "+netcfg.BinSysctl+" -w net.ipv6.conf.all.forwarding=0") < 0 {
		return fmt.Errorf("the box was not told to stop forwarding IPv6 at all\n  %s", w.tl)
	}
	return nil
}

// clientDNSCannotEscapeToAResolverOfItsOwnChoosing. A client with a resolver
// hardcoded into it is redirected, not permitted.
func clientDNSCannotEscapeToAResolverOfItsOwnChoosing(w *World) error {
	text, err := w.ruleset()
	if err != nil {
		return err
	}
	rs, err := parseNft(text)
	if err != nil {
		return err
	}
	pre, err := rs.chain("prerouting")
	if err != nil {
		return err
	}
	hot := `"` + w.plan.Hotspot + `"`
	var udp, tcp bool
	for _, r := range pre.withVerb("redirect") {
		if !strings.Contains(r.text, hot) || !strings.Contains(r.text, "dport 53") {
			continue
		}
		if strings.Contains(r.text, "udp ") {
			udp = true
		}
		if strings.Contains(r.text, "tcp ") {
			tcp = true
		}
	}
	if !udp || !tcp {
		return fmt.Errorf("client DNS is not redirected to this box on both protocols (udp %t, tcp %t), so a client with a hardcoded resolver reaches it", udp, tcp)
	}

	fwd, err := rs.chain("forward")
	if err != nil {
		return err
	}
	var dot, doq bool
	for _, r := range fwd.rules {
		if !strings.Contains(r.text, hot) || !strings.Contains(r.text, "dport 853") {
			continue
		}
		if r.verb == "reject" {
			dot = true
		}
		if r.verb == "drop" {
			doq = true
		}
	}
	if !dot || !doq {
		return fmt.Errorf("DNS over TLS and DNS over QUIC are not both stopped (853/tcp rejected %t, 853/udp dropped %t), so a client's queries walk past the resolver on this box", dot, doq)
	}

	// And the resolver the box itself forwards to must be on the box. A
	// forwarding target anywhere else is every lookup leaving outside the
	// tunnel, for every client.
	conf := w.hotspotPlan.DnsmasqConf
	if !strings.Contains(conf, "no-resolv") {
		return errors.New("dnsmasq is not stopped from reading /etc/resolv.conf, so it inherits whatever resolver the uplink handed this box")
	}
	for _, line := range strings.Split(conf, "\n") {
		if !strings.HasPrefix(line, "server=") {
			continue
		}
		if !strings.HasPrefix(line, "server=127.") && !strings.HasPrefix(line, "server=::1") {
			return fmt.Errorf("client DNS is forwarded off this box: %s", line)
		}
	}
	return nil
}

// everyInterfaceMatchIsByNameAndNotByIndex. Index matching is resolved when the
// ruleset loads, so a ruleset naming the tunnel by index cannot be loaded while
// the tunnel is down, which is exactly when it has to be in force.
func everyInterfaceMatchIsByNameAndNotByIndex(w *World) error {
	text, err := w.ruleset()
	if err != nil {
		return err
	}
	rs, err := parseNft(text)
	if err != nil {
		return err
	}
	for name, c := range rs.chains {
		for _, r := range c.rules {
			for _, f := range strings.Fields(r.text) {
				if f == "iif" || f == "oif" {
					return fmt.Errorf("chain %s matches an interface by index, which cannot be loaded while that interface is absent: %s", name, r.text)
				}
			}
		}
	}
	return nil
}

// clientsGetNothingRatherThanAWayOut is what a failed connect has to leave
// behind. The engine may be up and carrying nothing; what must not happen is
// that clients find another way.
func clientsGetNothingRatherThanAWayOut(w *World) error {
	if w.connectErr == nil {
		return errors.New("this connect succeeded, so there is no failure to check the aftermath of")
	}
	if w.plan == nil {
		// The flow failed before it touched the machine, which is the
		// strongest form of this: nothing was changed at all.
		return theMachineWasNotTouched(w)
	}
	return withTheTunnelGoneNoRuleLetsClientTrafficOut(w)
}

// ---------------------------------------------------------------------------
// Teardown and recovery
// ---------------------------------------------------------------------------

// everyRecordedChangeIsUndone: the teardown replayed the inverse of every step
// that had one, and none of the replays failed.
func everyRecordedChangeIsUndone(w *World) error {
	return checkUndone(w, w.teardown, "the teardown")
}

// everyRecordedChangeIsUndoneByTheReplay is the same claim about a journal
// picked up by a restarted process.
func everyRecordedChangeIsUndoneByTheReplay(w *World) error {
	return checkUndone(w, w.recovered, "the replay from the journal")
}

func checkUndone(w *World, rep netcfg.Report, what string) error {
	if rep.Failed != 0 {
		return fmt.Errorf("%s left %d of %d changes in place: %v", what, rep.Failed, len(rep.Results), rep.Err())
	}
	ran := map[string]bool{}
	for _, r := range rep.Results {
		ran[netcfg.RunnerKey(r.Step.Do)] = true
	}
	var missing []string
	var noInverse []string
	for _, s := range append(append([]netcfg.Step{}, w.preSteps...), w.postSteps...) {
		if s.Do.IsZero() {
			continue
		}
		if s.Undo.IsZero() {
			noInverse = append(noInverse, netcfg.RunnerKey(s.Do))
			continue
		}
		if !ran[netcfg.RunnerKey(s.Undo)] {
			missing = append(missing, netcfg.RunnerKey(s.Undo))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s did not undo %d change(s): %v", what, len(missing), missing)
	}
	// A step with no inverse is a statement, not an oversight, so each one is
	// checked against the two reasons this box accepts. Anything else is a
	// change to the machine with no way back, and it fails here.
	for _, cmd := range noInverse {
		if ok, _ := w.acceptableWithoutInverse(cmd); !ok {
			_, why := w.acceptableWithoutInverse(cmd)
			return fmt.Errorf(
				"this change was made with no way to undo it, and it is not one of the two kinds this "+
					"box accepts without one: %s\n  %s", cmd, why)
		}
	}
	return nil
}

// acceptableWithoutInverse reports whether a change with no recorded inverse is
// one of the two cases that are correct, and says which.
//
// One: bringing the hotspot interface up. Bringing a radio down on teardown is
// worse than leaving it up, because the machine's own WiFi client, and the
// panel the user is reading, may be on that radio. Removing the address is the
// inverse that matters and it is recorded.
//
// Two: a kernel knob on an interface THIS BOX CREATES. The knob did not exist
// when detection read the machine, so there is no previous value to restore,
// and writing back a value nobody measured would leave the machine in a state
// it was never in. The knob disappears with the interface, and that the
// interface does disappear is checked rather than assumed.
func (w *World) acceptableWithoutInverse(cmd string) (bool, string) {
	if strings.HasPrefix(cmd, netcfg.BinIP+" link set dev ") && strings.HasSuffix(cmd, " up") {
		return true, "bringing an interface up, whose inverse would take down a radio the user may be reading the panel over"
	}
	f := strings.Fields(cmd)
	if len(f) >= 3 && f[0] == netcfg.BinSysctl && f[1] == "-w" {
		knob, _, _ := strings.Cut(f[2], "=")
		iface, isPerIface := interfaceOfKnob(knob)
		if !isPerIface {
			return false, "this knob is not tied to an interface, so its previous value was readable and should have been recorded"
		}
		if _, wasRead := w.facts.Sysctl[knob]; wasRead {
			return false, "this knob WAS read during detection, so an inverse could have been recorded and was not"
		}
		created, gone := w.interfaceIsCreatedAndDestroyedByTheBox(iface)
		if !created {
			return false, fmt.Sprintf("%s is not an interface this box creates, so the knob existed and should have been read", iface)
		}
		if !gone {
			return false, fmt.Sprintf("%s is created by this box and is still there at the end, so the changed knob outlives the run", iface)
		}
		return true, "a knob on an interface this box creates and destroys, which did not exist to be read"
	}
	return false, "no reason on file"
}

// interfaceIsCreatedAndDestroyedByTheBox is true for the access point's own
// interface, which this program adds to the radio, and for the tunnel device,
// which the engine creates.
func (w *World) interfaceIsCreatedAndDestroyedByTheBox(iface string) (created, gone bool) {
	if w.plan == nil {
		return false, false
	}
	switch iface {
	case w.plan.Tun:
		return true, w.tl.indexOf("engine: stopped") >= 0
	case w.plan.Hotspot:
		if !w.plan.HotspotIsVirtual {
			return false, false
		}
		return true, w.tl.indexOf("net: "+netcfg.BinIw+" dev "+iface+" del") >= 0
	}
	return false, false
}

// nothingTheBoxChangedIsStillInPlace reads the timeline rather than the plan.
//
// checkUndone above asks "was the inverse of every PLANNED step replayed", and
// that question cannot see a change made outside the plan. This one asks "did
// everything the box actually did get undone", which is the question the user
// is asking when they turn the switch off, and it catches a change somebody
// added straight at the runner with no inverse recorded.
func nothingTheBoxChangedIsStillInPlace(w *World) error {
	added := map[string]string{}
	removed := map[string]bool{}
	writes := map[string][]string{}
	nftLoads := 0

	for _, e := range w.tl.all() {
		line, ok := strings.CutPrefix(e, "net: ")
		if !ok {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case netcfg.BinNft:
			nftLoads++
		case netcfg.BinSysctl:
			for _, a := range f[1:] {
				if k, v, found := strings.Cut(a, "="); found {
					writes[k] = append(writes[k], v)
				}
			}
		case netcfg.BinIw:
			for i, a := range f {
				if a == "interface" && i+2 < len(f) && f[i+1] == "add" {
					added["interface "+f[i+2]] = e
				}
			}
			if len(f) >= 4 && f[1] == "dev" && f[3] == "del" {
				removed["interface "+f[2]] = true
			}
		case netcfg.BinIP:
			key, verb, ok := ipChangeKey(f)
			if !ok {
				continue
			}
			if verb == "add" {
				added[key] = e
			} else {
				removed[key] = true
			}
		}
	}

	var left []string
	for k, e := range added {
		if !removed[k] {
			left = append(left, e)
		}
	}
	if len(left) > 0 {
		sort.Strings(left)
		return fmt.Errorf(
			"the box made %d change(s) and never undid them, so the machine was not left as it was found:\n  %s",
			len(left), strings.Join(left, "\n  "))
	}

	var knobs []string
	for k := range writes {
		knobs = append(knobs, k)
	}
	sort.Strings(knobs)
	for _, knob := range knobs {
		vals := writes[knob]
		want, wasRead := w.facts.Sysctl[knob]
		if !wasRead {
			iface, isPerIface := interfaceOfKnob(knob)
			if !isPerIface {
				return fmt.Errorf("the box changed kernel knob %s without reading it first, so it cannot know what to put back", knob)
			}
			created, gone := w.interfaceIsCreatedAndDestroyedByTheBox(iface)
			if !created || !gone {
				return fmt.Errorf(
					"kernel knob %s was changed with no previous value read, and %s is not an interface "+
						"this box creates and destroys, so the change outlives the run", knob, iface)
			}
			continue
		}
		if got := vals[len(vals)-1]; got != want {
			return fmt.Errorf(
				"kernel knob %s was read as %q before the change and left at %q, so the machine is in a state it was never in (writes: %v)",
				knob, want, got, vals)
		}
	}
	if nftLoads < 2 {
		return fmt.Errorf(
			"the ruleset was loaded %d time(s). Loading it once and never replacing it leaves this "+
				"program's table on a machine it promised to return", nftLoads)
	}
	return nil
}

// isChange reports whether a command alters the machine rather than reading it.
// Detection runs read-only commands, and an ordering rule about when a change
// may happen says nothing about when a read may.
func isChange(f []string) bool {
	if len(f) == 0 {
		return false
	}
	switch f[0] {
	case netcfg.BinSysctl:
		return len(f) > 1 && f[1] == "-w"
	case netcfg.BinNft:
		return true
	case netcfg.BinIw, netcfg.BinIP:
		for _, a := range f {
			if a == "add" || a == "del" || a == "set" {
				return true
			}
		}
	}
	return false
}

// ipChangeKey identifies the object an "ip" command changes, so that an add and
// its matching del produce the same key. The tokens that appear only on an add
// are dropped: "proto static", "metric 5" and "scope link" describe how a route
// is installed and are not part of what identifies it.
func ipChangeKey(f []string) (key, verb string, ok bool) {
	var out []string
	skip := 0
	for _, a := range f {
		if skip > 0 {
			skip--
			continue
		}
		switch a {
		case "add", "del":
			if verb == "" {
				verb = a
				continue
			}
		case "proto", "metric", "scope":
			skip = 1
			continue
		}
		out = append(out, a)
	}
	if verb == "" {
		return "", "", false
	}
	return strings.Join(out, " "), verb, true
}

func theJournalIsGone(w *World) error {
	if _, err := os.Stat(w.journalPath()); !errors.Is(err, os.ErrNotExist) {
		b, _ := os.ReadFile(w.journalPath())
		return fmt.Errorf("the journal is still on disk at %s, so the next start will replay changes that were already undone:\n%s",
			w.journalPath(), string(b))
	}
	return nil
}

func nothingIsLeftRunning(w *World) error {
	if w.eng != nil && w.eng.State().Phase == engine.PhaseRunning {
		return errors.New("the engine is still running")
	}
	// Both daemons were signalled and their pid files removed.
	for _, p := range []string{w.hotspotPaths().HostapdPID, w.hotspotPaths().DnsmasqPID} {
		if _, err := w.sys.ReadFile(p); err == nil {
			return fmt.Errorf("the pid file %s is still there, so the next start reads a pid that is dead or reused", p)
		}
	}
	return nil
}

// theLeftoverChangesAreUndoneFirst: recovery happens before the new plan is
// applied, not after.
func theLeftoverChangesAreUndoneFirst(w *World) error {
	if w.leftover == "" {
		return errors.New("this scenario did not leave a journal behind")
	}
	undo := w.tl.indexOf("net: " + w.leftover)
	if undo < 0 {
		return fmt.Errorf("the change left by the killed process was never undone: %s\n  %s", w.leftover, w.tl)
	}
	firstOther := -1
	for i, e := range w.tl.all() {
		if i == undo || !strings.HasPrefix(e, "net: ") {
			continue
		}
		firstOther = i
		break
	}
	if firstOther >= 0 && firstOther < undo {
		return fmt.Errorf(
			"the box touched the machine at event %d before undoing the leftover change at event %d, "+
				"so the plan was built against a machine that still held it\n  %s", firstOther, undo, w.tl)
	}
	if len(w.recovered.Results) == 0 {
		return errors.New("recovery reported that it replayed nothing")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Bad input: three states, three different things for the user to do
// ---------------------------------------------------------------------------

func theUserIsToldTheTextCouldNotBeRead(w *World) error {
	return wantStage(w, panel.StageParse, "Caspian could not read that link.")
}

func theUserIsToldTheLinkCannotBeUsedAsWritten(w *World) error {
	return wantStage(w, panel.StageEngine, "Caspian read that link, but it cannot be used as written.")
}

func theUserIsToldTheServerDidNotAnswer(w *World) error {
	return wantStage(w, panel.StageServer, "Caspian read that link and tried it, but the server did not answer.")
}

func wantStage(w *World, want panel.ConfigStage, headline string) error {
	if w.connectErr == nil {
		return errors.New("the connect succeeded, so there is nothing to word")
	}
	if w.problem.Stage != want {
		return fmt.Errorf(
			"the box put this failure in the %q state and it belongs in %q. The three need three "+
				"different things from the user, and one message for all three makes the third look "+
				"like the first. It said: %s",
			w.problem.Stage, want, w.problem.Text())
	}
	if w.problem.HeadlineText() != headline {
		return fmt.Errorf("headline is %q, want %q", w.problem.HeadlineText(), headline)
	}
	if w.problem.Advice == "" {
		return fmt.Errorf("the message tells the user what is wrong and not what to do: %q", w.problem.Text())
	}
	return nil
}

// theThreeBadConfigMessagesAreDifferent is the property the three states exist
// for. Asserting each one separately would still pass if all three said the
// same thing.
func theThreeBadConfigMessagesAreDifferent(w *World) error {
	seen := map[string]string{}
	for _, p := range []panel.Problem{
		panel.ParseProblem(errors.New("anything")),
		panel.EngineProblem(),
		panel.ServerProblem(),
	} {
		if other, dup := seen[p.HeadlineText()]; dup {
			return fmt.Errorf("the %s state and the %s state say the same thing: %q", other, p.Stage, p.Headline)
		}
		seen[p.HeadlineText()] = p.Stage.String()
	}
	return nil
}

func theMachineWasNotTouched(w *World) error {
	for _, e := range w.tl.all() {
		if strings.HasPrefix(e, "net: ") || strings.HasPrefix(e, "hotspot: ") {
			return fmt.Errorf(
				"the box changed the machine before it had read the pasted text: %s\n  %s", e, w.tl)
		}
	}
	if _, err := os.Stat(w.journalPath()); !errors.Is(err, os.ErrNotExist) {
		return errors.New("a journal of applied changes was written for a config that was never usable")
	}
	return nil
}

func theUserIsToldNoAdapterCanCreateAHotspot(w *World) error {
	if w.connectErr == nil {
		return errors.New("the box reported success on a machine that cannot create a hotspot")
	}
	var pe *netcfg.PlanError
	if !errors.As(w.connectErr, &pe) {
		return fmt.Errorf("the refusal carries no wording for the panel: %v", w.connectErr)
	}
	if !errors.Is(w.connectErr, netcfg.ErrNoAPCapableInterface) {
		return fmt.Errorf("the refusal is not about AP support: %v", w.connectErr)
	}
	msg := pe.UserMessage()
	for _, bad := range []string{"phy", "AP-capable", "nl80211", "iw "} {
		if strings.Contains(msg, bad) {
			return fmt.Errorf("the message uses wireless vocabulary a non-technical person cannot act on (%q): %s", bad, msg)
		}
	}
	if !strings.Contains(strings.ToLower(msg), "usb") {
		return fmt.Errorf("the message does not say what to go and do: %s", msg)
	}
	return nil
}

func theUserIsToldTheBoxHasNoInternetConnection(w *World) error {
	if w.connectErr == nil {
		return errors.New("the box reported success on a machine with no internet connection of its own")
	}
	if !errors.Is(w.connectErr, netcfg.ErrNoUplink) {
		return fmt.Errorf("the refusal is not about the uplink: %v", w.connectErr)
	}
	var pe *netcfg.PlanError
	if !errors.As(w.connectErr, &pe) {
		return fmt.Errorf("the refusal carries no wording for the panel: %v", w.connectErr)
	}
	msg := strings.ToLower(pe.UserMessage())
	if !strings.Contains(msg, "cable") && !strings.Contains(msg, "wifi") {
		return fmt.Errorf("the message does not say what to go and do: %s", pe.UserMessage())
	}
	return nil
}

// ---------------------------------------------------------------------------
// Secrets
// ---------------------------------------------------------------------------

// theCredentialAppearsInNothingTheUserOrALogCanSee scans everything the flow
// produced. The two places the credential is supposed to be are excluded here
// and asserted positively by the next step, so this cannot pass by the
// credential having gone missing altogether.
func theCredentialAppearsInNothingTheUserOrALogCanSee(w *World) error {
	for name, text := range w.everythingAUserOrALogCanSee() {
		for _, s := range secrets() {
			if s == "" {
				continue
			}
			if strings.Contains(text, s) {
				return fmt.Errorf("a credential appears in %s:\n%s", name, text)
			}
		}
	}
	return nil
}

// theCredentialReachesOnlyTheEngineConfigAndTheStateFile is the other half. A
// scan that finds nothing is also what a flow that lost the config produces.
func theCredentialReachesOnlyTheEngineConfigAndTheStateFile(w *World) error {
	if len(w.engineCfg) == 0 {
		return errors.New("no engine config was produced, so the scan above proves nothing")
	}
	if !strings.Contains(string(w.engineCfg), fakeUUID) {
		return errors.New("the engine config does not carry the user id, so it is not the document that would connect")
	}
	raw, err := os.ReadFile(filepath.Join(w.dir, "state.json"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(raw), fakeUUID) {
		return errors.New("the state file does not carry the pasted config, so it would not survive a reboot")
	}
	return nil
}

// theHotspotPasswordReachesTheAccessPointAndNothingElse. It has to be in the
// hostapd configuration, and it must not be in the DHCP and DNS configuration,
// in the firewall, in a log line or in anything the panel renders.
func theHotspotPasswordReachesTheAccessPointAndNothingElse(w *World) error {
	if !strings.Contains(w.hotspotPlan.HostapdConf, fakeHotspotPassphrase) {
		return errors.New("the access point configuration does not carry the passphrase, so no device could join")
	}
	if strings.Contains(w.hotspotPlan.DnsmasqConf, fakeHotspotPassphrase) {
		return errors.New("the passphrase is in the DHCP and DNS configuration, which is mode 0644")
	}
	spec := panel.HotspotSpec{
		SSID:       w.hotspotPlan.AP.SSID,
		Passphrase: w.hotspotPlan.AP.Passphrase,
		Interface:  w.hotspotPlan.AP.Interface,
		Channel:    w.hotspotPlan.AP.Channel,
		Subnet:     w.hotspotPlan.DNS.Subnet.String(),
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", spec, spec, spec), fakeHotspotPassphrase) {
		return errors.New("a single fmt verb on the panel's hotspot request prints the passphrase")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Idempotence
// ---------------------------------------------------------------------------

func onlyOneEngineIsRunning(w *World) error {
	if w.eng == nil {
		return errors.New("no engine")
	}
	st := w.eng.State()
	if st.Phase != engine.PhaseRunning {
		return fmt.Errorf("engine phase is %v after the second connect", st.Phase)
	}
	if w.engineRunningSince.IsZero() {
		return errors.New("the first connect did not record when the engine started")
	}
	if !st.Since.Equal(w.engineRunningSince) {
		return fmt.Errorf(
			"the engine has been running since %s and was running since %s before the second connect, "+
				"so pressing the switch again restarted the tunnel",
			st.Since.Format(time.RFC3339Nano), w.engineRunningSince.Format(time.RFC3339Nano))
	}
	return nil
}

func theAccessPointWasNotRestarted(w *World) error {
	paths := w.hotspotPaths()
	if got := w.sys.CountCalls(paths.HostapdBinary); got != w.hostapdStarts {
		return fmt.Errorf(
			"the access point was started %d time(s) and had been started %d time(s) before the second "+
				"connect, so every joined device was dropped", got, w.hostapdStarts)
	}
	if got := w.sys.CountCalls(paths.DnsmasqBinary); got != w.dnsmasqStarts {
		return fmt.Errorf("the DHCP and DNS server was restarted (%d starts, was %d)", got, w.dnsmasqStarts)
	}
	if len(w.sys.Signals) != 0 {
		return fmt.Errorf("a running process was signalled during the second connect: %v", w.sys.Signals)
	}
	if w.hotspotStat.AccessPoint.Detail != "already running" {
		return fmt.Errorf("the supervisor reported %q rather than leaving the access point alone", w.hotspotStat.AccessPoint.Detail)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The generated engine configuration
// ---------------------------------------------------------------------------

// noResolverInAnyGeneratedConfigurationIsAGoogleOne is design section 2 and
// section 6: Google is not used anywhere, including as a resolver default.
//
// The check is on the resolver VALUES rather than on the text of the documents.
// A substring scan looked right and was wrong: this scenario is called "the box
// needs no download and asks no Google server anything", Go puts the test name
// into the temporary directory it makes, that directory is the lease file path,
// and the lease file path is written into the DHCP configuration. The scan
// found the word Google in a file path and reported a leak. A check that can
// fail on its own name is not a check.
func noResolverInAnyGeneratedConfigurationIsAGoogleOne(w *World) error {
	if len(w.engineCfg) == 0 {
		return errors.New("no engine config was produced")
	}

	var doc struct {
		DNS struct {
			Servers []json.RawMessage `json:"servers"`
		} `json:"dns"`
	}
	if err := json.Unmarshal(w.engineCfg, &doc); err != nil {
		return fmt.Errorf("the generated engine config is not readable JSON: %w", err)
	}
	if len(doc.DNS.Servers) == 0 {
		return errors.New("the generated engine config names no resolver at all, so this check proves nothing")
	}
	for i, raw := range doc.DNS.Servers {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fmt.Errorf("resolver %d of %d is not a plain address, so which server it reaches is a question about what a name resolves to",
				i+1, len(doc.DNS.Servers))
		}
		if err := notAGoogleAddress(s); err != nil {
			return fmt.Errorf("resolver %d of %d: %w", i+1, len(doc.DNS.Servers), err)
		}
	}

	for _, line := range strings.Split(w.hotspotPlan.DnsmasqConf, "\n") {
		target, ok := strings.CutPrefix(line, "server=")
		if !ok {
			continue
		}
		addr, _, _ := strings.Cut(target, "#")
		if err := notAGoogleAddress(addr); err != nil {
			return fmt.Errorf("the DHCP and DNS configuration forwards to %w", err)
		}
	}
	return nil
}

// googlePublicDNS is every network Google Public DNS answers on, as prefixes
// rather than as the four well-known addresses, so that a neighbour of a famous
// address is covered too. The list is repeated here rather than read from
// internal/xcfg because a test that borrows the list it is checking against
// cannot catch that list being emptied.
var googlePublicDNS = []netip.Prefix{
	netip.MustParsePrefix("8.8.8.0/24"),
	netip.MustParsePrefix("8.8.4.0/24"),
	netip.MustParsePrefix("2001:4860:4860::/48"),
}

func notAGoogleAddress(s string) error {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return fmt.Errorf("%q is not a plain IP address", s)
	}
	for _, p := range googlePublicDNS {
		if p.Contains(addr.Unmap()) {
			return fmt.Errorf("%q, which is inside Google's %v", s, p)
		}
	}
	return nil
}

// noDownloadedGeoDataFileIsNeeded. A geoip: or geosite: rule reintroduces a
// downloaded data file to a product whose whole installer story is one verified
// binary.
func noDownloadedGeoDataFileIsNeeded(w *World) error {
	if len(w.engineCfg) == 0 {
		return errors.New("no engine config was produced")
	}
	for _, bad := range []string{"geoip:", "geosite:", "geoip.dat", "geosite.dat"} {
		if strings.Contains(string(w.engineCfg), bad) {
			return fmt.Errorf("the engine config uses %q, which needs a data file the installer does not ship", bad)
		}
	}
	return nil
}

// theHotspotSubnetAvoidsTheNetworkTheBoxIsAlreadyOn. A collision here is a
// client that reaches nothing while everything reports healthy.
func theHotspotSubnetAvoidsTheNetworkTheBoxIsAlreadyOn(w *World) error {
	if w.plan == nil {
		return errors.New("no plan")
	}
	for _, taken := range w.facts.OccupiedPrefixes() {
		if netcfg.Overlaps(w.plan.HotspotSubnet, taken) {
			return fmt.Errorf("the hotspot subnet %v overlaps %v, which this machine is already on", w.plan.HotspotSubnet, taken)
		}
		if netcfg.Overlaps(w.plan.TunSubnet, taken) {
			return fmt.Errorf("the tunnel subnet %v overlaps %v", w.plan.TunSubnet, taken)
		}
	}
	if netcfg.Overlaps(w.plan.HotspotSubnet, w.plan.TunSubnet) {
		return fmt.Errorf("the hotspot subnet %v and the tunnel subnet %v overlap each other", w.plan.HotspotSubnet, w.plan.TunSubnet)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shared helpers for the steps above
// ---------------------------------------------------------------------------

// ruleset returns the firewall text the box would load, with any deliberate
// defect applied.
func (w *World) ruleset() (string, error) {
	if w.plan == nil {
		return "", errors.New("no plan was produced, so there is no ruleset")
	}
	return w.defs.mutateRuleset(w.plan.Ruleset()), nil
}

func (w *World) forwardChain() (nftRuleset, nftChain, error) {
	text, err := w.ruleset()
	if err != nil {
		return nftRuleset{}, nftChain{}, err
	}
	rs, err := parseNft(text)
	if err != nil {
		return nftRuleset{}, nftChain{}, err
	}
	c, err := rs.chain("forward")
	return rs, c, err
}

func ruleTexts(rs []nftRule) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.text)
	}
	return out
}

// everythingAUserOrALogCanSee is every string this flow produced that could
// reach a screen, a log file, a support bundle or a file another account on the
// box can read. The two places a credential belongs, the engine config document
// and the 0600 state file, are deliberately absent.
func (w *World) everythingAUserOrALogCanSee() map[string]string {
	out := map[string]string{}

	logs := w.defs.mutateLogLines(w.logs)
	out["the appliance log"] = strings.Join(logs, "\n")
	out["the event timeline"] = strings.Join(w.tl.all(), "\n")

	var errTexts []string
	for _, e := range w.errs {
		errTexts = append(errTexts, e.Error())
	}
	out["every error the flow produced"] = strings.Join(errTexts, "\n")

	if w.problem.HeadlineText() != "" || w.problem.Advice != "" {
		out["the message shown to the user"] = w.problem.Text()
	}
	if w.lnk != nil {
		out["the panel's description of the config"] = w.lnk.Redacted() + "\n" + fmt.Sprintf("%v %+v %#v", w.lnk, w.lnk, w.lnk)
	}
	out["the saved state, rendered for diagnostics"] = w.store.Snapshot().Redacted() +
		"\n" + fmt.Sprintf("%v %+v %#v", w.store.Snapshot(), w.store.Snapshot(), w.store.Snapshot())

	if w.plan != nil {
		out["the generated firewall ruleset"] = w.plan.Ruleset()
		out["the plan explained to the user"] = w.plan.Explain() + "\n" + strings.Join(w.plan.Notes, "\n")
		var whys []string
		for _, s := range append(append([]netcfg.Step{}, w.preSteps...), w.postSteps...) {
			whys = append(whys, s.Op+" "+s.Why+" "+s.Do.String()+" "+s.Undo.String())
		}
		out["every command the box ran, as a log would render it"] = strings.Join(whys, "\n")
	}
	out["the DHCP and DNS configuration"] = w.hotspotPlan.DnsmasqConf
	if w.eng != nil {
		var lines []string
		for _, e := range w.eng.Logs() {
			lines = append(lines, e.Text)
		}
		out["the engine log the panel shows in advanced mode"] = strings.Join(lines, "\n")
	}
	if b, err := os.ReadFile(w.journalPath()); err == nil {
		out["the teardown journal on disk"] = string(b)
	}
	if w.detection.InternetInterface != "" {
		out["the detected line in basic mode"] = panel.DetectedLine(w.detection) +
			"\n" + panel.DeviceCountLine(panel.LangEN, w.status.Hotspot)
	}
	out["the request that crosses the privilege boundary, as a log would render it"] =
		fmt.Sprintf("%v", panel.StartRequest{
			ConfigJSON: w.engineCfg,
			Hotspot: panel.HotspotSpec{
				SSID:       w.hotspotPlan.AP.SSID,
				Passphrase: w.hotspotPlan.AP.Passphrase,
				Interface:  w.hotspotPlan.AP.Interface,
			},
		})
	return out
}

// acceptByUplinkName is the defect the uplink scenario is watched failing
// against: a forward chain that permits client traffic by naming interfaces
// other than the tunnel. A ruleset built like that really would stop blocking
// when the internet moved to an interface it does not name, which is the claim
// docs/BEHAVIOUR.md used to make about the ruleset this product actually builds.
func acceptByUplinkName(ruleset string) string {
	return strings.Replace(ruleset,
		`iifname "ap0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"`,
		`iifname "ap0" oifname "eth0" accept comment "INJECTED DEFECT: permits by uplink name"`,
		1)
}
