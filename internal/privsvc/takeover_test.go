// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// The command the Broadcom driver on the target refuses, and the error it
// returns. MEASURED on a real Pi on 2026-08-30 by the coordinator: phy0
// advertises a combination allowing an access point beside the station link,
// and this command fails while wlan0 is associated.
const (
	createIfaceCmd  = "iw phy phy0 interface add ap0 type __ap"
	driverRefusal   = "command failed: Input/output error (-5)"
	takeoverAddrCmd = "ip address add 10.83.51.1/24 dev wlan0"
	takeoverRuleCmd = "ip rule add from 10.83.51.0/24 lookup 8410 priority 8410"
	// The inverse of a step the first plan DID apply. The create-iface step is
	// third in the pre-engine list, after the firewall and the kernel knobs
	// and before the hotspot address, so forwarding being turned back off is
	// the marker that the first plan was undone.
	firstPlanUndoCmd = "sysctl -w net.ipv4.ip_forward=0"
)

// refuseSecondInterface makes the radio behave the way the target's does: it
// advertises the combination and refuses to create the interface.
func refuseSecondInterface(w *world) {
	w.runner.SetError(createIfaceCmd, errors.New(driverRefusal))
}

// TestARadioThatRefusesASecondInterfaceFallsBackToTakingOverTheExistingOne.
//
// A capability table is not proof the operation works. This is the whole
// reason internal/netcfg made the fallback a runtime one rather than a planner
// rule, and this service is what has to act on it.
func TestARadioThatRefusesASecondInterfaceFallsBackToTakingOverTheExistingOne(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("the start did not fall back and failed instead: %v\ntimeline:%s", err, w.tl)
	}

	// The access point ended up on the interface that already existed.
	st, err := w.svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Detection.HotspotInterface != "wlan0" {
		t.Fatalf("the hotspot is on %q, want wlan0: the fallback takes over the existing interface",
			st.Detection.HotspotInterface)
	}
	if !st.Connected() {
		t.Fatalf("the box is not carrying client traffic after a successful fallback: %+v", st)
	}

	// The first choice was actually tried. A service that planned the takeover
	// up front would pass every other assertion here and would cost the user
	// their other WiFi connection on every box, including the ones where
	// creating a second interface works.
	if w.tl.indexOf("net: "+createIfaceCmd) < 0 {
		t.Fatalf("the second interface was never attempted, so the fallback was chosen without trying the "+
			"option that costs the user nothing\ntimeline:%s", w.tl)
	}
	// And the fallback's own steps went on.
	if w.tl.indexOf("net: "+takeoverAddrCmd) < 0 {
		t.Fatalf("the hotspot address was never put on the taken-over interface\ntimeline:%s", w.tl)
	}
}

// TestTheFirstPlanIsUndoneBeforeTheFallbackIsApplied.
//
// Both plans touch the same firewall, the same kernel knobs and the same
// hotspot address, and the second names a different interface. Applying the
// fallback on top of a half-applied first plan leaves the journal describing a
// machine that never existed.
func TestTheFirstPlanIsUndoneBeforeTheFallbackIsApplied(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	undo := w.tl.indexOf("net: " + firstPlanUndoCmd)
	apply := w.tl.indexOf("net: " + takeoverAddrCmd)
	if undo < 0 {
		t.Fatalf("the first plan's hotspot address was never taken back off\ntimeline:%s", w.tl)
	}
	if apply < 0 {
		t.Fatalf("the fallback never put its hotspot address on\ntimeline:%s", w.tl)
	}
	if undo > apply {
		t.Fatalf("the fallback was applied before the first plan was undone\ntimeline:%s", w.tl)
	}

	// One configuration is in force at the end, not two.
	entries, err := netcfg.LoadJournal(w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("reading the journal: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(netcfg.RunnerKey(e.Do), " dev ap0") {
			t.Fatalf("the journal still holds a change to the interface that was never created: %s",
				netcfg.RunnerKey(e.Do))
		}
	}
}

// TestTheTakeoverSaysWhatItCost.
//
// Taking over an interface ends whatever WiFi connection it holds. On the
// target that is harmless because the uplink is wired, but somebody whose
// other connection drops is otherwise left guessing.
func TestTheTakeoverSaysWhatItCost(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}

	lg, err := w.svc.EngineLog(context.Background())
	if err != nil {
		t.Fatalf("engine log: %v", err)
	}
	var shown strings.Builder
	for _, e := range lg.Entries {
		shown.WriteString(e.Text)
		shown.WriteString("\n")
	}

	// The sentences are internal/netcfg's, not this package's, so they are
	// matched on substance rather than word for word. Three things have to
	// reach somebody whose other WiFi connection is about to drop: which
	// interface the hotspot moved to, that the interface has to come off the
	// network it is on, and that the internet connection was not the thing
	// taken.
	//
	// This assertion was rewritten on 2026-08-30 because the sentence it used
	// to look for, that the takeover "ends the WiFi connection wlan0 currently
	// holds", described an effect no code produced: the takeover renamed the
	// interface in the plan and released nothing. The words are now true and
	// the steps that make them true are asserted separately, by
	// TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease, because a test
	// on the prose alone is what let the false sentence stand.
	for _, want := range []struct {
		what  string
		anyOf []string
	}{
		{"which interface the hotspot moved to", []string{"hotspot is planned on wlan0", "Hotspot: WiFi on wlan0"}},
		{"that wlan0 has to come off the network it is on", []string{
			"must first be released", "has to be disconnected from the WiFi network it is on"}},
		{"that the internet connection was left alone", []string{"eth0 is not touched", "eth0 is untouched"}},
	} {
		if !containsAny(shown.String(), want.anyOf) {
			t.Fatalf("the advanced view does not say %s:\n%s", want.what, shown.String())
		}
	}
	if !containsAny(w.logs.String(), []string{"must first be released", "has to be disconnected"}) {
		t.Fatalf("the log does not say what the takeover cost:\n%s", w.logs.String())
	}
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// TestAFailingStepReachesTheLogAndTheAdvancedView.
//
// This is the gap a real start exposed. The fault reached the panel as
// "unknown", the advanced view had nothing, and the log named neither the
// command nor the error. The command is the fact that made it diagnosable.
func TestAFailingStepReachesTheLogAndTheAdvancedView(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}

	lg, err := w.svc.EngineLog(context.Background())
	if err != nil {
		t.Fatalf("engine log: %v", err)
	}
	var shown strings.Builder
	for _, e := range lg.Entries {
		shown.WriteString(e.Text)
		shown.WriteString("\n")
	}

	for _, where := range []struct {
		name string
		text string
	}{
		{"the advanced view", shown.String()},
		{"the log", w.logs.String()},
	} {
		if !strings.Contains(where.text, createIfaceCmd) {
			t.Errorf("%s does not name the command that failed (%q):\n%s", where.name, createIfaceCmd, where.text)
		}
		if !strings.Contains(where.text, "Input/output error (-5)") {
			t.Errorf("%s does not carry the error the driver returned:\n%s", where.name, where.text)
		}
	}
}

// TestTheServerAddressNeverAppearsInADiagnosticLine.
//
// The pinned host route carries the user's proxy server address in its
// argument vector. docs/LAYOUT.md says the config is never printed or logged,
// and docs/INSTALL.md makes the same point about the uninstaller's replay. A
// diagnostic that prints the command in full has to take that one value out.
func TestTheServerAddressNeverAppearsInADiagnosticLine(t *testing.T) {
	w := newWorld(t, func(w *world) {
		// This test models Linux commands even when run on Windows. No
		// daemon starts: the fake runner fails the earlier server route.
		w.cfg.HotspotPaths.LeaseFile = "/var/lib/caspian/dnsmasq.leases"
		w.cfg.HotspotPaths.HostapdControlDir = "/run/hostapd"
	})
	w.runner.SetError(serverRouteCmd, errors.New("RTNETLINK answers: Network is unreachable"))

	if err := w.svc.Start(context.Background(), startRequest(t)); err == nil {
		t.Fatalf("a start whose pinned host route failed reported success")
	}

	lg, err := w.svc.EngineLog(context.Background())
	if err != nil {
		t.Fatalf("engine log: %v", err)
	}
	var shown strings.Builder
	for _, e := range lg.Entries {
		shown.WriteString(e.Text)
		shown.WriteString("\n")
	}

	// The failure IS reported, with the operation and the error.
	if !strings.Contains(shown.String(), "Network is unreachable") {
		t.Fatalf("the route failure was not reported at all:\n%s", shown.String())
	}
	if !strings.Contains(shown.String(), "[server]") {
		t.Fatalf("the command was not reported with the server address taken out:\n%s", shown.String())
	}
	// And the address itself is in neither.
	for _, where := range []struct{ name, text string }{
		{"the advanced view", shown.String()},
		{"the log", w.logs.String()},
	} {
		if strings.Contains(where.text, "203.0.113.7") {
			t.Fatalf("%s contains the user's proxy server address:\n%s", where.name, where.text)
		}
	}
}

// TestTheTakeoverIsRefusedWhenTheOnlyCandidateIsTheUplink.
//
// Taking the interface that carries the internet would cut off the connection
// the box exists to share. internal/netcfg refuses in words a person can act
// on; this service passes the refusal through rather than reinterpreting it.
func TestTheTakeoverIsRefusedWhenTheOnlyCandidateIsTheUplink(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.runner = newRecordedMachine(fixtureIWList, fixtureIWRegGet)
		// The internet arrives over WiFi on wlan0, which is also the only
		// radio that could host the access point.
		w.runner.SetOutput("ip route show default", fixtureIPRouteDefaultWirelessOnly)
		w.cfg.Runner = tracedRunner{inner: w.runner, tl: w.tl}
	})
	refuseSecondInterface(w)

	err := w.svc.Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("the hotspot took over the interface carrying the internet\ntimeline:%s", w.tl)
	}
	if got := panel.FaultOf(err); got != panel.FaultNoAPAdapter {
		t.Fatalf("fault was %q, want %q: the remedy is a USB adapter or a cable, which is what that fault says",
			got, panel.FaultNoAPAdapter)
	}

	// netcfg's own sentence reached the advanced view rather than being
	// replaced by one written here.
	lg, _ := w.svc.EngineLog(context.Background())
	var shown strings.Builder
	for _, e := range lg.Entries {
		shown.WriteString(e.Text)
		shown.WriteString("\n")
	}
	if !strings.Contains(shown.String(), "cut off the internet") {
		t.Fatalf("internal/netcfg's refusal did not reach the advanced view:\n%s", shown.String())
	}

	// Nothing is left applied.
	assertMachineRestored(t, w)
	if w.svc.isRunning() {
		t.Fatalf("the service reports itself running after a refused takeover")
	}
	if _, statErr := os.Stat(w.cfg.JournalPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the journal survived a refused takeover")
	}
}

// TestAFailureThatIsNotTheInterfaceDoesNotTriggerTheFallback.
//
// The fallback costs the user a WiFi connection. It is only ever right for the
// one failure it was measured against, and a service that reached for it on
// any failure would take somebody's connection away to fix a broken route.
func TestAFailureThatIsNotTheInterfaceDoesNotTriggerTheFallback(t *testing.T) {
	w := newWorld(t)
	w.runner.SetError(serverRouteCmd, errors.New("RTNETLINK answers: Network is unreachable"))

	if err := w.svc.Start(context.Background(), startRequest(t)); err == nil {
		t.Fatalf("a start whose pinned host route failed reported success")
	}
	if w.tl.indexOf("net: "+takeoverAddrCmd) >= 0 {
		t.Fatalf("a route failure made the hotspot take over an existing interface\ntimeline:%s", w.tl)
	}
	assertMachineRestored(t, w)
}

// TestTheEngineAndThisServiceShareOneOrderedLog.
//
// The two rings describe one event. Reading them as two lists means working
// out the order by hand, which is what somebody helping a user has to do at
// exactly the moment they have least patience for it.
func TestTheEngineAndThisServiceShareOneOrderedLog(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}

	lg, err := w.svc.EngineLog(context.Background())
	if err != nil {
		t.Fatalf("engine log: %v", err)
	}
	if len(lg.Entries) < 2 {
		t.Fatalf("the merged log has %d entries, so it cannot be showing both rings", len(lg.Entries))
	}
	for i := 1; i < len(lg.Entries); i++ {
		if lg.Entries[i].At.Before(lg.Entries[i-1].At) {
			t.Fatalf("entry %d is older than the one before it, so the merge is not in time order", i)
		}
	}
	// Both rings are represented: the engine's line and this service's.
	var shown strings.Builder
	for _, e := range lg.Entries {
		shown.WriteString(e.Text)
		shown.WriteString("\n")
	}
	if !strings.Contains(shown.String(), "a redacted engine line") {
		t.Fatalf("the engine's own ring is missing from the merge:\n%s", shown.String())
	}
	if !strings.Contains(shown.String(), createIfaceCmd) {
		t.Fatalf("this service's own ring is missing from the merge:\n%s", shown.String())
	}
}

// TestNoCredentialReachesTheAdvancedView.
//
// The advanced view is a new path out of this service, so the rule that
// applies to every other one applies to it.
func TestNoCredentialReachesTheAdvancedView(t *testing.T) {
	w := newWorld(t)
	refuseSecondInterface(w)
	_ = w.svc.Start(context.Background(), startRequest(t))

	bad := startRequest(t)
	bad.Hotspot.Passphrase = "SecurePass123"
	_ = w.svc.Start(context.Background(), bad)

	lg, err := w.svc.EngineLog(context.Background())
	if err != nil {
		t.Fatalf("engine log: %v", err)
	}
	var shown strings.Builder
	for _, e := range lg.Entries {
		shown.WriteString(e.Text)
		shown.WriteString("\n")
	}
	if shown.Len() == 0 {
		t.Fatalf("nothing was recorded, so this test would pass on a service that records nothing")
	}
	for _, sec := range secrets() {
		if strings.Contains(shown.String(), sec) {
			t.Fatalf("a credential reached the advanced view: %q\n%s", sec, shown.String())
		}
	}
}

// TestAHotspotFailureReachesTheLogAndTheAdvancedView.
//
// The same defect as the network-step one, a layer over. internal/hotspot has
// already turned the daemon's output into a sentence, and this service was
// using that sentence only to pick a Fault and then dropping it, so the fact
// that makes a hostapd failure diagnosable reached nobody.
func TestAHotspotFailureReachesTheLogAndTheAdvancedView(t *testing.T) {
	w := newWorld(t)
	failHostapd(w)

	if err := w.svc.Start(context.Background(), startRequest(t)); err == nil {
		t.Fatalf("a start whose access point would not come up reported success")
	}

	shown := advancedView(t, w)
	for _, where := range []struct{ name, text string }{
		{"the advanced view", shown},
		{"the log", w.logs.String()},
	} {
		if !strings.Contains(where.text, "access point did not come up") {
			t.Errorf("%s does not say the access point failed:\n%s", where.name, where.text)
		}
		// internal/hotspot's own words for a failure it does not recognise,
		// which carry the daemon's message.
		if !strings.Contains(where.text, "does not recognise the reason") {
			t.Errorf("%s does not carry internal/hotspot's explanation:\n%s", where.name, where.text)
		}
		if !strings.Contains(where.text, "it did not work") {
			t.Errorf("%s does not carry what the daemon actually reported:\n%s", where.name, where.text)
		}
	}
}

// TestAnEngineRefusalReachesTheLogAndTheAdvancedView.
//
// "The engine rejected it" is one of the three states design section 8 step 11
// requires the panel to tell apart, and the engine's own words are the only
// thing that says WHICH part it rejected. They are already redacted by
// internal/engine.
func TestAnEngineRefusalReachesTheLogAndTheAdvancedView(t *testing.T) {
	w := newWorld(t)
	w.eng.startErr = errors.New("start: the engine refused this configuration")

	if err := w.svc.Start(context.Background(), startRequest(t)); err == nil {
		t.Fatalf("a start whose engine refused the configuration reported success")
	}
	shown := advancedView(t, w)
	for _, where := range []struct{ name, text string }{
		{"the advanced view", shown},
		{"the log", w.logs.String()},
	} {
		if !strings.Contains(where.text, "engine would not start") {
			t.Errorf("%s does not say the engine refused:\n%s", where.name, where.text)
		}
		if !strings.Contains(where.text, "the engine refused this configuration") {
			t.Errorf("%s does not carry the engine's own words:\n%s", where.name, where.text)
		}
	}
}

// advancedView renders what the panel would show in advanced mode.
func advancedView(t *testing.T, w *world) string {
	t.Helper()
	lg, e := w.svc.EngineLog(context.Background())
	if e != nil {
		t.Fatalf("engine log: %v", e)
	}
	var b strings.Builder
	for _, entry := range lg.Entries {
		b.WriteString(entry.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// TestTheJournalLeftByTheFailedRunOnTheTargetConvergesOnRecovery.
//
// This reproduces the exact leftover a real start on the target produced on
// 2026-08-30: five steps applied, the sixth (creating the access point's
// interface) refused by the driver, five rolled back and ONE journal entry
// left. Its inverse is "iw dev ap0 del" for an interface that was never
// created, which fails with "No such device (-19)".
//
// internal/netcfg now treats an inverse whose object is already gone as having
// achieved what it wanted (idempotence.go, notFoundMarkers), so the entry
// clears instead of being retried on every start for ever. This is that fix
// exercised through THIS package's recovery path, which is the path the box in
// front of somebody will actually take on its next start.
func TestTheJournalLeftByTheFailedRunOnTheTargetConvergesOnRecovery(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	// Build the leftover the way the target built it: apply until the driver
	// refuses, then walk away without tearing down, which is what a killed
	// process leaves.
	refuseSecondInterface(w)
	facts, plan, err := netcfg.DetectAndPlan(ctx, w.runner,
		[]netip.Addr{netip.MustParseAddr("203.0.113.7")}, w.cfg.netOptions())
	if err != nil {
		t.Fatalf("planning the previous run: %v", err)
	}
	ap, err := netcfg.NewApplier(w.runner, w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	if _, applyErr := ap.Apply(ctx, plan.PreEngineSteps(facts.Sysctl)); applyErr == nil {
		t.Fatalf("the create-interface step was expected to fail and did not")
	}
	if err := ap.Close(); err != nil {
		t.Fatalf("closing the journal: %v", err)
	}

	entries, err := netcfg.LoadJournal(w.cfg.JournalPath)
	if err != nil {
		t.Fatalf("reading the leftover journal: %v", err)
	}
	found := false
	for _, e := range entries {
		if netcfg.RunnerKey(e.Undo) == "iw dev ap0 del" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the leftover journal does not hold the inverse for the interface that was never created, "+
			"so this test is not reproducing the event it is named for; it holds %d entries", len(entries))
	}

	// The kernel's answer for removing an interface that does not exist.
	w.runner.SetError("iw dev ap0 del", errors.New("command failed: No such device (-19)"))
	w.runner.Reset()

	rep, err := w.svc.ReplayJournal(ctx)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if rep.Failed != 0 {
		t.Fatalf("recovery reported %d failed inverses; the one for an interface that was never created "+
			"has to converge, or every later start retries it for ever", rep.Failed)
	}
	if _, statErr := os.Stat(w.cfg.JournalPath); !errors.Is(statErr, os.ErrNotExist) {
		left, _ := netcfg.LoadJournal(w.cfg.JournalPath)
		t.Fatalf("the journal survived recovery with %d entries, so the next start would retry them", len(left))
	}
	// And it really did try, rather than converging by never running anything.
	tried := false
	for _, c := range w.runner.Commands() {
		if netcfg.RunnerKey(c) == "iw dev ap0 del" {
			tried = true
		}
	}
	if !tried {
		t.Fatalf("the inverse was never attempted, so the journal was cleared without undoing anything")
	}
}
