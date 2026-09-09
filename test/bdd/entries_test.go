// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// ---------------------------------------------------------------------------
// A pasted text that holds several entries, and choosing one of them.
//
// A provider's subscription, a Clash profile and a plain paste of three links
// all reach the box as one text holding several entries. The engine still
// receives exactly one outbound; internal/link.Select decides which
// (internal/link/list.go, Select), the choice lives in
// state.ProxyConfig.Selected, and internal/panel's bringUp reads the chosen
// entry when the switch is pressed (internal/panel/handlers.go, bringUp).
//
// What these scenarios assert is the DOCUMENT the appliance hands the engine:
// which host it names. The page is consulted in exactly one place, where the
// page is itself the claim (a stale choice is explained on screen), and there
// the real panel is rendered over the same state store rather than a string
// being compared to a message key.
//
// These scenarios keep their own list, entryBehaviours, rather than joining
// behaviours() in behaviour_test.go. TestBehaviourDocumentListsEveryScenario
// requires every entry of behaviours() to be a section of docs/BEHAVIOUR.md,
// and that document was out of scope for the change that added this file. The
// same three guards that hold for behaviours() are applied to this list below,
// so the separation costs no evidence, only a place in the document, which is
// a decision for the maintainer.
//
// Every host below is under .invalid, which RFC 6761 reserves so that nothing
// can ever resolve it. Every credential is invented.
// ---------------------------------------------------------------------------

const (
	entryHostA = "a.example.invalid"
	entryHostB = "b.example.invalid"
	entryHostC = "c.example.invalid"

	entryTrojanPassword = "not-a-real-trojan-password"
	entrySSPassword     = "not-a-real-shadowsocks-password"

	// pagePassword is set on the panel only by the one step that renders the
	// page. It is not a credential of anything.
	pagePassword = "orbit-lemon-canvas-nine"
)

// entryLink returns one usable link of the given protocol at the given host,
// carrying the given #name. The three protocols are three different outbound
// shapes, so a document naming the wrong one is caught by its host AND by its
// protocol.
func entryLink(protocol, host, name string) string {
	switch protocol {
	case "vless":
		return "vless://" + fakeUUID + "@" + host + ":443?security=tls&type=raw&sni=" + fakeSNI + "#" + name
	case "trojan":
		return "trojan://" + entryTrojanPassword + "@" + host + ":443?security=tls&type=raw&sni=" + fakeSNI + "#" + name
	case "ss":
		return "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:"+entrySSPassword)) + "@" + host + ":8388#" + name
	}
	panic("entryLink: unknown protocol " + protocol)
}

// threeEntryPaste is what a provider's subscription looks like once pasted:
// three links, three hosts, three names.
func threeEntryPaste() string {
	return strings.Join([]string{
		entryLink("vless", entryHostA, "alpha"),
		entryLink("trojan", entryHostB, "beta"),
		entryLink("ss", entryHostC, "gamma"),
	}, "\n")
}

// pasteWhoseSecondEntryThisBoxRefuses has a middle line the vendored parser
// accepts (a truncated id is a syntactically fine URI) and internal/link
// refuses when it fills the outbound. ParseAll lists entries 0 and 2 and
// counts the middle one as dropped; Select(raw, 1) is an error.
func pasteWhoseSecondEntryThisBoxRefuses() string {
	return strings.Join([]string{
		entryLink("vless", entryHostA, "alpha"),
		"vless://1111111@" + entryHostB + ":443?security=tls#badid",
		entryLink("ss", entryHostC, "gamma"),
	}, "\n")
}

// ---------------------------------------------------------------------------
// What the appliance does with a choice
// ---------------------------------------------------------------------------

// selectedEntryFor is the choice in force for a text: the stored index when
// the text is the stored one, and the first entry when it is not, because a
// new paste is a new list and an index into the old one means nothing in it
// (internal/state/store.go, SetProxyConfig).
func (w *World) selectedEntryFor(pasted string) int {
	if w.defs.ignoreTheSelection {
		return 0
	}
	if w.defs.startOnTheLastEntry {
		if list, err := link.ParseAll(pasted); err == nil && len(list.Entries) > 0 {
			return list.Entries[len(list.Entries)-1].Index
		}
	}
	p := w.store.Proxy()
	if p.IsConfigured() && p.Raw.Reveal() == pasted {
		return p.Selected
	}
	return 0
}

// readPastedEntry is step 1 of connect: the chosen entry of the pasted text,
// read the way internal/panel's bringUp reads it. A choice past the end of the
// list falls back to the first entry and is reported, not refused, so a
// shorter re-paste or a hand-edited state file cannot strand the box. An entry
// this box refuses is an error, because starting on a neighbour would connect
// through a server the person did not choose.
func (w *World) readPastedEntry() (*link.Link, bool, error) {
	selected := w.selectedEntryFor(w.pasted)
	l, clamped, err := link.Select(w.pasted, selected)
	if err != nil && w.defs.slideToANeighbourWhenRefused {
		// The defect: the chosen entry is unusable, so take the first one
		// instead. It reads as helpful and it connects through the wrong
		// server.
		l, err = link.Parse(w.pasted)
		clamped = false
	}
	if err != nil {
		return nil, false, err
	}
	if clamped && w.defs.strandOnAStaleSelection {
		// The defect: treat a stale choice as a bad config. A box that
		// cannot start because of an index in a file is a box nobody can
		// switch on from the panel.
		return nil, false, fmt.Errorf("the chosen entry %d is not in the list", selected)
	}
	return l, clamped, nil
}

// paste is what POST /config does: read the text far enough to know it holds
// something usable, then store it. Storing resets the choice to the first
// entry. Nothing on the machine is touched.
func (w *World) paste(raw string) error {
	w.pasted = raw
	l, _, err := link.Select(raw, 0)
	if err != nil {
		return w.fail(err)
	}
	return w.fail(w.store.SetProxyConfig(raw, l.Protocol, l.Tag))
}

var errEntryNotInList = errors.New("that entry is not in the list")

// chooseEntry is what POST /select does: accept a position only if it is in
// the list right now, then record it. The refusal is recorded on the World for
// the Then steps; it is not a failure of the When step, because whether the
// refusal was right is what those steps decide.
func (w *World) chooseEntry(i int) {
	w.selectedBefore = w.store.Proxy().Selected
	w.selectErr = nil
	if !w.store.Proxy().IsConfigured() {
		w.selectErr = w.fail(errors.New("no config is stored, so there is nothing to choose from"))
		return
	}
	if !w.defs.acceptAnyIndex {
		if i < 0 {
			w.selectErr = w.fail(errEntryNotInList)
			return
		}
		list, err := link.ParseAll(w.store.Proxy().Raw.Reveal())
		if err != nil {
			w.selectErr = w.fail(err)
			return
		}
		listed := false
		for _, e := range list.Entries {
			if e.Index == i {
				listed = true
			}
		}
		if !listed {
			w.selectErr = w.fail(errEntryNotInList)
			w.note("entry %d refused: not in the list", i)
			return
		}
	}
	if err := w.store.SelectProxyEntry(i); err != nil {
		w.selectErr = w.fail(err)
		return
	}
	w.note("config entry selected: %d", i)
}

// reconnect is the path a choice takes when the tunnel is up: the same
// stop-then-start a re-paste takes (internal/panel/handlers.go,
// afterConfigChange). Engine.Start is idempotent on a running engine
// (internal/engine/engine.go, Start returns nil when the phase is running),
// which is exactly why the stop is not optional: without it the engine keeps
// the document it was started with and the page says one thing while the box
// does another.
func (w *World) reconnect() error {
	if !w.defs.restartWithoutStopping {
		if err := w.disconnect(); err != nil {
			return err
		}
	}
	return w.connect()
}

// ---------------------------------------------------------------------------
// Given
// ---------------------------------------------------------------------------

// aConfigOfThreeEntriesHasBeenPasted stores a three-link text the way the
// paste form does. The box is off; nothing on the machine has been touched.
func aConfigOfThreeEntriesHasBeenPasted(w *World) error {
	if err := w.paste(threeEntryPaste()); err != nil {
		return fmt.Errorf("the three-entry text was not accepted: %w", err)
	}
	if got := w.store.Proxy().Selected; got != 0 {
		return fmt.Errorf("a fresh paste stored a choice of %d, want 0", got)
	}
	return nil
}

// aBoxConnectedOnAConfigOfThreeEntries has pasted the three-link text and
// pressed the switch once, on the first entry. The marks a restart is
// compared against are taken here.
func aBoxConnectedOnAConfigOfThreeEntries(w *World) error {
	if err := aConfigOfThreeEntriesHasBeenPasted(w); err != nil {
		return err
	}
	if err := w.connect(); err != nil {
		return fmt.Errorf("the first connect did not succeed, so nothing after it means anything: %w", err)
	}
	w.engineRunningSince = w.eng.State().Since
	return nil
}

// aStoredChoicePastTheEndOfTheList writes an index the list does not have
// straight into the store, which is what a hand-edited state file or a
// shorter re-paste under an older version leaves behind. It goes through the
// store and not through chooseEntry, because the point is a choice the list
// check never saw.
func aStoredChoicePastTheEndOfTheList(w *World) error {
	if err := w.store.SelectProxyEntry(7); err != nil {
		return fmt.Errorf("the store refused an index past the end, so this scenario cannot be set up: %w", err)
	}
	return nil
}

// aConfigWhoseSecondEntryThisBoxRefusesHasBeenPasted stores a text whose
// middle line parses as a URI and is refused as an outbound, and points the
// stored choice at it. The choice is written through the store for the same
// reason as above: POST /select would refuse the slot, because ParseAll does
// not list it, so the only way this state arises is a list that changed
// underneath a stored choice.
func aConfigWhoseSecondEntryThisBoxRefusesHasBeenPasted(w *World) error {
	if err := w.paste(pasteWhoseSecondEntryThisBoxRefuses()); err != nil {
		return fmt.Errorf("the text was not accepted at all: %w", err)
	}
	list, err := link.ParseAll(w.pasted)
	if err != nil {
		return err
	}
	if len(list.Entries) != 2 || list.Dropped != 1 {
		return fmt.Errorf("fixture: %d entries listed and %d dropped, want 2 and 1", len(list.Entries), list.Dropped)
	}
	return w.store.SelectProxyEntry(1)
}

// ---------------------------------------------------------------------------
// When
// ---------------------------------------------------------------------------

// theUserChoosesEntryTwo posts position 1, which is in the list.
func theUserChoosesEntryTwo(w *World) error {
	w.chooseEntry(1)
	return nil
}

// theUserChoosesEntrySeven posts a position the list does not have.
func theUserChoosesEntrySeven(w *World) error {
	w.chooseEntry(7)
	return nil
}

// theUserChoosesEntryTwoWhileConnected is the choice made with the tunnel up,
// which restarts the box on the new entry.
func theUserChoosesEntryTwoWhileConnected(w *World) error {
	if w.eng == nil || w.eng.State().Phase != engine.PhaseRunning {
		return errors.New("the box is not connected, so this is not a choice made while connected")
	}
	w.chooseEntry(1)
	if w.selectErr != nil {
		return fmt.Errorf("entry two of three was refused: %w", w.selectErr)
	}
	_ = w.reconnect()
	return nil
}

// ---------------------------------------------------------------------------
// Then
// ---------------------------------------------------------------------------

// theEngineDocumentNamesTheFirstHostOnly reads the document the appliance
// handed the engine, not the page.
func theEngineDocumentNamesTheFirstHostOnly(w *World) error {
	return documentNamesOnly(w, entryHostA, "vless")
}

func theEngineDocumentNamesTheSecondHostOnly(w *World) error {
	return documentNamesOnly(w, entryHostB, "trojan")
}

// documentNamesOnly requires the engine document to name one host of the
// three and neither of the others, and requires the outbound the document
// routes through (the one tagged xcfg.TagProxy) to be of that entry's
// protocol, so a document with the right host and the wrong outbound is
// caught too. The document is parsed as JSON rather than searched for a
// spelling: internal/xcfg indents it (internal/xcfg/build.go, Build), and a
// needle written for one layout is a check that stops checking on the next.
func documentNamesOnly(w *World, host, protocol string) error {
	if len(w.engineCfg) == 0 {
		return errors.New("no engine document was built, so there is nothing to read the host out of")
	}
	doc := string(w.engineCfg)
	if !strings.Contains(doc, host) {
		return fmt.Errorf("the engine document does not name %s", host)
	}
	for _, other := range []string{entryHostA, entryHostB, entryHostC} {
		if other != host && strings.Contains(doc, other) {
			return fmt.Errorf("the engine document names %s, which was not chosen", other)
		}
	}
	var parsed struct {
		Outbounds []struct {
			Tag      string `json:"tag"`
			Protocol string `json:"protocol"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(w.engineCfg, &parsed); err != nil {
		return fmt.Errorf("the engine document is not readable JSON: %w", err)
	}
	found := false
	for _, o := range parsed.Outbounds {
		if o.Tag != xcfg.TagProxy {
			continue
		}
		found = true
		if o.Protocol != protocol {
			return fmt.Errorf("the outbound the document routes through is %q, not the %s entry that was chosen", o.Protocol, protocol)
		}
	}
	if !found {
		return fmt.Errorf("the engine document has no outbound tagged %q, so nothing in it is the chosen entry", xcfg.TagProxy)
	}
	if w.lnk == nil || w.lnk.Address != host {
		return fmt.Errorf("the appliance read entry %d at %q, which is not %s", w.lnk.Index, w.lnk.Address, host)
	}
	return nil
}

// theEngineWasRestartedOnTheNewEntry: a running engine keeps the document it
// was started with, so a choice made while connected has to stop it first.
// Engine.State().Since moves only when the engine actually started again.
func theEngineWasRestartedOnTheNewEntry(w *World) error {
	if w.eng == nil {
		return errors.New("no engine")
	}
	st := w.eng.State()
	if st.Phase != engine.PhaseRunning {
		return fmt.Errorf("the engine is %v after the choice, not running (reason %q)", st.Phase, st.Reason)
	}
	if w.engineRunningSince.IsZero() {
		return errors.New("no mark was taken before the choice, so a restart cannot be told from no restart")
	}
	if !st.Since.After(w.engineRunningSince) {
		return fmt.Errorf(
			"the engine has been running since %s, which is when it started on the first entry, so it "+
				"was never stopped and is still carrying the old document whatever the page says",
			st.Since.Format(time.RFC3339Nano))
	}
	if w.tl.indexOf("engine: stopped") < 0 {
		return fmt.Errorf("the timeline records no engine stop before the restart\n  %s", w.tl)
	}
	return nil
}

// theChoiceIsRefused: the attempt produced a refusal and the store still
// holds the choice that was in force before it.
func theChoiceIsRefused(w *World) error {
	if w.selectErr == nil {
		return errors.New("the choice was accepted")
	}
	if got := w.store.Proxy().Selected; got != w.selectedBefore {
		return fmt.Errorf("the choice was refused and the store moved from %d to %d anyway", w.selectedBefore, got)
	}
	return nil
}

// theChoiceIsAccepted is the positive control for the step above.
func theChoiceIsAccepted(w *World) error {
	if w.selectErr != nil {
		return fmt.Errorf("the choice was refused: %w", w.selectErr)
	}
	return nil
}

// theBoxStartedOnTheFirstEntryBecauseTheChoiceWasStale: the appliance fell
// back rather than refusing, and said so.
func theBoxStartedOnTheFirstEntryBecauseTheChoiceWasStale(w *World) error {
	if !w.entryClamped {
		return errors.New("the appliance did not report that the stored choice was past the end of the list")
	}
	if w.store.Proxy().Selected == 0 {
		return errors.New("the stored choice is 0, so nothing here was stale and this scenario proves nothing")
	}
	return theEngineDocumentNamesTheFirstHostOnly(w)
}

// entryRadioRE is the radio the page draws per entry, from
// internal/panel/templates/index.html. It is the same expression
// internal/panel/select_test.go reads the page with.
var entryRadioRE = regexp.MustCompile(`<input type="radio" name="entry" value="(\d+)"( checked)?>`)

// thePageSaysTheChosenEntryIsGoneAndShowsTheFirstAsChosen renders the real
// panel over this World's state store and reads the dashboard. This is the
// one step in the file that looks at the page, because the page IS the
// claim here: the person has to be told why the box is not on the entry they
// remember choosing.
func thePageSaysTheChosenEntryIsGoneAndShowsTheFirstAsChosen(w *World) error {
	body, err := w.dashboardHTML()
	if err != nil {
		return err
	}
	want := panel.T(panel.LangEN, panel.MsgConfigEntryReset)
	if !strings.Contains(body, want) {
		return fmt.Errorf("the page does not say %q", want)
	}
	listed, checked := entriesOn(body)
	if len(listed) != 3 {
		return fmt.Errorf("the page lists %v, want three entries", listed)
	}
	if checked != 0 {
		return fmt.Errorf("entry %d is drawn as chosen, want 0 (the first)", checked)
	}
	return nil
}

func entriesOn(body string) (listed []int, checked int) {
	checked = -1
	for _, m := range entryRadioRE.FindAllStringSubmatch(body, -1) {
		i, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		listed = append(listed, i)
		if m[2] != "" {
			checked = i
		}
	}
	return listed, checked
}

// theConnectIsRefusedRatherThanUsingANeighbour: the chosen entry is one this
// box refuses, and the answer is a refusal worded as a config it could not
// read, with no engine document built and the machine untouched. Sliding to
// entry one would have connected through a server the person did not choose.
func theConnectIsRefusedRatherThanUsingANeighbour(w *World) error {
	if w.connectErr == nil {
		return errors.New("the box connected on an entry it should have refused, so it is on a neighbour")
	}
	if len(w.engineCfg) != 0 {
		return errors.New("an engine document was built for a refused entry")
	}
	if w.lnk != nil {
		return fmt.Errorf("the appliance read entry %d at %q instead of refusing", w.lnk.Index, w.lnk.Address)
	}
	return theUserIsToldTheTextCouldNotBeRead(w)
}

// ---------------------------------------------------------------------------
// Rendering the page over this World's store
// ---------------------------------------------------------------------------

// dashboardHTML serves the real panel (templates, catalogue and handlers)
// over the World's state store with the fake privileged side, signs in
// through the real form, and returns the dashboard.
//
// The fake privileged side is used here because the page reads status from
// it; the claim under test is what the page says about the STORED choice,
// which comes from the store the appliance wrote, not from the fake.
func (w *World) dashboardHTML() (string, error) {
	if err := w.store.SetPanelPassword(pagePassword); err != nil {
		return "", fmt.Errorf("setting the panel password: %w", err)
	}
	p, err := panel.New(panel.Config{
		Store:  w.store,
		Priv:   panel.NewFakePrivileged(),
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		return "", fmt.Errorf("panel.New: %w", err)
	}
	srv := httptest.NewServer(p)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	get := func(path string) (*http.Response, string, error) {
		res, err := client.Get(srv.URL + path)
		if err != nil {
			return nil, "", err
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		return res, string(b), err
	}

	_, login, err := get("/login")
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`name="csrf" value="([^"]*)"`).FindStringSubmatch(login)
	if m == nil || m[1] == "" {
		return "", errors.New("no form token on the login page")
	}
	res, err := client.PostForm(srv.URL+"/login", url.Values{"csrf": {m[1]}, "password": {pagePassword}})
	if err != nil {
		return "", err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
		return "", fmt.Errorf("signing in answered %d to %q, so the dashboard cannot be read", res.StatusCode, res.Header.Get("Location"))
	}
	res, body, err := get("/")
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the dashboard answered %d", res.StatusCode)
	}
	return body, nil
}

// ---------------------------------------------------------------------------
// The scenarios
// ---------------------------------------------------------------------------

// entryBehaviours is the list. See the file comment for why it is separate
// from behaviours().
func entryBehaviours() []*scenario {
	return []*scenario{

		// ---------------------------------------------------------------
		// Happy paths
		// ---------------------------------------------------------------
		Scenario("a paste of three links starts the box on the first of them").
			Given(aFreshBox).
			And(aConfigOfThreeEntriesHasBeenPasted).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theEngineIsRunning).
			And(theEngineDocumentNamesTheFirstHostOnly).
			BreaksWhen("the box starts on the last entry of the list instead of the first",
				func(d *defects) { d.startOnTheLastEntry = true }),

		Scenario("choosing entry two makes the engine document name the second host only").
			Given(aFreshBox).
			And(aConfigOfThreeEntriesHasBeenPasted).
			When(theUserChoosesEntryTwo).
			And(theUserPressesConnect).
			Then(theChoiceIsAccepted).
			And(theBoxConnects).
			And(theEngineDocumentNamesTheSecondHostOnly).
			BreaksWhen("the box starts on entry one whatever the person chose",
				func(d *defects) { d.ignoreTheSelection = true }),

		Scenario("choosing an entry while connected restarts the box on the new host").
			Given(aBoxConnectedOnAConfigOfThreeEntries).
			When(theUserChoosesEntryTwoWhileConnected).
			Then(theBoxConnects).
			And(theEngineWasRestartedOnTheNewEntry).
			And(theEngineDocumentNamesTheSecondHostOnly).
			BreaksWhen("the new entry is started over the running engine without stopping it, so the "+
				"engine keeps the old document",
				func(d *defects) { d.restartWithoutStopping = true }),

		// ---------------------------------------------------------------
		// Unhappy paths
		// ---------------------------------------------------------------
		Scenario("an entry the list does not have is refused and the box stays on its entry").
			Given(aFreshBox).
			And(aConfigOfThreeEntriesHasBeenPasted).
			And(theUserChoosesEntryTwo).
			When(theUserChoosesEntrySeven).
			And(theUserPressesConnect).
			Then(theChoiceIsRefused).
			And(theBoxConnects).
			And(theEngineDocumentNamesTheSecondHostOnly).
			BreaksWhen("a choice is recorded without checking it is in the list",
				func(d *defects) { d.acceptAnyIndex = true }),

		Scenario("a stored choice past the end of the list comes up on entry one and the page says so").
			Given(aFreshBox).
			And(aConfigOfThreeEntriesHasBeenPasted).
			And(aStoredChoicePastTheEndOfTheList).
			When(theUserPressesConnect).
			Then(theBoxConnects).
			And(theBoxStartedOnTheFirstEntryBecauseTheChoiceWasStale).
			And(thePageSaysTheChosenEntryIsGoneAndShowsTheFirstAsChosen).
			BreaksWhen("a stored choice past the end of the list stops the box from starting at all",
				func(d *defects) { d.strandOnAStaleSelection = true }),

		Scenario("a chosen entry this box refuses stops the connect rather than using a neighbour").
			Given(aFreshBox).
			And(aConfigWhoseSecondEntryThisBoxRefusesHasBeenPasted).
			When(theUserPressesConnect).
			Then(theConnectIsRefusedRatherThanUsingANeighbour).
			And(theMachineWasNotTouched).
			BreaksWhen("an entry this box refuses is replaced by entry one, which connects through a server "+
				"the person did not choose",
				func(d *defects) { d.slideToANeighbourWhenRefused = true }),
	}
}

// TestEntryBehaviour runs every entry scenario with no defect injected.
func TestEntryBehaviour(t *testing.T) {
	for _, s := range entryBehaviours() {
		s.run(t, defects{})
	}
}

// TestEveryEntryScenarioCanFail injects each scenario's named defect and
// requires red, for the same reason TestEveryScenarioCanFail does: a scenario
// nobody has watched fail is not evidence.
func TestEveryEntryScenarioCanFail(t *testing.T) {
	var table []string
	table = append(table, "scenario | defect injected | result")

	for _, s := range entryBehaviours() {
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
	t.Log("\nMUTATION TABLE (entries)\n" + strings.Join(table, "\n"))
}

// TestEveryEntryScenarioNamesADefectAndReadsAsASentence applies the two
// list-hygiene guards behaviours() gets, to this list.
func TestEveryEntryScenarioNamesADefectAndReadsAsASentence(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range entryBehaviours() {
		if s.defect == nil || s.defectName == "" {
			t.Errorf("scenario %q names no defect: add BreaksWhen, and watch it go red", s.name)
		}
		if len(s.steps) == 0 {
			t.Errorf("scenario %q has no steps", s.name)
		}
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
