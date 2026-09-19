// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

// The tests in this file were written BEFORE POST /select, ConfigEntries and
// the entry message keys existed, on 2026-09-08. Their first run was a compile
// failure naming the keys; that is the "red" recorded in the report.
//
// Every host and name below is invented. The three hosts are under .invalid,
// which RFC 6761 reserves so that nothing can ever resolve them.

const (
	entryHostA = "a.example.invalid"
	entryHostB = "b.example.invalid"
	entryHostC = "c.example.invalid"
)

// entryLink returns one usable link of the given protocol at the given host,
// carrying the given #name.
func entryLink(protocol, host, name string) string {
	switch protocol {
	case "vless":
		return "vless://" + fakeUUIDForPanel + "@" + host + ":443?security=tls&type=raw&sni=" + fakeSNIForPanel + "#" + name
	case "trojan":
		return "trojan://not-a-real-password@" + host + ":443?security=tls&type=raw&sni=" + fakeSNIForPanel + "#" + name
	case "ss":
		return "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:not-a-real-password")) + "@" + host + ":8388#" + name
	}
	panic("entryLink: unknown protocol " + protocol)
}

// threeEntryConfig is what a provider's subscription looks like once pasted:
// three links, three hosts, three names.
func threeEntryConfig() string {
	return strings.Join([]string{
		entryLink("vless", entryHostA, "alpha"),
		entryLink("trojan", entryHostB, "beta"),
		entryLink("ss", entryHostC, "gamma"),
	}, "\n")
}

// brokenLine is a link the vendored parser drops without a word: the port does
// not fit in sixteen bits (third_party/libxray-share/parse_share.go:107-109).
func brokenLine(host string) string {
	return "vless://" + fakeUUIDForPanel + "@" + host + ":99999?security=tls#broken"
}

// readyWith is harness.ready with a config of the caller's choosing.
func (h *harness) readyWith(raw string) {
	h.t.Helper()
	h.setup(testPassword)
	if err := h.store.SetHotspot("Caspian-test", "sun-rope-glass-mint"); err != nil {
		h.t.Fatalf("SetHotspot: %v", err)
	}
	if err := h.store.SetProxyConfig(raw, "vless", "Home"); err != nil {
		h.t.Fatalf("SetProxyConfig: %v", err)
	}
}

// entryRadioRE matches one radio on the entry list. The value is the entry's
// index and the trailing group says whether it is the checked one.
var entryRadioRE = regexp.MustCompile(`<input type="radio" name="entry" value="(\d+)"( checked)?>`)

// entriesOn returns the entry indexes listed on the page and which one is
// checked, or -1 if none is.
func entriesOn(t *testing.T, body string) (listed []int, checked int) {
	t.Helper()
	checked = -1
	for _, m := range entryRadioRE.FindAllStringSubmatch(body, -1) {
		i, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("radio value %q is not a number", m[1])
		}
		listed = append(listed, i)
		if m[2] != "" {
			if checked != -1 {
				t.Errorf("two entries are checked: %d and %d", checked, i)
			}
			checked = i
		}
	}
	return listed, checked
}

// selectEntry posts the selection the way the page does.
func (h *harness) selectEntry(value string) (*http.Response, string) {
	h.t.Helper()
	return h.postForm("/select", url.Values{"csrf": {h.tokenOn("/")}, "entry": {value}})
}

// switchOn presses the switch and fails the test if that did not redirect.
func (h *harness) switchOn() {
	h.t.Helper()
	res, body := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
	if res.StatusCode != http.StatusSeeOther {
		h.t.Fatalf("POST /power: status %d, body %s", res.StatusCode, body)
	}
}

// --- acceptance items 1 and 2 ------------------------------------------------

// TestThePageListsEveryEntryAndTheFirstIsChosen is acceptance item 1: three
// entries listed, entry one selected, and the document the privileged side
// receives names the first host.
func TestThePageListsEveryEntryAndTheFirstIsChosen(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())

	_, body := h.get("/")
	listed, checked := entriesOn(t, body)
	if len(listed) != 3 || listed[0] != 0 || listed[1] != 1 || listed[2] != 2 {
		t.Fatalf("entries listed = %v, want [0 1 2]", listed)
	}
	if checked != 0 {
		t.Errorf("entry %d is checked, want 0", checked)
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(body, `<bdi dir="ltr" class="mono">`+name+`</bdi>`) {
			t.Errorf("the name %q is not rendered inside an isolated element", name)
		}
	}
	// The dropped-line sentence is absent when nothing was dropped.
	// The English "many" sentence starts with its number, so its stem before
	// the verb is empty and proves nothing; the guard below skips that case
	// rather than passing on an empty needle.
	for _, k := range []Key{MsgConfigDroppedOne, MsgConfigDroppedMany} {
		stem := strings.SplitN(h.msg(k), "%", 2)[0]
		if stem != "" && strings.Contains(body, stem) {
			t.Errorf("the page reports dropped lines when none were: %s", h.msg(k))
		}
	}

	h.switchOn()
	starts := h.priv.Starts()
	if len(starts) != 1 {
		t.Fatalf("%d start requests, want 1", len(starts))
	}
	doc := string(starts[0].ConfigJSON)
	if !strings.Contains(doc, entryHostA) || strings.Contains(doc, entryHostB) || strings.Contains(doc, entryHostC) {
		t.Errorf("the config document does not name the first host alone")
	}
}

// TestSelectingAnEntryChangesTheConfigSentToThePrivilegedSide is acceptance item
// 2 with the box off: choose entry two, switch on, and the document names the
// second host.
func TestSelectingAnEntryChangesTheConfigSentToThePrivilegedSide(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())

	res, body := h.selectEntry("1")
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
		t.Fatalf("POST /select: status %d location %q, body %s", res.StatusCode, res.Header.Get("Location"), body)
	}
	if got := h.store.Proxy().Selected; got != 1 {
		t.Fatalf("Selected = %d after choosing entry 1", got)
	}
	_, body = h.get("/")
	if _, checked := entriesOn(t, body); checked != 1 {
		t.Errorf("entry %d is checked after choosing 1", checked)
	}
	if !strings.Contains(body, h.msg(MsgNoticeEntrySelected)) {
		t.Errorf("the page does not confirm the choice with %q", h.msg(MsgNoticeEntrySelected))
	}
	// The box was off, so nothing was asked to stop or start yet.
	if h.priv.Stops() != 0 || len(h.priv.Starts()) != 0 {
		t.Errorf("choosing an entry on a box that is off touched the privileged side: stops %d starts %d", h.priv.Stops(), len(h.priv.Starts()))
	}

	h.switchOn()
	starts := h.priv.Starts()
	if len(starts) != 1 {
		t.Fatalf("%d start requests, want 1", len(starts))
	}
	doc := string(starts[0].ConfigJSON)
	if !strings.Contains(doc, entryHostB) {
		t.Errorf("the config document does not name the chosen host %s", entryHostB)
	}
	if strings.Contains(doc, entryHostA) || strings.Contains(doc, entryHostC) {
		t.Errorf("the config document names a host that was not chosen")
	}
	if !strings.Contains(doc, `"protocol":"trojan"`) {
		t.Errorf("the config document is not the trojan entry: %.120s", doc)
	}
}

// TestSelectingWhileRunningRestartsWithTheNewEntry is acceptance item 2 with
// the box on: the same stop-then-start path a re-paste takes, and the new
// document names the new host.
func TestSelectingWhileRunningRestartsWithTheNewEntry(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())
	h.switchOn()
	if len(h.priv.Starts()) != 1 {
		t.Fatalf("setup: %d starts", len(h.priv.Starts()))
	}

	if res, _ := h.selectEntry("2"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /select: status %d", res.StatusCode)
	}
	if h.priv.Stops() != 1 {
		t.Errorf("stops = %d, want 1: the running engine has to be stopped before the new entry starts", h.priv.Stops())
	}
	starts := h.priv.Starts()
	if len(starts) != 2 {
		t.Fatalf("starts = %d, want 2", len(starts))
	}
	doc := string(starts[1].ConfigJSON)
	if !strings.Contains(doc, entryHostC) || strings.Contains(doc, entryHostA) {
		t.Errorf("the restart did not use the chosen entry")
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgNoticeEntryReconn)) {
		t.Errorf("the page does not say the box reconnected: want %q", h.msg(MsgNoticeEntryReconn))
	}
	if _, checked := entriesOn(t, body); checked != 2 {
		t.Errorf("entry %d is checked after choosing 2", checked)
	}
}

// TestTheSelectionSurvivesAReload: the choice is in the state file, so a box
// that loses power comes back on the entry it was on.
func TestTheSelectionSurvivesAReload(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())
	if res, _ := h.selectEntry("1"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /select: status %d", res.StatusCode)
	}
	reloaded, err := state.Load(h.store.Dir())
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if got := reloaded.Proxy().Selected; got != 1 {
		t.Errorf("Selected = %d after a reload from disk, want 1", got)
	}
}

// --- acceptance item 3 -------------------------------------------------------

// TestRePastingResetsTheSelection: a new list, entry one again.
func TestRePastingResetsTheSelection(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())
	if res, _ := h.selectEntry("2"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /select: status %d", res.StatusCode)
	}

	two := entryLink("vless", entryHostB, "beta") + "\n" + entryLink("ss", entryHostC, "gamma")
	res, body := h.postForm("/config", url.Values{"csrf": {h.tokenOn("/")}, "config": {two}})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /config: status %d, body %s", res.StatusCode, body)
	}
	if got := h.store.Proxy().Selected; got != 0 {
		t.Errorf("Selected = %d after a re-paste, want 0", got)
	}
	_, body = h.get("/")
	listed, checked := entriesOn(t, body)
	if len(listed) != 2 || checked != 0 {
		t.Errorf("after a re-paste the page lists %v with %d checked, want [0 1] with 0", listed, checked)
	}
}

// --- acceptance item 6, the stale selection -----------------------------------

// TestAStaleSelectionFallsBackToTheFirstEntryWithANotice: a selection the list
// no longer has (here written straight into the store, which is what a
// hand-edited file or a version skew looks like) must not strand the box. The
// page shows entry one chosen, says why, and switching on uses entry one.
func TestAStaleSelectionFallsBackToTheFirstEntryWithANotice(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())
	if err := h.store.SelectProxyEntry(7); err != nil {
		t.Fatalf("SelectProxyEntry(7): %v", err)
	}

	_, body := h.get("/")
	if _, checked := entriesOn(t, body); checked != 0 {
		t.Errorf("entry %d is checked with a stale selection, want 0", checked)
	}
	if !strings.Contains(body, h.msg(MsgConfigEntryReset)) {
		t.Errorf("the page does not explain the fallback with %q", h.msg(MsgConfigEntryReset))
	}
	// The box must still start, on entry one.
	h.switchOn()
	starts := h.priv.Starts()
	if len(starts) != 1 || !strings.Contains(string(starts[0].ConfigJSON), entryHostA) {
		t.Fatalf("a stale selection stopped the box from starting on the first entry: %d starts", len(starts))
	}
	// A good selection clears the notice.
	if res, _ := h.selectEntry("1"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /select: status %d", res.StatusCode)
	}
	_, body = h.get("/")
	if strings.Contains(body, h.msg(MsgConfigEntryReset)) {
		t.Error("the fallback notice is still shown after a valid selection")
	}
}

// --- the unhappy paths of the handler ----------------------------------------

// TestSelectRefusesValuesThatAreNotAnEntry covers every shape of a bad "entry"
// field: not a number, empty, negative, past the end, a fraction. Each one is
// answered with a sentence, changes nothing, and touches the privileged side
// not at all.
func TestSelectRefusesValuesThatAreNotAnEntry(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())
	h.switchOn() // so a wrongly accepted value would be visible as a restart

	for _, bad := range []string{"abc", "", "-1", "3", "9", "1.5", " 1 ", "0x1", "99999999999999999999"} {
		t.Run(strconv.Quote(bad), func(t *testing.T) {
			res, body := h.selectEntry(bad)
			if res.StatusCode != http.StatusSeeOther {
				t.Fatalf("status %d, want a redirect back to the page; body %s", res.StatusCode, body)
			}
			_, body = h.get("/")
			if !strings.Contains(body, h.msg(MsgSelectBadEntry)) {
				t.Errorf("the page does not say the entry is not in the list (want %q)", h.msg(MsgSelectBadEntry))
			}
			if got := h.store.Proxy().Selected; got != 0 {
				t.Errorf("Selected = %d after a refused value", got)
			}
		})
	}
	if h.priv.Stops() != 0 || len(h.priv.Starts()) != 1 {
		t.Errorf("a refused value reached the privileged side: stops %d starts %d", h.priv.Stops(), len(h.priv.Starts()))
	}
}

// TestSelectOnASingleEntryConfigRefusesAnyOtherIndex: a config with one entry
// draws no list, and an index into a list that is not there is refused.
func TestSelectOnASingleEntryConfigRefusesAnyOtherIndex(t *testing.T) {
	h := newHarness(t)
	h.ready()
	_, body := h.get("/")
	if listed, _ := entriesOn(t, body); len(listed) != 0 {
		t.Errorf("a single-entry config draws a list of %v", listed)
	}
	if res, _ := h.selectEntry("1"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	_, body = h.get("/")
	if !strings.Contains(body, h.msg(MsgSelectBadEntry)) {
		t.Error("choosing entry two of a one-entry config was not refused")
	}
	// Entry one of one is fine, and is not a change.
	if res, _ := h.selectEntry("0"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	if got := h.store.Proxy().Selected; got != 0 {
		t.Errorf("Selected = %d", got)
	}
}

// TestSelectWithNoConfigSaysSo: there is nothing to choose from.
func TestSelectWithNoConfigSaysSo(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	res, _ := h.selectEntry("0")
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgNoConfigYet)) {
		t.Errorf("the page does not say there is no config (want %q)", h.msg(MsgNoConfigYet))
	}
	if h.store.Proxy().IsConfigured() {
		t.Error("a selection with no config stored one")
	}
}

// TestSelectIsGatedLikeEveryOtherForm: no session is a redirect to login, a
// session with no token or a wrong token is the CSRF refusal, and a cross-site
// post is refused before any handler runs. TestEveryRouteRefusesWithoutASession
// covers the first from the route table; the other two are the ones a table
// cannot express.
func TestSelectIsGatedLikeEveryOtherForm(t *testing.T) {
	h := newHarness(t)
	h.readyWith(threeEntryConfig())
	if err := h.store.SelectProxyEntry(0); err != nil {
		t.Fatal(err)
	}

	// Signed in, no token.
	res, _ := h.postForm("/select", url.Values{"entry": {"1"}})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("no token: status %d, want 403", res.StatusCode)
	}
	// Signed in, wrong token.
	res, _ = h.postForm("/select", url.Values{"csrf": {"not-the-token"}, "entry": {"1"}})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("wrong token: status %d, want 403", res.StatusCode)
	}
	// Signed in, right token, cross-site.
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/select",
		strings.NewReader(url.Values{"csrf": {h.tokenOn("/")}, "entry": {"1"}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://attacker.invalid")
	if res, _ := h.do(req); res.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site: status %d, want 403", res.StatusCode)
	}
	if got := h.store.Proxy().Selected; got != 0 {
		t.Errorf("a refused request changed Selected to %d", got)
	}

	// Signed out.
	h.signedOut()
	res, _ = h.postForm("/select", url.Values{"entry": {"1"}})
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login" {
		t.Errorf("signed out: status %d location %q, want 303 to /login", res.StatusCode, res.Header.Get("Location"))
	}
	if got := h.store.Proxy().Selected; got != 0 {
		t.Errorf("an unauthenticated request changed Selected to %d", got)
	}
}

// --- acceptance item 4, the dropped line -------------------------------------

// TestDroppedLinesAreReportedOnThePage: the sentence appears for one and for
// several, and not at all for none. It appears even when only one entry is
// left, because "I pasted two and see one" is exactly the moment it is needed.
func TestDroppedLinesAreReportedOnThePage(t *testing.T) {
	one := h2c(t, "one dropped", strings.Join([]string{
		entryLink("vless", entryHostA, "alpha"),
		brokenLine(entryHostB),
		entryLink("ss", entryHostC, "gamma"),
	}, "\n"))
	if !strings.Contains(one.body, one.h.msg(MsgConfigDroppedOne)) {
		t.Errorf("one dropped: the page lacks %q", one.h.msg(MsgConfigDroppedOne))
	}
	if listed, _ := entriesOn(t, one.body); len(listed) != 2 {
		t.Errorf("one dropped: %d entries listed, want 2", len(listed))
	}

	two := h2c(t, "two dropped", strings.Join([]string{
		entryLink("vless", entryHostA, "alpha"),
		brokenLine(entryHostB),
		brokenLine(entryHostC),
		entryLink("ss", entryHostC, "gamma"),
	}, "\n"))
	if want := T(two.h.lang, MsgConfigDroppedMany, 2); !strings.Contains(two.body, want) {
		t.Errorf("two dropped: the page lacks %q", want)
	}

	// One survivor: no list, but the sentence.
	lone := h2c(t, "one survivor", entryLink("vless", entryHostA, "alpha")+"\n"+brokenLine(entryHostB))
	if listed, _ := entriesOn(t, lone.body); len(listed) != 0 {
		t.Errorf("one survivor: a list of %v was drawn for a single usable entry", listed)
	}
	if !strings.Contains(lone.body, lone.h.msg(MsgConfigDroppedOne)) {
		t.Error("one survivor: the dropped line is not reported")
	}

	// None dropped: neither sentence.
	none := h2c(t, "none dropped", threeEntryConfig())
	stem := strings.SplitN(none.h.msg(MsgConfigDroppedMany), "%", 2)[0]
	if strings.Contains(none.body, none.h.msg(MsgConfigDroppedOne)) || (stem != "" && strings.Contains(none.body, stem)) {
		t.Error("none dropped: the page reports a dropped line")
	}
}

type pageOf struct {
	h    *harness
	body string
}

// h2c ("harness to config") builds a signed-in panel holding raw and returns
// the dashboard.
func h2c(t *testing.T, name, raw string) pageOf {
	t.Helper()
	h := newHarness(t)
	h.readyWith(raw)
	_, body := h.get("/")
	if body == "" {
		t.Fatalf("%s: empty dashboard", name)
	}
	return pageOf{h: h, body: body}
}

// --- acceptance item 7, the provider's name ----------------------------------

// TestProviderNameIsCappedIsolatedAndNeverLogged: a name of right-to-left text,
// a control character and 200 bytes renders capped at 64 bytes on a word
// boundary, inside an isolated element, with the control character gone, and
// reaches no log line, no event and no status document.
func TestProviderNameIsCappedIsolatedAndNeverLogged(t *testing.T) {
	word := "سلام" // eight bytes
	long := strings.Repeat(word, 25)
	if len(long) != 200 {
		t.Fatalf("fixture is %d bytes, want 200", len(long))
	}
	raw := "vless://" + fakeUUIDForPanel + "@" + entryHostA + ":443?security=tls&type=raw&sni=" + fakeSNIForPanel + "#%01" + long + "\n" +
		entryLink("ss", entryHostC, "gamma")

	h := newHarness(t)
	h.readyWith(raw)
	h.switchOn()
	if res, _ := h.selectEntry("1"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /select: %d", res.StatusCode)
	}

	capped := strings.Repeat(word, 8) // the first 64 bytes, whole words
	for _, path := range []string{"/", "/?advanced=1"} {
		_, body := h.get(path)
		if !strings.Contains(body, `<bdi dir="ltr" class="mono">`+capped+`</bdi>`) {
			t.Errorf("%s: the capped name is not rendered inside an isolated element", path)
		}
		if strings.Contains(body, strings.Repeat(word, 9)) {
			t.Errorf("%s: more than 64 bytes of the name reached the page", path)
		}
		if strings.Contains(body, "\x01") {
			t.Errorf("%s: the control character reached the page", path)
		}
	}
	_, status := h.get("/status.json")
	if strings.Contains(status, word) {
		t.Error("the provider's name reached status.json")
	}
	if logs := h.logs.String(); strings.Contains(logs, word) {
		t.Errorf("the provider's name reached the log:\n%s", logs)
	}
	for _, e := range h.panel.events.entries() {
		for _, l := range Langs {
			if strings.Contains(e.Sentence(l), word) {
				t.Errorf("the provider's name reached an event sentence: %s", e.Sentence(l))
			}
		}
	}
	if len(h.panel.events.entries()) == 0 {
		t.Error("no events were recorded, so the event check proves nothing")
	}
}

// --- fillConfig's unhappy paths ----------------------------------------------

// TestAStoredConfigThatNoLongerParsesDrawsNoList: the problem sentence, and no
// radios to choose from a list that could not be read.
func TestAStoredConfigThatNoLongerParsesDrawsNoList(t *testing.T) {
	h := newHarness(t)
	h.readyWith(unparseableConfig())
	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgParseHeadline)) {
		t.Errorf("the page does not say the stored config cannot be read (want %q)", h.msg(MsgParseHeadline))
	}
	if listed, _ := entriesOn(t, body); len(listed) != 0 {
		t.Errorf("a list of %v was drawn for a config that does not parse", listed)
	}
}

// TestAChosenEntryThisBoxRefusesIsReportedAndTheOthersStayChoosable: the
// parser accepted the entry (a truncated id is a syntactically fine URI) and
// this box refuses it. The page says so and still lists the usable entries so
// the person can pick another, and switching on is refused with the same
// sentence rather than silently using a neighbour.
func TestAChosenEntryThisBoxRefusesIsReportedAndTheOthersStayChoosable(t *testing.T) {
	raw := strings.Join([]string{
		entryLink("vless", entryHostA, "alpha"),
		"vless://1111111@" + entryHostB + ":443?security=tls#badid",
		entryLink("ss", entryHostC, "gamma"),
	}, "\n")
	h := newHarness(t)
	h.readyWith(raw)
	if err := h.store.SelectProxyEntry(1); err != nil {
		t.Fatal(err)
	}

	_, body := h.get("/")
	if !strings.Contains(body, h.msg(MsgParseHeadline)) {
		t.Errorf("the page does not report the refused entry (want %q)", h.msg(MsgParseHeadline))
	}
	listed, _ := entriesOn(t, body)
	if len(listed) != 2 || listed[0] != 0 || listed[1] != 2 {
		t.Errorf("entries listed = %v, want [0 2]: the refused slot is skipped, the others keep their index", listed)
	}
	if !strings.Contains(body, h.msg(MsgConfigDroppedOne)) {
		t.Error("the refused entry is not counted as skipped")
	}

	res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /power: %d", res.StatusCode)
	}
	if n := len(h.priv.Starts()); n != 0 {
		t.Errorf("switching on with a refused entry chosen started the box %d times", n)
	}

	// Choosing a usable one recovers.
	if res, _ := h.selectEntry("2"); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /select: %d", res.StatusCode)
	}
	h.switchOn()
	if starts := h.priv.Starts(); len(starts) != 1 || !strings.Contains(string(starts[0].ConfigJSON), entryHostC) {
		t.Errorf("after choosing a usable entry the box did not start on it")
	}
}
