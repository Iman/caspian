// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/panel"
)

// behaviours is the whole suite. Each entry is a sentence a non-programmer can
// check, and the steps that check it.
//
// Every entry names, in BreaksWhen, the defect that must make it fail.
// TestEveryScenarioCanFail injects that defect and requires red, so the claim
// "this scenario is capable of failing" is a test result and not a promise.
func behaviours() []*scenario {
	return []*scenario{

		// ---------------------------------------------------------------
		// The product's first claim: a person pastes a config and the
		// devices in the room work.
		// ---------------------------------------------------------------
		Scenario("a pasted config brings the hotspot up and carries client traffic").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theConfigIsSavedOnDisk).
			And(aPlanIsProduced).
			And(theEngineIsRunning).
			And(theAccessPointIsBeaconing).
			And(thePanelReportsConnected).
			And(thePanelNamesWhatItDetected).
			And(clientTrafficLeavesOnlyThroughTheTunnel).
			BreaksWhen("the forward chain gains an accept rule that does not name the tunnel",
				func(d *defects) { d.ruleset = addAnAcceptThatDoesNotNameTheTunnel }),

		Scenario("the panel is reachable from the hotspot and never from a public address").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(thePanelIsServedOnTheHotspotAndNeverOnAPublicAddress).
			BreaksWhen("the panel falls back to the local network when the hotspot has no address yet",
				func(d *defects) { d.detection = fallBackToTheLocalNetwork }),

		// ---------------------------------------------------------------
		// Ordering. Both of these exist because getting them wrong opens a
		// window or silently misses the tunnel, and neither is visible in
		// the finished state: only the sequence shows it.
		// ---------------------------------------------------------------
		Scenario("the firewall is in force before the box will forward a packet").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theFirewallIsLoadedBeforeForwardingIsEnabled).
			And(theFirewallDoesNotWaitForTheTunnel).
			BreaksWhen("the ruleset is loaded after forwarding has been enabled",
				func(d *defects) { d.firewallAfterForwarding = true }),

		Scenario("everything that needs the tunnel device waits for the engine to make it").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theRPFilterDefaultIsSetBeforeTheEngineStarts).
			And(everyStepThatNamesTheTunnelWaitsForTheEngine).
			BreaksWhen("the tunnel steps are applied before the engine has created the device",
				func(d *defects) { d.postEngineStepsBeforeEngine = true }),

		Scenario("the engine's own connection to the server never enters the tunnel it is building").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theEngineReachesTheServerOutsideTheTunnel).
			BreaksWhen("the pinned host route to the server is left out of the plan",
				func(d *defects) { d.dropPinnedServerRoute = true }),

		// ---------------------------------------------------------------
		// Fail closed. When the tunnel device disappears the kernel drops
		// every route through it, so "the engine stopped" produces a leak
		// by default rather than a stop.
		// ---------------------------------------------------------------
		Scenario("with the tunnel gone, nothing lets client traffic out by the uplink").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			And(theTunnelDrops).
			Then(withTheTunnelGoneNoRuleLetsClientTrafficOut).
			And(theBoxNeverMasqueradesOntoTheUplink).
			And(everyInterfaceMatchIsByNameAndNotByIndex).
			BreaksWhen("the forward chain gains an accept rule that does not name the tunnel",
				func(d *defects) { d.ruleset = addAnAcceptThatDoesNotNameTheTunnel }),

		Scenario("clients are never offered the IPv6 the tunnel cannot carry").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(noClientIPv6IsOffered).
			BreaksWhen("the IPv6 drop rules are taken out of the forward chain",
				func(d *defects) { d.ruleset = removeTheIPv6Drops }),

		Scenario("a client's DNS question is answered on this box and resolved through the tunnel").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theHotspotForwardsClientDNSToTheEnginesOwnListener).
			And(clientQueriesLeaveOnlyByTheTunnel).
			BreaksWhen("the resolver's own queries are routed direct instead of into the tunnel",
				func(d *defects) { d.engineCfg = resolveClientQueriesDirectInsteadOfThroughTheTunnel }),

		Scenario("the box offers itself as the resolver and never names another").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theOnlyResolverOfferedToJoiningDevicesIsThisBox).
			BreaksWhen("the DHCP offer names a public resolver instead of this box",
				func(d *defects) { d.dnsmasqConf = offerAPublicResolverToJoiningDevices }),

		Scenario("a client cannot reach a resolver of its own choosing").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(clientDNSCannotEscapeToAResolverOfItsOwnChoosing).
			BreaksWhen("the DNS redirect is taken out of prerouting, leaving client DNS merely permitted",
				func(d *defects) { d.ruleset = removeTheDNSRedirect }),

		// ---------------------------------------------------------------
		// Teardown. Turning the switch off returns the machine to how it
		// was, and it has to survive the process being killed.
		// ---------------------------------------------------------------
		Scenario("turning the switch off returns every change the box made").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			And(theUserPressesDisconnect).
			Then(everyRecordedChangeIsUndone).
			And(nothingTheBoxChangedIsStillInPlace).
			And(theJournalIsGone).
			And(nothingIsLeftRunning).
			BreaksWhen("a routing change is made without first recording how to undo it",
				func(d *defects) { d.skipTeardownOfRoutes = true }),

		Scenario("a teardown replayed from the journal of a killed process undoes the same changes").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			And(theProcessIsKilledAndTheBoxRestarts).
			Then(everyRecordedChangeIsUndoneByTheReplay).
			And(nothingTheBoxChangedIsStillInPlace).
			And(theJournalIsGone).
			BreaksWhen("a routing change is made without first recording how to undo it",
				func(d *defects) { d.skipTeardownOfRoutes = true }),

		Scenario("a change of uplink leaves the box blocked and waiting for a reconnect").
			Given(aBoxThatIsAlreadyConnected).
			When(theInternetMovesToADifferentInterface).
			Then(theBoxDoesNotNoticeTheUplinkMoved).
			And(theBlockStillStopsClientTrafficByTheNewUplink).
			BreaksWhen("the forward chain accepts client traffic that does not name the tunnel, so a "+
				"ruleset naming the old uplink stops blocking when the internet moves",
				func(d *defects) { d.ruleset = acceptByUplinkName }),

		Scenario("a box killed halfway through cleans up before it does anything else").
			Given(aBoxLeftHalfConfiguredByAKilledProcess).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theLeftoverChangesAreUndoneFirst).
			BreaksWhen("the journal left by the killed process is not replayed at start",
				func(d *defects) { d.skipRecovery = true }),

		// ---------------------------------------------------------------
		// Bad input. Three states, worded three ways, because they need
		// three different things from the user.
		// ---------------------------------------------------------------
		Scenario("text that is not a link at all is refused before anything is touched").
			Given(aFreshBox).
			And(textThatIsNotALinkAtAll).
			When(theUserPressesConnect).
			Then(theUserIsToldTheTextCouldNotBeRead).
			And(theMachineWasNotTouched).
			And(theThreeBadConfigMessagesAreDifferent).
			BreaksWhen("the box looks at the machine before it reads the pasted text",
				func(d *defects) { d.detectBeforeParse = true }),

		Scenario("a link the engine will not accept is told apart from one that would not parse").
			Given(aFreshBox).
			And(aLinkTheEngineWillNotAccept).
			When(theUserPressesConnect).
			Then(theUserIsToldTheLinkCannotBeUsedAsWritten).
			And(theMachineWasNotTouched).
			BreaksWhen("every failure is worded as a failure to read the pasted text",
				func(d *defects) { d.classifyEveryFailureAsParse = true }),

		Scenario("a link whose server never answers is not blamed on the link").
			Given(aFreshBox).
			And(aValidRealityLink).
			And(aProxyServerThatNeverAnswers).
			When(theUserPressesConnect).
			Then(theUserIsToldTheServerDidNotAnswer).
			And(clientsGetNothingRatherThanAWayOut).
			BreaksWhen("every failure is worded as a failure to read the pasted text",
				func(d *defects) { d.classifyEveryFailureAsParse = true }),

		// ---------------------------------------------------------------
		// Secrets.
		// ---------------------------------------------------------------
		Scenario("the pasted credential never reaches a screen, a log or a readable file").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theCredentialAppearsInNothingTheUserOrALogCanSee).
			And(theCredentialReachesOnlyTheEngineConfigAndTheStateFile).
			BreaksWhen("one log line records the config as it was pasted",
				func(d *defects) { d.logLines = logTheConfigAsPasted }),

		Scenario("the hotspot password reaches the access point and nothing else").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theHotspotPasswordReachesTheAccessPointAndNothingElse).
			And(theCredentialAppearsInNothingTheUserOrALogCanSee).
			BreaksWhen("one log line records the hotspot password",
				func(d *defects) { d.logLines = logTheHotspotPassword }),

		// ---------------------------------------------------------------
		// Idempotence. The panel's switch, a reconnect after a drop and a
		// health check that decides to repair all reach the same code.
		// ---------------------------------------------------------------
		Scenario("pressing connect twice does not restart a working hotspot").
			Given(aBoxThatIsAlreadyConnected).
			When(theUserPressesConnectAgain).
			Then(theBoxConnects).
			And(onlyOneEngineIsRunning).
			And(theAccessPointWasNotRestarted).
			BreaksWhen("the hotspot configuration is regenerated differently on every connect",
				func(d *defects) { d.restartHotspotEveryConnect = true }),

		// ---------------------------------------------------------------
		// Machines that cannot do the job, told in words the audience can
		// act on.
		// ---------------------------------------------------------------
		Scenario("a machine whose radio cannot make a hotspot is told what to go and buy").
			Given(aFreshBox).
			And(aBoxWhoseOnlyRadioCannotCreateAHotspot).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theUserIsToldNoAdapterCanCreateAHotspot).
			BreaksWhen("a refusal loses its typed reason on the way to the panel",
				func(d *defects) { d.swallowRefusals = true }),

		Scenario("a machine with no internet connection of its own is told to plug something in").
			Given(aFreshBox).
			And(aBoxWithNoInternetConnectionOfItsOwn).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theUserIsToldTheBoxHasNoInternetConnection).
			BreaksWhen("a refusal loses its typed reason on the way to the panel",
				func(d *defects) { d.swallowRefusals = true }),

		Scenario("the hotspot takes an address range that does not clash with the network the box is on").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theHotspotSubnetAvoidsTheNetworkTheBoxIsAlreadyOn).
			BreaksWhen("the hotspot subnet is overridden to one the box is already on",
				func(d *defects) { d.collideTheHotspotSubnet = true }),

		// ---------------------------------------------------------------
		// The generated engine configuration, as a product promise rather
		// than as a package detail.
		// ---------------------------------------------------------------
		Scenario("the box needs no download and asks no Google server anything").
			Given(aFreshBox).
			And(aValidRealityLink).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(noDownloadedGeoDataFileIsNeeded).
			And(noResolverInAnyGeneratedConfigurationIsAGoogleOne).
			BreaksWhen("a Google resolver is put into the generated configuration",
				func(d *defects) { d.engineCfg = putAGoogleResolverIn }),
	}
}

// ---------------------------------------------------------------------------
// The deliberate defects, one function each so the mutation table names
// something a reader can go and look at.
// ---------------------------------------------------------------------------

// addAnAcceptThatDoesNotNameTheTunnel adds the rule somebody adds when a client
// reports that replies are not coming back. It reads as harmless. It is the
// leak: it keeps matching when the tunnel is gone, and what it permits then
// leaves by the uplink.
func addAnAcceptThatDoesNotNameTheTunnel(text string) string {
	return insertIntoChain(text, "forward", "\t\tct state established,related accept")
}

// removeTheIPv6Drops is the change made by somebody who noticed that IPv6 is
// blocked and assumed it was an oversight.
func removeTheIPv6Drops(text string) string {
	return dropLines(text, func(l string) bool {
		return strings.Contains(l, "nfproto ipv6") && strings.Contains(l, "drop")
	})
}

// removeTheDNSRedirect leaves client DNS merely permitted instead of
// redirected, so a client with a hardcoded resolver reaches it.
func removeTheDNSRedirect(text string) string {
	return dropLines(text, func(l string) bool { return strings.Contains(l, "redirect to :") })
}

func logTheConfigAsPasted(lines []string) []string {
	return append(lines, "connecting with "+realityShareLink())
}

func logTheHotspotPassword(lines []string) []string {
	return append(lines, "hotspot "+hotspotSSID+" password "+fakeHotspotPassphrase)
}

// fallBackToTheLocalNetwork is the shape design section 5.6 names as a hazard
// it does not solve: the hotspot interface does not exist until the access
// point starts, so there is a moment when the panel has no address on it. The
// tempting fix is to serve it somewhere else instead.
func fallBackToTheLocalNetwork(d *panel.Detection) {
	d.HotspotAddress = ""
}

func putAGoogleResolverIn(cfg []byte) []byte {
	return []byte(strings.Replace(string(cfg), `"9.9.9.9"`, `"8.8.8.8"`, 1))
}

// insertIntoChain puts a line immediately after the policy line of a chain.
func insertIntoChain(text, chain, line string) string {
	var out []string
	in := false
	for _, l := range strings.Split(text, "\n") {
		out = append(out, l)
		if strings.HasPrefix(strings.TrimSpace(l), "chain "+chain+" ") {
			in = true
			continue
		}
		if in && strings.Contains(l, "policy ") {
			out = append(out, line)
			in = false
		}
	}
	return strings.Join(out, "\n")
}

func dropLines(text string, drop func(string) bool) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if drop(l) {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// ---------------------------------------------------------------------------
// Running the suite
// ---------------------------------------------------------------------------

// TestBehaviour runs every scenario with no defect injected. This is the suite.
func TestBehaviour(t *testing.T) {
	for _, s := range behaviours() {
		s.run(t, defects{})
	}
}

// TestEveryScenarioCanFail is the evidence that the suite above is worth
// running. It injects each scenario's named defect and requires that scenario
// to go red.
//
// A scenario nobody has watched fail is not evidence. This is that watching,
// turned into a test so it happens on every run rather than once, by hand, on
// the day the scenario was written.
func TestEveryScenarioCanFail(t *testing.T) {
	var table []string
	table = append(table, "scenario | defect injected | result")

	for _, s := range behaviours() {
		s := s
		t.Run(s.name, func(t *testing.T) {
			if s.defect == nil {
				t.Fatalf("this scenario names no defect, so nobody has seen it fail")
			}
			var d defects
			s.defect(&d)

			w := newWorld(t, d)
			defer w.close()
			res := s.execute(w)
			if res.ok() {
				t.Errorf(
					"this scenario passed with the defect %q injected, so it does not test what it says:\n%s",
					s.defectName, res.transcript)
				table = append(table, fmt.Sprintf("%s | %s | STILL GREEN", s.name, s.defectName))
				return
			}
			table = append(table, fmt.Sprintf("%s | %s | RED at %q: %s",
				s.name, s.defectName, s.steps[res.failedAt].phrase, firstLine(res.err.Error())))
		})
	}
	t.Log("\nMUTATION TABLE\n" + strings.Join(table, "\n"))
}

// firstLine keeps the mutation table to one row per scenario. The full failure,
// timeline and all, is on the subtest that produced it.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// TestEveryScenarioNamesADefect stops a scenario being added with no way to
// prove it can fail.
func TestEveryScenarioNamesADefect(t *testing.T) {
	for _, s := range behaviours() {
		if s.defect == nil || s.defectName == "" {
			t.Errorf("scenario %q names no defect: add BreaksWhen, and watch it go red", s.name)
		}
		if len(s.steps) == 0 {
			t.Errorf("scenario %q has no steps", s.name)
		}
	}
}

// TestScenarioNamesAreSentences keeps the list readable by somebody who does
// not write Go, which is the only reason the names are written out in full.
func TestScenarioNamesAreSentences(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range behaviours() {
		if seen[s.name] {
			t.Errorf("two scenarios are called %q", s.name)
		}
		seen[s.name] = true
		if len(strings.Fields(s.name)) < 5 {
			t.Errorf("scenario name %q is too short to be a behaviour", s.name)
		}
		if s.name != strings.ToLower(s.name[:1])+s.name[1:] {
			t.Errorf("scenario name %q starts with a capital; these read as sentences in a list", s.name)
		}
	}
}

// TestBehaviourDocumentListsEveryScenario keeps docs/BEHAVIOUR.md from rotting.
// The document is the answer to "what does this product actually guarantee", so
// a scenario that is not in it is a guarantee nobody can find, and a scenario
// named in it that no longer exists is worse.
func TestBehaviourDocumentListsEveryScenario(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "BEHAVIOUR.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc := string(raw)

	var missing []string
	for _, s := range behaviours() {
		if !strings.Contains(doc, s.name) {
			missing = append(missing, s.name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("docs/BEHAVIOUR.md does not list %d scenario(s):\n  %s", len(missing), strings.Join(missing, "\n  "))
	}

	// And the other direction: a name in the document that is not a scenario.
	names := map[string]bool{}
	for _, s := range behaviours() {
		names[s.name] = true
	}
	for _, line := range strings.Split(doc, "\n") {
		l := strings.TrimSpace(line)
		if !strings.HasPrefix(l, "### ") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(l, "### "))
		if !names[name] {
			t.Errorf("docs/BEHAVIOUR.md has a section %q with no scenario behind it", name)
		}
	}
}
