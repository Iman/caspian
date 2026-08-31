// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
)

// TestTheTileRowAnswersTheFourQuestions checks the summary row.
//
// Four tiles, each answering one question with one short value: is it on, how
// many devices, which config, how long it has been up. A tile that needs a
// sentence of explanation is the wrong tile, so this also checks none of them
// carries one.
func TestTheTileRowAnswersTheFourQuestions(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test", Devices: 3})
	h.priv.SetEngineState(engine.State{
		Phase: engine.PhaseRunning,
		Since: h.clock.Now().Add(-3 * time.Hour),
	})

	_, body := h.get("/")

	for _, want := range []struct {
		what string
		key  Key
	}{
		{"is it on", MsgTileStatus},
		{"how many devices", MsgTileDevices},
		{"which config", MsgTileConfig},
		{"how long it has been up", MsgTileUptime},
	} {
		if label := h.msg(want.key); !strings.Contains(body, label) {
			t.Errorf("no tile answers %q (looked for the label %q)", want.what, label)
		}
	}

	// The values themselves.
	if !strings.Contains(body, h.msg(MsgStatusConnected)) {
		t.Error("the status tile does not say it is connected")
	}
	if !strings.Contains(body, `id="device-count"`) {
		t.Error("there is no device count tile for the poll script to update")
	}
	if want := T(h.lang, MsgUptimeHours, 3); !strings.Contains(body, want) {
		t.Errorf("the uptime tile does not say %q", want)
	}
	if !strings.Contains(body, "Home") {
		t.Error("the config tile does not name the config in use")
	}

	// A tile shows only what the redacted view exposes: no tile renders the
	// pasted config, and the passphrase is not a tile either.
	tiles := body[strings.Index(body, `class="tiles"`):strings.Index(body, "</section>")]
	if secret, found := containsAny(tiles, testLinkSecrets()); found {
		t.Errorf("a tile renders %q from the pasted config", secret)
	}
	if strings.Contains(tiles, "sun-rope-glass-mint") {
		t.Error("a tile renders the hotspot passphrase")
	}
}

// TestTheSwitchIsInTheTileRowAndProminent holds the maintainer's constraint: a
// dashboard that shows state beautifully and buries the control is a worse
// product than the one before it.
func TestTheSwitchIsInTheTileRowAndProminent(t *testing.T) {
	h := newHarness(t)
	h.ready()
	_, body := h.get("/")

	tilesAt := strings.Index(body, `class="tiles"`)
	switchAt := strings.Index(body, `action="/power"`)
	hotspotAt := strings.Index(body, `id="wifi-heading"`)
	if tilesAt < 0 || switchAt < 0 || hotspotAt < 0 {
		t.Fatal("the dashboard is missing the tiles, the switch or the hotspot section")
	}
	if switchAt < tilesAt {
		t.Error("the switch is above the tile row rather than inside it")
	}
	if switchAt > hotspotAt {
		t.Error("the switch is below the hotspot section; it has to be the first thing on the page")
	}
	// It is the large control, not a link among others. Three appearances are
	// legitimate and a fourth is a typo: teal when the box is off, soft green
	// when it is on and carrying traffic, dusty brown when it is on and not,
	// which is the cut and the fault. The set is checked rather than the
	// prefix, so a misspelled class still fails here.
	appearances := []string{`class="big go"`, `class="big ok"`, `class="big danger"`}
	found := ""
	for _, a := range appearances {
		if strings.Contains(body, a) {
			found = a
			break
		}
	}
	if found == "" {
		t.Errorf("the switch is not rendered as the large control in any known appearance %v", appearances)
	}
}

// TestThePageOrderIsTheOneTheDesignArguedFor pins the order below the tiles.
func TestThePageOrderIsTheOneTheDesignArguedFor(t *testing.T) {
	h := newHarness(t)
	h.ready()
	_, body := h.get("/")

	order := []struct {
		what   string
		marker string
	}{
		{"the tile row", `class="tiles"`},
		{"the hotspot", `id="wifi-heading"`},
		{"the config box", `id="config-heading"`},
		{"the detected line", `id="detected"`},
		{"events", `id="events-heading"`},
		{"the advanced link", `class="advanced-toggle"`},
	}
	last := -1
	for _, o := range order {
		at := strings.Index(body, o.marker)
		if at < 0 {
			t.Fatalf("%s is missing from the page", o.what)
		}
		if at < last {
			t.Errorf("%s is out of order", o.what)
		}
		last = at
	}
}

// TestStateIsNotCarriedByColourAlone is the rule the palette makes sharper: the
// soft green and the dusty brown are close in lightness and both desaturated,
// so as a pair they are weak for reduced colour vision and weak again in
// daylight. The word and the shape have to carry the state on their own.
func TestStateIsNotCarriedByColourAlone(t *testing.T) {
	h := newHarness(t)
	h.ready()

	// Off.
	_, off := h.get("/")
	if !strings.Contains(off, "dot-off") {
		t.Error("the off state has no shape class")
	}
	if !strings.Contains(off, h.msg(MsgStatusOff)) {
		t.Error("the off state has no word")
	}

	// On.
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test"})
	h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
	_, on := h.get("/")
	if !strings.Contains(on, "dot-on") {
		t.Error("the connected state has no shape class")
	}
	if !strings.Contains(on, h.msg(MsgStatusConnected)) {
		t.Error("the connected state has no word")
	}
	if strings.Contains(on, "dot-off") {
		t.Error("both shape classes are on the page at once, so the shape says nothing")
	}

	// The three shapes have to be visually different without colour, which
	// means different border styles and fills rather than three colours.
	css, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, want := range []string{".dot-on", ".dot-off", ".dot-working"} {
		if !strings.Contains(text, want) {
			t.Errorf("the stylesheet has no rule for %s", want)
		}
	}
	if !strings.Contains(text, "dashed") {
		t.Error("the three markers are not distinguished by border style, so they may differ only by colour")
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

// TestEventsAreSentencesInBothLanguagesAndCarryNothingSecret is the events area
// requirement: entries are sentences rather than log lines, in both languages,
// carrying nothing secret.
//
// They carry nothing secret by construction rather than by filtering: an Event
// is a closed vocabulary value and a timestamp, with nowhere to put a string.
func TestEventsAreSentencesInBothLanguagesAndCarryNothingSecret(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test"})

	// Do things that produce events.
	h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
	h.postForm("/hotspot", url.Values{
		"csrf": {h.tokenOn("/")}, "ssid": {"Caspian-test2"}, "passphrase": {"sun-rope-glass-mint"},
	})
	h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"0"}})

	events := h.panel.events.entries()
	if len(events) < 3 {
		t.Fatalf("only %d events were recorded, so this test checks little", len(events))
	}

	// Newest first, which is the order a dashboard wants.
	for i := 1; i < len(events); i++ {
		if events[i].At.After(events[i-1].At) {
			t.Error("the events are not newest first")
			break
		}
	}

	for _, lang := range Langs {
		for _, e := range events {
			s := e.Sentence(lang)
			if strings.Contains(s, missingMarker) {
				t.Errorf("%s: event %q has no message", lang, e.Kind)
			}
			// A sentence, not a log line: it ends in a full stop and does not
			// carry a bare code.
			if !strings.HasSuffix(strings.TrimSpace(s), ".") {
				t.Errorf("%s: event %q does not read as a sentence: %q", lang, e.Kind, s)
			}
			if strings.Contains(s, string(e.Kind)) {
				t.Errorf("%s: event %q renders its own code: %q", lang, e.Kind, s)
			}
		}
	}

	// On the page, in both languages, with no credential anywhere near it.
	for _, lang := range Langs {
		h.get("/?lang=" + string(lang))
		h.lang = lang
		_, body := h.get("/")
		section := body[strings.Index(body, `id="events-heading"`):]
		if end := strings.Index(section, "</section>"); end > 0 {
			section = section[:end]
		}
		if !strings.Contains(section, T(lang, MsgEventSwitchedOn)) {
			t.Errorf("%s: the events area does not show that Caspian was switched on", lang)
		}
		if secret, found := containsAny(section, testLinkSecrets()); found {
			t.Errorf("%s: the events area contains %q", lang, secret)
		}
		if strings.Contains(section, "sun-rope-glass-mint") {
			t.Errorf("%s: the events area contains the hotspot passphrase", lang)
		}
	}
}

// TestEventsRecordConnectionChangesOnce checks the observer.
//
// Several browsers poll at once, so a transition seen by ten pollers has to
// produce one event, not ten.
func TestEventsRecordConnectionChangesOnce(t *testing.T) {
	h := newHarness(t)
	h.ready()

	// The first observation must not record anything: a box that was never on
	// has not just dropped.
	for i := 0; i < 5; i++ {
		h.get("/status.json")
	}
	if n := countEvents(h, EventDisconnected); n != 0 {
		t.Errorf("%d disconnection events on a box that was never connected", n)
	}

	// Come up, and poll repeatedly.
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test"})
	h.priv.SetEngineState(engine.State{Phase: engine.PhaseRunning, Since: h.clock.Now()})
	for i := 0; i < 5; i++ {
		h.get("/status.json")
	}
	if n := countEvents(h, EventConnected); n != 1 {
		t.Errorf("%d connection events for one transition, want 1", n)
	}

	// Drop, and poll repeatedly.
	h.priv.SetEngineState(engine.State{Phase: engine.PhaseStopped, Since: h.clock.Now()})
	for i := 0; i < 5; i++ {
		h.get("/status.json")
	}
	if n := countEvents(h, EventDisconnected); n != 1 {
		t.Errorf("%d disconnection events for one transition, want 1", n)
	}

	// A privileged service that is not answering says nothing about the
	// tunnel, so it must not be recorded as a drop.
	h.priv.SetEngineState(engine.State{Phase: engine.PhaseRunning, Since: h.clock.Now()})
	h.get("/status.json")
	before := countEvents(h, EventDisconnected)
	h.priv.FailStatusWith(FaultUnavailable)
	for i := 0; i < 3; i++ {
		h.get("/status.json")
	}
	if after := countEvents(h, EventDisconnected); after != before {
		t.Errorf("a blind poll invented %d disconnection events", after-before)
	}
}

func countEvents(h *harness, kind EventKind) int {
	n := 0
	for _, e := range h.panel.events.entries() {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// TestEventsAreBoundedAndNotPersisted covers the two things the events area
// deliberately does not do.
func TestEventsAreBoundedAndNotPersisted(t *testing.T) {
	h := newHarness(t)
	h.ready()

	for i := 0; i < eventCapacity*2; i++ {
		h.panel.events.add(EventSignedIn, FaultNone)
	}
	if got := len(h.panel.events.entries()); got != eventCapacity {
		t.Errorf("the ring holds %d entries, want it bounded at %d", got, eventCapacity)
	}

	// Nothing about events reaches the state file. It holds credentials and is
	// written through a privileged path; a UI activity log does not belong on
	// that path, and the page says so rather than implying a full history.
	data, err := readStateFile(h)
	if err == nil && strings.Contains(string(data), "event") {
		t.Errorf("events reached the state file: %s", data)
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgEventsNote)) {
		t.Error("the page does not say the events list is cleared on restart")
	}
}

// TestTheDashboardSaysWhatHappensWhenTheTunnelDrops pins the one promise this
// appliance makes that a user cannot see it keeping.
//
// The firewall stops forwarded client traffic the moment the tunnel goes: the
// forward chain's accepts all name the tunnel device, so they stop matching and
// the drop policy takes everything. Until 2026-08-30 the only place that was
// written down was the advanced view, behind a link most people never open, so
// on the dashboard a dropped tunnel looked like the internet breaking rather
// than like the box doing its job.
//
// The check is that the sentence is on the ORDINARY page, in both languages. A
// promise a user has to go looking for is not one they can rely on.
func TestTheDashboardSaysWhatHappensWhenTheTunnelDrops(t *testing.T) {
	for _, lang := range Langs {
		h := newHarness(t)
		h.ready()
		if lang == LangEN {
			h.useEnglish()
		}

		_, body := h.get("/")

		if strings.Contains(body, `class="advanced"`) {
			t.Fatal("the harness opened the advanced view, so this test would pass on the wrong page")
		}
		for _, key := range []Key{MsgAdvDropLabel, MsgAdvDropValue} {
			want := h.msg(key)
			if !strings.Contains(body, want) {
				t.Errorf("%s: the dashboard does not say %q, so nothing tells the user their traffic stops rather than leaking",
					lang, want)
			}
		}
	}
}

// TestCuttingClientTrafficIsVisibleAndReversible pins the three things that
// make the emergency control safe to press.
//
// It exists because the control reintroduces, by design, the state this package
// spent a week removing: the engine is running and the hotspot is running and
// the user's devices reach nothing. Everything below is what stops that reading
// as a working connection.
func TestCuttingClientTrafficIsVisibleAndReversible(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test"})
	h.priv.SetEngineState(engine.State{Phase: engine.PhaseRunning})

	_, running := h.get("/")
	if !strings.Contains(running, `action="/cut"`) {
		t.Fatal("a running box offers no way to cut client traffic")
	}
	if strings.Contains(running, h.msg(MsgCutBanner)) {
		t.Error("the cut banner is on the page of a box that is not cut")
	}

	h.postForm("/cut", url.Values{"csrf": {h.tokenOn("/")}, "cut": {"1"}})

	_, cut := h.get("/")
	if !strings.Contains(cut, h.msg(MsgCutBanner)) {
		t.Error("client traffic is cut and the page does not say so")
	}
	if !strings.Contains(cut, h.msg(MsgCutRestoreButton)) {
		t.Error("the control does not offer the way back, so the only visible undo is the power switch")
	}
	// The whole point. The engine and the hotspot are both up, so anything
	// deriving "connected" from those two alone reports a working connection
	// to somebody whose devices reach nothing.
	if strings.Contains(cut, `id="status-word">`+h.msg(MsgStatusConnected)) {
		t.Error("the box reports itself connected while client traffic is cut")
	}

	h.postForm("/cut", url.Values{"csrf": {h.tokenOn("/")}, "cut": {"0"}})
	_, back := h.get("/")
	if strings.Contains(back, h.msg(MsgCutBanner)) {
		t.Error("the cut banner survived the restore")
	}
}

// TestCuttingABoxThatIsNotRunningSaysSomethingTrue is the stale-tab case: the
// page was open, the appliance was switched off elsewhere, and the button was
// pressed. Without its own word the refusal arrives as "Caspian could not work
// out what", which is untrue, because we know exactly what.
func TestCuttingABoxThatIsNotRunningSaysSomethingTrue(t *testing.T) {
	h := newHarness(t)
	h.ready()

	h.postForm("/cut", url.Values{"csrf": {h.tokenOn("/")}, "cut": {"1"}})

	_, body := h.get("/")
	want := h.msg(MsgFaultNotRunning)
	if !strings.Contains(body, want) {
		t.Errorf("a cut on a box that is not running does not say %q", want)
	}
	if strings.Contains(body, h.msg(MsgFaultUnknown)) {
		t.Error("the refusal reached the user as the unknown fault, which is not true here")
	}
}

// TestTheInstructionFollowsTheStateRatherThanTheRunningHotspot.
//
// The page carries one line saying what to do now, for the reader who has never
// seen it and will press something before reading anything. Its first version
// asked the LIVE hotspot for its name to decide whether one had been chosen,
// and a hotspot that is switched off has no live name, so a box that was set up
// and switched off went on telling its owner to choose a WiFi name they had
// already chosen. It reads what is saved.
func TestTheInstructionFollowsTheStateRatherThanTheRunningHotspot(t *testing.T) {
	h := newHarness(t)
	h.ready()

	// ready() has stored a config and a hotspot name and left the box off, so
	// the only thing left to do is press the switch.
	_, off := h.get("/")
	if !strings.Contains(off, h.msg(MsgNextSwitchon)) {
		t.Errorf("a configured box that is switched off does not say to switch it on")
	}
	if strings.Contains(off, h.msg(MsgNextSetwifi)) {
		t.Error("a box whose WiFi name is saved is still asking for a WiFi name, because the instruction is reading the running hotspot rather than the saved one")
	}

	h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test"})
	h.priv.SetEngineState(engine.State{Phase: engine.PhaseRunning})
	_, on := h.get("/")
	if !strings.Contains(on, h.msg(MsgNextJoin)) {
		t.Error("a working box with nothing joined does not say how to join a device")
	}

	h.postForm("/cut", url.Values{"csrf": {h.tokenOn("/")}, "cut": {"1"}})
	_, cut := h.get("/")
	if !strings.Contains(cut, h.msg(MsgNextResume)) {
		t.Error("a cut box does not say how to undo it")
	}
}

// TestBothControlsSayWhatTheyStopOnTheBarItself is the guard left behind by the
// question that prompted these sentences: the person who wrote this appliance
// could not tell the switch from the cut on his own dashboard, and asked why
// there are two.
//
// Both controls stop the internet for the joined devices. Only one of them
// takes the WiFi down as well, and that difference is the whole reason there
// are two. It was stated in internal/privsvc/cut.go, in the generated ruleset's
// own comments and on the help page, and nowhere on the bar that carries the
// two buttons, which is the one place a person reads before pressing one.
//
// Both languages, because a sentence that exists only in English is missing
// from the primary language of this panel.
func TestBothControlsSayWhatTheyStopOnTheBarItself(t *testing.T) {
	for _, lang := range Langs {
		h := newHarness(t)
		h.ready()
		if lang == LangEN {
			h.useEnglish()
		}
		// The cut control is offered only while the box is running, so its
		// caption can only be on the page of a running box.
		h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test"})
		h.priv.SetEngineState(engine.State{Phase: engine.PhaseRunning})

		_, body := h.get("/")

		for _, k := range []Key{MsgPowerCaption, MsgCutCaption} {
			want := h.msg(k)
			if !strings.Contains(body, want) {
				t.Errorf("%s: the dashboard does not say %q, so the two controls are told apart only on the help page",
					lang, want)
			}
		}

		// Each sentence belongs to its own control rather than to the bar, so
		// it has to follow the form it describes. Checked because the obvious
		// way to satisfy the loop above is one paragraph of both sentences
		// somewhere in the middle, which is the arrangement that failed.
		if strings.Index(body, h.msg(MsgPowerCaption)) < strings.Index(body, `action="/power"`) {
			t.Errorf("%s: the power sentence is not attached to the power control", lang)
		}
		if strings.Index(body, h.msg(MsgCutCaption)) < strings.Index(body, `action="/cut"`) {
			t.Errorf("%s: the cut sentence is not attached to the cut control", lang)
		}
	}
}

// TestTheHelpPageExplainsWhyThereAreTwoControls.
//
// The bar gets one sentence per control, which is all a bar has room for. The
// full answer is the help page's job: what each control tears down, and the
// lockout that follows from switching the appliance off while holding nothing
// but a phone that is on its hotspot.
//
// The last check is the load-bearing one. The page already had a one-line
// glossary entry for each control, and the reason the reader needs is the
// lockout, so a page that meets the glossary first has answered the smaller
// question and buried the larger one.
func TestTheHelpPageExplainsWhyThereAreTwoControls(t *testing.T) {
	for _, lang := range Langs {
		h := newHarness(t)
		h.ready()
		if lang == LangEN {
			h.useEnglish()
		}

		_, body := h.get("/help")

		for _, k := range []Key{
			MsgHelpTwoHeading, MsgHelpTwoIntro,
			MsgHelpTwoPowerHeading, MsgHelpTwoPowerBody,
			MsgHelpTwoCutHeading, MsgHelpTwoCutBody,
			MsgHelpTwoLockout, MsgHelpTwoWhich, MsgHelpTwoRestart,
		} {
			if !strings.Contains(body, h.msg(k)) {
				t.Errorf("%s: the help page does not carry %q", lang, k)
			}
		}

		lockout := strings.Index(body, h.msg(MsgHelpTwoLockout))
		glossary := strings.Index(body, h.msg(MsgHelpControlsCut))
		if lockout < 0 || glossary < 0 {
			t.Fatalf("%s: the help page is missing the lockout or the glossary, so the order below checks nothing", lang)
		}
		if lockout > glossary {
			t.Errorf("%s: the reason there are two controls sits below the one-line glossary", lang)
		}
	}
}
